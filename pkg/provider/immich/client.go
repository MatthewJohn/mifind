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

	"github.com/rs/zerolog"
)

// Client is an HTTP client for the Immich API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	logger     *zerolog.Logger
}

// NewClient creates a new Immich API client.
// If insecureSkipVerify is true, TLS certificate verification will be skipped.
func NewClient(baseURL, apiKey string, insecureSkipVerify bool) *Client {
	return NewClientWithLogger(baseURL, apiKey, insecureSkipVerify, nil)
}

// NewClientWithLogger creates a new Immich API client with debug logging.
func NewClientWithLogger(baseURL, apiKey string, insecureSkipVerify bool, logger *zerolog.Logger) *Client {
	client := &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		logger:  logger,
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
// Note: The Immich search API only returns assets and albums, not people.
func (c *Client) Search(ctx context.Context, query string, limit int) (*SearchResponse, error) {
	return c.SearchWithFilters(ctx, query, limit, nil, "", "", "", "")
}

// SearchWithFilters performs a search query with filters for people, locations, etc.
// Uses smaller default limits (25 for text, 100 for filter-only) to avoid timeouts.
func (c *Client) SearchWithFilters(ctx context.Context, query string, limit int, peopleIDs []string, country, state, city, albumID string) (*SearchResponse, error) {
	// Determine default size based on search type
	defaultSize := 100
	if query != "" {
		defaultSize = 25 // Text searches are slower
	}

	return c.doSearchRequest(ctx, query, limit, defaultSize, peopleIDs, country, state, city, albumID, false)
}

// doSearchRequest is the internal search implementation that all search methods use.
// endpoint: "smart" for text search, "metadata" for filter-only
// defaultSize: used when limit < 1
// withExif: if true, only returns assets with EXIF data (used for filter values)
func (c *Client) doSearchRequest(ctx context.Context, query string, limit, defaultSize int, peopleIDs []string, country, state, city, albumID string, withExif bool) (*SearchResponse, error) {
	if c.logger != nil {
		c.logger.Debug().
			Str("query", query).
			Int("limit", limit).
			Int("defaultSize", defaultSize).
			Bool("withExif", withExif).
			Strs("people", peopleIDs).
			Str("country", country).
			Str("state", state).
			Str("city", city).
			Str("album", albumID).
			Msg("Immich: Search request")
	}

	// Use /api/search/smart for text queries, /api/search/metadata for filter-only searches
	// /api/search/smart includes exifInfo and supports text search
	// /api/search/metadata includes exifInfo but does NOT support text query
	var endpoint string
	if query != "" {
		endpoint = "smart"
	} else {
		endpoint = "metadata"
	}
	url := fmt.Sprintf("%s/api/search/%s", c.baseURL, endpoint)

	// Ensure size is at least 1 (Immich API requires size >= 1)
	searchSize := limit
	if searchSize < 1 {
		searchSize = defaultSize
	}

	// Build search request body
	reqBody := map[string]any{
		"page":      1,
		"size":      searchSize,
		"isVisible": true, // Only search visible assets
	}

	// Add query (only for /api/search/smart)
	if query != "" {
		reqBody["query"] = query
	}

	// Add withExif flag for filter value searches
	if withExif {
		reqBody["withExif"] = true
	}

	// Add people filter if specified
	if len(peopleIDs) > 0 {
		reqBody["personIds"] = peopleIDs
	}

	// Add location filter if specified
	if country != "" {
		reqBody["country"] = country
	}
	if state != "" {
		reqBody["state"] = state
	}
	if city != "" {
		reqBody["city"] = city
	}

	// Add album filter if specified
	if albumID != "" {
		reqBody["albumId"] = albumID
	}

	if c.logger != nil {
		c.logger.Debug().
			RawJSON("body", mustMarshalJSON(reqBody)).
			Msg("Immich: Search request body")
	}

	var result SearchResponse
	if err := c.doPost(ctx, url, reqBody, &result); err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	if c.logger != nil {
		assetCount := 0
		if result.Assets != nil {
			assetCount = len(result.Assets.Items)
		}
		c.logger.Debug().
			Int("assets", assetCount).
			Msg("Immich: Search response")
	}

	return &result, nil
}

// GetCitySearch performs a search to get assets with location data for filter values.
// Uses /api/search/cities endpoint which returns representative assets for each unique city.
// The response format is a simple array of assets, not a SearchResponse.
func (c *Client) GetCitySearch(ctx context.Context) ([]Asset, error) {
	url := fmt.Sprintf("%s/api/search/cities", c.baseURL)

	var result []Asset
	if err := c.doGet(ctx, url, &result); err != nil {
		return nil, fmt.Errorf("get city search failed: %w", err)
	}

	return result, nil
}

// mustMarshalJSON marshals to JSON or panics.
func mustMarshalJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
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

	if c.logger != nil {
		c.logger.Debug().
			Int("limit", limit).
			Int("size", size).
			Msg("Immich: ListPeople request")
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

	if c.logger != nil {
		c.logger.Debug().
			Int("total", result.Total).
			Int("count", result.Count).
			Int("returned", len(result.Items)).
			Msg("Immich: ListPeople response")
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

// GetPersonAssets retrieves all assets for a person.
func (c *Client) GetPersonAssets(ctx context.Context, personID string, limit int) ([]Asset, error) {
	url := fmt.Sprintf("%s/api/people/%s/assets", c.baseURL, personID)

	if c.logger != nil {
		c.logger.Debug().
			Str("person_id", personID).
			Int("limit", limit).
			Msg("Immich: GetPersonAssets request")
	}

	var result []Asset
	if err := c.doGet(ctx, url, &result); err != nil {
		return nil, fmt.Errorf("get person assets failed: %w", err)
	}

	// Apply limit if specified
	if limit > 0 && len(result) > limit {
		return result[:limit], nil
	}

	if c.logger != nil {
		c.logger.Debug().
			Int("count", len(result)).
			Msg("Immich: GetPersonAssets response")
	}

	return result, nil
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
	if c.logger != nil {
		c.logger.Debug().
			Str("method", req.Method).
			Str("url", req.URL.String()).
			Msg("Immich: HTTP request")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if c.logger != nil {
			c.logger.Error().Err(err).Msg("Immich: HTTP request failed")
		}
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if c.logger != nil {
		c.logger.Debug().
			Int("status", resp.StatusCode).
			Str("content_type", resp.Header.Get("Content-Type")).
			Msg("Immich: HTTP response")
	}

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		if c.logger != nil {
			c.logger.Error().
				Int("status", resp.StatusCode).
				Str("body", string(body)).
				Msg("Immich: HTTP error response")
		}
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	// Decode response
	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			if c.logger != nil {
				c.logger.Error().Err(err).Msg("Immich: Failed to decode response")
			}
			return fmt.Errorf("failed to decode response: %w", err)
		}

		if c.logger != nil {
			c.logger.Debug().
				RawJSON("response", mustMarshalJSON(result)).
				Msg("Immich: Parsed response")
		}
	}

	return nil
}

// SetTimeout sets the HTTP client timeout.
func (c *Client) SetTimeout(timeout time.Duration) {
	c.httpClient.Timeout = timeout
}
