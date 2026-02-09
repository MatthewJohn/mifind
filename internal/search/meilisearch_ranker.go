package search

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/meilisearch/meilisearch-go"
	"github.com/rs/zerolog"
	"github.com/yourname/mifind/internal/types"
	searchpkg "github.com/yourname/mifind/pkg/search"
)

// MeilisearchRanker provides Meilisearch-based ranking with real-time indexing.
// This ranker indexes entities on every search and uses Meilisearch's BM25 ranking.
type MeilisearchRanker struct {
	client *searchpkg.Client
	config RankingConfig
	logger *zerolog.Logger
}

// NewMeilisearchRanker creates a new Meilisearch-based ranker.
func NewMeilisearchRanker(config RankingConfig, logger *zerolog.Logger) (*MeilisearchRanker, error) {
	client, err := searchpkg.NewClient(
		config.Meilisearch.URL,
		config.Meilisearch.IndexUID,
		config.Meilisearch.APIKey,
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Meilisearch client: %w", err)
	}

	return &MeilisearchRanker{
		client: client,
		config: config,
		logger: logger,
	}, nil
}

// Name returns the name of this ranking strategy.
func (r *MeilisearchRanker) Name() string {
	return "meilisearch"
}

// Rank scores and orders entities using Meilisearch.
// Uses entity ID filtering to ensure only current search results are returned,
// allowing concurrent searches without clearing the index.
func (r *MeilisearchRanker) Rank(ctx context.Context, entities []EntityWithProvider, query SearchQuery) ([]RankedEntity, error) {
	start := time.Now()

	// Step 1: Apply custom attribute filters in Go (before indexing)
	filteredEntities := r.applyAttributeFilters(entities, query)

	if len(filteredEntities) == 0 {
		// No entities match the filters
		return []RankedEntity{}, nil
	}

	r.logger.Debug().
		Int("input_entities", len(entities)).
		Int("filtered_entities", len(filteredEntities)).
		Msg("Applied attribute filters before Meilisearch ranking")

	// Step 2: Index the filtered entities into Meilisearch (upsert, no clearing)
	if err := r.indexEntities(ctx, filteredEntities); err != nil {
		r.logger.Warn().Err(err).Msg("Failed to index entities, falling back to basic scoring")
		return r.fallbackRanking(filteredEntities, query), nil
	}

	// Step 3: Build Meilisearch search request with entity ID filter
	// This ensures we only get results from the current search, not stale data
	searchReq := r.buildSearchRequestWithIDs(query, filteredEntities)

	// Step 4: Execute search
	searchResp, err := r.client.Search(query.Query, searchReq)
	if err != nil {
		r.logger.Warn().Err(err).Msg("Meilisearch search failed, falling back to basic scoring")
		return r.fallbackRanking(filteredEntities, query), nil
	}

	// Step 5: Convert results back to RankedEntity
	ranked := r.convertSearchResults(searchResp, filteredEntities, query)

	r.logger.Debug().
		Int("input_entities", len(entities)).
		Int("filtered_entities", len(filteredEntities)).
		Int("meilisearch_hits", len(searchResp.Hits)).
		Int("ranked_results", len(ranked)).
		Dur("duration", time.Since(start)).
		Msg("Meilisearch ranking completed")

	return ranked, nil
}

// indexEntities performs real-time upsert of entities into Meilisearch.
func (r *MeilisearchRanker) indexEntities(ctx context.Context, entities []EntityWithProvider) error {
	if len(entities) == 0 {
		return nil
	}

	documents := make([]map[string]any, len(entities))
	for i, entity := range entities {
		documents[i] = r.entityToDocument(entity)
	}

	return r.client.UpdateDocuments(documents)
}

