package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/c-premus/documcp/internal/database"
	"github.com/c-premus/documcp/internal/model"
	"github.com/c-premus/documcp/internal/stringutil"
)

// ZimArchiveUpsert holds the fields needed to upsert a ZIM archive from a catalog sync.
type ZimArchiveUpsert struct {
	Name             string
	Title            string
	Description      string
	Language         string
	Category         string
	Creator          string
	Publisher        string
	Favicon          string
	ArticleCount     int64
	MediaCount       int64
	FileSize         int64
	Tags             []string
	HasFulltextIndex bool
}

// ZimArchiveRepository handles ZIM archive persistence.
type ZimArchiveRepository struct {
	db     *pgxpool.Pool
	logger *slog.Logger
}

// NewZimArchiveRepository creates a new ZimArchiveRepository.
func NewZimArchiveRepository(db *pgxpool.Pool, logger *slog.Logger) *ZimArchiveRepository {
	return &ZimArchiveRepository{db: db, logger: logger}
}

// List returns enabled ZIM archives plus the pre-LIMIT total in a single
// round trip using COUNT(*) OVER (). Optional filters are bound as typed
// NULL when absent (the ::text cast lets PostgreSQL compare against a NULL
// parameter). Callers that pass limit == 0 see no LIMIT clause via NULLIF,
// preserving the "zero means unlimited" contract that the integration tests
// depend on.
func (r *ZimArchiveRepository) List(ctx context.Context, category, language, query string, limit, offset int) ([]model.ZimArchive, int, error) {
	categoryArg := nullStr(category)
	languageArg := nullStr(language)
	queryArg := nullLikePattern(query)

	rows, err := database.Select[zimArchiveListRow](ctx, r.db,
		`SELECT *, COUNT(*) OVER () AS total FROM zim_archives
		WHERE is_enabled = true
		  AND ($1::text IS NULL OR category = $1)
		  AND ($2::text IS NULL OR language = $2)
		  AND ($3::text IS NULL OR name ILIKE $3 OR title ILIKE $3)
		ORDER BY name
		LIMIT NULLIF($4::bigint, 0) OFFSET $5`,
		categoryArg, languageArg, queryArg, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("listing zim archives: %w", err)
	}
	archives := make([]model.ZimArchive, len(rows))
	var total int
	for i := range rows {
		archives[i] = rows[i].ZimArchive
		if i == 0 {
			total = int(rows[i].Total)
		}
	}
	return archives, total, nil
}

// zimArchiveListRow extends model.ZimArchive with the windowed COUNT(*) OVER ()
// total so a single scan yields both the page and the true filtered total.
type zimArchiveListRow struct {
	model.ZimArchive
	Total int64 `db:"total"`
}

// nullLikePattern returns a `%escaped%` ILIKE pattern, or nil when the input
// is empty so the bound parameter is NULL and the predicate skips.
func nullLikePattern(s string) *string {
	if s == "" {
		return nil
	}
	pat := "%" + escapeLike(s) + "%"
	return &pat
}

// ListSearchable returns all enabled and searchable ZIM archives, ordered by
// article count descending. Capped at maxUnboundedList rows. Used by unified
// search to determine which archives participate in federated Kiwix fan-out.
func (r *ZimArchiveRepository) ListSearchable(ctx context.Context) ([]model.ZimArchive, error) {
	archives, err := database.Select[model.ZimArchive](ctx, r.db,
		`SELECT * FROM zim_archives WHERE is_enabled = true AND is_searchable = true
		ORDER BY article_count DESC LIMIT $1`, maxUnboundedList)
	if err != nil {
		return nil, fmt.Errorf("listing searchable zim archives: %w", err)
	}
	return archives, nil
}

// FindByName returns a ZIM archive by its name, if enabled.
func (r *ZimArchiveRepository) FindByName(ctx context.Context, name string) (*model.ZimArchive, error) {
	archive, err := database.Get[model.ZimArchive](ctx, r.db,
		`SELECT * FROM zim_archives WHERE name = $1 AND is_enabled = true`, name)
	if err != nil {
		return nil, fmt.Errorf("finding zim archive by name %s: %w", name, err)
	}
	return &archive, nil
}

// FindByUUID returns a ZIM archive by its UUID.
func (r *ZimArchiveRepository) FindByUUID(ctx context.Context, uuid string) (*model.ZimArchive, error) {
	archive, err := database.Get[model.ZimArchive](ctx, r.db,
		`SELECT * FROM zim_archives WHERE uuid = $1`, uuid)
	if err != nil {
		return nil, fmt.Errorf("finding zim archive by uuid %s: %w", uuid, err)
	}
	return &archive, nil
}

// FindDisabled returns all disabled ZIM archives, capped at maxUnboundedList.
func (r *ZimArchiveRepository) FindDisabled(ctx context.Context) ([]model.ZimArchive, error) {
	archives, err := database.Select[model.ZimArchive](ctx, r.db,
		`SELECT * FROM zim_archives WHERE is_enabled = false LIMIT $1`, maxUnboundedList)
	if err != nil {
		return nil, fmt.Errorf("finding disabled zim archives: %w", err)
	}
	return archives, nil
}

