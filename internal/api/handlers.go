package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"github.com/yourname/mifind/internal/provider"
	"github.com/yourname/mifind/internal/search"
	"github.com/yourname/mifind/internal/types"
)

// FilterValueCache caches filter values with TTL.
type FilterValueCache struct {
	values     map[string][]provider.FilterOption
	expiresAt  map[string]time.Time
	mu         sync.RWMutex
	ttl        time.Duration
}

// NewFilterValueCache creates a new filter value cache.
func NewFilterValueCache(ttl time.Duration) *FilterValueCache {
	return &FilterValueCache{
		values:    make(map[string][]provider.FilterOption),
		expiresAt: make(map[string]time.Time),
		ttl:       ttl,
	}
}

// Get retrieves cached values if not expired.
func (c *FilterValueCache) Get(key string) ([]provider.FilterOption, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	values, exists := c.values[key]
	if !exists {
		return nil, false
	}

 expires, ok := c.expiresAt[key]
	if !ok || time.Now().After(expires) {
		return nil, false
	}

	return values, true
}

// Set stores values in cache with expiration.
func (c *FilterValueCache) Set(key string, values []provider.FilterOption) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.values[key] = values
	c.expiresAt[key] = time.Now().Add(c.ttl)
}

// Handlers provides HTTP handlers for the mifind API.
type Handlers struct {
	manager       *provider.Manager
	federator     *search.Federator
	ranker        *search.Ranker
	filters       *search.Filters
	relationships *search.Relationships
	typeRegistry  *types.TypeRegistry
	logger        *zerolog.Logger
	filterCache   *FilterValueCache
}

// NewHandlers creates a new handlers instance.
func NewHandlers(
	manager *provider.Manager,
	federator *search.Federator,
	ranker *search.Ranker,
	filters *search.Filters,
	relationships *search.Relationships,
	typeRegistry *types.TypeRegistry,
	logger *zerolog.Logger,
) *Handlers {
	return &Handlers{
		manager:       manager,
		federator:     federator,
		ranker:        ranker,
		filters:       filters,
		relationships: relationships,
		typeRegistry:  typeRegistry,
		logger:        logger,
		filterCache:   NewFilterValueCache(24 * time.Hour), // 1-day cache
	}
}

// RegisterRoutes registers all API routes with the given router.
func (h *Handlers) RegisterRoutes(router *mux.Router) {
	// API subrouter for API endpoints
	apiRouter := router.PathPrefix("/api").Subrouter()

	// Search endpoints
	apiRouter.HandleFunc("/search", h.Search).Methods("POST")
	apiRouter.HandleFunc("/search/federated", h.SearchFederated).Methods("POST")

	// Entity endpoints
	apiRouter.HandleFunc("/entity/{id}", h.GetEntity).Methods("GET")
	apiRouter.HandleFunc("/entity/{id}/expand", h.ExpandEntity).Methods("GET")
	apiRouter.HandleFunc("/entity/{id}/related", h.GetRelated).Methods("GET")

	// Type endpoints
	apiRouter.HandleFunc("/types", h.ListTypes).Methods("GET")
	apiRouter.HandleFunc("/types/{name}", h.GetType).Methods("GET")

	// Filter endpoints
	apiRouter.HandleFunc("/filters", h.GetFilters).Methods("GET")

	// Provider endpoints
	apiRouter.HandleFunc("/providers", h.ListProviders).Methods("GET")
	apiRouter.HandleFunc("/providers/status", h.ProvidersStatus).Methods("GET")

	// Thumbnail proxy endpoint
	apiRouter.HandleFunc("/thumbnail", h.ProxyThumbnail).Methods("GET")

	// Health check
	apiRouter.HandleFunc("/health", h.Health).Methods("GET")

	// Serve static files (React UI) - must be last as it catches all routes
	// The Index handler is no longer needed at root since UI serves there
	router.PathPrefix("/").Handler(http.FileServer(SPAFileSystem()))
}

