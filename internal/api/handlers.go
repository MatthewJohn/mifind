package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"github.com/yourname/mifind/internal/provider"
	"github.com/yourname/mifind/internal/search"
	"github.com/yourname/mifind/internal/types"
)

// Handlers provides HTTP handlers for the mifind API.
type Handlers struct {
	manager       *provider.Manager
	federator     *search.Federator
	ranker        *search.Ranker
	filters       *search.Filters
	relationships *search.Relationships
	typeRegistry  *types.TypeRegistry
	logger        *zerolog.Logger
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
	}
}

// RegisterRoutes registers all API routes with the given router.
func (h *Handlers) RegisterRoutes(router *mux.Router) {
	// Search endpoints
	router.HandleFunc("/search", h.Search).Methods("POST")
	router.HandleFunc("/search/federated", h.SearchFederated).Methods("POST")

	// Entity endpoints
	router.HandleFunc("/entity/{id}", h.GetEntity).Methods("GET")
	router.HandleFunc("/entity/{id}/expand", h.ExpandEntity).Methods("GET")
	router.HandleFunc("/entity/{id}/related", h.GetRelated).Methods("GET")

	// Type endpoints
	router.HandleFunc("/types", h.ListTypes).Methods("GET")
	router.HandleFunc("/types/{name}", h.GetType).Methods("GET")

	// Filter endpoints
	router.HandleFunc("/filters", h.GetFilters).Methods("GET")

	// Provider endpoints
	router.HandleFunc("/providers", h.ListProviders).Methods("GET")
	router.HandleFunc("/providers/status", h.ProvidersStatus).Methods("GET")

	// Health check
	router.HandleFunc("/health", h.Health).Methods("GET")
	router.HandleFunc("/", h.Index).Methods("GET")
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
	Entities   []EntityWithScore `json:"entities"`
	TotalCount int               `json:"total_count"`
	TypeCounts map[string]int    `json:"type_counts"`
	Duration   float64           `json:"duration_ms"`
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

	// Build search query
	query := search.NewSearchQuery(req.Query)
	query.Filters = req.Filters
	query.Type = req.Type
	query.Limit = req.Limit
	query.Offset = req.Offset
	query.TypeWeights = req.TypeWeights
	query.IncludeRelated = req.IncludeRelated
	query.MaxDepth = req.MaxDepth

	// Execute search
	response := h.federator.Search(r.Context(), query)
	result := h.ranker.Rank(response, query)

	// Build response
	entities := make([]EntityWithScore, len(result.Entities))
	for i, ranked := range result.Entities {
		entities[i] = EntityWithScore{
			Entity:   ranked.Entity,
			Score:    ranked.Score,
			Provider: ranked.Provider,
		}
	}

	resp := SearchResponse{
		Entities:   entities,
		TotalCount: result.TotalCount,
		TypeCounts: result.TypeCounts,
		Duration:   float64(time.Since(start).Microseconds()) / 1000,
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
func (h *Handlers) GetFilters(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("search")
	typeName := r.URL.Query().Get("type")

	// Execute search to get entities
	searchQuery := search.NewSearchQuery(query)
	searchQuery.Limit = 100 // Limit for filter extraction

	response := h.federator.Search(r.Context(), searchQuery)
	result := h.ranker.Rank(response, searchQuery)

	// Extract entities from ranked results
	entities := make([]types.Entity, len(result.Entities))
	for i, ranked := range result.Entities {
		entities[i] = ranked.Entity
	}

	// Extract filters
	filterResult := h.filters.ExtractFilters(entities, typeName)

	h.writeJSON(w, http.StatusOK, filterResult)
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
