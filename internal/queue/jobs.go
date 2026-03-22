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

// DocumentIndexArgs dispatches document search indexing.
type DocumentIndexArgs struct {
	DocumentID int64  `json:"document_id"`
	DocUUID    string `json:"doc_uuid"`
}

// Kind returns the job kind identifier for DocumentIndexArgs.
func (DocumentIndexArgs) Kind() string { return "document_index" }

// InsertOpts returns the River insert options for DocumentIndexArgs.
func (DocumentIndexArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       "default",
		Priority:    2,
		MaxAttempts: 4,
	}
}

// ReindexAllArgs dispatches a full reindex of all documents.
type ReindexAllArgs struct{}

// Kind returns the job kind identifier for ReindexAllArgs.
func (ReindexAllArgs) Kind() string { return "reindex_all" }

// InsertOpts returns the River insert options for ReindexAllArgs.
func (ReindexAllArgs) InsertOpts() river.InsertOpts {
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

// SyncConfluenceArgs dispatches Confluence space synchronization.
type SyncConfluenceArgs struct{}

// Kind returns the job kind identifier for SyncConfluenceArgs.
func (SyncConfluenceArgs) Kind() string { return "sync_confluence" }

// InsertOpts returns the River insert options for SyncConfluenceArgs.
func (SyncConfluenceArgs) InsertOpts() river.InsertOpts {
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

// VerifySearchIndexArgs dispatches search index consistency verification.
type VerifySearchIndexArgs struct{}

// Kind returns the job kind identifier for VerifySearchIndexArgs.
func (VerifySearchIndexArgs) Kind() string { return "verify_search_index" }

// InsertOpts returns the River insert options for VerifySearchIndexArgs.
func (VerifySearchIndexArgs) InsertOpts() river.InsertOpts {
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

// CleanupDisabledZimArgs dispatches removal of disabled ZIM archives from search.
type CleanupDisabledZimArgs struct{}

// Kind returns the job kind identifier for CleanupDisabledZimArgs.
func (CleanupDisabledZimArgs) Kind() string { return "cleanup_disabled_zim" }

// InsertOpts returns the River insert options for CleanupDisabledZimArgs.
func (CleanupDisabledZimArgs) InsertOpts() river.InsertOpts {
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
