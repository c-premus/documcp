//go:build integration

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/c-premus/documcp/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c-premus/documcp/internal/model"
)

func TestGitTemplateRepository_Create(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewGitTemplateRepository(testPool, testutil.DiscardLogger(), nil)

	tmpl := &model.GitTemplate{
		UUID:          testUUID("git-tmpl-create-001"),
		Name:          "Go Microservice",
		Slug:          "go-microservice",
		RepositoryURL: "https://github.com/example/go-microservice",
		Branch:        "main",
		Status:        model.GitTemplateStatusPending,
		IsPublic:      true,
		IsEnabled:     true,
	}

	err := repo.Create(ctx, tmpl)
	require.NoError(t, err)
	assert.NotZero(t, tmpl.ID, "ID should be set after insert")
	assert.True(t, tmpl.CreatedAt.Valid, "CreatedAt should be set")
	assert.True(t, tmpl.UpdatedAt.Valid, "UpdatedAt should be set")

	t.Run("with user_id", func(t *testing.T) {
		oauthRepo := NewOAuthRepository(testPool, testutil.DiscardLogger())
		user := &model.User{Name: "Template Owner", Email: "owner@example.com"}
		require.NoError(t, oauthRepo.CreateUser(ctx, user))

		tmplWithUser := &model.GitTemplate{
			UUID:          testUUID("git-tmpl-create-user"),
			Name:          "User Template",
			Slug:          "user-template",
			RepositoryURL: "https://github.com/example/user-template",
			Branch:        "main",
			Status:        model.GitTemplateStatusPending,
			IsPublic:      true,
			IsEnabled:     true,
			UserID:        sql.NullInt64{Int64: user.ID, Valid: true},
		}

		err := repo.Create(ctx, tmplWithUser)
		require.NoError(t, err)
		assert.NotZero(t, tmplWithUser.ID)

		found, err := repo.FindByUUID(ctx, testUUID("git-tmpl-create-user"))
		require.NoError(t, err)
		assert.True(t, found.UserID.Valid)
		assert.Equal(t, user.ID, found.UserID.Int64)
	})

	t.Run("with optional fields", func(t *testing.T) {
		tmplOpt := &model.GitTemplate{
			UUID:          testUUID("git-tmpl-create-optional"),
			Name:          "Optional Fields Template",
			Slug:          "optional-fields-template",
			RepositoryURL: "https://github.com/example/optional",
			Branch:        "develop",
			Status:        model.GitTemplateStatusPending,
			IsPublic:      false,
			IsEnabled:     true,
			Description:   sql.NullString{String: "A template with optional fields", Valid: true},
			Category:      sql.NullString{String: "backend", Valid: true},
			Tags:          json.RawMessage(`["go","api"]`),
		}

		err := repo.Create(ctx, tmplOpt)
		require.NoError(t, err)
		assert.NotZero(t, tmplOpt.ID)

		found, err := repo.FindByUUID(ctx, testUUID("git-tmpl-create-optional"))
		require.NoError(t, err)
		assert.True(t, found.Description.Valid)
		assert.Equal(t, "A template with optional fields", found.Description.String)
		assert.True(t, found.Category.Valid)
		assert.Equal(t, "backend", found.Category.String)
		require.NotEmpty(t, found.Tags)
		assert.JSONEq(t, `["go","api"]`, string(found.Tags))
	})
}

// TestGitTemplateRepository_TagsContributeToSearchVector is a regression
// guard for the migration 000017 STORED search_vector rebuild on
// git_templates. Mirrors the zim_archives guard.
func TestGitTemplateRepository_TagsContributeToSearchVector(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewGitTemplateRepository(testPool, testutil.DiscardLogger(), nil)

	tmpl := &model.GitTemplate{
		UUID:          testUUID("git-tmpl-fts-tags"),
		Name:          "Unrelated Name",
		Slug:          "unrelated-name",
		RepositoryURL: "https://example.com/repo",
		Branch:        "main",
		Status:        model.GitTemplateStatusPending,
		IsEnabled:     true,
		Tags:          json.RawMessage(`["kubernetes","helm"]`),
	}
	require.NoError(t, repo.Create(ctx, tmpl))

	var hitName string
	err := testPool.QueryRow(ctx,
		`SELECT name FROM git_templates
		 WHERE search_vector @@ to_tsquery('documcp_english', 'kubernetes')`,
	).Scan(&hitName)
	require.NoError(t, err, "tag-only FTS query should return the seeded row")
	assert.Equal(t, "Unrelated Name", hitName)
}

