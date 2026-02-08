package filesystem

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// File represents a file from the filesystem.
// This struct must exactly match pkg/provider/filesystem/types.go
type File struct {
	ID        string         `json:"id"`
	Path      string         `json:"path"`
	Name      string         `json:"name"`
	Extension string         `json:"extension"`
	Size      int64          `json:"size"`
	MimeType  string         `json:"mime_type"`
	Modified  time.Time      `json:"modified"`
	Created   time.Time      `json:"created,omitempty"`
	IsDir     bool           `json:"is_dir"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// IndexedFile represents a file as stored in Meilisearch.
type IndexedFile struct {
	ID         string `json:"id"`
	Path       string `json:"path"`
	Name       string `json:"name"`
	Extension  string `json:"extension"`
	MimeType   string `json:"mime_type"`
	Size       int64  `json:"size"`
	Modified   int64  `json:"modified"`
	IsDir      bool   `json:"is_dir"`
	SearchText string `json:"search_text"` // Concatenated for search
	ParentPath string `json:"parent_path"` // For filtering
}

// ToIndexedFile converts a File to an IndexedFile.
func (f *File) ToIndexedFile() IndexedFile {
	return IndexedFile{
		ID:         f.ID,
		Path:       f.Path,
		Name:       f.Name,
		Extension:  f.Extension,
		MimeType:   f.MimeType,
		Size:       f.Size,
		Modified:   f.Modified.Unix(),
		IsDir:      f.IsDir,
		SearchText: f.buildSearchText(),
		ParentPath: f.getParentPath(),
	}
}

// buildSearchText builds a concatenated search text string.
func (f *File) buildSearchText() string {
	parts := []string{f.Name, f.Path}
	if f.Extension != "" {
		parts = append(parts, f.Extension)
	}
	return strings.Join(parts, " ")
}

// getParentPath returns the parent directory path.
func (f *File) getParentPath() string {
	if f.IsDir {
		return filepath.Dir(f.Path)
	}
	return filepath.Dir(f.Path)
}

// FileFromPath creates a File from a filesystem path.
func FileFromPath(path string, info os.FileInfo) (*File, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Get file info
	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	// Extract extension
	ext := filepath.Ext(path)
	if ext != "" {
		ext = strings.TrimPrefix(ext, ".")
	}

	// Get MIME type (will be detected later by the scanner)
	mimeType := DetectMIMETypeBasic(path, fileInfo.IsDir())

	// Get modification time
	modTime := fileInfo.ModTime()

	// Create file ID
	fileID := GenerateFileID(absPath)

	return &File{
		ID:        fileID,
		Path:      absPath,
		Name:      filepath.Base(absPath),
		Extension: ext,
		Size:      fileInfo.Size(),
		MimeType:  mimeType,
		Modified:  modTime,
		IsDir:     fileInfo.IsDir(),
	}, nil
}

// GenerateFileID generates a unique file ID from a path.
// The ID is a 16-character hex string representing the SHA256 hash of the path.
func GenerateFileID(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	// Normalize path for cross-platform consistency
	absPath = filepath.ToSlash(absPath)
	absPath = strings.TrimPrefix(absPath, "/")

	hash := sha256.Sum256([]byte(absPath))
	return fmt.Sprintf("%x", hash)[:16]
}

// GenerateEntityID generates a full entity ID in the format "filesystem:<instance_id>:<file_id>".
func GenerateEntityID(instanceID, fileID string) string {
	return fmt.Sprintf("filesystem:%s:%s", instanceID, fileID)
}

// ParseEntityID parses an entity ID and returns the instance ID and file ID.
func ParseEntityID(entityID string) (instanceID, fileID string, err error) {
	parts := strings.Split(entityID, ":")
	if len(parts) != 3 || parts[0] != "filesystem" {
		return "", "", fmt.Errorf("invalid entity ID format: %s", entityID)
	}
	return parts[1], parts[2], nil
}

// FilesByModified implements sort.Interface for []File based on modification time.
type FilesByModified []File

func (f FilesByModified) Len() int           { return len(f) }
func (f FilesByModified) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f FilesByModified) Less(i, j int) bool { return f[i].Modified.Before(f[j].Modified) }

// FilesByName implements sort.Interface for []File based on name.
type FilesByName []File

func (f FilesByName) Len() int           { return len(f) }
func (f FilesByName) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f FilesByName) Less(i, j int) bool { return f[i].Name < f[j].Name }

// FilesBySize implements sort.Interface for []File based on size.
type FilesBySize []File

func (f FilesBySize) Len() int           { return len(f) }
func (f FilesBySize) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f FilesBySize) Less(i, j int) bool { return f[i].Size < f[j].Size }

// BrowseResult represents browse results from the filesystem-api.
// This struct must exactly match pkg/provider/filesystem/types.go
type BrowseResult struct {
	Files  []File `json:"files"`
	Path   string `json:"path"`
	IsRoot bool   `json:"is_root"`
	Parent string `json:"parent,omitempty"`
}

// FileFilter defines a filter for files.
type FileFilter struct {
	Extension  string
	MimeType   string
	MinSize    int64
	MaxSize    int64
	ModifiedSince time.Time
	ModifiedUntil time.Time
	Path       string // Parent path filter
}

// Matches checks if a file matches the filter.
func (f *FileFilter) Matches(file *File) bool {
	// Check extension
	if f.Extension != "" && file.Extension != f.Extension {
		return false
	}

	// Check MIME type
	if f.MimeType != "" && file.MimeType != f.MimeType {
		return false
	}

	// Check size
	if f.MinSize > 0 && file.Size < f.MinSize {
		return false
	}
	if f.MaxSize > 0 && file.Size > f.MaxSize {
		return false
	}

	// Check modification time
	if !f.ModifiedSince.IsZero() && file.Modified.Before(f.ModifiedSince) {
		return false
	}
	if !f.ModifiedUntil.IsZero() && file.Modified.After(f.ModifiedUntil) {
		return false
	}

	// Check path (parent directory)
	if f.Path != "" {
		parentPath := filepath.Dir(file.Path)
		if parentPath != f.Path && !strings.HasPrefix(parentPath, f.Path+string(filepath.Separator)) {
			return false
		}
	}

	return true
}
