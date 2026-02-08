package filesystem

import (
	"context"
	"fmt"
	"time"

	"github.com/meilisearch/meilisearch-go"
	"github.com/rs/zerolog"
)

// Indexer handles Meilisearch indexing operations.
type Indexer struct {
	client    meilisearch.ServiceManager
	config    *MeilisearchConfig
	logger    *zerolog.Logger
	indexName string
	index     meilisearch.IndexManager
}

// NewIndexer creates a new indexer.
func NewIndexer(config *MeilisearchConfig, logger *zerolog.Logger) (*Indexer, error) {
	// Create Meilisearch client
	var client meilisearch.ServiceManager
	if config.APIKey != "" {
		client = meilisearch.New(config.URL, meilisearch.WithAPIKey(config.APIKey))
	} else {
		client = meilisearch.New(config.URL)
	}

	indexer := &Indexer{
		client:    client,
		config:    config,
		logger:    logger,
		indexName: config.IndexName,
	}

	// Initialize index
	if err := indexer.initIndex(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to initialize index: %w", err)
	}

	return indexer, nil
}

// initIndex initializes the Meilisearch index with proper settings.
func (i *Indexer) initIndex(ctx context.Context) error {
	// Get or create index
	_, err := i.client.GetIndex(i.indexName)
	if err != nil {
		// Index doesn't exist, create it
		task, err := i.client.CreateIndex(&meilisearch.IndexConfig{
			Uid:        i.indexName,
			PrimaryKey: "id",
		})
		if err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}

		// Wait for task to complete
		if _, err := i.waitForTask(task.TaskUID); err != nil {
			return fmt.Errorf("failed to wait for index creation: %w", err)
		}
	}

	// Get the index
	i.index = i.client.Index(i.indexName)

	// Update index settings
	if err := i.updateIndexSettings(ctx); err != nil {
		return fmt.Errorf("failed to update index settings: %w", err)
	}

	return nil
}

// updateIndexSettings configures the index with searchable, filterable, and sortable fields.
func (i *Indexer) updateIndexSettings(ctx context.Context) error {
	settings := &meilisearch.Settings{
		SearchableAttributes: []string{
			"name",
			"path",
			"search_text",
		},
		FilterableAttributes: []string{
			"extension",
			"mime_type",
			"is_dir",
			"parent_path",
			"size",
			"modified",
		},
		SortableAttributes: []string{
			"name",
			"size",
			"modified",
		},
		DisplayedAttributes: []string{
			"id",
			"path",
			"name",
			"extension",
			"mime_type",
			"size",
			"modified",
			"is_dir",
			"parent_path",
		},
		RankingRules: []string{
			"words",
			"typo",
			"proximity",
			"attribute",
			"sort",
			"exactness",
		},
	}

	task, err := i.index.UpdateSettings(settings)
	if err != nil {
		return fmt.Errorf("failed to update settings: %w", err)
	}

	// Wait for task to complete
	if _, err := i.waitForTask(task.TaskUID); err != nil {
		return fmt.Errorf("failed to wait for settings update: %w", err)
	}

	i.logger.Info().Msg("Meilisearch index settings updated")
	return nil
}

// IndexFiles indexes a batch of files.
func (i *Indexer) IndexFiles(ctx context.Context, files []*File) error {
	if len(files) == 0 {
		return nil
	}

	// Convert files to indexed documents
	documents := make([]IndexedFile, len(files))
	for idx, file := range files {
		documents[idx] = file.ToIndexedFile()
	}

	// Index documents in batches
	batchSize := 1000
	for start := 0; start < len(documents); start += batchSize {
		end := start + batchSize
		if end > len(documents) {
			end = len(documents)
		}

		batch := documents[start:end]

		// Convert to []interface{} for Meilisearch
		batchInterface := make([]interface{}, len(batch))
		for idx, doc := range batch {
			batchInterface[idx] = doc
		}

		task, err := i.index.UpdateDocuments(batchInterface)
		if err != nil {
			return fmt.Errorf("failed to index batch: %w", err)
		}

		// Wait for batch to complete
		if _, err := i.waitForTask(task.TaskUID); err != nil {
			return fmt.Errorf("failed to wait for batch indexing: %w", err)
		}
	}

	i.logger.Info().
		Int("count", len(documents)).
		Msg("Indexed files")

	return nil
}

// DeleteFiles removes documents from the index by their IDs.
func (i *Indexer) DeleteFiles(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	task, err := i.index.DeleteDocuments(ids)
	if err != nil {
		return fmt.Errorf("failed to delete documents: %w", err)
	}

	// Wait for task to complete
	if _, err := i.waitForTask(task.TaskUID); err != nil {
		return fmt.Errorf("failed to wait for deletion: %w", err)
	}

	i.logger.Info().
		Int("count", len(ids)).
		Msg("Deleted files from index")

	return nil
}

// ClearIndex removes all documents from the index.
func (i *Indexer) ClearIndex(ctx context.Context) error {
	task, err := i.index.DeleteAllDocuments()
	if err != nil {
		return fmt.Errorf("failed to clear index: %w", err)
	}

	// Wait for task to complete
	if _, err := i.waitForTask(task.TaskUID); err != nil {
		return fmt.Errorf("failed to wait for clear: %w", err)
	}

	i.logger.Info().Msg("Cleared index")
	return nil
}

// GetStats returns index statistics.
func (i *Indexer) GetStats(ctx context.Context) (*meilisearch.StatsIndex, error) {
	stats, err := i.index.GetStats()
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}
	return stats, nil
}

// waitForTask waits for a Meilisearch task to complete.
func (i *Indexer) waitForTask(taskUID int64) (*meilisearch.Task, error) {
	// Use the built-in WaitForTask with a timeout
	task, err := i.client.WaitForTask(taskUID, 100*time.Millisecond)
	if err != nil {
		return nil, fmt.Errorf("task failed: %w", err)
	}

	if task.Status != meilisearch.TaskStatusSucceeded {
		return nil, fmt.Errorf("task failed: %s", task.Error)
	}

	return task, nil
}

// IsHealthy checks if Meilisearch is healthy.
func (i *Indexer) IsHealthy(ctx context.Context) bool {
	// Try to get index stats
	_, err := i.GetStats(ctx)
	return err == nil
}

// GetDocumentCount returns the number of documents in the index.
func (i *Indexer) GetDocumentCount(ctx context.Context) (int64, error) {
	stats, err := i.GetStats(ctx)
	if err != nil {
		return 0, err
	}
	return stats.NumberOfDocuments, nil
}
