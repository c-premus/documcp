package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/c-premus/documcp/internal/database"
	"github.com/c-premus/documcp/internal/model"
)

// DocumentFilePath is a lightweight struct for file cleanup operations.
type DocumentFilePath struct {
	ID       int64  `db:"id"`
	UUID     string `db:"uuid"`
	FilePath string `db:"file_path"`
}

// TitleSuggestion is returned by autocomplete queries.
type TitleSuggestion struct {
	UUID  string `db:"uuid"`
	Title string `db:"title"`
}

// DocumentRepository handles document persistence.
type DocumentRepository struct {
	db     *pgxpool.Pool
	logger *slog.Logger
}

// NewDocumentRepository creates a new DocumentRepository.
func NewDocumentRepository(db *pgxpool.Pool, logger *slog.Logger) *DocumentRepository {
	return &DocumentRepository{db: db, logger: logger}
}

// FindByUUID returns a document by its UUID, excluding soft-deleted records.
func (r *DocumentRepository) FindByUUID(ctx context.Context, uuid string) (*model.Document, error) {
	doc, err := database.Get[model.Document](ctx, r.db,
		`SELECT * FROM documents WHERE uuid = $1 AND deleted_at IS NULL`, uuid)
	if err != nil {
		return nil, fmt.Errorf("finding document by uuid %s: %w", uuid, err)
	}
	return &doc, nil
}

// FindByID returns a document by its primary key.
func (r *DocumentRepository) FindByID(ctx context.Context, id int64) (*model.Document, error) {
	doc, err := database.Get[model.Document](ctx, r.db,
		`SELECT * FROM documents WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return nil, fmt.Errorf("finding document by id %d: %w", id, err)
	}
	return &doc, nil
}

// Create inserts a new document and sets the generated ID on doc.
func (r *DocumentRepository) Create(ctx context.Context, doc *model.Document) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO documents (
			uuid, title, description, file_type, file_path, file_size,
			mime_type, url, content, content_hash, metadata, processed_at,
			word_count, user_id, is_public, status, error_message,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12,
			$13, $14, $15, $16, $17,
			NOW(), NOW()
		) RETURNING id`,
		doc.UUID, doc.Title, doc.Description, doc.FileType, doc.FilePath, doc.FileSize,
		doc.MIMEType, doc.URL, doc.Content, doc.ContentHash, doc.Metadata, doc.ProcessedAt,
		doc.WordCount, doc.UserID, doc.IsPublic, doc.Status, doc.ErrorMessage,
	).Scan(&doc.ID)
	if err != nil {
		return fmt.Errorf("creating document %q: %w", doc.Title, err)
	}
	return nil
}

