//go:build integration

package repository

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c-premus/documcp/internal/model"
)

func TestSearchQueryRepository_Create(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewSearchQueryRepository(testDB, discardLogger())

	t.Run("null user_id", func(t *testing.T) {
		sq := &model.SearchQuery{
			Query:        "golang concurrency",
			ResultsCount: 42,
		}

		err := repo.Create(ctx, sq)
		require.NoError(t, err)
		assert.NotZero(t, sq.ID, "ID should be set after insert")
	})

	t.Run("with filters", func(t *testing.T) {
		sq := &model.SearchQuery{
			Query:        "docker networking",
			ResultsCount: 15,
			Filters:      sql.NullString{String: `{"type":"pdf","tag":"docs"}`, Valid: true},
		}

		err := repo.Create(ctx, sq)
		require.NoError(t, err)
		assert.NotZero(t, sq.ID, "ID should be set after insert")
	})

	t.Run("with user_id", func(t *testing.T) {
		// Insert a user to satisfy the FK constraint.
		var userID int64
		err := testDB.QueryRowContext(ctx,
			`INSERT INTO users (name, email, created_at, updated_at)
			VALUES ($1, $2, NOW(), NOW()) RETURNING id`,
			"Test User", "test@example.com",
		).Scan(&userID)
		require.NoError(t, err)

		sq := &model.SearchQuery{
			UserID:       sql.NullInt64{Int64: userID, Valid: true},
			Query:        "kubernetes pods",
			ResultsCount: 7,
		}

		err = repo.Create(ctx, sq)
		require.NoError(t, err)
		assert.NotZero(t, sq.ID, "ID should be set after insert")
	})

	t.Run("multiple creates get unique IDs", func(t *testing.T) {
		sq1 := &model.SearchQuery{
			Query:        "first query",
			ResultsCount: 10,
		}
		sq2 := &model.SearchQuery{
			Query:        "second query",
			ResultsCount: 20,
		}

		require.NoError(t, repo.Create(ctx, sq1))
		require.NoError(t, repo.Create(ctx, sq2))

		assert.NotZero(t, sq1.ID)
		assert.NotZero(t, sq2.ID)
		assert.NotEqual(t, sq1.ID, sq2.ID, "each insert should produce a unique ID")
	})
}

func TestSearchQueryRepository_PopularQueries(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewSearchQueryRepository(testDB, discardLogger())

	// Insert search queries with varying frequencies and casing.
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
			sq := &model.SearchQuery{
				Query:        q.query,
				ResultsCount: 10,
			}
			require.NoError(t, repo.Create(ctx, sq))
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
