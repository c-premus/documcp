package queue

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/riverqueue/river"

	"git.999.haus/chris/DocuMCP-go/internal/model"
	"git.999.haus/chris/DocuMCP-go/internal/observability"
)

// retryBackoffs defines the exponential backoff schedule for retries.
var retryBackoffs = []time.Duration{60 * time.Second, 120 * time.Second, 300 * time.Second}

// nextRetryFromBackoffs returns the next retry time using the backoff schedule.
func nextRetryFromBackoffs(attempt int) time.Time {
	idx := max(0, min(attempt-1, len(retryBackoffs)-1))
	return time.Now().Add(retryBackoffs[idx])
}

// --- Interfaces (defined where consumed) ---.

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

// DocumentLister finds documents by status for reindexing.
type DocumentLister interface {
	FindByStatus(ctx context.Context, status string, limit int) ([]model.Document, error)
}

// --- Document Workers ---.

// DocumentExtractWorker processes document extraction jobs.
type DocumentExtractWorker struct {
	river.WorkerDefaults[DocumentExtractArgs]
	Pipeline DocumentProcessor
	Metrics  *observability.Metrics
}

// Work executes the document extraction job for a single document.
func (w *DocumentExtractWorker) Work(ctx context.Context, job *river.Job[DocumentExtractArgs]) error {
	if w.Pipeline == nil {
		return errors.New("extract worker not configured: pipeline is nil")
	}
	start := time.Now()
	err := w.Pipeline.ProcessDocument(ctx, job.Args.DocumentID)
	if err == nil {
		recordJobCompleted(w.Metrics, job.Queue, job.Kind, time.Since(start))
	}
	return err
}

// NextRetry returns the next retry time for a failed document extraction.
func (w *DocumentExtractWorker) NextRetry(job *river.Job[DocumentExtractArgs]) time.Time {
	return nextRetryFromBackoffs(job.Attempt)
}

// DocumentIndexWorker processes document indexing jobs.
type DocumentIndexWorker struct {
	river.WorkerDefaults[DocumentIndexArgs]
	Indexer DocumentIndexer
	Metrics *observability.Metrics
}

// Work executes the document indexing job for a single document.
func (w *DocumentIndexWorker) Work(ctx context.Context, job *river.Job[DocumentIndexArgs]) error {
	if w.Indexer == nil {
		return errors.New("index worker not configured: indexer is nil")
	}
	start := time.Now()
	err := w.Indexer.IndexDocumentByID(ctx, job.Args.DocumentID)
	if err == nil {
		recordJobCompleted(w.Metrics, job.Queue, job.Kind, time.Since(start))
	}
	return err
}

// NextRetry returns the next retry time for a failed document indexing.
func (w *DocumentIndexWorker) NextRetry(job *river.Job[DocumentIndexArgs]) time.Time {
	return nextRetryFromBackoffs(job.Attempt)
}

// ReindexAllWorker processes full reindex jobs.
type ReindexAllWorker struct {
	river.WorkerDefaults[ReindexAllArgs]
	Indexer DocumentIndexer
	Lister  DocumentLister
	Logger  *slog.Logger
}

// Work executes the full reindex job, re-indexing all processed documents.
func (w *ReindexAllWorker) Work(ctx context.Context, _ *river.Job[ReindexAllArgs]) error {
	if w.Lister == nil || w.Indexer == nil {
		return fmt.Errorf("reindex worker not configured: lister=%v, indexer=%v", w.Lister != nil, w.Indexer != nil)
	}

	logger := w.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// Query documents that have been processed (indexed or extracted).
	statuses := []string{"indexed", "processed", "extracted"}
	var total, succeeded, failed int

	for _, status := range statuses {
		docs, err := w.Lister.FindByStatus(ctx, status, 10000)
		if err != nil {
			logger.Error("reindex: failed to list documents", "status", status, "error", err)
			continue
		}

		for i := range docs {
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("reindex canceled after %d/%d documents: %w", succeeded, total, err)
			}

			total++
			if err := w.Indexer.IndexDocumentByID(ctx, docs[i].ID); err != nil {
				failed++
				logger.Warn("reindex: failed to index document", "doc_id", docs[i].ID, "error", err)
				continue
			}
			succeeded++
		}
	}

	logger.Info("reindex completed", "total", total, "succeeded", succeeded, "failed", failed)

	if failed > 0 {
		return fmt.Errorf("reindex completed with %d failures out of %d documents", failed, total)
	}
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