// Update updates an existing document by its ID.
func (r *DocumentRepository) Update(ctx context.Context, doc *model.Document) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE documents SET
			title = $1, description = $2, file_type = $3, file_path = $4,
			file_size = $5, mime_type = $6, url = $7, content = $8,
			content_hash = $9, metadata = $10, processed_at = $11,
			word_count = $12, user_id = $13, is_public = $14, status = $15,
			error_message = $16, updated_at = NOW()
		WHERE id = $17 AND deleted_at IS NULL`,
		doc.Title, doc.Description, doc.FileType, doc.FilePath,
		doc.FileSize, doc.MIMEType, doc.URL, doc.Content,
		doc.ContentHash, doc.Metadata, doc.ProcessedAt,
		doc.WordCount, doc.UserID, doc.IsPublic, doc.Status,
		doc.ErrorMessage, doc.ID,
	)
	if err != nil {
		return fmt.Errorf("updating document %d: %w", doc.ID, err)
	}
	if tag.RowsAffected() == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// SoftDelete sets deleted_at on a document.
func (r *DocumentRepository) SoftDelete(ctx context.Context, id int64) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE documents SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("soft deleting document %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// TagsForDocument returns all tags associated with a document.
func (r *DocumentRepository) TagsForDocument(ctx context.Context, documentID int64) ([]model.DocumentTag, error) {
	tags, err := database.Select[model.DocumentTag](ctx, r.db,
		`SELECT * FROM document_tags WHERE document_id = $1 ORDER BY tag`, documentID)
	if err != nil {
		return nil, fmt.Errorf("finding tags for document %d: %w", documentID, err)
	}
	return tags, nil
}

// TagsForDocuments returns tags for multiple documents in a single query,
// grouped by document ID. Documents with no tags are absent from the map.
func (r *DocumentRepository) TagsForDocuments(ctx context.Context, documentIDs []int64) (map[int64][]model.DocumentTag, error) {
	if len(documentIDs) == 0 {
		return map[int64][]model.DocumentTag{}, nil
	}
	tags, err := database.Select[model.DocumentTag](ctx, r.db,
		`SELECT * FROM document_tags WHERE document_id = ANY($1) ORDER BY document_id, tag`, documentIDs)
	if err != nil {
		return nil, fmt.Errorf("finding tags for %d documents: %w", len(documentIDs), err)
	}
	result := make(map[int64][]model.DocumentTag, len(documentIDs))
	for _, tag := range tags {
		result[tag.DocumentID] = append(result[tag.DocumentID], tag)
	}
	return result, nil
}

// ReplaceTags deletes existing tags, inserts new ones, and refreshes the
// denormalized documents.tags_text column — all within a single transaction.
// tags_text feeds the STORED search_vector generated column.
func (r *DocumentRepository) ReplaceTags(ctx context.Context, documentID int64, tags []string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction for replacing tags on document %d: %w", documentID, err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op; error is irrelevant

	_, err = tx.Exec(ctx, `DELETE FROM document_tags WHERE document_id = $1`, documentID)
	if err != nil {
		return fmt.Errorf("deleting tags for document %d: %w", documentID, err)
	}

	if len(tags) > 0 {
		// Single multi-row INSERT replaces the per-tag loop: 50 tags =
		// 1 round trip instead of 50. validateTags (service layer) caps
		// count at 50 and per-tag length at 100, so the generated
		// placeholder list is bounded.
		var sb strings.Builder
		sb.WriteString(`INSERT INTO document_tags (document_id, tag, created_at, updated_at) VALUES `)
		args := make([]any, 0, len(tags)+1)
		args = append(args, documentID)
		for i, tag := range tags {
			if i > 0 {
				sb.WriteString(", ")
			}
			// $1 is documentID (bound once); $2, $3, ... are tag values.
			fmt.Fprintf(&sb, "($1, $%d, NOW(), NOW())", i+2)
			args = append(args, tag)
		}
		if _, err = tx.Exec(ctx, sb.String(), args...); err != nil {
			return fmt.Errorf("inserting %d tags for document %d: %w", len(tags), documentID, err)
		}
	}

	var tagsText sql.NullString
	if len(tags) > 0 {
		tagsText = sql.NullString{String: strings.Join(tags, " "), Valid: true}
	}
	_, err = tx.Exec(ctx,
		`UPDATE documents SET tags_text = $1, updated_at = NOW() WHERE id = $2`,
		tagsText, documentID)
	if err != nil {
		return fmt.Errorf("updating tags_text for document %d: %w", documentID, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing tags replacement for document %d: %w", documentID, err)
	}
	return nil
}

// Count returns the total number of non-deleted documents.
func (r *DocumentRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM documents WHERE deleted_at IS NULL`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting documents: %w", err)
	}
	return count, nil
}