func TestGitTemplateRepository_FindByUUID(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewGitTemplateRepository(testPool, testutil.DiscardLogger(), nil)

	tmpl := &model.GitTemplate{
		UUID:          testUUID("git-tmpl-find-uuid"),
		Name:          "Find By UUID Template",
		Slug:          "find-by-uuid-template",
		RepositoryURL: "https://github.com/example/find-uuid",
		Branch:        "main",
		Status:        model.GitTemplateStatusSynced,
		IsPublic:      true,
		IsEnabled:     true,
	}
	require.NoError(t, repo.Create(ctx, tmpl))

	found, err := repo.FindByUUID(ctx, testUUID("git-tmpl-find-uuid"))
	require.NoError(t, err)
	assert.Equal(t, tmpl.ID, found.ID)
	assert.Equal(t, testUUID("git-tmpl-find-uuid"), found.UUID)
	assert.Equal(t, "Find By UUID Template", found.Name)
	assert.Equal(t, "find-by-uuid-template", found.Slug)
	assert.Equal(t, "https://github.com/example/find-uuid", found.RepositoryURL)
	assert.Equal(t, "main", found.Branch)
	assert.Equal(t, model.GitTemplateStatusSynced, found.Status)
	assert.True(t, found.IsPublic)
	assert.True(t, found.IsEnabled)
	assert.True(t, found.CreatedAt.Valid)
	assert.True(t, found.UpdatedAt.Valid)

	t.Run("not found", func(t *testing.T) {
		_, err := repo.FindByUUID(ctx, testUUID("nonexistent-uuid"))
		assert.Error(t, err)
	})

	t.Run("soft-deleted not found", func(t *testing.T) {
		require.NoError(t, repo.SoftDelete(ctx, tmpl.ID))

		_, err := repo.FindByUUID(ctx, testUUID("git-tmpl-find-uuid"))
		assert.Error(t, err)
	})

	t.Run("disabled not found", func(t *testing.T) {
		disabledTmpl := &model.GitTemplate{
			UUID:          testUUID("git-tmpl-find-uuid-disabled"),
			Name:          "Disabled Template",
			Slug:          "disabled-template",
			RepositoryURL: "https://github.com/example/disabled",
			Branch:        "main",
			Status:        model.GitTemplateStatusPending,
			IsPublic:      true,
			IsEnabled:     false,
		}
		require.NoError(t, repo.Create(ctx, disabledTmpl))

		_, err := repo.FindByUUID(ctx, testUUID("git-tmpl-find-uuid-disabled"))
		assert.Error(t, err)
	})
}

func TestGitTemplateRepository_List(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewGitTemplateRepository(testPool, testutil.DiscardLogger(), nil)

	// Create 3 enabled templates: 2 backend, 1 frontend.
	templates := []model.GitTemplate{
		{
			UUID:          testUUID("git-tmpl-list-be1"),
			Name:          "Backend One",
			Slug:          "backend-one",
			RepositoryURL: "https://github.com/example/be1",
			Branch:        "main",
			Status:        model.GitTemplateStatusSynced,
			IsPublic:      true,
			IsEnabled:     true,
			Category:      sql.NullString{String: "backend", Valid: true},
		},
		{
			UUID:          testUUID("git-tmpl-list-be2"),
			Name:          "Backend Two",
			Slug:          "backend-two",
			RepositoryURL: "https://github.com/example/be2",
			Branch:        "main",
			Status:        model.GitTemplateStatusSynced,
			IsPublic:      true,
			IsEnabled:     true,
			Category:      sql.NullString{String: "backend", Valid: true},
		},
		{
			UUID:          testUUID("git-tmpl-list-fe1"),
			Name:          "Frontend One",
			Slug:          "frontend-one",
			RepositoryURL: "https://github.com/example/fe1",
			Branch:        "main",
			Status:        model.GitTemplateStatusSynced,
			IsPublic:      true,
			IsEnabled:     true,
			Category:      sql.NullString{String: "frontend", Valid: true},
		},
	}
	for i := range templates {
		require.NoError(t, repo.Create(ctx, &templates[i]))
	}

	// Create a disabled template.
	disabledTmpl := &model.GitTemplate{
		UUID:          testUUID("git-tmpl-list-disabled"),
		Name:          "Disabled Template",
		Slug:          "disabled-template",
		RepositoryURL: "https://github.com/example/disabled",
		Branch:        "main",
		Status:        model.GitTemplateStatusPending,
		IsPublic:      true,
		IsEnabled:     false,
		Category:      sql.NullString{String: "backend", Valid: true},
	}
	require.NoError(t, repo.Create(ctx, disabledTmpl))

	// Create a soft-deleted template.
	deletedTmpl := &model.GitTemplate{
		UUID:          testUUID("git-tmpl-list-deleted"),
		Name:          "Deleted Template",
		Slug:          "deleted-template",
		RepositoryURL: "https://github.com/example/deleted",
		Branch:        "main",
		Status:        model.GitTemplateStatusSynced,
		IsPublic:      true,
		IsEnabled:     true,
		Category:      sql.NullString{String: "backend", Valid: true},
	}
	require.NoError(t, repo.Create(ctx, deletedTmpl))
	require.NoError(t, repo.SoftDelete(ctx, deletedTmpl.ID))

	t.Run("all enabled non-deleted", func(t *testing.T) {
		results, err := repo.List(ctx, "", 0, 0)
		require.NoError(t, err)
		assert.Len(t, results, 3, "should exclude disabled and soft-deleted")
	})

	t.Run("filter by category", func(t *testing.T) {
		results, err := repo.List(ctx, "backend", 0, 0)
		require.NoError(t, err)
		assert.Len(t, results, 2)
		for _, r := range results {
			assert.Equal(t, "backend", r.Category.String)
		}
	})

	t.Run("limit", func(t *testing.T) {
		results, err := repo.List(ctx, "", 1, 0)
		require.NoError(t, err)
		assert.Len(t, results, 1)
	})
}

