package repository

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jmoiron/sqlx"

	"git.999.haus/chris/DocuMCP-go/internal/model"
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
	db     *sqlx.DB
	logger *slog.Logger
}

// NewDocumentRepository creates a new DocumentRepository.
func NewDocumentRepository(db *sqlx.DB, logger *slog.Logger) *DocumentRepository {
	return &DocumentRepository{db: db, logger: logger}
}

// FindByUUID returns a document by its UUID, excluding soft-deleted records.
func (r *DocumentRepository) FindByUUID(ctx context.Context, uuid string) (*model.Document, error) {
	var doc model.Document
	err := r.db.GetContext(ctx, &doc,
		`SELECT * FROM documents WHERE uuid = $1 AND deleted_at IS NULL`, uuid)
	if err != nil {
		return nil, fmt.Errorf("finding document by uuid %s: %w", uuid, err)
	}
	return &doc, nil
}

// FindByID returns a document by its primary key.
func (r *DocumentRepository) FindByID(ctx context.Context, id int64) (*model.Document, error) {
	var doc model.Document
	err := r.db.GetContext(ctx, &doc,
		`SELECT * FROM documents WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return nil, fmt.Errorf("finding document by id %d: %w", id, err)
	}
	return &doc, nil
}

// Create inserts a new document and sets the generated ID on doc.
func (r *DocumentRepository) Create(ctx context.Context, doc *model.Document) error {
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO documents (
			uuid, title, description, file_type, file_path, file_size,
			mime_type, url, content, content_hash, metadata, processed_at,
			word_count, user_id, is_public, status, error_message,
			meilisearch_indexed_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12,
			$13, $14, $15, $16, $17,
			$18, NOW(), NOW()
		) RETURNING id`,
		doc.UUID, doc.Title, doc.Description, doc.FileType, doc.FilePath, doc.FileSize,
		doc.MIMEType, doc.URL, doc.Content, doc.ContentHash, doc.Metadata, doc.ProcessedAt,
		doc.WordCount, doc.UserID, doc.IsPublic, doc.Status, doc.ErrorMessage,
		doc.MeilisearchIndexedAt,
	).Scan(&doc.ID)
	if err != nil {
		return fmt.Errorf("creating document %q: %w", doc.Title, err)
	}
	return nil
}

// Update updates an existing document by its ID.
func (r *DocumentRepository) Update(ctx context.Context, doc *model.Document) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE documents SET
			title = $1, description = $2, file_type = $3, file_path = $4,
			file_size = $5, mime_type = $6, url = $7, content = $8,
			content_hash = $9, metadata = $10, processed_at = $11,
			word_count = $12, user_id = $13, is_public = $14, status = $15,
			error_message = $16, meilisearch_indexed_at = $17, updated_at = NOW()
		WHERE id = $18 AND deleted_at IS NULL`,
		doc.Title, doc.Description, doc.FileType, doc.FilePath,
		doc.FileSize, doc.MIMEType, doc.URL, doc.Content,
		doc.ContentHash, doc.Metadata, doc.ProcessedAt,
		doc.WordCount, doc.UserID, doc.IsPublic, doc.Status,
		doc.ErrorMessage, doc.MeilisearchIndexedAt, doc.ID,
	)
	if err != nil {
		return fmt.Errorf("updating document %d: %w", doc.ID, err)
	}
	return nil
}

// SoftDelete sets deleted_at on a document.
func (r *DocumentRepository) SoftDelete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE documents SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("soft deleting document %d: %w", id, err)
	}
	return nil
}

// TagsForDocument returns all tags associated with a document.
func (r *DocumentRepository) TagsForDocument(ctx context.Context, documentID int64) ([]model.DocumentTag, error) {
	var tags []model.DocumentTag
	err := r.db.SelectContext(ctx, &tags,
		`SELECT * FROM document_tags WHERE document_id = $1 ORDER BY tag`, documentID)
	if err != nil {
		return nil, fmt.Errorf("finding tags for document %d: %w", documentID, err)
	}
	return tags, nil
}

// ReplaceTags deletes existing tags and inserts new ones within a transaction.
func (r *DocumentRepository) ReplaceTags(ctx context.Context, documentID int64, tags []string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction for replacing tags on document %d: %w", documentID, err)
	}
	defer tx.Rollback() //nolint:errcheck

	_, err = tx.ExecContext(ctx, `DELETE FROM document_tags WHERE document_id = $1`, documentID)
	if err != nil {
		return fmt.Errorf("deleting tags for document %d: %w", documentID, err)
	}

	for _, tag := range tags {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO document_tags (document_id, tag, created_at, updated_at) VALUES ($1, $2, NOW(), NOW())`,
			documentID, tag)
		if err != nil {
			return fmt.Errorf("inserting tag %q for document %d: %w", tag, documentID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing tags replacement for document %d: %w", documentID, err)
	}
	return nil
}

