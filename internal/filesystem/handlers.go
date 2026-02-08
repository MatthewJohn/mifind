package filesystem

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

// ServiceInterface defines the interface for the filesystem service.
// This allows for mock implementations in tests.
type ServiceInterface interface {
	Scan(ctx context.Context) (*ScanResult, error)
	ScanIncremental(ctx context.Context) (*ScanResult, error)
	Search(ctx context.Context, req SearchRequest) (*SearchResult, error)
	Browse(ctx context.Context, path string) (*BrowseResult, error)
	GetFile(ctx context.Context, id string) (*File, error)
	GetStats(ctx context.Context) (*ServiceStats, error)
	IsHealthy(ctx context.Context) error
	Shutdown(ctx context.Context) error
	InstanceID() string
}

// Handlers provides HTTP handlers for the filesystem API.
type Handlers struct {
	service ServiceInterface
	logger  *zerolog.Logger
	version string
}

// NewHandlers creates a new handlers instance.
func NewHandlers(service ServiceInterface, logger *zerolog.Logger, version string) *Handlers {
	return &Handlers{
		service: service,
		logger:  logger,
		version: version,
	}
}

// RegisterRoutes registers all API routes with the given router.
func (h *Handlers) RegisterRoutes(router *mux.Router) {
	// Search endpoint
	router.HandleFunc("/search", h.Search).Methods("POST")

	// Browse endpoint
	router.HandleFunc("/browse", h.Browse).Methods("GET")

	// File endpoint
	router.HandleFunc("/file/{id}", h.GetFile).Methods("GET")

	// Health check
	router.HandleFunc("/health", h.Health).Methods("GET")

	// Stats endpoint
	router.HandleFunc("/stats", h.Stats).Methods("GET")

	// Index endpoint
	router.HandleFunc("/", h.Index).Methods("GET")
}

// Search handles search requests.
func (h *Handlers) Search(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request: %v", err))
		return
	}

	// Set defaults
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Limit > 1000 {
		req.Limit = 1000
	}

	// Execute search
	result, err := h.service.Search(r.Context(), req)
	if err != nil {
		h.logger.Error().Err(err).Msg("Search failed")
		h.writeError(w, http.StatusInternalServerError, "search failed")
		return
	}

	h.logger.Info().
		Str("query", req.Query).
		Int("results", len(result.Files)).
		Int("total_count", result.TotalCount).
		Dur("duration", time.Since(start)).
		Msg("Search completed")

	h.writeJSON(w, http.StatusOK, result)
}

// Browse handles browse requests.
func (h *Handlers) Browse(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "/"
	}

	// Execute browse
	result, err := h.service.Browse(r.Context(), path)
	if err != nil {
		h.logger.Error().Err(err).Str("path", path).Msg("Browse failed")
		h.writeError(w, http.StatusInternalServerError, fmt.Sprintf("browse failed: %v", err))
		return
	}

	h.writeJSON(w, http.StatusOK, result)
}

// GetFile handles get file requests.
func (h *Handlers) GetFile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	if id == "" {
		h.writeError(w, http.StatusBadRequest, "file ID is required")
		return
	}

	// Get file
	file, err := h.service.GetFile(r.Context(), id)
	if err != nil {
		h.logger.Error().Err(err).Str("id", id).Msg("Get file failed")
		h.writeError(w, http.StatusNotFound, "file not found")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"file": file,
	})
}

// Health handles health check requests.
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().Unix(),
		"version":   h.version,
	}

	// Check service health
	if err := h.service.IsHealthy(r.Context()); err != nil {
		health["status"] = "degraded"
		health["error"] = err.Error()
		h.writeJSON(w, http.StatusServiceUnavailable, health)
		return
	}

	// Add stats if available
	if stats, err := h.service.GetStats(r.Context()); err == nil {
		health["indexed_documents"] = stats.IndexedDocuments
		health["last_scan_time"] = stats.LastScanTime.Unix()
		health["meilisearch_url"] = stats.MeilisearchURL
	}

	h.writeJSON(w, http.StatusOK, health)
}

// Stats handles stats requests.
func (h *Handlers) Stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.service.GetStats(r.Context())
	if err != nil {
		h.logger.Error().Err(err).Msg("Get stats failed")
		h.writeError(w, http.StatusInternalServerError, "failed to get stats")
		return
	}

	h.writeJSON(w, http.StatusOK, stats)
}

// Index returns the API index.
func (h *Handlers) Index(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"name":        "filesystem-api",
		"version":     h.version,
		"description": "Filesystem search and browse API",
		"endpoints": map[string]string{
			"POST /search":      "Search for files",
			"GET  /browse":      "Browse directory",
			"GET  /file/{id}":   "Get file by ID",
			"GET  /health":      "Health check",
			"GET  /stats":       "Service statistics",
			"GET  /":            "API index",
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
