package search

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/yourname/mifind/internal/provider"
	"github.com/yourname/mifind/internal/types"
)

// Federator broadcasts search queries to multiple providers and aggregates results.
type Federator struct {
	manager *provider.Manager
	ranker  RankingStrategy
	logger  *zerolog.Logger
	timeout time.Duration
}

// NewFederator creates a new search federator.
func NewFederator(manager *provider.Manager, ranker RankingStrategy, logger *zerolog.Logger, timeout time.Duration) *Federator {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &Federator{
		manager: manager,
		ranker:  ranker,
		logger:  logger,
		timeout: timeout,
	}
}

// FederatedResult contains results from a single provider.
type FederatedResult struct {
	Provider   string
	Entities   []types.Entity
	Error      error
	Duration   time.Duration
	TypeCounts map[string]int
}

// FederatedResponse contains aggregated results from all providers.
type FederatedResponse struct {
	Results        []FederatedResult
	RankedEntities []RankedEntity
	TotalCount     int
	TypeCounts     map[string]int
	HasErrors      bool
	Duration       time.Duration
}

// Search broadcasts a search query to all providers and aggregates results.
func (f *Federator) Search(ctx context.Context, query SearchQuery) FederatedResponse {
	start := time.Now()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	// Get all provider names
	providerNames := f.manager.List()

	// If no providers, return empty response
	if len(providerNames) == 0 {
		return FederatedResponse{
			Results:        []FederatedResult{},
			RankedEntities: []RankedEntity{},
			TotalCount:     0,
			TypeCounts:     make(map[string]int),
			HasErrors:      false,
			Duration:       time.Since(start),
		}
	}

	// Search all providers concurrently
	results := make(chan FederatedResult, len(providerNames))
	var wg sync.WaitGroup

	for _, name := range providerNames {
		wg.Add(1)
		go func(providerName string) {
			defer wg.Done()

			result := f.searchProvider(ctx, providerName, query)
			results <- result
		}(name)
	}

	// Wait for all searches to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Aggregate results
	var allResults []FederatedResult
	var allEntities []EntityWithProvider
	totalCount := 0
	typeCounts := make(map[string]int)
	hasErrors := false

	for result := range results {
		allResults = append(allResults, result)
		totalCount += len(result.Entities)

		// Collect entities for ranking
		for _, entity := range result.Entities {
			allEntities = append(allEntities, EntityWithProvider{
				Entity:   entity,
				Provider: result.Provider,
			})
		}

		// Aggregate type counts
		for typeName, count := range result.TypeCounts {
			typeCounts[typeName] += count
		}

		if result.Error != nil {
			hasErrors = true
		}
	}

	// Rank entities using the configured ranking strategy
	rankedEntities := make([]RankedEntity, 0) // Default: empty
	if f.ranker != nil && len(allEntities) > 0 {
		ranked, err := f.ranker.Rank(ctx, allEntities, query)
		if err != nil {
			f.logger.Warn().Err(err).Msg("Ranking failed, returning unsorted results")
		} else {
			// Use the ranked results with scores preserved
			rankedEntities = ranked
		}
	}

	return FederatedResponse{
		Results:        allResults,
		RankedEntities: rankedEntities,
		TotalCount:     totalCount,
		TypeCounts:     typeCounts,
		HasErrors:      hasErrors,
		Duration:       time.Since(start),
	}
}

// searchProvider searches a single provider and returns the result.
func (f *Federator) searchProvider(ctx context.Context, providerName string, query SearchQuery) FederatedResult {
	start := time.Now()

	// Check if provider is connected
	if !f.manager.IsConnected(providerName) {
		return FederatedResult{
			Provider:   providerName,
			Entities:   []types.Entity{},
			Error:      fmt.Errorf("provider not connected"),
			Duration:   time.Since(start),
			TypeCounts: map[string]int{},
		}
	}

	// Get the provider
	prov, ok := f.manager.Get(providerName)
	if !ok {
		return FederatedResult{
			Provider:   providerName,
			Entities:   []types.Entity{},
			Error:      fmt.Errorf("provider not found"),
			Duration:   time.Since(start),
			TypeCounts: map[string]int{},
		}
	}

	// Get provider's filter capabilities to only send relevant filters
	capabilities, err := prov.FilterCapabilities(ctx)
	if err != nil {
		capabilities = make(map[string]provider.FilterCapability)
	}

	// Filter the query's filters to only include keys the provider supports
	filteredFilters := make(map[string]any)
	for key, value := range query.Filters {
		if _, supported := capabilities[key]; supported {
			filteredFilters[key] = value
		}
	}

	// If the provider doesn't support any of the query's filters, skip it entirely
	// This prevents irrelevant results when filtering by provider-specific attributes
	// Exception: always search if there's a text query and no filters are being applied
	if len(query.Filters) > 0 && len(filteredFilters) == 0 {
		f.logger.Debug().
			Str("provider", providerName).
			Str("query", query.Query).
			Strs("unsupported_filters", getFilterKeys(query.Filters)).
			Msg("Provider does not support any of the query filters, skipping provider")
		return FederatedResult{
			Provider:   providerName,
			Entities:   []types.Entity{},
			Error:      nil, // Not an error, just no results
			Duration:   time.Since(start),
			TypeCounts: map[string]int{},
		}
	}

	// Create provider query with filtered filters
	providerQuery := query.providerQuery()
	providerQuery.Filters = filteredFilters

	// Execute search
	entities, err := prov.Search(ctx, providerQuery)

	// Count by type
	typeCounts := make(map[string]int)
	for _, entity := range entities {
		typeCounts[entity.Type]++
	}

	return FederatedResult{
		Provider:   providerName,
		Entities:   entities,
		Error:      err,
		Duration:   time.Since(start),
		TypeCounts: typeCounts,
	}
}

