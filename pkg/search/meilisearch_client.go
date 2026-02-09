package search

import (
	"context"
	"fmt"
	"time"

	"github.com/meilisearch/meilisearch-go"
	"github.com/rs/zerolog"
)

// Client wraps the Meilisearch client with mifind-specific configuration.
type Client struct {
	client   meilisearch.ServiceManager
	logger   *zerolog.Logger
	indexUID string
	index    meilisearch.IndexManager
}

// NewClient creates a new Meilisearch client wrapper.
func NewClient(url, indexUID, apiKey string, logger *zerolog.Logger) (*Client, error) {
	if url == "" {
		return nil, fmt.Errorf("meilisearch URL is required")
	}
	if indexUID == "" {
		return nil, fmt.Errorf("index UID is required")
	}

	var client meilisearch.ServiceManager
	if apiKey != "" {
		client = meilisearch.New(url, meilisearch.WithAPIKey(apiKey))
	} else {
		client = meilisearch.New(url)
	}

	c := &Client{
		client:   client,
		logger:   logger,
		indexUID: indexUID,
		index:    client.Index(indexUID),
	}

	// Initialize index
	if err := c.initIndex(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to initialize index: %w", err)
	}

	return c, nil
}

// initIndex initializes the Meilisearch index with proper settings.
func (c *Client) initIndex(ctx context.Context) error {
	// Get or create index
	_, err := c.client.GetIndex(c.indexUID)
	if err != nil {
		// Index doesn't exist, create it
		task, err := c.client.CreateIndex(&meilisearch.IndexConfig{
			Uid:        c.indexUID,
			PrimaryKey: "id",
		})
		if err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}

		// Wait for task to complete
		if _, err := c.waitForTask(task.TaskUID); err != nil {
			return fmt.Errorf("failed to wait for index creation: %w", err)
		}
	}

	// Update index settings
	if err := c.updateIndexSettings(ctx); err != nil {
		return fmt.Errorf("failed to update index settings: %w", err)
	}

	return nil
}

// updateIndexSettings configures the index with searchable, filterable, and sortable fields.
func (c *Client) updateIndexSettings(ctx context.Context) error {
	settings := &meilisearch.Settings{
		SearchableAttributes: []string{
			"title",
			"description",
			"search_tokens",
		},
		FilterableAttributes: []string{
			"type",
			"provider",
			"timestamp",
		},
		SortableAttributes: []string{
			"timestamp",
			"provider_score",
		},
		RankingRules: []string{
			"provider_score:desc",
			"timestamp:desc",
			"words",
			"typo",
			"proximity",
			"exactness",
		},
	}

	task, err := c.index.UpdateSettings(settings)
	if err != nil {
		return fmt.Errorf("failed to update settings: %w", err)
	}

	// Wait for task to complete
	if _, err := c.waitForTask(task.TaskUID); err != nil {
		return fmt.Errorf("failed to wait for settings update: %w", err)
	}

	c.logger.Info().Str("index", c.indexUID).Msg("Meilisearch index settings updated")
	return nil
}

// UpdateDocuments upserts documents into the index.
func (c *Client) UpdateDocuments(documents []map[string]any) error {
	if len(documents) == 0 {
		return nil
	}

	// Convert to []interface{} for Meilisearch
	docsInterface := make([]interface{}, len(documents))
	for idx, doc := range documents {
		docsInterface[idx] = doc
	}

	task, err := c.index.UpdateDocuments(docsInterface)
	if err != nil {
		return fmt.Errorf("failed to update documents: %w", err)
	}

	// Don't wait for update to complete (async)
	c.logger.Debug().Int("count", len(documents)).Int64("task", task.TaskUID).Msg("Queued document update in Meilisearch")
	return nil
}

// Search executes a search query.
func (c *Client) Search(query string, opts *meilisearch.SearchRequest) (*meilisearch.SearchResponse, error) {
	if opts == nil {
		opts = &meilisearch.SearchRequest{}
	}

	resp, err := c.index.Search(query, opts)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	return resp, nil
}

// DeleteDocuments deletes documents by their IDs.
func (c *Client) DeleteDocuments(ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	task, err := c.index.DeleteDocuments(ids)
	if err != nil {
		return fmt.Errorf("failed to delete documents: %w", err)
	}

	c.logger.Debug().Int("count", len(ids)).Int64("task", task.TaskUID).Msg("Queued document deletion from Meilisearch")
	return nil
}

// DeleteAll deletes all documents from the index.
func (c *Client) DeleteAll() error {
	task, err := c.index.DeleteAllDocuments()
	if err != nil {
		return fmt.Errorf("failed to delete all documents: %w", err)
	}

	c.logger.Info().Int64("task", task.TaskUID).Msg("Queued delete all documents from Meilisearch")
	return nil
}

// GetStats returns statistics about the index.
func (c *Client) GetStats() (*meilisearch.StatsIndex, error) {
	stats, err := c.index.GetStats()
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}
	return stats, nil
}

// waitForTask waits for a Meilisearch task to complete.
func (c *Client) waitForTask(taskUID int64) (*meilisearch.Task, error) {
	// Use the built-in WaitForTask with a timeout
	task, err := c.client.WaitForTask(taskUID, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("task failed: %w", err)
	}

	if task.Status != meilisearch.TaskStatusSucceeded {
		return nil, fmt.Errorf("task failed: %s", task.Error)
	}

	return task, nil
}

// WaitForTask waits for a specific task to complete with custom timeout.
func (c *Client) WaitForTask(taskUID int64, timeout time.Duration) (*meilisearch.Task, error) {
	task, err := c.client.WaitForTask(taskUID, timeout)
	if err != nil {
		return nil, fmt.Errorf("task %d failed or timed out: %w", taskUID, err)
	}

	if task.Status != meilisearch.TaskStatusSucceeded {
		return nil, fmt.Errorf("task %d failed: %s", taskUID, task.Error)
	}

	return task, nil
}