// entityToDocument converts an EntityWithProvider to a Meilisearch document.
func (r *MeilisearchRanker) entityToDocument(entity EntityWithProvider) map[string]any {
	// Transform entity ID to be Meilisearch-compatible
	// Meilisearch only allows alphanumeric, hyphens, and underscores
	// Replace colons with hyphens and other special chars with underscores
	meilisearchID := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, entity.Entity.ID)

	doc := map[string]any{
		"id":            meilisearchID,
		"original_id":   entity.Entity.ID, // Store original ID for lookup
		"title":         entity.Entity.Title,
		"description":   entity.Entity.Description,
		"type":          entity.Entity.Type,
		"provider":      entity.Provider,
		"timestamp":     entity.Entity.Timestamp.Unix(),
		"search_tokens": strings.Join(entity.Entity.SearchTokens, " "),
	}

	// Add attributes as a map
	if len(entity.Entity.Attributes) > 0 {
		// Flatten attributes for better searchability
		for key, val := range entity.Entity.Attributes {
			doc["attr_"+key] = val
		}
		doc["attributes"] = entity.Entity.Attributes
	}

	// Store provider's relevance score if available
	if providerScore, ok := entity.Entity.Attributes["_score"]; ok {
		if score, ok := providerScore.(float64); ok {
			doc["provider_score"] = score
		} else if scoreInt, ok := providerScore.(int); ok {
			doc["provider_score"] = float64(scoreInt)
		}
	}

	return doc
}

// buildSearchRequest constructs a Meilisearch search request from SearchQuery.
// NOTE: Only pushes basic filters (type, provider) to Meilisearch.
// Custom attribute filters are applied in Go after getting results,
// because provider-specific attributes (like person, album) are not
// pre-configured as filterable in Meilisearch.
func (r *MeilisearchRanker) buildSearchRequest(query SearchQuery) *meilisearch.SearchRequest {
	// Use a large limit to get all results (pagination is applied after ranking)
	limit := query.Limit
	if limit == 0 {
		limit = 10000 // Get all results, will paginate in Go
	}

	req := &meilisearch.SearchRequest{
		Limit:  int64(limit),
		Offset: int64(query.Offset),
	}

	// Only add basic filters that are configured as filterable in Meilisearch
	filterParts := []string{}

	// Type filter (always available)
	if query.Type != "" {
		filterParts = append(filterParts, fmt.Sprintf("type = \"%s\"", query.Type))
	}

	// Provider filter (always available)
	if len(query.TypeWeights) == 1 {
		for provider := range query.TypeWeights {
			filterParts = append(filterParts, fmt.Sprintf("provider = \"%s\"", provider))
		}
	}

	// Custom filters are NOT pushed to Meilisearch
	// They will be applied in Go code after getting results

	if len(filterParts) > 0 {
		req.Filter = strings.Join(filterParts, " AND ")
	}

	return req
}

// buildSearchRequestWithIDs constructs a Meilisearch search request that filters
// by entity IDs, ensuring only results from the current search are returned.
// This allows concurrent searches without clearing the index.
func (r *MeilisearchRanker) buildSearchRequestWithIDs(query SearchQuery, entities []EntityWithProvider) *meilisearch.SearchRequest {
	// Use a large limit to get all results (pagination is applied after ranking)
	limit := query.Limit
	if limit == 0 {
		limit = 10000 // Get all results, will paginate in Go
	}

	req := &meilisearch.SearchRequest{
		Limit:  int64(limit),
		Offset: int64(query.Offset),
	}

	// Build filter parts starting with entity ID filter
	filterParts := []string{}

	// Add entity ID filter to ensure we only get results from current search
	if len(entities) > 0 {
		idFilters := make([]string, 0, len(entities))
		for _, entity := range entities {
			// Transform entity ID to Meilisearch-compatible format
			meilisearchID := r.transformEntityID(entity.Entity.ID)
			idFilters = append(idFilters, fmt.Sprintf("id = \"%s\"", meilisearchID))
		}
		// Use OR for entity IDs (any of these IDs)
		if len(idFilters) > 0 {
			filterParts = append(filterParts, "("+strings.Join(idFilters, " OR ")+")")
		}
	}

	// Type filter (always available)
	if query.Type != "" {
		filterParts = append(filterParts, fmt.Sprintf("type = \"%s\"", query.Type))
	}

	// Provider filter (always available)
	if len(query.TypeWeights) == 1 {
		for provider := range query.TypeWeights {
			filterParts = append(filterParts, fmt.Sprintf("provider = \"%s\"", provider))
		}
	}

	if len(filterParts) > 0 {
		req.Filter = strings.Join(filterParts, " AND ")
	}

	return req
}

