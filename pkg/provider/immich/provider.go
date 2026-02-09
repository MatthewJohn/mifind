package immich

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/yourname/mifind/internal/provider"
	"github.com/yourname/mifind/internal/types"
)

// Provider implements the provider interface for Immich.
// It connects to an Immich server to search and browse photos, videos, albums, and people.
type Provider struct {
	provider.BaseProvider
	client *Client
	logger *zerolog.Logger
}

// NewProvider creates a new Immich provider.
func NewProvider() *Provider {
	return &Provider{
		BaseProvider: *provider.NewBaseProvider(provider.ProviderMetadata{
			Name:        "immich",
			Description: "Immich photo and video server",
			ConfigSchema: provider.AddStandardConfigFields(map[string]provider.ConfigField{
				"url": {
					Type:        "string",
					Required:    true,
					Description: "URL of the Immich server (e.g., https://immich.example.com)",
				},
				"api_key": {
					Type:        "string",
					Required:    true,
					Description: "Immich API key",
				},
				"insecure_skip_verify": {
					Type:        "bool",
					Required:    false,
					Description: "Skip TLS certificate verification (for self-signed certificates)",
				},
				"debug": {
					Type:        "bool",
					Required:    false,
					Description: "Enable debug logging for Immich API calls",
				},
			}),
		}),
	}
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return "immich"
}

// Initialize sets up the Immich provider with the given configuration.
func (p *Provider) Initialize(ctx context.Context, config map[string]any) error {
	// Get and set instance ID
	instanceID, ok := config["instance_id"].(string)
	if !ok || instanceID == "" {
		return provider.NewProviderError(provider.ErrorTypeConfig, "instance_id is required", nil)
	}
	p.SetInstanceID(instanceID)

	// Get URL
	url, ok := config["url"].(string)
	if !ok || url == "" {
		return provider.NewProviderError(provider.ErrorTypeConfig, "url is required", nil)
	}

	// Get API key
	apiKey, ok := config["api_key"].(string)
	if !ok || apiKey == "" {
		return provider.NewProviderError(provider.ErrorTypeConfig, "api_key is required", nil)
	}

	// Get insecure_skip_verify option (optional, defaults to false)
	insecureSkipVerify := false
	if skipVerify, ok := config["insecure_skip_verify"].(bool); ok {
		insecureSkipVerify = skipVerify
	}

	// Get debug option (optional, defaults to false)
	debug := false
	if dbg, ok := config["debug"].(bool); ok {
		debug = dbg
	}

	// Create logger if debug is enabled
	logger := zerolog.Nop()
	if debug {
		log := zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "15:04:05"}).
			With().Timestamp().Str("component", "immich").Logger()
		logger = log
		log.Info().Bool("debug", true).Msg("Immich provider initialized with debug logging")
	}
	p.logger = &logger

	// Create client
	p.client = NewClientWithLogger(url, apiKey, insecureSkipVerify, &logger)

	// Test connection
	if err := p.client.Health(ctx); err != nil {
		return provider.NewProviderError(provider.ErrorTypeAuth, "failed to connect to Immich", err)
	}

	return nil
}

// Discover performs a full discovery of all assets.
// For Immich, this returns a subset of recent assets to avoid loading everything.
func (p *Provider) Discover(ctx context.Context) ([]types.Entity, error) {
	// Get recent assets (limit to 100 for discovery)
	searchResult, err := p.client.Search(ctx, "", 100)
	if err != nil {
		return nil, err
	}

	var entities []types.Entity
	if searchResult.Assets != nil {
		for _, asset := range searchResult.Assets.Items {
			entities = append(entities, p.assetToEntity(asset))
		}
	}

	// Also discover albums
	albums, err := p.client.ListAlbums(ctx, 50)
	if err == nil {
		for _, album := range albums {
			entities = append(entities, p.albumToEntity(album))
		}
	}

	// Also discover people
	people, err := p.client.ListPeople(ctx, 50)
	if err == nil {
		for _, person := range people {
			entities = append(entities, p.personToEntity(person))
		}
	}

	return entities, nil
}

