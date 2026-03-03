//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"git.999.haus/chris/DocuMCP-go/internal/model"
)

// createTestExternalService inserts a minimal ExternalService for FK satisfaction.
func createTestExternalService(t *testing.T, ctx context.Context) *model.ExternalService {
	t.Helper()

	svcRepo := NewExternalServiceRepository(testDB, discardLogger())
	svc := &model.ExternalService{
		UUID:      testUUID("zim-svc-001"),
		Name:      "Test Kiwix",
		Slug:      "test-kiwix-svc",
		Type:      "kiwix",
		BaseURL:   "https://kiwix.example.com",
		Priority:  1,
		Status:    "healthy",
		IsEnabled: true,
	}
	require.NoError(t, svcRepo.Create(ctx, svc))
	return svc
}

func TestZimArchiveRepository_UpsertFromCatalog(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	svc := createTestExternalService(t, ctx)
	repo := NewZimArchiveRepository(testDB, discardLogger())

	entry := ZimArchiveUpsert{
		Name:         "Wikipedia English",
		Title:        "Wikipedia (en)",
		Description:  "The free encyclopedia",
		Language:     "en",
		Category:     "wikipedia",
		Creator:      "Wikimedia Foundation",
		Publisher:    "Kiwix",
		Favicon:      "https://kiwix.example.com/favicon.png",
		ArticleCount: 6000000,
		MediaCount:   500000,
		FileSize:     90000000000,
		Tags:         []string{"wiki", "encyclopedia"},
	}

	err := repo.UpsertFromCatalog(ctx, svc.ID, entry)
	require.NoError(t, err)

	found, err := repo.FindByName(ctx, "Wikipedia English")
	require.NoError(t, err)

	assert.Equal(t, "Wikipedia English", found.Name)
	assert.Equal(t, "Wikipedia (en)", found.Title)
	assert.Equal(t, "wikipedia-english", found.Slug)
	assert.Equal(t, "en", found.Language)
	assert.True(t, found.Description.Valid)
	assert.Equal(t, "The free encyclopedia", found.Description.String)
	assert.True(t, found.Category.Valid)
	assert.Equal(t, "wikipedia", found.Category.String)
	assert.True(t, found.Creator.Valid)
	assert.Equal(t, "Wikimedia Foundation", found.Creator.String)
	assert.True(t, found.Publisher.Valid)
	assert.Equal(t, "Kiwix", found.Publisher.String)
	assert.True(t, found.Favicon.Valid)
	assert.Equal(t, "https://kiwix.example.com/favicon.png", found.Favicon.String)
	assert.Equal(t, int64(6000000), found.ArticleCount)
	assert.Equal(t, int64(500000), found.MediaCount)
	assert.Equal(t, int64(90000000000), found.FileSize)
	assert.True(t, found.IsEnabled)
	assert.True(t, found.ExternalServiceID.Valid)
	assert.Equal(t, svc.ID, found.ExternalServiceID.Int64)
	assert.True(t, found.LastSyncedAt.Valid)
	assert.True(t, found.CreatedAt.Valid)
	assert.True(t, found.UpdatedAt.Valid)

	// Verify tags stored as JSON.
	assert.True(t, found.Tags.Valid)
	tags, err := found.ParseTags()
	require.NoError(t, err)
	assert.Equal(t, []string{"wiki", "encyclopedia"}, tags)

	t.Run("upsert existing", func(t *testing.T) {
		updatedEntry := entry
		updatedEntry.Title = "Wikipedia (en) Updated"
		updatedEntry.ArticleCount = 6500000

		err := repo.UpsertFromCatalog(ctx, svc.ID, updatedEntry)
		require.NoError(t, err)

		found, err := repo.FindByName(ctx, "Wikipedia English")
		require.NoError(t, err)

		assert.Equal(t, "Wikipedia (en) Updated", found.Title)
		assert.Equal(t, int64(6500000), found.ArticleCount)
		// Slug should remain unchanged on upsert (ON CONFLICT updates mutable fields only).
		assert.Equal(t, "wikipedia-english", found.Slug)
	})

	t.Run("slug with special characters", func(t *testing.T) {
		entry := ZimArchiveUpsert{
			Name:     "Wikivoyage — Europe (2024)",
			Title:    "Wikivoyage Europe",
			Language: "en",
		}

		err := repo.UpsertFromCatalog(ctx, svc.ID, entry)
		require.NoError(t, err)

		found, err := repo.FindByName(ctx, "Wikivoyage — Europe (2024)")
		require.NoError(t, err)

		// Special chars stripped, spaces→hyphens, no consecutive hyphens.
		assert.Equal(t, "wikivoyage-europe-2024", found.Slug)
	})

	t.Run("with empty optional fields", func(t *testing.T) {
		minimalEntry := ZimArchiveUpsert{
			Name:     "Minimal Archive",
			Title:    "Minimal",
			Language: "en",
		}

		err := repo.UpsertFromCatalog(ctx, svc.ID, minimalEntry)
		require.NoError(t, err)

		found, err := repo.FindByName(ctx, "Minimal Archive")
		require.NoError(t, err)

		assert.Equal(t, "Minimal Archive", found.Name)
		assert.Equal(t, "minimal-archive", found.Slug)
		assert.False(t, found.Description.Valid)
		assert.False(t, found.Category.Valid)
		assert.False(t, found.Creator.Valid)
		assert.False(t, found.Publisher.Valid)
		assert.False(t, found.Favicon.Valid)
		assert.False(t, found.Tags.Valid)
	})
}

