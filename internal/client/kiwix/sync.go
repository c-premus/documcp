package kiwix

import (
	"context"
	"fmt"
	"log/slog"
)

// ArchiveRepo defines the repository methods needed for syncing ZIM archives.
type ArchiveRepo interface {
	UpsertFromCatalog(ctx context.Context, serviceID int64, entry CatalogEntry) error
	DisableOrphaned(ctx context.Context, serviceID int64, activeNames []string) (int, error)
}

// ArchiveIndexer defines the search indexer methods needed for syncing ZIM archives.
type ArchiveIndexer interface {
	IndexZimArchive(ctx context.Context, record ZimArchiveRecord) error
}

// ZimArchiveRecord is the Meilisearch-indexable record for a ZIM archive.
type ZimArchiveRecord struct {
	UUID         string   `json:"uuid"`
	Name         string   `json:"name"`
	Title        string   `json:"title"`
	Description  string   `json:"description,omitempty"`
	Language     string   `json:"language"`
	Category     string   `json:"category,omitempty"`
	Creator      string   `json:"creator,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	ArticleCount int64    `json:"article_count"`
}

// SyncParams holds dependencies for syncing ZIM archives from a Kiwix catalog.
type SyncParams struct {
	ServiceID int64
	Entries   []CatalogEntry
	Repo      ArchiveRepo
	Indexer   ArchiveIndexer
	Logger    *slog.Logger
}

// Sync reconciles catalog entries with the database and search index.
// It upserts each entry, indexes it in Meilisearch, and disables archives
// that are no longer present in the catalog.
func Sync(ctx context.Context, params SyncParams) error {
	if len(params.Entries) == 0 {
		params.Logger.Info("no catalog entries to sync")
		return nil
	}

	activeNames := make([]string, 0, len(params.Entries))
	var upsertErrors int

	for i := range params.Entries {
		entry := &params.Entries[i]
		if err := params.Repo.UpsertFromCatalog(ctx, params.ServiceID, *entry); err != nil {
			params.Logger.Error("upserting archive from catalog",
				"name", entry.Name,
				"error", err,
			)
			upsertErrors++
			continue
		}

		activeNames = append(activeNames, entry.Name)

		if params.Indexer != nil {
			record := ZimArchiveRecord{
				UUID:         entry.ID,
				Name:         entry.Name,
				Title:        entry.Title,
				Description:  entry.Description,
				Language:     entry.Language,
				Category:     entry.Category,
				Creator:      entry.Creator,
				Tags:         entry.Tags,
				ArticleCount: entry.ArticleCount,
			}
			if err := params.Indexer.IndexZimArchive(ctx, record); err != nil {
				params.Logger.Error("indexing archive in search",
					"name", entry.Name,
					"error", err,
				)
			}
		}
	}

	disabled, err := params.Repo.DisableOrphaned(ctx, params.ServiceID, activeNames)
	if err != nil {
		return fmt.Errorf("disabling orphaned archives: %w", err)
	}

	params.Logger.Info("sync completed",
		"total", len(params.Entries),
		"upserted", len(activeNames),
		"errors", upsertErrors,
		"disabled", disabled,
	)

	return nil
}
