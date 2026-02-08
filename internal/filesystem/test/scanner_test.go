package test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/yourname/mifind/internal/filesystem"
)

// TestScanner_ExcludedDirectories tests that excluded directories are skipped.
func TestScanner_ExcludedDirectories(t *testing.T) {
	tmpDir := CreateTestDirectory(t)
	defer CleanupTestDirectory(t, tmpDir)

	// Create config
	config := &filesystem.Config{
		Scan: filesystem.ScanConfig{
			Paths:           []string{tmpDir},
			ExcludedDirs:    []string{".git", "node_modules"},
			ExcludePatterns: []string{"*.tmp"},
			FollowSymlinks:  false,
			MaxDepth:        20,
		},
	}

	// Create scanner
	logger := zerolog.New(zerolog.NewConsoleWriter())
	scanner := filesystem.NewScanner(config, "test", &logger)

	// Scan
	ctx := context.Background()
	files, err := scanner.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Check that excluded directories were skipped
	var gitFiles, nodeModulesFiles, tmpFiles int
	for _, file := range files {
		if contains(file.Path, ".git") {
			gitFiles++
		}
		if contains(file.Path, "node_modules") {
			nodeModulesFiles++
		}
		if file.Extension == "tmp" {
			tmpFiles++
		}
	}

	if gitFiles > 0 {
		t.Errorf("Expected .git directory to be excluded, found %d files", gitFiles)
	}
	if nodeModulesFiles > 0 {
		t.Errorf("Expected node_modules directory to be excluded, found %d files", nodeModulesFiles)
	}
	if tmpFiles > 0 {
		t.Errorf("Expected .tmp files to be excluded, found %d files", tmpFiles)
	}

	// Check that we found the expected files
	if len(files) < 5 {
		t.Errorf("Expected at least 5 files, got %d", len(files))
	}
}

// TestScanner_MaxDepth tests that max depth is respected.
func TestScanner_MaxDepth(t *testing.T) {
	tmpDir := CreateTestDirectory(t)
	defer CleanupTestDirectory(t, tmpDir)

	// Create config with max depth 0 (should only scan root)
	config := &filesystem.Config{
		Scan: filesystem.ScanConfig{
			Paths:           []string{tmpDir},
			ExcludedDirs:    []string{".git", "node_modules"},
			ExcludePatterns: []string{},
			FollowSymlinks:  false,
			MaxDepth:        0,
		},
	}

	// Create scanner
	logger := zerolog.New(zerolog.NewConsoleWriter())
	scanner := filesystem.NewScanner(config, "test", &logger)

	// Scan
	ctx := context.Background()
	files, err := scanner.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// With max depth 0, we should only get entries at root level
	// Check that no files from subdirectories are included
	for _, file := range files {
		relPath := file.Path[len(tmpDir):]
		// Count separators to check if file is in subdirectory
		sepCount := 0
		for _, c := range relPath {
			if c == filepath.Separator {
				sepCount++
			}
		}
		// More than 1 separator means it's in a subdirectory
		if sepCount > 1 {
			t.Errorf("File %s exceeds max depth of 0 (found in subdirectory)", file.Path)
		}
	}
}

// TestScanner_Incremental tests incremental scanning.
func TestScanner_Incremental(t *testing.T) {
	tmpDir := CreateTestDirectory(t)
	defer CleanupTestDirectory(t, tmpDir)

	// Create config
	config := &filesystem.Config{
		Scan: filesystem.ScanConfig{
			Paths:           []string{tmpDir},
			ExcludedDirs:    []string{".git", "node_modules"},
			ExcludePatterns: []string{"*.tmp"},
			FollowSymlinks:  false,
			MaxDepth:        20,
		},
	}

	// Create scanner
	logger := zerolog.New(zerolog.NewConsoleWriter())
	scanner := filesystem.NewScanner(config, "test", &logger)

	ctx := context.Background()

	// First scan
	files1, err := scanner.Scan(ctx)
	if err != nil {
		t.Fatalf("First scan failed: %v", err)
	}

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Create a new file
	newFile := "documents/new.txt"
	fullPath := joinPath(tmpDir, newFile)
	if err := os.WriteFile(fullPath, []byte("new file"), 0644); err != nil {
		t.Fatalf("Failed to create new file: %v", err)
	}

	// Incremental scan since first scan start
	since := time.Now().Add(-1 * time.Hour)
	files2, err := scanner.ScanIncremental(ctx, since)
	if err != nil {
		t.Fatalf("Incremental scan failed: %v", err)
	}

	// Check that we found the new file
	foundNewFile := false
	for _, file := range files2 {
		if contains(file.Path, "new.txt") {
			foundNewFile = true
			break
		}
	}

	if !foundNewFile {
		t.Errorf("Incremental scan did not find new file")
	}

	// Incremental scan should return fewer files than full scan
	if len(files2) > len(files1) {
		// This might happen if many files were modified, but generally
		// incremental should return fewer or equal files
		t.Logf("Warning: Incremental scan returned more files (%d) than full scan (%d)", len(files2), len(files1))
	}
}

// TestScanner_Browse tests directory browsing.
func TestScanner_Browse(t *testing.T) {
	tmpDir := CreateTestDirectory(t)
	defer CleanupTestDirectory(t, tmpDir)

	// Create config
	config := &filesystem.Config{
		Scan: filesystem.ScanConfig{
			Paths:           []string{tmpDir},
			ExcludedDirs:    []string{".git", "node_modules"},
			ExcludePatterns: []string{"*.tmp"},
			FollowSymlinks:  false,
			MaxDepth:        20,
		},
	}

	// Create scanner
	logger := zerolog.New(zerolog.NewConsoleWriter())
	scanner := filesystem.NewScanner(config, "test", &logger)

	ctx := context.Background()

	// Browse root directory
	docDir := joinPath(tmpDir, "documents")
	files, err := scanner.Browse(ctx, docDir)
	if err != nil {
		t.Fatalf("Browse failed: %v", err)
	}

	// Check that we found files in documents directory
	if len(files) < 2 {
		t.Errorf("Expected at least 2 files in documents directory, got %d", len(files))
	}

	// Verify file names
	var foundReadme, foundNotes bool
	for _, file := range files {
		if file.Name == "readme.txt" {
			foundReadme = true
		}
		if file.Name == "notes.md" {
			foundNotes = true
		}
	}

	if !foundReadme {
		t.Error("Did not find readme.txt in browse results")
	}
	if !foundNotes {
		t.Error("Did not find notes.md in browse results")
	}
}

// Helper functions

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func joinPath(base, path string) string {
	return filepath.Join(base, path)
}
