package filesystem

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is an HTTP client for the filesystem-api service.
type Client struct {
	baseURL    string
	httpClient *http.Client
	apiKey     string
}

// NewClient creates a new filesystem-api client.
func NewClient(baseURL string, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiKey: apiKey,
	}
}

// Search performs a search query against the filesystem-api.
func (c *Client) Search(ctx context.Context, req SearchRequest) (*SearchResult, error) {
	var result SearchResult
	if err := c.doPost(ctx, "/search", req, &result); err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}
	return &result, nil
}

// Browse lists files in a directory.
func (c *Client) Browse(ctx context.Context, path string) (*BrowseResult, error) {
	var result BrowseResult
	url := fmt.Sprintf("/browse?path=%s", path)
	if err := c.doGet(ctx, url, &result); err != nil {
		return nil, fmt.Errorf("browse failed: %w", err)
	}
	return &result, nil
}

// GetFile retrieves a single file by ID.
func (c *Client) GetFile(ctx context.Context, id string) (*GetFileResponse, error) {
	var result GetFileResponse
	url := fmt.Sprintf("/file/%s", id)
	if err := c.doGet(ctx, url, &result); err != nil {
		return nil, fmt.Errorf("get file failed: %w", err)
	}
	return &result, nil
}

// Health checks the health of the filesystem-api service.
func (c *Client) Health(ctx context.Context) (*HealthResponse, error) {
	var result HealthResponse
	if err := c.doGet(ctx, "/health", &result); err != nil {
		return nil, fmt.Errorf("health check failed: %w", err)
	}
	return &result, nil
}

// doGet performs an HTTP GET request.
func (c *Client) doGet(ctx context.Context, path string, result any) error {
	req, err := c.newRequest(ctx, "GET", path, nil)
	if err != nil {
		return err
	}

	return c.doRequest(req, result)
}

// doPost performs an HTTP POST request.
func (c *Client) doPost(ctx context.Context, path string, body any, result any) error {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := c.newRequest(ctx, "POST", path, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	return c.doRequest(req, result)
}

// newRequest creates a new HTTP request with auth headers.
func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add API key if provided
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}

	return req, nil
}

// doRequest executes an HTTP request and decodes the response.
func (c *Client) doRequest(req *http.Request, result any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	// Decode response
	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

// SetTimeout sets the HTTP client timeout.
func (c *Client) SetTimeout(timeout time.Duration) {
	c.httpClient.Timeout = timeout
}

// SetBaseURL sets the base URL for the client.
func (c *Client) SetBaseURL(url string) {
	c.baseURL = url
}