// ListAll returns all ZIM archives (including disabled) with optional search query.
func (r *ZimArchiveRepository) ListAll(ctx context.Context, query string, limit int) ([]model.ZimArchive, error) {
	var conditions []string
	var args []any
	argIdx := 1

	if query != "" {
		conditions = append(conditions, fmt.Sprintf("(name ILIKE $%d OR title ILIKE $%d)", argIdx, argIdx+1))
		likeQuery := "%" + escapeLike(query) + "%"
		args = append(args, likeQuery, likeQuery)
		argIdx += 2
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	q := `SELECT * FROM zim_archives` + whereClause + ` ORDER BY name`
	if limit > 0 {
		q += fmt.Sprintf(` LIMIT $%d`, argIdx)
		args = append(args, limit)
	}

	archives, err := database.Select[model.ZimArchive](ctx, r.db, q, args...)
	if err != nil {
		return nil, fmt.Errorf("listing all zim archives: %w", err)
	}
	return archives, nil
}

// ListAllUUIDs returns ZIM archive UUIDs, capped at maxUnboundedList rows.
// Used by the reconciliation worker; callers that need more must paginate.
// The cap matches the pattern on other internal reconciliation list methods.
func (r *ZimArchiveRepository) ListAllUUIDs(ctx context.Context) ([]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT uuid FROM zim_archives LIMIT $1`, maxUnboundedList)
	if err != nil {
		return nil, fmt.Errorf("listing all zim archive uuids: %w", err)
	}
	uuids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("listing all zim archive uuids: %w", err)
	}
	return uuids, nil
}

// FindByUUIDs returns ZIM archives matching the given UUIDs.
// Used by search index reconciliation to re-index missing entries.
func (r *ZimArchiveRepository) FindByUUIDs(ctx context.Context, uuids []string) ([]model.ZimArchive, error) {
	if len(uuids) == 0 {
		return nil, nil
	}
	archives, err := database.Select[model.ZimArchive](ctx, r.db,
		`SELECT * FROM zim_archives WHERE uuid = ANY($1)`, uuids)
	if err != nil {
		return nil, fmt.Errorf("finding zim archives by uuids: %w", err)
	}
	return archives, nil
}

// Count returns the total number of ZIM archives.
func (r *ZimArchiveRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM zim_archives`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting zim archives: %w", err)
	}
	return count, nil
}

// ToggleEnabled toggles the is_enabled flag for a ZIM archive.
func (r *ZimArchiveRepository) ToggleEnabled(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx,
		`UPDATE zim_archives SET is_enabled = NOT is_enabled, updated_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("toggling enabled for zim archive %d: %w", id, err)
	}
	return nil
}

// ToggleSearchable toggles the is_searchable flag for a ZIM archive.
func (r *ZimArchiveRepository) ToggleSearchable(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx,
		`UPDATE zim_archives SET is_searchable = NOT is_searchable, updated_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("toggling searchable for zim archive %d: %w", id, err)
	}
	return nil
}

// UpsertFromCatalog inserts or updates a ZIM archive from a catalog sync entry.
// On conflict by name, it updates the mutable fields and sets last_synced_at.
func (r *ZimArchiveRepository) UpsertFromCatalog(ctx context.Context, serviceID int64, entry ZimArchiveUpsert) error {
	var tagsJSON []byte
	if len(entry.Tags) > 0 {
		b, err := json.Marshal(entry.Tags)
		if err != nil {
			return fmt.Errorf("marshaling tags for zim archive %q: %w", entry.Name, err)
		}
		tagsJSON = b
	}

	_, err := r.db.Exec(ctx,
		`INSERT INTO zim_archives (
			uuid, name, slug, title, description, language, category,
			creator, publisher, favicon, article_count, media_count,
			file_size, tags, has_fulltext_index, external_service_id, is_enabled,
			last_synced_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11,
			$12, $13, $14, $15, true,
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
			has_fulltext_index = EXCLUDED.has_fulltext_index,
			external_service_id = EXCLUDED.external_service_id,
			last_synced_at = NOW(),
			updated_at = NOW()`,
		entry.Name, stringutil.Slugify(entry.Name), entry.Title, nullStr(entry.Description),
		nullStr(entry.Language), nullStr(entry.Category),
		nullStr(entry.Creator), nullStr(entry.Publisher), nullStr(entry.Favicon),
		entry.ArticleCount, entry.MediaCount,
		entry.FileSize, tagsJSON, entry.HasFulltextIndex, serviceID,
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
		result, err := r.db.Exec(ctx,
			`UPDATE zim_archives SET is_enabled = false, updated_at = NOW()
			WHERE external_service_id = $1 AND is_enabled = true`, serviceID)
		if err != nil {
			return 0, fmt.Errorf("disabling all orphaned zim archives for service %d: %w", serviceID, err)
		}
		return int(result.RowsAffected()), nil
	}

	result, err := r.db.Exec(ctx,
		`UPDATE zim_archives SET is_enabled = false, updated_at = NOW()
		WHERE external_service_id = $1 AND is_enabled = true AND NOT (name = ANY($2))`,
		serviceID, activeNames)
	if err != nil {
		return 0, fmt.Errorf("disabling orphaned zim archives for service %d: %w", serviceID, err)
	}

	return int(result.RowsAffected()), nil
}

// FindUUIDsByExternalServiceID returns the UUIDs of all ZIM archives
// associated with the given external service, capped at maxUnboundedList. Used
// for index cleanup on service deletion.
func (r *ZimArchiveRepository) FindUUIDsByExternalServiceID(ctx context.Context, serviceID int64) ([]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT uuid FROM zim_archives WHERE external_service_id = $1 LIMIT $2`,
		serviceID, maxUnboundedList)
	if err != nil {
		return nil, fmt.Errorf("finding ZIM archive UUIDs for service %d: %w", serviceID, err)
	}
	uuids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("finding ZIM archive UUIDs for service %d: %w", serviceID, err)
	}
	return uuids, nil
}

// nullStr returns a pointer to s if non-empty, nil otherwise. Used for nullable columns.
func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