// SearchRequest represents a search request.
type SearchRequest struct {
	Query          string             `json:"query"`
	Filters        map[string]any     `json:"filters,omitempty"`
	Type           string             `json:"type,omitempty"`
	Limit          int                `json:"limit,omitempty"`
	Offset         int                `json:"offset,omitempty"`
	TypeWeights    map[string]float64 `json:"type_weights,omitempty"`
	IncludeRelated bool               `json:"include_related,omitempty"`
	MaxDepth       int                `json:"max_depth,omitempty"`
}

// SearchResponse represents a search response.
type SearchResponse struct {
	Entities     []EntityWithScore                   `json:"entities"`
	TotalCount   int                                 `json:"total_count"`
	TypeCounts   map[string]int                      `json:"type_counts"`
	Duration     float64                             `json:"duration_ms"`
	Filters      search.FilterResult                 `json:"filters,omitempty"`
	Capabilities map[string]provider.FilterCapability `json:"capabilities,omitempty"`
}

// EntityWithScore is an entity with its ranking score.
type EntityWithScore struct {
	types.Entity
	Score    float64 `json:"score,omitempty"`
	Provider string  `json:"provider,omitempty"`
}

// Search handles search requests.
func (h *Handlers) Search(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request: %v", err))
		return
	}

	// Build search query - don't apply limit/offset at provider level
	// We'll apply pagination after getting all results
	query := search.NewSearchQuery(req.Query)
	query.Filters = req.Filters
	query.Type = req.Type
	// Don't set query.Limit/Offset - we'll paginate after ranking
	query.TypeWeights = req.TypeWeights
	query.IncludeRelated = req.IncludeRelated
	query.MaxDepth = req.MaxDepth

	// Execute search (get all results from providers)
	response := h.federator.Search(r.Context(), query)

	// Use the ranked entities from the Federator (which now includes ranking with scores)
	result := search.RankedResult{
		Entities:   response.RankedEntities,
		TotalCount: len(response.RankedEntities),
		TypeCounts: response.TypeCounts,
		Duration:   response.Duration,
	}

	// Apply pagination after ranking
	limit := req.Limit
	if limit == 0 {
		limit = 24 // Default page size
	}
	offset := req.Offset

	// Calculate total from all ranked results
	totalCount := len(result.Entities)

	// Apply pagination slice
	pageStart := offset
	if pageStart > totalCount {
		pageStart = totalCount
	}
	pageEnd := pageStart + limit
	if pageEnd > totalCount {
		pageEnd = totalCount
	}

	paginatedEntities := result.Entities[pageStart:pageEnd]

	// Build response with paginated entities
	entities := make([]EntityWithScore, len(paginatedEntities))
	for i, ranked := range paginatedEntities {
		entities[i] = EntityWithScore{
			Entity:   ranked.Entity,
			Score:    ranked.Score,
			Provider: ranked.Provider,
		}
	}

	// Extract filters and capabilities for the search results
	// Get all entities for filter extraction (not just paginated)
	allEntities := make([]types.Entity, len(result.Entities))
	for i, ranked := range result.Entities {
		allEntities[i] = ranked.Entity
	}

	// Extract filters from search results
	filterResult := h.filters.ExtractFilters(allEntities, query.Type)

	// Get capabilities from providers that returned results
	capabilities := h.getProviderCapabilitiesForResults(r.Context(), response.Results)

	// Always include type filter capabilities from type registry
	// Include actual counts from search results
	h.addTypeFilterCapabilities(capabilities, result.TypeCounts)

	resp := SearchResponse{
		Entities:     entities,
		TotalCount:   totalCount, // Total count of all results (not just this page)
		TypeCounts:   result.TypeCounts,
		Duration:     float64(time.Since(start).Microseconds()) / 1000,
		Filters:      filterResult,
		Capabilities: capabilities,
	}

	h.writeJSON(w, http.StatusOK, resp)
}

// SearchFederatedResponse represents a federated search response.
type SearchFederatedResponse struct {
	Results    []ProviderResult `json:"results"`
	TotalCount int              `json:"total_count"`
	TypeCounts map[string]int   `json:"type_counts"`
	HasErrors  bool             `json:"has_errors"`
	Duration   float64          `json:"duration_ms"`
}