func TestZimArchiveRepository_List(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	svc := createTestExternalService(t, ctx)
	repo := NewZimArchiveRepository(testDB, discardLogger())

	entries := []ZimArchiveUpsert{
		{Name: "Wikipedia English", Title: "Wikipedia (en)", Language: "en", Category: "wikipedia"},
		{Name: "Wikipedia French", Title: "Wikipedia (fr)", Language: "fr", Category: "wikipedia"},
		{Name: "StackExchange French", Title: "StackExchange (fr)", Language: "fr", Category: "stackexchange"},
	}
	for _, e := range entries {
		require.NoError(t, repo.UpsertFromCatalog(ctx, svc.ID, e))
	}

	// Disable one archive to test enabled-only filtering.
	var disableID int64
	err := testDB.QueryRowContext(ctx,
		"SELECT id FROM zim_archives WHERE name = $1", "Wikipedia French").Scan(&disableID)
	require.NoError(t, err)
	require.NoError(t, repo.ToggleEnabled(ctx, disableID))

	t.Run("all enabled", func(t *testing.T) {
		results, err := repo.List(ctx, "", "", "", 0)
		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("filter by category", func(t *testing.T) {
		results, err := repo.List(ctx, "wikipedia", "", "", 0)
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "Wikipedia English", results[0].Name)
	})

	t.Run("filter by language", func(t *testing.T) {
		results, err := repo.List(ctx, "", "en", "", 0)
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "Wikipedia English", results[0].Name)
	})

	t.Run("filter by search query", func(t *testing.T) {
		results, err := repo.List(ctx, "", "", "Wikipedia", 0)
		require.NoError(t, err)
		// Only Wikipedia English is enabled; Wikipedia French was disabled.
		assert.Len(t, results, 1)
		assert.Equal(t, "Wikipedia English", results[0].Name)
	})

	t.Run("combined filters", func(t *testing.T) {
		results, err := repo.List(ctx, "wikipedia", "en", "", 0)
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "Wikipedia English", results[0].Name)
	})

	t.Run("limit", func(t *testing.T) {
		results, err := repo.List(ctx, "", "", "", 1)
		require.NoError(t, err)
		assert.Len(t, results, 1)
	})
}

func TestZimArchiveRepository_FindByName(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	svc := createTestExternalService(t, ctx)
	repo := NewZimArchiveRepository(testDB, discardLogger())

	entry := ZimArchiveUpsert{
		Name:     "FindMe Archive",
		Title:    "Find Me",
		Language: "en",
		Category: "wikipedia",
	}
	require.NoError(t, repo.UpsertFromCatalog(ctx, svc.ID, entry))

	t.Run("found", func(t *testing.T) {
		found, err := repo.FindByName(ctx, "FindMe Archive")
		require.NoError(t, err)
		assert.Equal(t, "FindMe Archive", found.Name)
		assert.Equal(t, "Find Me", found.Title)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := repo.FindByName(ctx, "Nonexistent Archive")
		assert.Error(t, err)
	})

	t.Run("disabled not found", func(t *testing.T) {
		var id int64
		err := testDB.QueryRowContext(ctx,
			"SELECT id FROM zim_archives WHERE name = $1", "FindMe Archive").Scan(&id)
		require.NoError(t, err)

		require.NoError(t, repo.ToggleEnabled(ctx, id))

		_, err = repo.FindByName(ctx, "FindMe Archive")
		assert.Error(t, err)
	})
}

