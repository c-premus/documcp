package queue

import (
	"testing"

	"github.com/riverqueue/river"
	"github.com/stretchr/testify/assert"
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
		{"SyncConfluence", SyncConfluenceArgs{}, "sync_confluence"},
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
		name        string
		args        river.JobArgs
		wantQueue   string
		wantMaxAtt  int
		wantPri     int
	}{
		{"DocumentExtract", DocumentExtractArgs{DocumentID: 1, DocUUID: "a"}, "high", 4, 1},
		{"DocumentIndex", DocumentIndexArgs{DocumentID: 2, DocUUID: "b"}, "default", 4, 2},
		{"ReindexAll", ReindexAllArgs{}, "low", 2, 4},
		{"SyncKiwix", SyncKiwixArgs{}, "low", 2, 4},
		{"SyncConfluence", SyncConfluenceArgs{}, "low", 2, 4},
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
		SyncConfluenceArgs{},
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