// DiscoverSince performs incremental discovery since the given timestamp.
func (p *Provider) DiscoverSince(ctx context.Context, since time.Time) ([]types.Entity, error) {
	// Immich API doesn't have a direct "since" parameter for search
	// For now, return empty - this could be enhanced with date filtering
	return []types.Entity{}, nil
}

// Hydrate retrieves full details of an entity by ID.
func (p *Provider) Hydrate(ctx context.Context, id string) (types.Entity, error) {
	entityID, err := provider.ParseEntityID(id)
	if err != nil {
		return types.Entity{}, provider.ErrNotFound
	}

	// Extract the resource type from the entity type
	// For now, assume all IDs are assets
	asset, err := p.client.GetAsset(ctx, entityID.ResourceID())
	if err == nil {
		return p.assetToEntity(*asset), nil
	}

	// Try as album
	album, err := p.client.GetAlbum(ctx, entityID.ResourceID())
	if err == nil {
		return p.albumToEntity(*album), nil
	}

	// Try as person
	person, err := p.client.GetPerson(ctx, entityID.ResourceID())
	if err == nil {
		return p.personToEntity(*person), nil
	}

	return types.Entity{}, provider.ErrNotFound
}

// GetRelated retrieves entities related to an entity.
func (p *Provider) GetRelated(ctx context.Context, id string, relType string) ([]types.Entity, error) {
	entityID, err := provider.ParseEntityID(id)
	if err != nil {
		return nil, err
	}
	resourceID := entityID.ResourceID()

	switch relType {
	case types.RelAlbum:
		// Get assets in the album
		assets, err := p.client.GetAlbumAssets(ctx, resourceID, 100)
		if err != nil {
			return nil, err
		}
		var entities []types.Entity
		for _, asset := range assets {
			entities = append(entities, p.assetToEntity(asset))
		}
		return entities, nil

	case types.RelPerson:
		// For now, people search is complex and requires additional API calls
		return []types.Entity{}, nil

	default:
		return []types.Entity{}, nil
	}
}

// Search performs a search query on Immich.
func (p *Provider) Search(ctx context.Context, query provider.SearchQuery) ([]types.Entity, error) {
	// Debug log search request
	p.logger.Debug().
		Str("query", query.Query).
		Int("limit", query.Limit).
		Str("type", query.Type).
		Interface("filters", query.Filters).
		Msg("Immich: Search request")

	// Extract filters from query
	var peopleIDs []string
	var city, state, country string
	var albumID string

	// Handle person filter - can be a single value or array of values
	if personFilter, ok := query.Filters["person"]; ok && personFilter != nil {
		switch v := personFilter.(type) {
		case string:
			peopleIDs = []string{v}
		case []string:
			peopleIDs = v
		case []any:
			peopleIDs = make([]string, 0, len(v))
			for _, item := range v {
				if str, ok := item.(string); ok {
					peopleIDs = append(peopleIDs, str)
				}
			}
		}
		p.logger.Debug().
			Strs("people_ids", peopleIDs).
			Msg("Immich: Filter by people")
	}

	// Handle location filters
	if cityFilter, ok := query.Filters[types.AttrLocationCity]; ok && cityFilter != nil {
		city = fmt.Sprint(cityFilter)
		p.logger.Debug().
			Str("city", city).
			Msg("Immich: Filter by city")
	}
	if stateFilter, ok := query.Filters[types.AttrLocationState]; ok && stateFilter != nil {
		city = fmt.Sprint(stateFilter)
		p.logger.Debug().
			Str("state", state).
			Msg("Immich: Filter by state")
	}
	if countryFilter, ok := query.Filters[types.AttrLocationCountry]; ok && countryFilter != nil {
		city = fmt.Sprint(countryFilter)
		p.logger.Debug().
			Str("country", country).
			Msg("Immich: Filter by country")
	}

	// Handle album filter
	if albumFilter, ok := query.Filters[types.AttrAlbum]; ok && albumFilter != nil {
		albumID = fmt.Sprint(albumFilter)
		p.logger.Debug().
			Str("album_id", albumID).
			Msg("Immich: Filter by album")
	}

	searchResult, err := p.client.SearchWithFilters(ctx, query.Query, query.Limit, peopleIDs, country, state, city, albumID)
	if err != nil {
		p.logger.Error().Err(err).Msg("Immich: Search request failed")
		return nil, err
	}

	p.logger.Debug().
		Int("assets", len(searchResult.Assets.Items)).
		Int("albums", len(searchResult.Albums.Items)).
		Msg("Immich: Search response")

	var entities []types.Entity

	// Add assets
	if searchResult.Assets != nil {
		for _, asset := range searchResult.Assets.Items {
			// Filter by type if specified
			if query.Type != "" {
				assetType := FileTypeToMifindType(asset.Type)
				if query.Type != assetType {
					continue
				}
			}
			entities = append(entities, p.assetToEntity(asset))
		}
	}

	// Add albums
	if searchResult.Albums != nil {
		for _, album := range searchResult.Albums.Items {
			entities = append(entities, p.albumToEntity(album))
		}
	}

	// Add people - fallback since Immich search API doesn't return people
	// Fetch all people and filter locally by name (only if no person filter applied)
	if len(peopleIDs) == 0 {
		people, err := p.client.ListPeople(ctx, 0)
		if err == nil && len(people) > 0 {
			queryLower := strings.ToLower(query.Query)
			for _, person := range people {
				if person.Name != "" && strings.Contains(strings.ToLower(person.Name), queryLower) {
					entities = append(entities, p.personToEntity(person))
					if query.Limit > 0 && len(entities) >= query.Limit {
						break
					}
				}
			}
		}
	}

	return entities, nil
}