// transformEntityID converts an entity ID to Meilisearch-compatible format.
// Meilisearch only allows alphanumeric, hyphens, and underscores.
func (r *MeilisearchRanker) transformEntityID(id string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, id)
}

// buildFilterPart builds a single filter part for Meilisearch.
func (r *MeilisearchRanker) buildFilterPart(key string, value any) string {
	switch v := value.(type) {
	case string:
		return fmt.Sprintf("attr_%s = \"%s\"", key, v)
	case int, int64:
		return fmt.Sprintf("attr_%s = %v", key, v)
	case float64:
		return fmt.Sprintf("attr_%s = %f", key, v)
	case bool:
		return fmt.Sprintf("attr_%s = %t", key, v)
	case []string:
		if len(v) == 0 {
			return ""
		}
		if len(v) == 1 {
			return fmt.Sprintf("attr_%s = \"%s\"", key, v[0])
		}
		var orParts []string
		for _, item := range v {
			orParts = append(orParts, fmt.Sprintf("attr_%s = \"%s\"", key, item))
		}
		return "(" + strings.Join(orParts, " OR ") + ")"
	default:
		return ""
	}
}

// convertSearchResults converts Meilisearch search results to RankedEntity.
func (r *MeilisearchRanker) convertSearchResults(resp *meilisearch.SearchResponse, entities []EntityWithProvider, query SearchQuery) []RankedEntity {
	// Create a map of entity ID to EntityWithProvider for quick lookup
	entityMap := make(map[string]EntityWithProvider)
	for _, entity := range entities {
		entityMap[entity.Entity.ID] = entity
	}

	ranked := make([]RankedEntity, 0, len(resp.Hits))

	skippedMissingOriginalID := 0
	skippedNotFoundInMap := 0

	for _, hit := range resp.Hits {
		hitMap, ok := hit.(map[string]any)
		if !ok {
			continue
		}

		// Use original_id to look up the entity (the id field has transformed chars)
		originalID, ok := hitMap["original_id"].(string)
		if !ok {
			skippedMissingOriginalID++
			r.logger.Debug().Str("hit", fmt.Sprintf("%v", hit)).Msg("Missing original_id field in Meilisearch hit")
			continue
		}

		entity, ok := entityMap[originalID]
		if !ok {
			skippedNotFoundInMap++
			r.logger.Debug().Str("original_id", originalID).Msg("Entity not found in entityMap")
			continue
		}

		// Calculate score based on Meilisearch ranking position
		// Meilisearch returns results ranked by relevance, so we use position as inverse score
		score := 1.0 / float64(len(ranked)+1) // Earlier results get higher scores

		ranked = append(ranked, RankedEntity{
			Entity:   entity.Entity,
			Score:    score,
			Provider: entity.Provider,
		})
	}

	r.logger.Debug().
		Int("meilisearch_hits", len(resp.Hits)).
		Int("converted_entities", len(ranked)).
		Int("skipped_missing_original_id", skippedMissingOriginalID).
		Int("skipped_not_found_in_map", skippedNotFoundInMap).
		Msg("Converted Meilisearch results to RankedEntity")

	return ranked
}

