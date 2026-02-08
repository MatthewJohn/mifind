package test

import (
	"testing"

	"github.com/yourname/mifind/internal/filesystem"
)

// TestMIMEType_DetectionImages tests MIME type detection for images.
func TestMIMEType_DetectionImages(t *testing.T) {
	testCases := []struct {
		ext      string
		expected string
	}{
		{".jpg", filesystem.MIMEImageJPEG},
		{".jpeg", filesystem.MIMEImageJPEG},
		{".png", filesystem.MIMEImagePNG},
		{".gif", filesystem.MIMEImageGIF},
		{".webp", filesystem.MIMEImageWebP},
		{".svg", filesystem.MIMEImageSVG},
	}

	for _, tc := range testCases {
		t.Run(tc.ext, func(t *testing.T) {
			mimeType := filesystem.DetectMIMETypeBasic("test"+tc.ext, false)
			if mimeType != tc.expected {
				t.Errorf("Expected MIME type %s for %s, got %s", tc.expected, tc.ext, mimeType)
			}
		})
	}
}

// TestMIMEType_DetectionDocuments tests MIME type detection for documents.
func TestMIMEType_DetectionDocuments(t *testing.T) {
	testCases := []struct {
		ext      string
		expected string
	}{
		{".pdf", filesystem.MIMEApplicationPDF},
		{".txt", filesystem.MIMETextPlain},
		{".md", filesystem.MIMETextMarkdown},
		{".html", filesystem.MIMETextHTML},
		{".json", filesystem.MIMETextJSON},
		{".xml", filesystem.MIMETextXML},
	}

	for _, tc := range testCases {
		t.Run(tc.ext, func(t *testing.T) {
			mimeType := filesystem.DetectMIMETypeBasic("test"+tc.ext, false)
			if mimeType != tc.expected {
				t.Errorf("Expected MIME type %s for %s, got %s", tc.expected, tc.ext, mimeType)
			}
		})
	}
}

// TestMIMEType_DetectionArchives tests MIME type detection for archives.
func TestMIMEType_DetectionArchives(t *testing.T) {
	testCases := []struct {
		ext      string
		expected string
	}{
		{".zip", filesystem.MIMEApplicationZip},
		{".tar", filesystem.MIMEApplicationTar},
		{".gz", filesystem.MIMEApplicationGZip},
		{".rar", "application/x-rar-compressed"},
		{".7z", "application/x-7z-compressed"},
	}

	for _, tc := range testCases {
		t.Run(tc.ext, func(t *testing.T) {
			mimeType := filesystem.DetectMIMETypeBasic("test"+tc.ext, false)
			if mimeType != tc.expected {
				t.Errorf("Expected MIME type %s for %s, got %s", tc.expected, tc.ext, mimeType)
			}
		})
	}
}

// TestMIMEType_DetectionVideo tests MIME type detection for videos.
func TestMIMEType_DetectionVideo(t *testing.T) {
	testCases := []struct {
		ext      string
		expected string
	}{
		{".mp4", filesystem.MIMEVideoMP4},
		{".webm", filesystem.MIMEVideoWebM},
		{".avi", filesystem.MIMEVideoAVI},
		{".mkv", filesystem.MIMEVideoMKV},
		{".mov", filesystem.MIMEVideoMOV},
	}

	for _, tc := range testCases {
		t.Run(tc.ext, func(t *testing.T) {
			mimeType := filesystem.DetectMIMETypeBasic("test"+tc.ext, false)
			if mimeType != tc.expected {
				t.Errorf("Expected MIME type %s for %s, got %s", tc.expected, tc.ext, mimeType)
			}
		})
	}
}

// TestMIMEType_DetectionAudio tests MIME type detection for audio.
func TestMIMEType_DetectionAudio(t *testing.T) {
	testCases := []struct {
		ext      string
		expected string
	}{
		{".mp3", filesystem.MIMEAudioMP3},
		{".wav", filesystem.MIMEAudioWAV},
		{".ogg", filesystem.MIMEAudioOGG},
		{".flac", filesystem.MIMEAudioFLAC},
		{".m4a", filesystem.MIMEAudioM4A},
	}

	for _, tc := range testCases {
		t.Run(tc.ext, func(t *testing.T) {
			mimeType := filesystem.DetectMIMETypeBasic("test"+tc.ext, false)
			if mimeType != tc.expected {
				t.Errorf("Expected MIME type %s for %s, got %s", tc.expected, tc.ext, mimeType)
			}
		})
	}
}

// TestFileID_Generation tests file ID generation.
func TestFileID_Generation(t *testing.T) {
	testCases := []struct {
		path     string
		expected string // We'll check that the same path generates the same ID
	}{
		{"/home/user/documents/file.txt", ""},
		{"/home/user/documents/file.txt", ""}, // Same path
		{"/home/user/documents/other.txt", ""},
	}

	// Generate IDs
	ids := make([]string, len(testCases))
	for i, tc := range testCases {
		ids[i] = filesystem.GenerateFileID(tc.path)
		if ids[i] == "" {
			t.Errorf("GenerateFileID returned empty string for path %s", tc.path)
		}
		if len(ids[i]) != 16 {
			t.Errorf("Expected file ID length 16, got %d for path %s", len(ids[i]), tc.path)
		}
	}

	// Check that same path generates same ID
	if ids[0] != ids[1] {
		t.Errorf("Expected same file ID for same path, got %s and %s", ids[0], ids[1])
	}

	// Check that different paths generate different IDs
	if ids[0] == ids[2] {
		t.Errorf("Expected different file IDs for different paths, got %s for both", ids[0])
	}
}

