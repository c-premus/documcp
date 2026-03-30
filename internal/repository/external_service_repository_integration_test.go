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

func TestExternalServiceRepository_CreateAndFind(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewExternalServiceRepository(testPool, discardLogger())

	svc := &model.ExternalService{
		UUID:         testUUID("create-find-001"),
		Name:         "Test Service",
		Slug:         "test-service",
		Type:         "kiwix",
		BaseURL:      "https://kiwix.example.com",
		APIKey:       sql.NullString{String: "secret-key", Valid: true},
		Config:       sql.NullString{String: `{"space":"DEV"}`, Valid: true},
		Priority:     10,
		Status:       "unknown",
		IsEnabled:    true,
		IsEnvManaged: false,
	}

	err := repo.Create(ctx, svc)
	require.NoError(t, err)
	assert.NotZero(t, svc.ID, "ID should be set after insert")
	assert.True(t, svc.CreatedAt.Valid, "CreatedAt should be set")
	assert.True(t, svc.UpdatedAt.Valid, "UpdatedAt should be set")

	t.Run("FindByUUID", func(t *testing.T) {
		found, err := repo.FindByUUID(ctx, testUUID("create-find-001"))
		require.NoError(t, err)

		assert.Equal(t, svc.ID, found.ID)
		assert.Equal(t, testUUID("create-find-001"), found.UUID)
		assert.Equal(t, "Test Service", found.Name)
		assert.Equal(t, "test-service", found.Slug)
		assert.Equal(t, "kiwix", found.Type)
		assert.Equal(t, "https://kiwix.example.com", found.BaseURL)
		assert.True(t, found.APIKey.Valid)
		assert.Equal(t, "secret-key", found.APIKey.String)
		assert.True(t, found.Config.Valid)
		assert.Equal(t, `{"space":"DEV"}`, found.Config.String)
		assert.Equal(t, 10, found.Priority)
		assert.Equal(t, "unknown", found.Status)
		assert.True(t, found.IsEnabled)
		assert.False(t, found.IsEnvManaged)
		assert.Equal(t, 0, found.ErrorCount)
		assert.Equal(t, 0, found.ConsecutiveFailures)
		assert.True(t, found.CreatedAt.Valid)
		assert.True(t, found.UpdatedAt.Valid)
	})

	t.Run("FindBySlug", func(t *testing.T) {
		found, err := repo.FindBySlug(ctx, "test-service")
		require.NoError(t, err)

		assert.Equal(t, svc.ID, found.ID)
		assert.Equal(t, testUUID("create-find-001"), found.UUID)
		assert.Equal(t, "test-service", found.Slug)
	})

	t.Run("FindByUUID_NotFound", func(t *testing.T) {
		_, err := repo.FindByUUID(ctx, "nonexistent-uuid")
		assert.Error(t, err)
	})

	t.Run("FindBySlug_NotFound", func(t *testing.T) {
		_, err := repo.FindBySlug(ctx, "nonexistent-slug")
		assert.Error(t, err)
	})
}

