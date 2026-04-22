//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/c-premus/documcp/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchQueryRepository_PopularQueries(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewSearchQueryRepository(testPool, testutil.DiscardLogger())

	// Insert search queries with varying frequencies and casing via direct SQL.
	queries := []struct {
		query string
		count int
	}{
		{"golang", 3},
		{"Golang", 1}, // should merge with "golang" (case-insensitive)
		{"docker", 2},
		{"kubernetes", 1},
	}

	for _, q := range queries {
		for range q.count {
			_, err := testPool.Exec(ctx,
				`INSERT INTO search_queries (query, results_count, created_at, updated_at)
				VALUES ($1, $2, NOW(), NOW())`,
				q.query, 10,
			)
			require.NoError(t, err)
		}
	}

	t.Run("groups case-insensitively and orders by count descending", func(t *testing.T) {
		results, err := repo.PopularQueries(ctx, 10)
		require.NoError(t, err)
		require.Len(t, results, 3, "should have 3 distinct queries (golang, docker, kubernetes)")

		// "golang" (3 + 1 = 4 occurrences) should be first.
		assert.Equal(t, "golang", results[0].Query)
		assert.Equal(t, int64(4), results[0].Count)

		// "docker" (2 occurrences) should be second.
		assert.Equal(t, "docker", results[1].Query)
		assert.Equal(t, int64(2), results[1].Count)

		// "kubernetes" (1 occurrence) should be third.
		assert.Equal(t, "kubernetes", results[2].Query)
		assert.Equal(t, int64(1), results[2].Count)
	})

	t.Run("respects limit parameter", func(t *testing.T) {
		results, err := repo.PopularQueries(ctx, 2)
		require.NoError(t, err)
		assert.Len(t, results, 2)
		assert.Equal(t, "golang", results[0].Query)
		assert.Equal(t, "docker", results[1].Query)
	})

	t.Run("empty table returns empty slice", func(t *testing.T) {
		truncateAll(t)
		results, err := repo.PopularQueries(ctx, 10)
		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestSearchQueryRepository_DeleteOlderThan(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewSearchQueryRepository(testPool, testutil.DiscardLogger())

	// Seed three rows: one 100 days old, one 30 days old, one brand new.
	// created_at is set explicitly so the test doesn't depend on wall-clock drift.
	rows := []struct {
		query string
		age   time.Duration
	}{
		{"old-query", 100 * 24 * time.Hour},
		{"mid-query", 30 * 24 * time.Hour},
		{"fresh-query", 0},
	}
	for _, r := range rows {
		createdAt := time.Now().Add(-r.age)
		_, err := testPool.Exec(ctx,
			`INSERT INTO search_queries (query, results_count, created_at, updated_at)
			VALUES ($1, 0, $2, $2)`,
			r.query, createdAt,
		)
		require.NoError(t, err)
	}

	t.Run("deletes rows older than cutoff, keeps fresher rows", func(t *testing.T) {
		deleted, err := repo.DeleteOlderThan(ctx, 90*24*time.Hour)
		require.NoError(t, err)
		assert.Equal(t, int64(1), deleted, "only the 100-day-old row should be deleted")

		var remaining int
		err = testPool.QueryRow(ctx, `SELECT COUNT(*) FROM search_queries`).Scan(&remaining)
		require.NoError(t, err)
		assert.Equal(t, 2, remaining)
	})

	t.Run("aggressive retention deletes more rows", func(t *testing.T) {
		deleted, err := repo.DeleteOlderThan(ctx, 7*24*time.Hour)
		require.NoError(t, err)
		assert.Equal(t, int64(1), deleted, "the 30-day-old row should now be deleted")

		var remaining int
		err = testPool.QueryRow(ctx, `SELECT COUNT(*) FROM search_queries`).Scan(&remaining)
		require.NoError(t, err)
		assert.Equal(t, 1, remaining, "only the fresh row should remain")
	})

	t.Run("no-op when every row is fresher than cutoff", func(t *testing.T) {
		deleted, err := repo.DeleteOlderThan(ctx, 365*24*time.Hour)
		require.NoError(t, err)
		assert.Equal(t, int64(0), deleted)
	})
}
