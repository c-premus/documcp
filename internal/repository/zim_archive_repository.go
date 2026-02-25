package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jmoiron/sqlx"

	"git.999.haus/chris/DocuMCP-go/internal/model"
)

// ZimArchiveUpsert holds the fields needed to upsert a ZIM archive from a catalog sync.
type ZimArchiveUpsert struct {
	Name         string
	Title        string
	Description  string
	Language     string
	Category     string
	Creator      string
	Publisher    string
	Favicon      string
	ArticleCount int64
	MediaCount   int64
	FileSize     int64
	Tags         []string
}

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

// ListAll returns all ZIM archives (including disabled) with optional search query.
func (r *ZimArchiveRepository) ListAll(ctx context.Context, query string, limit int) ([]model.ZimArchive, error) {
	q := `SELECT * FROM zim_archives WHERE 1=1`
	args := []any{}
	argIdx := 1

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
		return nil, fmt.Errorf("listing all zim archives: %w", err)
	}
	return archives, nil
}

// Count returns the total number of ZIM archives.
func (r *ZimArchiveRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM zim_archives`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting zim archives: %w", err)
	}
	return count, nil
}

// ToggleEnabled toggles the is_enabled flag for a ZIM archive.
func (r *ZimArchiveRepository) ToggleEnabled(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE zim_archives SET is_enabled = NOT is_enabled, updated_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("toggling enabled for zim archive %d: %w", id, err)
	}
	return nil
}

// ToggleSearchable toggles the is_searchable flag for a ZIM archive.
func (r *ZimArchiveRepository) ToggleSearchable(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE zim_archives SET is_searchable = NOT is_searchable, updated_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("toggling searchable for zim archive %d: %w", id, err)
	}
	return nil
}

// UpsertFromCatalog inserts or updates a ZIM archive from a catalog sync entry.
// On conflict by name, it updates the mutable fields and sets last_synced_at.
func (r *ZimArchiveRepository) UpsertFromCatalog(ctx context.Context, serviceID int64, entry ZimArchiveUpsert) error {
	var tagsJSON *string
	if len(entry.Tags) > 0 {
		b, err := json.Marshal(entry.Tags)
		if err != nil {
			return fmt.Errorf("marshalling tags for zim archive %q: %w", entry.Name, err)
		}
		s := string(b)
		tagsJSON = &s
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO zim_archives (
			uuid, name, slug, title, description, language, category,
			creator, publisher, favicon, article_count, media_count,
			file_size, tags, external_service_id, is_enabled,
			last_synced_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11,
			$12, $13, $14, true,
			NOW(), NOW(), NOW()
		)
		ON CONFLICT (name) DO UPDATE SET
			title = EXCLUDED.title,
			description = EXCLUDED.description,
			language = EXCLUDED.language,
			category = EXCLUDED.category,
			creator = EXCLUDED.creator,
			publisher = EXCLUDED.publisher,
			favicon = EXCLUDED.favicon,
			article_count = EXCLUDED.article_count,
			media_count = EXCLUDED.media_count,
			file_size = EXCLUDED.file_size,
			tags = EXCLUDED.tags,
			external_service_id = EXCLUDED.external_service_id,
			last_synced_at = NOW(),
			updated_at = NOW()`,
		entry.Name, slugifyName(entry.Name), entry.Title, nullStr(entry.Description),
		nullStr(entry.Language), nullStr(entry.Category),
		nullStr(entry.Creator), nullStr(entry.Publisher), nullStr(entry.Favicon),
		entry.ArticleCount, entry.MediaCount,
		entry.FileSize, tagsJSON, serviceID,
	)
	if err != nil {
		return fmt.Errorf("upserting zim archive %q: %w", entry.Name, err)
	}
	return nil
}

// DisableOrphaned disables ZIM archives belonging to the given service that are
// not in the activeNames list. Returns the number of rows affected.
func (r *ZimArchiveRepository) DisableOrphaned(ctx context.Context, serviceID int64, activeNames []string) (int, error) {
	if len(activeNames) == 0 {
		// Disable all archives for this service.
		result, err := r.db.ExecContext(ctx,
			`UPDATE zim_archives SET is_enabled = false, updated_at = NOW()
			WHERE external_service_id = $1 AND is_enabled = true`, serviceID)
		if err != nil {
			return 0, fmt.Errorf("disabling all orphaned zim archives for service %d: %w", serviceID, err)
		}
		n, _ := result.RowsAffected()
		return int(n), nil
	}

	query, args, err := sqlx.In(
		`UPDATE zim_archives SET is_enabled = false, updated_at = NOW()
		WHERE external_service_id = ? AND is_enabled = true AND name NOT IN (?)`,
		serviceID, activeNames)
	if err != nil {
		return 0, fmt.Errorf("building IN clause for zim archive orphan check: %w", err)
	}

	query = r.db.Rebind(query)

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("disabling orphaned zim archives for service %d: %w", serviceID, err)
	}

	n, _ := result.RowsAffected()
	return int(n), nil
}

// nullStr returns a pointer to s if non-empty, nil otherwise. Used for nullable columns.
func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// slugifyName converts a name to a URL-friendly slug.
func slugifyName(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		if r == ' ' || r == '-' || r == '_' {
			return '-'
		}
		return -1
	}, s)

	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")

	return s
}