// ProviderResult represents results from a single provider.
type ProviderResult struct {
	Provider   string         `json:"provider"`
	Entities   []types.Entity `json:"entities"`
	Error      string         `json:"error,omitempty"`
	Duration   float64        `json:"duration_ms"`
	TypeCounts map[string]int `json:"type_counts"`
}

// SearchFederated handles federated search requests (returns per-provider results).
func (h *Handlers) SearchFederated(w http.ResponseWriter, r *http.Request) {
	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request: %v", err))
		return
	}

	// Build search query
	query := search.NewSearchQuery(req.Query)
	query.Filters = req.Filters
	query.Type = req.Type
	query.Limit = req.Limit
	query.Offset = req.Offset

	// Execute search
	response := h.federator.Search(r.Context(), query)

	// Build response
	results := make([]ProviderResult, len(response.Results))
	for i, result := range response.Results {
		errMsg := ""
		if result.Error != nil {
			errMsg = result.Error.Error()
		}

		results[i] = ProviderResult{
			Provider:   result.Provider,
			Entities:   result.Entities,
			Error:      errMsg,
			Duration:   float64(result.Duration.Microseconds()) / 1000,
			TypeCounts: result.TypeCounts,
		}
	}

	resp := SearchFederatedResponse{
		Results:    results,
		TotalCount: response.TotalCount,
		TypeCounts: response.TypeCounts,
		HasErrors:  response.HasErrors,
		Duration:   float64(response.Duration.Microseconds()) / 1000,
	}

	h.writeJSON(w, http.StatusOK, resp)
}

// GetEntity retrieves a single entity by ID.
func (h *Handlers) GetEntity(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	entity, err := h.manager.Hydrate(r.Context(), id)
	if err != nil {
		if err == provider.ErrNotFound {
			h.writeError(w, http.StatusNotFound, fmt.Sprintf("entity not found: %s", id))
			return
		}
		h.writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get entity: %v", err))
		return
	}

	h.writeJSON(w, http.StatusOK, entity)
}

// ExpandEntity retrieves an entity with its relationships expanded.
func (h *Handlers) ExpandEntity(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	// Get max depth from query params
	maxDepth := 1
	if depthStr := r.URL.Query().Get("depth"); depthStr != "" {
		if d, err := strconv.Atoi(depthStr); err == nil {
			maxDepth = d
		}
	}

	expanded, err := h.relationships.Expand(r.Context(), id, maxDepth)
	if err != nil {
		if err == provider.ErrNotFound {
			h.writeError(w, http.StatusNotFound, fmt.Sprintf("entity not found: %s", id))
			return
		}
		h.writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to expand entity: %v", err))
		return
	}

	h.writeJSON(w, http.StatusOK, expanded)
}

// GetRelated retrieves entities related to the given entity.
func (h *Handlers) GetRelated(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	relType := r.URL.Query().Get("type")

	limit := 0
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	entities, err := h.relationships.GetRelated(r.Context(), id, relType, limit)
	if err != nil {
		if err == provider.ErrNotFound {
			h.writeError(w, http.StatusNotFound, fmt.Sprintf("entity not found: %s", id))
			return
		}
		h.writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get related: %v", err))
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"entities": entities,
		"count":    len(entities),
	})
}

// ListTypes returns all registered types.
func (h *Handlers) ListTypes(w http.ResponseWriter, r *http.Request) {
	allTypes := h.typeRegistry.GetAll()

	// Convert to simple format
	typeList := make([]map[string]interface{}, 0, len(allTypes))
	for _, t := range allTypes {
		typeList = append(typeList, map[string]interface{}{
			"name":        t.Name,
			"parent":      t.Parent,
			"description": t.Description,
		})
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"types": typeList,
		"count": len(typeList),
	})
}

// GetType returns details for a specific type.
func (h *Handlers) GetType(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	typeDef := h.typeRegistry.Get(name)
	if typeDef == nil {
		h.writeError(w, http.StatusNotFound, fmt.Sprintf("type not found: %s", name))
		return
	}

	// Get ancestors
	ancestors := h.typeRegistry.GetAncestors(name)
	ancestorNames := make([]string, len(ancestors))
	for i, a := range ancestors {
		ancestorNames[i] = a.Name
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"name":        typeDef.Name,
		"parent":      typeDef.Parent,
		"ancestors":   ancestorNames,
		"description": typeDef.Description,
		"attributes":  typeDef.Attributes,
		"filters":     typeDef.Filters,
	})
}

