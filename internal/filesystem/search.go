package filesystem

import (
	"context"
	"fmt"
	"time"

	"github.com/meilisearch/meilisearch-go"
	"github.com/rs/zerolog"
)

// SearchRequest represents a search request.
type SearchRequest struct {
	Query   string         `json:"query"`
	Filters map[string]any `json:"filters,omitempty"`
	Limit   int            `json:"limit,omitempty"`
	Offset  int            `json:"offset,omitempty"`
}

// SearchResult represents the search results.
type SearchResult struct {
	Files      []File         `json:"files"`
	TotalCount int            `json:"total_count"`
	Query      string         `json:"query"`
	Filters    map[string]any `json:"filters,omitempty"`
}

// Search handles search queries against Meilisearch.
type Search struct {
	indexer *Indexer
	logger  *zerolog.Logger
}

// NewSearch creates a new search handler.
func NewSearch(indexer *Indexer, logger *zerolog.Logger) *Search {
	return &Search{
		indexer: indexer,
		logger:  logger,
	}
}

// Search executes a search query.
func (s *Search) Search(ctx context.Context, req SearchRequest) (*SearchResult, error) {
	// Build Meilisearch search request
	searchReq := &meilisearch.SearchRequest{
		Query:  req.Query,
		Limit:  int64(req.Limit),
		Offset: int64(req.Offset),
	}

	// Add filters if provided
	if len(req.Filters) > 0 {
		filterStr := s.buildFilterString(req.Filters)
		if filterStr != "" {
			searchReq.Filter = filterStr
		}
	}

	// Execute search
	searchResp, err := s.indexer.index.Search(req.Query, searchReq)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Convert results to File objects
	files := make([]File, 0, len(searchResp.Hits))
	for _, hit := range searchResp.Hits {
		// Type assert hit to map[string]interface{}
		hitMap, ok := hit.(map[string]interface{})
		if !ok {
			s.logger.Warn().Msg("Failed to convert search hit to file - not a map")
			continue
		}
		file, err := s.hitToFile(hitMap)
		if err != nil {
			s.logger.Warn().Err(err).Msg("Failed to convert search hit to file")
			continue
		}
		files = append(files, file)
	}

	return &SearchResult{
		Files:      files,
		TotalCount: int(searchResp.EstimatedTotalHits),
		Query:      req.Query,
		Filters:    req.Filters,
	}, nil
}

// buildFilterString converts filter map to Meilisearch filter syntax.
func (s *Search) buildFilterString(filters map[string]any) string {
	var parts []string

	for key, value := range filters {
		part := s.buildFilterPart(key, value)
		if part != "" {
			parts = append(parts, part)
		}
	}

	if len(parts) == 0 {
		return ""
	}

	// Combine with AND
	return joinWithAnd(parts)
}

// buildFilterPart builds a single filter part.
func (s *Search) buildFilterPart(key string, value any) string {
	switch v := value.(type) {
	case string:
		return fmt.Sprintf("%s = \"%s\"", key, v)
	case int, int64, float64:
		return fmt.Sprintf("%s = %v", key, v)
	case bool:
		return fmt.Sprintf("%s = %t", key, v)
	case []string:
		if len(v) == 0 {
			return ""
		}
		if len(v) == 1 {
			return fmt.Sprintf("%s = \"%s\"", key, v[0])
		}
		// Multiple values: use OR
		var orParts []string
		for _, item := range v {
			orParts = append(orParts, fmt.Sprintf("%s = \"%s\"", key, item))
		}
		return "(" + joinWithOr(orParts) + ")"
	case []any:
		if len(v) == 0 {
			return ""
		}
		if len(v) == 1 {
			return s.buildFilterPart(key, v[0])
		}
		// Multiple values: use OR
		var orParts []string
		for _, item := range v {
			orParts = append(orParts, s.buildFilterPart(key, item))
		}
		return "(" + joinWithOr(orParts) + ")"
	default:
		return ""
	}
}

// hitToFile converts a Meilisearch hit to a File.
func (s *Search) hitToFile(hit map[string]any) (File, error) {
	// Extract values from hit
	id, _ := hit["id"].(string)
	path, _ := hit["path"].(string)
	name, _ := hit["name"].(string)
	extension, _ := hit["extension"].(string)
	mimeType, _ := hit["mime_type"].(string)
	size, _ := hit["size"].(int64)
	modified, _ := hit["modified"].(int64)
	isDir, _ := hit["is_dir"].(bool)

	return File{
		ID:        id,
		Path:      path,
		Name:      name,
		Extension: extension,
		MimeType:  mimeType,
		Size:      size,
		Modified:  parseUnixTime(modified),
		IsDir:     isDir,
	}, nil
}

// parseUnixTime converts a unix timestamp to time.Time.
func parseUnixTime(timestamp int64) time.Time {
	if timestamp == 0 {
		return time.Time{}
	}
	return time.Unix(timestamp, 0)
}

// joinWithAnd joins filter parts with AND.
func joinWithAnd(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}

	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += " AND " + parts[i]
	}
	return result
}

// joinWithOr joins filter parts with OR.
func joinWithOr(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}

	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += " OR " + parts[i]
	}
	return result
}

// GetFilterSuggestions returns suggested filter values based on current index.
func (s *Search) GetFilterSuggestions(ctx context.Context, field string, limit int) ([]string, error) {
	// Use Meilisearch faceted search
	searchReq := &meilisearch.SearchRequest{
		Query:  "",
		Limit:  0,
		Facets: []string{field},
	}

	searchResp, err := s.indexer.index.Search("", searchReq)
	if err != nil {
		return nil, fmt.Errorf("facet search failed: %w", err)
	}

	// Extract facet values
	var values []string
	if searchResp.FacetDistribution != nil {
		// Type assert to map[string]map[string]int
		if facetDistribution, ok := searchResp.FacetDistribution.(map[string]map[string]int); ok {
			if fieldFacets, ok := facetDistribution[field]; ok {
				for value := range fieldFacets {
					values = append(values, value)
					if limit > 0 && len(values) >= limit {
						break
					}
				}
			}
		}
	}

	return values, nil
}
