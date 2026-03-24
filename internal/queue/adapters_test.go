package queue

import (
	"context"
	"testing"

	"github.com/riverqueue/river"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gitclient "git.999.haus/chris/DocuMCP-go/internal/client/git"
	"git.999.haus/chris/DocuMCP-go/internal/client/kiwix"
	"git.999.haus/chris/DocuMCP-go/internal/model"
	"git.999.haus/chris/DocuMCP-go/internal/repository"
	"git.999.haus/chris/DocuMCP-go/internal/search"
)

// ---------------------------------------------------------------------------
// Adapter Kind() and InsertOpts() tests
// ---------------------------------------------------------------------------

// These tests verify that each job args adapter returns the expected Kind
// string and InsertOpts configuration. This complements the comprehensive
// tests in jobs_test.go by focusing on the adapter contract.

func TestAdapters_KindValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args river.JobArgs
		want string
	}{
		{"SyncKiwix", SyncKiwixArgs{}, "sync_kiwix"},
		{"SyncGitTemplates", SyncGitTemplatesArgs{}, "sync_git_templates"},
		{"CleanupOAuthTokens", CleanupOAuthTokensArgs{}, "cleanup_oauth_tokens"},
		{"CleanupOrphanedFiles", CleanupOrphanedFilesArgs{}, "cleanup_orphaned_files"},
		{"VerifySearchIndex", VerifySearchIndexArgs{}, "verify_search_index"},
		{"PurgeSoftDeleted", PurgeSoftDeletedArgs{}, "purge_soft_deleted"},
		{"CleanupDisabledZim", CleanupDisabledZimArgs{}, "cleanup_disabled_zim"},
		{"HealthCheckServices", HealthCheckServicesArgs{}, "health_check_services"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.args.Kind())
		})
	}
}

func TestAdapters_InsertOpts_QueueAndMaxAttempts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       river.JobArgs
		wantQueue  string
		wantMaxAtt int
		wantPri    int
	}{
		{"DocumentExtract", DocumentExtractArgs{DocumentID: 1, DocUUID: "a"}, "high", 4, 1},
		{"DocumentIndex", DocumentIndexArgs{DocumentID: 2, DocUUID: "b"}, "default", 4, 2},
		{"ReindexAll", ReindexAllArgs{}, "low", 2, 4},
		{"SyncKiwix", SyncKiwixArgs{}, "low", 2, 4},
		{"SyncGitTemplates", SyncGitTemplatesArgs{}, "low", 2, 4},
		{"CleanupOAuthTokens", CleanupOAuthTokensArgs{}, "low", 2, 4},
		{"CleanupOrphanedFiles", CleanupOrphanedFilesArgs{}, "low", 2, 4},
		{"VerifySearchIndex", VerifySearchIndexArgs{}, "low", 2, 4},
		{"PurgeSoftDeleted", PurgeSoftDeletedArgs{}, "low", 2, 4},
		{"CleanupDisabledZim", CleanupDisabledZimArgs{}, "low", 2, 4},
		{"HealthCheckServices", HealthCheckServicesArgs{}, "low", 2, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			argsWithOpts, ok := tt.args.(river.JobArgsWithInsertOpts)
			if !ok {
				t.Fatal("args does not implement JobArgsWithInsertOpts")
			}

			opts := argsWithOpts.InsertOpts()
			assert.Equal(t, tt.wantQueue, opts.Queue, "Queue")
			assert.Equal(t, tt.wantMaxAtt, opts.MaxAttempts, "MaxAttempts")
			assert.Equal(t, tt.wantPri, opts.Priority, "Priority")
		})
	}
}

func TestAdapters_SchedulerJobs_HaveUniqueOpts(t *testing.T) {
	t.Parallel()

	schedulerArgs := []river.JobArgs{
		ReindexAllArgs{},
		SyncKiwixArgs{},
		SyncGitTemplatesArgs{},
		CleanupOAuthTokensArgs{},
		CleanupOrphanedFilesArgs{},
		VerifySearchIndexArgs{},
		PurgeSoftDeletedArgs{},
		CleanupDisabledZimArgs{},
		HealthCheckServicesArgs{},
	}

	for _, args := range schedulerArgs {
		t.Run(args.Kind(), func(t *testing.T) {
			t.Parallel()

			argsWithOpts := args.(river.JobArgsWithInsertOpts)
			opts := argsWithOpts.InsertOpts()

			assert.True(t, opts.UniqueOpts.ByQueue, "scheduler jobs should be unique by queue")
			assert.NotEmpty(t, opts.UniqueOpts.ByState, "scheduler jobs should have unique state constraints")
		})
	}
}

func TestAdapters_DocumentJobs_NoUniqueOpts(t *testing.T) {
	t.Parallel()

	documentArgs := []river.JobArgs{
		DocumentExtractArgs{DocumentID: 1, DocUUID: "a"},
		DocumentIndexArgs{DocumentID: 2, DocUUID: "b"},
	}

	for _, args := range documentArgs {
		t.Run(args.Kind(), func(t *testing.T) {
			t.Parallel()

			argsWithOpts := args.(river.JobArgsWithInsertOpts)
			opts := argsWithOpts.InsertOpts()

			assert.False(t, opts.UniqueOpts.ByQueue, "document jobs should not be unique by queue")
			assert.Nil(t, opts.UniqueOpts.ByState, "document jobs should not have unique state constraints")
		})
	}
}