// GetFilters returns available filters for a search query.
// It also returns provider filter capabilities and pre-obtained filter values.
// When a search query is provided, capabilities are dynamically filtered to only
// include providers that actually returned results.
func (h *Handlers) GetFilters(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("search")
	typeName := r.URL.Query().Get("type")

	// Execute search to get entities (if query is provided)
	var filterResult search.FilterResult
	var preObtainedValues map[string][]provider.FilterOption
	var capabilities map[string]provider.FilterCapability

	if query != "" || typeName != "" {
		searchQuery := search.NewSearchQuery(query)
		searchQuery.Limit = 100 // Limit for filter extraction
		searchQuery.Type = typeName

		response := h.federator.Search(r.Context(), searchQuery)
		result := h.ranker.Rank(response, searchQuery)

		// Extract entities from ranked results
		entities := make([]types.Entity, len(result.Entities))
		for i, ranked := range result.Entities {
			entities[i] = ranked.Entity
		}

		// Extract filters from search results (for result-based filters like extensions)
		filterResult = h.filters.ExtractFilters(entities, typeName)

		// Get capabilities only from providers that returned results
		capabilities = h.getProviderCapabilitiesForResults(r.Context(), response.Results)

		// IMPORTANT: Always include type filter capabilities from type registry
		// This ensures entity type filters (file, photo, video, etc.) are always available
		// even when providers that expose them are skipped due to unsupported filters
		h.addTypeFilterCapabilities(capabilities, filterResult.TypeCounts)

		// Fetch pre-obtained values for providers with results (e.g., Immich people, albums)
		// These are provider-wide filters, not result-based
		preObtainedValues = h.getPreObtainedFilterValues(r.Context(), capabilities)
	} else {
		// No search query - get all capabilities
		allCapabilities, err := h.manager.FilterCapabilities(r.Context())
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get filter capabilities: %v", err))
			return
		}
		capabilities = allCapabilities

		// Always include type filter capabilities from type registry
		// No type counts available when no search query
		h.addTypeFilterCapabilities(capabilities, nil)

		// Fetch pre-obtained filter values
		preObtainedValues = h.getPreObtainedFilterValues(r.Context(), capabilities)
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"capabilities": capabilities,
		"filters":      filterResult,
		"values":       preObtainedValues,
	})
}

// getProviderCapabilitiesForResults returns filter capabilities only from providers
// that actually returned results in the current search.
func (h *Handlers) getProviderCapabilitiesForResults(ctx context.Context, results []search.FederatedResult) map[string]provider.FilterCapability {
	capabilities := make(map[string]provider.FilterCapability)

	for _, result := range results {
		// Skip providers that had errors or returned no entities
		if result.Error != nil || len(result.Entities) == 0 {
			continue
		}

		prov, ok := h.manager.Get(result.Provider)
		if !ok {
			continue
		}

		providerCaps, err := prov.FilterCapabilities(ctx)
		if err != nil {
			h.logger.Warn().
				Str("provider", result.Provider).
				Err(err).
				Msg("Failed to get filter capabilities for provider")
			continue
		}

		// Merge this provider's capabilities
		for key, cap := range providerCaps {
			capabilities[key] = cap
		}
	}

	return capabilities
}

// addTypeFilterCapabilities ensures type filter capabilities are always available.
// The entity type filter is a core feature that should be visible regardless of
// which providers returned results (e.g., when filtering by person, filesystem
// is skipped but we still want to show "file" as a type option).
// The typeCounts parameter is used to populate actual counts for each type.
func (h *Handlers) addTypeFilterCapabilities(capabilities map[string]provider.FilterCapability, typeCounts map[string]int) {
	// Get all registered types from the type registry
	allTypes := h.typeRegistry.GetAll()

	// Build filter options from type registry with actual counts
	typeOptions := make([]provider.FilterOption, 0, len(allTypes))
	for _, t := range allTypes {
		count := 0
		if typeCounts != nil {
			count = typeCounts[t.Name]
		}
		typeOptions = append(typeOptions, provider.FilterOption{
			Value: t.Name,
			Label: t.Name,
			Count: count,
		})
	}

	// Always add the type filter capability with all registered types
	capabilities[types.AttrType] = provider.FilterCapability{
		Type:             types.AttributeTypeString,
		SupportsEq:       true,
		SupportsNeq:      true,
		SupportsContains: false,
		Options:          typeOptions,
		Description:      "Entity type",
	}
}

