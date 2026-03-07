package queue

import (
	"context"
	"time"

	"github.com/riverqueue/river"

	"git.999.haus/chris/DocuMCP-go/internal/observability"
)

// retryBackoffs defines the exponential backoff schedule for retries.
var retryBackoffs = []time.Duration{60 * time.Second, 120 * time.Second, 300 * time.Second}

// nextRetryFromBackoffs returns the next retry time using the backoff schedule.
func nextRetryFromBackoffs(attempt int) time.Time {
	idx := max(0, min(attempt-1, len(retryBackoffs)-1))
	return time.Now().Add(retryBackoffs[idx])
}

// --- Interfaces (defined where consumed) ---

// DocumentProcessor extracts content from a document.
// Implemented by *service.DocumentPipeline.
type DocumentProcessor interface {
	ProcessDocument(ctx context.Context, docID int64) error
}

// DocumentIndexer indexes a document in the search engine.
// Implemented by *service.DocumentPipeline.
type DocumentIndexer interface {
	IndexDocumentByID(ctx context.Context, docID int64) error
}

// --- Document Workers ---

// DocumentExtractWorker processes document extraction jobs.
type DocumentExtractWorker struct {
	river.WorkerDefaults[DocumentExtractArgs]
	Pipeline DocumentProcessor
	Metrics  *observability.Metrics
}

func (w *DocumentExtractWorker) Work(ctx context.Context, job *river.Job[DocumentExtractArgs]) error {
	start := time.Now()
	err := w.Pipeline.ProcessDocument(ctx, job.Args.DocumentID)
	if err == nil {
		recordJobCompleted(w.Metrics, job.Queue, job.Kind, time.Since(start))
	}
	return err
}

func (w *DocumentExtractWorker) NextRetry(job *river.Job[DocumentExtractArgs]) time.Time {
	return nextRetryFromBackoffs(job.Attempt)
}

// DocumentIndexWorker processes document indexing jobs.
type DocumentIndexWorker struct {
	river.WorkerDefaults[DocumentIndexArgs]
	Indexer DocumentIndexer
	Metrics *observability.Metrics
}

func (w *DocumentIndexWorker) Work(ctx context.Context, job *river.Job[DocumentIndexArgs]) error {
	start := time.Now()
	err := w.Indexer.IndexDocumentByID(ctx, job.Args.DocumentID)
	if err == nil {
		recordJobCompleted(w.Metrics, job.Queue, job.Kind, time.Since(start))
	}
	return err
}

func (w *DocumentIndexWorker) NextRetry(job *river.Job[DocumentIndexArgs]) time.Time {
	return nextRetryFromBackoffs(job.Attempt)
}

// ReindexAllWorker processes full reindex jobs.
type ReindexAllWorker struct {
	river.WorkerDefaults[ReindexAllArgs]
	Indexer DocumentIndexer
}

func (w *ReindexAllWorker) Work(_ context.Context, _ *river.Job[ReindexAllArgs]) error {
	// Placeholder: full reindex logic would iterate all documents.
	// This is dispatched manually via the admin API, not via periodic jobs.
	return nil
}

// recordJobCompleted increments the completed counter and observes duration.
func recordJobCompleted(m *observability.Metrics, queue, kind string, d time.Duration) {
	if m == nil {
		return
	}
	m.QueueJobsCompleted.WithLabelValues(queue, kind).Inc()
	m.QueueJobDuration.WithLabelValues(queue, kind).Observe(d.Seconds())
}
