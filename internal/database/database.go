// Package database provides PostgreSQL connection pooling and migration support.
package database

import (
	"database/sql"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

// SQLDBFromPool wraps a pgxpool.Pool as a *sql.DB for use with libraries
// that require database/sql (e.g., goose migrations). The returned *sql.DB
// shares the underlying pool and should be closed when no longer needed.
func SQLDBFromPool(pool *pgxpool.Pool) *sql.DB {
	return stdlib.OpenDBFromPool(pool)
}