// getPreObtainedFilterValues fetches pre-obtained filter values from providers.
// Uses a 1-day in-memory cache to avoid repeated slow provider API calls.
// Cache misses are fetched asynchronously with a timeout to avoid blocking.
func (h *Handlers) getPreObtainedFilterValues(_ context.Context, capabilities map[string]provider.FilterCapability) map[string][]provider.FilterOption {
	result := make(map[string][]provider.FilterOption)

	// Get values for each filter that has capabilities
	for filterName := range capabilities {
		// Check cache first
		cachedValues, found := h.filterCache.Get(filterName)
		if found {
			result[filterName] = cachedValues
			continue
		}

		// Cache miss - fetch in background with timeout
		// Use a separate context to avoid blocking the HTTP response
		fetchCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)

		values, err := h.manager.GetFilterValues(fetchCtx, filterName)
		cancel() // Always cancel the context

		if err != nil {
			// Only log non-timeout errors to avoid noise
			if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
				h.logger.Warn().
					Str("filter", filterName).
					Err(err).
					Msg("Failed to get pre-obtained filter values")
			}
			continue
		}

		// Cache the values for 1 day
		if len(values) > 0 {
			h.filterCache.Set(filterName, values)
			result[filterName] = values
		}
	}

	return result
}

// ListProviders returns all registered providers.
func (h *Handlers) ListProviders(w http.ResponseWriter, r *http.Request) {
	// Get provider list from registry
	// TODO: Access registry through manager or dependency
	statuses := h.federator.GetStatus()

	providers := make([]map[string]interface{}, 0, len(statuses))
	for _, s := range statuses {
		providers = append(providers, map[string]interface{}{
			"name":                 s.Name,
			"connected":            s.Connected,
			"supports_incremental": s.SupportsIncremental,
		})
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"providers": providers,
		"count":     len(providers),
	})
}

// ProvidersStatus returns the status of all providers.
func (h *Handlers) ProvidersStatus(w http.ResponseWriter, r *http.Request) {
	statuses := h.federator.GetStatus()

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"providers": statuses,
		"count":     len(statuses),
	})
}

// Health returns the health status of the API.
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	statuses := h.federator.GetStatus()

	connected := 0
	for _, s := range statuses {
		if s.Connected {
			connected++
		}
	}

	health := map[string]interface{}{
		"status":    "ok",
		"providers": map[string]interface{}{"total": len(statuses), "connected": connected},
		"timestamp": time.Now().Unix(),
	}

	if connected == 0 && len(statuses) > 0 {
		health["status"] = "degraded"
	}

	h.writeJSON(w, http.StatusOK, health)
}

// Index returns the API index.
func (h *Handlers) Index(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"name":        "mifind API",
		"version":     "0.1.0",
		"description": "Unified personal search API",
		"endpoints": map[string]string{
			"/search":              "POST - Search across all providers",
			"/search/federated":    "POST - Search with per-provider results",
			"/entity/{id}":         "GET - Get entity by ID",
			"/entity/{id}/expand":  "GET - Get entity with relationships",
			"/entity/{id}/related": "GET - Get related entities",
			"/types":               "GET - List all types",
			"/types/{name}":        "GET - Get type details",
			"/filters":             "GET - Get available filters",
			"/providers":           "GET - List providers",
			"/providers/status":    "GET - Provider status",
			"/health":              "GET - Health check",
		},
	})
}

