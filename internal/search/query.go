package search

import (
	"fmt"

	"github.com/yourname/mifind/internal/provider"
	"github.com/yourname/mifind/internal/search/filters"
	"github.com/yourname/mifind/internal/types"
)

// TypedSearchQuery is a strongly-typed search query with validated filters.
// This replaces the untyped Filters map[string]any with map[string]filters.FilterValue.
type TypedSearchQuery struct {
	// Query is the search string
	Query string

	// TypedFilters specifies validated attribute filters with type-safe values
	TypedFilters map[string]filters.FilterValue

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

// NewTypedSearchQuery creates a new typed search query with default values.
func NewTypedSearchQuery(query string) *TypedSearchQuery {
	return &TypedSearchQuery{
		Query:        query,
		TypedFilters: make(map[string]filters.FilterValue),
	}
}

// WithFilter adds a typed filter to the query.
func (q *TypedSearchQuery) WithFilter(name string, filter filters.FilterValue) *TypedSearchQuery {
	if q.TypedFilters == nil {
		q.TypedFilters = make(map[string]filters.FilterValue)
	}
	q.TypedFilters[name] = filter
	return q
}

// Validate validates the query against the type registry.
// Returns an error if:
// - The type is not registered
// - Any filter has an invalid value
// - Any filter operation is not supported
func (q *TypedSearchQuery) Validate(registry *types.TypeRegistry) error {
	// Validate type if specified
	if q.Type != "" {
		if _, err := registry.ParseType(q.Type); err != nil {
			return fmt.Errorf("invalid type: %w", err)
		}
	}

	// Validate all filters
	allAttrs := registry.GetAllAttributes()
	for filterName, filterValue := range q.TypedFilters {
		attrDef, exists := allAttrs[filterName]
		if !exists {
			return &filters.ValidationError{
				FilterName: filterName,
				Reason:     fmt.Sprintf("unknown attribute: %q", filterName),
			}
		}

		if !attrDef.Filterable {
			return &filters.ValidationError{
				FilterName: filterName,
				Reason:     fmt.Sprintf("attribute %q is not filterable", filterName),
			}
		}

		if err := filterValue.Validate(attrDef); err != nil {
			return err
		}
	}

	return nil
}

// ToProviderQuery converts this typed query to the legacy provider.SearchQuery format.
// This maintains backward compatibility with existing providers.
func (q *TypedSearchQuery) ToProviderQuery() provider.SearchQuery {
	// Convert typed filters to legacy format
	legacyFilters := make(map[string]any)
	for name, filter := range q.TypedFilters {
		legacyFilters[name] = filter.Value()
	}

	limit := q.ProviderLimit
	if limit == 0 && q.Limit > 0 {
		limit = q.Limit
	}

	return provider.SearchQuery{
		Query:            q.Query,
		Filters:          legacyFilters,
		Type:             q.Type,
		RelationshipType: q.RelationshipType,
		Limit:            limit,
		Offset:           q.Offset,
	}
}

// ToSearchQuery converts this typed query to the search.SearchQuery format.
// This maintains backward compatibility with the existing search package.
func (q *TypedSearchQuery) ToSearchQuery() SearchQuery {
	// Convert typed filters to legacy format
	legacyFilters := make(map[string]any)
	for name, filter := range q.TypedFilters {
		legacyFilters[name] = filter.Value()
	}

	return SearchQuery{
		Query:            q.Query,
		Filters:          legacyFilters,
		Type:             q.Type,
		RelationshipType: q.RelationshipType,
		Limit:            q.Limit,
		Offset:           q.Offset,
		ProviderLimit:    q.ProviderLimit,
		TypeWeights:      q.TypeWeights,
		IncludeRelated:   q.IncludeRelated,
		MaxDepth:         q.MaxDepth,
	}
}

// ParseAndValidate creates a TypedSearchQuery from raw filter data and validates it.
// This is a convenience function for the common case of parsing HTTP request data.
func ParseAndValidate(queryStr string, filterData map[string]any, registry *types.TypeRegistry) (*TypedSearchQuery, error) {
	// Create parser
	parser := filters.NewParser(registry)

	// Parse filters
	typedFilters, err := parser.ParseFilters(filterData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse filters: %w", err)
	}

	// Create typed query
	q := &TypedSearchQuery{
		Query:        queryStr,
		TypedFilters: typedFilters,
	}

	// Validate
	if err := q.Validate(registry); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return q, nil
}