// Count returns the total number of non-deleted documents.
func (r *DocumentRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents WHERE deleted_at IS NULL`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting documents: %w", err)
	}
	return count, nil
}

// CreateVersion inserts a new document version and sets the generated ID.
func (r *DocumentRepository) CreateVersion(ctx context.Context, version *model.DocumentVersion) error {
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO document_versions (document_id, version, file_path, content, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW()) RETURNING id`,
		version.DocumentID, version.Version, version.FilePath, version.Content, version.Metadata,
	).Scan(&version.ID)
	if err != nil {
		return fmt.Errorf("creating version %d for document %d: %w", version.Version, version.DocumentID, err)
	}
	return nil
}

// ListAllUUIDs returns all document UUIDs including soft-deleted ones.
func (r *DocumentRepository) ListAllUUIDs(ctx context.Context) ([]string, error) {
	var uuids []string
	err := r.db.SelectContext(ctx, &uuids, `SELECT uuid FROM documents`)
	if err != nil {
		return nil, fmt.Errorf("listing all document uuids: %w", err)
	}
	return uuids, nil
}

// ListActiveFilePaths returns file paths for non-deleted documents.
func (r *DocumentRepository) ListActiveFilePaths(ctx context.Context) ([]DocumentFilePath, error) {
	var paths []DocumentFilePath
	err := r.db.SelectContext(ctx, &paths,
		`SELECT id, uuid, file_path FROM documents
		WHERE deleted_at IS NULL AND file_path IS NOT NULL AND file_path != ''`)
	if err != nil {
		return nil, fmt.Errorf("listing active file paths: %w", err)
	}
	return paths, nil
}

// PurgeSoftDeleted deletes documents soft-deleted longer than olderThan duration.
// Returns deleted file paths for disk cleanup.
func (r *DocumentRepository) PurgeSoftDeleted(ctx context.Context, olderThan time.Duration) ([]DocumentFilePath, error) {
	cutoff := time.Now().Add(-olderThan)

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning purge soft-deleted transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// 1. Select documents to purge.
	var paths []DocumentFilePath
	err = tx.SelectContext(ctx, &paths,
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
	tagQuery, tagArgs, err := sqlx.In(`DELETE FROM document_tags WHERE document_id IN (?)`, ids)
	if err != nil {
		return nil, fmt.Errorf("building IN clause for document_tags purge: %w", err)
	}
	tagQuery = tx.Rebind(tagQuery)
	if _, err := tx.ExecContext(ctx, tagQuery, tagArgs...); err != nil {
		return nil, fmt.Errorf("purging document_tags: %w", err)
	}

	// 3. Delete document_versions.
	verQuery, verArgs, err := sqlx.In(`DELETE FROM document_versions WHERE document_id IN (?)`, ids)
	if err != nil {
		return nil, fmt.Errorf("building IN clause for document_versions purge: %w", err)
	}
	verQuery = tx.Rebind(verQuery)
	if _, err := tx.ExecContext(ctx, verQuery, verArgs...); err != nil {
		return nil, fmt.Errorf("purging document_versions: %w", err)
	}

	// 4. Delete documents.
	docQuery, docArgs, err := sqlx.In(`DELETE FROM documents WHERE id IN (?)`, ids)
	if err != nil {
		return nil, fmt.Errorf("building IN clause for documents purge: %w", err)
	}
	docQuery = tx.Rebind(docQuery)
	if _, err := tx.ExecContext(ctx, docQuery, docArgs...); err != nil {
		return nil, fmt.Errorf("purging documents: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing purge soft-deleted transaction: %w", err)
	}

	r.logger.Info("purged soft-deleted documents", "count", len(paths))
	return paths, nil
}

// SuggestTitles returns title suggestions matching the given prefix (case-insensitive).
func (r *DocumentRepository) SuggestTitles(ctx context.Context, prefix string, limit int) ([]TitleSuggestion, error) {
	var suggestions []TitleSuggestion
	err := r.db.SelectContext(ctx, &suggestions,
		`SELECT uuid, title FROM documents
		WHERE deleted_at IS NULL AND is_public = true AND title ILIKE $1
		ORDER BY title LIMIT $2`,
		prefix+"%", limit)
	if err != nil {
		return nil, fmt.Errorf("suggesting titles with prefix %q: %w", prefix, err)
	}
	return suggestions, nil
}