func TestZimArchiveRepository_FindByUUID(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	svc := createTestExternalService(t, ctx)
	repo := NewZimArchiveRepository(testDB, discardLogger())

	entry := ZimArchiveUpsert{
		Name:     "UUID Archive",
		Title:    "UUID Test",
		Language: "en",
		Category: "wikipedia",
	}
	require.NoError(t, repo.UpsertFromCatalog(ctx, svc.ID, entry))

	var uuid string
	err := testDB.QueryRowContext(ctx,
		"SELECT uuid FROM zim_archives WHERE name = $1", "UUID Archive").Scan(&uuid)
	require.NoError(t, err)

	t.Run("found", func(t *testing.T) {
		found, err := repo.FindByUUID(ctx, uuid)
		require.NoError(t, err)
		assert.Equal(t, "UUID Archive", found.Name)
		assert.Equal(t, uuid, found.UUID)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := repo.FindByUUID(ctx, testUUID("nonexistent-uuid"))
		assert.Error(t, err)
	})

	t.Run("disabled still found", func(t *testing.T) {
		var id int64
		err := testDB.QueryRowContext(ctx,
			"SELECT id FROM zim_archives WHERE name = $1", "UUID Archive").Scan(&id)
		require.NoError(t, err)

		require.NoError(t, repo.ToggleEnabled(ctx, id))

		// FindByUUID does NOT filter by is_enabled, so disabled archives are still returned.
		found, err := repo.FindByUUID(ctx, uuid)
		require.NoError(t, err)
		assert.Equal(t, "UUID Archive", found.Name)
		assert.False(t, found.IsEnabled)
	})
}

func TestZimArchiveRepository_FindDisabled(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	svc := createTestExternalService(t, ctx)
	repo := NewZimArchiveRepository(testDB, discardLogger())

	// Insert 3 archives: 2 enabled, 1 disabled.
	entries := []ZimArchiveUpsert{
		{Name: "Enabled Archive Alpha", Title: "Alpha", Language: "en", Category: "wikipedia"},
		{Name: "Enabled Archive Beta", Title: "Beta", Language: "fr", Category: "stackexchange"},
		{Name: "Disabled Archive Gamma", Title: "Gamma", Language: "de", Category: "wikipedia"},
	}
	for _, e := range entries {
		require.NoError(t, repo.UpsertFromCatalog(ctx, svc.ID, e))
	}

	// Disable one archive.
	var disableID int64
	err := testDB.QueryRowContext(ctx,
		"SELECT id FROM zim_archives WHERE name = $1", "Disabled Archive Gamma").Scan(&disableID)
	require.NoError(t, err)
	require.NoError(t, repo.ToggleEnabled(ctx, disableID))

	t.Run("returns only disabled archives", func(t *testing.T) {
		results, err := repo.FindDisabled(ctx)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, "Disabled Archive Gamma", results[0].Name)
		assert.False(t, results[0].IsEnabled)
	})

	t.Run("empty when all enabled", func(t *testing.T) {
		// Re-enable the disabled archive.
		require.NoError(t, repo.ToggleEnabled(ctx, disableID))

		results, err := repo.FindDisabled(ctx)
		require.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("returns multiple disabled archives", func(t *testing.T) {
		// Disable two archives.
		var alphaID, betaID int64
		err := testDB.QueryRowContext(ctx,
			"SELECT id FROM zim_archives WHERE name = $1", "Enabled Archive Alpha").Scan(&alphaID)
		require.NoError(t, err)
		err = testDB.QueryRowContext(ctx,
			"SELECT id FROM zim_archives WHERE name = $1", "Enabled Archive Beta").Scan(&betaID)
		require.NoError(t, err)

		require.NoError(t, repo.ToggleEnabled(ctx, alphaID))
		require.NoError(t, repo.ToggleEnabled(ctx, betaID))

		results, err := repo.FindDisabled(ctx)
		require.NoError(t, err)
		assert.Len(t, results, 2)

		// Verify both are disabled.
		for _, r := range results {
			assert.False(t, r.IsEnabled)
		}
	})
}

func TestZimArchiveRepository_ListAllAndCount(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	svc := createTestExternalService(t, ctx)
	repo := NewZimArchiveRepository(testDB, discardLogger())

	entries := []ZimArchiveUpsert{
		{Name: "Archive Alpha", Title: "Alpha", Language: "en", Category: "wikipedia"},
		{Name: "Archive Beta", Title: "Beta", Language: "fr", Category: "stackexchange"},
		{Name: "Archive Gamma", Title: "Gamma", Language: "en", Category: "wikipedia"},
	}
	for _, e := range entries {
		require.NoError(t, repo.UpsertFromCatalog(ctx, svc.ID, e))
	}

	// Disable one archive.
	var disableID int64
	err := testDB.QueryRowContext(ctx,
		"SELECT id FROM zim_archives WHERE name = $1", "Archive Beta").Scan(&disableID)
	require.NoError(t, err)
	require.NoError(t, repo.ToggleEnabled(ctx, disableID))

	t.Run("list all includes disabled", func(t *testing.T) {
		results, err := repo.ListAll(ctx, "", 0)
		require.NoError(t, err)
		assert.Len(t, results, 3)
	})

	t.Run("count includes all", func(t *testing.T) {
		count, err := repo.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, 3, count)
	})

	t.Run("list all with search query", func(t *testing.T) {
		results, err := repo.ListAll(ctx, "Alpha", 0)
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "Archive Alpha", results[0].Name)
	})

	t.Run("list all with limit", func(t *testing.T) {
		results, err := repo.ListAll(ctx, "", 2)
		require.NoError(t, err)
		assert.Len(t, results, 2)
	})
}

