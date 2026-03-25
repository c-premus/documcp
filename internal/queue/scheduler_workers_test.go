package queue

import (
	"context"
	"database/sql"
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
	"github.com/c-premus/documcp/internal/search"
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

// ---------------------------------------------------------------------------
// Mock implementations
// ---------------------------------------------------------------------------

type mockExternalServiceFinder struct {
	findEnabledByTypeFn func(ctx context.Context, serviceType string) ([]model.ExternalService, error)
}

func (m *mockExternalServiceFinder) FindEnabledByType(ctx context.Context, serviceType string) ([]model.ExternalService, error) {
	if m.findEnabledByTypeFn != nil {
		return m.findEnabledByTypeFn(ctx, serviceType)
	}
	return nil, nil
}

type mockExternalServiceHealthChecker struct {
	findAllEnabledFn func(ctx context.Context) ([]model.ExternalService, error)
	updateHealthFn   func(ctx context.Context, id int64, status string, latencyMs int, lastError string) error
}

func (m *mockExternalServiceHealthChecker) FindAllEnabled(ctx context.Context) ([]model.ExternalService, error) {
	if m.findAllEnabledFn != nil {
		return m.findAllEnabledFn(ctx)
	}
	return nil, nil
}

func (m *mockExternalServiceHealthChecker) UpdateHealthStatus(ctx context.Context, id int64, status string, latencyMs int, lastError string) error {
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

type mockDocumentRepo struct {
	activeFilePaths []repository.DocumentFilePath
	activePathsErr  error
	allUUIDs        []string
	allUUIDsErr     error
	foundDocs       []model.Document
	findByUUIDsErr  error
	tagsForDoc      []model.DocumentTag
	tagsForDocErr   error
	purgedDocs      []repository.DocumentFilePath
	purgeErr        error
	purgeCalledWith time.Duration
}

func (m *mockDocumentRepo) ListActiveFilePaths(_ context.Context) ([]repository.DocumentFilePath, error) {
	return m.activeFilePaths, m.activePathsErr
}

func (m *mockDocumentRepo) ListAllUUIDs(_ context.Context) ([]string, error) {
	return m.allUUIDs, m.allUUIDsErr
}

func (m *mockDocumentRepo) FindByUUIDs(_ context.Context, _ []string) ([]model.Document, error) {
	return m.foundDocs, m.findByUUIDsErr
}

func (m *mockDocumentRepo) TagsForDocument(_ context.Context, _ int64) ([]model.DocumentTag, error) {
	return m.tagsForDoc, m.tagsForDocErr
}

func (m *mockDocumentRepo) PurgeSoftDeleted(_ context.Context, olderThan time.Duration) ([]repository.DocumentFilePath, error) {
	m.purgeCalledWith = olderThan
	return m.purgedDocs, m.purgeErr
}

type mockZimArchiveRepo struct {
	disabledArchives []model.ZimArchive
	findDisabledErr  error
}

func (m *mockZimArchiveRepo) FindDisabled(_ context.Context) ([]model.ZimArchive, error) {
	return m.disabledArchives, m.findDisabledErr
}

func (m *mockZimArchiveRepo) UpsertFromCatalog(_ context.Context, _ int64, _ repository.ZimArchiveUpsert) error {
	return nil
}

func (m *mockZimArchiveRepo) DisableOrphaned(_ context.Context, _ int64, _ []string) (int, error) {
	return 0, nil
}

func (m *mockZimArchiveRepo) ListAllUUIDs(_ context.Context) ([]string, error) {
	return nil, nil
}

func (m *mockZimArchiveRepo) FindByUUIDs(_ context.Context, _ []string) ([]model.ZimArchive, error) {
	return nil, nil
}

type mockGitTemplateRepo struct {
	templates []model.GitTemplate
	listErr   error
}

func (m *mockGitTemplateRepo) List(_ context.Context, _ string, _, _ int) ([]model.GitTemplate, error) {
	return m.templates, m.listErr
}

func (m *mockGitTemplateRepo) UpdateSyncStatus(_ context.Context, _ int64, _, _ string, _ int, _ int64, _ string) error {
	return nil
}

func (m *mockGitTemplateRepo) ReplaceFiles(_ context.Context, _ int64, _ []repository.GitTemplateFileInsert) error {
	return nil
}

func (m *mockGitTemplateRepo) ListAllUUIDs(_ context.Context) ([]string, error) {
	return nil, nil
}

func (m *mockGitTemplateRepo) FindByUUIDs(_ context.Context, _ []string) ([]model.GitTemplate, error) {
	return nil, nil
}

type mockSearchIndexer struct {
	indexedUUIDs       map[string]bool
	listUUIDsErr       error
	deleteDocErr       error
	deleteZimErr       error
	deletedDocUUIDs    []string
	deletedZimUUIDs    []string
	indexedDocRecords  []search.DocumentRecord
	indexDocErr        error
}

func (m *mockSearchIndexer) ListIndexedDocumentUUIDs(_ context.Context) (map[string]bool, error) {
	return m.indexedUUIDs, m.listUUIDsErr
}

func (m *mockSearchIndexer) DeleteDocument(_ context.Context, uuid string) error {
	m.deletedDocUUIDs = append(m.deletedDocUUIDs, uuid)
	return m.deleteDocErr
}

func (m *mockSearchIndexer) DeleteZimArchive(_ context.Context, uuid string) error {
	m.deletedZimUUIDs = append(m.deletedZimUUIDs, uuid)
	return m.deleteZimErr
}

func (m *mockSearchIndexer) IndexDocument(_ context.Context, doc search.DocumentRecord) error {
	m.indexedDocRecords = append(m.indexedDocRecords, doc)
	return m.indexDocErr
}

func (m *mockSearchIndexer) IndexZimArchive(_ context.Context, _ search.ZimArchiveRecord) error {
	return nil
}

func (m *mockSearchIndexer) IndexGitTemplate(_ context.Context, _ search.GitTemplateRecord) error {
	return nil
}

func (m *mockSearchIndexer) ListIndexedZimUUIDs(_ context.Context) (map[string]bool, error) {
	return nil, nil
}

func (m *mockSearchIndexer) ListIndexedGitTemplateUUIDs(_ context.Context) (map[string]bool, error) {
	return nil, nil
}

func (m *mockSearchIndexer) DeleteGitTemplate(_ context.Context, _ string) error {
	return nil
}

// --- Enhanced mocks for VerifySearchIndex zim/git branches ---

// mockZimArchiveRepoWithUUIDs wraps mockZimArchiveRepo but returns configurable UUIDs.
type mockZimArchiveRepoWithUUIDs struct {
	*mockZimArchiveRepo
	allUUIDs    []string
	allUUIDsErr error
}

func (m *mockZimArchiveRepoWithUUIDs) ListAllUUIDs(_ context.Context) ([]string, error) {
	return m.allUUIDs, m.allUUIDsErr
}

func (m *mockZimArchiveRepoWithUUIDs) FindByUUIDs(_ context.Context, _ []string) ([]model.ZimArchive, error) {
	return nil, nil
}

// mockGitTemplateRepoWithUUIDs wraps mockGitTemplateRepo but returns configurable UUIDs.
type mockGitTemplateRepoWithUUIDs struct {
	*mockGitTemplateRepo
	allUUIDs    []string
	allUUIDsErr error
}

func (m *mockGitTemplateRepoWithUUIDs) ListAllUUIDs(_ context.Context) ([]string, error) {
	return m.allUUIDs, m.allUUIDsErr
}

func (m *mockGitTemplateRepoWithUUIDs) FindByUUIDs(_ context.Context, _ []string) ([]model.GitTemplate, error) {
	return nil, nil
}

// mockSearchIndexerFull extends the search indexer mock with configurable responses
// for zim and git template UUID listing.
type mockSearchIndexerFull struct {
	docUUIDs       map[string]bool
	zimUUIDs       map[string]bool
	gitUUIDs       map[string]bool
	listUUIDsErr   error
	listZimUUIDErr error
	listGitUUIDErr error
	deleteDocErr   error
	deleteZimErr   error
	deleteGitErr   error

	deletedDocUUIDs []string
	deletedZimUUIDs []string
	deletedGitUUIDs []string
}

func (m *mockSearchIndexerFull) ListIndexedDocumentUUIDs(_ context.Context) (map[string]bool, error) {
	return m.docUUIDs, m.listUUIDsErr
}

func (m *mockSearchIndexerFull) ListIndexedZimUUIDs(_ context.Context) (map[string]bool, error) {
	return m.zimUUIDs, m.listZimUUIDErr
}

func (m *mockSearchIndexerFull) ListIndexedGitTemplateUUIDs(_ context.Context) (map[string]bool, error) {
	return m.gitUUIDs, m.listGitUUIDErr
}

func (m *mockSearchIndexerFull) DeleteDocument(_ context.Context, uuid string) error {
	m.deletedDocUUIDs = append(m.deletedDocUUIDs, uuid)
	return m.deleteDocErr
}

func (m *mockSearchIndexerFull) DeleteZimArchive(_ context.Context, uuid string) error {
	m.deletedZimUUIDs = append(m.deletedZimUUIDs, uuid)
	return m.deleteZimErr
}

func (m *mockSearchIndexerFull) DeleteGitTemplate(_ context.Context, uuid string) error {
	m.deletedGitUUIDs = append(m.deletedGitUUIDs, uuid)
	return m.deleteGitErr
}

func (m *mockSearchIndexerFull) IndexDocument(_ context.Context, _ search.DocumentRecord) error {
	return nil
}

func (m *mockSearchIndexerFull) IndexZimArchive(_ context.Context, _ search.ZimArchiveRecord) error {
	return nil
}

func (m *mockSearchIndexerFull) IndexGitTemplate(_ context.Context, _ search.GitTemplateRecord) error {
	return nil
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
				Tags:          sql.NullString{String: `["go","api"]`, Valid: true},
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
				Tags:          sql.NullString{Valid: false},
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
				Tags:          sql.NullString{String: `["alpha","beta","gamma"]`, Valid: true},
			},
			check: func(t *testing.T, r git.SyncTemplate) {
				t.Helper()
				assert.Equal(t, []string{"alpha", "beta", "gamma"}, r.Tags)
			},
		},
		{
			name: "empty tags string",
			input: model.GitTemplate{
				ID:            3,
				UUID:          "uuid-3",
				Name:          "tmpl3",
				Slug:          "tmpl3",
				RepositoryURL: "https://example.com/repo3",
				Branch:        "main",
				Tags:          sql.NullString{String: "", Valid: true},
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
				Tags:          sql.NullString{String: `not-json`, Valid: true},
			},
			wantErr:   true,
			errSubstr: "parsing tags for template 4",
		},
		{
			name: "nil tags (Valid=false)",
			input: model.GitTemplate{
				ID:            5,
				UUID:          "uuid-5",
				Name:          "tmpl5",
				Slug:          "tmpl5",
				RepositoryURL: "https://example.com/repo5",
				Branch:        "main",
				Tags:          sql.NullString{Valid: false},
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
// CleanupOrphanedFilesWorker
// ---------------------------------------------------------------------------

func TestCleanupOrphanedFilesWorker_Work(t *testing.T) {
	t.Parallel()

	t.Run("nil DocRepo skips gracefully", func(t *testing.T) {
		t.Parallel()

		worker := &CleanupOrphanedFilesWorker{
			Deps: SchedulerDeps{
				DocRepo:     nil,
				StoragePath: "/tmp/test",
				Logger:      testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(CleanupOrphanedFilesArgs{}))
		require.NoError(t, err)
	})

	t.Run("empty StoragePath skips gracefully", func(t *testing.T) {
		t.Parallel()

		worker := &CleanupOrphanedFilesWorker{
			Deps: SchedulerDeps{
				DocRepo:     &mockDocumentRepo{},
				StoragePath: "",
				Logger:      testLogger(),
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
				DocRepo:     mock,
				StoragePath: dir,
				Logger:      testLogger(),
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
				DocRepo:     mock,
				StoragePath: t.TempDir(),
				Logger:      testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(CleanupOrphanedFilesArgs{}))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "listing active file paths")
	})

	t.Run("non-existent storage path returns walk error", func(t *testing.T) {
		t.Parallel()

		mock := &mockDocumentRepo{
			activeFilePaths: []repository.DocumentFilePath{},
		}

		worker := &CleanupOrphanedFilesWorker{
			Deps: SchedulerDeps{
				DocRepo:     mock,
				StoragePath: filepath.Join(t.TempDir(), "does-not-exist"),
				Logger:      testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(CleanupOrphanedFilesArgs{}))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "walking storage directory")
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
				DocRepo:     mock,
				StoragePath: dir,
				Logger:      testLogger(),
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
// VerifySearchIndexWorker
// ---------------------------------------------------------------------------

func TestVerifySearchIndexWorker_Work(t *testing.T) {
	t.Parallel()

	t.Run("nil DocRepo skips gracefully", func(t *testing.T) {
		t.Parallel()

		worker := &VerifySearchIndexWorker{
			Deps: SchedulerDeps{
				DocRepo: nil,
				Indexer: nil,
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(VerifySearchIndexArgs{}))
		require.NoError(t, err)
	})

	t.Run("nil Indexer skips gracefully", func(t *testing.T) {
		t.Parallel()

		worker := &VerifySearchIndexWorker{
			Deps: SchedulerDeps{
				DocRepo: &mockDocumentRepo{},
				Indexer: nil,
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(VerifySearchIndexArgs{}))
		require.NoError(t, err)
	})

	t.Run("all docs in index no deletions", func(t *testing.T) {
		t.Parallel()

		indexer := &mockSearchIndexer{
			indexedUUIDs: map[string]bool{"u1": true, "u2": true},
		}
		docRepo := &mockDocumentRepo{
			allUUIDs: []string{"u1", "u2"},
		}

		worker := &VerifySearchIndexWorker{
			Deps: SchedulerDeps{
				DocRepo: docRepo,
				Indexer: indexer,
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(VerifySearchIndexArgs{}))
		require.NoError(t, err)
		assert.Empty(t, indexer.deletedDocUUIDs, "no documents should be deleted")
	})

	t.Run("orphaned docs in index are deleted", func(t *testing.T) {
		t.Parallel()

		indexer := &mockSearchIndexer{
			indexedUUIDs: map[string]bool{"u1": true, "u2": true, "orphan1": true},
		}
		docRepo := &mockDocumentRepo{
			allUUIDs: []string{"u1", "u2"},
		}

		worker := &VerifySearchIndexWorker{
			Deps: SchedulerDeps{
				DocRepo: docRepo,
				Indexer: indexer,
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(VerifySearchIndexArgs{}))
		require.NoError(t, err)
		assert.Contains(t, indexer.deletedDocUUIDs, "orphan1")
	})

	t.Run("missing docs from index logged but no error", func(t *testing.T) {
		t.Parallel()

		indexer := &mockSearchIndexer{
			indexedUUIDs: map[string]bool{"u1": true},
		}
		docRepo := &mockDocumentRepo{
			allUUIDs: []string{"u1", "u2", "u3"},
		}

		worker := &VerifySearchIndexWorker{
			Deps: SchedulerDeps{
				DocRepo: docRepo,
				Indexer: indexer,
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(VerifySearchIndexArgs{}))
		require.NoError(t, err)
		// u2, u3 are missing from the index but no error is returned.
		assert.Empty(t, indexer.deletedDocUUIDs)
	})

	t.Run("ListAllUUIDs error logged not returned", func(t *testing.T) {
		t.Parallel()

		docRepo := &mockDocumentRepo{
			allUUIDsErr: errors.New("db error"),
		}

		worker := &VerifySearchIndexWorker{
			Deps: SchedulerDeps{
				DocRepo: docRepo,
				Indexer: &mockSearchIndexer{},
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(VerifySearchIndexArgs{}))
		require.NoError(t, err, "document errors are logged, not propagated")
	})

	t.Run("ListIndexedDocumentUUIDs error logged not returned", func(t *testing.T) {
		t.Parallel()

		docRepo := &mockDocumentRepo{
			allUUIDs: []string{"u1"},
		}
		indexer := &mockSearchIndexer{
			listUUIDsErr: errors.New("search unavailable"),
		}

		worker := &VerifySearchIndexWorker{
			Deps: SchedulerDeps{
				DocRepo: docRepo,
				Indexer: indexer,
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(VerifySearchIndexArgs{}))
		require.NoError(t, err, "index listing errors are logged, not propagated")
	})

	t.Run("DeleteDocument error logged but does not fail job", func(t *testing.T) {
		t.Parallel()

		indexer := &mockSearchIndexer{
			indexedUUIDs: map[string]bool{"u1": true, "orphan": true},
			deleteDocErr: errors.New("delete failed"),
		}
		docRepo := &mockDocumentRepo{
			allUUIDs: []string{"u1"},
		}

		worker := &VerifySearchIndexWorker{
			Deps: SchedulerDeps{
				DocRepo: docRepo,
				Indexer: indexer,
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(VerifySearchIndexArgs{}))
		require.NoError(t, err)
		assert.Contains(t, indexer.deletedDocUUIDs, "orphan")
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

	t.Run("success files removed from disk and index", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		filePath := filepath.Join(dir, "purged.pdf")
		require.NoError(t, os.WriteFile(filePath, []byte("old"), 0o600))

		indexer := &mockSearchIndexer{}
		docRepo := &mockDocumentRepo{
			purgedDocs: []repository.DocumentFilePath{
				{ID: 1, UUID: "purge-uuid", FilePath: "purged.pdf"},
			},
		}

		worker := &PurgeSoftDeletedWorker{
			Deps: SchedulerDeps{
				DocRepo:     docRepo,
				Indexer:     indexer,
				StoragePath: dir,
				Logger:      testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(PurgeSoftDeletedArgs{}))
		require.NoError(t, err)

		// File removed from disk.
		_, statErr := os.Stat(filePath)
		assert.True(t, os.IsNotExist(statErr), "purged file should be removed")

		// Document removed from search index.
		assert.Contains(t, indexer.deletedDocUUIDs, "purge-uuid")
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
				DocRepo:     docRepo,
				Indexer:     &mockSearchIndexer{},
				StoragePath: dir,
				Logger:      testLogger(),
			},
		}

		// Should not error -- os.IsNotExist errors are silently handled.
		err := worker.Work(context.Background(), makeJob(PurgeSoftDeletedArgs{}))
		require.NoError(t, err)
	})

	t.Run("nil Indexer skips index deletion", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		docRepo := &mockDocumentRepo{
			purgedDocs: []repository.DocumentFilePath{
				{ID: 1, UUID: "no-idx-uuid", FilePath: ""},
			},
		}

		worker := &PurgeSoftDeletedWorker{
			Deps: SchedulerDeps{
				DocRepo:     docRepo,
				Indexer:     nil,
				StoragePath: dir,
				Logger:      testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(PurgeSoftDeletedArgs{}))
		require.NoError(t, err)
	})

	t.Run("DeleteDocument index error logged but does not fail", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		indexer := &mockSearchIndexer{
			deleteDocErr: errors.New("index delete failed"),
		}
		docRepo := &mockDocumentRepo{
			purgedDocs: []repository.DocumentFilePath{
				{ID: 1, UUID: "idx-err-uuid", FilePath: ""},
			},
		}

		worker := &PurgeSoftDeletedWorker{
			Deps: SchedulerDeps{
				DocRepo:     docRepo,
				Indexer:     indexer,
				StoragePath: dir,
				Logger:      testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(PurgeSoftDeletedArgs{}))
		require.NoError(t, err)
		assert.Contains(t, indexer.deletedDocUUIDs, "idx-err-uuid")
	})
}

// ---------------------------------------------------------------------------
// CleanupDisabledZimWorker
// ---------------------------------------------------------------------------

func TestCleanupDisabledZimWorker_Work(t *testing.T) {
	t.Parallel()

	t.Run("nil ZimRepo skips gracefully", func(t *testing.T) {
		t.Parallel()

		worker := &CleanupDisabledZimWorker{
			Deps: SchedulerDeps{
				ZimRepo: nil,
				Indexer: nil,
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(CleanupDisabledZimArgs{}))
		require.NoError(t, err)
	})

	t.Run("nil Indexer skips gracefully", func(t *testing.T) {
		t.Parallel()

		worker := &CleanupDisabledZimWorker{
			Deps: SchedulerDeps{
				ZimRepo: &mockZimArchiveRepo{},
				Indexer: nil,
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(CleanupDisabledZimArgs{}))
		require.NoError(t, err)
	})

	t.Run("success disabled archives removed from index", func(t *testing.T) {
		t.Parallel()

		indexer := &mockSearchIndexer{}
		zimRepo := &mockZimArchiveRepo{
			disabledArchives: []model.ZimArchive{
				{UUID: "zim-1"},
				{UUID: "zim-2"},
			},
		}

		worker := &CleanupDisabledZimWorker{
			Deps: SchedulerDeps{
				ZimRepo: zimRepo,
				Indexer: indexer,
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(CleanupDisabledZimArgs{}))
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"zim-1", "zim-2"}, indexer.deletedZimUUIDs)
	})

	t.Run("FindDisabled error returned", func(t *testing.T) {
		t.Parallel()

		zimRepo := &mockZimArchiveRepo{
			findDisabledErr: errors.New("db error"),
		}

		worker := &CleanupDisabledZimWorker{
			Deps: SchedulerDeps{
				ZimRepo: zimRepo,
				Indexer: &mockSearchIndexer{},
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(CleanupDisabledZimArgs{}))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "finding disabled ZIM archives")
	})

	t.Run("DeleteZimArchive error logged continues to next", func(t *testing.T) {
		t.Parallel()

		indexer := &mockSearchIndexer{
			deleteZimErr: errors.New("delete failed"),
		}
		zimRepo := &mockZimArchiveRepo{
			disabledArchives: []model.ZimArchive{
				{UUID: "fail-zim"},
				{UUID: "ok-zim"},
			},
		}

		worker := &CleanupDisabledZimWorker{
			Deps: SchedulerDeps{
				ZimRepo: zimRepo,
				Indexer: indexer,
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(CleanupDisabledZimArgs{}))
		require.NoError(t, err)
		// Both were attempted despite the error.
		assert.Len(t, indexer.deletedZimUUIDs, 2)
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

		var updatedStatus string
		checker := &mockExternalServiceHealthChecker{
			findAllEnabledFn: func(_ context.Context) ([]model.ExternalService, error) {
				return []model.ExternalService{
					{ID: 1, BaseURL: "://invalid-url"},
				}, nil
			},
			updateHealthFn: func(_ context.Context, _ int64, status string, _ int, _ string) error {
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
		assert.Equal(t, "unhealthy", updatedStatus)
	})

	t.Run("loopback service blocked by SSRF marked unhealthy", func(t *testing.T) {
		t.Parallel()

		// The worker uses security.SafeTransport which blocks loopback addresses.
		// Services on 127.0.0.1 (e.g. httptest servers) are correctly marked unhealthy.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		var updatedStatus string
		checker := &mockExternalServiceHealthChecker{
			findAllEnabledFn: func(_ context.Context) ([]model.ExternalService, error) {
				return []model.ExternalService{
					{ID: 1, BaseURL: srv.URL},
				}, nil
			},
			updateHealthFn: func(_ context.Context, _ int64, status string, _ int, _ string) error {
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
		assert.Equal(t, "unhealthy", updatedStatus)
	})

	t.Run("unreachable service marked unhealthy", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		// Close immediately so connections fail.
		srv.Close()

		var updatedStatus string
		checker := &mockExternalServiceHealthChecker{
			findAllEnabledFn: func(_ context.Context) ([]model.ExternalService, error) {
				return []model.ExternalService{
					{ID: 1, BaseURL: srv.URL},
				}, nil
			},
			updateHealthFn: func(_ context.Context, _ int64, status string, _ int, _ string) error {
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
		assert.Equal(t, "unhealthy", updatedStatus)
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

		statuses := make(map[int64]string)
		checker := &mockExternalServiceHealthChecker{
			findAllEnabledFn: func(_ context.Context) ([]model.ExternalService, error) {
				return []model.ExternalService{
					{ID: 1, BaseURL: srv1.URL},
					{ID: 2, BaseURL: srv2.URL},
				}, nil
			},
			updateHealthFn: func(_ context.Context, id int64, status string, _ int, _ string) error {
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
		assert.Equal(t, "unhealthy", statuses[1])
		assert.Equal(t, "unhealthy", statuses[2])
	})
}

// ---------------------------------------------------------------------------
// VerifySearchIndexWorker — ZIM and Git template index verification
// ---------------------------------------------------------------------------

func TestVerifySearchIndexWorker_Work_ZimRepoVerification(t *testing.T) {
	t.Parallel()

	t.Run("orphaned zim archives in index are deleted", func(t *testing.T) {
		t.Parallel()

		zimRepo := &mockZimArchiveRepo{}
		// Override ListAllUUIDs to return specific UUIDs.
		zimRepoWithUUIDs := &mockZimArchiveRepoWithUUIDs{
			mockZimArchiveRepo: zimRepo,
			allUUIDs:           []string{"zim-1", "zim-2"},
		}

		indexer := &mockSearchIndexerFull{
			docUUIDs: map[string]bool{"u1": true},
			zimUUIDs: map[string]bool{"zim-1": true, "zim-2": true, "zim-orphan": true},
			gitUUIDs: map[string]bool{},
		}
		docRepo := &mockDocumentRepo{
			allUUIDs: []string{"u1"},
		}

		worker := &VerifySearchIndexWorker{
			Deps: SchedulerDeps{
				DocRepo: docRepo,
				Indexer: indexer,
				ZimRepo: zimRepoWithUUIDs,
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(VerifySearchIndexArgs{}))
		require.NoError(t, err)
		assert.Contains(t, indexer.deletedZimUUIDs, "zim-orphan")
		assert.Len(t, indexer.deletedZimUUIDs, 1)
	})

	t.Run("zim ListAllUUIDs error is logged not returned", func(t *testing.T) {
		t.Parallel()

		zimRepo := &mockZimArchiveRepoWithUUIDs{
			mockZimArchiveRepo: &mockZimArchiveRepo{},
			allUUIDsErr:        errors.New("zim db error"),
		}
		indexer := &mockSearchIndexerFull{
			docUUIDs: map[string]bool{},
		}
		docRepo := &mockDocumentRepo{
			allUUIDs: []string{},
		}

		worker := &VerifySearchIndexWorker{
			Deps: SchedulerDeps{
				DocRepo: docRepo,
				Indexer: indexer,
				ZimRepo: zimRepo,
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(VerifySearchIndexArgs{}))
		require.NoError(t, err)
	})

	t.Run("zim ListIndexedZimUUIDs error is logged not returned", func(t *testing.T) {
		t.Parallel()

		zimRepo := &mockZimArchiveRepoWithUUIDs{
			mockZimArchiveRepo: &mockZimArchiveRepo{},
			allUUIDs:           []string{"zim-1"},
		}
		indexer := &mockSearchIndexerFull{
			docUUIDs:       map[string]bool{},
			listZimUUIDErr: errors.New("search down"),
		}
		docRepo := &mockDocumentRepo{
			allUUIDs: []string{},
		}

		worker := &VerifySearchIndexWorker{
			Deps: SchedulerDeps{
				DocRepo: docRepo,
				Indexer: indexer,
				ZimRepo: zimRepo,
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(VerifySearchIndexArgs{}))
		require.NoError(t, err)
	})
}

func TestVerifySearchIndexWorker_Work_GitRepoVerification(t *testing.T) {
	t.Parallel()

	t.Run("orphaned git templates in index are deleted", func(t *testing.T) {
		t.Parallel()

		gitRepo := &mockGitTemplateRepoWithUUIDs{
			mockGitTemplateRepo: &mockGitTemplateRepo{},
			allUUIDs:            []string{"git-1"},
		}

		indexer := &mockSearchIndexerFull{
			docUUIDs: map[string]bool{},
			gitUUIDs: map[string]bool{"git-1": true, "git-orphan": true},
		}
		docRepo := &mockDocumentRepo{
			allUUIDs: []string{},
		}

		worker := &VerifySearchIndexWorker{
			Deps: SchedulerDeps{
				DocRepo: docRepo,
				Indexer: indexer,
				GitRepo: gitRepo,
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(VerifySearchIndexArgs{}))
		require.NoError(t, err)
		assert.Contains(t, indexer.deletedGitUUIDs, "git-orphan")
		assert.Len(t, indexer.deletedGitUUIDs, 1)
	})

	t.Run("git ListAllUUIDs error is logged not returned", func(t *testing.T) {
		t.Parallel()

		gitRepo := &mockGitTemplateRepoWithUUIDs{
			mockGitTemplateRepo: &mockGitTemplateRepo{},
			allUUIDsErr:         errors.New("git db error"),
		}
		indexer := &mockSearchIndexerFull{
			docUUIDs: map[string]bool{},
		}
		docRepo := &mockDocumentRepo{
			allUUIDs: []string{},
		}

		worker := &VerifySearchIndexWorker{
			Deps: SchedulerDeps{
				DocRepo: docRepo,
				Indexer: indexer,
				GitRepo: gitRepo,
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(VerifySearchIndexArgs{}))
		require.NoError(t, err)
	})

	t.Run("git ListIndexedGitTemplateUUIDs error is logged not returned", func(t *testing.T) {
		t.Parallel()

		gitRepo := &mockGitTemplateRepoWithUUIDs{
			mockGitTemplateRepo: &mockGitTemplateRepo{},
			allUUIDs:            []string{"git-1"},
		}
		indexer := &mockSearchIndexerFull{
			docUUIDs:       map[string]bool{},
			listGitUUIDErr: errors.New("search down"),
		}
		docRepo := &mockDocumentRepo{
			allUUIDs: []string{},
		}

		worker := &VerifySearchIndexWorker{
			Deps: SchedulerDeps{
				DocRepo: docRepo,
				Indexer: indexer,
				GitRepo: gitRepo,
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(VerifySearchIndexArgs{}))
		require.NoError(t, err)
	})

	t.Run("both zim and git repos verified together", func(t *testing.T) {
		t.Parallel()

		zimRepo := &mockZimArchiveRepoWithUUIDs{
			mockZimArchiveRepo: &mockZimArchiveRepo{},
			allUUIDs:           []string{"zim-1"},
		}
		gitRepo := &mockGitTemplateRepoWithUUIDs{
			mockGitTemplateRepo: &mockGitTemplateRepo{},
			allUUIDs:            []string{"git-1"},
		}
		indexer := &mockSearchIndexerFull{
			docUUIDs: map[string]bool{"u1": true},
			zimUUIDs: map[string]bool{"zim-1": true, "zim-orphan": true},
			gitUUIDs: map[string]bool{"git-1": true, "git-orphan": true},
		}
		docRepo := &mockDocumentRepo{
			allUUIDs: []string{"u1"},
		}

		worker := &VerifySearchIndexWorker{
			Deps: SchedulerDeps{
				DocRepo: docRepo,
				Indexer: indexer,
				ZimRepo: zimRepo,
				GitRepo: gitRepo,
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(VerifySearchIndexArgs{}))
		require.NoError(t, err)
		assert.Contains(t, indexer.deletedZimUUIDs, "zim-orphan")
		assert.Contains(t, indexer.deletedGitUUIDs, "git-orphan")
	})

	t.Run("delete git template error logged but does not fail job", func(t *testing.T) {
		t.Parallel()

		gitRepo := &mockGitTemplateRepoWithUUIDs{
			mockGitTemplateRepo: &mockGitTemplateRepo{},
			allUUIDs:            []string{"git-1"},
		}
		indexer := &mockSearchIndexerFull{
			docUUIDs:       map[string]bool{},
			gitUUIDs:       map[string]bool{"git-orphan": true},
			deleteGitErr:   errors.New("index delete failed"),
		}
		docRepo := &mockDocumentRepo{
			allUUIDs: []string{},
		}

		worker := &VerifySearchIndexWorker{
			Deps: SchedulerDeps{
				DocRepo: docRepo,
				Indexer: indexer,
				GitRepo: gitRepo,
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(VerifySearchIndexArgs{}))
		require.NoError(t, err)
		assert.Contains(t, indexer.deletedGitUUIDs, "git-orphan")
	})

	t.Run("delete zim archive error logged but does not fail job", func(t *testing.T) {
		t.Parallel()

		zimRepo := &mockZimArchiveRepoWithUUIDs{
			mockZimArchiveRepo: &mockZimArchiveRepo{},
			allUUIDs:           []string{"zim-1"},
		}
		indexer := &mockSearchIndexerFull{
			docUUIDs:     map[string]bool{},
			zimUUIDs:     map[string]bool{"zim-orphan": true},
			deleteZimErr: errors.New("index delete failed"),
		}
		docRepo := &mockDocumentRepo{
			allUUIDs: []string{},
		}

		worker := &VerifySearchIndexWorker{
			Deps: SchedulerDeps{
				DocRepo: docRepo,
				Indexer: indexer,
				ZimRepo: zimRepo,
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(VerifySearchIndexArgs{}))
		require.NoError(t, err)
		assert.Contains(t, indexer.deletedZimUUIDs, "zim-orphan")
	})
}

// ---------------------------------------------------------------------------
// VerifySearchIndexWorker — Re-indexing missing entries
// ---------------------------------------------------------------------------

func TestVerifySearchIndexWorker_ReindexMissing(t *testing.T) {
	t.Parallel()

	t.Run("missing documents are re-indexed", func(t *testing.T) {
		t.Parallel()

		docRepo := &mockDocumentRepo{
			allUUIDs: []string{"doc-1", "doc-2"},
			foundDocs: []model.Document{
				{UUID: "doc-1", Title: "First", FileType: "pdf", Status: "processed", IsPublic: true},
				{UUID: "doc-2", Title: "Second", FileType: "markdown", Status: "processed"},
			},
		}
		indexer := &mockSearchIndexer{
			indexedUUIDs: map[string]bool{}, // both missing from index
		}

		worker := &VerifySearchIndexWorker{
			Deps: SchedulerDeps{
				DocRepo: docRepo,
				Indexer: indexer,
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(VerifySearchIndexArgs{}))
		require.NoError(t, err)
		assert.Len(t, indexer.indexedDocRecords, 2)
		assert.Equal(t, "doc-1", indexer.indexedDocRecords[0].UUID)
		assert.Equal(t, "doc-2", indexer.indexedDocRecords[1].UUID)
	})

	t.Run("FindByUUIDs error logged but does not fail job", func(t *testing.T) {
		t.Parallel()

		docRepo := &mockDocumentRepo{
			allUUIDs:       []string{"doc-missing"},
			findByUUIDsErr: errors.New("db connection lost"),
		}
		indexer := &mockSearchIndexer{
			indexedUUIDs: map[string]bool{},
		}

		worker := &VerifySearchIndexWorker{
			Deps: SchedulerDeps{
				DocRepo: docRepo,
				Indexer: indexer,
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(VerifySearchIndexArgs{}))
		require.NoError(t, err)
		assert.Empty(t, indexer.indexedDocRecords)
	})

	t.Run("mixed: re-index missing and delete orphaned", func(t *testing.T) {
		t.Parallel()

		docRepo := &mockDocumentRepo{
			allUUIDs: []string{"in-both", "missing-from-index"},
			foundDocs: []model.Document{
				{UUID: "missing-from-index", Title: "Needs Reindex", FileType: "pdf", Status: "processed"},
			},
		}
		indexer := &mockSearchIndexer{
			indexedUUIDs: map[string]bool{
				"in-both":  true,
				"orphaned": true, // not in DB
			},
		}

		worker := &VerifySearchIndexWorker{
			Deps: SchedulerDeps{
				DocRepo: docRepo,
				Indexer: indexer,
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(VerifySearchIndexArgs{}))
		require.NoError(t, err)
		// Orphaned entry deleted.
		assert.Contains(t, indexer.deletedDocUUIDs, "orphaned")
		// Missing entry re-indexed.
		assert.Len(t, indexer.indexedDocRecords, 1)
		assert.Equal(t, "missing-from-index", indexer.indexedDocRecords[0].UUID)
	})

	t.Run("no missing and no orphaned does nothing", func(t *testing.T) {
		t.Parallel()

		docRepo := &mockDocumentRepo{
			allUUIDs: []string{"u1"},
		}
		indexer := &mockSearchIndexer{
			indexedUUIDs: map[string]bool{"u1": true},
		}

		worker := &VerifySearchIndexWorker{
			Deps: SchedulerDeps{
				DocRepo: docRepo,
				Indexer: indexer,
				Logger:  testLogger(),
			},
		}

		err := worker.Work(context.Background(), makeJob(VerifySearchIndexArgs{}))
		require.NoError(t, err)
		assert.Empty(t, indexer.deletedDocUUIDs)
		assert.Empty(t, indexer.indexedDocRecords)
	})
}

// ---------------------------------------------------------------------------
// SyncKiwixWorker
// ---------------------------------------------------------------------------

func TestSyncKiwixWorker_Work(t *testing.T) {
	t.Parallel()

	t.Run("no enabled services returns nil", func(t *testing.T) {
		t.Parallel()

		finder := &mockExternalServiceFinder{
			findEnabledByTypeFn: func(_ context.Context, _ string) ([]model.ExternalService, error) {
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
				return nil, assert.AnError
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
	})

	t.Run("service with invalid URL is skipped", func(t *testing.T) {
		t.Parallel()

		finder := &mockExternalServiceFinder{
			findEnabledByTypeFn: func(_ context.Context, _ string) ([]model.ExternalService, error) {
				return []model.ExternalService{
					{ID: 1, BaseURL: "://bad-url"},
				}, nil
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

	t.Run("service with invalid URL and health checker logs error", func(t *testing.T) {
		t.Parallel()

		// Invalid URL causes kiwix.NewClient to reject it. The worker skips the
		// service with a log message but does not call HealthChecker (client creation
		// failure is not a health check failure).
		finder := &mockExternalServiceFinder{
			findEnabledByTypeFn: func(_ context.Context, _ string) ([]model.ExternalService, error) {
				return []model.ExternalService{
					{ID: 1, BaseURL: "://bad-url"},
					{ID: 2, BaseURL: "ftp://not-http"},
				}, nil
			},
		}

		checker := &mockExternalServiceHealthChecker{}

		worker := &SyncKiwixWorker{
			Deps: SchedulerDeps{
				Services:      finder,
				HealthChecker: checker,
				Logger:        testLogger(),
			},
		}

		// Both services have invalid URLs; worker succeeds (skips them).
		err := worker.Work(context.Background(), makeJob(SyncKiwixArgs{}))
		require.NoError(t, err)
	})

	t.Run("multiple services with mixed valid and invalid URLs", func(t *testing.T) {
		t.Parallel()

		finder := &mockExternalServiceFinder{
			findEnabledByTypeFn: func(_ context.Context, _ string) ([]model.ExternalService, error) {
				return []model.ExternalService{
					{ID: 1, BaseURL: "://invalid"},
					{ID: 2, BaseURL: "http://"},
				}, nil
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
}

// ---------------------------------------------------------------------------
// SyncGitTemplatesWorker
// ---------------------------------------------------------------------------

func TestSyncGitTemplatesWorker_Work(t *testing.T) {
	t.Parallel()

	t.Run("no templates returns nil", func(t *testing.T) {
		t.Parallel()

		gitRepo := &mockGitTemplateRepo{
			templates: []model.GitTemplate{},
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

	t.Run("list error returns wrapped error", func(t *testing.T) {
		t.Parallel()

		gitRepo := &mockGitTemplateRepo{
			listErr: errors.New("db timeout"),
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
		assert.Contains(t, err.Error(), "db timeout")
	})

	t.Run("valid template with unreachable repo fails sync gracefully", func(t *testing.T) {
		t.Parallel()

		gitRepo := &mockGitTemplateRepo{
			templates: []model.GitTemplate{
				{
					ID:            1,
					UUID:          "uuid-valid",
					Name:          "valid-template",
					Slug:          "valid-template",
					RepositoryURL: "https://nonexistent.invalid/repo.git",
					Branch:        "main",
				},
			},
		}

		worker := &SyncGitTemplatesWorker{
			Deps: SchedulerDeps{
				GitRepo:    gitRepo,
				GitTempDir: t.TempDir(),
				Logger:     testLogger(),
			},
		}

		// git.Sync will fail because the repo URL is unreachable.
		// The worker should log the error and return nil.
		err := worker.Work(context.Background(), makeJob(SyncGitTemplatesArgs{}))
		require.NoError(t, err)
	})

	t.Run("valid template with indexer set syncs with indexer adapter", func(t *testing.T) {
		t.Parallel()

		gitRepo := &mockGitTemplateRepo{
			templates: []model.GitTemplate{
				{
					ID:            2,
					UUID:          "uuid-idx",
					Name:          "idx-template",
					Slug:          "idx-template",
					RepositoryURL: "https://nonexistent.invalid/repo2.git",
					Branch:        "main",
					Tags:          sql.NullString{String: `["go"]`, Valid: true},
				},
			},
		}

		indexer := &mockSearchIndexer{
			indexedUUIDs: map[string]bool{},
		}

		worker := &SyncGitTemplatesWorker{
			Deps: SchedulerDeps{
				GitRepo:    gitRepo,
				GitTempDir: t.TempDir(),
				Indexer:    indexer,
				Logger:     testLogger(),
			},
		}

		// Will fail at git clone but exercises the indexer adapter creation path.
		err := worker.Work(context.Background(), makeJob(SyncGitTemplatesArgs{}))
		require.NoError(t, err)
	})

	t.Run("template with invalid tags is skipped", func(t *testing.T) {
		t.Parallel()

		gitRepo := &mockGitTemplateRepo{
			templates: []model.GitTemplate{
				{
					ID:            1,
					UUID:          "uuid-bad-tags",
					Name:          "bad-tags",
					Slug:          "bad-tags",
					RepositoryURL: "https://example.com/repo",
					Branch:        "main",
					Tags:          sql.NullString{String: "not-valid-json", Valid: true},
				},
			},
		}

		worker := &SyncGitTemplatesWorker{
			Deps: SchedulerDeps{
				GitRepo:    gitRepo,
				GitTempDir: t.TempDir(),
				Logger:     testLogger(),
			},
		}

		// Template with invalid tags is skipped, no error returned.
		err := worker.Work(context.Background(), makeJob(SyncGitTemplatesArgs{}))
		require.NoError(t, err)
	})
}
