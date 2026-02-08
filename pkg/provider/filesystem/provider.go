package filesystem

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/yourname/mifind/internal/provider"
	"github.com/yourname/mifind/internal/types"
)

// Provider implements the provider interface for filesystem data.
// It connects to a filesystem-api service to search and browse files.
type Provider struct {
	provider.BaseProvider
	client *Client
	url    string
	apiKey string
}

// NewProvider creates a new filesystem provider.
func NewProvider() *Provider {
	return &Provider{
		BaseProvider: *provider.NewBaseProvider(provider.ProviderMetadata{
			Name:        "filesystem",
			Description: "Filesystem provider via filesystem-api",
			ConfigSchema: map[string]provider.ConfigField{
				"instance_id": {
					Type:        "string",
					Required:    true,
					Description: "Unique ID for this provider instance (e.g., 'myfs')",
				},
				"url": {
					Type:        "string",
					Required:    true,
					Description: "URL of the filesystem-api service",
				},
				"api_key": {
					Type:        "string",
					Required:    false,
					Description: "API key for authentication",
				},
			},
		}),
	}
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return "filesystem"
}

// Initialize sets up the filesystem provider with the given configuration.
func (p *Provider) Initialize(ctx context.Context, config map[string]any) error {
	// Get instance ID
	instanceID, ok := config["instance_id"].(string)
	if !ok || instanceID == "" {
		return provider.NewProviderError(provider.ErrorTypeConfig, "instance_id is required", nil)
	}
	// Set instance ID on BaseProvider so BuildEntityID works correctly
	p.SetInstanceID(instanceID)

	// Get URL
	url, ok := config["url"].(string)
	if !ok || url == "" {
		return provider.NewProviderError(provider.ErrorTypeConfig, "url is required", nil)
	}
	p.url = url

	var apiKey string
	if ak, ok := config["api_key"].(string); ok {
		apiKey = ak
	}
	p.apiKey = apiKey

	// Create HTTP client
	p.client = NewClient(p.url, p.apiKey)

	// Test connection
	health, err := p.client.Health(ctx)
	if err != nil {
		return provider.NewProviderError(provider.ErrorTypeAuth, "failed to connect to filesystem-api", err)
	}

	if health.Status != "ok" {
		return provider.NewProviderError(provider.ErrorTypeAuth, fmt.Sprintf("filesystem-api not healthy: %s", health.Status), nil)
	}

	return nil
}

// Discover performs a full discovery of all files.
// For filesystem provider, we return an empty slice since discovery
// should be done incrementally via the filesystem-api.
func (p *Provider) Discover(ctx context.Context) ([]types.Entity, error) {
	// Filesystem discovery is expensive - use DiscoverSince for incremental updates
	// Return empty slice for full discovery
	return []types.Entity{}, nil
}

// DiscoverSince performs incremental discovery since the given timestamp.
func (p *Provider) DiscoverSince(ctx context.Context, since time.Time) ([]types.Entity, error) {
	// Search for all files modified since the given time
	// This requires filesystem-api to support time-based filtering
	// For now, return empty
	return []types.Entity{}, nil
}

// Hydrate retrieves full details of a file by ID.
func (p *Provider) Hydrate(ctx context.Context, id string) (types.Entity, error) {
	// Extract file ID from entity ID
	entityID, err := provider.ParseEntityID(id)
	if err != nil {
		return types.Entity{}, provider.ErrNotFound
	}
	fileID := entityID.ResourceID()

	// Get file from API
	resp, err := p.client.GetFile(ctx, fileID)
	if err != nil {
		return types.Entity{}, provider.ErrNotFound
	}

	return p.fileToEntity(resp.File), nil
}

