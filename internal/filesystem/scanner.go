package filesystem

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// Scanner handles filesystem scanning operations.
type Scanner struct {
	config     *Config
	logger     *zerolog.Logger
	instanceID string
	pathCache  map[string]string // Cache for quick path lookups
	cacheMutex sync.RWMutex
}

// NewScanner creates a new scanner.
func NewScanner(config *Config, instanceID string, logger *zerolog.Logger) *Scanner {
	return &Scanner{
		config:     config,
		logger:     logger,
		instanceID: instanceID,
		pathCache:  make(map[string]string),
	}
}

// Scan performs a full scan of all configured paths.
func (s *Scanner) Scan(ctx context.Context) ([]*File, error) {
	return s.ScanWithOptions(ctx, ScanOptions{FullMetadata: true})
}

// ScanShallow performs a shallow scan (filenames only, no metadata lookup).
func (s *Scanner) ScanShallow(ctx context.Context) ([]*File, error) {
	return s.ScanWithOptions(ctx, ScanOptions{FullMetadata: false})
}

// ScanOptions controls scan behavior.
type ScanOptions struct {
	FullMetadata bool // If false, skip expensive metadata operations
}

// ScanWithOptions performs a scan with the given options.
func (s *Scanner) ScanWithOptions(ctx context.Context, opts ScanOptions) ([]*File, error) {
	scanType := "full"
	if !opts.FullMetadata {
		scanType = "shallow"
	}
	s.logger.Info().Str("scan_type", scanType).Msg("Starting filesystem scan")

	var allFiles []*File
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Create a channel for errors
	errChan := make(chan error, len(s.config.Scan.Paths))

	// Scan each path concurrently
	for _, scanPath := range s.config.Scan.Paths {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()

			files, err := s.scanPathWithOptions(ctx, path, 0, opts)
			if err != nil {
				s.logger.Error().Err(err).Str("path", path).Msg("Failed to scan path")
				errChan <- fmt.Errorf("path %s: %w", path, err)
				return
			}

			mu.Lock()
			allFiles = append(allFiles, files...)
			mu.Unlock()

			s.logger.Info().
				Str("path", path).
				Int("count", len(files)).
				Str("scan_type", scanType).
				Msg("Scanned path")
		}(scanPath)
	}

	// Wait for all scans to complete
	wg.Wait()
	close(errChan)

	// Check for errors
	var scanErrors []error
	for err := range errChan {
		scanErrors = append(scanErrors, err)
	}

	if len(scanErrors) > 0 {
		s.logger.Warn().
			Int("errors", len(scanErrors)).
			Msg("Scan completed with some errors")
	}

	s.logger.Info().
		Int("total_files", len(allFiles)).
		Str("scan_type", scanType).
		Msg("Scan complete")

	return allFiles, nil
}

// ScanIncremental performs an incremental scan for files modified since the given time.
func (s *Scanner) ScanIncremental(ctx context.Context, since time.Time) ([]*File, error) {
	s.logger.Info().
		Time("since", since).
		Msg("Starting incremental filesystem scan")

	var allFiles []*File
	var mu sync.Mutex
	var wg sync.WaitGroup

	errChan := make(chan error, len(s.config.Scan.Paths))

	// Scan each path concurrently
	for _, scanPath := range s.config.Scan.Paths {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()

			files, err := s.scanPathIncremental(ctx, path, since, 0)
			if err != nil {
				s.logger.Error().Err(err).Str("path", path).Msg("Failed to scan path incrementally")
				errChan <- fmt.Errorf("path %s: %w", path, err)
				return
			}

			mu.Lock()
			allFiles = append(allFiles, files...)
			mu.Unlock()

			s.logger.Info().
				Str("path", path).
				Int("count", len(files)).
				Msg("Incremental scan complete for path")
		}(scanPath)
	}

	wg.Wait()
	close(errChan)

	var scanErrors []error
	for err := range errChan {
		scanErrors = append(scanErrors, err)
	}

	if len(scanErrors) > 0 {
		s.logger.Warn().
			Int("errors", len(scanErrors)).
			Msg("Incremental scan completed with some errors")
	}

	s.logger.Info().
		Int("total_files", len(allFiles)).
		Msg("Incremental scan complete")

	return allFiles, nil
}