// ProxyThumbnail proxies a thumbnail image from a provider to avoid CORS issues.
// Supports two modes:
// - url=<direct_url>: Proxies the given URL directly (for compatibility)
// - id=<entity_id>: Looks up the entity and proxies its thumbnail from the provider
func (h *Handlers) ProxyThumbnail(w http.ResponseWriter, r *http.Request) {
	// Check for entity ID parameter first
	entityID := r.URL.Query().Get("id")
	if entityID != "" {
		h.proxyThumbnailByID(w, r, entityID)
		return
	}

	// Fall back to direct URL parameter for backward compatibility
	thumbnailURL := r.URL.Query().Get("url")
	if thumbnailURL == "" {
		h.writeError(w, http.StatusBadRequest, "missing id or url parameter")
		return
	}

	h.proxyThumbnailURL(w, r, thumbnailURL)
}

// proxyThumbnailByID looks up an entity and proxies its thumbnail.
func (h *Handlers) proxyThumbnailByID(w http.ResponseWriter, r *http.Request, entityID string) {
	// Parse entity ID to get provider name
	parts := strings.Split(entityID, ":")
	if len(parts) != 3 {
		h.writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid entity ID format: %s", entityID))
		return
	}
	providerName := parts[0]

	// Check if the provider implements ThumbnailProvider
	prov, ok := h.manager.Get(providerName)
	if !ok {
		h.writeError(w, http.StatusNotFound, fmt.Sprintf("provider not found: %s", providerName))
		return
	}

	if thumbnailProvider, ok := prov.(provider.ThumbnailProvider); ok {
		// Use the provider's authenticated thumbnail fetching
		data, contentType, err := thumbnailProvider.GetThumbnail(r.Context(), entityID)
		if err != nil {
			h.writeError(w, http.StatusBadGateway, fmt.Sprintf("failed to fetch thumbnail: %v", err))
			return
		}

		// Set headers and write response
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
		return
	}

	// Fall back to hydrating entity and getting thumbnail URL from attributes
	entity, err := h.manager.Hydrate(r.Context(), entityID)
	if err != nil {
		if err == provider.ErrNotFound {
			h.writeError(w, http.StatusNotFound, fmt.Sprintf("entity not found: %s", entityID))
			return
		}
		h.writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get entity: %v", err))
		return
	}

	// Get the original thumbnail URL from private attributes
	thumbnailURL := ""
	if url, ok := entity.Attributes["_immich_thumbnail_url"].(string); ok {
		thumbnailURL = url
	} else if url, ok := entity.Attributes["thumbnail_url"].(string); ok {
		// Fallback to public thumbnail_url if private attribute doesn't exist
		thumbnailURL = url
	}

	if thumbnailURL == "" {
		h.writeError(w, http.StatusNotFound, "entity has no thumbnail URL")
		return
	}

	h.proxyThumbnailURL(w, r, thumbnailURL)
}

// proxyThumbnailURL fetches and proxies a thumbnail from the given URL.
func (h *Handlers) proxyThumbnailURL(w http.ResponseWriter, r *http.Request, thumbnailURL string) {

	// Validate URL format
	_, err := url.Parse(thumbnailURL)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid url")
		return
	}

	// Check if we have any configured providers (basic validation)
	providers := h.manager.List()
	if len(providers) == 0 {
		h.writeError(w, http.StatusServiceUnavailable, "no providers configured")
		return
	}

	// Fetch the thumbnail from the provider
	client := &http.Client{
		Timeout: 30 * time.Second,
		// Don't follow redirects automatically to avoid security issues
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Get(thumbnailURL)
	if err != nil {
		h.writeError(w, http.StatusBadGateway, fmt.Sprintf("failed to fetch thumbnail: %v", err))
		return
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		h.writeError(w, http.StatusBadGateway, fmt.Sprintf("provider returned status %d", resp.StatusCode))
		return
	}

	// Copy headers from the provider response
	for key, values := range resp.Header {
		// Skip certain headers that shouldn't be proxied
		if key == "Content-Encoding" || key == "Transfer-Encoding" || key == "Connection" {
			continue
		}
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Add CORS headers since this is being proxied
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Copy the image data
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		h.logger.Error().Err(err).Msg("failed to copy thumbnail response")
	}
}

// writeJSON writes a JSON response.
func (h *Handlers) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error().Err(err).Msg("failed to encode JSON response")
	}
}

// writeError writes an error response.
func (h *Handlers) writeError(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, map[string]interface{}{
		"error": message,
	})
}
