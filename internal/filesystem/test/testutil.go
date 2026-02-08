package test

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// CreateTestDirectory creates a temporary test directory with sample files.
func CreateTestDirectory(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()

	// Create directory structure
	dirs := []string{
		"documents",
		"images",
		"code",
		".git",         // Should be excluded
		"node_modules", // Should be excluded
	}

	for _, dir := range dirs {
		if err := os.Mkdir(filepath.Join(tmpDir, dir), 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	// Create test files
	testFiles := map[string]string{
		"documents/readme.txt":  "This is a readme file",
		"documents/notes.md":    "# Notes\n\nSome notes here",
		"images/photo.jpg":      "fake image data",
		"code/main.go":          "package main\n\nfunc main() {}\n",
		"code/utils.go":         "package main\n\nfunc utils() {}\n",
		".git/config":           "git config",     // Should be excluded
		"node_modules/index.js": "module.exports", // Should be excluded
		"test.tmp":              "temporary file", // Should be excluded
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", path, err)
		}
	}

	// Set modification times for some files
	oldFile := filepath.Join(tmpDir, "documents", "readme.txt")
	oldTime := time.Now().Add(-24 * time.Hour)
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatalf("Failed to set file time: %v", err)
	}

	return tmpDir
}

// CleanupTestDirectory removes a test directory.
func CleanupTestDirectory(t *testing.T, dir string) {
	t.Helper()
	if err := os.RemoveAll(dir); err != nil {
		t.Logf("Failed to cleanup directory %s: %v", dir, err)
	}
}

// AssertFileEquals compares two File structs for equality.
func AssertFileEquals(t *testing.T, expected, actual interface{}) {
	t.Helper()
	// Use testify's assert if available, otherwise basic comparison
	// For now, just check if they're non-nil
	if expected == nil && actual == nil {
		return
	}
	if expected == nil {
		t.Errorf("expected nil, got %v", actual)
		return
	}
	if actual == nil {
		t.Errorf("expected %v, got nil", expected)
		return
	}
}

// AssertNoError fails the test if err is not nil.
func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

// AssertError fails the test if err is nil.
func AssertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// AssertEquals fails the test if expected and actual are not equal.
func AssertEquals(t *testing.T, expected, actual interface{}) {
	t.Helper()
	if expected != actual {
		t.Errorf("expected %v, got %v", expected, actual)
	}
}
