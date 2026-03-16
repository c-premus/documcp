package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

// RunMigrations runs all pending database migrations from the given directory.
// It uses a goose Provider with PostgreSQL advisory locking to prevent
// concurrent migration runs from conflicting.
func RunMigrations(db *sql.DB, migrationsDir string) error {
	sessionLocker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("creating session locker: %w", err)
	}

	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		db,
		os.DirFS(migrationsDir),
		goose.WithSessionLocker(sessionLocker),
	)
	if err != nil {
		return fmt.Errorf("creating goose provider: %w", err)
	}

	if _, err := provider.Up(context.Background()); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	return nil
}
