package repository

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jmoiron/sqlx"

	"git.999.haus/chris/DocuMCP-go/internal/model"
)

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
