//go:build integration

package repository

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"git.999.haus/chris/DocuMCP-go/internal/model"
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
