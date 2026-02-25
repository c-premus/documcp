package database

import (
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
)

// RunMigrations runs all pending database migrations from the given directory.
func RunMigrations(db *sql.DB, migrationsDir string) error {
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("setting goose dialect: %w", err)
	}

	if err := goose.Up(db, migrationsDir); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	return nil
}

// RollbackMigration rolls back the most recently applied migration.
func RollbackMigration(db *sql.DB, migrationsDir string) error {
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("setting goose dialect: %w", err)
	}

	if err := goose.Down(db, migrationsDir); err != nil {
		return fmt.Errorf("rolling back migration: %w", err)
	}

	return nil
}
