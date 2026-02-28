//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"git.999.haus/chris/DocuMCP-go/internal/model"
)

func TestConfluenceSpaceRepository_UpsertFromAPI(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewConfluenceSpaceRepository(testDB, discardLogger())

	// FK setup: create an external service.
	svcRepo := NewExternalServiceRepository(testDB, discardLogger())
	svc := &model.ExternalService{
		UUID:      testUUID("cs-svc-001"),
		Name:      "Test Confluence",
		Slug:      "test-confluence-svc",
		Type:      "confluence",
		BaseURL:   "https://confluence.example.com",
		Priority:  1,
		Status:    "healthy",
		IsEnabled: true,
	}
	require.NoError(t, svcRepo.Create(ctx, svc))

	space := ConfluenceSpaceUpsert{
		ConfluenceID: "12345",
		Key:          "DEV",
		Name:         "Development",
		Description:  "Dev space",
		Type:         "global",
		Status:       "current",
		HomepageID:   "99",
		IconURL:      "https://confluence.example.com/icon.png",
	}

	err := repo.UpsertFromAPI(ctx, svc.ID, space)
	require.NoError(t, err)

	// Verify via FindByKey.
	found, err := repo.FindByKey(ctx, "DEV")
	require.NoError(t, err)

	assert.Equal(t, "12345", found.ConfluenceID)
	assert.Equal(t, "DEV", found.Key)
	assert.Equal(t, "Development", found.Name)
	assert.Equal(t, "global", found.Type)
	assert.Equal(t, "current", found.Status)
	assert.True(t, found.ExternalServiceID.Valid)
	assert.Equal(t, svc.ID, found.ExternalServiceID.Int64)
	assert.True(t, found.IsEnabled)
	assert.True(t, found.LastSyncedAt.Valid)

	t.Run("upsert existing", func(t *testing.T) {
		updated := ConfluenceSpaceUpsert{
			ConfluenceID: "12345",
			Key:          "DEV",
			Name:         "Development Updated",
			Description:  "Updated dev space",
			Type:         "global",
			Status:       "current",
		}
		err := repo.UpsertFromAPI(ctx, svc.ID, updated)
		require.NoError(t, err)

		found, err := repo.FindByKey(ctx, "DEV")
		require.NoError(t, err)
		assert.Equal(t, "Development Updated", found.Name)
	})

	t.Run("defaults", func(t *testing.T) {
		defaults := ConfluenceSpaceUpsert{
			ConfluenceID: "99999",
			Key:          "DEFAULTS",
			Name:         "Defaults Space",
		}
		err := repo.UpsertFromAPI(ctx, svc.ID, defaults)
		require.NoError(t, err)

		found, err := repo.FindByKey(ctx, "DEFAULTS")
		require.NoError(t, err)
		assert.Equal(t, "global", found.Type)
		assert.Equal(t, "current", found.Status)
	})
}

func TestConfluenceSpaceRepository_List(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewConfluenceSpaceRepository(testDB, discardLogger())

	// FK setup.
	svcRepo := NewExternalServiceRepository(testDB, discardLogger())
	svc := &model.ExternalService{
		UUID:      testUUID("cs-list-svc"),
		Name:      "List Confluence",
		Slug:      "list-confluence-svc",
		Type:      "confluence",
		BaseURL:   "https://confluence.example.com",
		Priority:  1,
		Status:    "healthy",
		IsEnabled: true,
	}
	require.NoError(t, svcRepo.Create(ctx, svc))

	spaces := []ConfluenceSpaceUpsert{
		{ConfluenceID: "1", Key: "SPACEA", Name: "Space A", Type: "global", Status: "current"},
		{ConfluenceID: "2", Key: "SPACEB", Name: "Space B", Type: "global", Status: "current"},
		{ConfluenceID: "3", Key: "PERSONAL", Name: "Personal Space", Type: "personal", Status: "current"},
	}
	for _, s := range spaces {
		require.NoError(t, repo.UpsertFromAPI(ctx, svc.ID, s))
	}

	// Disable one space to verify List excludes disabled.
	spaceB, err := repo.FindByKey(ctx, "SPACEB")
	require.NoError(t, err)
	require.NoError(t, repo.ToggleEnabled(ctx, spaceB.ID))

	t.Run("all enabled", func(t *testing.T) {
		results, err := repo.List(ctx, "", "", 0)
		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("filter by type", func(t *testing.T) {
		results, err := repo.List(ctx, "global", "", 0)
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "SPACEA", results[0].Key)
	})

	t.Run("filter by search query", func(t *testing.T) {
		results, err := repo.List(ctx, "", "Space A", 0)
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "SPACEA", results[0].Key)
	})

	t.Run("limit", func(t *testing.T) {
		results, err := repo.List(ctx, "", "", 1)
		require.NoError(t, err)
		assert.Len(t, results, 1)
	})
}

