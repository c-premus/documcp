package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/c-premus/documcp/internal/database"
	"github.com/c-premus/documcp/internal/model"
)

// DocumentListParams holds filters and pagination for listing documents.
type DocumentListParams struct {
	FileType      string
	Status        string
	UserID        *int64
	IsPublic      *bool
	OwnerOrPublic *int64 // If set, filter: (user_id = $N OR is_public = true)
	Query         string // simple ILIKE search on title
	Limit         int
	Offset        int
	OrderBy       string // "created_at", "updated_at", "title"
	OrderDir      string // "asc" or "desc"
}

// DocumentListResult holds a paginated list of documents.
type DocumentListResult struct {
	Documents []model.Document
	Total     int
}

// List returns a filtered, paginated list of documents.
func (r *DocumentRepository) List(ctx context.Context, params DocumentListParams) (*DocumentListResult, error) {
	var conditions []string
	var args []any
	argIdx := 1

	conditions = append(conditions, "deleted_at IS NULL")

	if params.FileType != "" {
		conditions = append(conditions, fmt.Sprintf("file_type = $%d", argIdx))
		args = append(args, params.FileType)
		argIdx++
	}
	if params.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, params.Status)
		argIdx++
	}
	if params.OwnerOrPublic != nil {
		conditions = append(conditions, fmt.Sprintf("(user_id = $%d OR is_public = true)", argIdx))
		args = append(args, *params.OwnerOrPublic)
		argIdx++
	}
	if params.UserID != nil {
		conditions = append(conditions, fmt.Sprintf("user_id = $%d", argIdx))
		args = append(args, *params.UserID)
		argIdx++
	}
	if params.IsPublic != nil {
		conditions = append(conditions, fmt.Sprintf("is_public = $%d", argIdx))
		args = append(args, *params.IsPublic)
		argIdx++
	}
	if params.Query != "" {
		conditions = append(conditions, fmt.Sprintf("title ILIKE $%d", argIdx))
		args = append(args, "%"+escapeLike(params.Query)+"%")
		argIdx++
	}

	where := strings.Join(conditions, " AND ")

	// Count total matching rows.
	countQuery := "SELECT COUNT(*) FROM documents WHERE " + where
	var total int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting documents: %w", err)
	}

	// Determine ordering.
	orderBy := "created_at"
	if params.OrderBy != "" {
		switch params.OrderBy {
		case "created_at", "updated_at", "title":
			orderBy = params.OrderBy
		}
	}
	orderDir := "DESC"
	if strings.EqualFold(params.OrderDir, "asc") {
		orderDir = "ASC"
	}

	limit := params.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	selectQuery := fmt.Sprintf(
		`SELECT id, uuid, title, description, file_type, file_path, file_size, mime_type,
		url, content_hash, metadata, processed_at, word_count, user_id, is_public,
		status, error_message, created_at, updated_at, deleted_at
		FROM documents WHERE %s ORDER BY %s %s LIMIT $%d OFFSET $%d`,
		where, orderBy, orderDir, argIdx, argIdx+1,
	)
	args = append(args, limit, params.Offset)

	docs, err := database.Select[model.Document](ctx, r.db, selectQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("listing documents: %w", err)
	}

	return &DocumentListResult{
		Documents: docs,
		Total:     total,
	}, nil
}

// FindByStatus returns documents with the given status, limited to count.
func (r *DocumentRepository) FindByStatus(ctx context.Context, status string, limit int) ([]model.Document, error) {
	docs, err := database.Select[model.Document](ctx, r.db,
		`SELECT * FROM documents WHERE status = $1 AND deleted_at IS NULL ORDER BY created_at ASC LIMIT $2`,
		status, limit)
	if err != nil {
		return nil, fmt.Errorf("finding documents by status %q: %w", status, err)
	}
	return docs, nil
}

// pgxCollectStrings collects a single-column string result set from pgx.Rows.
func pgxCollectStrings(rows pgx.Rows) ([]string, error) {
	return pgx.CollectRows(rows, pgx.RowTo[string])
}
