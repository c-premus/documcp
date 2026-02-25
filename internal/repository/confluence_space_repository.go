package repository

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jmoiron/sqlx"

	"git.999.haus/chris/DocuMCP-go/internal/model"
)

// ConfluenceSpaceUpsert holds the fields needed to upsert a Confluence space from an API sync.
type ConfluenceSpaceUpsert struct {
	ConfluenceID string
	Key          string
	Name         string
	Description  string
	Type         string
	Status       string
	HomepageID   string
	IconURL      string
}

// ConfluenceSpaceRepository handles Confluence space persistence.
type ConfluenceSpaceRepository struct {
	db     *sqlx.DB
	logger *slog.Logger
}

// NewConfluenceSpaceRepository creates a new ConfluenceSpaceRepository.
func NewConfluenceSpaceRepository(db *sqlx.DB, logger *slog.Logger) *ConfluenceSpaceRepository {
	return &ConfluenceSpaceRepository{db: db, logger: logger}
}

// List returns enabled Confluence spaces with optional filtering by type and search query.
func (r *ConfluenceSpaceRepository) List(ctx context.Context, spaceType, query string, limit int) ([]model.ConfluenceSpace, error) {
	q := `SELECT * FROM confluence_spaces WHERE is_enabled = true`
	args := []any{}
	argIdx := 1

	if spaceType != "" {
		q += fmt.Sprintf(` AND type = $%d`, argIdx)
		args = append(args, spaceType)
		argIdx++
	}

	if query != "" {
		q += fmt.Sprintf(` AND (name ILIKE $%d OR key ILIKE $%d OR description ILIKE $%d)`, argIdx, argIdx+1, argIdx+2)
		likeQuery := "%" + query + "%"
		args = append(args, likeQuery, likeQuery, likeQuery)
		argIdx += 3
	}

	q += ` ORDER BY name`

	if limit > 0 {
		q += fmt.Sprintf(` LIMIT $%d`, argIdx)
		args = append(args, limit)
	}

	var spaces []model.ConfluenceSpace
	err := r.db.SelectContext(ctx, &spaces, q, args...)
	if err != nil {
		return nil, fmt.Errorf("listing confluence spaces: %w", err)
	}
	return spaces, nil
}

// FindByKey returns a Confluence space by its key, if enabled.
func (r *ConfluenceSpaceRepository) FindByKey(ctx context.Context, key string) (*model.ConfluenceSpace, error) {
	var space model.ConfluenceSpace
	err := r.db.GetContext(ctx, &space,
		`SELECT * FROM confluence_spaces WHERE key = $1 AND is_enabled = true`, key)
	if err != nil {
		return nil, fmt.Errorf("finding confluence space by key %s: %w", key, err)
	}
	return &space, nil
}

// FindByUUID returns a Confluence space by its UUID.
func (r *ConfluenceSpaceRepository) FindByUUID(ctx context.Context, uuid string) (*model.ConfluenceSpace, error) {
	var space model.ConfluenceSpace
	err := r.db.GetContext(ctx, &space,
		`SELECT * FROM confluence_spaces WHERE uuid = $1`, uuid)
	if err != nil {
		return nil, fmt.Errorf("finding confluence space by uuid %s: %w", uuid, err)
	}
	return &space, nil
}

// ListAll returns all Confluence spaces (including disabled) with optional search query.
func (r *ConfluenceSpaceRepository) ListAll(ctx context.Context, query string, limit int) ([]model.ConfluenceSpace, error) {
	q := `SELECT * FROM confluence_spaces WHERE 1=1`
	args := []any{}
	argIdx := 1

	if query != "" {
		q += fmt.Sprintf(` AND (name ILIKE $%d OR key ILIKE $%d)`, argIdx, argIdx+1)
		likeQuery := "%" + query + "%"
		args = append(args, likeQuery, likeQuery)
		argIdx += 2
	}

	q += ` ORDER BY name`
	if limit > 0 {
		q += fmt.Sprintf(` LIMIT $%d`, argIdx)
		args = append(args, limit)
	}

	var spaces []model.ConfluenceSpace
	err := r.db.SelectContext(ctx, &spaces, q, args...)
	if err != nil {
		return nil, fmt.Errorf("listing all confluence spaces: %w", err)
	}
	return spaces, nil
}