func TestConfluenceSpaceRepository_FindByKey(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewConfluenceSpaceRepository(testDB, discardLogger())

	// FK setup.
	svcRepo := NewExternalServiceRepository(testDB, discardLogger())
	svc := &model.ExternalService{
		UUID:      testUUID("cs-fbk-svc"),
		Name:      "FindByKey Confluence",
		Slug:      "fbk-confluence-svc",
		Type:      "confluence",
		BaseURL:   "https://confluence.example.com",
		Priority:  1,
		Status:    "healthy",
		IsEnabled: true,
	}
	require.NoError(t, svcRepo.Create(ctx, svc))

	space := ConfluenceSpaceUpsert{
		ConfluenceID: "100",
		Key:          "FINDME",
		Name:         "Find Me Space",
		Type:         "global",
		Status:       "current",
	}
	require.NoError(t, repo.UpsertFromAPI(ctx, svc.ID, space))

	t.Run("found", func(t *testing.T) {
		found, err := repo.FindByKey(ctx, "FINDME")
		require.NoError(t, err)
		assert.Equal(t, "Find Me Space", found.Name)
		assert.Equal(t, "FINDME", found.Key)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := repo.FindByKey(ctx, "NONEXISTENT")
		assert.Error(t, err)
	})

	t.Run("disabled not found", func(t *testing.T) {
		found, err := repo.FindByKey(ctx, "FINDME")
		require.NoError(t, err)

		require.NoError(t, repo.ToggleEnabled(ctx, found.ID))

		_, err = repo.FindByKey(ctx, "FINDME")
		assert.Error(t, err)
	})
}

func TestConfluenceSpaceRepository_FindByUUID(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewConfluenceSpaceRepository(testDB, discardLogger())

	// FK setup.
	svcRepo := NewExternalServiceRepository(testDB, discardLogger())
	svc := &model.ExternalService{
		UUID:      testUUID("cs-fbu-svc"),
		Name:      "FindByUUID Confluence",
		Slug:      "fbu-confluence-svc",
		Type:      "confluence",
		BaseURL:   "https://confluence.example.com",
		Priority:  1,
		Status:    "healthy",
		IsEnabled: true,
	}
	require.NoError(t, svcRepo.Create(ctx, svc))

	space := ConfluenceSpaceUpsert{
		ConfluenceID: "200",
		Key:          "UUIDTEST",
		Name:         "UUID Test Space",
		Type:         "global",
		Status:       "current",
	}
	require.NoError(t, repo.UpsertFromAPI(ctx, svc.ID, space))

	// UpsertFromAPI uses gen_random_uuid(), so query the UUID from the database.
	var uuid string
	err := testDB.QueryRowContext(ctx, `SELECT uuid FROM confluence_spaces WHERE key = $1`, "UUIDTEST").Scan(&uuid)
	require.NoError(t, err)

	t.Run("found", func(t *testing.T) {
		found, err := repo.FindByUUID(ctx, uuid)
		require.NoError(t, err)
		assert.Equal(t, "UUID Test Space", found.Name)
		assert.Equal(t, "UUIDTEST", found.Key)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := repo.FindByUUID(ctx, testUUID("nonexistent-uuid"))
		assert.Error(t, err)
	})

	t.Run("includes disabled", func(t *testing.T) {
		// Disable the space.
		found, err := repo.FindByUUID(ctx, uuid)
		require.NoError(t, err)
		require.NoError(t, repo.ToggleEnabled(ctx, found.ID))

		// FindByUUID should still return it.
		found, err = repo.FindByUUID(ctx, uuid)
		require.NoError(t, err)
		assert.Equal(t, "UUIDTEST", found.Key)
		assert.False(t, found.IsEnabled)
	})
}

