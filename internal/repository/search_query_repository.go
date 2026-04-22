package repository

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/c-premus/documcp/internal/database"
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

// DeleteOlderThan removes search_queries rows with created_at older than age.
// Returns the number of rows deleted. The periodic cleanup worker uses this to
// bound table growth; retention size also bounds the aggregation scan in
// PopularQueries.
func (r *SearchQueryRepository) DeleteOlderThan(ctx context.Context, age time.Duration) (int64, error) {
	cutoff := time.Now().Add(-age)
	result, err := r.db.Exec(ctx,
		`DELETE FROM search_queries WHERE created_at < $1`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("deleting search_queries older than %s: %w", age, err)
	}
	return result.RowsAffected(), nil
}
