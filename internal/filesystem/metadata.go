package filesystem

import (
	"path/filepath"
	"strings"
)

// MIME type constants
const (
	MIMETextPlain        = "text/plain"
	MIMEDirectory        = "inode/directory"
	MIMEApplicationOctet = "application/octet-stream"

	// Image types
	MIMEImageJPEG = "image/jpeg"
	MIMEImagePNG  = "image/png"
	MIMEImageGIF  = "image/gif"
	MIMEImageWebP = "image/webp"
	MIMEImageSVG  = "image/svg+xml"

	// Video types
	MIMEVideoMP4  = "video/mp4"
	MIMEVideoWebM = "video/webm"
	MIMEVideoAVI  = "video/x-msvideo"
	MIMEVideoMKV  = "video/x-matroska"
	MIMEVideoMOV  = "video/quicktime"

	// Audio types
	MIMEAudioMP3  = "audio/mpeg"
	MIMEAudioWAV  = "audio/wav"
	MIMEAudioOGG  = "audio/ogg"
	MIMEAudioFLAC = "audio/flac"
	MIMEAudioM4A  = "audio/mp4"

	// Document types
	MIMEApplicationPDF  = "application/pdf"
	MIMEApplicationZip  = "application/zip"
	MIMEApplicationGZip = "application/gzip"
	MIMEApplicationTar  = "application/x-tar"

	// Microsoft Office types
	MIMEApplicationDocX  = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	MIMEApplicationXLSX  = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	MIMEApplicationPPTX  = "application/vnd.openxmlformats-officedocument.presentationml.presentation"

	// Text-based types
	MIMETextHTML   = "text/html"
	MIMETextCSS    = "text/css"
	MIMETextJS     = "text/javascript"
	MIMETextJSON   = "application/json"
	MIMETextXML    = "application/xml"
	MIMETextYAML   = "application/x-yaml"
	MIMETextMarkdown = "text/markdown"
)

// MIMETypeMap maps file extensions to MIME types.
// This is a fallback for when the mimetype library is not available.
var MIMETypeMap = map[string]string{
	// Images
	".jpg":  MIMEImageJPEG,
	".jpeg": MIMEImageJPEG,
	".png":  MIMEImagePNG,
	".gif":  MIMEImageGIF,
	".webp": MIMEImageWebP,
	".svg":  MIMEImageSVG,
	".ico":  "image/x-icon",
	".bmp":  "image/bmp",
	".tiff": "image/tiff",
	".heic": "image/heic",
	".avif": "image/avif",

	// Video
	".mp4":  MIMEVideoMP4,
	".webm": MIMEVideoWebM,
	".avi":  MIMEVideoAVI,
	".mkv":  MIMEVideoMKV,
	".mov":  MIMEVideoMOV,
	".flv":  "video/x-flv",
	".wmv":  "video/x-ms-wmv",
	".m4v":  "video/mp4",

	// Audio
	".mp3":  MIMEAudioMP3,
	".wav":  MIMEAudioWAV,
	".ogg":  MIMEAudioOGG,
	".flac": MIMEAudioFLAC,
	".m4a":  MIMEAudioM4A,
	".aac":  "audio/aac",
	".wma":  "audio/x-ms-wma",
	".opus": "audio/opus",

	// Documents
	".pdf": MIMEApplicationPDF,
	".doc": "application/msword",
	".docx": MIMEApplicationDocX,
	".xls": "application/vnd.ms-excel",
	".xlsx": MIMEApplicationXLSX,
	".ppt": "application/vnd.ms-powerpoint",
	".pptx": MIMEApplicationPPTX,

	// Text
	".txt":  MIMETextPlain,
	".md":   MIMETextMarkdown,
	".rst":  MIMETextPlain,
	".log":  MIMETextPlain,
	".html": MIMETextHTML,
	".htm":  MIMETextHTML,
	".xml":  MIMETextXML,
	".json": MIMETextJSON,
	".yaml": MIMETextYAML,
	".yml":  MIMETextYAML,
	".toml": "application/toml",
	".ini":  "text/plain",
	".cfg":  "text/plain",
	".conf": "text/plain",

	// Code
	".css":  MIMETextCSS,
	".scss": "text/x-scss",
	".sass": "text/x-sass",
	".less": "text/x-less",
	".js":   MIMETextJS,
	".jsx":  "text/javascript",
	".ts":   "text/typescript",
	".tsx":  "text/typescript",
	".py":   "text/x-python",
	".rb":   "text/x-ruby",
	".go":   "text/x-go",
	".rs":   "text/x-rust",
	".java": "text/x-java",
	".c":    "text/x-c",
	".cpp":  "text/x-c++",
	".h":    "text/x-c",
	".hpp":  "text/x-c++",
	".cs":   "text/x-csharp",
	".php":  "text/x-php",
	".sh":   "text/x-shellscript",
	".bash": "text/x-shellscript",
	".zsh":  "text/x-shellscript",

	// Archives
	".zip":  MIMEApplicationZip,
	".tar":  MIMEApplicationTar,
	".gz":   MIMEApplicationGZip,
	".rar":  "application/x-rar-compressed",
	".7z":   "application/x-7z-compressed",
	".bz2":  "application/x-bzip2",
	".xz":   "application/x-xz",
}

