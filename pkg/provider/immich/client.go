package immich

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is an HTTP client for the Immich API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new Immich API client.
// If insecureSkipVerify is true, TLS certificate verification will be skipped.
func NewClient(baseURL, apiKey string, insecureSkipVerify bool) *Client {
	client := &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	if insecureSkipVerify {
		client.httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}

	return client
}

// Search performs a search query against the Immich API.
func (c *Client) Search(ctx context.Context, query string, limit int) (*SearchResponse, error) {
	// Immich uses /api/search/METADATA for smart search with metadata
	url := fmt.Sprintf("%s/api/search/metadata", c.baseURL)

	// Build search request body
	reqBody := map[string]any{
		"page":      1,
		"pageSize":  limit,
		"query":     query,
		"algorithm": "smart", // Use smart search
	}

	var result SearchResponse
	if err := c.doPost(ctx, url, reqBody, &result); err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	return &result, nil
}

// GetAsset retrieves a single asset by ID.
func (c *Client) GetAsset(ctx context.Context, id string) (*Asset, error) {
	url := fmt.Sprintf("%s/api/assets/%s", c.baseURL, id)

	var result Asset
	if err := c.doGet(ctx, url, &result); err != nil {
		return nil, fmt.Errorf("get asset failed: %w", err)
	}

	return &result, nil
}

// GetAlbum retrieves a single album by ID.
func (c *Client) GetAlbum(ctx context.Context, id string) (*Album, error) {
	url := fmt.Sprintf("%s/api/albums/%s", c.baseURL, id)

	var result Album
	if err := c.doGet(ctx, url, &result); err != nil {
		return nil, fmt.Errorf("get album failed: %w", err)
	}

	return &result, nil
}

// GetAlbumAssets retrieves all assets in an album.
// Uses getAlbumInfo which returns assets in the AlbumResponseDto.
func (c *Client) GetAlbumAssets(ctx context.Context, albumID string, limit int) ([]Asset, error) {
	url := fmt.Sprintf("%s/api/albums/%s", c.baseURL, albumID)

	var result struct {
		Assets []Asset `json:"assets"`
	}

	if err := c.doGet(ctx, url, &result); err != nil {
		return nil, fmt.Errorf("get album assets failed: %w", err)
	}

	// Apply limit if specified
	if limit > 0 && len(result.Assets) > limit {
		return result.Assets[:limit], nil
	}

	return result.Assets, nil
}

// ListAlbums lists all albums.
// The Immich API returns all albums at once (no pagination).
func (c *Client) ListAlbums(ctx context.Context, limit int) ([]Album, error) {
	url := fmt.Sprintf("%s/api/albums", c.baseURL)

	var result []Album
	if err := c.doGet(ctx, url, &result); err != nil {
		return nil, fmt.Errorf("list albums failed: %w", err)
	}

	// Apply limit if specified
	if limit > 0 && len(result) > limit {
		return result[:limit], nil
	}

	return result, nil
}

// ListPeople lists all detected people.
// If limit is 0, returns all people (up to API max).
func (c *Client) ListPeople(ctx context.Context, limit int) ([]Person, error) {
	size := limit
	if limit == 0 {
		size = 1000 // Use the API max
	}

	// Immich uses 'size' not 'pageSize' for people endpoint
	url := fmt.Sprintf("%s/api/people?page=1&size=%d", c.baseURL, size)

	var result struct {
		Total int      `json:"total"`
		Count int      `json:"count"`
		Items []Person `json:"people"`
	}

	if err := c.doGet(ctx, url, &result); err != nil {
		return nil, fmt.Errorf("list people failed: %w", err)
	}

	return result.Items, nil
}

// GetPerson retrieves a single person by ID.
func (c *Client) GetPerson(ctx context.Context, id string) (*Person, error) {
	url := fmt.Sprintf("%s/api/people/%s", c.baseURL, id)

	var result Person
	if err := c.doGet(ctx, url, &result); err != nil {
		return nil, fmt.Errorf("get person failed: %w", err)
	}

	return &result, nil
}

// Health checks the health of the Immich API.
func (c *Client) Health(ctx context.Context) error {
	// Immich doesn't have a standard health endpoint, so we'll use ping
	url := fmt.Sprintf("%s/api/server/ping", c.baseURL)

	req, err := c.newRequest(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("health check failed: status %d: %s", resp.StatusCode, string(body))
	}

	return nil
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
	req, err := http.NewRequestWithContext(ctx, method, path, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Immich uses API key in the X-API-Key header
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Accept", "application/json")

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
	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// SetTimeout sets the HTTP client timeout.
func (c *Client) SetTimeout(timeout time.Duration) {
	c.httpClient.Timeout = timeout
}
