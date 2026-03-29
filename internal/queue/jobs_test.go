package queue

import (
	"testing"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/assert"
)

func TestJobArgs_Kind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args river.JobArgs
		want string
	}{
		{"DocumentExtractArgs", DocumentExtractArgs{}, "document_extract"},
		{"SyncKiwixArgs", SyncKiwixArgs{}, "sync_kiwix"},
		{"SyncGitTemplatesArgs", SyncGitTemplatesArgs{}, "sync_git_templates"},
		{"CleanupOAuthTokensArgs", CleanupOAuthTokensArgs{}, "cleanup_oauth_tokens"},
		{"CleanupOrphanedFilesArgs", CleanupOrphanedFilesArgs{}, "cleanup_orphaned_files"},
		{"PurgeSoftDeletedArgs", PurgeSoftDeletedArgs{}, "purge_soft_deleted"},
		{"HealthCheckServicesArgs", HealthCheckServicesArgs{}, "health_check_services"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.args.Kind())
		})
	}
}

func TestJobArgs_InsertOpts(t *testing.T) {
	t.Parallel()

	// expectedUniqueStates is the common set of states used by all scheduler/singleton jobs.
	expectedUniqueStates := []rivertype.JobState{
		rivertype.JobStatePending,
		rivertype.JobStateAvailable,
		rivertype.JobStateRunning,
		rivertype.JobStateRetryable,
		rivertype.JobStateScheduled,
	}

	tests := []struct {
		name        string
		args        river.JobArgs
		wantQueue   string
		wantPri     int
		wantMaxAtt  int
		wantUnique  bool
		wantByQueue bool
		wantStates  []rivertype.JobState
	}{
		{
			name:       "DocumentExtractArgs_high_priority",
			args:       DocumentExtractArgs{DocumentID: 1, DocUUID: "abc"},
			wantQueue:  "high",
			wantPri:    1,
			wantMaxAtt: 4,
			wantUnique: false,
		},
		{
			name:        "SyncKiwixArgs_singleton",
			args:        SyncKiwixArgs{},
			wantQueue:   "low",
			wantPri:     4,
			wantMaxAtt:  2,
			wantUnique:  true,
			wantByQueue: true,
			wantStates:  expectedUniqueStates,
		},
		{
			name:        "SyncGitTemplatesArgs_singleton",
			args:        SyncGitTemplatesArgs{},
			wantQueue:   "low",
			wantPri:     4,
			wantMaxAtt:  2,
			wantUnique:  true,
			wantByQueue: true,
			wantStates:  expectedUniqueStates,
		},
		{
			name:        "CleanupOAuthTokensArgs_singleton",
			args:        CleanupOAuthTokensArgs{},
			wantQueue:   "low",
			wantPri:     4,
			wantMaxAtt:  2,
			wantUnique:  true,
			wantByQueue: true,
			wantStates:  expectedUniqueStates,
		},
		{
			name:        "CleanupOrphanedFilesArgs_singleton",
			args:        CleanupOrphanedFilesArgs{},
			wantQueue:   "low",
			wantPri:     4,
			wantMaxAtt:  2,
			wantUnique:  true,
			wantByQueue: true,
			wantStates:  expectedUniqueStates,
		},
		{
			name:        "PurgeSoftDeletedArgs_singleton",
			args:        PurgeSoftDeletedArgs{},
			wantQueue:   "low",
			wantPri:     4,
			wantMaxAtt:  2,
			wantUnique:  true,
			wantByQueue: true,
			wantStates:  expectedUniqueStates,
		},
		{
			name:        "HealthCheckServicesArgs_singleton",
			args:        HealthCheckServicesArgs{},
			wantQueue:   "low",
			wantPri:     4,
			wantMaxAtt:  2,
			wantUnique:  true,
			wantByQueue: true,
			wantStates:  expectedUniqueStates,
		},
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
			assert.Equal(t, tt.wantPri, opts.Priority, "Priority")
			assert.Equal(t, tt.wantMaxAtt, opts.MaxAttempts, "MaxAttempts")

			if tt.wantUnique {
				assert.True(t, opts.UniqueOpts.ByQueue, "UniqueOpts.ByQueue")
				assert.Equal(t, tt.wantStates, opts.UniqueOpts.ByState, "UniqueOpts.ByState")
			} else {
				assert.False(t, opts.UniqueOpts.ByQueue, "UniqueOpts.ByQueue should be false for non-singleton jobs")
				assert.Nil(t, opts.UniqueOpts.ByState, "UniqueOpts.ByState should be nil for non-singleton jobs")
			}
		})
	}
}

func TestDocumentExtractArgs_fieldsPreserved(t *testing.T) {
	t.Parallel()

	args := DocumentExtractArgs{DocumentID: 42, DocUUID: "test-uuid-123"}
	assert.Equal(t, int64(42), args.DocumentID)
	assert.Equal(t, "test-uuid-123", args.DocUUID)
}