// GetRelated retrieves entities related to a file.
func (p *Provider) GetRelated(ctx context.Context, id string, relType string) ([]types.Entity, error) {
	entityID, err := provider.ParseEntityID(id)
	if err != nil {
		return nil, err
	}
	fileID := entityID.ResourceID()

	// Get the file to find related entities
	resp, err := p.client.GetFile(ctx, fileID)
	if err != nil {
		return nil, err
	}

	file := resp.File

	switch relType {
	case types.RelFolder, types.RelParent:
		// Return parent folder
		if file.Path == "" || file.Path == "/" {
			return []types.Entity{}, nil
		}

		parentPath := filepath.Dir(file.Path)
		if parentPath == file.Path {
			return []types.Entity{}, nil
		}

		// Browse parent directory
		browseResp, err := p.client.Browse(ctx, parentPath)
		if err != nil {
			return nil, err
		}

		var entities []types.Entity
		for _, f := range browseResp.Files {
			entities = append(entities, p.fileToEntity(f))
		}
		return entities, nil

	case types.RelChild:
		// Return children if this is a directory
		if !file.IsDir {
			return []types.Entity{}, nil
		}

		browseResp, err := p.client.Browse(ctx, file.Path)
		if err != nil {
			return nil, err
		}

		var entities []types.Entity
		for _, f := range browseResp.Files {
			entities = append(entities, p.fileToEntity(f))
		}
		return entities, nil

	case types.RelSibling:
		// Return siblings (files in same directory)
		parentPath := filepath.Dir(file.Path)
		if parentPath == "" || parentPath == file.Path {
			return []types.Entity{}, nil
		}

		browseResp, err := p.client.Browse(ctx, parentPath)
		if err != nil {
			return nil, err
		}

		var entities []types.Entity
		for _, f := range browseResp.Files {
			// Skip self
			if p.BuildEntityID(f.ID).String() != id {
				entities = append(entities, p.fileToEntity(f))
			}
		}
		return entities, nil

	default:
		// Unknown relationship type
		return []types.Entity{}, nil
	}
}

// Search performs a search query on the filesystem.
func (p *Provider) Search(ctx context.Context, query provider.SearchQuery) ([]types.Entity, error) {
	// Build search request
	searchReq := SearchRequest{
		Query:   query.Query,
		Filters: make(map[string]any),
		Limit:   query.Limit,
		Offset:  query.Offset,
	}

	// Add filters (but not type - that's handled internally)
	for k, v := range query.Filters {
		// Skip type filter as it's not filterable in Meilisearch
		// Type filtering will be handled on results after search
		if k != "type" {
			searchReq.Filters[k] = v
		}
	}

	// Note: query.Type is also excluded from Meilisearch filters
	// Type filtering is handled internally after getting results

	// Execute search
	result, err := p.client.Search(ctx, searchReq)
	if err != nil {
		return nil, err
	}

	// Convert files to entities
	entities := make([]types.Entity, 0, len(result.Files))
	for _, file := range result.Files {
		entity := p.fileToEntity(file)

		// Apply type filtering (handled internally since Meilisearch doesn't support it)
		typeFilter := query.Type
		if typeFilter == "" {
			// Also check filters map for type
			if v, ok := query.Filters["type"]; ok {
				typeFilter = v.(string)
			}
		}

		// Include entity if no type filter or if it matches
		if typeFilter == "" || typeMatches(entity.Type, typeFilter) {
			entities = append(entities, entity)
		}
	}

	return entities, nil
}

// SupportsIncremental returns true - filesystem provider supports incremental updates.
func (p *Provider) SupportsIncremental() bool {
	return true
}

// Shutdown shuts down the filesystem provider.
func (p *Provider) Shutdown(ctx context.Context) error {
	// No cleanup needed
	return nil
}