func TestZimArchiveRepository_ToggleEnabledAndSearchable(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	svc := createTestExternalService(t, ctx)
	repo := NewZimArchiveRepository(testDB, discardLogger())

	entry := ZimArchiveUpsert{
		Name:     "Toggle Archive",
		Title:    "Toggle Test",
		Language: "en",
		Category: "wikipedia",
	}
	require.NoError(t, repo.UpsertFromCatalog(ctx, svc.ID, entry))

	var id int64
	err := testDB.QueryRowContext(ctx,
		"SELECT id FROM zim_archives WHERE name = $1", "Toggle Archive").Scan(&id)
	require.NoError(t, err)

	// Verify defaults from UpsertFromCatalog: is_enabled=true, is_searchable=true (DB default).
	var archive model.ZimArchive
	err = testDB.GetContext(ctx, &archive,
		"SELECT * FROM zim_archives WHERE id = $1", id)
	require.NoError(t, err)
	assert.True(t, archive.IsEnabled)
	assert.True(t, archive.IsSearchable)

	t.Run("toggle enabled off", func(t *testing.T) {
		require.NoError(t, repo.ToggleEnabled(ctx, id))

		var a model.ZimArchive
		err := testDB.GetContext(ctx, &a, "SELECT * FROM zim_archives WHERE id = $1", id)
		require.NoError(t, err)
		assert.False(t, a.IsEnabled)
	})

	t.Run("toggle enabled on", func(t *testing.T) {
		require.NoError(t, repo.ToggleEnabled(ctx, id))

		var a model.ZimArchive
		err := testDB.GetContext(ctx, &a, "SELECT * FROM zim_archives WHERE id = $1", id)
		require.NoError(t, err)
		assert.True(t, a.IsEnabled)
	})

	t.Run("toggle searchable off", func(t *testing.T) {
		require.NoError(t, repo.ToggleSearchable(ctx, id))

		var a model.ZimArchive
		err := testDB.GetContext(ctx, &a, "SELECT * FROM zim_archives WHERE id = $1", id)
		require.NoError(t, err)
		assert.False(t, a.IsSearchable)
	})

	t.Run("toggle searchable on", func(t *testing.T) {
		require.NoError(t, repo.ToggleSearchable(ctx, id))

		var a model.ZimArchive
		err := testDB.GetContext(ctx, &a, "SELECT * FROM zim_archives WHERE id = $1", id)
		require.NoError(t, err)
		assert.True(t, a.IsSearchable)
	})
}

func TestZimArchiveRepository_DisableOrphaned(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	svc := createTestExternalService(t, ctx)
	repo := NewZimArchiveRepository(testDB, discardLogger())

	entries := []ZimArchiveUpsert{
		{Name: "Active Archive", Title: "Active", Language: "en", Category: "wikipedia"},
		{Name: "Orphan One", Title: "Orphan 1", Language: "en", Category: "wikipedia"},
		{Name: "Orphan Two", Title: "Orphan 2", Language: "fr", Category: "stackexchange"},
	}
	for _, e := range entries {
		require.NoError(t, repo.UpsertFromCatalog(ctx, svc.ID, e))
	}

	t.Run("disable orphaned with one active", func(t *testing.T) {
		disabled, err := repo.DisableOrphaned(ctx, svc.ID, []string{"Active Archive"})
		require.NoError(t, err)
		assert.Equal(t, 2, disabled)

		// Active archive should still be enabled.
		active, err := repo.FindByName(ctx, "Active Archive")
		require.NoError(t, err)
		assert.True(t, active.IsEnabled)

		// Orphaned archives should be disabled (FindByName filters by is_enabled=true).
		_, err = repo.FindByName(ctx, "Orphan One")
		assert.Error(t, err)

		_, err = repo.FindByName(ctx, "Orphan Two")
		assert.Error(t, err)
	})

	t.Run("empty active names disables all remaining", func(t *testing.T) {
		disabled, err := repo.DisableOrphaned(ctx, svc.ID, []string{})
		require.NoError(t, err)
		assert.Equal(t, 1, disabled) // Only "Active Archive" was still enabled.

		// All archives should now be disabled.
		_, err = repo.FindByName(ctx, "Active Archive")
		assert.Error(t, err)
	})
}
