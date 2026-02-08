package types

import "time"

// Entity represents a lightweight item in the mifind system.
// It is a unified representation that can come from any provider.
type Entity struct {
	// ID is a stable, provider-scoped identifier
	ID string

	// Type is the hierarchical type name (e.g., "file.media.video", "media.asset.photo")
	Type string

	// Provider is the name of the source provider (e.g., "filesystem", "immich")
	Provider string

	// Title is the display name for this entity
	Title string

	// Description is an optional human-readable description
	Description string

	// Attributes contains typed values for filtering and display
	// Keys should use consistent naming across providers for common concepts
	Attributes map[string]any

	// Relationships contains optional connections to other entities
	Relationships []Relationship

	// SearchTokens contains flattened text for full-text search indexing
	SearchTokens []string

	// Timestamp is the cache/last-seen timestamp for this entity
	Timestamp time.Time
}

// Relationship represents a connection between two entities.
type Relationship struct {
	// Type is the relationship kind (e.g., "album", "artist", "folder", "parent")
	Type string

	// TargetID is the ID of the related entity
	TargetID string
}

// NewEntity creates a new Entity with the provided fields.
// It initializes maps and slices to avoid nil references.
func NewEntity(id, entityType, provider, title string) Entity {
	now := time.Now()
	return Entity{
		ID:            id,
		Type:          entityType,
		Provider:      provider,
		Title:         title,
		Attributes:    make(map[string]any),
		Relationships: make([]Relationship, 0),
		SearchTokens:  make([]string, 0),
		Timestamp:     now,
	}
}

// AddAttribute adds a key-value pair to the entity's attributes.
func (e *Entity) AddAttribute(key string, value any) {
	if e.Attributes == nil {
		e.Attributes = make(map[string]any)
	}
	e.Attributes[key] = value
}

// AddRelationship adds a relationship to the entity.
func (e *Entity) AddRelationship(relType, targetID string) {
	e.Relationships = append(e.Relationships, Relationship{
		Type:     relType,
		TargetID: targetID,
	})
}

// AddSearchToken adds a token to the entity's search tokens.
func (e *Entity) AddSearchToken(token string) {
	e.SearchTokens = append(e.SearchTokens, token)
}

// GetAttribute returns an attribute value by key, with a boolean indicating presence.
func (e *Entity) GetAttribute(key string) (any, bool) {
	if e.Attributes == nil {
		return nil, false
	}
	val, ok := e.Attributes[key]
	return val, ok
}

// GetAttributeString returns an attribute value as a string, if possible.
func (e *Entity) GetAttributeString(key string) (string, bool) {
	val, ok := e.GetAttribute(key)
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

// GetAttributeInt64 returns an attribute value as an int64, if possible.
func (e *Entity) GetAttributeInt64(key string) (int64, bool) {
	val, ok := e.GetAttribute(key)
	if !ok {
		return 0, false
	}
	switch v := val.(type) {
	case int:
		return int64(v), true
	case int64:
		return v, true
	case int32:
		return int64(v), true
	case float64:
		return int64(v), true
	default:
		return 0, false
	}
}

// GetAttributeFloat64 returns an attribute value as a float64, if possible.
func (e *Entity) GetAttributeFloat64(key string) (float64, bool) {
	val, ok := e.GetAttribute(key)
	if !ok {
		return 0, false
	}
	switch v := val.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		return 0, false
	}
}