// applyAttributeFilters applies custom attribute filters to entities in Go.
// This is done before indexing so Meilisearch only sees relevant entities.
func (r *MeilisearchRanker) applyAttributeFilters(entities []EntityWithProvider, query SearchQuery) []EntityWithProvider {
	if len(query.Filters) == 0 {
		return entities
	}

	filtered := make([]EntityWithProvider, 0, len(entities))

	for _, entity := range entities {
		if r.matchesFilters(entity.Entity, query.Filters) {
			filtered = append(filtered, entity)
		}
	}

	return filtered
}

// matchesFilters checks if an entity matches the given attribute filters.
func (r *MeilisearchRanker) matchesFilters(entity types.Entity, filters map[string]any) bool {
	for key, filterVal := range filters {
		entityVal, exists := entity.Attributes[key]
		if !exists {
			return false
		}

		// Simple equality check for most types
		if !r.matchValues(entityVal, filterVal) {
			return false
		}
	}

	return true
}

// matchValues checks if two values match.
func (r *MeilisearchRanker) matchValues(entityVal, filterVal any) bool {
	switch ev := entityVal.(type) {
	case string:
		if fv, ok := filterVal.(string); ok {
			return ev == fv
		}
		return false
	case int, int64:
		fvInt, ok := asInt64(filterVal)
		if !ok {
			return false
		}
		return ev == int(fvInt)
	case float64:
		fvFloat, ok := filterVal.(float64)
		if !ok {
			return false
		}
		return ev == fvFloat
	case bool:
		fvBool, ok := filterVal.(bool)
		if !ok {
			return false
		}
		return ev == fvBool
	case []string:
		fvSlice, ok := filterVal.([]string)
		if !ok {
			// Check if filter value is in the entity slice
			fvStr, ok := filterVal.(string)
			if !ok {
				return false
			}
			for _, item := range ev {
				if item == fvStr {
					return true
				}
			}
			return false
		}
		// Check if slices have any overlap
		for _, eItem := range ev {
			for _, fItem := range fvSlice {
				if eItem == fItem {
					return true
				}
			}
		}
		return false
	default:
		// Fallback to string comparison
		return fmt.Sprintf("%v", ev) == fmt.Sprintf("%v", filterVal)
	}
}

// asInt64 converts a value to int64.
func asInt64(v any) (int64, bool) {
	switch val := v.(type) {
	case int:
		return int64(val), true
	case int64:
		return val, true
	case float64:
		return int64(val), true
	default:
		return 0, false
	}
}

// fallbackRanking provides basic ranking when Meilisearch fails.
func (r *MeilisearchRanker) fallbackRanking(entities []EntityWithProvider, query SearchQuery) []RankedEntity {
	// Use in-memory ranking as fallback
	inMemoryRanker := NewInMemoryRanker(r.config)
	result, err := inMemoryRanker.Rank(context.Background(), entities, query)
	if err != nil {
		// Last resort: return entities in original order with neutral score
		result := make([]RankedEntity, len(entities))
		for i, entity := range entities {
			result[i] = RankedEntity{
				Entity:   entity.Entity,
				Score:    0.5,
				Provider: entity.Provider,
			}
		}
		return result
	}
	return result
}

// Cleanup performs index cleanup based on configuration.
func (r *MeilisearchRanker) Cleanup(ctx context.Context) error {
	if !r.config.Meilisearch.Cleanup.Enabled {
		return nil
	}

	r.logger.Info().Msg("Starting Meilisearch index cleanup")

	// For now, we'll do a full rebuild
	// In production, you might want to implement more sophisticated cleanup
	if err := r.client.DeleteAll(); err != nil {
		return fmt.Errorf("failed to clear index: %w", err)
	}

	r.logger.Info().Msg("Meilisearch index cleanup completed")
	return nil
}

// GetStats returns statistics about the Meilisearch index.
func (r *MeilisearchRanker) GetStats() (*meilisearch.StatsIndex, error) {
	return r.client.GetStats()
}
