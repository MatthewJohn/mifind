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

// TestIntegration_FullScanAndSearch tests the full workflow of scanning and searching.
// Note: This test requires a running Meilisearch instance.
// Run with: go test -tags=integration ./internal/filesystem/test/...
func TestIntegration_FullScanAndSearch(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test requires Meilisearch to be running
	// For CI/CD, you would typically start Meilisearch as a sidecar or in Docker

	tmpDir := CreateTestDirectory(t)
	defer CleanupTestDirectory(t, tmpDir)

	// Create config
	config := &filesystem.Config{
		Meilisearch: filesystem.MeilisearchConfig{
			URL:       getEnv("MEILISEARCH_URL", "http://localhost:7700"),
			APIKey:    getEnv("MEILISEARCH_API_KEY", ""),
			IndexName: "test_filesystem_" + time.Now().Format("20060102_150405"),
		},
		Scan: filesystem.ScanConfig{
			Paths:           []string{tmpDir},
			ExcludedDirs:    []string{".git", "node_modules"},
			ExcludePatterns: []string{"*.tmp"},
			FollowSymlinks:  false,
			MaxDepth:        20,
		},
	}

	// Create service
	logger := zerolog.New(zerolog.NewConsoleWriter())
	svc, err := filesystem.NewService(config, &logger)
	if err != nil {
		t.Skipf("Skipping integration test - failed to create service (Meilisearch not available?): %v", err)
		return
	}
	defer svc.Shutdown(context.Background())

	// Perform full scan
	ctx := context.Background()
	scanResult, err := svc.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if scanResult.FilesScanned == 0 {
		t.Error("Expected to scan at least some files")
	}

	// Test search
	searchReq := filesystem.SearchRequest{
		Query: "readme",
		Limit: 10,
	}

	searchResult, err := svc.Search(ctx, searchReq)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should find at least the readme.txt file
	if searchResult.TotalCount == 0 {
		t.Error("Expected search to find at least one result for 'readme'")
	}

	// Test browse
	docDir := filepath.Join(tmpDir, "documents")
	browseResult, err := svc.Browse(ctx, docDir)
	if err != nil {
		t.Fatalf("Browse failed: %v", err)
	}

	if len(browseResult.Files) == 0 {
		t.Error("Expected browse to return at least some files")
	}

	// Test get file
	if len(searchResult.Files) > 0 {
		fileID := searchResult.Files[0].ID
		file, err := svc.GetFile(ctx, fileID)
		if err != nil {
			t.Fatalf("GetFile failed: %v", err)
		}

		if file.ID != fileID {
			t.Errorf("Expected file ID %s, got %s", fileID, file.ID)
		}
	}

	// Test stats
	stats, err := svc.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if stats.IndexedDocuments == 0 {
		t.Error("Expected indexed documents to be greater than 0")
	}

	// Test health
	if err := svc.IsHealthy(ctx); err != nil {
		t.Errorf("Expected service to be healthy: %v", err)
	}
}

// TestIntegration_IncrementalUpdate tests incremental scanning.
// Note: This test requires a running Meilisearch instance.
func TestIntegration_IncrementalUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := CreateTestDirectory(t)
	defer CleanupTestDirectory(t, tmpDir)

	// Create config
	config := &filesystem.Config{
		Meilisearch: filesystem.MeilisearchConfig{
			URL:       getEnv("MEILISEARCH_URL", "http://localhost:7700"),
			APIKey:    getEnv("MEILISEARCH_API_KEY", ""),
			IndexName: "test_filesystem_incremental_" + time.Now().Format("20060102_150405"),
		},
		Scan: filesystem.ScanConfig{
			Paths:           []string{tmpDir},
			ExcludedDirs:    []string{".git", "node_modules"},
			ExcludePatterns: []string{"*.tmp"},
			FollowSymlinks:  false,
			MaxDepth:        20,
		},
	}

	// Create service
	logger := zerolog.New(zerolog.NewConsoleWriter())
	svc, err := filesystem.NewService(config, &logger)
	if err != nil {
		t.Skipf("Skipping integration test - failed to create service (Meilisearch not available?): %v", err)
		return
	}
	defer svc.Shutdown(context.Background())

	ctx := context.Background()

	// Initial scan
	_, err = svc.Scan(ctx)
	if err != nil {
		t.Fatalf("Initial scan failed: %v", err)
	}

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Create a new file
	newFile := filepath.Join(tmpDir, "new_test_file.txt")
	if err := os.WriteFile(newFile, []byte("new test file content"), 0644); err != nil {
		t.Fatalf("Failed to create new file: %v", err)
	}

	// Incremental scan
	scanResult, err := svc.ScanIncremental(ctx)
	if err != nil {
		t.Fatalf("Incremental scan failed: %v", err)
	}

	// Should have found at least the new file
	if scanResult.FilesScanned == 0 {
		t.Error("Expected incremental scan to find at least the new file")
	}

	// Search for the new file
	searchReq := filesystem.SearchRequest{
		Query: "new test file",
		Limit: 10,
	}

	searchResult, err := svc.Search(ctx, searchReq)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if searchResult.TotalCount == 0 {
		t.Error("Expected search to find the new file")
	}
}

// Helper function to get environment variable with default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