// Count returns the total number of Confluence spaces.
func (r *ConfluenceSpaceRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM confluence_spaces`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting confluence spaces: %w", err)
	}
	return count, nil
}

// ToggleEnabled toggles the is_enabled flag for a Confluence space.
func (r *ConfluenceSpaceRepository) ToggleEnabled(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE confluence_spaces SET is_enabled = NOT is_enabled, updated_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("toggling enabled for confluence space %d: %w", id, err)
	}
	return nil
}

// ToggleSearchable toggles the is_searchable flag for a Confluence space.
func (r *ConfluenceSpaceRepository) ToggleSearchable(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE confluence_spaces SET is_searchable = NOT is_searchable, updated_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("toggling searchable for confluence space %d: %w", id, err)
	}
	return nil
}

// UpsertFromAPI inserts or updates a Confluence space from an API sync.
// On conflict by key, it updates the mutable fields and sets last_synced_at.
func (r *ConfluenceSpaceRepository) UpsertFromAPI(ctx context.Context, serviceID int64, space ConfluenceSpaceUpsert) error {
	spaceType := space.Type
	if spaceType == "" {
		spaceType = "global"
	}
	spaceStatus := space.Status
	if spaceStatus == "" {
		spaceStatus = "current"
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO confluence_spaces (
			uuid, confluence_id, key, name, description, type, status,
			homepage_id, icon_url, external_service_id, is_enabled,
			last_synced_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, $2, $3, $4, $5, $6,
			$7, $8, $9, true,
			NOW(), NOW(), NOW()
		)
		ON CONFLICT (key) DO UPDATE SET
			confluence_id = EXCLUDED.confluence_id,
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			type = EXCLUDED.type,
			status = EXCLUDED.status,
			homepage_id = EXCLUDED.homepage_id,
			icon_url = EXCLUDED.icon_url,
			external_service_id = EXCLUDED.external_service_id,
			last_synced_at = NOW(),
			updated_at = NOW()`,
		space.ConfluenceID, space.Key, space.Name, nullableStr(space.Description),
		spaceType, spaceStatus,
		nullableStr(space.HomepageID), nullableStr(space.IconURL), serviceID,
	)
	if err != nil {
		return fmt.Errorf("upserting confluence space %q: %w", space.Key, err)
	}
	return nil
}

// DisableOrphaned disables Confluence spaces belonging to the given service that
// are not in the activeKeys list. Returns the number of rows affected.
func (r *ConfluenceSpaceRepository) DisableOrphaned(ctx context.Context, serviceID int64, activeKeys []string) (int, error) {
	if len(activeKeys) == 0 {
		result, err := r.db.ExecContext(ctx,
			`UPDATE confluence_spaces SET is_enabled = false, updated_at = NOW()
			WHERE external_service_id = $1 AND is_enabled = true`, serviceID)
		if err != nil {
			return 0, fmt.Errorf("disabling all orphaned confluence spaces for service %d: %w", serviceID, err)
		}
		n, _ := result.RowsAffected()
		return int(n), nil
	}

	query, args, err := sqlx.In(
		`UPDATE confluence_spaces SET is_enabled = false, updated_at = NOW()
		WHERE external_service_id = ? AND is_enabled = true AND key NOT IN (?)`,
		serviceID, activeKeys)
	if err != nil {
		return 0, fmt.Errorf("building IN clause for confluence space orphan check: %w", err)
	}

	query = r.db.Rebind(query)

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("disabling orphaned confluence spaces for service %d: %w", serviceID, err)
	}

	n, _ := result.RowsAffected()
	return int(n), nil
}

// nullableStr returns a pointer to s if non-empty, nil otherwise. Used for nullable columns.
func nullableStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