// FilterCapabilities returns the filter capabilities for the filesystem provider.
func (p *Provider) FilterCapabilities(ctx context.Context) (map[string]provider.FilterCapability, error) {
	return map[string]provider.FilterCapability{
		types.AttrPath: {
			Type:             types.AttributeTypeString,
			SupportsEq:       true,
			SupportsContains: true,
			SupportsGlob:     true,
			Description:      "File system path",
		},
		types.AttrExtension: {
			Type:             types.AttributeTypeString,
			SupportsEq:       true,
			SupportsNeq:      true,
			SupportsContains: false,
			Description:      "File extension (without dot)",
		},
		types.AttrMimeType: {
			Type:             types.AttributeTypeString,
			SupportsEq:       true,
			SupportsNeq:      true,
			SupportsContains: true,
			Description:      "MIME type",
		},
		types.AttrSize: {
			Type:          types.AttributeTypeInt64,
			SupportsEq:    true,
			SupportsNeq:   true,
			SupportsRange: true,
			Min:           ptrFloat64(0),
			Description:   "File size in bytes",
		},
		types.AttrModified: {
			Type:          types.AttributeTypeTime,
			SupportsEq:    true,
			SupportsRange: true,
			Description:   "Last modification timestamp (Unix)",
		},
		types.AttrCreated: {
			Type:          types.AttributeTypeTime,
			SupportsEq:    true,
			SupportsRange: true,
			Description:   "Creation timestamp (Unix)",
		},
	}, nil
}

// ptrFloat64 returns a pointer to a float64.
func ptrFloat64(v float64) *float64 {
	return &v
}

// fileToEntity converts a File to an Entity.
func (p *Provider) fileToEntity(file File) types.Entity {
	entityID := p.BuildEntityID(file.ID).String()
	entityType := FileTypeToMifindType(file.Extension, file.IsDir)

	entity := types.NewEntity(entityID, entityType, p.Name(), file.Name)

	// Set Timestamp to file's modified time instead of cache time
	if !file.Modified.IsZero() {
		entity.Timestamp = file.Modified
	}

	entity.AddAttribute(types.AttrPath, file.Path)

	// Only add size if it's valid (> 0)
	if file.Size > 0 {
		entity.AddAttribute(types.AttrSize, file.Size)
	}

	entity.AddAttribute(types.AttrExtension, strings.TrimPrefix(file.Extension, "."))
	entity.AddAttribute(types.AttrMimeType, file.MimeType)

	// Validate modified timestamp before converting
	modifiedTime := file.Modified
	if modifiedTime.Before(time.Unix(0, 1)) {
		// Clamp to minimum valid timestamp (1970-01-01 00:00:01 UTC)
		modifiedTime = time.Unix(0, 1)
	}
	entity.AddAttribute(types.AttrModified, modifiedTime.Unix())

	if !file.Created.IsZero() {
		entity.AddAttribute(types.AttrCreated, file.Created.Unix())
	}

	// Add directory-specific attributes
	if file.IsDir {
		entity.AddAttribute("is_dir", true)
	}

	// Add search tokens
	entity.AddSearchToken(file.Name)
	entity.AddSearchToken(file.Extension)
	entity.AddSearchToken(file.Path)
	entity.AddSearchToken(file.MimeType)

	// Add parent folder relationship
	if file.Path != "" && file.Path != "/" {
		parentPath := filepath.Dir(file.Path)
		if parentPath != file.Path {
			// Use parent path as relationship target ID
			// In a real implementation, we'd need to resolve this to an actual entity ID
			entity.AddRelationship(types.RelParent, "dir:"+parentPath)
		}
	}

	// Add metadata from the API response
	for k, v := range file.Metadata {
		entity.AddAttribute(k, v)
	}

	return entity
}

// typeMatches checks if an entity type matches a filter type.
// It supports exact match and prefix matching (e.g., "file" matches "file.media.image").
func typeMatches(entityType, filterType string) bool {
	if entityType == filterType {
		return true
	}
	// Prefix match: if filter is "file", it matches "file.media.image"
	if strings.HasPrefix(entityType, filterType+".") {
		return true
	}
	// Also check if filter is a parent type by checking if entity type starts with filter
	// This handles cases like filtering by "file" to get all file subtypes
	return strings.HasPrefix(entityType, filterType+".")
}