// CreateVersion inserts a new document version and sets the generated ID.
func (r *DocumentRepository) CreateVersion(ctx context.Context, version *model.DocumentVersion) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO document_versions (document_id, version, file_path, content, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW()) RETURNING id`,
		version.DocumentID, version.Version, version.FilePath, version.Content, version.Metadata,
	).Scan(&version.ID)
	if err != nil {
		return fmt.Errorf("creating version %d for document %d: %w", version.Version, version.DocumentID, err)
	}
	return nil
}

// ListAllUUIDs returns all document UUIDs including soft-deleted ones, capped
// at maxUnboundedList rows per call. Callers that need more must paginate.
func (r *DocumentRepository) ListAllUUIDs(ctx context.Context) ([]string, error) {
	rows, err := r.db.Query(ctx, `SELECT uuid FROM documents LIMIT $1`, maxUnboundedList)
	if err != nil {
		return nil, fmt.Errorf("listing all document uuids: %w", err)
	}
	uuids, err := pgxCollectStrings(rows)
	if err != nil {
		return nil, fmt.Errorf("listing all document uuids: %w", err)
	}
	return uuids, nil
}

// FindByUUIDs returns documents matching the given UUIDs (including soft-deleted).
// Used by search index reconciliation to re-index missing entries.
func (r *DocumentRepository) FindByUUIDs(ctx context.Context, uuids []string) ([]model.Document, error) {
	if len(uuids) == 0 {
		return nil, nil
	}
	docs, err := database.Select[model.Document](ctx, r.db,
		`SELECT * FROM documents WHERE uuid = ANY($1)`, uuids)
	if err != nil {
		return nil, fmt.Errorf("finding documents by uuids: %w", err)
	}
	return docs, nil
}

// ListActiveFilePaths returns file paths for non-deleted documents, capped at
// maxUnboundedList rows. Used by the orphaned-file cleanup job.
func (r *DocumentRepository) ListActiveFilePaths(ctx context.Context) ([]DocumentFilePath, error) {
	paths, err := database.Select[DocumentFilePath](ctx, r.db,
		`SELECT id, uuid, file_path FROM documents
		WHERE deleted_at IS NULL AND file_path IS NOT NULL AND file_path != ''
		LIMIT $1`, maxUnboundedList)
	if err != nil {
		return nil, fmt.Errorf("listing active file paths: %w", err)
	}
	return paths, nil
}

// PurgeSoftDeleted deletes documents soft-deleted longer than olderThan duration.
// Returns deleted file paths for disk cleanup.
func (r *DocumentRepository) PurgeSoftDeleted(ctx context.Context, olderThan time.Duration) ([]DocumentFilePath, error) {
	cutoff := time.Now().Add(-olderThan)

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning purge soft-deleted transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op; error is irrelevant

	// 1. Select documents to purge.
	paths, err := database.Select[DocumentFilePath](ctx, tx,
		`SELECT id, uuid, file_path FROM documents
		WHERE deleted_at IS NOT NULL AND deleted_at < $1`, cutoff)
	if err != nil {
		return nil, fmt.Errorf("selecting soft-deleted documents for purge: %w", err)
	}

	if len(paths) == 0 {
		return nil, nil
	}

	ids := make([]int64, len(paths))
	for i, p := range paths {
		ids[i] = p.ID
	}

	// 2. Delete document_tags.
	if _, err = tx.Exec(ctx, `DELETE FROM document_tags WHERE document_id = ANY($1)`, ids); err != nil {
		return nil, fmt.Errorf("purging document_tags: %w", err)
	}

	// 3. Delete document_versions.
	if _, err = tx.Exec(ctx, `DELETE FROM document_versions WHERE document_id = ANY($1)`, ids); err != nil {
		return nil, fmt.Errorf("purging document_versions: %w", err)
	}

	// 4. Delete documents.
	if _, err := tx.Exec(ctx, `DELETE FROM documents WHERE id = ANY($1)`, ids); err != nil {
		return nil, fmt.Errorf("purging documents: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing purge soft-deleted transaction: %w", err)
	}

	r.logger.Info("purged soft-deleted documents", "count", len(paths))
	return paths, nil
}

// FindByUUIDIncludingDeleted returns a document by its UUID, including soft-deleted records.
func (r *DocumentRepository) FindByUUIDIncludingDeleted(ctx context.Context, uuid string) (*model.Document, error) {
	doc, err := database.Get[model.Document](ctx, r.db,
		`SELECT * FROM documents WHERE uuid = $1`, uuid)
	if err != nil {
		return nil, fmt.Errorf("finding document (including deleted) by uuid %s: %w", uuid, err)
	}
	return &doc, nil
}

// Restore clears the deleted_at timestamp on a soft-deleted document.
func (r *DocumentRepository) Restore(ctx context.Context, id int64) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE documents SET deleted_at = NULL, updated_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("restoring document %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// PurgeSingle hard-deletes a single document and its associated tags and versions.
// Returns the file_path for disk cleanup.
func (r *DocumentRepository) PurgeSingle(ctx context.Context, id int64) (string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("beginning purge single transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op; error is irrelevant

	// Get file path before deletion.
	var filePath string
	err = tx.QueryRow(ctx,
		`SELECT COALESCE(file_path, '') FROM documents WHERE id = $1`, id).Scan(&filePath)
	if err != nil {
		return "", fmt.Errorf("selecting file path for document %d: %w", id, err)
	}

	// Delete tags.
	if _, err := tx.Exec(ctx, `DELETE FROM document_tags WHERE document_id = $1`, id); err != nil {
		return "", fmt.Errorf("purging tags for document %d: %w", id, err)
	}

	// Delete versions.
	if _, err := tx.Exec(ctx, `DELETE FROM document_versions WHERE document_id = $1`, id); err != nil {
		return "", fmt.Errorf("purging versions for document %d: %w", id, err)
	}

	// Delete document.
	if _, err := tx.Exec(ctx, `DELETE FROM documents WHERE id = $1`, id); err != nil {
		return "", fmt.Errorf("purging document %d: %w", id, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("committing purge single transaction: %w", err)
	}

	r.logger.Info("purged single document", "id", id)
	return filePath, nil
}

// ListDeleted returns soft-deleted documents with pagination.
// When userID is non-nil, results are scoped to documents owned by that user.
func (r *DocumentRepository) ListDeleted(ctx context.Context, limit, offset int, userID *int64) ([]model.Document, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	where := "deleted_at IS NOT NULL"
	var args []any
	argIdx := 1

	if userID != nil {
		where += fmt.Sprintf(" AND user_id = $%d", argIdx)
		args = append(args, *userID)
		argIdx++
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM documents WHERE " + where
	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting deleted documents: %w", err)
	}

	selectQuery := fmt.Sprintf(
		`SELECT id, uuid, title, description, file_type, file_path, file_size, mime_type,
		url, content_hash, metadata, processed_at, word_count, user_id, is_public,
		status, error_message, created_at, updated_at, deleted_at
		FROM documents WHERE %s ORDER BY deleted_at DESC LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1,
	)
	args = append(args, limit, offset)

	docs, err := database.Select[model.Document](ctx, r.db, selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing deleted documents: %w", err)
	}

	return docs, total, nil
}

