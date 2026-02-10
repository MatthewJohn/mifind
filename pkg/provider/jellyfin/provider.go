package jellyfin

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/yourname/mifind/internal/provider"
	"github.com/yourname/mifind/internal/types"
)

const (
	// Entity types
	TypeMovie  = "media.asset.jellyfin.movie"
	TypeSeries = "media.asset.jellyfin.series"
	TypeSeason = "media.asset.jellyfin.season"
	TypeEpisode = "media.asset.jellyfin.episode"

	// Relationship types
	RelSeasons  = "seasons"
	RelEpisodes = "episodes"

	// Attribute names
	AttrGenre          = "genre"
	AttrStudio         = "studio"
	AttrYear           = "year"
	AttrRating         = "rating"
	AttrOfficialRating = "official_rating"
	AttrRuntime        = "runtime"
	AttrOverview       = "overview"
)

// Provider implements the provider interface for Jellyfin.
// It connects to a Jellyfin server to search and browse movies and TV shows.
type Provider struct {
	provider.BaseProvider
	client *Client
}

// NewProvider creates a new Jellyfin provider.
func NewProvider() *Provider {
	return &Provider{
		BaseProvider: *provider.NewBaseProvider(provider.ProviderMetadata{
			Name:        "jellyfin",
			Description: "Jellyfin media server",
			ConfigSchema: provider.AddStandardConfigFields(map[string]provider.ConfigField{
				"url": {
					Type:        "string",
					Required:    true,
					Description: "Jellyfin server URL (e.g., https://jellyfin.example.com)",
				},
				"api_key": {
					Type:        "string",
					Required:    true,
					Description: "Jellyfin API key",
				},
				"user_id": {
					Type:        "string",
					Required:    true,
					Description: "Jellyfin user ID for user-specific content",
				},
			}),
		}),
	}
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return "jellyfin"
}

// Initialize sets up the Jellyfin provider with the given configuration.
func (p *Provider) Initialize(ctx context.Context, config map[string]any) error {
	// Get and set instance ID
	instanceID, ok := config["instance_id"].(string)
	if !ok || instanceID == "" {
		return provider.NewProviderError(provider.ErrorTypeConfig, "instance_id is required", nil)
	}
	p.SetInstanceID(instanceID)

	// Get URL
	jellyfinURL, ok := config["url"].(string)
	if !ok || jellyfinURL == "" {
		return provider.NewProviderError(provider.ErrorTypeConfig, "url is required", nil)
	}

	// Get API key
	apiKey, ok := config["api_key"].(string)
	if !ok || apiKey == "" {
		return provider.NewProviderError(provider.ErrorTypeConfig, "api_key is required", nil)
	}

	// Get user ID
	userID, ok := config["user_id"].(string)
	if !ok || userID == "" {
		return provider.NewProviderError(provider.ErrorTypeConfig, "user_id is required", nil)
	}

	// Create client
	p.client = NewClient(jellyfinURL, apiKey, userID)

	// Test connection
	if err := p.client.Health(ctx); err != nil {
		return provider.NewProviderError(provider.ErrorTypeAuth, "failed to connect to Jellyfin", err)
	}

	return nil
}

// Discover performs a full discovery of all items.
// For Jellyfin, we return a subset of recent items to avoid loading everything.
func (p *Provider) Discover(ctx context.Context) ([]types.Entity, error) {
	return p.discoverWithLimit(ctx, 100)
}

// DiscoverSince performs incremental discovery since the given timestamp.
func (p *Provider) DiscoverSince(ctx context.Context, since time.Time) ([]types.Entity, error) {
	// Use minPremiereDate to get items added/modified since the given time
	sinceStr := since.Format(time.RFC3339)
	params := GetItemsParams{
		MinPremiereDate: sinceStr,
		IncludeItemTypes: []string{"Movie", "Series"},
		Limit:           500,
		Recursive:       true,
		SortBy:          "DateCreated",
		SortOrder:       "Descending",
	}

	resp, err := p.client.GetItems(ctx, params)
	if err != nil {
		return nil, err
	}

	entities := make([]types.Entity, 0, len(resp.Items))
	for _, item := range resp.Items {
		entities = append(entities, p.itemToEntity(item))
	}

	return entities, nil
}