// DetectMIMETypeBasic detects the MIME type of a file using extension-based detection.
// This is a basic version used during file creation; the full version uses the mimetype library.
func DetectMIMETypeBasic(path string, isDir bool) string {
	if isDir {
		return MIMEDirectory
	}

	ext := strings.ToLower(filepath.Ext(path))

	// Check our MIME type map first
	if mimeType, ok := MIMETypeMap[ext]; ok {
		return mimeType
	}

	// Return default
	return MIMEApplicationOctet
}

// DetectMIMETypeAdvanced detects the MIME type of a file using content detection.
// This uses the mimetype library for accurate detection.
func DetectMIMETypeAdvanced(path string, isDir bool) string {
	if isDir {
		return MIMEDirectory
	}

	// Try to detect by content
	// Note: This requires the mimetype library to be imported
	// For now, use the basic detection
	// TODO: Implement with github.com/gabriel-vasile/mimetype
	return DetectMIMETypeBasic(path, isDir)
}

// IsImage checks if a MIME type is an image.
func IsImage(mimeType string) bool {
	return strings.HasPrefix(mimeType, "image/")
}

// IsVideo checks if a MIME type is a video.
func IsVideo(mimeType string) bool {
	return strings.HasPrefix(mimeType, "video/")
}

// IsAudio checks if a MIME type is audio.
func IsAudio(mimeType string) bool {
	return strings.HasPrefix(mimeType, "audio/")
}

// IsDocument checks if a MIME type is a document.
func IsDocument(mimeType string) bool {
	return strings.HasPrefix(mimeType, "text/") ||
		strings.HasPrefix(mimeType, "application/") &&
		(strings.Contains(mimeType, "pdf") ||
			strings.Contains(mimeType, "document") ||
			strings.Contains(mimeType, "sheet") ||
			strings.Contains(mimeType, "presentation"))
}

// IsArchive checks if a MIME type is an archive.
func IsArchive(mimeType string) bool {
	return mimeType == MIMEApplicationZip ||
		mimeType == MIMEApplicationGZip ||
		mimeType == MIMEApplicationTar ||
		strings.Contains(mimeType, "rar") ||
		strings.Contains(mimeType, "7z") ||
		strings.Contains(mimeType, "bzip2") ||
		strings.Contains(mimeType, "xz")
}

// IsText checks if a MIME type is text-based.
func IsText(mimeType string) bool {
	return strings.HasPrefix(mimeType, "text/")
}

// IsCode checks if a file extension indicates code.
func IsCode(ext string) bool {
	codeExtensions := map[string]bool{
		".js": true, ".jsx": true, ".ts": true, ".tsx": true,
		".py": true, ".rb": true, ".go": true, ".rs": true,
		".java": true, ".c": true, ".cpp": true, ".h": true, ".hpp": true,
		".cs": true, ".php": true, ".sh": true, ".bash": true, ".zsh": true,
		".css": true, ".scss": true, ".sass": true, ".less": true,
		".json": true, ".xml": true, ".yaml": true, ".yml": true,
	}
	return codeExtensions[strings.ToLower(ext)]
}

// GetCategory returns a human-readable category for a MIME type.
func GetCategory(mimeType string) string {
	switch {
	case mimeType == MIMEDirectory:
		return "folder"
	case IsImage(mimeType):
		return "image"
	case IsVideo(mimeType):
		return "video"
	case IsAudio(mimeType):
		return "audio"
	case IsDocument(mimeType):
		return "document"
	case IsArchive(mimeType):
		return "archive"
	case IsCode(mimeType):
		return "code"
	case IsText(mimeType):
		return "text"
	default:
		return "file"
	}
}

// GetExtensionForMimeType returns a common file extension for a MIME type.
func GetExtensionForMimeType(mimeType string) string {
	for ext, mt := range MIMETypeMap {
		if mt == mimeType {
			return ext
		}
	}
	return ""
}
