package confluence

import (
	"context"
	"fmt"
	"log/slog"
)

// SpaceRepo defines repository methods needed for syncing Confluence spaces
// with the database.
type SpaceRepo interface {
	UpsertFromAPI(ctx context.Context, serviceID int64, space Space) error
	DisableOrphaned(ctx context.Context, serviceID int64, activeKeys []string) (int, error)
}

// SpaceIndexer defines search indexer methods needed for syncing Confluence
// spaces with the search index.
type SpaceIndexer interface {
	IndexConfluenceSpace(ctx context.Context, record ConfluenceSpaceRecord) error
}

// ConfluenceSpaceRecord is the Meilisearch-indexable record for a Confluence space.
//
//nolint:revive // exported stutter is intentional; renaming would be a breaking change
type ConfluenceSpaceRecord struct {
	UUID              string `json:"uuid"`
	ConfluenceID      string `json:"confluence_id"`
	Key               string `json:"key"`
	Name              string `json:"name"`
	Description       string `json:"description,omitempty"`
	Type              string `json:"type"`
	Status            string `json:"status,omitempty"`
	ExternalServiceID int64  `json:"external_service_id"`
	IsEnabled         bool   `json:"is_enabled"`
}

// SyncParams holds dependencies for syncing Confluence spaces.
type SyncParams struct {
	ServiceID int64
	Spaces    []Space
	Repo      SpaceRepo
	Indexer   SpaceIndexer
	Logger    *slog.Logger
}

// Sync reconciles API spaces with the database and search index.
// For each space it upserts into the database and indexes for search.
// After processing all spaces, it disables any database records that were
// not present in the API response (orphaned spaces).
func Sync(ctx context.Context, params SyncParams) error {
	logger := params.Logger
	if logger == nil {
		logger = slog.Default()
	}

	activeKeys := make([]string, 0, len(params.Spaces))

	for i := range params.Spaces {
		space := &params.Spaces[i]
		activeKeys = append(activeKeys, space.Key)

		if err := params.Repo.UpsertFromAPI(ctx, params.ServiceID, *space); err != nil {
			logger.Error("upserting confluence space",
				"key", space.Key,
				"error", err,
			)
			continue
		}

		if params.Indexer != nil {
			record := ConfluenceSpaceRecord{
				ConfluenceID:      space.ID,
				Key:               space.Key,
				Name:              space.Name,
				Description:       space.Description,
				Type:              space.Type,
				Status:            space.Status,
				ExternalServiceID: params.ServiceID,
				IsEnabled:         true,
			}

			if err := params.Indexer.IndexConfluenceSpace(ctx, record); err != nil {
				logger.Error("indexing confluence space",
					"key", space.Key,
					"error", err,
				)
				continue
			}
		}

		logger.Debug("synced confluence space", "key", space.Key, "name", space.Name)
	}

	disabled, err := params.Repo.DisableOrphaned(ctx, params.ServiceID, activeKeys)
	if err != nil {
		return fmt.Errorf("disabling orphaned spaces: %w", err)
	}

	logger.Info("confluence space sync complete",
		"service_id", params.ServiceID,
		"synced", len(params.Spaces),
		"disabled", disabled,
	)

	return nil
}
