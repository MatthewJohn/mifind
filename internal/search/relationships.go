package search

import (
	"context"

	"github.com/rs/zerolog"
	"github.com/yourname/mifind/internal/provider"
	"github.com/yourname/mifind/internal/types"
)

// Relationships handles relationship traversal and expansion.
type Relationships struct {
	manager *provider.Manager
	logger  *zerolog.Logger
}

// NewRelationships creates a new relationships handler.
func NewRelationships(manager *provider.Manager, logger *zerolog.Logger) *Relationships {
	return &Relationships{
		manager: manager,
		logger:  logger,
	}
}

// RelatedResult contains related entities with metadata.
type RelatedResult struct {
	// EntityType is the type of relationship
	EntityType string

	// Entities are the related entities
	Entities []types.Entity

	// Direction indicates if these are incoming or outgoing relationships
	Direction types.RelationshipDirection
}

// GetRelated retrieves entities related to the given entity ID.
func (r *Relationships) GetRelated(ctx context.Context, id string, relType string, limit int) ([]types.Entity, error) {
	return r.manager.GetRelated(ctx, id, relType)
}

// Expand retrieves an entity with all its relationships populated.
func (r *Relationships) Expand(ctx context.Context, id string, maxDepth int) (*ExpandedEntity, error) {
	// Get the source entity
	entity, err := r.manager.Hydrate(ctx, id)
	if err != nil {
		return nil, err
	}

	expanded := &ExpandedEntity{
		Entity:        entity,
		Related:       make(map[string][]types.Entity),
		Relationships: make(map[string][]types.Relationship),
	}

	// Populate relationships
	if maxDepth > 0 {
		for _, rel := range entity.Relationships {
			// Get related entities
			related, err := r.manager.GetRelated(ctx, rel.TargetID, rel.Type)
			if err != nil {
				r.logger.Warn().
					Str("id", id).
					Str("rel_type", rel.Type).
					Str("target_id", rel.TargetID).
					Err(err).
					Msg("Failed to get related entity")
				continue
			}

			// Group by relationship type
			if expanded.Related[rel.Type] == nil {
				expanded.Related[rel.Type] = make([]types.Entity, 0)
			}
			expanded.Related[rel.Type] = append(expanded.Related[rel.Type], related...)

			// Store relationship metadata
			if expanded.Relationships[rel.Type] == nil {
				expanded.Relationships[rel.Type] = make([]types.Relationship, 0)
			}
			expanded.Relationships[rel.Type] = append(expanded.Relationships[rel.Type], rel)
		}
	}

	return expanded, nil
}

// ExpandMultiple expands multiple entities in parallel.
func (r *Relationships) ExpandMultiple(ctx context.Context, ids []string, maxDepth int) ([]*ExpandedEntity, error) {
	results := make([]*ExpandedEntity, len(ids))

	for i, id := range ids {
		expanded, err := r.Expand(ctx, id, maxDepth)
		if err != nil {
			r.logger.Warn().
				Str("id", id).
				Err(err).
				Msg("Failed to expand entity")
			results[i] = nil
		} else {
			results[i] = expanded
		}
	}

	return results, nil
}

// GroupByType groups related entities by their relationship type.
func (r *Relationships) GroupByType(related []types.Entity, relationships []types.Relationship) map[string][]types.Entity {
	grouped := make(map[string][]types.Entity)

	for i, entity := range related {
		var relType string
		if i < len(relationships) {
			relType = relationships[i].Type
		}

		if grouped[relType] == nil {
			grouped[relType] = make([]types.Entity, 0)
		}
		grouped[relType] = append(grouped[relType], entity)
	}

	return grouped
}

// FindPath finds a path of relationships between two entities.
// Returns a slice of entity IDs representing the path.
func (r *Relationships) FindPath(ctx context.Context, fromID, toID string, maxDepth int) ([]string, error) {
	visited := make(map[string]bool)
	path := make([]string, 0)

	return r.findPathDFS(ctx, fromID, toID, maxDepth, visited, path)
}