// SupportsIncremental returns false - Immich provider doesn't support efficient incremental updates.
func (p *Provider) SupportsIncremental() bool {
	return false
}

// SupportsRelevanceScore returns true - Immich may return relevance scores from search.
func (p *Provider) SupportsRelevanceScore() bool {
	return true
}

// Shutdown shuts down the Immich provider.
func (p *Provider) Shutdown(ctx context.Context) error {
	// No cleanup needed
	return nil
}

// FilterValues returns pre-obtained filter values for supported filters.
func (p *Provider) FilterValues(ctx context.Context, filterName string) ([]provider.FilterOption, error) {
	switch filterName {
	case types.AttrPerson:
		return p.getPeopleFilterOptions(ctx)
	case types.AttrAlbum:
		return p.getAlbumsFilterOptions(ctx)
	case types.AttrLocationCity:
		return p.getCityFilterOptions(ctx)
	case types.AttrLocationState:
		return p.getStateFilterOptions(ctx)
	case types.AttrLocationCountry:
		return p.getCountryFilterOptions(ctx)
	default:
		return []provider.FilterOption{}, nil
	}
}

// getPeopleFilterOptions returns all people as filter options.
func (p *Provider) getPeopleFilterOptions(ctx context.Context) ([]provider.FilterOption, error) {
	people, err := p.client.ListPeople(ctx, 0) // 0 = no limit, get all
	if err != nil {
		return nil, fmt.Errorf("failed to list people: %w", err)
	}

	options := make([]provider.FilterOption, len(people))
	for i, person := range people {
		label := person.Name
		if label == "" {
			label = "Unknown Person"
		}
		options[i] = provider.FilterOption{
			Value: person.ID,
			Label: label,
			Count: 0, // Could be enhanced to include asset count
		}
	}
	return options, nil
}

// getAlbumsFilterOptions returns all albums as filter options.
func (p *Provider) getAlbumsFilterOptions(ctx context.Context) ([]provider.FilterOption, error) {
	albums, err := p.client.ListAlbums(ctx, 0) // 0 = no limit, get all
	if err != nil {
		return nil, fmt.Errorf("failed to list albums: %w", err)
	}

	options := make([]provider.FilterOption, len(albums))
	for i, album := range albums {
		options[i] = provider.FilterOption{
			Value: album.ID,
			Label: album.AlbumName,
			Count: album.AssetCount,
		}
	}
	return options, nil
}

