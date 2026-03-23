//go:build integration

package repository

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"git.999.haus/chris/DocuMCP-go/internal/database"
)

var testDB *sqlx.DB

func TestMain(m *testing.M) {
	// Skip gracefully if Docker is not available (e.g., CI without DinD).
	if _, err := exec.LookPath("docker"); err != nil {
		log.Printf("skipping integration tests: docker not found in PATH")
		os.Exit(0)
	}

	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:17-alpine",
		postgres.WithDatabase("documcp_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		log.Printf("skipping integration tests: starting postgres container: %v", err)
		os.Exit(0)
	}

	defer func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			log.Printf("terminating postgres container: %v", err)
		}
	}()

	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("getting connection string: %v", err)
	}

	testDB, err = sqlx.Connect("pgx", dsn)
	if err != nil {
		log.Fatalf("connecting to test database: %v", err)
	}
	defer func() {
		if err := testDB.Close(); err != nil {
			log.Printf("closing test database: %v", err)
		}
	}()

	if err := database.RunMigrations(testDB.DB, "../../migrations"); err != nil {
		log.Fatalf("running migrations: %v", err)
	}

	os.Exit(m.Run())
}

// truncateAll removes all data from application tables between tests for isolation.
// Tables are truncated in dependency order with CASCADE to handle foreign keys.
func truncateAll(t *testing.T) {
	t.Helper()

	tables := []string{
		"document_tags",
		"document_versions",
		"documents",
		"oauth_refresh_tokens",
		"oauth_access_tokens",
		"oauth_authorization_codes",
		"oauth_device_codes",
		"oauth_clients",
		"search_queries",
		"git_template_files",
		"git_templates",
		"zim_archives",
		"external_services",
		"users",
	}

	for _, table := range tables {
		if _, err := testDB.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table)); err != nil {
			t.Fatalf("truncating table %s: %v", table, err)
		}
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// testUUID generates a deterministic UUID v5-style string from a seed.
// This satisfies PostgreSQL's uuid column type while keeping test values readable.
func testUUID(seed string) string {
	h := sha256.Sum256([]byte(seed))
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		h[0:4], h[4:6], h[6:8], h[8:10], h[10:16])
}