// findPathDFS performs depth-first search to find a path.
func (r *Relationships) findPathDFS(ctx context.Context, currentID, targetID string, depth int, visited map[string]bool, path []string) ([]string, error) {
	// Mark current as visited
	visited[currentID] = true
	path = append(path, currentID)

	// Found the target
	if currentID == targetID {
		return path, nil
	}

	// Reached max depth
	if depth <= 0 {
		return nil, nil
	}

	// Get the current entity
	entity, err := r.manager.Hydrate(ctx, currentID)
	if err != nil {
		return nil, err
	}

	// Explore relationships
	for _, rel := range entity.Relationships {
		targetID := rel.TargetID

		// Skip if already visited
		if visited[targetID] {
			continue
		}

		// Recursively search
		result, err := r.findPathDFS(ctx, targetID, targetID, depth-1, visited, path)
		if err != nil {
			continue
		}
		if result != nil {
			return result, nil
		}
	}

	// Backtrack
	path = path[:len(path)-1]
	return nil, nil
}

// ExpandByType expands entities and groups by relationship type.
func (r *Relationships) ExpandByType(ctx context.Context, id string, maxDepth int) (map[string][]types.Entity, error) {
	expanded, err := r.Expand(ctx, id, maxDepth)
	if err != nil {
		return nil, err
	}

	return expanded.Related, nil
}

// FilterByRelationship filters entities by their relationship type.
func (r *Relationships) FilterByRelationship(entities []types.Entity, relType string) []types.Entity {
	var filtered []types.Entity

	for _, entity := range entities {
		for _, rel := range entity.Relationships {
			if rel.Type == relType {
				filtered = append(filtered, entity)
				break
			}
		}
	}

	return filtered
}

// GetRelationshipTypes returns all unique relationship types in a set of entities.
func (r *Relationships) GetRelationshipTypes(entities []types.Entity) []string {
	typeMap := make(map[string]bool)

	for _, entity := range entities {
		for _, rel := range entity.Relationships {
			typeMap[rel.Type] = true
		}
	}

	types := make([]string, 0, len(typeMap))
	for t := range typeMap {
		types = append(types, t)
	}

	return types
}

// CountRelationships counts relationships by type for an entity.
func (r *Relationships) CountRelationships(entity types.Entity) map[string]int {
	counts := make(map[string]int)

	for _, rel := range entity.Relationships {
		counts[rel.Type]++
	}

	return counts
}

// ExpandEntity is a convenience method to expand a single entity with depth 1.
func (r *Relationships) ExpandEntity(ctx context.Context, id string) (*ExpandedEntity, error) {
	return r.Expand(ctx, id, 1)
}

// ExpandedEntity represents an entity with its related entities populated.
type ExpandedEntity struct {
	// Entity is the source entity
	Entity types.Entity

	// Related groups related entities by relationship type
	Related map[string][]types.Entity

	// Relationships stores the relationship metadata for each type
	Relationships map[string][]types.Relationship
}

// GetRelatedByType returns related entities of a specific type.
func (e *ExpandedEntity) GetRelatedByType(relType string) []types.Entity {
	if e.Related == nil {
		return []types.Entity{}
	}
	return e.Related[relType]
}

// HasRelated checks if the entity has related entities of a specific type.
func (e *ExpandedEntity) HasRelated(relType string) bool {
	if e.Related == nil {
		return false
	}
	_, exists := e.Related[relType]
	return exists && len(e.Related[relType]) > 0
}

// TotalRelatedCount returns the total number of related entities.
func (e *ExpandedEntity) TotalRelatedCount() int {
	if e.Related == nil {
		return 0
	}

	count := 0
	for _, entities := range e.Related {
		count += len(entities)
	}
	return count
}

// RelationshipCount returns the count of relationships by type.
func (e *ExpandedEntity) RelationshipCount() map[string]int {
	counts := make(map[string]int)

	for relType, entities := range e.Related {
		counts[relType] = len(entities)
	}

	return counts
}

// SearchByRelationship searches for entities that have a specific relationship type.
func (r *Relationships) SearchByRelationship(ctx context.Context, query provider.SearchQuery, relType string) ([]types.Entity, error) {
	// Search all providers
	providerResults := r.manager.SearchAll(ctx, query)

	// Flatten results
	var allEntities []types.Entity
	for _, entities := range providerResults {
		allEntities = append(allEntities, entities...)
	}

	// Filter by relationship type
	return r.FilterByRelationship(allEntities, relType), nil
}
