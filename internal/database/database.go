// Package database provides PostgreSQL connection pooling and migration support.
package database

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// New creates a new database connection pool using pgx as the underlying driver.
// dsn is a PostgreSQL connection string, for example:
//
//	"host=localhost port=5432 dbname=documcp user=documcp password=secret sslmode=disable"
func New(dsn string, maxOpen, maxIdle int, maxLifetime time.Duration) (*sqlx.DB, error) {
	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(maxLifetime)
	db.SetConnMaxIdleTime(maxLifetime)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return db, nil
}
