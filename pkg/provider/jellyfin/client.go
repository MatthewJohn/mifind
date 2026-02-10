package jellyfin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client is an HTTP client for the Jellyfin API.
type Client struct {
	baseURL    string
	apiKey     string
	userID     string
	deviceID   string
	httpClient *http.Client
}

// NewClient creates a new Jellyfin API client.
func NewClient(baseURL, apiKey, userID string) *Client {
	return &Client{
		baseURL:  baseURL,
		apiKey:   apiKey,
		userID:   userID,
		deviceID: "mifind-" + apiKey[:8], // Use part of API key as device ID
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Health checks if the Jellyfin server is accessible.
func (c *Client) Health(ctx context.Context) error {
	req, err := c.newRequest(ctx, "GET", "/Users/Me", nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed: status %d", resp.StatusCode)
	}

	return nil
}

// GetCurrentUser retrieves the current user information.
func (c *Client) GetCurrentUser(ctx context.Context) (*UserResponse, error) {
	req, err := c.newRequest(ctx, "GET", "/Users/Me", nil)
	if err != nil {
		return nil, err
	}

	var result UserResponse
	if err := c.doRequest(req, &result); err != nil {
		return nil, fmt.Errorf("get current user failed: %w", err)
	}

	return &result, nil
}

// GetItems retrieves items based on a query.
func (c *Client) GetItems(ctx context.Context, params GetItemsParams) (*ItemsResponse, error) {
	req, err := c.newRequest(ctx, "GET", "/Items", nil)
	if err != nil {
		return nil, err
	}

	// Add query parameters
	q := req.URL.Query()
	if params.SearchTerm != "" {
		q.Add("searchTerm", params.SearchTerm)
	}
	if len(params.IncludeItemTypes) > 0 {
		for _, t := range params.IncludeItemTypes {
			q.Add("includeItemTypes", t)
		}
	}
	if len(params.Genres) > 0 {
		q.Add("genres", joinPipe(params.Genres))
	}
	if len(params.Studios) > 0 {
		q.Add("studios", joinPipe(params.Studios))
	}
	if len(params.Years) > 0 {
		q.Add("years", joinComma(params.Years))
	}
	if params.MinCommunityRating > 0 {
		q.Add("minCommunityRating", fmt.Sprintf("%.1f", params.MinCommunityRating))
	}
	if params.MaxOfficialRating != "" {
		q.Add("maxOfficialRating", params.MaxOfficialRating)
	}
	if params.MinPremiereDate != "" {
		q.Add("minPremiereDate", params.MinPremiereDate)
	}
	if params.MaxPremiereDate != "" {
		q.Add("maxPremiereDate", params.MaxPremiereDate)
	}
	if params.Limit > 0 {
		q.Add("limit", fmt.Sprintf("%d", params.Limit))
	}
	if params.StartIndex > 0 {
		q.Add("startIndex", fmt.Sprintf("%d", params.StartIndex))
	}
	if params.SortBy != "" {
		q.Add("sortBy", params.SortBy)
	}
	if params.SortOrder != "" {
		q.Add("sortOrder", params.SortOrder)
	}
	if params.Recursive {
		q.Add("recursive", "true")
	}
	if params.ParentID != "" {
		q.Add("parentId", params.ParentID)
	}
	req.URL.RawQuery = q.Encode()

	var result ItemsResponse
	if err := c.doRequest(req, &result); err != nil {
		return nil, fmt.Errorf("get items failed: %w", err)
	}

	return &result, nil
}

// GetItem retrieves a single item by ID.
func (c *Client) GetItem(ctx context.Context, itemID string) (*Item, error) {
	path := fmt.Sprintf("/Users/%s/Items/%s", c.userID, itemID)
	req, err := c.newRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result Item
	if err := c.doRequest(req, &result); err != nil {
		return nil, fmt.Errorf("get item failed: %w", err)
	}

	return &result, nil
}

// GetGenres retrieves all genres for the user's library.
func (c *Client) GetGenres(ctx context.Context) ([]GenreItem, error) {
	path := "/Genres"
	req, err := c.newRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("userId", c.userID)
	q.Add("sortBy", "SortName")
	q.Add("sortOrder", "Ascending")
	req.URL.RawQuery = q.Encode()

	var result GenreItemsResponse
	if err := c.doRequest(req, &result); err != nil {
		return nil, fmt.Errorf("get genres failed: %w", err)
	}

	return result.Items, nil
}

// GetStudios retrieves all studios for the user's library.
func (c *Client) GetStudios(ctx context.Context) ([]StudioItem, error) {
	path := "/Studios"
	req, err := c.newRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("userId", c.userID)
	q.Add("sortBy", "SortName")
	q.Add("sortOrder", "Ascending")
	req.URL.RawQuery = q.Encode()

	var result StudioItemsResponse
	if err := c.doRequest(req, &result); err != nil {
		return nil, fmt.Errorf("get studios failed: %w", err)
	}

	return result.Items, nil
}

// GetSeasons retrieves seasons for a TV series.
func (c *Client) GetSeasons(ctx context.Context, seriesID string) (*ItemsResponse, error) {
	path := fmt.Sprintf("/Shows/%s/Seasons", seriesID)
	req, err := c.newRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("userId", c.userID)
	req.URL.RawQuery = q.Encode()

	var result ItemsResponse
	if err := c.doRequest(req, &result); err != nil {
		return nil, fmt.Errorf("get seasons failed: %w", err)
	}

	return &result, nil
}

// GetEpisodes retrieves episodes for a season.
func (c *Client) GetEpisodes(ctx context.Context, seriesID, seasonID string) (*ItemsResponse, error) {
	path := fmt.Sprintf("/Shows/%s/Episodes", seriesID)
	req, err := c.newRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("userId", c.userID)
	q.Add("seasonId", seasonID)
	req.URL.RawQuery = q.Encode()

	var result ItemsResponse
	if err := c.doRequest(req, &result); err != nil {
		return nil, fmt.Errorf("get episodes failed: %w", err)
	}

	return &result, nil
}

// GetItemsParams represents query parameters for the Items endpoint.
type GetItemsParams struct {
	SearchTerm         string
	IncludeItemTypes   []string // Movie, Series, Season, Episode
	Genres             []string
	Studios            []string
	Years              []string
	MinCommunityRating float64
	MaxOfficialRating  string
	MinPremiereDate    string
	MaxPremiereDate    string
	Limit              int
	StartIndex         int
	SortBy             string
	SortOrder          string
	Recursive          bool
	ParentID           string
}

// newRequest creates a new HTTP request with Jellyfin authentication headers.
func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	// Build URL
	baseURL, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	// Ensure path starts with /
	if path[0] != '/' {
		path = "/" + path
	}

	reqURL := baseURL.ResolveReference(&url.URL{Path: path})

	req, err := http.NewRequestWithContext(ctx, method, reqURL.String(), body)
	if err != nil {
		return nil, err
	}

	// Jellyfin uses custom X-Emby-Authorization header
	// Format: MediaBrowser Client="client", Device="device", DeviceId="deviceId", Version="version", Token="token"
	authHeader := fmt.Sprintf(
		`MediaBrowser Client="mifind", Device="mifind", DeviceId="%s", Version="1.0.0", Token="%s"`,
		c.deviceID,
		c.apiKey,
	)
	req.Header.Set("X-Emby-Authorization", authHeader)
	req.Header.Set("Accept", "application/json")

	return req, nil
}

// doRequest executes an HTTP request and decodes the JSON response.
func (c *Client) doRequest(req *http.Request, result any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed: status %d: %s", resp.StatusCode, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("decode response failed: %w", err)
	}

	return nil
}

// joinPipe joins strings with pipe delimiter (for genres, studios).
func joinPipe(items []string) string {
	result := ""
	for i, item := range items {
		if i > 0 {
			result += "|"
		}
		result += item
	}
	return result
}

// joinComma joins strings with comma delimiter (for years).
func joinComma(items []string) string {
	result := ""
	for i, item := range items {
		if i > 0 {
			result += ","
		}
		result += item
	}
	return result
}
