package filesystem

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// Service orchestrates filesystem scanning, indexing, and search operations.
type Service struct {
	config     *Config
	scanner    *Scanner
	indexer    *Indexer
	search     *Search
	logger     *zerolog.Logger
	instanceID string

	// Track last scan time for incremental updates
	lastScanMu   sync.RWMutex
	lastScanTime time.Time
}

// NewService creates a new service.
func NewService(config *Config, logger *zerolog.Logger) (*Service, error) {
	// Generate instance ID from config paths
	instanceID := generateInstanceID(config.Scan.Paths)

	// Create scanner
	scanner := NewScanner(config, instanceID, logger)

	// Create indexer
	indexer, err := NewIndexer(&config.Meilisearch, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create indexer: %w", err)
	}

	// Create search
	search := NewSearch(indexer, logger)

	return &Service{
		config:     config,
		scanner:    scanner,
		indexer:    indexer,
		search:     search,
		logger:     logger,
		instanceID: instanceID,
	}, nil
}

// generateInstanceID generates a unique instance ID from scan paths.
func generateInstanceID(paths []string) string {
	// Use the first path's base name as instance ID
	if len(paths) == 0 {
		return "default"
	}

	// Get base name of first path
	base := filepath.Base(paths[0])
	// Sanitize: replace non-alphanumeric with underscore
	base = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, base)

	if base == "" || base == "." || base == "/" {
		return "default"
	}

	return base
}

// Scan performs a full filesystem scan and indexes all files.
func (s *Service) Scan(ctx context.Context) (*ScanResult, error) {
	s.logger.Info().Msg("Starting full scan")

	files, err := s.scanner.Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("scan failed: %w", err)
	}

	// Index files with full metadata
	if err := s.indexer.IndexFilesWithLevel(ctx, files, "full"); err != nil {
		return nil, fmt.Errorf("indexing failed: %w", err)
	}

	// Update last scan time
	s.lastScanMu.Lock()
	s.lastScanTime = time.Now()
	s.lastScanMu.Unlock()

	return &ScanResult{
		FilesScanned: len(files),
		ScanTime:     time.Now(),
	}, nil
}

// ScanIncremental performs an incremental scan for files modified since the last scan.
func (s *Service) ScanIncremental(ctx context.Context) (*ScanResult, error) {
	s.lastScanMu.RLock()
	since := s.lastScanTime
	s.lastScanMu.RUnlock()

	// If no previous scan, do a full scan
	if since.IsZero() {
		return s.Scan(ctx)
	}

	s.logger.Info().Time("since", since).Msg("Starting incremental scan")

	files, err := s.scanner.ScanIncremental(ctx, since)
	if err != nil {
		return nil, fmt.Errorf("incremental scan failed: %w", err)
	}

	// Index files with full metadata (incremental scans always get full metadata)
	if err := s.indexer.IndexFilesWithLevel(ctx, files, "full"); err != nil {
		return nil, fmt.Errorf("indexing failed: %w", err)
	}

	// Update last scan time
	s.lastScanMu.Lock()
	s.lastScanTime = time.Now()
	s.lastScanMu.Unlock()

	return &ScanResult{
		FilesScanned: len(files),
		ScanTime:     time.Now(),
		Since:        since,
	}, nil
}

// ScanShallow performs a shallow scan (filenames only, no expensive metadata).
func (s *Service) ScanShallow(ctx context.Context) (*ScanResult, error) {
	s.logger.Info().Msg("Starting shallow scan")

	files, err := s.scanner.ScanShallow(ctx)
	if err != nil {
		return nil, fmt.Errorf("shallow scan failed: %w", err)
	}

	// Index files with shallow level (names only)
	if err := s.indexer.IndexFilesWithLevel(ctx, files, "shallow"); err != nil {
		return nil, fmt.Errorf("indexing failed: %w", err)
	}

	return &ScanResult{
		FilesScanned: len(files),
		ScanTime:     time.Now(),
	}, nil
}

// EnrichFiles enriches specific files with full metadata.
func (s *Service) EnrichFiles(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	s.logger.Info().Int("count", len(ids)).Msg("Enriching files with metadata")

	var files []*File
	for _, id := range ids {
		file, err := s.scanner.GetFile(ctx, id)
		if err != nil {
			s.logger.Warn().Err(err).Str("id", id).Msg("Failed to get file for enrichment")
			continue
		}
		files = append(files, file)
	}

	if len(files) == 0 {
		return nil
	}

	return s.indexer.IndexFilesWithLevel(ctx, files, "full")
}

// Search performs a search query.
func (s *Service) Search(ctx context.Context, req SearchRequest) (*SearchResult, error) {
	return s.search.Search(ctx, req)
}

// Browse lists files in a directory.
func (s *Service) Browse(ctx context.Context, path string) (*BrowseResult, error) {
	files, err := s.scanner.Browse(ctx, path)
	if err != nil {
		return nil, err
	}

	// Convert to response format
	resultFiles := make([]File, len(files))
	for i, f := range files {
		resultFiles[i] = *f
	}

	// Determine parent path
	parent := ""
	parentPath := filepath.Dir(path)
	if parentPath != path && s.scanner.isPathAllowed(parentPath) {
		parent = parentPath
	}

	return &BrowseResult{
		Files:  resultFiles,
		Path:   path,
		IsRoot: s.scanner.IsRootPath(path),
		Parent: parent,
	}, nil
}

// GetFile retrieves a single file by ID.
func (s *Service) GetFile(ctx context.Context, id string) (*File, error) {
	return s.scanner.GetFile(ctx, id)
}

// GetStats returns service statistics.
func (s *Service) GetStats(ctx context.Context) (*ServiceStats, error) {
	docCount, err := s.indexer.GetDocumentCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get document count: %w", err)
	}

	s.lastScanMu.RLock()
	lastScan := s.lastScanTime
	s.lastScanMu.RUnlock()

	return &ServiceStats{
		IndexedDocuments: int(docCount),
		LastScanTime:     lastScan,
		MeilisearchURL:   s.config.Meilisearch.URL,
		ScanPaths:        s.config.Scan.Paths,
	}, nil
}

// IsHealthy checks if the service and its dependencies are healthy.
func (s *Service) IsHealthy(ctx context.Context) error {
	// Check Meilisearch
	if !s.indexer.IsHealthy(ctx) {
		return fmt.Errorf("meilisearch is not healthy")
	}

	return nil
}

// Shutdown gracefully shuts down the service.
func (s *Service) Shutdown(ctx context.Context) error {
	s.logger.Info().Msg("Shutting down service")
	// Close Meilisearch client if it has a Close method
	if client, ok := interface{}(s.indexer.client).(interface{ Close() }); ok {
		client.Close()
	}
	return nil
}

// InstanceID returns the service instance ID.
func (s *Service) InstanceID() string {
	return s.instanceID
}

// ScanResult represents the result of a scan operation.
type ScanResult struct {
	FilesScanned int       `json:"files_scanned"`
	ScanTime     time.Time `json:"scan_time"`
	Since        time.Time `json:"since,omitempty"`
}

// ServiceStats represents service statistics.
type ServiceStats struct {
	IndexedDocuments int       `json:"indexed_documents"`
	LastScanTime     time.Time `json:"last_scan_time"`
	MeilisearchURL   string    `json:"meilisearch_url"`
	ScanPaths        []string  `json:"scan_paths"`
}
