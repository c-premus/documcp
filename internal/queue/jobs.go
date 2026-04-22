package queue

import (
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// --- Document processing jobs ---.

// DocumentExtractArgs dispatches document content extraction.
type DocumentExtractArgs struct {
	DocumentID int64  `json:"document_id"`
	DocUUID    string `json:"doc_uuid"`
	UserID     int64  `json:"user_id,omitempty"`
}

// Kind returns the job kind identifier for DocumentExtractArgs.
func (DocumentExtractArgs) Kind() string { return "document_extract" }

// InsertOpts returns the River insert options for DocumentExtractArgs.
func (DocumentExtractArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       "high",
		Priority:    1,
		MaxAttempts: 4,
	}
}

// --- Scheduler migration jobs ---.

// SyncKiwixArgs dispatches Kiwix ZIM archive synchronization.
type SyncKiwixArgs struct{}

// Kind returns the job kind identifier for SyncKiwixArgs.
func (SyncKiwixArgs) Kind() string { return "sync_kiwix" }

// InsertOpts returns the River insert options for SyncKiwixArgs.
func (SyncKiwixArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       "low",
		Priority:    4,
		MaxAttempts: 2,
		UniqueOpts: river.UniqueOpts{
			ByQueue: true,
			ByState: []rivertype.JobState{
				rivertype.JobStatePending,
				rivertype.JobStateAvailable,
				rivertype.JobStateRunning,
				rivertype.JobStateRetryable,
				rivertype.JobStateScheduled,
			},
		},
	}
}

// SyncGitTemplatesArgs dispatches Git template repository synchronization.
type SyncGitTemplatesArgs struct{}

// Kind returns the job kind identifier for SyncGitTemplatesArgs.
func (SyncGitTemplatesArgs) Kind() string { return "sync_git_templates" }

// InsertOpts returns the River insert options for SyncGitTemplatesArgs.
func (SyncGitTemplatesArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       "low",
		Priority:    4,
		MaxAttempts: 2,
		UniqueOpts: river.UniqueOpts{
			ByQueue: true,
			ByState: []rivertype.JobState{
				rivertype.JobStatePending,
				rivertype.JobStateAvailable,
				rivertype.JobStateRunning,
				rivertype.JobStateRetryable,
				rivertype.JobStateScheduled,
			},
		},
	}
}

// CleanupOAuthTokensArgs dispatches expired OAuth token purge.
type CleanupOAuthTokensArgs struct{}

// Kind returns the job kind identifier for CleanupOAuthTokensArgs.
func (CleanupOAuthTokensArgs) Kind() string { return "cleanup_oauth_tokens" }

// InsertOpts returns the River insert options for CleanupOAuthTokensArgs.
func (CleanupOAuthTokensArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       "low",
		Priority:    4,
		MaxAttempts: 2,
		UniqueOpts: river.UniqueOpts{
			ByQueue: true,
			ByState: []rivertype.JobState{
				rivertype.JobStatePending,
				rivertype.JobStateAvailable,
				rivertype.JobStateRunning,
				rivertype.JobStateRetryable,
				rivertype.JobStateScheduled,
			},
		},
	}
}

// CleanupOrphanedFilesArgs dispatches orphaned file cleanup.
type CleanupOrphanedFilesArgs struct{}

// Kind returns the job kind identifier for CleanupOrphanedFilesArgs.
func (CleanupOrphanedFilesArgs) Kind() string { return "cleanup_orphaned_files" }

// InsertOpts returns the River insert options for CleanupOrphanedFilesArgs.
func (CleanupOrphanedFilesArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       "low",
		Priority:    4,
		MaxAttempts: 2,
		UniqueOpts: river.UniqueOpts{
			ByQueue: true,
			ByState: []rivertype.JobState{
				rivertype.JobStatePending,
				rivertype.JobStateAvailable,
				rivertype.JobStateRunning,
				rivertype.JobStateRetryable,
				rivertype.JobStateScheduled,
			},
		},
	}
}

// PurgeSoftDeletedArgs dispatches permanent removal of soft-deleted documents.
type PurgeSoftDeletedArgs struct{}

// Kind returns the job kind identifier for PurgeSoftDeletedArgs.
func (PurgeSoftDeletedArgs) Kind() string { return "purge_soft_deleted" }

// InsertOpts returns the River insert options for PurgeSoftDeletedArgs.
func (PurgeSoftDeletedArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       "low",
		Priority:    4,
		MaxAttempts: 2,
		UniqueOpts: river.UniqueOpts{
			ByQueue: true,
			ByState: []rivertype.JobState{
				rivertype.JobStatePending,
				rivertype.JobStateAvailable,
				rivertype.JobStateRunning,
				rivertype.JobStateRetryable,
				rivertype.JobStateScheduled,
			},
		},
	}
}

// CleanupSearchQueriesArgs dispatches retention-based cleanup of search_queries rows.
type CleanupSearchQueriesArgs struct{}

// Kind returns the job kind identifier for CleanupSearchQueriesArgs.
func (CleanupSearchQueriesArgs) Kind() string { return "cleanup_search_queries" }

// InsertOpts returns the River insert options for CleanupSearchQueriesArgs.
func (CleanupSearchQueriesArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       "low",
		Priority:    4,
		MaxAttempts: 2,
		UniqueOpts: river.UniqueOpts{
			ByQueue: true,
			ByState: []rivertype.JobState{
				rivertype.JobStatePending,
				rivertype.JobStateAvailable,
				rivertype.JobStateRunning,
				rivertype.JobStateRetryable,
				rivertype.JobStateScheduled,
			},
		},
	}
}

// HealthCheckServicesArgs dispatches external service health checks.
type HealthCheckServicesArgs struct{}

// Kind returns the job kind identifier for HealthCheckServicesArgs.
func (HealthCheckServicesArgs) Kind() string { return "health_check_services" }

// InsertOpts returns the River insert options for HealthCheckServicesArgs.
func (HealthCheckServicesArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       "low",
		Priority:    4,
		MaxAttempts: 2,
		UniqueOpts: river.UniqueOpts{
			ByQueue: true,
			ByState: []rivertype.JobState{
				rivertype.JobStatePending,
				rivertype.JobStateAvailable,
				rivertype.JobStateRunning,
				rivertype.JobStateRetryable,
				rivertype.JobStateScheduled,
			},
		},
	}
}
