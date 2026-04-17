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
	"strings"
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

// TestFederatedSearch_SourceTotals is a regression guard for a double-bug
// in the api-design audit 7a closure: the SQL `source` column emits
// MCP-facing singular names ('document', 'zim_archive', 'git_template')
// while callers pass plural index UIDs ('documents', ...) via
// FederatedSearchParams.Indexes. An earlier fix keyed SourceTotals by the
// SQL column value; the handler looked up by index UID and always missed,
// producing zero totals in production even when matches existed.
//
// Fixed behavior: sqlSourceToIndex in searcher.go normalizes SQL column
// values to index UIDs before storing in SourceTotals. This test exercises
// the real SQL path to catch drift between the union clauses' `AS source`
// literals and the translation map.
func TestFederatedSearch_SourceTotals(t *testing.T) {
	ctx := context.Background()

	if _, err := testPool.Exec(ctx, "TRUNCATE TABLE documents CASCADE"); err != nil {
		t.Fatalf("truncating documents: %v", err)
	}

	// Insert three documents so a non-trivial count flows through.
	insertIndexedDocument(ctx, t, "fed-src-1", "kubernetes primer", "content one")
	insertIndexedDocument(ctx, t, "fed-src-2", "kubernetes operators", "content two")
	insertIndexedDocument(ctx, t, "fed-src-3", "kubernetes networking", "content three")

	s := search.NewSearcher(testPool, slog.Default())
	resp, err := s.FederatedSearch(ctx, search.FederatedSearchParams{
		Query:   "kubernetes",
		Indexes: []string{search.IndexDocuments}, // documents only
		Limit:   10,
		IsAdmin: true,
	})
	if err != nil {
		t.Fatalf("FederatedSearch returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("FederatedSearch returned nil response")
	}

	// The headline total should match.
	if resp.EstimatedTotal != 3 {
		t.Errorf("EstimatedTotal = %d, want 3", resp.EstimatedTotal)
	}

	// SourceTotals must be keyed by the index UID the caller passed in,
	// not the SQL column value. If a future refactor changes either side
	// (SQL `AS source` literal OR sqlSourceToIndex map) without updating
	// the other, this check flips to 0.
	got, ok := resp.SourceTotals[search.IndexDocuments]
	if !ok {
		t.Fatalf("SourceTotals missing key %q; got %v", search.IndexDocuments, resp.SourceTotals)
	}
	if got != 3 {
		t.Errorf("SourceTotals[%q] = %d, want 3", search.IndexDocuments, got)
	}
}

// TestSearch_WithSnippets is a regression guard for the wire bug where
// SearchDocumentsInput.IncludeSnippets was advertised in the contract and
// accepted by the MCP handler but never threaded through to SQL. Before the
// fix, the extra jsonb omitted the "snippet" key entirely regardless of the
// caller's request. This test exercises the real ts_headline path end to end.
func TestSearch_WithSnippets(t *testing.T) {
	ctx := context.Background()

	if _, err := testPool.Exec(ctx, "TRUNCATE TABLE documents CASCADE"); err != nil {
		t.Fatalf("truncating documents: %v", err)
	}

	// Body chosen so ts_headline has enough context to emit a fragment —
	// MinWords=5 means short bodies produce no headline.
	insertIndexedDocument(ctx, t, "snip-doc",
		"kubernetes primer",
		"This document covers kubernetes deployment strategies including rolling updates, blue-green patterns, and canary releases across multiple clusters.")

	s := search.NewSearcher(testPool, slog.Default())

	t.Run("WithSnippets=true emits a ts_headline fragment in Extra", func(t *testing.T) {
		resp, err := s.Search(ctx, search.SearchParams{
			Query:        "kubernetes",
			IndexUID:     search.IndexDocuments,
			Limit:        10,
			IsAdmin:      true,
			WithSnippets: true,
		})
		if err != nil {
			t.Fatalf("Search returned error: %v", err)
		}
		if len(resp.Hits) == 0 {
			t.Fatal("expected at least 1 hit")
		}
		snippet := search.ExtraString(resp.Hits[0].Extra, "snippet")
		if snippet == "" {
			t.Fatal("snippet empty; want ts_headline fragment")
		}
		// Markdown highlight markers prove ts_headline ran with the
		// configured StartSel/StopSel, not some default HTML path.
		if !strings.Contains(snippet, "**kubernetes**") {
			t.Errorf("snippet %q does not contain **kubernetes** highlight", snippet)
		}
	})

	t.Run("WithSnippets=false omits the snippet key entirely", func(t *testing.T) {
		resp, err := s.Search(ctx, search.SearchParams{
			Query:        "kubernetes",
			IndexUID:     search.IndexDocuments,
			Limit:        10,
			IsAdmin:      true,
			WithSnippets: false,
		})
		if err != nil {
			t.Fatalf("Search returned error: %v", err)
		}
		if len(resp.Hits) == 0 {
			t.Fatal("expected at least 1 hit")
		}
		if _, ok := resp.Hits[0].Extra["snippet"]; ok {
			t.Errorf("Extra contains snippet key when WithSnippets=false; got %q", resp.Hits[0].Extra["snippet"])
		}
	})
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
