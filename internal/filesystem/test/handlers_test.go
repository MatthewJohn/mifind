package test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"github.com/yourname/mifind/internal/filesystem"
)

// TestHandler_Health tests the health check endpoint.
func TestHandler_Health(t *testing.T) {
	// Create a mock service
	service := &mockService{}

	// Create handlers
	logger := zerolog.New(zerolog.NewConsoleWriter())
	handlers := filesystem.NewHandlers(service, &logger, "0.1.0")

	// Create request
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	// Call handler
	handlers.Health(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %v", response["status"])
	}

	if response["version"] != "0.1.0" {
		t.Errorf("Expected version '0.1.0', got %v", response["version"])
	}
}

// TestHandler_Search_BasicQuery tests the search endpoint with a basic query.
func TestHandler_Search_BasicQuery(t *testing.T) {
	// Create a mock service
	service := &mockService{
		searchResult: &filesystem.SearchResult{
			Files:      []filesystem.File{},
			TotalCount: 0,
			Query:      "test",
		},
	}

	// Create handlers
	logger := zerolog.New(zerolog.NewConsoleWriter())
	handlers := filesystem.NewHandlers(service, &logger, "0.1.0")

	// Create request body
	reqBody := filesystem.SearchRequest{
		Query: "test",
		Limit: 10,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// Create request
	req := httptest.NewRequest("POST", "/search", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Call handler
	handlers.Search(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response filesystem.SearchResult
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Query != "test" {
		t.Errorf("Expected query 'test', got %v", response.Query)
	}
}

// TestHandler_Search_InvalidRequest tests the search endpoint with invalid request.
func TestHandler_Search_InvalidRequest(t *testing.T) {
	// Create a mock service
	service := &mockService{}

	// Create handlers
	logger := zerolog.New(zerolog.NewConsoleWriter())
	handlers := filesystem.NewHandlers(service, &logger, "0.1.0")

	// Create request with invalid JSON
	req := httptest.NewRequest("POST", "/search", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Call handler
	handlers.Search(w, req)

	// Check response
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// TestHandler_GetFile_ValidID tests getting a file by ID.
func TestHandler_GetFile_ValidID(t *testing.T) {
	// Create a mock service
	service := &mockService{
		file: &filesystem.File{
			ID:   "test123",
			Name: "test.txt",
		},
	}

	// Create handlers
	logger := zerolog.New(zerolog.NewConsoleWriter())
	handlers := filesystem.NewHandlers(service, &logger, "0.1.0")

	// Create request
	req := httptest.NewRequest("GET", "/file/test123", nil)
	w := httptest.NewRecorder()

	// Add mux vars
	req = mux.SetURLVars(req, map[string]string{"id": "test123"})

	// Call handler
	handlers.GetFile(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	fileMap, ok := response["file"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected 'file' field in response")
	}

	if fileMap["id"] != "test123" {
		t.Errorf("Expected file ID 'test123', got %v", fileMap["id"])
	}
}

// TestHandler_GetFile_InvalidID tests getting a file with invalid ID.
func TestHandler_GetFile_InvalidID(t *testing.T) {
	// Create a mock service that returns error
	service := &mockService{
		getFileErr: true,
	}

	// Create handlers
	logger := zerolog.New(zerolog.NewConsoleWriter())
	handlers := filesystem.NewHandlers(service, &logger, "0.1.0")

	// Create request
	req := httptest.NewRequest("GET", "/file/invalid", nil)
	w := httptest.NewRecorder()

	// Add mux vars
	req = mux.SetURLVars(req, map[string]string{"id": "invalid"})

	// Call handler
	handlers.GetFile(w, req)

	// Check response
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

// TestHandler_Browse tests the browse endpoint.
func TestHandler_Browse(t *testing.T) {
	// Create a mock service
	service := &mockService{
		browseResult: &filesystem.BrowseResult{
			Files: []filesystem.File{
				{ID: "1", Name: "file1.txt"},
				{ID: "2", Name: "file2.txt"},
			},
			Path:   "/test",
			IsRoot: true,
		},
	}

	// Create handlers
	logger := zerolog.New(zerolog.NewConsoleWriter())
	handlers := filesystem.NewHandlers(service, &logger, "0.1.0")

	// Create request
	req := httptest.NewRequest("GET", "/browse?path=/test", nil)
	w := httptest.NewRecorder()

	// Call handler
	handlers.Browse(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response filesystem.BrowseResult
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(response.Files) != 2 {
		t.Errorf("Expected 2 files, got %d", len(response.Files))
	}

	if response.Path != "/test" {
		t.Errorf("Expected path '/test', got %v", response.Path)
	}
}

// TestHandler_Index tests the index endpoint.
func TestHandler_Index(t *testing.T) {
	// Create a mock service
	service := &mockService{}

	// Create handlers
	logger := zerolog.New(zerolog.NewConsoleWriter())
	handlers := filesystem.NewHandlers(service, &logger, "0.1.0")

	// Create request
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	// Call handler
	handlers.Index(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["name"] != "filesystem-api" {
		t.Errorf("Expected name 'filesystem-api', got %v", response["name"])
	}

	if response["version"] != "0.1.0" {
		t.Errorf("Expected version '0.1.0', got %v", response["version"])
	}
}

// Mock service for testing

type mockService struct {
	searchResult *filesystem.SearchResult
	browseResult *filesystem.BrowseResult
	file         *filesystem.File
	getFileErr   bool
	searchErr    bool
	browseErr    bool
	stats        *filesystem.ServiceStats
	healthErr    error
}

func (m *mockService) Scan(ctx context.Context) (*filesystem.ScanResult, error) {
	return &filesystem.ScanResult{}, nil
}

func (m *mockService) ScanIncremental(ctx context.Context) (*filesystem.ScanResult, error) {
	return &filesystem.ScanResult{}, nil
}

func (m *mockService) ScanShallow(ctx context.Context) (*filesystem.ScanResult, error) {
	return &filesystem.ScanResult{}, nil
}

func (m *mockService) EnrichFiles(ctx context.Context, ids []string) error {
	return nil
}

func (m *mockService) Search(ctx context.Context, req filesystem.SearchRequest) (*filesystem.SearchResult, error) {
	if m.searchErr {
		return nil, &testError{}
	}
	return m.searchResult, nil
}

func (m *mockService) Browse(ctx context.Context, path string) (*filesystem.BrowseResult, error) {
	if m.browseErr {
		return nil, &testError{}
	}
	return m.browseResult, nil
}

func (m *mockService) GetFile(ctx context.Context, id string) (*filesystem.File, error) {
	if m.getFileErr {
		return nil, &testError{}
	}
	return m.file, nil
}

func (m *mockService) GetStats(ctx context.Context) (*filesystem.ServiceStats, error) {
	if m.stats == nil {
		return &filesystem.ServiceStats{}, nil
	}
	return m.stats, nil
}

func (m *mockService) IsHealthy(ctx context.Context) error {
	return m.healthErr
}

func (m *mockService) Shutdown(ctx context.Context) error {
	return nil
}

func (m *mockService) InstanceID() string {
	return "test"
}

type testError struct{}

func (e *testError) Error() string {
	return "test error"
}