func (p *Provider) getCityFilterOptions(ctx context.Context) ([]provider.FilterOption, error) {
	_, _, cityFilterOptions, err := p.getLocationFilterOptions(ctx)
	return cityFilterOptions, err
}

func (p *Provider) getStateFilterOptions(ctx context.Context) ([]provider.FilterOption, error) {
	_, stateFilterOptions, _, err := p.getLocationFilterOptions(ctx)
	return stateFilterOptions, err
}

func (p *Provider) getCountryFilterOptions(ctx context.Context) ([]provider.FilterOption, error) {
	countryFilterOptions, _, _, err := p.getLocationFilterOptions(ctx)
	return countryFilterOptions, err
}

// getLocationFilterOptions returns unique locations as filter options.
func (p *Provider) getLocationFilterOptions(ctx context.Context) ([]provider.FilterOption, []provider.FilterOption, []provider.FilterOption, error) {
	// For locations, we need to extract unique location values from assets
	// This requires a search with no query to get all assets, then extract locations
	searchResults, err := p.client.GetCitySearch(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to search for locations: %w", err)
	}

	// Extract unique locations
	cityMap := make(map[string]int)
	stateMap := make(map[string]int)
	countryMap := make(map[string]int)

	for _, asset := range searchResults {
		if asset.City != "" {
			cityMap[asset.City]++
		}
		if asset.State != "" {
			stateMap[asset.State]++
		}
		if asset.Country != "" {
			countryMap[asset.Country]++
		}
	}

	cityOptions := make([]provider.FilterOption, 0, len(cityMap))
	for city, count := range cityMap {
		cityOptions = append(cityOptions, provider.FilterOption{
			Value: city,
			Label: city,
			Count: count,
		})
	}
	stateOptions := make([]provider.FilterOption, 0, len(stateMap))
	for state, count := range cityMap {
		stateOptions = append(stateOptions, provider.FilterOption{
			Value: state,
			Label: state,
			Count: count,
		})
	}
	countryOptions := make([]provider.FilterOption, 0, len(countryMap))
	for country, count := range countryMap {
		countryOptions = append(countryOptions, provider.FilterOption{
			Value: country,
			Label: country,
			Count: count,
		})
	}
	return countryOptions, stateOptions, cityOptions, nil
}

// FilterCapabilities returns the filter capabilities for the Immich provider.
func (p *Provider) FilterCapabilities(ctx context.Context) (map[string]provider.FilterCapability, error) {
	return map[string]provider.FilterCapability{
		types.AttrType: {
			Type:        types.AttributeTypeString,
			SupportsEq:  true,
			SupportsNeq: true,
			Options: []provider.FilterOption{
				{Value: types.TypeMediaAssetPhoto, Label: "Photo"},
				{Value: types.TypeMediaAssetVideo, Label: "Video"},
			},
			Description: "Media asset type",
		},
		types.AttrIsFavorite: {
			Type:       types.AttributeTypeBool,
			SupportsEq: true,
			Options: []provider.FilterOption{
				{Value: "true", Label: "Yes"},
				{Value: "false", Label: "No"},
			},
			Description: "Favorite status",
		},
		types.AttrIsArchived: {
			Type:       types.AttributeTypeBool,
			SupportsEq: true,
			Options: []provider.FilterOption{
				{Value: "true", Label: "Yes"},
				{Value: "false", Label: "No"},
			},
			Description: "Archived status",
		},
		types.AttrCreated: {
			Type:          types.AttributeTypeTime,
			SupportsEq:    true,
			SupportsRange: true,
			Description:   "Creation timestamp (Unix)",
		},
		types.AttrWidth: {
			Type:          types.AttributeTypeInt,
			SupportsEq:    true,
			SupportsRange: true,
			Min:           ptrFloat64(1),
			Description:   "Image/video width in pixels",
		},
		types.AttrHeight: {
			Type:          types.AttributeTypeInt,
			SupportsEq:    true,
			SupportsRange: true,
			Min:           ptrFloat64(1),
			Description:   "Image/video height in pixels",
		},
		types.AttrCamera: {
			Type:             types.AttributeTypeString,
			SupportsEq:       true,
			SupportsContains: true,
			Description:      "Camera make/model",
		},
		types.AttrGPS: {
			Type:          types.AttributeTypeGPS,
			SupportsRange: true,
			Description:   "GPS coordinates",
		},
		types.AttrLocationCity: {
			Type:             types.AttributeTypeString,
			SupportsEq:       true,
			SupportsContains: true,
			Description:      "City name",
		},
		types.AttrLocationState: {
			Type:             types.AttributeTypeString,
			SupportsEq:       true,
			SupportsContains: true,
			Description:      "State name",
		},
		types.AttrLocationCountry: {
			Type:             types.AttributeTypeString,
			SupportsEq:       true,
			SupportsContains: true,
			Description:      "Country name",
		},
		types.AttrAlbum: {
			Type:        types.AttributeTypeString,
			SupportsEq:  true,
			Description: "Album name",
		},
		types.AttrPerson: {
			Type:        types.AttributeTypeStringSlice,
			SupportsEq:  true,
			Description: "Person (detected faces)",
		},
	}, nil
}