func TestGitTemplateRepository_Count(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewGitTemplateRepository(testPool, testutil.DiscardLogger(), nil)

	// Create 3 templates.
	for i := range 3 {
		tmpl := &model.GitTemplate{
			UUID:          testUUID("git-tmpl-count-" + string(rune('a'+i))),
			Name:          "Count Template " + string(rune('A'+i)),
			Slug:          "count-template-" + string(rune('a'+i)),
			RepositoryURL: "https://github.com/example/count-" + string(rune('a'+i)),
			Branch:        "main",
			Status:        model.GitTemplateStatusSynced,
			IsPublic:      true,
			IsEnabled:     true,
		}
		require.NoError(t, repo.Create(ctx, tmpl))

		// Soft-delete the last one.
		if i == 2 {
			require.NoError(t, repo.SoftDelete(ctx, tmpl.ID))
		}
	}

	count, err := repo.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, count, "should exclude soft-deleted")
}

func TestGitTemplateRepository_Update(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewGitTemplateRepository(testPool, testutil.DiscardLogger(), nil)

	tmpl := &model.GitTemplate{
		UUID:          testUUID("git-tmpl-update"),
		Name:          "Original Name",
		Slug:          "original-slug",
		RepositoryURL: "https://github.com/example/update",
		Branch:        "main",
		Status:        model.GitTemplateStatusPending,
		IsPublic:      true,
		IsEnabled:     true,
	}
	require.NoError(t, repo.Create(ctx, tmpl))

	tmpl.Name = "Updated Name"
	tmpl.Slug = "updated-slug"
	tmpl.Description = sql.NullString{String: "Now with a description", Valid: true}

	err := repo.Update(ctx, tmpl)
	require.NoError(t, err)

	found, err := repo.FindByUUID(ctx, testUUID("git-tmpl-update"))
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", found.Name)
	assert.Equal(t, "updated-slug", found.Slug)
	assert.True(t, found.Description.Valid)
	assert.Equal(t, "Now with a description", found.Description.String)
	assert.True(t, found.UpdatedAt.Time.After(found.CreatedAt.Time) || found.UpdatedAt.Time.Equal(found.CreatedAt.Time))

	t.Run("cannot update soft-deleted", func(t *testing.T) {
		require.NoError(t, repo.SoftDelete(ctx, tmpl.ID))

		tmpl.Name = "Should Not Persist"
		err := repo.Update(ctx, tmpl)
		// Update does not return an error for 0 rows affected.
		require.NoError(t, err)

		// FindByUUID should fail because the template is soft-deleted.
		_, err = repo.FindByUUID(ctx, testUUID("git-tmpl-update"))
		assert.Error(t, err)
	})
}

