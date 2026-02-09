package search

import (
	"context"

	"github.com/yourname/mifind/internal/types"
)

// RankingStrategy defines the interface for ranking search results.
// Different implementations can provide various ranking algorithms
// (e.g., in-memory scoring, Meilisearch-based ranking, etc.)
type RankingStrategy interface {
	// Name returns the name of this ranking strategy
	Name() string

	// Rank scores and orders entities based on the search query.
	// Takes a slice of entities from multiple providers and returns
	// them ranked by relevance.
	Rank(ctx context.Context, entities []EntityWithProvider, query SearchQuery) ([]RankedEntity, error)
}

// EntityWithProvider wraps an entity with its source provider information.
type EntityWithProvider struct {
	Entity   types.Entity
	Provider string
}


// RankingConfig defines configuration for ranking strategies.
type RankingConfig struct {
	// Strategy specifies which ranking strategy to use
	Strategy string

	// ProviderWeights specifies weights for different providers
	ProviderWeights map[string]float64

	// TypeWeights specifies weights for different entity types
	TypeWeights map[string]float64

	// Meilisearch config for MeilisearchRanker
	Meilisearch MeilisearchConfig
}

// MeilisearchConfig contains Meilisearch-specific configuration.
type MeilisearchConfig struct {
	// URL is the Meilisearch server URL
	URL string

	// IndexUID is the index to use for storing entities
	IndexUID string

	// APIKey is the optional API key for authentication
	APIKey string

	// Cleanup config for index management
	Cleanup CleanupConfig
}

// CleanupConfig defines cleanup strategy for the Meilisearch index.
type CleanupConfig struct {
	// Enabled specifies if cleanup is enabled
	Enabled bool

	// Interval specifies how often to run cleanup (e.g., "24h")
	Interval string

	// MaxAge specifies the maximum age of entities to keep (e.g., "720h" = 30 days)
	// If specified, entities older than this are removed
	MaxAge string
}

// DefaultRankingConfig returns default ranking configuration.
func DefaultRankingConfig() RankingConfig {
	return RankingConfig{
		Strategy:        "in-memory",
		ProviderWeights: make(map[string]float64),
		TypeWeights:     make(map[string]float64),
		Meilisearch: MeilisearchConfig{
			URL:      "http://localhost:7700",
			IndexUID: "mifind_entities",
			APIKey:   "",
			Cleanup: CleanupConfig{
				Enabled: false,
				Interval: "24h",
				MaxAge:   "720h",
			},
		},
	}
}