func TestConfluenceSpaceRepository_ListAllAndCount(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewConfluenceSpaceRepository(testDB, discardLogger())

	// FK setup.
	svcRepo := NewExternalServiceRepository(testDB, discardLogger())
	svc := &model.ExternalService{
		UUID:      testUUID("cs-la-svc"),
		Name:      "ListAll Confluence",
		Slug:      "la-confluence-svc",
		Type:      "confluence",
		BaseURL:   "https://confluence.example.com",
		Priority:  1,
		Status:    "healthy",
		IsEnabled: true,
	}
	require.NoError(t, svcRepo.Create(ctx, svc))

	spaces := []ConfluenceSpaceUpsert{
		{ConfluenceID: "1", Key: "ALL1", Name: "Alpha Space", Type: "global", Status: "current"},
		{ConfluenceID: "2", Key: "ALL2", Name: "Beta Space", Type: "global", Status: "current"},
		{ConfluenceID: "3", Key: "ALL3", Name: "Gamma Space", Type: "personal", Status: "current"},
	}
	for _, s := range spaces {
		require.NoError(t, repo.UpsertFromAPI(ctx, svc.ID, s))
	}

	// Disable one space.
	disableMe, err := repo.FindByKey(ctx, "ALL2")
	require.NoError(t, err)
	require.NoError(t, repo.ToggleEnabled(ctx, disableMe.ID))

	t.Run("ListAll includes disabled", func(t *testing.T) {
		results, err := repo.ListAll(ctx, "", 0)
		require.NoError(t, err)
		assert.Len(t, results, 3)
	})

	t.Run("ListAll with search query", func(t *testing.T) {
		results, err := repo.ListAll(ctx, "Alpha", 0)
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "ALL1", results[0].Key)
	})

	t.Run("Count returns all", func(t *testing.T) {
		count, err := repo.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, 3, count)
	})
}

func TestConfluenceSpaceRepository_ToggleEnabled(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewConfluenceSpaceRepository(testDB, discardLogger())

	// FK setup.
	svcRepo := NewExternalServiceRepository(testDB, discardLogger())
	svc := &model.ExternalService{
		UUID:      testUUID("cs-te-svc"),
		Name:      "ToggleEnabled Confluence",
		Slug:      "te-confluence-svc",
		Type:      "confluence",
		BaseURL:   "https://confluence.example.com",
		Priority:  1,
		Status:    "healthy",
		IsEnabled: true,
	}
	require.NoError(t, svcRepo.Create(ctx, svc))

	space := ConfluenceSpaceUpsert{
		ConfluenceID: "500",
		Key:          "TOGGLE",
		Name:         "Toggle Space",
		Type:         "global",
		Status:       "current",
	}
	require.NoError(t, repo.UpsertFromAPI(ctx, svc.ID, space))

	// Get the ID via FindByKey.
	found, err := repo.FindByKey(ctx, "TOGGLE")
	require.NoError(t, err)
	assert.True(t, found.IsEnabled, "newly upserted space should be enabled")

	// Toggle off.
	require.NoError(t, repo.ToggleEnabled(ctx, found.ID))

	var isEnabled bool
	err = testDB.QueryRowContext(ctx, `SELECT is_enabled FROM confluence_spaces WHERE id = $1`, found.ID).Scan(&isEnabled)
	require.NoError(t, err)
	assert.False(t, isEnabled)

	// Toggle back on.
	require.NoError(t, repo.ToggleEnabled(ctx, found.ID))

	err = testDB.QueryRowContext(ctx, `SELECT is_enabled FROM confluence_spaces WHERE id = $1`, found.ID).Scan(&isEnabled)
	require.NoError(t, err)
	assert.True(t, isEnabled)
}

