package search

import (
	"fmt"
	"sort"
	"time"

	"github.com/yourname/mifind/internal/types"
)

// Ranker scores and orders search results from multiple providers.
type Ranker struct {
	// DefaultWeights are default weights for scoring factors
	DefaultWeights Weights
}

// Weights defines scoring factors for ranking.
type Weights struct {
	// TextRelevance is the weight for text match relevance
	TextRelevance float64

	// TypeBoost is the weight for type-based boosting
	TypeBoost float64

	// Recency is the weight for recency (newer items score higher)
	Recency float64

	// ProviderWeight weights different providers
	ProviderWeight map[string]float64

	// TypeWeight weights different types
	TypeWeight map[string]float64
}

// DefaultWeights returns default scoring weights.
func DefaultWeights() Weights {
	return Weights{
		TextRelevance:  1.0,
		TypeBoost:      0.5,
		Recency:        0.3,
		ProviderWeight: make(map[string]float64),
		TypeWeight:     make(map[string]float64),
	}
}

// NewRanker creates a new ranker with default weights.
func NewRanker() *Ranker {
	return &Ranker{
		DefaultWeights: DefaultWeights(),
	}
}

// RankedEntity is an entity with its ranking score.
type RankedEntity struct {
	Entity   types.Entity
	Score    float64
	Provider string
}

// RankedResult contains ranked search results.
type RankedResult struct {
	Entities   []RankedEntity
	TotalCount int
	TypeCounts map[string]int
	Duration   time.Duration
}

// Rank ranks and deduplicates entities from multiple providers.
func (r *Ranker) Rank(response FederatedResponse, query SearchQuery) RankedResult {
	start := time.Now()

	// Flatten all entities with their provider info
	var unranked []RankedEntity
	typeCounts := make(map[string]int)

	for _, result := range response.Results {
		for _, entity := range result.Entities {
			unranked = append(unranked, RankedEntity{
				Entity:   entity,
				Score:    0, // Will be calculated
				Provider: result.Provider,
			})
			typeCounts[entity.Type]++
		}
	}

	// Score all entities
	for i := range unranked {
		unranked[i].Score = r.scoreEntity(unranked[i], query)
	}

	// Deduplicate by entity ID
	deduped := r.deduplicate(unranked)

	// Sort by score (descending)
	sort.Slice(deduped, func(i, j int) bool {
		// Sort by score descending, then by timestamp descending
		if deduped[i].Score != deduped[j].Score {
			return deduped[i].Score > deduped[j].Score
		}
		return deduped[i].Entity.Timestamp.After(deduped[j].Entity.Timestamp)
	})

	// Apply pagination
	offset := query.Offset
	limit := query.Limit
	if limit <= 0 {
		limit = len(deduped)
	}

	// Handle out of bounds
	if offset >= len(deduped) {
		return RankedResult{
			Entities:   []RankedEntity{},
			TotalCount: len(deduped),
			TypeCounts: typeCounts,
			Duration:   time.Since(start),
		}
	}

	end := offset + limit
	if end > len(deduped) {
		end = len(deduped)
	}

	return RankedResult{
		Entities:   deduped[offset:end],
		TotalCount: len(deduped),
		TypeCounts: typeCounts,
		Duration:   time.Since(start),
	}
}

// scoreEntity calculates a relevance score for an entity.
func (r *Ranker) scoreEntity(ranked RankedEntity, query SearchQuery) float64 {
	entity := ranked.Entity
	score := 0.0

	// Text relevance score
	score += r.textRelevanceScore(entity, query.Query) * r.DefaultWeights.TextRelevance

	// Type boost
	if typeWeight, ok := r.DefaultWeights.TypeWeight[entity.Type]; ok {
		score += typeWeight * r.DefaultWeights.TypeBoost
	} else if queryTypeWeight, ok := query.TypeWeights[entity.Type]; ok {
		score += queryTypeWeight * r.DefaultWeights.TypeBoost
	}

	// Provider boost
	if providerWeight, ok := r.DefaultWeights.ProviderWeight[ranked.Provider]; ok {
		score += providerWeight
	}

	// Recency score (newer items score higher)
	score += r.recencyScore(entity.Timestamp) * r.DefaultWeights.Recency

	return score
}

// textRelevanceScore calculates a text relevance score.
func (r *Ranker) textRelevanceScore(entity types.Entity, query string) float64 {
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
func (r *Ranker) recencyScore(timestamp time.Time) float64 {
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
func (r *Ranker) deduplicate(entities []RankedEntity) []RankedEntity {
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
func (r *Ranker) SetTypeWeight(typeName string, weight float64) {
	r.DefaultWeights.TypeWeight[typeName] = weight
}

// SetProviderWeight sets a weight for a specific provider.
func (r *Ranker) SetProviderWeight(providerName string, weight float64) {
	r.DefaultWeights.ProviderWeight[providerName] = weight
}

// SetTextRelevanceWeight sets the text relevance weight.
func (r *Ranker) SetTextRelevanceWeight(weight float64) {
	r.DefaultWeights.TextRelevance = weight
}

// SetRecencyWeight sets the recency weight.
func (r *Ranker) SetRecencyWeight(weight float64) {
	r.DefaultWeights.Recency = weight
}

// Helper functions for string operations

func toLower(s string) string {
	if s == "" {
		return ""
	}
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

func contains(s, substr string) bool {
	return indexOf(s, substr) >= 0
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func indexOf(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	if len(s) < len(substr) {
		return -1
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func toString(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case int, int64, float64:
		return fmt.Sprintf("%v", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case time.Time:
		return val.Format(time.RFC3339)
	case []string:
		result := ""
		for _, s := range val {
			result += s + " "
		}
		return result
	case types.GPS:
		return fmt.Sprintf("%f,%f", val.Latitude, val.Longitude)
	default:
		return fmt.Sprintf("%v", val)
	}
}