// Hydrate retrieves full details of an entity by ID.
func (p *Provider) Hydrate(ctx context.Context, id string) (types.Entity, error) {
	entityID, err := provider.ParseEntityID(id)
	if err != nil {
		return types.Entity{}, provider.ErrNotFound
	}

	item, err := p.client.GetItem(ctx, entityID.ResourceID())
	if err != nil {
		return types.Entity{}, provider.ErrNotFound
	}

	return p.itemToEntity(*item), nil
}

// GetRelated retrieves entities related to an entity.
func (p *Provider) GetRelated(ctx context.Context, id string, relType string) ([]types.Entity, error) {
	entityID, err := provider.ParseEntityID(id)
	if err != nil {
		return nil, err
	}
	itemID := entityID.ResourceID()

	switch relType {
	case RelSeasons:
		// Get seasons for a series
		seasonsResp, err := p.client.GetSeasons(ctx, itemID)
		if err != nil {
			return nil, err
		}
		entities := make([]types.Entity, 0, len(seasonsResp.Items))
		for _, season := range seasonsResp.Items {
			entities = append(entities, p.itemToEntity(season))
		}
		return entities, nil

	case RelEpisodes:
		// Get episodes for a season
		// First, we need to get the series ID
		item, err := p.client.GetItem(ctx, itemID)
		if err != nil {
			return nil, err
		}
		// Find parent series ID
		var seriesID string
		if item.Type == "Season" && item.ParentIndexNumber > 0 {
			// For a season, we need to get its parent series
			// The item doesn't directly give us this, so we need to search
			seriesResp, err := p.client.GetItems(ctx, GetItemsParams{
				IncludeItemTypes: []string{"Series"},
				Recursive:        true,
				Limit:            1,
			})
			if err == nil && len(seriesResp.Items) > 0 {
				seriesID = seriesResp.Items[0].ID
			}
		} else if item.Type == "Series" {
			seriesID = item.ID
		}

		if seriesID == "" {
			return nil, fmt.Errorf("could not find series ID")
		}

		episodesResp, err := p.client.GetEpisodes(ctx, seriesID, itemID)
		if err != nil {
			return nil, err
		}
		entities := make([]types.Entity, 0, len(episodesResp.Items))
		for _, episode := range episodesResp.Items {
			entities = append(entities, p.itemToEntity(episode))
		}
		return entities, nil

	default:
		return nil, provider.ErrNotFound
	}
}

// Search performs a search query on this provider.
func (p *Provider) Search(ctx context.Context, query provider.SearchQuery) ([]types.Entity, error) {
	params := GetItemsParams{
		SearchTerm: query.Query,
		Limit:      query.Limit,
		StartIndex: query.Offset,
		Recursive:  true,
	}

	// Map filters to Jellyfin parameters
	if genre, ok := query.Filters[AttrGenre].([]string); ok && len(genre) > 0 {
		params.Genres = genre
	}
	if studio, ok := query.Filters[AttrStudio].([]string); ok && len(studio) > 0 {
		params.Studios = studio
	}
	if years, ok := query.Filters[AttrYear].([]string); ok && len(years) > 0 {
		params.Years = years
	}
	if minRating, ok := query.Filters[AttrRating].(float64); ok {
		params.MinCommunityRating = minRating
	}
	if officialRating, ok := query.Filters[AttrOfficialRating].(string); ok {
		params.MaxOfficialRating = officialRating
	}

	// Filter by type
	switch query.Type {
	case TypeMovie:
		params.IncludeItemTypes = []string{"Movie"}
	case TypeSeries:
		params.IncludeItemTypes = []string{"Series"}
	case TypeSeason:
		params.IncludeItemTypes = []string{"Season"}
	case TypeEpisode:
		params.IncludeItemTypes = []string{"Episode"}
	}

	resp, err := p.client.GetItems(ctx, params)
	if err != nil {
		return nil, err
	}

	entities := make([]types.Entity, 0, len(resp.Items))
	for _, item := range resp.Items {
		entities = append(entities, p.itemToEntity(item))
	}

	return entities, nil
}