func TestDocumentExtractArgs_FieldMapping(t *testing.T) {
	t.Parallel()

	args := DocumentExtractArgs{DocumentID: 42, DocUUID: "test-uuid-123"}
	assert.Equal(t, int64(42), args.DocumentID)
	assert.Equal(t, "test-uuid-123", args.DocUUID)
	assert.Equal(t, "document_extract", args.Kind())
}

func TestDocumentIndexArgs_FieldMapping(t *testing.T) {
	t.Parallel()

	args := DocumentIndexArgs{DocumentID: 99, DocUUID: "idx-uuid-456"}
	assert.Equal(t, int64(99), args.DocumentID)
	assert.Equal(t, "idx-uuid-456", args.DocUUID)
	assert.Equal(t, "document_index", args.Kind())
}

// ---------------------------------------------------------------------------
// Adapter data-flow tests
// ---------------------------------------------------------------------------

// stubZimRepo captures calls to UpsertFromCatalog and DisableOrphaned.
type stubZimRepo struct {
	upsertServiceID int64
	upsertEntry     repository.ZimArchiveUpsert
	disableNames    []string
}

func (s *stubZimRepo) FindDisabled(_ context.Context) ([]model.ZimArchive, error) { return nil, nil }

func (s *stubZimRepo) UpsertFromCatalog(_ context.Context, serviceID int64, entry repository.ZimArchiveUpsert) error {
	s.upsertServiceID = serviceID
	s.upsertEntry = entry
	return nil
}

func (s *stubZimRepo) DisableOrphaned(_ context.Context, _ int64, names []string) (int, error) {
	s.disableNames = names
	return len(names), nil
}

// stubSearchIndexer captures calls to IndexZimArchive and IndexGitTemplate.
type stubSearchIndexer struct {
	zimRecord search.ZimArchiveRecord
	gitRecord search.GitTemplateRecord
}

func (s *stubSearchIndexer) ListIndexedDocumentUUIDs(_ context.Context) (map[string]bool, error) {
	return nil, nil
}
func (s *stubSearchIndexer) DeleteDocument(_ context.Context, _ string) error  { return nil }
func (s *stubSearchIndexer) DeleteZimArchive(_ context.Context, _ string) error { return nil }

func (s *stubSearchIndexer) IndexZimArchive(_ context.Context, rec search.ZimArchiveRecord) error {
	s.zimRecord = rec
	return nil
}

func (s *stubSearchIndexer) IndexGitTemplate(_ context.Context, rec search.GitTemplateRecord) error {
	s.gitRecord = rec
	return nil
}

// stubGitRepo captures calls to UpdateSyncStatus and ReplaceFiles.
type stubGitRepo struct {
	syncStatusTemplateID int64
	syncStatus           string
	replacedFiles        []repository.GitTemplateFileInsert
}

func (s *stubGitRepo) List(_ context.Context, _ string, _, _ int) ([]model.GitTemplate, error) {
	return nil, nil
}

func (s *stubGitRepo) UpdateSyncStatus(_ context.Context, templateID int64, status, _ string, _ int, _ int64, _ string) error {
	s.syncStatusTemplateID = templateID
	s.syncStatus = status
	return nil
}

func (s *stubGitRepo) ReplaceFiles(_ context.Context, _ int64, files []repository.GitTemplateFileInsert) error {
	s.replacedFiles = files
	return nil
}

func TestKiwixRepoAdapter_UpsertFromCatalog(t *testing.T) {
	t.Parallel()

	stub := &stubZimRepo{}
	adapter := &kiwixRepoAdapter{repo: stub}

	entry := kiwix.CatalogEntry{
		Name:         "wiki_en",
		Title:        "Wikipedia English",
		Description:  "Offline Wikipedia",
		Language:     "en",
		Category:     "wikipedia",
		Creator:      "Wikimedia",
		Publisher:    "Kiwix",
		Favicon:      "https://example.com/favicon.ico",
		ArticleCount: 6000000,
		MediaCount:   100000,
		FileSize:     42000000000,
		Tags:         []string{"wiki", "en"},
	}

	err := adapter.UpsertFromCatalog(context.Background(), 7, entry)
	require.NoError(t, err)

	assert.Equal(t, int64(7), stub.upsertServiceID)
	assert.Equal(t, "wiki_en", stub.upsertEntry.Name)
	assert.Equal(t, "Wikipedia English", stub.upsertEntry.Title)
	assert.Equal(t, "Offline Wikipedia", stub.upsertEntry.Description)
	assert.Equal(t, "en", stub.upsertEntry.Language)
	assert.Equal(t, "wikipedia", stub.upsertEntry.Category)
	assert.Equal(t, []string{"wiki", "en"}, stub.upsertEntry.Tags)
	assert.Equal(t, int64(6000000), stub.upsertEntry.ArticleCount)
	assert.Equal(t, int64(42000000000), stub.upsertEntry.FileSize)
}

