//go:build integration

package search_test

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

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/c-premus/documcp/internal/database"
	"github.com/c-premus/documcp/internal/search"
)

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
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

	testPool, err = pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("connecting to test database: %v", err)
	}
	defer testPool.Close()

	sqlDB := database.SQLDBFromPool(testPool)
	defer sqlDB.Close() //nolint:errcheck // best-effort cleanup in test teardown
	if err := database.RunMigrations(sqlDB, "../../migrations"); err != nil {
		log.Fatalf("running migrations: %v", err)
	}

	os.Exit(m.Run())
}

// TestSearch_TrigramFallback is a regression guard for a double-bug in
// trigramFallbackDocuments that shipped because no test exercised the path:
// the SQL `%` operator was over-escaped (`%%%%` in Go → `%%` in SQL, no such
// operator) so any FTS-miss crashed with SQLSTATE 42883. Since the fallback
// only fires when FTS returns zero — rare in normal use — it went unnoticed
// until a misspelled query in the wild triggered it.
//
// The test inserts a document titled "authentication" and searches for the
// misspelling "authentikation". FTS cannot match (stems diverge); the
// trigram similarity operator is the only path to a hit. The test asserts
// the call returns cleanly AND that a hit is returned.
func TestSearch_TrigramFallback(t *testing.T) {
	ctx := context.Background()

	if _, err := testPool.Exec(ctx, "TRUNCATE TABLE documents CASCADE"); err != nil {
		t.Fatalf("truncating documents: %v", err)
	}

	insertIndexedDocument(ctx, t, "doc-auth", "authentication", "Body text unrelated to title matching.")

	s := search.NewSearcher(testPool, slog.Default())
	resp, err := s.Search(ctx, search.SearchParams{
		Query:    "authentikation", // misspelling — FTS misses, trigram must fire
		IndexUID: search.IndexDocuments,
		Limit:    10,
		IsAdmin:  true, // skip visibility filter
	})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("Search returned nil response")
	}
	if len(resp.Hits) == 0 {
		t.Fatal("trigram fallback returned 0 hits for near-match title; expected at least 1")
	}
	if resp.Hits[0].Title != "authentication" {
		t.Errorf("hit title = %q, want %q", resp.Hits[0].Title, "authentication")
	}
}

// insertIndexedDocument writes a minimal indexed document row. Status is set
// explicitly because the column default ("processing") predates migration
// 000014's CHECK constraint and would violate it.
func insertIndexedDocument(ctx context.Context, t *testing.T, seed, title, content string) {
	t.Helper()

	const q = `INSERT INTO documents (uuid, title, content, file_type, file_path, file_size, mime_type, is_public, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())`

	if _, err := testPool.Exec(ctx, q,
		testUUID(seed), title, content, "markdown", "/test/"+seed+".md", len(content), "text/markdown", true, "indexed",
	); err != nil {
		t.Fatalf("inserting document %q: %v", seed, err)
	}
}

// testUUID generates a deterministic UUID-shaped string from a seed.
func testUUID(seed string) string {
	h := sha256.Sum256([]byte(seed))
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		h[0:4], h[4:6], h[6:8], h[8:10], h[10:16])
}
