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

// mapKeys extracts keys from a map with string keys for logging/debugging.
// Uses generics to work with any map[string]V type.
func mapKeys[M ~map[string]V, V any](m M) []string {
	if m == nil {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// MeilisearchRanker provides Meilisearch-based ranking with real-time indexing.
// This ranker indexes entities on every search and uses Meilisearch's BM25 ranking.
type MeilisearchRanker struct {
	client       *searchpkg.Client
	config       RankingConfig
	logger       *zerolog.Logger
	typeRegistry *types.TypeRegistry
}

// NewMeilisearchRanker creates a new Meilisearch-based ranker.
func NewMeilisearchRanker(config RankingConfig, logger *zerolog.Logger, typeRegistry *types.TypeRegistry) (*MeilisearchRanker, error) {
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
		client:       client,
		config:       config,
		logger:       logger,
		typeRegistry: typeRegistry,
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

	// Log input
	r.logger.Debug().
		Str("query", query.Query).
		Str("type", query.Type).
		Int("input_entities", len(entities)).
		Strs("filters", mapKeys(query.Filters)).
		Msg("Meilisearch Rank: starting")

	// Step 2: Index the filtered entities into Meilisearch (upsert, no clearing)
	if err := r.indexEntities(ctx, entities); err != nil {
		r.logger.Warn().Err(err).Msg("Failed to index entities, falling back to basic scoring")
		return r.fallbackRanking(entities, query), nil
	}

	// Step 3: Build Meilisearch search request with entity ID filter
	// This ensures we only get results from the current search, not stale data
	searchReq := r.buildSearchRequestWithIDs(query, entities)

	// Step 4: Execute search
	// When provider-level filters are active (person, album, location), use wildcard search
	// to match all entities since providers have already done the filtering
	// @TODO Remove this as we should NOT filter out those that don't match - is this really useful?
	searchQuery := query.Query
	if r.hasProviderLevelFilters(query.Filters) {
		// Use wildcard to match all when provider filters are active
		// Providers have already filtered by person/album/location, so we should return all results
		searchQuery = "*"
		r.logger.Debug().Msg("Provider-level filters active, using wildcard search")
	}

	searchResp, err := r.client.Search(searchQuery, searchReq)
	if err != nil {
		r.logger.Warn().Err(err).Msg("Meilisearch search failed, falling back to basic scoring")
		return r.fallbackRanking(entities, query), nil
	}

	// Step 5: Convert results back to RankedEntity
	ranked := r.convertSearchResults(searchResp, entities, query)

	r.logger.Debug().
		Int("input_entities", len(entities)).
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

	r.logger.Debug().
		Int("count", len(entities)).
		Strs("sample_ids", getSampleIDs(entities, 3)).
		Msg("Indexing entities into Meilisearch")

	documents := make([]map[string]any, len(entities))
	for i, entity := range entities {
		documents[i] = r.entityToDocument(entity)
	}

	err := r.client.UpdateDocuments(documents)
	if err != nil {
		r.logger.Error().Err(err).Msg("Failed to update documents in Meilisearch")
		return err
	}

	r.logger.Debug().
		Int("count", len(documents)).
		Msg("Successfully indexed entities into Meilisearch")

	return nil
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

		// Log the ID filter for debugging
		r.logger.Debug().
			Int("num_entities", len(entities)).
			Int("num_id_filters", len(idFilters)).
			Strs("sample_ids", getSampleIDs(entities, 5)).
			Msg("Built entity ID filter for Meilisearch")
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

		r.logger.Debug().
			Str("filter", req.Filter.(string)).
			Msg("Meilisearch search request filter")
	}

	return req
}

// getSampleIDs returns a sample of entity IDs for logging.
func getSampleIDs(entities []EntityWithProvider, count int) []string {
	if len(entities) == 0 {
		return []string{}
	}
	sampleSize := count
	if len(entities) < sampleSize {
		sampleSize = len(entities)
	}
	ids := make([]string, sampleSize)
	for i := 0; i < sampleSize; i++ {
		ids[i] = entities[i].Entity.ID
	}
	return ids
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

// convertSearchResults converts Meilisearch search results to RankedEntity.
// IMPORTANT: Ranking should NEVER filter - if Meilisearch doesn't return an entity,
// it should still be included at the bottom of the ranking (with low score).
// This ensures all entities from providers are always returned.
func (r *MeilisearchRanker) convertSearchResults(resp *meilisearch.SearchResponse, entities []EntityWithProvider, query SearchQuery) []RankedEntity {
	// Create a map of entity ID to EntityWithProvider for quick lookup
	entityMap := make(map[string]EntityWithProvider)
	for _, entity := range entities {
		entityMap[entity.Entity.ID] = entity
	}

	// Track which entities were returned by Meilisearch
	returnedIDs := make(map[string]bool)

	ranked := make([]RankedEntity, 0, len(entities))

	skippedMissingOriginalID := 0
	skippedNotFoundInMap := 0

	// Process Meilisearch hits in order (they come pre-ranked by relevance)
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

		returnedIDs[originalID] = true

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

	// IMPORTANT: Re-add any entities that Meilisearch didn't return
	// This ensures ranking NEVER filters - it only orders results
	// Entities not returned by Meilisearch get a low score (near bottom)
	meilisearchMissed := 0
	baseScore := 1.0 / float64(len(ranked)+1) // Score just below the lowest Meilisearch result
	for _, entity := range entities {
		if !returnedIDs[entity.Entity.ID] {
			meilisearchMissed++
			// Add with a slightly lower score than the lowest ranked result
			// Use a small decrement to maintain order while placing at bottom
			fallbackScore := baseScore * 0.9
			ranked = append(ranked, RankedEntity{
				Entity:   entity.Entity,
				Score:    fallbackScore,
				Provider: entity.Provider,
			})
		}
	}

	r.logger.Debug().
		Int("meilisearch_hits", len(resp.Hits)).
		Int("converted_entities", len(ranked)).
		Int("skipped_missing_original_id", skippedMissingOriginalID).
		Int("skipped_not_found_in_map", skippedNotFoundInMap).
		Int("meilisearch_missed_readded", meilisearchMissed).
		Msg("Converted Meilisearch results to RankedEntity (all entities preserved)")

	return ranked
}

// hasProviderLevelFilters checks if the query contains provider-level filters
// that are handled by providers (marked with ProviderLevel in attribute metadata).
func (r *MeilisearchRanker) hasProviderLevelFilters(filters map[string]any) bool {
	// Get all attribute definitions to check which are provider-level
	allAttrs := r.typeRegistry.GetAllAttributes()

	for key := range filters {
		if attrDef, exists := allAttrs[key]; exists {
			if attrDef.Filter.ProviderLevel {
				return true
			}
		}
	}
	return false
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
