// Package search provides Meilisearch integration for full-text indexing and
// querying across documents, ZIM archives, Confluence spaces, and Git templates.
package search

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/meilisearch/meilisearch-go"
)

// Index UIDs matching the PHP version.
const (
	IndexDocuments        = "documents"
	IndexZimArchives      = "zim_archives"
	IndexConfluenceSpaces = "confluence_spaces"
	IndexGitTemplates     = "git_templates"
)

// Client wraps the Meilisearch service manager and provides index setup.
type Client struct {
	ms     meilisearch.ServiceManager
	logger *slog.Logger
}

// NewClient creates a new Meilisearch client from the given host and API key.
func NewClient(host, key string, logger *slog.Logger) *Client {
	var opts []meilisearch.Option
	if key != "" {
		opts = append(opts, meilisearch.WithAPIKey(key))
	}

	return &Client{
		ms:     meilisearch.New(host, opts...),
		logger: logger,
	}
}

// ConfigureIndexes creates or updates the four indexes with their settings.
// This is safe to call on startup — Meilisearch handles idempotent settings updates.
func (c *Client) ConfigureIndexes(ctx context.Context) error {
	configs := []struct {
		uid      string
		primary  string
		settings *meilisearch.Settings
	}{
		{
			uid:     IndexDocuments,
			primary: "uuid",
			settings: &meilisearch.Settings{
				SearchableAttributes: []string{"title", "description", "content", "tags"},
				FilterableAttributes: []string{
					"uuid", "file_type", "status", "user_id",
					"is_public", "tags", "created_at", "updated_at",
					"__soft_deleted",
				},
				SortableAttributes: []string{"created_at", "updated_at", "word_count"},
				StopWords:          []string{"the", "a", "an", "and", "or", "but"},
				Synonyms: map[string][]string{
					"php": {"hypertext-preprocessor"},
					"js":  {"javascript"},
					"ts":  {"typescript"},
				},
			},
		},
		{
			uid:     IndexZimArchives,
			primary: "uuid",
			settings: &meilisearch.Settings{
				SearchableAttributes: []string{"title", "name", "description", "creator", "tags"},
				FilterableAttributes: []string{
					"uuid", "language", "category", "creator", "tags", "article_count",
				},
				SortableAttributes: []string{"title", "article_count"},
			},
		},
		{
			uid:     IndexConfluenceSpaces,
			primary: "uuid",
			settings: &meilisearch.Settings{
				SearchableAttributes: []string{"name", "key", "description"},
				FilterableAttributes: []string{
					"uuid", "key", "type", "status",
					"external_service_id", "is_enabled", "__soft_deleted",
				},
				SortableAttributes: []string{"name", "key"},
			},
		},
		{
			uid:     IndexGitTemplates,
			primary: "uuid",
			settings: &meilisearch.Settings{
				SearchableAttributes: []string{"name", "description", "readme_content", "category", "tags"},
				FilterableAttributes: []string{
					"uuid", "slug", "category", "user_id",
					"is_public", "status", "__soft_deleted",
				},
				SortableAttributes: []string{"name", "created_at"},
			},
		},
	}

	for _, cfg := range configs {
		task, err := c.ms.CreateIndexWithContext(ctx, &meilisearch.IndexConfig{
			Uid:        cfg.uid,
			PrimaryKey: cfg.primary,
		})
		if err != nil {
			// Index may already exist — that's fine.
			c.logger.Debug("index creation (may already exist)", "index", cfg.uid, "error", err)
		} else {
			c.logger.Info("index created", "index", cfg.uid, "task_uid", task.TaskUID)
		}

		idx := c.ms.Index(cfg.uid)
		settingsTask, err := idx.UpdateSettingsWithContext(ctx, cfg.settings)
		if err != nil {
			return fmt.Errorf("configuring index %q settings: %w", cfg.uid, err)
		}
		c.logger.Info("index settings updated", "index", cfg.uid, "task_uid", settingsTask.TaskUID)
	}

	return nil
}

// ServiceManager returns the underlying meilisearch.ServiceManager for
// direct access when needed.
func (c *Client) ServiceManager() meilisearch.ServiceManager {
	return c.ms
}

// Healthy checks whether Meilisearch is reachable and healthy.
func (c *Client) Healthy() bool {
	return c.ms.IsHealthy()
}
