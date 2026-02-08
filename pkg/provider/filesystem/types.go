package filesystem

import "time"

// API types for the filesystem-api service responses.

// File represents a file from the filesystem-api.
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

// SearchResult represents search results from the filesystem-api.
type SearchResult struct {
	Files      []File         `json:"files"`
	TotalCount int            `json:"total_count"`
	Query      string         `json:"query"`
	Filters    map[string]any `json:"filters,omitempty"`
}

// BrowseResult represents browse results from the filesystem-api.
type BrowseResult struct {
	Files  []File `json:"files"`
	Path   string `json:"path"`
	IsRoot bool   `json:"is_root"`
	Parent string `json:"parent,omitempty"`
}

// GetFileResponse represents the response for getting a single file.
type GetFileResponse struct {
	File File `json:"file"`
}

// HealthResponse represents the health check response.
type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp int64  `json:"timestamp"`
	Version   string `json:"version,omitempty"`
}

// SearchRequest represents a search request to the filesystem-api.
type SearchRequest struct {
	Query   string         `json:"query"`
	Filters map[string]any `json:"filters,omitempty"`
	Limit   int            `json:"limit,omitempty"`
	Offset  int            `json:"offset,omitempty"`
}

// Entity conversion helpers.

// ToEntityID converts a filesystem file ID to an entity ID.
func ToEntityID(fileID string) string {
	return "filesystem:" + fileID
}

// FromEntityID extracts the file ID from an entity ID.
func FromEntityID(entityID string) string {
	// Remove "filesystem:" prefix if present
	if len(entityID) > 11 && entityID[:11] == "filesystem:" {
		return entityID[11:]
	}
	return entityID
}

// FileTypeToMifindType converts a file extension to a mifind type.
func FileTypeToMifindType(extension string, isDir bool) string {
	if isDir {
		return "collection.folder"
	}

	switch extension {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".svg", ".tiff", ".heic", ".avif":
		return "file.media.image"
	case ".mp4", ".mov", ".avi", ".mkv", ".webm", ".flv", ".wmv", ".m4v":
		return "file.media.video"
	case ".mp3", ".wav", ".flac", ".ogg", ".m4a", ".aac", ".wma", ".opus":
		return "file.media.music"
	case ".pdf":
		return "file.document.pdf"
	case ".doc", ".docx":
		return "file.document.word"
	case ".xls", ".xlsx":
		return "file.document.spreadsheet"
	case ".ppt", ".pptx":
		return "file.document.presentation"
	case ".txt", ".md", ".rst", ".log":
		return "file.document.text"
	case ".html", ".htm":
		return "file.document.html"
	case ".css", ".scss", ".sass", ".less":
		return "file.document.code.css"
	case ".js", ".jsx", ".ts", ".tsx":
		return "file.document.code.javascript"
	case ".py", ".rb", ".go", ".rs", ".java", ".c", ".cpp", ".h", ".hpp":
		return "file.document.code"
	case ".json", ".xml", "yaml", ".yml", ".toml", ".ini", ".cfg":
		return "file.document.config"
	case ".zip", ".tar", ".gz", ".rar", ".7z":
		return "file.archive"
	default:
		return "file"
	}
}
