package mock

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yourname/mifind/internal/provider"
	"github.com/yourname/mifind/internal/types"
)

// MockProvider is a test provider that returns predefined entities.
// Useful for development, testing, and demonstrations.
type MockProvider struct {
	provider.BaseProvider
	entities    map[string]types.Entity
	mu          sync.RWMutex
	initialized bool
}

// NewMockProvider creates a new mock provider.
func NewMockProvider() *MockProvider {
	return &MockProvider{
		BaseProvider: *provider.NewBaseProvider(provider.ProviderMetadata{
			Name:        "mock",
			Description: "Mock provider for testing",
			ConfigSchema: map[string]provider.ConfigField{
				"entity_count": {
					Type:        "int",
					Required:    false,
					Description: "Number of mock entities to generate",
					Default:     10,
				},
			},
		}),
		entities: make(map[string]types.Entity),
	}
}

// factory creates a new mock provider (used for registration).
func factory() provider.Provider {
	return NewMockProvider()
}

// Name returns the provider name.
func (m *MockProvider) Name() string {
	return "mock"
}

// Initialize sets up the mock provider with optional configuration.
func (m *MockProvider) Initialize(ctx context.Context, config map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entityCount := 10
	if count, ok := config["entity_count"].(int); ok {
		entityCount = count
	}

	// Generate mock entities
	for i := 0; i < entityCount; i++ {
		id := fmt.Sprintf("mock:entity:%d", i)
		entity := m.generateMockEntity(id, i)
		m.entities[id] = entity
	}

	m.initialized = true
	return nil
}

// Discover returns all mock entities.
func (m *MockProvider) Discover(ctx context.Context) ([]types.Entity, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.initialized {
		return nil, provider.ErrNotConfigured
	}

	entities := make([]types.Entity, 0, len(m.entities))
	for _, entity := range m.entities {
		entities = append(entities, entity)
	}

	return entities, nil
}

// DiscoverSince returns entities "modified" since the given time.
// For mock provider, this returns entities with index > since timestamp.
func (m *MockProvider) DiscoverSince(ctx context.Context, since time.Time) ([]types.Entity, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.initialized {
		return nil, provider.ErrNotConfigured
	}

	entities := make([]types.Entity, 0)
	for _, entity := range m.entities {
		if entity.Timestamp.After(since) {
			entities = append(entities, entity)
		}
	}

	return entities, nil
}

// Hydrate returns the full entity by ID.
func (m *MockProvider) Hydrate(ctx context.Context, id string) (types.Entity, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entity, exists := m.entities[id]
	if !exists {
		return types.Entity{}, provider.ErrNotFound
	}

	return entity, nil
}

// GetRelated returns related entities.
// For mock provider, returns entities with similar mock relationships.
func (m *MockProvider) GetRelated(ctx context.Context, id string, relType string) ([]types.Entity, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entity, exists := m.entities[id]
	if !exists {
		return nil, provider.ErrNotFound
	}

	var related []types.Entity
	for _, rel := range entity.Relationships {
		if relType != "" && rel.Type != relType {
			continue
		}
		if relEntity, ok := m.entities[rel.TargetID]; ok {
			related = append(related, relEntity)
		}
	}

	return related, nil
}

// Search performs a search over mock entities.
func (m *MockProvider) Search(ctx context.Context, query provider.SearchQuery) ([]types.Entity, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []types.Entity

	for _, entity := range m.entities {
		// Type filter
		if query.Type != "" && entity.Type != query.Type {
			continue
		}

		// Text search (simple contains match)
		if query.Query != "" {
			matched := false
			// Search in title
			if contains(query.Query, entity.Title) {
				matched = true
			}
			// Search in description
			if contains(query.Query, entity.Description) {
				matched = true
			}
			// Search in attributes
			for key, val := range entity.Attributes {
				valStr := fmt.Sprintf("%v", val)
				if contains(query.Query, key) || contains(query.Query, valStr) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		// Apply filters
		if !m.matchesFilters(entity, query.Filters) {
			continue
		}

		results = append(results, entity)
	}

	// Apply pagination
	if query.Offset >= len(results) {
		return []types.Entity{}, nil
	}

	end := len(results)
	if query.Limit > 0 && query.Offset+query.Limit < end {
		end = query.Offset + query.Limit
	}

	return results[query.Offset:end], nil
}

// SupportsIncremental returns true - mock provider supports incremental updates.
func (m *MockProvider) SupportsIncremental() bool {
	return true
}

// Shutdown is a no-op for mock provider.
func (m *MockProvider) Shutdown(ctx context.Context) error {
	return nil
}

// AddEntity adds a custom entity to the mock provider.
func (m *MockProvider) AddEntity(entity types.Entity) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.entities[entity.ID] = entity
}

// Clear removes all entities from the mock provider.
func (m *MockProvider) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.entities = make(map[string]types.Entity)
}