func TestGitTemplateRepository_SoftDelete(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewGitTemplateRepository(testPool, testutil.DiscardLogger(), nil)

	tmpl := &model.GitTemplate{
		UUID:          testUUID("git-tmpl-softdelete"),
		Name:          "To Be Deleted",
		Slug:          "to-be-deleted",
		RepositoryURL: "https://github.com/example/softdelete",
		Branch:        "main",
		Status:        model.GitTemplateStatusSynced,
		IsPublic:      true,
		IsEnabled:     true,
	}
	require.NoError(t, repo.Create(ctx, tmpl))

	err := repo.SoftDelete(ctx, tmpl.ID)
	require.NoError(t, err)

	_, err = repo.FindByUUID(ctx, testUUID("git-tmpl-softdelete"))
	assert.Error(t, err)

	t.Run("idempotent", func(t *testing.T) {
		// SoftDelete again on already-deleted template (WHERE deleted_at IS NULL
		// matches 0 rows, but no SQL error).
		err := repo.SoftDelete(ctx, tmpl.ID)
		assert.NoError(t, err)
	})
}

func TestGitTemplateRepository_UpdateSyncStatus(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewGitTemplateRepository(testPool, testutil.DiscardLogger(), nil)

	tmpl := &model.GitTemplate{
		UUID:          testUUID("git-tmpl-sync"),
		Name:          "Sync Template",
		Slug:          "sync-template",
		RepositoryURL: "https://github.com/example/sync",
		Branch:        "main",
		Status:        model.GitTemplateStatusPending,
		IsPublic:      true,
		IsEnabled:     true,
	}
	require.NoError(t, repo.Create(ctx, tmpl))

	err := repo.UpdateSyncStatus(ctx, tmpl.ID, model.GitTemplateStatusSynced, "abc123sha", 10, 5000, "")
	require.NoError(t, err)

	// Verify fields via direct query since FindByUUID is sufficient here (enabled + not deleted).
	found, err := repo.FindByUUID(ctx, testUUID("git-tmpl-sync"))
	require.NoError(t, err)
	assert.Equal(t, model.GitTemplateStatusSynced, found.Status)
	assert.True(t, found.LastCommitSHA.Valid)
	assert.Equal(t, "abc123sha", found.LastCommitSHA.String)
	assert.Equal(t, 10, found.FileCount)
	assert.Equal(t, int64(5000), found.TotalSizeBytes)
	assert.False(t, found.ErrorMessage.Valid, "error_message should be NULL when empty string passed")
	assert.True(t, found.LastSyncedAt.Valid)

	t.Run("with error message", func(t *testing.T) {
		err := repo.UpdateSyncStatus(ctx, tmpl.ID, model.GitTemplateStatusFailed, "def456sha", 0, 0, "clone failed: timeout")
		require.NoError(t, err)

		var status string
		var errMsg sql.NullString
		scanErr := testPool.QueryRow(ctx,
			`SELECT status, error_message FROM git_templates WHERE id = $1`, tmpl.ID,
		).Scan(&status, &errMsg)
		require.NoError(t, scanErr)
		assert.Equal(t, "failed", status)
		assert.True(t, errMsg.Valid)
		assert.Equal(t, "clone failed: timeout", errMsg.String)
	})
}

func TestGitTemplateRepository_ReplaceFiles(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewGitTemplateRepository(testPool, testutil.DiscardLogger(), nil)

	tmpl := &model.GitTemplate{
		UUID:          testUUID("git-tmpl-replacefiles"),
		Name:          "Replace Files Template",
		Slug:          "replace-files-template",
		RepositoryURL: "https://github.com/example/replacefiles",
		Branch:        "main",
		Status:        model.GitTemplateStatusSynced,
		IsPublic:      true,
		IsEnabled:     true,
	}
	require.NoError(t, repo.Create(ctx, tmpl))

	// Insert first batch of 2 files.
	firstBatch := []GitTemplateFileInsert{
		{
			Path:        "cmd/main.go",
			Filename:    "main.go",
			Extension:   ".go",
			Content:     "package main",
			ContentHash: "hash1",
			SizeBytes:   12,
			IsEssential: true,
			Variables:   []string{"PROJECT_NAME"},
		},
		{
			Path:        "go.mod",
			Filename:    "go.mod",
			Extension:   ".mod",
			Content:     "module example",
			ContentHash: "hash2",
			SizeBytes:   14,
			IsEssential: false,
		},
	}

	err := repo.ReplaceFiles(ctx, tmpl.ID, firstBatch)
	require.NoError(t, err)

	files, err := repo.FilesForTemplate(ctx, tmpl.ID)
	require.NoError(t, err)
	assert.Len(t, files, 2)

	// Files should be ordered by path: "cmd/main.go" then "go.mod".
	assert.Equal(t, "cmd/main.go", files[0].Path)
	assert.Equal(t, "go.mod", files[1].Path)

	// Replace with a different single file.
	secondBatch := []GitTemplateFileInsert{
		{
			Path:        "README.md",
			Filename:    "README.md",
			Extension:   ".md",
			Content:     "# Example",
			ContentHash: "hash3",
			SizeBytes:   9,
			IsEssential: true,
		},
	}

	err = repo.ReplaceFiles(ctx, tmpl.ID, secondBatch)
	require.NoError(t, err)

	files, err = repo.FilesForTemplate(ctx, tmpl.ID)
	require.NoError(t, err)
	assert.Len(t, files, 1, "old files should be deleted")
	assert.Equal(t, "README.md", files[0].Path)
}