// ListDistinctTags returns distinct tags matching a prefix, excluding soft-deleted documents.
// When userID is non-nil, only tags from public documents or documents owned by that user are returned.
func (r *DocumentRepository) ListDistinctTags(ctx context.Context, prefix string, limit int, userID *int64) ([]string, error) {
	query := `SELECT DISTINCT dt.tag FROM document_tags dt
		JOIN documents d ON d.id = dt.document_id
		WHERE d.deleted_at IS NULL AND dt.tag ILIKE $1`
	args := []any{escapeLike(prefix) + "%"}

	if userID != nil {
		query += ` AND (d.is_public = true OR d.user_id = $3)`
		args = append(args, limit, *userID)
	} else {
		args = append(args, limit)
	}
	query += ` ORDER BY dt.tag LIMIT $2`

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing distinct tags: %w", err)
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, fmt.Errorf("scanning tag: %w", err)
		}
		tags = append(tags, tag)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating tags: %w", err)
	}
	if tags == nil {
		tags = []string{}
	}
	return tags, nil
}

// SuggestTitles returns title suggestions matching the given prefix (case-insensitive).
func (r *DocumentRepository) SuggestTitles(ctx context.Context, prefix string, limit int) ([]TitleSuggestion, error) {
	suggestions, err := database.Select[TitleSuggestion](ctx, r.db,
		`SELECT uuid, title FROM documents
		WHERE deleted_at IS NULL AND is_public = true AND title ILIKE $1
		ORDER BY title LIMIT $2`,
		escapeLike(prefix)+"%", limit)
	if err != nil {
		return nil, fmt.Errorf("suggesting titles with prefix %q: %w", prefix, err)
	}
	return suggestions, nil
}
