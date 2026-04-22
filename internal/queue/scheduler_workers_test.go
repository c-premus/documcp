package queue

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c-premus/documcp/internal/client/git"
	"github.com/c-premus/documcp/internal/model"
	"github.com/c-premus/documcp/internal/repository"
	"github.com/c-premus/documcp/internal/storage"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func testLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func makeJob[T river.JobArgs](args T) *river.Job[T] {
	return &river.Job[T]{
		JobRow: &rivertype.JobRow{ID: 1},
		Args:   args,
	}
}

// testBlob builds an FSBlob rooted at dir. Used by scheduler worker tests
// that seed on-disk files and expect the worker to iterate them through
// the blob interface.
func testBlob(t *testing.T, dir string) storage.Blob {
	t.Helper()
	b, err := storage.NewFSBlob(dir)
	if err != nil {
		t.Fatalf("NewFSBlob: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return b
}

// ---------------------------------------------------------------------------
// Mock implementations
// ---------------------------------------------------------------------------

type mockExternalServiceHealthChecker struct {
	findAllEnabledFn func(ctx context.Context) ([]model.ExternalService, error)
	updateHealthFn   func(ctx context.Context, id int64, status model.ExternalServiceStatus, latencyMs int, lastError string) error
}

func (m *mockExternalServiceHealthChecker) FindAllEnabled(ctx context.Context) ([]model.ExternalService, error) {
	if m.findAllEnabledFn != nil {
		return m.findAllEnabledFn(ctx)
	}
	return nil, nil
}

func (m *mockExternalServiceHealthChecker) UpdateHealthStatus(ctx context.Context, id int64, status model.ExternalServiceStatus, latencyMs int, lastError string) error {
	if m.updateHealthFn != nil {
		return m.updateHealthFn(ctx, id, status, latencyMs, lastError)
	}
	return nil
}

type mockOAuthTokenPurger struct {
	purgeCount int64
	purgeErr   error
	calledDays int
}

func (m *mockOAuthTokenPurger) PurgeExpiredTokens(_ context.Context, retentionDays int) (int64, error) {
	m.calledDays = retentionDays
	return m.purgeCount, m.purgeErr
}

func (m *mockOAuthTokenPurger) DeleteExpiredScopeGrants(_ context.Context) (int64, error) {
	return 0, nil
}

type mockSearchQueryRetentionPurger struct {
	deleteCount int64
	deleteErr   error
	calledAge   time.Duration
}

func (m *mockSearchQueryRetentionPurger) DeleteOlderThan(_ context.Context, age time.Duration) (int64, error) {
	m.calledAge = age
	return m.deleteCount, m.deleteErr
}

type mockExternalServiceFinder struct {
	findEnabledByTypeFn func(ctx context.Context, serviceType string) ([]model.ExternalService, error)
}

func (m *mockExternalServiceFinder) FindEnabledByType(ctx context.Context, serviceType string) ([]model.ExternalService, error) {
	if m.findEnabledByTypeFn != nil {
		return m.findEnabledByTypeFn(ctx, serviceType)
	}
	return nil, nil
}

type mockGitTemplateRepo struct {
	listFn             func(ctx context.Context, category string, limit, offset int) ([]model.GitTemplate, error)
	updateSyncStatusFn func(ctx context.Context, templateID int64, status model.GitTemplateStatus, commitSHA string, fileCount int, totalSize int64, errMsg string) error
	replaceFilesFn     func(ctx context.Context, templateID int64, files []repository.GitTemplateFileInsert) error
	updateSearchFn     func(ctx context.Context, templateID int64, readmeContent, filePaths string) error
}

func (m *mockGitTemplateRepo) List(ctx context.Context, category string, limit, offset int) ([]model.GitTemplate, error) {
	if m.listFn != nil {
		return m.listFn(ctx, category, limit, offset)
	}
	return nil, nil
}

func (m *mockGitTemplateRepo) UpdateSyncStatus(ctx context.Context, templateID int64, status model.GitTemplateStatus, commitSHA string, fileCount int, totalSize int64, errMsg string) error {
	if m.updateSyncStatusFn != nil {
		return m.updateSyncStatusFn(ctx, templateID, status, commitSHA, fileCount, totalSize, errMsg)
	}
	return nil
}

func (m *mockGitTemplateRepo) ReplaceFiles(ctx context.Context, templateID int64, files []repository.GitTemplateFileInsert) error {
	if m.replaceFilesFn != nil {
		return m.replaceFilesFn(ctx, templateID, files)
	}
	return nil
}

func (m *mockGitTemplateRepo) UpdateSearchContent(ctx context.Context, templateID int64, readmeContent, filePaths string) error {
	if m.updateSearchFn != nil {
		return m.updateSearchFn(ctx, templateID, readmeContent, filePaths)
	}
	return nil
}

type mockDocumentRepo struct {
	activeFilePaths []repository.DocumentFilePath
	activePathsErr  error
	purgedDocs      []repository.DocumentFilePath
	purgeErr        error
	purgeCalledWith time.Duration
}

func (m *mockDocumentRepo) ListActiveFilePaths(_ context.Context) ([]repository.DocumentFilePath, error) {
	return m.activeFilePaths, m.activePathsErr
}

func (m *mockDocumentRepo) PurgeSoftDeleted(_ context.Context, olderThan time.Duration) ([]repository.DocumentFilePath, error) {
	m.purgeCalledWith = olderThan
	return m.purgedDocs, m.purgeErr
}

// ---------------------------------------------------------------------------
// toSyncTemplate
// ---------------------------------------------------------------------------

func TestToSyncTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     model.GitTemplate
		wantErr   bool
		errSubstr string
		check     func(t *testing.T, result git.SyncTemplate)
	}{
		{
			name: "all fields populated",
			input: model.GitTemplate{
				ID:            42,
				UUID:          "abc-123",
				Name:          "my-template",
				Slug:          "my-template",
				Description:   sql.NullString{String: "A description", Valid: true},
				RepositoryURL: "https://github.com/example/repo",
				Branch:        "main",
				GitToken:      sql.NullString{String: "ghp_secret", Valid: true},
				Category:      sql.NullString{String: "web", Valid: true},
				Tags:          json.RawMessage(`["go","api"]`),
				LastCommitSHA: sql.NullString{String: "abc123def", Valid: true},
			},
			check: func(t *testing.T, r git.SyncTemplate) {
				t.Helper()
				assert.Equal(t, int64(42), r.ID)
				assert.Equal(t, "abc-123", r.UUID)
				assert.Equal(t, "my-template", r.Name)
				assert.Equal(t, "my-template", r.Slug)
				assert.Equal(t, "A description", r.Description)
				assert.Equal(t, "https://github.com/example/repo", r.RepositoryURL)
				assert.Equal(t, "main", r.Branch)
				assert.Equal(t, "ghp_secret", r.Token)
				assert.Equal(t, "web", r.Category)
				assert.Equal(t, []string{"go", "api"}, r.Tags)
				assert.Equal(t, "abc123def", r.LastCommitSHA)
			},
		},
		{
			name: "null optional fields",
			input: model.GitTemplate{
				ID:            1,
				UUID:          "uuid-1",
				Name:          "tmpl",
				Slug:          "tmpl",
				RepositoryURL: "https://example.com/repo",
				Branch:        "main",
				Description:   sql.NullString{Valid: false},
				Category:      sql.NullString{Valid: false},
				GitToken:      sql.NullString{Valid: false},
				LastCommitSHA: sql.NullString{Valid: false},
				Tags:          nil,
			},
			check: func(t *testing.T, r git.SyncTemplate) {
				t.Helper()
				assert.Empty(t, r.Description)
				assert.Empty(t, r.Category)
				assert.Empty(t, r.Token)
				assert.Empty(t, r.LastCommitSHA)
				assert.Nil(t, r.Tags)
			},
		},
		{
			name: "valid JSON tags",
			input: model.GitTemplate{
				ID:            2,
				UUID:          "uuid-2",
				Name:          "tmpl2",
				Slug:          "tmpl2",
				RepositoryURL: "https://example.com/repo2",
				Branch:        "dev",
				Tags:          json.RawMessage(`["alpha","beta","gamma"]`),
			},
			check: func(t *testing.T, r git.SyncTemplate) {
				t.Helper()
				assert.Equal(t, []string{"alpha", "beta", "gamma"}, r.Tags)
			},
		},
		{
			name: "empty tags (zero-length RawMessage)",
			input: model.GitTemplate{
				ID:            3,
				UUID:          "uuid-3",
				Name:          "tmpl3",
				Slug:          "tmpl3",
				RepositoryURL: "https://example.com/repo3",
				Branch:        "main",
				Tags:          json.RawMessage{},
			},
			check: func(t *testing.T, r git.SyncTemplate) {
				t.Helper()
				assert.Nil(t, r.Tags)
			},
		},
		{
			name: "invalid JSON tags returns error",
			input: model.GitTemplate{
				ID:            4,
				UUID:          "uuid-4",
				Name:          "tmpl4",
				Slug:          "tmpl4",
				RepositoryURL: "https://example.com/repo4",
				Branch:        "main",
				Tags:          json.RawMessage(`not-json`),
			},
			wantErr:   true,
			errSubstr: "parsing tags for template 4",
		},
		{
			name: "nil tags",
			input: model.GitTemplate{
				ID:            5,
				UUID:          "uuid-5",
				Name:          "tmpl5",
				Slug:          "tmpl5",
				RepositoryURL: "https://example.com/repo5",
				Branch:        "main",
				Tags:          nil,
			},
			check: func(t *testing.T, r git.SyncTemplate) {
				t.Helper()
				assert.Nil(t, r.Tags)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := toSyncTemplate(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)
				return
			}
			require.NoError(t, err)
			if tt.check != nil {
				tt.check(t, result)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// CleanupOAuthTokensWorker
// ---------------------------------------------------------------------------

func TestCleanupOAuthTokensWorker_Work(t *testing.T) {
	t.Parallel()

	t.Run("nil OAuthRepo skips gracefully", func(t *testing.T) {
		t.Parallel()

		worker := &CleanupOAuthTokensWorker{
			Deps: SchedulerDeps{
				OAuthRepo: nil,
				Logger:    testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(CleanupOAuthTokensArgs{}))
		require.NoError(t, err)
	})

	t.Run("success returns purge count", func(t *testing.T) {
		t.Parallel()

		mock := &mockOAuthTokenPurger{purgeCount: 5}
		worker := &CleanupOAuthTokensWorker{
			Deps: SchedulerDeps{
				OAuthRepo: mock,
				Logger:    testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(CleanupOAuthTokensArgs{}))
		require.NoError(t, err)
		assert.Equal(t, 7, mock.calledDays)
	})

	t.Run("PurgeExpiredTokens error is wrapped", func(t *testing.T) {
		t.Parallel()

		mock := &mockOAuthTokenPurger{purgeErr: errors.New("db down")}
		worker := &CleanupOAuthTokensWorker{
			Deps: SchedulerDeps{
				OAuthRepo: mock,
				Logger:    testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(CleanupOAuthTokensArgs{}))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "purging expired OAuth tokens")
		assert.Contains(t, err.Error(), "db down")
	})
}

// ---------------------------------------------------------------------------
// CleanupSearchQueriesWorker
// ---------------------------------------------------------------------------

func TestCleanupSearchQueriesWorker_Work(t *testing.T) {
	t.Parallel()

	t.Run("nil SearchQueryRepo skips gracefully", func(t *testing.T) {
		t.Parallel()

		worker := &CleanupSearchQueriesWorker{
			Deps: SchedulerDeps{
				SearchQueryRepo:      nil,
				SearchQueryRetention: 30 * 24 * time.Hour,
				Logger:               testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(CleanupSearchQueriesArgs{}))
		require.NoError(t, err)
	})

	t.Run("zero retention disables deletion", func(t *testing.T) {
		t.Parallel()

		mock := &mockSearchQueryRetentionPurger{}
		worker := &CleanupSearchQueriesWorker{
			Deps: SchedulerDeps{
				SearchQueryRepo:      mock,
				SearchQueryRetention: 0,
				Logger:               testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(CleanupSearchQueriesArgs{}))
		require.NoError(t, err)
		assert.Equal(t, time.Duration(0), mock.calledAge, "DeleteOlderThan should not be called")
	})

	t.Run("success passes retention through", func(t *testing.T) {
		t.Parallel()

		mock := &mockSearchQueryRetentionPurger{deleteCount: 42}
		worker := &CleanupSearchQueriesWorker{
			Deps: SchedulerDeps{
				SearchQueryRepo:      mock,
				SearchQueryRetention: 90 * 24 * time.Hour,
				Logger:               testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(CleanupSearchQueriesArgs{}))
		require.NoError(t, err)
		assert.Equal(t, 90*24*time.Hour, mock.calledAge)
	})

	t.Run("DeleteOlderThan error is wrapped", func(t *testing.T) {
		t.Parallel()

		mock := &mockSearchQueryRetentionPurger{deleteErr: errors.New("db down")}
		worker := &CleanupSearchQueriesWorker{
			Deps: SchedulerDeps{
				SearchQueryRepo:      mock,
				SearchQueryRetention: 30 * 24 * time.Hour,
				Logger:               testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(CleanupSearchQueriesArgs{}))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "purging search_queries")
		assert.Contains(t, err.Error(), "db down")
	})
}

// ---------------------------------------------------------------------------
// CleanupOrphanedFilesWorker
// ---------------------------------------------------------------------------

func TestCleanupOrphanedFilesWorker_Work(t *testing.T) {
	t.Parallel()

	t.Run("nil DocRepo skips gracefully", func(t *testing.T) {
		t.Parallel()

		worker := &CleanupOrphanedFilesWorker{
			Deps: SchedulerDeps{
				DocRepo: nil,
				Blob:    testBlob(t, t.TempDir()),
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(CleanupOrphanedFilesArgs{}))
		require.NoError(t, err)
	})

	t.Run("nil Blob skips gracefully", func(t *testing.T) {
		t.Parallel()

		worker := &CleanupOrphanedFilesWorker{
			Deps: SchedulerDeps{
				DocRepo: &mockDocumentRepo{},
				Blob:    nil,
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(CleanupOrphanedFilesArgs{}))
		require.NoError(t, err)
	})

	t.Run("success with orphan files removed", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()

		// Create active and orphan files.
		activeFile := filepath.Join(dir, "active.txt")
		orphanFile := filepath.Join(dir, "orphan.txt")
		require.NoError(t, os.WriteFile(activeFile, []byte("keep"), 0o600))
		require.NoError(t, os.WriteFile(orphanFile, []byte("remove"), 0o600))

		mock := &mockDocumentRepo{
			activeFilePaths: []repository.DocumentFilePath{
				{ID: 1, UUID: "uuid-1", FilePath: "active.txt"},
			},
		}

		worker := &CleanupOrphanedFilesWorker{
			Deps: SchedulerDeps{
				DocRepo: mock,
				Blob:    testBlob(t, dir),
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(CleanupOrphanedFilesArgs{}))
		require.NoError(t, err)

		// Active file still exists.
		_, statErr := os.Stat(activeFile)
		require.NoError(t, statErr, "active file should still exist")

		// Orphan file removed.
		_, statErr = os.Stat(orphanFile)
		assert.True(t, os.IsNotExist(statErr), "orphan file should be removed")
	})

	t.Run("ListActiveFilePaths error is returned", func(t *testing.T) {
		t.Parallel()

		mock := &mockDocumentRepo{
			activePathsErr: errors.New("db error"),
		}

		worker := &CleanupOrphanedFilesWorker{
			Deps: SchedulerDeps{
				DocRepo: mock,
				Blob:    testBlob(t, t.TempDir()),
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(CleanupOrphanedFilesArgs{}))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "listing active file paths")
	})

	t.Run("active files preserved orphans deleted", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()

		// Create multiple files.
		file1 := filepath.Join(dir, "doc1.pdf")
		file2 := filepath.Join(dir, "doc2.pdf")
		file3 := filepath.Join(dir, "orphan1.pdf")
		for _, f := range []string{file1, file2, file3} {
			require.NoError(t, os.WriteFile(f, []byte("data"), 0o600))
		}

		mock := &mockDocumentRepo{
			activeFilePaths: []repository.DocumentFilePath{
				{ID: 1, UUID: "u1", FilePath: "doc1.pdf"},
				{ID: 2, UUID: "u2", FilePath: "doc2.pdf"},
			},
		}

		worker := &CleanupOrphanedFilesWorker{
			Deps: SchedulerDeps{
				DocRepo: mock,
				Blob:    testBlob(t, dir),
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(CleanupOrphanedFilesArgs{}))
		require.NoError(t, err)

		// Active files preserved.
		for _, f := range []string{file1, file2} {
			_, statErr := os.Stat(f)
			require.NoError(t, statErr, "active file should exist: %s", f)
		}

		// Orphan deleted.
		_, statErr := os.Stat(file3)
		assert.True(t, os.IsNotExist(statErr), "orphan file should be removed")
	})
}

// ---------------------------------------------------------------------------
// PurgeSoftDeletedWorker
// ---------------------------------------------------------------------------

func TestPurgeSoftDeletedWorker_Work(t *testing.T) {
	t.Parallel()

	t.Run("nil DocRepo skips gracefully", func(t *testing.T) {
		t.Parallel()

		worker := &PurgeSoftDeletedWorker{
			Deps: SchedulerDeps{
				DocRepo: nil,
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(PurgeSoftDeletedArgs{}))
		require.NoError(t, err)
	})

	t.Run("success files removed from disk", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		filePath := filepath.Join(dir, "purged.pdf")
		require.NoError(t, os.WriteFile(filePath, []byte("old"), 0o600))

		docRepo := &mockDocumentRepo{
			purgedDocs: []repository.DocumentFilePath{
				{ID: 1, UUID: "purge-uuid", FilePath: "purged.pdf"},
			},
		}

		worker := &PurgeSoftDeletedWorker{
			Deps: SchedulerDeps{
				DocRepo: docRepo,
				Blob:    testBlob(t, dir),
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(PurgeSoftDeletedArgs{}))
		require.NoError(t, err)

		// File removed from disk.
		_, statErr := os.Stat(filePath)
		assert.True(t, os.IsNotExist(statErr), "purged file should be removed")
	})

	t.Run("PurgeSoftDeleted error returned", func(t *testing.T) {
		t.Parallel()

		docRepo := &mockDocumentRepo{
			purgeErr: errors.New("purge failed"),
		}

		worker := &PurgeSoftDeletedWorker{
			Deps: SchedulerDeps{
				DocRepo: docRepo,
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(PurgeSoftDeletedArgs{}))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "purging soft-deleted documents")
	})

	t.Run("file removal error logged for non-existent file", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		// Do NOT create the file -- it does not exist on disk.
		docRepo := &mockDocumentRepo{
			purgedDocs: []repository.DocumentFilePath{
				{ID: 1, UUID: "gone-uuid", FilePath: "nonexistent.pdf"},
			},
		}

		worker := &PurgeSoftDeletedWorker{
			Deps: SchedulerDeps{
				DocRepo: docRepo,
				Blob:    testBlob(t, dir),
				Logger:  testLogger(),
			},
		}

		// Should not error -- Blob.Delete is idempotent on missing keys.
		err := worker.Work(context.Background(), makeJob(PurgeSoftDeletedArgs{}))
		require.NoError(t, err)
	})

	t.Run("empty file path skips file removal", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		docRepo := &mockDocumentRepo{
			purgedDocs: []repository.DocumentFilePath{
				{ID: 1, UUID: "no-file-uuid", FilePath: ""},
			},
		}

		worker := &PurgeSoftDeletedWorker{
			Deps: SchedulerDeps{
				DocRepo: docRepo,
				Blob:    testBlob(t, dir),
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(PurgeSoftDeletedArgs{}))
		require.NoError(t, err)
	})
}

// ---------------------------------------------------------------------------
// HealthCheckServicesWorker
// ---------------------------------------------------------------------------

func TestHealthCheckServicesWorker_Work(t *testing.T) {
	t.Parallel()

	t.Run("nil HealthChecker skips gracefully", func(t *testing.T) {
		t.Parallel()

		worker := &HealthCheckServicesWorker{
			Deps: SchedulerDeps{
				HealthChecker: nil,
				Logger:        testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(HealthCheckServicesArgs{}))
		require.NoError(t, err)
	})

	t.Run("no enabled services returns nil", func(t *testing.T) {
		t.Parallel()

		checker := &mockExternalServiceHealthChecker{
			findAllEnabledFn: func(_ context.Context) ([]model.ExternalService, error) {
				return []model.ExternalService{}, nil
			},
		}

		worker := &HealthCheckServicesWorker{
			Deps: SchedulerDeps{
				HealthChecker: checker,
				Logger:        testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(HealthCheckServicesArgs{}))
		require.NoError(t, err)
	})

	t.Run("FindAllEnabled error is returned", func(t *testing.T) {
		t.Parallel()

		checker := &mockExternalServiceHealthChecker{
			findAllEnabledFn: func(_ context.Context) ([]model.ExternalService, error) {
				return nil, assert.AnError
			},
		}

		worker := &HealthCheckServicesWorker{
			Deps: SchedulerDeps{
				HealthChecker: checker,
				Logger:        testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(HealthCheckServicesArgs{}))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "finding enabled external services")
	})

	t.Run("service with invalid URL is marked unhealthy", func(t *testing.T) {
		t.Parallel()

		var updatedStatus model.ExternalServiceStatus
		checker := &mockExternalServiceHealthChecker{
			findAllEnabledFn: func(_ context.Context) ([]model.ExternalService, error) {
				return []model.ExternalService{
					{ID: 1, BaseURL: "://invalid-url"},
				}, nil
			},
			updateHealthFn: func(_ context.Context, _ int64, status model.ExternalServiceStatus, _ int, _ string) error {
				updatedStatus = status
				return nil
			},
		}

		worker := &HealthCheckServicesWorker{
			Deps: SchedulerDeps{
				HealthChecker: checker,
				Logger:        testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(HealthCheckServicesArgs{}))
		require.NoError(t, err)
		assert.Equal(t, model.ExternalServiceStatusUnhealthy, updatedStatus)
	})

	t.Run("loopback service blocked by SSRF marked unhealthy", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		var updatedStatus model.ExternalServiceStatus
		checker := &mockExternalServiceHealthChecker{
			findAllEnabledFn: func(_ context.Context) ([]model.ExternalService, error) {
				return []model.ExternalService{
					{ID: 1, BaseURL: srv.URL},
				}, nil
			},
			updateHealthFn: func(_ context.Context, _ int64, status model.ExternalServiceStatus, _ int, _ string) error {
				updatedStatus = status
				return nil
			},
		}

		worker := &HealthCheckServicesWorker{
			Deps: SchedulerDeps{
				HealthChecker: checker,
				Logger:        testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(HealthCheckServicesArgs{}))
		require.NoError(t, err)
		assert.Equal(t, model.ExternalServiceStatusUnhealthy, updatedStatus)
	})

	t.Run("unreachable service marked unhealthy", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		// Close immediately so connections fail.
		srv.Close()

		var updatedStatus model.ExternalServiceStatus
		checker := &mockExternalServiceHealthChecker{
			findAllEnabledFn: func(_ context.Context) ([]model.ExternalService, error) {
				return []model.ExternalService{
					{ID: 1, BaseURL: srv.URL},
				}, nil
			},
			updateHealthFn: func(_ context.Context, _ int64, status model.ExternalServiceStatus, _ int, _ string) error {
				updatedStatus = status
				return nil
			},
		}

		worker := &HealthCheckServicesWorker{
			Deps: SchedulerDeps{
				HealthChecker: checker,
				Logger:        testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(HealthCheckServicesArgs{}))
		require.NoError(t, err)
		assert.Equal(t, model.ExternalServiceStatusUnhealthy, updatedStatus)
	})

	t.Run("multiple services all marked unhealthy via SSRF filter", func(t *testing.T) {
		t.Parallel()

		srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer srv1.Close()

		srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer srv2.Close()

		statuses := make(map[int64]model.ExternalServiceStatus)
		checker := &mockExternalServiceHealthChecker{
			findAllEnabledFn: func(_ context.Context) ([]model.ExternalService, error) {
				return []model.ExternalService{
					{ID: 1, BaseURL: srv1.URL},
					{ID: 2, BaseURL: srv2.URL},
				}, nil
			},
			updateHealthFn: func(_ context.Context, id int64, status model.ExternalServiceStatus, _ int, _ string) error {
				statuses[id] = status
				return nil
			},
		}

		worker := &HealthCheckServicesWorker{
			Deps: SchedulerDeps{
				HealthChecker: checker,
				Logger:        testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(HealthCheckServicesArgs{}))
		require.NoError(t, err)
		// Both are on loopback, so both are blocked by SSRF protection.
		assert.Equal(t, model.ExternalServiceStatusUnhealthy, statuses[1])
		assert.Equal(t, model.ExternalServiceStatusUnhealthy, statuses[2])
	})
}

// ---------------------------------------------------------------------------
// SyncKiwixWorker
// ---------------------------------------------------------------------------

func TestSyncKiwixWorker_Work(t *testing.T) {
	t.Parallel()

	t.Run("no enabled kiwix services returns nil", func(t *testing.T) {
		t.Parallel()

		finder := &mockExternalServiceFinder{
			findEnabledByTypeFn: func(_ context.Context, serviceType string) ([]model.ExternalService, error) {
				assert.Equal(t, "kiwix", serviceType)
				return []model.ExternalService{}, nil
			},
		}

		worker := &SyncKiwixWorker{
			Deps: SchedulerDeps{
				Services: finder,
				Logger:   testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(SyncKiwixArgs{}))
		require.NoError(t, err)
	})

	t.Run("FindEnabledByType error is returned", func(t *testing.T) {
		t.Parallel()

		finder := &mockExternalServiceFinder{
			findEnabledByTypeFn: func(_ context.Context, _ string) ([]model.ExternalService, error) {
				return nil, errors.New("db connection lost")
			},
		}

		worker := &SyncKiwixWorker{
			Deps: SchedulerDeps{
				Services: finder,
				Logger:   testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(SyncKiwixArgs{}))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "finding enabled kiwix services")
		assert.Contains(t, err.Error(), "db connection lost")
	})
}

// ---------------------------------------------------------------------------
// SyncGitTemplatesWorker
// ---------------------------------------------------------------------------

func TestSyncGitTemplatesWorker_Work(t *testing.T) {
	t.Parallel()

	t.Run("no git templates found returns nil", func(t *testing.T) {
		t.Parallel()

		gitRepo := &mockGitTemplateRepo{
			listFn: func(_ context.Context, category string, limit, offset int) ([]model.GitTemplate, error) {
				assert.Empty(t, category)
				assert.Equal(t, 100, limit)
				assert.Equal(t, 0, offset)
				return []model.GitTemplate{}, nil
			},
		}

		worker := &SyncGitTemplatesWorker{
			Deps: SchedulerDeps{
				GitRepo: gitRepo,
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(SyncGitTemplatesArgs{}))
		require.NoError(t, err)
	})

	t.Run("List error is returned", func(t *testing.T) {
		t.Parallel()

		gitRepo := &mockGitTemplateRepo{
			listFn: func(_ context.Context, _ string, _, _ int) ([]model.GitTemplate, error) {
				return nil, errors.New("query timeout")
			},
		}

		worker := &SyncGitTemplatesWorker{
			Deps: SchedulerDeps{
				GitRepo: gitRepo,
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(SyncGitTemplatesArgs{}))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "listing git templates")
		assert.Contains(t, err.Error(), "query timeout")
	})
}