func TestGitTemplateRepository_FilesForTemplate(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewGitTemplateRepository(testPool, testutil.DiscardLogger(), nil)

	tmpl := &model.GitTemplate{
		UUID:          testUUID("git-tmpl-files"),
		Name:          "Files Template",
		Slug:          "files-template",
		RepositoryURL: "https://github.com/example/files",
		Branch:        "main",
		Status:        model.GitTemplateStatusSynced,
		IsPublic:      true,
		IsEnabled:     true,
	}
	require.NoError(t, repo.Create(ctx, tmpl))

	insertFiles := []GitTemplateFileInsert{
		{
			Path:        "internal/handler/routes.go",
			Filename:    "routes.go",
			Extension:   ".go",
			Content:     "package handler",
			ContentHash: "hash-routes",
			SizeBytes:   15,
			IsEssential: true,
			Variables:   []string{"APP_NAME", "PORT"},
		},
		{
			Path:        "Dockerfile",
			Filename:    "Dockerfile",
			Extension:   "",
			Content:     "FROM golang:1.25",
			ContentHash: "hash-docker",
			SizeBytes:   16,
			IsEssential: false,
		},
		{
			Path:        "cmd/server/main.go",
			Filename:    "main.go",
			Extension:   ".go",
			Content:     "package main\nfunc main() {}",
			ContentHash: "hash-main",
			SizeBytes:   27,
			IsEssential: true,
		},
	}
	require.NoError(t, repo.ReplaceFiles(ctx, tmpl.ID, insertFiles))

	files, err := repo.FilesForTemplate(ctx, tmpl.ID)
	require.NoError(t, err)
	require.Len(t, files, 3)

	// Verify ordering by path: "Dockerfile", "cmd/server/main.go", "internal/handler/routes.go".
	assert.Equal(t, "Dockerfile", files[0].Path)
	assert.Equal(t, "Dockerfile", files[0].Filename)
	assert.False(t, files[0].Extension.Valid, "empty extension should be NULL")
	assert.True(t, files[0].Content.Valid)
	assert.Equal(t, "FROM golang:1.25", files[0].Content.String)
	assert.Equal(t, int64(16), files[0].SizeBytes)
	assert.False(t, files[0].IsEssential)

	assert.Equal(t, "cmd/server/main.go", files[1].Path)
	assert.Equal(t, "main.go", files[1].Filename)
	assert.True(t, files[1].Extension.Valid)
	assert.Equal(t, ".go", files[1].Extension.String)
	assert.True(t, files[1].IsEssential)

	assert.Equal(t, "internal/handler/routes.go", files[2].Path)
	assert.Equal(t, "routes.go", files[2].Filename)
	assert.Equal(t, int64(15), files[2].SizeBytes)
	assert.True(t, files[2].IsEssential)
}

func TestGitTemplateRepository_FindFileByPath(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewGitTemplateRepository(testPool, testutil.DiscardLogger(), nil)

	tmpl := &model.GitTemplate{
		UUID:          testUUID("git-tmpl-findfile"),
		Name:          "Find File Template",
		Slug:          "find-file-template",
		RepositoryURL: "https://github.com/example/findfile",
		Branch:        "main",
		Status:        model.GitTemplateStatusSynced,
		IsPublic:      true,
		IsEnabled:     true,
	}
	require.NoError(t, repo.Create(ctx, tmpl))

	insertFiles := []GitTemplateFileInsert{
		{
			Path:        "main.go",
			Filename:    "main.go",
			Extension:   ".go",
			Content:     "package main",
			ContentHash: "hash-main",
			SizeBytes:   12,
			IsEssential: true,
		},
		{
			Path:        "config/config.yaml",
			Filename:    "config.yaml",
			Extension:   ".yaml",
			Content:     "port: 8080",
			ContentHash: "hash-config",
			SizeBytes:   10,
			IsEssential: false,
		},
	}
	require.NoError(t, repo.ReplaceFiles(ctx, tmpl.ID, insertFiles))

	found, err := repo.FindFileByPath(ctx, tmpl.ID, "main.go")
	require.NoError(t, err)
	assert.Equal(t, "main.go", found.Path)
	assert.Equal(t, "main.go", found.Filename)
	assert.True(t, found.Content.Valid)
	assert.Equal(t, "package main", found.Content.String)
	assert.True(t, found.IsEssential)

	t.Run("not found", func(t *testing.T) {
		_, err := repo.FindFileByPath(ctx, tmpl.ID, "nonexistent.go")
		assert.Error(t, err)
	})
}