// AttributeExtensions returns provider-specific attribute definitions.
// Extends core attributes (album, person) with provider-level metadata for Immich.
// Location filters (city, state, country) are also provider-level for Immich.
func (p *Provider) AttributeExtensions(ctx context.Context) map[string]types.AttributeDef {
	return map[string]types.AttributeDef{
		// Extend core person attribute with provider-specific UI/behavior
		types.AttrPerson: {
			Name:          types.AttrPerson,
			Type:          types.AttributeTypeStringSlice,
			Filterable:    true,
			Description:   "Person (detected faces) - filter handled by Immich API",
			AlwaysVisible:  true, // Always show person filter
			UI: types.UIConfig{
				Widget:   "multiselect",
				Icon:     "Users",
				Group:    "provider-immich",
				Label:    "People",
				Priority: 10,
			},
			Filter: types.FilterConfig{
				SupportsEq:     true,
				Cacheable:      true,
				CacheTTL:       24 * time.Hour,
				ProviderLevel:  true, // Filtered by Immich API, not entity attributes
			},
		},
		// Location filters - provider-level for Immich
		types.AttrLocationCity: {
			Name:       types.AttrLocationCity,
			Type:       types.AttributeTypeString,
			Filterable: true,
			Description: "City name - filter handled by Immich API",
			UI: types.UIConfig{
				Widget:   "input",
				Icon:     "MapPin",
				Group:    "provider-immich",
				Label:    "City",
				Priority: 12,
			},
			Filter: types.FilterConfig{
				SupportsEq:       true,
				SupportsContains: true,
				Cacheable:        true,
				CacheTTL:         24 * time.Hour,
				ProviderLevel:    true, // Filtered by Immich API, not entity attributes
			},
		},
		types.AttrLocationState: {
			Name:       types.AttrLocationState,
			Type:       types.AttributeTypeString,
			Filterable: true,
			Description: "State/Province name - filter handled by Immich API",
			UI: types.UIConfig{
				Widget:   "input",
				Icon:     "MapPin",
				Group:    "provider-immich",
				Label:    "State",
				Priority: 13,
			},
			Filter: types.FilterConfig{
				SupportsEq:       true,
				SupportsContains: true,
				Cacheable:        true,
				CacheTTL:         24 * time.Hour,
				ProviderLevel:    true, // Filtered by Immich API, not entity attributes
			},
		},
		types.AttrLocationCountry: {
			Name:       types.AttrLocationCountry,
			Type:       types.AttributeTypeString,
			Filterable: true,
			Description: "Country name - filter handled by Immich API",
			UI: types.UIConfig{
				Widget:   "input",
				Icon:     "Map",
				Group:    "provider-immich",
				Label:    "Country",
				Priority: 14,
			},
			Filter: types.FilterConfig{
				SupportsEq:       true,
				SupportsContains: true,
				Cacheable:        true,
				CacheTTL:         24 * time.Hour,
				ProviderLevel:    true, // Filtered by Immich API, not entity attributes
			},
		},
	}
}

