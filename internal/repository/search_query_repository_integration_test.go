//go:build integration

package repository

import (
	"context"
	"testing"

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