// DiscoverAll runs discovery on all providers and aggregates results.
func (f *Federator) DiscoverAll(ctx context.Context) ([]types.Entity, error) {
	return f.manager.DiscoverAll(ctx)
}

// GetStatus returns the status of all providers.
func (f *Federator) GetStatus() []provider.ProviderStatus {
	return f.manager.Status()
}

// SetTimeout sets the timeout for federated searches.
func (f *Federator) SetTimeout(timeout time.Duration) {
	f.timeout = timeout
}

// SearchQuery wraps the provider SearchQuery with additional metadata.
type SearchQuery struct {
	// Query is the search string
	Query string

	// Filters specifies attribute filters
	Filters map[string]any

	// Type filters by entity type
	Type string

	// RelationshipType filters by relationship type
	RelationshipType string

	// Limit specifies max results per provider (0 = no limit)
	Limit int

	// Offset specifies results to skip
	Offset int

	// ProviderLimit specifies max results per provider (0 = use Limit)
	ProviderLimit int

	// TypeWeights specifies weights for each type (for ranking)
	TypeWeights map[string]float64

	// IncludeRelated specifies whether to include related entities
	IncludeRelated bool

	// MaxDepth specifies how deep to follow relationships
	MaxDepth int
}

// providerQuery converts the search query to a provider query.
func (q SearchQuery) providerQuery() provider.SearchQuery {
	limit := q.ProviderLimit
	if limit == 0 && q.Limit > 0 {
		// Distribute limit across providers evenly
		// This is a simple heuristic; could be improved
		limit = q.Limit
	}

	return provider.SearchQuery{
		Query:            q.Query,
		Filters:          q.Filters,
		Type:             q.Type,
		RelationshipType: q.RelationshipType,
		Limit:            limit,
		Offset:           q.Offset,
	}
}

// NewSearchQuery creates a new search query with default values.
func NewSearchQuery(query string) SearchQuery {
	return SearchQuery{
		Query:          query,
		Filters:        make(map[string]any),
		Limit:          0, // No limit by default
		Offset:         0,
		ProviderLimit:  0,
		TypeWeights:    make(map[string]float64),
		IncludeRelated: false,
		MaxDepth:       1,
	}
}

// WithFilter adds a filter to the query.
func (q SearchQuery) WithFilter(key string, value any) SearchQuery {
	q.Filters[key] = value
	return q
}

// WithType sets the type filter.
func (q SearchQuery) WithType(typeName string) SearchQuery {
	q.Type = typeName
	return q
}

// WithLimit sets the result limit.
func (q SearchQuery) WithLimit(limit int) SearchQuery {
	q.Limit = limit
	return q
}

// WithOffset sets the result offset.
func (q SearchQuery) WithOffset(offset int) SearchQuery {
	q.Offset = offset
	return q
}

// WithTypeWeight sets a weight for a specific type.
func (q SearchQuery) WithTypeWeight(typeName string, weight float64) SearchQuery {
	q.TypeWeights[typeName] = weight
	return q
}

// WithRelated enables including related entities.
func (q SearchQuery) WithRelated(enabled bool, maxDepth int) SearchQuery {
	q.IncludeRelated = enabled
	q.MaxDepth = maxDepth
	return q
}

// getFilterKeys returns the keys from a filters map for logging.
func getFilterKeys(filters map[string]any) []string {
	keys := make([]string, 0, len(filters))
	for k := range filters {
		keys = append(keys, k)
	}
	return keys
}