// FilterCapabilities returns the filter capabilities for Jellyfin.
func (p *Provider) FilterCapabilities(ctx context.Context) (map[string]provider.FilterCapability, error) {
	return map[string]provider.FilterCapability{
		AttrGenre: {
			Type:          types.AttributeTypeString,
			SupportsEq:    true,
			SupportsContains: false,
			Description:   "Filter by genre",
		},
		AttrStudio: {
			Type:          types.AttributeTypeString,
			SupportsEq:    true,
			SupportsContains: false,
			Description:   "Filter by studio",
		},
		AttrYear: {
			Type:          types.AttributeTypeInt,
			SupportsEq:    true,
			SupportsRange: true,
			Description:   "Filter by production year",
		},
		AttrRating: {
			Type:          types.AttributeTypeFloat,
			SupportsRange: true,
			Description:   "Filter by community rating",
		},
		AttrOfficialRating: {
			Type:          types.AttributeTypeString,
			SupportsEq:    true,
			Description:   "Filter by official rating (PG, PG-13, TV-MA, etc)",
		},
	}, nil
}

// FilterValues returns available filter values for the given filter name.
func (p *Provider) FilterValues(ctx context.Context, filterName string) ([]provider.FilterOption, error) {
	switch filterName {
	case AttrGenre:
		genres, err := p.client.GetGenres(ctx)
		if err != nil {
			return nil, err
		}
		options := make([]provider.FilterOption, 0, len(genres))
		for _, genre := range genres {
			options = append(options, provider.FilterOption{
				Value: genre.Name,
				Label: genre.Name,
			})
		}
		return options, nil

	case AttrStudio:
		studios, err := p.client.GetStudios(ctx)
		if err != nil {
			return nil, err
		}
		options := make([]provider.FilterOption, 0, len(studios))
		for _, studio := range studios {
			options = append(options, provider.FilterOption{
				Value: studio.Name,
				Label: studio.Name,
			})
		}
		return options, nil

	default:
		return nil, nil
	}
}

// Shutdown gracefully shuts down the provider.
func (p *Provider) Shutdown(ctx context.Context) error {
	return nil
}

// itemToEntity converts a Jellyfin Item to a types.Entity.
func (p *Provider) itemToEntity(item Item) types.Entity {
	// Determine entity type
	var entityType string
	switch item.Type {
	case "Movie":
		entityType = TypeMovie
	case "Series":
		entityType = TypeSeries
	case "Season":
		entityType = TypeSeason
	case "Episode":
		entityType = TypeEpisode
	default:
		entityType = "media.asset.jellyfin." + item.Type
	}

	// Build entity ID
	entityID := p.BuildEntityID(item.ID).String()

	// Create entity
	entity := types.NewEntity(entityID, entityType, p.Name(), item.Name)

	// Set description
	if item.Overview != "" {
		entity.Description = item.Overview
	} else if item.OriginalTitle != "" && item.OriginalTitle != item.Name {
		entity.Description = item.OriginalTitle
	}

	// Add timestamp
	if item.PremiereDate != nil {
		entity.Timestamp = *item.PremiereDate
	}

	// Add attributes
	if len(item.Genres) > 0 {
		entity.AddAttribute(AttrGenre, item.Genres)
	}
	if len(item.Studios) > 0 {
		studioNames := make([]string, 0, len(item.Studios))
		for _, studio := range item.Studios {
			if studio.Name != "" {
				studioNames = append(studioNames, studio.Name)
			}
		}
		if len(studioNames) > 0 {
			entity.AddAttribute(AttrStudio, studioNames)
		}
	}
	if item.ProductionYear > 0 {
		entity.AddAttribute(AttrYear, item.ProductionYear)
	}
	if item.CommunityRating > 0 {
		entity.AddAttribute(AttrRating, item.CommunityRating)
	}
	if item.OfficialRating != "" {
		entity.AddAttribute(AttrOfficialRating, item.OfficialRating)
	}
	if item.RunTimeTicks > 0 {
		// Convert ticks to minutes (1 tick = 100 nanoseconds)
		minutes := int(item.RunTimeTicks / 600000000)
		entity.AddAttribute(AttrRuntime, minutes)
	}
	if item.Overview != "" {
		entity.AddAttribute(AttrOverview, item.Overview)
	}

	// Add search tokens
	entity.AddSearchToken(item.Name)
	if item.OriginalTitle != "" {
		entity.AddSearchToken(item.OriginalTitle)
	}
	for _, genre := range item.Genres {
		entity.AddSearchToken(genre)
	}
	for _, studio := range item.Studios {
		entity.AddSearchToken(studio.Name)
	}

	// Add relationships
	if item.Type == "Series" {
		// Series has seasons
		entity.AddRelationship(RelSeasons, "")
	} else if item.Type == "Season" {
		// Season has episodes
		entity.AddRelationship(RelEpisodes, "")
		// Season is part of a series
		if item.SeriesName != "" {
			entity.AddRelationship(types.RelParent, item.SeriesName)
		}
	} else if item.Type == "Episode" {
		// Episode is part of a season/series
		if item.SeriesName != "" {
			entity.AddRelationship(types.RelParent, item.SeriesName)
		}
	}

	return entity
}