func TestExternalServiceRepository_FindEnabledByType(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewExternalServiceRepository(testPool, discardLogger())

	services := []model.ExternalService{
		{
			UUID:      testUUID("enabled-type-001"),
			Name:      "Kiwix High Priority",
			Slug:      "kiwix-high",
			Type:      "kiwix",
			BaseURL:   "https://k1.example.com",
			Priority:  1,
			Status:    "healthy",
			IsEnabled: true,
		},
		{
			UUID:      testUUID("enabled-type-002"),
			Name:      "Kiwix Low Priority",
			Slug:      "kiwix-low",
			Type:      "kiwix",
			BaseURL:   "https://k2.example.com",
			Priority:  20,
			Status:    "healthy",
			IsEnabled: true,
		},
		{
			UUID:      testUUID("enabled-type-003"),
			Name:      "Kiwix Disabled",
			Slug:      "kiwix-disabled",
			Type:      "kiwix",
			BaseURL:   "https://k3.example.com",
			Priority:  5,
			Status:    "healthy",
			IsEnabled: false,
		},
		{
			UUID:      testUUID("enabled-type-004"),
			Name:      "Git Service",
			Slug:      "git-service",
			Type:      "git",
			BaseURL:   "https://git.example.com",
			Priority:  1,
			Status:    "healthy",
			IsEnabled: true,
		},
	}

	for i := range services {
		require.NoError(t, repo.Create(ctx, &services[i]))
	}

	tests := []struct {
		name        string
		serviceType string
		wantCount   int
		wantSlugs   []string // expected order by priority
	}{
		{
			name:        "kiwix enabled only",
			serviceType: "kiwix",
			wantCount:   2,
			wantSlugs:   []string{"kiwix-high", "kiwix-low"},
		},
		{
			name:        "git enabled",
			serviceType: "git",
			wantCount:   1,
			wantSlugs:   []string{"git-service"},
		},
		{
			name:        "nonexistent type",
			serviceType: "nonexistent",
			wantCount:   0,
			wantSlugs:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := repo.FindEnabledByType(ctx, tt.serviceType)
			require.NoError(t, err)
			assert.Len(t, result, tt.wantCount)

			if tt.wantSlugs != nil {
				gotSlugs := make([]string, len(result))
				for i, s := range result {
					gotSlugs[i] = s.Slug
				}
				assert.Equal(t, tt.wantSlugs, gotSlugs, "results should be ordered by priority")
			}
		})
	}
}

func TestExternalServiceRepository_List(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewExternalServiceRepository(testPool, discardLogger())

	services := []model.ExternalService{
		{
			UUID:      testUUID("list-001"),
			Name:      "Kiwix Healthy",
			Slug:      "kiwix-healthy",
			Type:      "kiwix",
			BaseURL:   "https://k1.example.com",
			Priority:  5,
			Status:    "healthy",
			IsEnabled: true,
		},
		{
			UUID:      testUUID("list-002"),
			Name:      "Kiwix Unhealthy",
			Slug:      "kiwix-unhealthy",
			Type:      "kiwix",
			BaseURL:   "https://k2.example.com",
			Priority:  10,
			Status:    "unhealthy",
			IsEnabled: true,
		},
		{
			UUID:      testUUID("list-003"),
			Name:      "Git Healthy",
			Slug:      "git-healthy",
			Type:      "git",
			BaseURL:   "https://git.example.com",
			Priority:  1,
			Status:    "healthy",
			IsEnabled: true,
		},
		{
			UUID:      testUUID("list-004"),
			Name:      "Kiwix Unknown",
			Slug:      "kiwix-unknown",
			Type:      "kiwix",
			BaseURL:   "https://kiwix.example.com",
			Priority:  3,
			Status:    "unknown",
			IsEnabled: false,
		},
	}

	for i := range services {
		require.NoError(t, repo.Create(ctx, &services[i]))
	}

	tests := []struct {
		name       string
		filterType string
		status     string
		limit      int
		offset     int
		wantCount  int
		wantTotal  int
	}{
		{
			name:      "no filters",
			limit:     50,
			wantCount: 4,
			wantTotal: 4,
		},
		{
			name:       "filter by type kiwix",
			filterType: "kiwix",
			limit:      50,
			wantCount:  2,
			wantTotal:  2,
		},
		{
			name:      "filter by status healthy",
			status:    "healthy",
			limit:     50,
			wantCount: 2,
			wantTotal: 2,
		},
		{
			name:       "filter by type and status",
			filterType: "kiwix",
			status:     "healthy",
			limit:      50,
			wantCount:  1,
			wantTotal:  1,
		},
		{
			name:       "filter with no matches",
			filterType: "git",
			status:     "unhealthy",
			limit:      50,
			wantCount:  0,
			wantTotal:  0,
		},
		{
			name:      "pagination limit",
			limit:     2,
			wantCount: 2,
			wantTotal: 4,
		},
		{
			name:      "pagination offset",
			limit:     2,
			offset:    2,
			wantCount: 2,
			wantTotal: 4,
		},
		{
			name:      "pagination offset beyond results",
			limit:     50,
			offset:    10,
			wantCount: 0,
			wantTotal: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, total, err := repo.List(ctx, tt.filterType, tt.status, tt.limit, tt.offset)
			require.NoError(t, err)
			assert.Len(t, results, tt.wantCount)
			assert.Equal(t, tt.wantTotal, total)
		})
	}
}