func TestConfluenceSpaceRepository_ToggleSearchable(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewConfluenceSpaceRepository(testDB, discardLogger())

	// FK setup.
	svcRepo := NewExternalServiceRepository(testDB, discardLogger())
	svc := &model.ExternalService{
		UUID:      testUUID("cs-ts-svc"),
		Name:      "ToggleSearchable Confluence",
		Slug:      "ts-confluence-svc",
		Type:      "confluence",
		BaseURL:   "https://confluence.example.com",
		Priority:  1,
		Status:    "healthy",
		IsEnabled: true,
	}
	require.NoError(t, svcRepo.Create(ctx, svc))

	space := ConfluenceSpaceUpsert{
		ConfluenceID: "600",
		Key:          "SEARCH",
		Name:         "Searchable Space",
		Type:         "global",
		Status:       "current",
	}
	require.NoError(t, repo.UpsertFromAPI(ctx, svc.ID, space))

	found, err := repo.FindByKey(ctx, "SEARCH")
	require.NoError(t, err)

	// Check initial state (default from the INSERT is not explicitly set, check actual).
	var isSearchable bool
	err = testDB.QueryRowContext(ctx, `SELECT is_searchable FROM confluence_spaces WHERE id = $1`, found.ID).Scan(&isSearchable)
	require.NoError(t, err)
	initialState := isSearchable

	// Toggle.
	require.NoError(t, repo.ToggleSearchable(ctx, found.ID))

	err = testDB.QueryRowContext(ctx, `SELECT is_searchable FROM confluence_spaces WHERE id = $1`, found.ID).Scan(&isSearchable)
	require.NoError(t, err)
	assert.Equal(t, !initialState, isSearchable)

	// Toggle back.
	require.NoError(t, repo.ToggleSearchable(ctx, found.ID))

	err = testDB.QueryRowContext(ctx, `SELECT is_searchable FROM confluence_spaces WHERE id = $1`, found.ID).Scan(&isSearchable)
	require.NoError(t, err)
	assert.Equal(t, initialState, isSearchable)
}

func TestConfluenceSpaceRepository_DisableOrphaned(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewConfluenceSpaceRepository(testDB, discardLogger())

	// FK setup.
	svcRepo := NewExternalServiceRepository(testDB, discardLogger())
	svc := &model.ExternalService{
		UUID:      testUUID("cs-do-svc"),
		Name:      "DisableOrphaned Confluence",
		Slug:      "do-confluence-svc",
		Type:      "confluence",
		BaseURL:   "https://confluence.example.com",
		Priority:  1,
		Status:    "healthy",
		IsEnabled: true,
	}
	require.NoError(t, svcRepo.Create(ctx, svc))

	spaces := []ConfluenceSpaceUpsert{
		{ConfluenceID: "1", Key: "ACTIVE", Name: "Active Space", Type: "global", Status: "current"},
		{ConfluenceID: "2", Key: "ORPHAN1", Name: "Orphan One", Type: "global", Status: "current"},
		{ConfluenceID: "3", Key: "ORPHAN2", Name: "Orphan Two", Type: "global", Status: "current"},
	}
	for _, s := range spaces {
		require.NoError(t, repo.UpsertFromAPI(ctx, svc.ID, s))
	}

	// DisableOrphaned with only ACTIVE in the active keys list.
	disabled, err := repo.DisableOrphaned(ctx, svc.ID, []string{"ACTIVE"})
	require.NoError(t, err)
	assert.Equal(t, 2, disabled)

	// Verify orphaned spaces are disabled.
	orphan1, err := repo.FindByKey(ctx, "ORPHAN1")
	assert.Error(t, err, "ORPHAN1 should not be found via FindByKey (disabled)")
	assert.Nil(t, orphan1)

	orphan2, err := repo.FindByKey(ctx, "ORPHAN2")
	assert.Error(t, err, "ORPHAN2 should not be found via FindByKey (disabled)")
	assert.Nil(t, orphan2)

	// ACTIVE should still be enabled.
	active, err := repo.FindByKey(ctx, "ACTIVE")
	require.NoError(t, err)
	assert.True(t, active.IsEnabled)

	t.Run("empty active keys", func(t *testing.T) {
		// ACTIVE is the only remaining enabled space. Disable it with empty active keys.
		disabled, err := repo.DisableOrphaned(ctx, svc.ID, []string{})
		require.NoError(t, err)
		assert.Equal(t, 1, disabled)

		_, err = repo.FindByKey(ctx, "ACTIVE")
		assert.Error(t, err, "ACTIVE should now be disabled")
	})
}