// Browse lists files in a directory.
func (s *Scanner) Browse(ctx context.Context, path string) ([]*File, error) {
	// Validate path is within configured scan paths
	if !s.isPathAllowed(path) {
		return nil, fmt.Errorf("path is not within configured scan paths: %s", path)
	}

	// Check if path exists
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat path: %w", err)
	}

	// If it's a file, return just that file
	if !info.IsDir() {
		file, err := s.createFile(path, info)
		if err != nil {
			return nil, err
		}
		return []*File{file}, nil
	}

	// Read directory entries
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var files []*File
	for _, entry := range entries {
		fullPath := filepath.Join(path, entry.Name())

		// Skip excluded directories
		if entry.IsDir() && s.config.IsExcludedDir(entry.Name()) {
			continue
		}

		// Skip files matching exclude patterns
		if !entry.IsDir() && s.config.MatchesExcludePattern(entry.Name()) {
			continue
		}

		// Get file info
		info, err := entry.Info()
		if err != nil {
			s.logger.Warn().Err(err).Str("path", fullPath).Msg("Failed to get file info")
			continue
		}

		file, err := s.createFile(fullPath, info)
		if err != nil {
			s.logger.Warn().Err(err).Str("path", fullPath).Msg("Failed to create file record")
			continue
		}

		files = append(files, file)
	}

	return files, nil
}

// GetFile retrieves a single file by ID.
func (s *Scanner) GetFile(ctx context.Context, id string) (*File, error) {
	// Look up the file by ID in the cache
	s.cacheMutex.RLock()
	path, found := s.pathCache[id]
	s.cacheMutex.RUnlock()

	if !found {
		return nil, fmt.Errorf("file not found: %s", id)
	}

	// Check if file still exists
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("file not found: %s", id)
	}

	return s.createFile(path, info)
}

// GetFileByPath retrieves a file by its path.
func (s *Scanner) GetFileByPath(ctx context.Context, path string) (*File, error) {
	// Validate path
	if !s.isPathAllowed(path) {
		return nil, fmt.Errorf("path is not within configured scan paths: %s", path)
	}

	// Get file info
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	return s.createFile(path, info)
}

// scanPathWithOptions scans a single path recursively with options.
func (s *Scanner) scanPathWithOptions(ctx context.Context, path string, depth int, opts ScanOptions) ([]*File, error) {
	// Check context
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Check max depth
	if depth > s.config.Scan.MaxDepth {
		return nil, nil
	}

	// Check if path exists
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat path: %w", err)
	}

	var files []*File

	// If it's a file, create a file record and return
	if !info.IsDir() {
		file, err := s.createFileWithOptions(path, info, opts)
		if err != nil {
			return nil, err
		}
		return []*File{file}, nil
	}

	// Read directory entries
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		fullPath := filepath.Join(path, entry.Name())

		// Skip excluded directories
		if entry.IsDir() && s.config.IsExcludedDir(entry.Name()) {
			continue
		}

		// Skip files matching exclude patterns
		if !entry.IsDir() && s.config.MatchesExcludePattern(entry.Name()) {
			continue
		}

		// Handle symlinks
		if entry.Type()&os.ModeSymlink != 0 {
			if !s.config.Scan.FollowSymlinks {
				continue
			}
			// Resolve symlink
			resolved, err := filepath.EvalSymlinks(fullPath)
			if err != nil {
				s.logger.Warn().Err(err).Str("path", fullPath).Msg("Failed to resolve symlink")
				continue
			}
			fullPath = resolved
		}

		// Get file info
		fileInfo, err := os.Stat(fullPath)
		if err != nil {
			s.logger.Warn().Err(err).Str("path", fullPath).Msg("Failed to get file info")
			continue
		}

		// If it's a directory, recurse
		if fileInfo.IsDir() {
			subFiles, err := s.scanPathWithOptions(ctx, fullPath, depth+1, opts)
			if err != nil {
				s.logger.Warn().Err(err).Str("path", fullPath).Msg("Failed to scan subdirectory")
				continue
			}
			files = append(files, subFiles...)
			continue
		}

		// Create file record
		file, err := s.createFileWithOptions(fullPath, fileInfo, opts)
		if err != nil {
			s.logger.Warn().Err(err).Str("path", fullPath).Msg("Failed to create file record")
			continue
		}

		files = append(files, file)
	}

	return files, nil
}

// scanPath scans a single path recursively (full metadata).
func (s *Scanner) scanPath(ctx context.Context, path string, depth int) ([]*File, error) {
	return s.scanPathWithOptions(ctx, path, depth, ScanOptions{FullMetadata: true})
}