func TestExternalServiceRepository_Update(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewExternalServiceRepository(testPool, discardLogger())

	svc := &model.ExternalService{
		UUID:      testUUID("update-001"),
		Name:      "Original Name",
		Slug:      "original-slug",
		Type:      "kiwix",
		BaseURL:   "https://original.example.com",
		Priority:  5,
		Status:    "unknown",
		IsEnabled: true,
	}
	require.NoError(t, repo.Create(ctx, svc))

	svc.Name = "Updated Name"
	svc.Slug = "updated-slug"
	svc.BaseURL = "https://updated.example.com"
	svc.Priority = 1
	svc.IsEnabled = false

	err := repo.Update(ctx, svc)
	require.NoError(t, err)

	found, err := repo.FindByUUID(ctx, testUUID("update-001"))
	require.NoError(t, err)

	assert.Equal(t, "Updated Name", found.Name)
	assert.Equal(t, "updated-slug", found.Slug)
	assert.Equal(t, "https://updated.example.com", found.BaseURL)
	assert.Equal(t, 1, found.Priority)
	assert.False(t, found.IsEnabled)
	// Type should remain unchanged since Update does not modify it.
	assert.Equal(t, "kiwix", found.Type)
	// UpdatedAt should be refreshed.
	assert.True(t, found.UpdatedAt.Time.After(found.CreatedAt.Time) || found.UpdatedAt.Time.Equal(found.CreatedAt.Time))
}

func TestExternalServiceRepository_Delete(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewExternalServiceRepository(testPool, discardLogger())

	svc := &model.ExternalService{
		UUID:      testUUID("delete-001"),
		Name:      "To Be Deleted",
		Slug:      "to-be-deleted",
		Type:      "git",
		BaseURL:   "https://delete.example.com",
		Priority:  1,
		Status:    "unknown",
		IsEnabled: true,
	}
	require.NoError(t, repo.Create(ctx, svc))

	// Confirm it exists.
	_, err := repo.FindByUUID(ctx, testUUID("delete-001"))
	require.NoError(t, err)

	err = repo.Delete(ctx, svc.ID)
	require.NoError(t, err)

	// Verify it is gone.
	_, err = repo.FindByUUID(ctx, testUUID("delete-001"))
	assert.Error(t, err)
}

