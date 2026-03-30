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

// SyncParams holds dependencies for syncing ZIM archives from a Kiwix catalog.
type SyncParams struct {
	ServiceID int64
	Entries   []CatalogEntry
	Repo      ArchiveRepo
	Logger    *slog.Logger
}

// Sync reconciles catalog entries with the database. Search indexing is handled
// automatically by PostgreSQL GENERATED columns on the zim_archives table.
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