func TestGitTemplateRepository_Search(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewGitTemplateRepository(testPool, testutil.DiscardLogger(), nil)

	templates := []model.GitTemplate{
		{
			UUID:          testUUID("git-tmpl-search-1"),
			Name:          "React Dashboard",
			Slug:          "react-dashboard",
			RepositoryURL: "https://github.com/example/react-dash",
			Branch:        "main",
			Status:        model.GitTemplateStatusSynced,
			IsPublic:      true,
			IsEnabled:     true,
			Category:      sql.NullString{String: "frontend", Valid: true},
		},
		{
			UUID:          testUUID("git-tmpl-search-2"),
			Name:          "Go API Server",
			Slug:          "go-api-server",
			RepositoryURL: "https://github.com/example/go-api",
			Branch:        "main",
			Status:        model.GitTemplateStatusSynced,
			IsPublic:      true,
			IsEnabled:     true,
			Description:   sql.NullString{String: "A REST API with react admin panel", Valid: true},
			Category:      sql.NullString{String: "backend", Valid: true},
		},
		{
			UUID:          testUUID("git-tmpl-search-3"),
			Name:          "Python CLI",
			Slug:          "python-cli",
			RepositoryURL: "https://github.com/example/python-cli",
			Branch:        "main",
			Status:        model.GitTemplateStatusSynced,
			IsPublic:      true,
			IsEnabled:     true,
			ReadmeContent: sql.NullString{String: "Uses react-inspired component architecture", Valid: true},
			Category:      sql.NullString{String: "tools", Valid: true},
		},
	}
	for i := range templates {
		require.NoError(t, repo.Create(ctx, &templates[i]))
	}

	// Create a disabled template with "react" in name -- should NOT appear in Search.
	disabledTmpl := &model.GitTemplate{
		UUID:          testUUID("git-tmpl-search-disabled"),
		Name:          "React Disabled",
		Slug:          "react-disabled",
		RepositoryURL: "https://github.com/example/react-disabled",
		Branch:        "main",
		Status:        model.GitTemplateStatusPending,
		IsPublic:      true,
		IsEnabled:     false,
		Category:      sql.NullString{String: "frontend", Valid: true},
	}
	require.NoError(t, repo.Create(ctx, disabledTmpl))

	// Create a soft-deleted template with "react" in name -- should NOT appear in Search.
	deletedTmpl := &model.GitTemplate{
		UUID:          testUUID("git-tmpl-search-deleted"),
		Name:          "React Deleted",
		Slug:          "react-deleted",
		RepositoryURL: "https://github.com/example/react-deleted",
		Branch:        "main",
		Status:        model.GitTemplateStatusSynced,
		IsPublic:      true,
		IsEnabled:     true,
		Category:      sql.NullString{String: "frontend", Valid: true},
	}
	require.NoError(t, repo.Create(ctx, deletedTmpl))
	require.NoError(t, repo.SoftDelete(ctx, deletedTmpl.ID))

	t.Run("matches name description and readme", func(t *testing.T) {
		results, err := repo.Search(ctx, "react", "", 0)
		require.NoError(t, err)
		assert.Len(t, results, 3, "should match name, description, and readme_content")
	})

	t.Run("with category filter", func(t *testing.T) {
		results, err := repo.Search(ctx, "react", "frontend", 0)
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "React Dashboard", results[0].Name)
	})

	t.Run("with limit", func(t *testing.T) {
		results, err := repo.Search(ctx, "react", "", 2)
		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("no matches", func(t *testing.T) {
		results, err := repo.Search(ctx, "nonexistent-query-xyz", "", 0)
		require.NoError(t, err)
		assert.Len(t, results, 0)
	})
}
