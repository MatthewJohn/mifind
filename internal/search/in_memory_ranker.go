package search

import (
	"context"
	"sort"
	"time"

	"github.com/yourname/mifind/internal/types"
)

// InMemoryRanker provides in-memory scoring and ranking of entities.
// This is the fallback ranking strategy when Meilisearch is not available.
type InMemoryRanker struct {
	config RankingConfig
}

// NewInMemoryRanker creates a new in-memory ranker with the given config.
func NewInMemoryRanker(config RankingConfig) *InMemoryRanker {
	return &InMemoryRanker{
		config: config,
	}
}

// Name returns the name of this ranking strategy.
func (r *InMemoryRanker) Name() string {
	return "in-memory"
}

// Rank scores and orders entities based on the search query.
func (r *InMemoryRanker) Rank(ctx context.Context, entities []EntityWithProvider, query SearchQuery) ([]RankedEntity, error) {
	// Convert to RankedEntity and calculate scores
	ranked := make([]RankedEntity, len(entities))
	for i, entity := range entities {
		ranked[i] = RankedEntity{
			Entity:   entity.Entity,
			Score:    r.scoreEntity(entity, query),
			Provider: entity.Provider,
		}
	}

	// Deduplicate by entity ID (keep highest score)
	deduped := r.deduplicate(ranked)

	// Sort by score (descending), then by timestamp
	sort.Slice(deduped, func(i, j int) bool {
		if deduped[i].Score != deduped[j].Score {
			return deduped[i].Score > deduped[j].Score
		}
		return deduped[i].Entity.Timestamp.After(deduped[j].Entity.Timestamp)
	})

	return deduped, nil
}

// scoreEntity calculates a relevance score for an entity.
func (r *InMemoryRanker) scoreEntity(entity EntityWithProvider, query SearchQuery) float64 {
	e := entity.Entity
	score := 0.0

	// Text relevance score (weight: 1.0)
	score += r.textRelevanceScore(e, query.Query)

	// Type boost (weight: 0.5)
	if typeWeight, ok := r.config.TypeWeights[e.Type]; ok {
		score += typeWeight * 0.5
	} else if queryTypeWeight, ok := query.TypeWeights[e.Type]; ok {
		score += queryTypeWeight * 0.5
	}

	// Provider boost
	if providerWeight, ok := r.config.ProviderWeights[entity.Provider]; ok {
		score += providerWeight
	}

	// Recency score (weight: 0.3)
	score += r.recencyScore(e.Timestamp) * 0.3

	return score
}

// textRelevanceScore calculates a text relevance score.
func (r *InMemoryRanker) textRelevanceScore(entity types.Entity, query string) float64 {
	if query == "" {
		return 0.5 // Neutral score for empty queries
	}

	score := 0.0
	queryLower := toLower(query)

	// Check title match
	titleLower := toLower(entity.Title)
	if contains(titleLower, queryLower) {
		if titleLower == queryLower {
			score += 2.0 // Exact match
		} else if startsWith(titleLower, queryLower) {
			score += 1.5 // Prefix match
		} else {
			score += 1.0 // Contains match
		}
	}

	// Check description match
	if entity.Description != "" {
		descLower := toLower(entity.Description)
		if contains(descLower, queryLower) {
			score += 0.5
		}
	}

	// Check search tokens
	for _, token := range entity.SearchTokens {
		tokenLower := toLower(token)
		if contains(tokenLower, queryLower) {
			score += 0.3
		}
	}

	// Check attributes
	for key, val := range entity.Attributes {
		valStr := toString(val)
		if contains(toLower(key), queryLower) || contains(toLower(valStr), queryLower) {
			score += 0.2
		}
	}

	// Normalize score
	if score > 3.0 {
		score = 3.0
	}

	return score / 3.0
}

// recencyScore calculates a recency score (0-1) based on timestamp.
func (r *InMemoryRanker) recencyScore(timestamp time.Time) float64 {
	if timestamp.IsZero() {
		return 0.0
	}

	age := time.Since(timestamp)
	if age < 0 {
		age = -age // Handle future timestamps
	}

	// Decay over time: 1.0 for very recent, 0.0 for very old
	// Using exponential decay with half-life of 30 days
	const halfLife = 30 * 24 * time.Hour
	decay := float64(age) / float64(halfLife)
	score := 1.0 / (1.0 + decay)

	return score
}

// deduplicate removes duplicate entities based on ID.
// Keeps the entity with the highest score.
func (r *InMemoryRanker) deduplicate(entities []RankedEntity) []RankedEntity {
	seen := make(map[string]*RankedEntity)

	for i := range entities {
		id := entities[i].Entity.ID
		if existing, ok := seen[id]; ok {
			// Keep the one with higher score
			if entities[i].Score > existing.Score {
				seen[id] = &entities[i]
			}
		} else {
			seen[id] = &entities[i]
		}
	}

	// Convert map back to slice
	result := make([]RankedEntity, 0, len(seen))
	for _, entity := range seen {
		result = append(result, *entity)
	}

	return result
}

// SetTypeWeight sets a weight for a specific entity type.
func (r *InMemoryRanker) SetTypeWeight(typeName string, weight float64) {
	if r.config.TypeWeights == nil {
		r.config.TypeWeights = make(map[string]float64)
	}
	r.config.TypeWeights[typeName] = weight
}

// SetProviderWeight sets a weight for a specific provider.
func (r *InMemoryRanker) SetProviderWeight(providerName string, weight float64) {
	if r.config.ProviderWeights == nil {
		r.config.ProviderWeights = make(map[string]float64)
	}
	r.config.ProviderWeights[providerName] = weight
}