func TestExternalServiceRepository_FindAllEnabled(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewExternalServiceRepository(testPool, discardLogger())

	// Insert services: 2 enabled (different types), 1 disabled.
	enabledKiwix := &model.ExternalService{
		UUID:      testUUID("find-all-enabled-001"),
		Name:      "Alpha Kiwix",
		Slug:      "alpha-kiwix",
		Type:      "kiwix",
		BaseURL:   "https://kiwix.example.com",
		Priority:  5,
		Status:    "healthy",
		IsEnabled: true,
	}
	enabledGit := &model.ExternalService{
		UUID:      testUUID("find-all-enabled-002"),
		Name:      "Beta Git",
		Slug:      "beta-git",
		Type:      "git",
		BaseURL:   "https://git.example.com",
		Priority:  1,
		Status:    "healthy",
		IsEnabled: true,
	}
	disabled := &model.ExternalService{
		UUID:      testUUID("find-all-enabled-003"),
		Name:      "Gamma Kiwix",
		Slug:      "gamma-kiwix",
		Type:      "kiwix",
		BaseURL:   "https://kiwix.example.com",
		Priority:  1,
		Status:    "unknown",
		IsEnabled: false,
	}

	for _, svc := range []*model.ExternalService{enabledKiwix, enabledGit, disabled} {
		require.NoError(t, repo.Create(ctx, svc))
	}

	t.Run("returns only enabled services ordered by name", func(t *testing.T) {
		results, err := repo.FindAllEnabled(ctx)
		require.NoError(t, err)
		assert.Len(t, results, 2)

		// Ordered by name: "Alpha Kiwix" before "Beta Git".
		assert.Equal(t, "Alpha Kiwix", results[0].Name)
		assert.Equal(t, "Beta Git", results[1].Name)
	})

	t.Run("excludes disabled services", func(t *testing.T) {
		results, err := repo.FindAllEnabled(ctx)
		require.NoError(t, err)

		for _, svc := range results {
			assert.True(t, svc.IsEnabled, "FindAllEnabled should only return enabled services")
			assert.NotEqual(t, "Gamma Kiwix", svc.Name, "disabled service should not appear")
		}
	})

	t.Run("empty when all disabled", func(t *testing.T) {
		truncateAll(t)

		onlyDisabled := &model.ExternalService{
			UUID:      testUUID("find-all-enabled-004"),
			Name:      "Disabled Only",
			Slug:      "disabled-only",
			Type:      "git",
			BaseURL:   "https://disabled.example.com",
			Priority:  1,
			Status:    "unknown",
			IsEnabled: false,
		}
		require.NoError(t, repo.Create(ctx, onlyDisabled))

		results, err := repo.FindAllEnabled(ctx)
		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestExternalServiceRepository_UpdateHealthStatus(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewExternalServiceRepository(testPool, discardLogger())

	svc := &model.ExternalService{
		UUID:      testUUID("health-001"),
		Name:      "Health Check Target",
		Slug:      "health-check-target",
		Type:      "kiwix",
		BaseURL:   "https://health.example.com",
		Priority:  1,
		Status:    "unknown",
		IsEnabled: true,
	}
	require.NoError(t, repo.Create(ctx, svc))

	t.Run("update to healthy", func(t *testing.T) {
		err := repo.UpdateHealthStatus(ctx, svc.ID, "healthy", 42, "")
		require.NoError(t, err)

		found, err := repo.FindByUUID(ctx, testUUID("health-001"))
		require.NoError(t, err)

		assert.Equal(t, "healthy", found.Status)
		assert.True(t, found.LastCheckAt.Valid)
		assert.True(t, found.LastLatencyMS.Valid)
		assert.Equal(t, int64(42), found.LastLatencyMS.Int64)
		assert.Equal(t, 0, found.ConsecutiveFailures)
		assert.Equal(t, 0, found.ErrorCount)
	})

	t.Run("update to unhealthy first time", func(t *testing.T) {
		err := repo.UpdateHealthStatus(ctx, svc.ID, "unhealthy", 500, "connection timeout")
		require.NoError(t, err)

		found, err := repo.FindByUUID(ctx, testUUID("health-001"))
		require.NoError(t, err)

		assert.Equal(t, "unhealthy", found.Status)
		assert.True(t, found.LastLatencyMS.Valid)
		assert.Equal(t, int64(500), found.LastLatencyMS.Int64)
		assert.Equal(t, 1, found.ErrorCount)
		assert.Equal(t, 1, found.ConsecutiveFailures)
		assert.True(t, found.LastError.Valid)
		assert.Equal(t, "connection timeout", found.LastError.String)
		assert.True(t, found.LastErrorAt.Valid)
	})

	t.Run("update to unhealthy second time", func(t *testing.T) {
		err := repo.UpdateHealthStatus(ctx, svc.ID, "unhealthy", 600, "connection refused")
		require.NoError(t, err)

		found, err := repo.FindByUUID(ctx, testUUID("health-001"))
		require.NoError(t, err)

		assert.Equal(t, "unhealthy", found.Status)
		assert.Equal(t, 2, found.ErrorCount)
		assert.Equal(t, 2, found.ConsecutiveFailures)
		assert.Equal(t, "connection refused", found.LastError.String)
	})

	t.Run("recovery to healthy resets consecutive failures", func(t *testing.T) {
		err := repo.UpdateHealthStatus(ctx, svc.ID, "healthy", 30, "")
		require.NoError(t, err)

		found, err := repo.FindByUUID(ctx, testUUID("health-001"))
		require.NoError(t, err)

		assert.Equal(t, "healthy", found.Status)
		assert.Equal(t, 0, found.ConsecutiveFailures)
		// error_count should NOT reset on healthy — it is cumulative.
		assert.Equal(t, 2, found.ErrorCount)
		assert.Equal(t, int64(30), found.LastLatencyMS.Int64)
	})
}

func TestExternalServiceRepository_Count(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewExternalServiceRepository(testPool, discardLogger())

	t.Run("empty table", func(t *testing.T) {
		count, err := repo.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	// Create 3 services.
	svc1 := &model.ExternalService{
		UUID:      testUUID("count-001"),
		Name:      "Count Service One",
		Slug:      "count-service-one",
		Type:      "kiwix",
		BaseURL:   "https://k1.example.com",
		Priority:  1,
		Status:    "unknown",
		IsEnabled: true,
	}
	svc2 := &model.ExternalService{
		UUID:      testUUID("count-002"),
		Name:      "Count Service Two",
		Slug:      "count-service-two",
		Type:      "git",
		BaseURL:   "https://g1.example.com",
		Priority:  2,
		Status:    "unknown",
		IsEnabled: true,
	}
	svc3 := &model.ExternalService{
		UUID:      testUUID("count-003"),
		Name:      "Count Service Three",
		Slug:      "count-service-three",
		Type:      "kiwix",
		BaseURL:   "https://k1.example.com",
		Priority:  3,
		Status:    "unknown",
		IsEnabled: true,
	}

	for _, svc := range []*model.ExternalService{svc1, svc2, svc3} {
		require.NoError(t, repo.Create(ctx, svc))
	}

	count, err := repo.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, count)

	// Delete one and verify count drops.
	require.NoError(t, repo.Delete(ctx, svc1.ID))

	count, err = repo.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestExternalServiceRepository_ReorderPriorities(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewExternalServiceRepository(testPool, discardLogger())

	// Create 3 services with default priority 0.
	svc1 := &model.ExternalService{
		UUID:      testUUID("reorder-001"),
		Name:      "Reorder One",
		Slug:      "reorder-one",
		Type:      "kiwix",
		BaseURL:   "https://r1.example.com",
		Priority:  0,
		Status:    "unknown",
		IsEnabled: true,
	}
	svc2 := &model.ExternalService{
		UUID:      testUUID("reorder-002"),
		Name:      "Reorder Two",
		Slug:      "reorder-two",
		Type:      "git",
		BaseURL:   "https://r2.example.com",
		Priority:  0,
		Status:    "unknown",
		IsEnabled: true,
	}
	svc3 := &model.ExternalService{
		UUID:      testUUID("reorder-003"),
		Name:      "Reorder Three",
		Slug:      "reorder-three",
		Type:      "kiwix",
		BaseURL:   "https://r3.example.com",
		Priority:  0,
		Status:    "unknown",
		IsEnabled: true,
	}

	for _, svc := range []*model.ExternalService{svc1, svc2, svc3} {
		require.NoError(t, repo.Create(ctx, svc))
	}

	// Reorder: svc3 first, svc1 second, svc2 third.
	err := repo.ReorderPriorities(ctx, []int64{svc3.ID, svc1.ID, svc2.ID})
	require.NoError(t, err)

	// Verify priorities via direct SQL.
	var priority int
	err = testPool.QueryRow(ctx,
		`SELECT priority FROM external_services WHERE id = $1`, svc3.ID).Scan(&priority)
	require.NoError(t, err)
	assert.Equal(t, 0, priority, "svc3 should have priority 0")

	err = testPool.QueryRow(ctx,
		`SELECT priority FROM external_services WHERE id = $1`, svc1.ID).Scan(&priority)
	require.NoError(t, err)
	assert.Equal(t, 1, priority, "svc1 should have priority 1")

	err = testPool.QueryRow(ctx,
		`SELECT priority FROM external_services WHERE id = $1`, svc2.ID).Scan(&priority)
	require.NoError(t, err)
	assert.Equal(t, 2, priority, "svc2 should have priority 2")

	t.Run("empty slice", func(t *testing.T) {
		err := repo.ReorderPriorities(ctx, []int64{})
		require.NoError(t, err)
	})
}
