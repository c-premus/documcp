package database

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Querier is the common interface satisfied by *pgxpool.Pool and pgx.Tx.
// It allows repository helpers to work in both transactional and non-transactional contexts.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Get executes a query that returns a single row and scans it into T.
// T must be a struct with `db` tags matching column names.
// Returns pgx.ErrNoRows (which wraps sql.ErrNoRows) if no rows found.
func Get[T any](ctx context.Context, q Querier, sql string, args ...any) (T, error) {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		var zero T
		return zero, err
	}
	return pgx.CollectOneRow(rows, pgx.RowToStructByNameLax[T])
}

// Select executes a query that returns multiple rows and scans them into a slice of T.
// T must be a struct with `db` tags matching column names.
// Returns an empty (non-nil) slice if no rows found.
func Select[T any](ctx context.Context, q Querier, sql string, args ...any) ([]T, error) {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	result, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[T])
	if err != nil {
		return nil, err
	}
	if result == nil {
		result = []T{}
	}
	return result, nil
}
