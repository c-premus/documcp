//go:build integration

package database_test

import (
	"context"
	"log"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/c-premus/documcp/internal/database"
)

var testDSN string

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

	testDSN, err = pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("getting connection string: %v", err)
	}

	os.Exit(m.Run())
}

func TestNew_Success(t *testing.T) {
	db, err := database.New(testDSN, 5, 2, 5*time.Minute)
	require.NoError(t, err)
	defer db.Close()

	assert.NoError(t, db.Ping())
	assert.Equal(t, 5, db.Stats().MaxOpenConnections)
}

func TestNew_InvalidDSN(t *testing.T) {
	_, err := database.New("postgres://invalid:invalid@localhost:1/nope?sslmode=disable", 5, 2, 5*time.Minute)
	assert.Error(t, err)
}

func TestNewPgxPool_Success(t *testing.T) {
	ctx := context.Background()

	pool, err := database.NewPgxPool(ctx, testDSN, 5, 2, 30*time.Minute, 5*time.Minute)
	require.NoError(t, err)
	defer pool.Close()

	assert.NoError(t, pool.Ping(ctx))
}

func TestNewPgxPool_InvalidDSN(t *testing.T) {
	ctx := context.Background()

	_, err := database.NewPgxPool(ctx, "postgres://invalid:invalid@localhost:1/nope?sslmode=disable", 5, 2, 30*time.Minute, 5*time.Minute)
	assert.Error(t, err)
}

func TestRunMigrations_Success(t *testing.T) {
	db, err := database.New(testDSN, 5, 2, 5*time.Minute)
	require.NoError(t, err)
	defer db.Close()

	err = database.RunMigrations(db.DB, "../../migrations")
	require.NoError(t, err)

	// Verify application tables exist after migration.
	_, err = db.Exec("SELECT 1 FROM documents LIMIT 0")
	assert.NoError(t, err)

	_, err = db.Exec("SELECT 1 FROM users LIMIT 0")
	assert.NoError(t, err)
}

func TestRunMigrations_InvalidDir(t *testing.T) {
	db, err := database.New(testDSN, 5, 2, 5*time.Minute)
	require.NoError(t, err)
	defer db.Close()

	err = database.RunMigrations(db.DB, "/nonexistent/migrations")
	assert.Error(t, err)
}

func TestRunMigrations_Idempotent(t *testing.T) {
	db, err := database.New(testDSN, 5, 2, 5*time.Minute)
	require.NoError(t, err)
	defer db.Close()

	err = database.RunMigrations(db.DB, "../../migrations")
	require.NoError(t, err)

	// Running migrations a second time should succeed (no-op).
	err = database.RunMigrations(db.DB, "../../migrations")
	assert.NoError(t, err)
}

func TestRunRiverMigrations_Success(t *testing.T) {
	ctx := context.Background()

	// Set up sqlx DB and run goose migrations first (River depends on schema).
	db, err := database.New(testDSN, 5, 2, 5*time.Minute)
	require.NoError(t, err)
	defer db.Close()

	err = database.RunMigrations(db.DB, "../../migrations")
	require.NoError(t, err)

	// Create pgxpool for River migrations.
	pool, err := database.NewPgxPool(ctx, testDSN, 5, 2, 30*time.Minute, 5*time.Minute)
	require.NoError(t, err)
	defer pool.Close()

	err = database.RunRiverMigrations(ctx, pool)
	require.NoError(t, err)

	// Verify River's internal table exists.
	_, err = db.Exec("SELECT 1 FROM river_migration LIMIT 0")
	assert.NoError(t, err)
}
