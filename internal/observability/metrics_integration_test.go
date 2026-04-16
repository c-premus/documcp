//go:build integration

package observability_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/c-premus/documcp/internal/database"
	"github.com/c-premus/documcp/internal/observability"
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

func TestRegisterDocumentCount_ReflectsActualCount(t *testing.T) {
	ctx := context.Background()

	// Ensure a clean documents table.
	if _, err := testPool.Exec(ctx, "TRUNCATE TABLE documents CASCADE"); err != nil {
		t.Fatalf("truncating documents: %v", err)
	}

	// Swap the default Prometheus registerer/gatherer to an isolated registry
	// so RegisterDocumentCount (which uses prometheus.MustRegister) does not
	// pollute or conflict with the global registry.
	origReg := prometheus.DefaultRegisterer
	origGath := prometheus.DefaultGatherer

	reg := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = reg
	prometheus.DefaultGatherer = reg

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = origReg
		prometheus.DefaultGatherer = origGath
	})

	observability.RegisterDocumentCount(testPool)

	// --- Zero documents -------------------------------------------------------
	count := gatherDocumentCount(t, reg)
	if count != 0 {
		t.Fatalf("expected 0 documents, got %v", count)
	}

	// --- Insert three documents -----------------------------------------------
	for i := 1; i <= 3; i++ {
		insertDocument(ctx, t, fmt.Sprintf("doc-%d", i), fmt.Sprintf("Title %d", i))
	}

	count = gatherDocumentCount(t, reg)
	if count != 3 {
		t.Fatalf("expected 3 documents, got %v", count)
	}

	// --- Soft-delete one document ---------------------------------------------
	if _, err := testPool.Exec(ctx,
		`UPDATE documents SET deleted_at = NOW() WHERE uuid = $1`,
		testUUID("doc-2"),
	); err != nil {
		t.Fatalf("soft-deleting document: %v", err)
	}

	count = gatherDocumentCount(t, reg)
	if count != 2 {
		t.Fatalf("expected 2 documents after soft-delete, got %v", count)
	}
}

// gatherDocumentCount collects metrics from the registry and returns the value
// of the documcp_documents gauge, or fails the test if the metric is absent.
func gatherDocumentCount(t *testing.T, reg *prometheus.Registry) float64 {
	t.Helper()

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gathering metrics: %v", err)
	}

	for _, fam := range families {
		if fam.GetName() == "documcp_documents" {
			metrics := fam.GetMetric()
			if len(metrics) == 0 {
				t.Fatal("documcp_documents family has no metrics")
			}
			return metrics[0].GetGauge().GetValue()
		}
	}

	t.Fatal("documcp_documents metric not found in gathered families")
	return 0 // unreachable
}

// insertDocument inserts a minimal document row directly via SQL. Status is
// set explicitly because the column default ("processing") predates migration
// 000014's CHECK constraint and would violate it — production code always
// sets status explicitly via DocumentService.Create, so the default is dead.
func insertDocument(ctx context.Context, t *testing.T, seed, title string) {
	t.Helper()

	const q = `INSERT INTO documents (uuid, title, content, file_type, file_path, file_size, mime_type, is_public, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())`

	if _, err := testPool.Exec(ctx, q,
		testUUID(seed), title, "test content", "markdown", "/test/"+seed+".md", 12, "text/markdown", true, "indexed",
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
