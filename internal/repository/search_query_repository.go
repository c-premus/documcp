package repository

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jmoiron/sqlx"

	"git.999.haus/chris/DocuMCP-go/internal/model"
)

// SearchQueryRepository handles search query persistence.
type SearchQueryRepository struct {
	db     *sqlx.DB
	logger *slog.Logger
}

// NewSearchQueryRepository creates a new SearchQueryRepository.
func NewSearchQueryRepository(db *sqlx.DB, logger *slog.Logger) *SearchQueryRepository {
	return &SearchQueryRepository{db: db, logger: logger}
}

// Create inserts a new search query record and sets the generated ID.
func (r *SearchQueryRepository) Create(ctx context.Context, sq *model.SearchQuery) error {
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO search_queries (user_id, query, results_count, filters, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW()) RETURNING id`,
		sq.UserID, sq.Query, sq.ResultsCount, sq.Filters,
	).Scan(&sq.ID)
	if err != nil {
		return fmt.Errorf("creating search query: %w", err)
	}
	return nil
}
