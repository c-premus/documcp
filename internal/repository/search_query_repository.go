package repository

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/c-premus/documcp/internal/database"
	"github.com/c-premus/documcp/internal/model"
)

// PopularQuery represents a search query with its occurrence count.
type PopularQuery struct {
	Query string `db:"query" json:"query"`
	Count int64  `db:"count" json:"count"`
}

// SearchQueryRepository handles search query persistence.
type SearchQueryRepository struct {
	db     *pgxpool.Pool
	logger *slog.Logger
}

// NewSearchQueryRepository creates a new SearchQueryRepository.
func NewSearchQueryRepository(db *pgxpool.Pool, logger *slog.Logger) *SearchQueryRepository {
	return &SearchQueryRepository{db: db, logger: logger}
}

// Create inserts a new search query record and sets the generated ID.
func (r *SearchQueryRepository) Create(ctx context.Context, sq *model.SearchQuery) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO search_queries (user_id, query, results_count, filters, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW()) RETURNING id`,
		sq.UserID, sq.Query, sq.ResultsCount, sq.Filters,
	).Scan(&sq.ID)
	if err != nil {
		return fmt.Errorf("creating search query: %w", err)
	}
	return nil
}

// PopularQueries returns the most frequent search queries.
func (r *SearchQueryRepository) PopularQueries(ctx context.Context, limit int) ([]PopularQuery, error) {
	queries, err := database.Select[PopularQuery](ctx, r.db,
		`SELECT LOWER(query) AS query, COUNT(*) AS count
		FROM search_queries
		GROUP BY LOWER(query)
		ORDER BY count DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("querying popular searches: %w", err)
	}
	return queries, nil
}
