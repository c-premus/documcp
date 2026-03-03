package queue

import (
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// --- Document processing jobs ---

// DocumentExtractArgs dispatches document content extraction.
type DocumentExtractArgs struct {
	DocumentID int64  `json:"document_id"`
	DocUUID    string `json:"doc_uuid"`
}

func (DocumentExtractArgs) Kind() string { return "document_extract" }

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

func (DocumentIndexArgs) Kind() string { return "document_index" }

func (DocumentIndexArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       "default",
		Priority:    2,
		MaxAttempts: 4,
	}
}

// ReindexAllArgs dispatches a full reindex of all documents.
type ReindexAllArgs struct{}

func (ReindexAllArgs) Kind() string { return "reindex_all" }

func (ReindexAllArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       "low",
		Priority:    4,
		MaxAttempts: 2,
		UniqueOpts: river.UniqueOpts{
			ByQueue: true,
			ByState: []rivertype.JobState{
				rivertype.JobStateAvailable,
				rivertype.JobStateRunning,
				rivertype.JobStateRetryable,
				rivertype.JobStateScheduled,
			},
		},
	}
}

// --- Scheduler migration jobs ---

// SyncKiwixArgs dispatches Kiwix ZIM archive synchronization.
type SyncKiwixArgs struct{}

func (SyncKiwixArgs) Kind() string { return "sync_kiwix" }

func (SyncKiwixArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       "low",
		Priority:    4,
		MaxAttempts: 2,
		UniqueOpts: river.UniqueOpts{
			ByQueue: true,
			ByState: []rivertype.JobState{
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

func (SyncConfluenceArgs) Kind() string { return "sync_confluence" }

func (SyncConfluenceArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       "low",
		Priority:    4,
		MaxAttempts: 2,
		UniqueOpts: river.UniqueOpts{
			ByQueue: true,
			ByState: []rivertype.JobState{
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

func (SyncGitTemplatesArgs) Kind() string { return "sync_git_templates" }

func (SyncGitTemplatesArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       "low",
		Priority:    4,
		MaxAttempts: 2,
		UniqueOpts: river.UniqueOpts{
			ByQueue: true,
			ByState: []rivertype.JobState{
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

func (CleanupOAuthTokensArgs) Kind() string { return "cleanup_oauth_tokens" }

func (CleanupOAuthTokensArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       "low",
		Priority:    4,
		MaxAttempts: 2,
		UniqueOpts: river.UniqueOpts{
			ByQueue: true,
			ByState: []rivertype.JobState{
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

func (CleanupOrphanedFilesArgs) Kind() string { return "cleanup_orphaned_files" }

func (CleanupOrphanedFilesArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       "low",
		Priority:    4,
		MaxAttempts: 2,
		UniqueOpts: river.UniqueOpts{
			ByQueue: true,
			ByState: []rivertype.JobState{
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

func (VerifySearchIndexArgs) Kind() string { return "verify_search_index" }

func (VerifySearchIndexArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       "low",
		Priority:    4,
		MaxAttempts: 2,
		UniqueOpts: river.UniqueOpts{
			ByQueue: true,
			ByState: []rivertype.JobState{
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

func (PurgeSoftDeletedArgs) Kind() string { return "purge_soft_deleted" }

func (PurgeSoftDeletedArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       "low",
		Priority:    4,
		MaxAttempts: 2,
		UniqueOpts: river.UniqueOpts{
			ByQueue: true,
			ByState: []rivertype.JobState{
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

func (CleanupDisabledZimArgs) Kind() string { return "cleanup_disabled_zim" }

func (CleanupDisabledZimArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       "low",
		Priority:    4,
		MaxAttempts: 2,
		UniqueOpts: river.UniqueOpts{
			ByQueue: true,
			ByState: []rivertype.JobState{
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

func (HealthCheckServicesArgs) Kind() string { return "health_check_services" }

func (HealthCheckServicesArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       "low",
		Priority:    4,
		MaxAttempts: 2,
		UniqueOpts: river.UniqueOpts{
			ByQueue: true,
			ByState: []rivertype.JobState{
				rivertype.JobStateAvailable,
				rivertype.JobStateRunning,
				rivertype.JobStateRetryable,
				rivertype.JobStateScheduled,
			},
		},
	}
}