// ptrFloat64 returns a pointer to a float64.
func ptrFloat64(v float64) *float64 {
	return &v
}

// assetToEntity converts an Immich asset to an Entity.
func (p *Provider) assetToEntity(asset Asset) types.Entity {
	entityID := p.BuildEntityID(asset.ID).String()
	entityType := FileTypeToMifindType(asset.Type)

	entity := types.NewEntity(entityID, entityType, p.Name(), asset.OriginalFileName)
	entity.AddAttribute(types.AttrPath, asset.OriginalPath)
	entity.AddAttribute("original_file_name", asset.OriginalFileName)

	// Debug: Log file size from API
	if p.client.logger != nil {
		p.client.logger.Debug().
			Str("asset_id", asset.ID).
			Int64("file_size", asset.FileSize).
			Str("original_file_name", asset.OriginalFileName).
			Msg("Asset file size from API")
	}

	entity.AddAttribute(types.AttrSize, asset.FileSize)

	// Build web URL for Immich asset
	webURL := fmt.Sprintf("%s/photos/%s", strings.TrimSuffix(p.client.baseURL, "/api"), asset.ID)
	entity.AddAttribute("web_url", webURL)

	// Build thumbnail URL - use mifind proxy endpoint
	// Store original Immich URL for the proxy to use
	originalThumbnailURL := fmt.Sprintf("%s/api/assets/%s/thumbnail?size=thumbnail", p.client.baseURL, asset.ID)
	entity.AddAttribute("_immich_thumbnail_url", originalThumbnailURL)
	// Public thumbnail URL uses mifind proxy
	entity.AddAttribute("thumbnail_url", fmt.Sprintf("/api/thumbnail?id=%s", entityID))

	entity.AddAttribute("is_favorite", asset.IsFavorite)
	entity.AddAttribute("is_archived", asset.IsArchived)
	entity.AddAttribute(types.AttrCreated, asset.LocalDateTime.Unix())

	if asset.Width > 0 {
		entity.AddAttribute(types.AttrWidth, asset.Width)
	}
	if asset.Height > 0 {
		entity.AddAttribute(types.AttrHeight, asset.Height)
	}
	if asset.Duration.Value() > 0 {
		entity.AddAttribute(types.AttrDuration, int64(asset.Duration.Value()))
	}
	if asset.Description != "" {
		entity.AddAttribute(types.AttrDescription, asset.Description)
	}
	if asset.Country != "" {
		entity.AddAttribute(types.AttrLocationCountry, asset.Country)
	}
	if asset.State != "" {
		entity.AddAttribute(types.AttrLocationState, asset.State)
	}
	if asset.City != "" {
		entity.AddAttribute(types.AttrLocationCity, asset.City)
	}

	// Add EXIF data
	if asset.ExifInfo != nil {
		exif := asset.ExifInfo
		if exif.Make != "" {
			entity.AddAttribute(types.AttrCamera, exif.Make)
			if exif.Model != "" {
				entity.AddAttribute(types.AttrCamera, fmt.Sprintf("%s %s", exif.Make, exif.Model))
			}
		}
		if exif.ISOSpeed > 0 {
			entity.AddAttribute(types.AttrISO, exif.ISOSpeed)
		}
		if exif.FNumber > 0 {
			entity.AddAttribute(types.AttrAperture, fmt.Sprintf("f/%.1f", exif.FNumber))
		}
		if exif.LensModel != "" {
			entity.AddAttribute(types.AttrLens, exif.LensModel)
		}
		if exif.DateTimeOriginal != "" {
			entity.AddAttribute("date_time_original", exif.DateTimeOriginal)
		}
		if exif.GPS != nil {
			entity.AddAttribute(types.AttrGPS, types.GPS{
				Latitude:  exif.GPS.Latitude,
				Longitude: exif.GPS.Longitude,
			})
			entity.AddAttribute(types.AttrLatitude, exif.GPS.Latitude)
			entity.AddAttribute(types.AttrLongitude, exif.GPS.Longitude)
		}
	}

	// Add search tokens
	entity.AddSearchToken(asset.OriginalFileName)
	if asset.Description != "" {
		entity.AddSearchToken(asset.Description)
	}
	if asset.Country != "" {
		entity.AddSearchToken(asset.Country)
	}
	if asset.State != "" {
		entity.AddSearchToken(asset.State)
	}
	if asset.City != "" {
		entity.AddSearchToken(asset.City)
	}

	// Add album relationship
	// This would require additional API calls to get the asset's albums
	// For now, skip this

	return entity
}