// discoverWithLimit performs discovery with a limit on the number of items.
func (p *Provider) discoverWithLimit(ctx context.Context, limit int) ([]types.Entity, error) {
	params := GetItemsParams{
		IncludeItemTypes: []string{"Movie", "Series"},
		Limit:           limit,
		Recursive:       true,
		SortBy:          "DateCreated",
		SortOrder:       "Descending",
	}

	resp, err := p.client.GetItems(ctx, params)
	if err != nil {
		return nil, err
	}

	entities := make([]types.Entity, 0, len(resp.Items))
	for _, item := range resp.Items {
		entities = append(entities, p.itemToEntity(item))
	}

	return entities, nil
}

// AttributeExtensions returns provider-specific attribute extensions.
func (p *Provider) AttributeExtensions(ctx context.Context) map[string]types.AttributeDef {
	return map[string]types.AttributeDef{
		AttrGenre: {
			Name: "genre",
			Type: types.AttributeTypeString,
			UI: types.UIConfig{
				Widget: "multiselect",
				Icon:   "Tag",
				Group:  "jellyfin",
				Label:  "Genre",
			},
			Filter: types.FilterConfig{
				SupportsEq:  true,
				Cacheable:   true,
				CacheTTL:    24 * time.Hour,
			},
		},
		AttrStudio: {
			Name: "studio",
			Type: types.AttributeTypeString,
			UI: types.UIConfig{
				Widget: "multiselect",
				Icon:   "Building",
				Group:  "jellyfin",
				Label:  "Studio",
			},
			Filter: types.FilterConfig{
				SupportsEq:  true,
				Cacheable:   true,
				CacheTTL:    24 * time.Hour,
			},
		},
		AttrYear: {
			Name: "year",
			Type: types.AttributeTypeInt,
			UI: types.UIConfig{
				Widget: "range",
				Icon:   "Calendar",
				Group:  "jellyfin",
				Label:  "Year",
			},
			Filter: types.FilterConfig{
				SupportsEq:    true,
				SupportsRange: true,
				Cacheable:     true,
				CacheTTL:      24 * time.Hour,
			},
		},
		AttrRating: {
			Name: "rating",
			Type: types.AttributeTypeFloat,
			UI: types.UIConfig{
				Widget: "range",
				Icon:   "Star",
				Group:  "jellyfin",
				Label:  "Community Rating",
			},
			Filter: types.FilterConfig{
				SupportsRange: true,
			},
		},
		AttrOfficialRating: {
			Name: "official_rating",
			Type: types.AttributeTypeString,
			UI: types.UIConfig{
				Widget: "select",
				Icon:   "Shield",
				Group:  "jellyfin",
				Label:  "Rating",
			},
			Filter: types.FilterConfig{
				SupportsEq: true,
			},
		},
	}
}

// Log returns a logger for this provider.
func (p *Provider) Log() *zerolog.Logger {
	logger := zerolog.Nop()
	return &logger
}
