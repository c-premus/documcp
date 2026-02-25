package repository

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jmoiron/sqlx"

	"git.999.haus/chris/DocuMCP-go/internal/model"
)

// ZimArchiveRepository handles ZIM archive persistence.
type ZimArchiveRepository struct {
	db     *sqlx.DB
	logger *slog.Logger
}

// NewZimArchiveRepository creates a new ZimArchiveRepository.
func NewZimArchiveRepository(db *sqlx.DB, logger *slog.Logger) *ZimArchiveRepository {
	return &ZimArchiveRepository{db: db, logger: logger}
}

// List returns enabled ZIM archives with optional filtering by category, language, and search query.
func (r *ZimArchiveRepository) List(ctx context.Context, category, language, query string, limit int) ([]model.ZimArchive, error) {
	q := `SELECT * FROM zim_archives WHERE is_enabled = true`
	args := []any{}
	argIdx := 1

	if category != "" {
		q += fmt.Sprintf(` AND category = $%d`, argIdx)
		args = append(args, category)
		argIdx++
	}

	if language != "" {
		q += fmt.Sprintf(` AND language = $%d`, argIdx)
		args = append(args, language)
		argIdx++
	}

	if query != "" {
		q += fmt.Sprintf(` AND (name ILIKE $%d OR title ILIKE $%d)`, argIdx, argIdx+1)
		likeQuery := "%" + query + "%"
		args = append(args, likeQuery, likeQuery)
		argIdx += 2
	}

	q += ` ORDER BY name`

	if limit > 0 {
		q += fmt.Sprintf(` LIMIT $%d`, argIdx)
		args = append(args, limit)
	}

	var archives []model.ZimArchive
	err := r.db.SelectContext(ctx, &archives, q, args...)
	if err != nil {
		return nil, fmt.Errorf("listing zim archives: %w", err)
	}
	return archives, nil
}

// FindByName returns a ZIM archive by its name, if enabled.
func (r *ZimArchiveRepository) FindByName(ctx context.Context, name string) (*model.ZimArchive, error) {
	var archive model.ZimArchive
	err := r.db.GetContext(ctx, &archive,
		`SELECT * FROM zim_archives WHERE name = $1 AND is_enabled = true`, name)
	if err != nil {
		return nil, fmt.Errorf("finding zim archive by name %s: %w", name, err)
	}
	return &archive, nil
}

// FindByUUID returns a ZIM archive by its UUID.
func (r *ZimArchiveRepository) FindByUUID(ctx context.Context, uuid string) (*model.ZimArchive, error) {
	var archive model.ZimArchive
	err := r.db.GetContext(ctx, &archive,
		`SELECT * FROM zim_archives WHERE uuid = $1`, uuid)
	if err != nil {
		return nil, fmt.Errorf("finding zim archive by uuid %s: %w", uuid, err)
	}
	return &archive, nil
}