// albumToEntity converts an Immich album to an Entity.
func (p *Provider) albumToEntity(album Album) types.Entity {
	entityID := p.BuildEntityID(album.ID).String()
	entityType := "collection.album"

	entity := types.NewEntity(entityID, entityType, p.Name(), album.AlbumName)
	entity.AddAttribute(types.AttrAlbum, album.AlbumName)
	entity.AddAttribute("asset_count", album.AssetCount)
	entity.AddAttribute(types.AttrCreated, album.CreatedAt.Unix())

	if album.Description != "" {
		entity.AddAttribute(types.AttrDescription, album.Description)
	}

	// Build web URL for Immich album
	webURL := fmt.Sprintf("%s/albums/%s", strings.TrimSuffix(p.client.baseURL, "/api"), album.ID)
	entity.AddAttribute("web_url", webURL)

	entity.AddSearchToken(album.AlbumName)
	if album.Description != "" {
		entity.AddSearchToken(album.Description)
	}

	return entity
}

// personToEntity converts an Immich person to an Entity.
func (p *Provider) personToEntity(person Person) types.Entity {
	entityID := p.BuildEntityID(person.ID).String()
	entityType := "person"

	displayName := person.Name
	if displayName == "" {
		displayName = "Unknown Person"
	}

	entity := types.NewEntity(entityID, entityType, p.Name(), displayName)
	entity.AddAttribute("is_hidden", person.IsHidden)

	if person.BirthDate != "" {
		entity.AddAttribute("birth_date", person.BirthDate)
	}

	// Build web URL for Immich person
	webURL := fmt.Sprintf("%s/people/%s", strings.TrimSuffix(p.client.baseURL, "/api"), person.ID)
	entity.AddAttribute("web_url", webURL)

	// Build thumbnail URL if person has a thumbnail
	if person.ThumbnailPath != "" {
		// Store original Immich URL for the proxy to use
		originalThumbnailURL := fmt.Sprintf("%s/api/people/%s/thumbnail", p.client.baseURL, person.ID)
		entity.AddAttribute("_immich_thumbnail_url", originalThumbnailURL)
		// Public thumbnail URL uses mifind proxy
		entity.AddAttribute("thumbnail_url", fmt.Sprintf("/api/thumbnail?id=%s", entityID))
	}

	entity.AddSearchToken(displayName)

	return entity
}

// GetThumbnail fetches a thumbnail for an Immich entity using the authenticated client.
// Implements the provider.ThumbnailProvider interface.
func (p *Provider) GetThumbnail(ctx context.Context, id string) ([]byte, string, error) {
	// Parse the entity ID to validate format
	// Format: immich:instanceID:resourceID
	parts := strings.Split(id, ":")
	if len(parts) != 3 {
		return nil, "", fmt.Errorf("invalid entity ID format: %s", id)
	}

	entity, err := p.Hydrate(ctx, id)
	if err != nil {
		return nil, "", fmt.Errorf("failed to hydrate entity: %w", err)
	}

	// Get the original Immich thumbnail URL from private attributes
	thumbnailURL, ok := entity.Attributes["_immich_thumbnail_url"].(string)
	if !ok || thumbnailURL == "" {
		return nil, "", fmt.Errorf("entity has no thumbnail URL")
	}

	// Use the authenticated Immich client to fetch the thumbnail
	req, err := p.client.newRequest(ctx, "GET", thumbnailURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}

	// Don't require JSON response for thumbnails
	req.Header.Del("Accept")

	resp, err := p.client.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch thumbnail: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	// Read the image data
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read response: %w", err)
	}

	// Get content type
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}

	return data, contentType, nil
}
