package search

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/meilisearch/meilisearch-go"
	"github.com/rs/zerolog"
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
// It performs real-time indexing of entities before searching.
func (r *MeilisearchRanker) Rank(ctx context.Context, entities []EntityWithProvider, query SearchQuery) ([]RankedEntity, error) {
	start := time.Now()

	// Step 1: Index all entities into Meilisearch (real-time upsert)
	if err := r.indexEntities(ctx, entities); err != nil {
		r.logger.Warn().Err(err).Msg("Failed to index entities, falling back to basic scoring")
		return r.fallbackRanking(entities, query), nil
	}

	// Step 2: Build Meilisearch search request
	searchReq := r.buildSearchRequest(query)

	// Step 3: Execute search
	searchResp, err := r.client.Search(query.Query, searchReq)
	if err != nil {
		r.logger.Warn().Err(err).Msg("Meilisearch search failed, falling back to basic scoring")
		return r.fallbackRanking(entities, query), nil
	}

	// Step 4: Convert results back to RankedEntity
	ranked := r.convertSearchResults(searchResp, entities)

	r.logger.Debug().
		Int("input_entities", len(entities)).
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
	doc := map[string]any{
		"id":            entity.Entity.ID,
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
func (r *MeilisearchRanker) buildSearchRequest(query SearchQuery) *meilisearch.SearchRequest {
	req := &meilisearch.SearchRequest{
		Limit:  int64(query.Limit),
		Offset: int64(query.Offset),
	}

	// Add filters
	filterParts := []string{}

	// Type filter
	if query.Type != "" {
		filterParts = append(filterParts, fmt.Sprintf("type = \"%s\"", query.Type))
	}

	// Provider filter (from TypeWeights map - if only one provider has weight, filter to it)
	if len(query.TypeWeights) == 1 {
		for provider := range query.TypeWeights {
			filterParts = append(filterParts, fmt.Sprintf("provider = \"%s\"", provider))
		}
	}

	// Custom filters
	for key, val := range query.Filters {
		filterParts = append(filterParts, r.buildFilterPart(key, val))
	}

	if len(filterParts) > 0 {
		req.Filter = strings.Join(filterParts, " AND ")
	}

	return req
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
func (r *MeilisearchRanker) convertSearchResults(resp *meilisearch.SearchResponse, entities []EntityWithProvider) []RankedEntity {
	// Create a map of entity ID to EntityWithProvider for quick lookup
	entityMap := make(map[string]EntityWithProvider)
	for _, entity := range entities {
		entityMap[entity.Entity.ID] = entity
	}

	ranked := make([]RankedEntity, 0, len(resp.Hits))

	for _, hit := range resp.Hits {
		hitMap, ok := hit.(map[string]any)
		if !ok {
			continue
		}

		id, ok := hitMap["id"].(string)
		if !ok {
			continue
		}

		entity, ok := entityMap[id]
		if !ok {
			continue
		}

		// Calculate score based on Meilisearch ranking
		// Meilisearch doesn't return a score, so we use position as inverse score
		score := 1.0 // All results from Meilisearch are considered relevant

		ranked = append(ranked, RankedEntity{
			Entity:   entity.Entity,
			Score:    score,
			Provider: entity.Provider,
		})
	}

	return ranked
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