// TestIsDirectory tests directory detection.
func TestIsDirectory(t *testing.T) {
	mimeType := filesystem.DetectMIMETypeBasic("/path/to/dir", true)
	if mimeType != filesystem.MIMEDirectory {
		t.Errorf("Expected directory MIME type, got %s", mimeType)
	}
}

// TestUnknownExtension tests unknown file extension.
func TestUnknownExtension(t *testing.T) {
	mimeType := filesystem.DetectMIMETypeBasic("test.unknownext", false)
	if mimeType == "" {
		t.Errorf("Expected default MIME type for unknown extension, got empty string")
	}
	if mimeType != filesystem.MIMEApplicationOctet && mimeType != "application/octet-stream" {
		t.Logf("Got MIME type for unknown extension: %s", mimeType)
	}
}

// TestIsImage tests IsImage function.
func TestIsImage(t *testing.T) {
	testCases := []struct {
		mimeType string
		expected bool
	}{
		{filesystem.MIMEImageJPEG, true},
		{filesystem.MIMEImagePNG, true},
		{filesystem.MIMETextPlain, false},
		{filesystem.MIMEApplicationPDF, false},
	}

	for _, tc := range testCases {
		t.Run(tc.mimeType, func(t *testing.T) {
			result := filesystem.IsImage(tc.mimeType)
			if result != tc.expected {
				t.Errorf("IsImage(%s) = %v, expected %v", tc.mimeType, result, tc.expected)
			}
		})
	}
}

// TestIsVideo tests IsVideo function.
func TestIsVideo(t *testing.T) {
	testCases := []struct {
		mimeType string
		expected bool
	}{
		{filesystem.MIMEVideoMP4, true},
		{filesystem.MIMEVideoWebM, true},
		{filesystem.MIMETextPlain, false},
		{filesystem.MIMEImageJPEG, false},
	}

	for _, tc := range testCases {
		t.Run(tc.mimeType, func(t *testing.T) {
			result := filesystem.IsVideo(tc.mimeType)
			if result != tc.expected {
				t.Errorf("IsVideo(%s) = %v, expected %v", tc.mimeType, result, tc.expected)
			}
		})
	}
}

// TestIsAudio tests IsAudio function.
func TestIsAudio(t *testing.T) {
	testCases := []struct {
		mimeType string
		expected bool
	}{
		{filesystem.MIMEAudioMP3, true},
		{filesystem.MIMEAudioWAV, true},
		{filesystem.MIMETextPlain, false},
		{filesystem.MIMEImageJPEG, false},
	}

	for _, tc := range testCases {
		t.Run(tc.mimeType, func(t *testing.T) {
			result := filesystem.IsAudio(tc.mimeType)
			if result != tc.expected {
				t.Errorf("IsAudio(%s) = %v, expected %v", tc.mimeType, result, tc.expected)
			}
		})
	}
}

// TestIsDocument tests IsDocument function.
func TestIsDocument(t *testing.T) {
	testCases := []struct {
		mimeType string
		expected bool
	}{
		{filesystem.MIMEApplicationPDF, true},
		{filesystem.MIMETextPlain, true},
		{filesystem.MIMETextHTML, true},
		{filesystem.MIMEImageJPEG, false},
		{filesystem.MIMEVideoMP4, false},
	}

	for _, tc := range testCases {
		t.Run(tc.mimeType, func(t *testing.T) {
			result := filesystem.IsDocument(tc.mimeType)
			if result != tc.expected {
				t.Errorf("IsDocument(%s) = %v, expected %v", tc.mimeType, result, tc.expected)
			}
		})
	}
}

// TestIsArchive tests IsArchive function.
func TestIsArchive(t *testing.T) {
	testCases := []struct {
		mimeType string
		expected bool
	}{
		{filesystem.MIMEApplicationZip, true},
		{filesystem.MIMEApplicationTar, true},
		{filesystem.MIMEApplicationGZip, true},
		{filesystem.MIMETextPlain, false},
		{filesystem.MIMEImageJPEG, false},
	}

	for _, tc := range testCases {
		t.Run(tc.mimeType, func(t *testing.T) {
			result := filesystem.IsArchive(tc.mimeType)
			if result != tc.expected {
				t.Errorf("IsArchive(%s) = %v, expected %v", tc.mimeType, result, tc.expected)
			}
		})
	}
}

// TestGetCategory tests GetCategory function.
func TestGetCategory(t *testing.T) {
	testCases := []struct {
		mimeType string
		expected string
	}{
		{filesystem.MIMEDirectory, "folder"},
		{filesystem.MIMEImageJPEG, "image"},
		{filesystem.MIMEVideoMP4, "video"},
		{filesystem.MIMEAudioMP3, "audio"},
		{filesystem.MIMEApplicationPDF, "document"},
		{filesystem.MIMEApplicationZip, "archive"},
		{filesystem.MIMETextPlain, "document"},
	}

	for _, tc := range testCases {
		t.Run(tc.mimeType, func(t *testing.T) {
			result := filesystem.GetCategory(tc.mimeType)
			if result != tc.expected {
				t.Errorf("GetCategory(%s) = %v, expected %v", tc.mimeType, result, tc.expected)
			}
		})
	}
}