// scanPathIncremental scans a single path for files modified since the given time.
func (s *Scanner) scanPathIncremental(ctx context.Context, path string, since time.Time, depth int) ([]*File, error) {
	// Check context
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Check max depth
	if depth > s.config.Scan.MaxDepth {
		return nil, nil
	}

	// Check if path exists
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat path: %w", err)
	}

	var files []*File

	// If it's a file, check if it was modified
	if !info.IsDir() {
		if info.ModTime().After(since) {
			file, err := s.createFile(path, info)
			if err != nil {
				return nil, err
			}
			return []*File{file}, nil
		}
		return nil, nil
	}

	// Read directory entries
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		fullPath := filepath.Join(path, entry.Name())

		// Skip excluded directories
		if entry.IsDir() && s.config.IsExcludedDir(entry.Name()) {
			continue
		}

		// Skip files matching exclude patterns
		if !entry.IsDir() && s.config.MatchesExcludePattern(entry.Name()) {
			continue
		}

		// Handle symlinks
		if entry.Type()&os.ModeSymlink != 0 {
			if !s.config.Scan.FollowSymlinks {
				continue
			}
			resolved, err := filepath.EvalSymlinks(fullPath)
			if err != nil {
				s.logger.Warn().Err(err).Str("path", fullPath).Msg("Failed to resolve symlink")
				continue
			}
			fullPath = resolved
		}

		// Get file info
		fileInfo, err := os.Stat(fullPath)
		if err != nil {
			s.logger.Warn().Err(err).Str("path", fullPath).Msg("Failed to get file info")
			continue
		}

		// If it's a directory, recurse
		if fileInfo.IsDir() {
			subFiles, err := s.scanPathIncremental(ctx, fullPath, since, depth+1)
			if err != nil {
				s.logger.Warn().Err(err).Str("path", fullPath).Msg("Failed to scan subdirectory")
				continue
			}
			files = append(files, subFiles...)
			continue
		}

		// Check if file was modified
		if fileInfo.ModTime().After(since) {
			file, err := s.createFile(fullPath, fileInfo)
			if err != nil {
				s.logger.Warn().Err(err).Str("path", fullPath).Msg("Failed to create file record")
				continue
			}
			files = append(files, file)
		}
	}

	return files, nil
}

// createFile creates a File record from a path and os.FileInfo.
func (s *Scanner) createFile(path string, info os.FileInfo) (*File, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Extract extension
	ext := filepath.Ext(path)
	if ext != "" {
		ext = ext[1:] // Remove the dot
	}

	// Detect MIME type using mimetype library if available
	mimeType := DetectMIMETypeAdvanced(absPath, info.IsDir())

	// Generate file ID
	fileID := GenerateFileID(absPath)

	// Cache the path
	s.cacheMutex.Lock()
	s.pathCache[fileID] = absPath
	s.cacheMutex.Unlock()

	return &File{
		ID:        fileID,
		Path:      absPath,
		Name:      filepath.Base(absPath),
		Extension: ext,
		Size:      info.Size(),
		MimeType:  mimeType,
		Modified:  info.ModTime(),
		IsDir:     info.IsDir(),
	}, nil
}

// createFileWithOptions creates a File record from a path and os.FileInfo with options.
func (s *Scanner) createFileWithOptions(path string, info os.FileInfo, opts ScanOptions) (*File, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Extract extension
	ext := filepath.Ext(path)
	if ext != "" {
		ext = ext[1:] // Remove the dot
	}

	// Detect MIME type (skip in shallow mode)
	var mimeType string
	if opts.FullMetadata {
		mimeType = DetectMIMETypeAdvanced(absPath, info.IsDir())
	} else {
		if info.IsDir() {
			mimeType = MIMEDirectory
		} else {
			mimeType = DetectMIMETypeBasic(absPath, info.IsDir())
		}
	}

	// Generate file ID
	fileID := GenerateFileID(absPath)

	// Cache the path
	s.cacheMutex.Lock()
	s.pathCache[fileID] = absPath
	s.cacheMutex.Unlock()

	return &File{
		ID:        fileID,
		Path:      absPath,
		Name:      filepath.Base(absPath),
		Extension: ext,
		Size:      info.Size(),
		MimeType:  mimeType,
		Modified:  info.ModTime(),
		IsDir:     info.IsDir(),
	}, nil
}

// isPathAllowed checks if a path is within the configured scan paths.
func (s *Scanner) isPathAllowed(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	for _, scanPath := range s.config.Scan.Paths {
		absScanPath, err := filepath.Abs(scanPath)
		if err != nil {
			continue
		}

		// Check if path is within scan path
		if absPath == absScanPath || isSubPath(absScanPath, absPath) {
			return true
		}
	}

	return false
}

// isSubPath checks if child is a subpath of parent.
func isSubPath(parent, child string) bool {
	parent = filepath.Clean(parent)
	child = filepath.Clean(child)

	if parent == child {
		return true
	}

	if !strings.HasPrefix(child, parent+string(filepath.Separator)) {
		return false
	}

	return true
}

// GetParentPath returns the parent directory path for a given path.
func GetParentPath(path string) string {
	return filepath.Dir(path)
}

// IsRootPath checks if a path is a root path (no parent within scan paths).
func (s *Scanner) IsRootPath(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	for _, scanPath := range s.config.Scan.Paths {
		absScanPath, err := filepath.Abs(scanPath)
		if err != nil {
			continue
		}

		if absPath == absScanPath {
			return true
		}
	}

	return false
}