// generateMockEntity creates a mock entity for testing.
func (m *MockProvider) generateMockEntity(id string, index int) types.Entity {
	now := time.Now()

	// Create different entity types based on index
	entityType := "item"
	title := fmt.Sprintf("Mock Item %d", index)
	description := fmt.Sprintf("This is mock entity number %d", index)

	switch index % 5 {
	case 0:
		entityType = "file.document"
		title = fmt.Sprintf("Document%d.txt", index)
		description = "A text document"
	case 1:
		entityType = "file.media.video"
		title = fmt.Sprintf("Video%d.mp4", index)
		description = "A video file"
	case 2:
		entityType = "file.media.image"
		title = fmt.Sprintf("Photo%d.jpg", index)
		description = "An image file"
	case 3:
		entityType = "media.asset.photo"
		title = fmt.Sprintf("Photo %d", index)
		description = "A photo asset"
	case 4:
		entityType = "media.asset.video"
		title = fmt.Sprintf("Video %d", index)
		description = "A video asset"
	}

	entity := types.Entity{
		ID:          id,
		Type:        entityType,
		Provider:    "mock",
		Title:       title,
		Description: description,
		Attributes:  make(map[string]any),
		Relationships: []types.Relationship{
			{
				Type:     types.RelRelatedTo,
				TargetID: fmt.Sprintf("mock:entity:%d", (index+1)%10),
			},
		},
		SearchTokens: []string{title, description, entityType},
		Timestamp:    now.Add(time.Duration(index) * time.Second),
	}

	// Add some attributes based on type
	if index%5 == 0 {
		entity.Attributes[types.AttrPath] = fmt.Sprintf("/docs/Document%d.txt", index)
		entity.Attributes[types.AttrSize] = int64(1024 * (index + 1))
		entity.Attributes[types.AttrExtension] = "txt"
	} else if index%5 == 1 || index%5 == 4 {
		entity.Attributes[types.AttrDuration] = int64(60 + index*10)
		entity.Attributes[types.AttrWidth] = 1920
		entity.Attributes[types.AttrHeight] = 1080
	} else if index%5 == 2 || index%5 == 3 {
		entity.Attributes[types.AttrWidth] = 1920
		entity.Attributes[types.AttrHeight] = 1080
		entity.Attributes[types.AttrCamera] = "Mock Camera"
		entity.Attributes[types.AttrGPS] = types.GPS{
			Latitude:  37.7749 + float64(index)*0.01,
			Longitude: -122.4194 + float64(index)*0.01,
		}
	}

	return entity
}

// matchesFilters checks if an entity matches the given filters.
func (m *MockProvider) matchesFilters(entity types.Entity, filters map[string]any) bool {
	for key, filterVal := range filters {
		entityVal, exists := entity.Attributes[key]
		if !exists {
			return false
		}

		// Simple equality check
		// In a real implementation, this would handle different filter types
		if fmt.Sprintf("%v", filterVal) != fmt.Sprintf("%v", entityVal) {
			return false
		}
	}

	return true
}

// contains checks if a string contains a substring (case-insensitive).
func contains(s, substr string) bool {
	return containsIgnoreCase(s, substr)
}

// containsIgnoreCase performs a case-insensitive substring check.
func containsIgnoreCase(s, substr string) bool {
	s = toLower(s)
	substr = toLower(substr)
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		indexOf(s, substr) >= 0)
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