func TestKiwixRepoAdapter_DisableOrphaned(t *testing.T) {
	t.Parallel()

	stub := &stubZimRepo{}
	adapter := &kiwixRepoAdapter{repo: stub}

	count, err := adapter.DisableOrphaned(context.Background(), 3, []string{"a", "b"})
	require.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.Equal(t, []string{"a", "b"}, stub.disableNames)
}

func TestKiwixIndexerAdapter_IndexZimArchive(t *testing.T) {
	t.Parallel()

	stub := &stubSearchIndexer{}
	adapter := &kiwixIndexerAdapter{indexer: stub}

	record := kiwix.ZimArchiveRecord{
		UUID:         "zim-uuid-1",
		Name:         "devdocs",
		Title:        "DevDocs",
		Description:  "API docs",
		Language:     "en",
		Category:     "devdocs",
		Creator:      "freeCodeCamp",
		Tags:         []string{"dev", "docs"},
		ArticleCount: 5000,
	}

	err := adapter.IndexZimArchive(context.Background(), record)
	require.NoError(t, err)

	assert.Equal(t, "zim-uuid-1", stub.zimRecord.UUID)
	assert.Equal(t, "devdocs", stub.zimRecord.Name)
	assert.Equal(t, "DevDocs", stub.zimRecord.Title)
	assert.Equal(t, "API docs", stub.zimRecord.Description)
	assert.Equal(t, []string{"dev", "docs"}, stub.zimRecord.Tags)
	assert.Equal(t, int64(5000), stub.zimRecord.ArticleCount)
}

func TestGitRepoAdapter_UpdateSyncStatus(t *testing.T) {
	t.Parallel()

	stub := &stubGitRepo{}
	adapter := &gitRepoAdapter{repo: stub}

	err := adapter.UpdateSyncStatus(context.Background(), 42, "synced", "abc123", 10, 1024, "")
	require.NoError(t, err)

	assert.Equal(t, int64(42), stub.syncStatusTemplateID)
	assert.Equal(t, "synced", stub.syncStatus)
}

func TestGitRepoAdapter_ReplaceFiles(t *testing.T) {
	t.Parallel()

	stub := &stubGitRepo{}
	adapter := &gitRepoAdapter{repo: stub}

	files := []gitclient.TemplateFile{
		{
			Path:        "templates/main.go",
			Filename:    "main.go",
			Extension:   ".go",
			Content:     "package main",
			ContentHash: "sha256abc",
			SizeBytes:   12,
			IsEssential: true,
			Variables:   []string{"{{project_name}}"},
		},
	}

	err := adapter.ReplaceFiles(context.Background(), 99, files)
	require.NoError(t, err)

	require.Len(t, stub.replacedFiles, 1)
	assert.Equal(t, "templates/main.go", stub.replacedFiles[0].Path)
	assert.Equal(t, "main.go", stub.replacedFiles[0].Filename)
	assert.Equal(t, ".go", stub.replacedFiles[0].Extension)
	assert.Equal(t, "package main", stub.replacedFiles[0].Content)
	assert.Equal(t, "sha256abc", stub.replacedFiles[0].ContentHash)
	assert.Equal(t, int64(12), stub.replacedFiles[0].SizeBytes)
	assert.True(t, stub.replacedFiles[0].IsEssential)
	assert.Equal(t, []string{"{{project_name}}"}, stub.replacedFiles[0].Variables)
}

func TestGitIndexerAdapter_IndexGitTemplate(t *testing.T) {
	t.Parallel()

	stub := &stubSearchIndexer{}
	adapter := &gitIndexerAdapter{indexer: stub}

	record := gitclient.GitTemplateRecord{
		UUID:          "git-uuid-1",
		Name:          "go-api",
		Slug:          "go-api",
		Description:   "Go API template",
		ReadmeContent: "# Go API",
		Category:      "backend",
		Tags:          []string{"go", "api"},
		IsPublic:      true,
		Status:        "synced",
		SoftDeleted:   false,
	}

	err := adapter.IndexGitTemplate(context.Background(), record)
	require.NoError(t, err)

	assert.Equal(t, "git-uuid-1", stub.gitRecord.UUID)
	assert.Equal(t, "go-api", stub.gitRecord.Name)
	assert.Equal(t, "go-api", stub.gitRecord.Slug)
	assert.Equal(t, "Go API template", stub.gitRecord.Description)
	assert.Equal(t, "# Go API", stub.gitRecord.ReadmeContent)
	assert.Equal(t, "backend", stub.gitRecord.Category)
	assert.Equal(t, []string{"go", "api"}, stub.gitRecord.Tags)
	assert.True(t, stub.gitRecord.IsPublic)
	assert.Equal(t, "synced", stub.gitRecord.Status)
	assert.False(t, stub.gitRecord.SoftDeleted)
}
