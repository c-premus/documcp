package queue

import (
	"context"
	"errors"
	"time"

	"github.com/riverqueue/river"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/c-premus/documcp/internal/observability"
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

// --- Document Workers ---.

// DocumentExtractWorker processes document extraction jobs.
type DocumentExtractWorker struct {
	river.WorkerDefaults[DocumentExtractArgs]
	Pipeline DocumentProcessor
	Metrics  *observability.Metrics
}

// Work executes the document extraction job for a single document.
func (w *DocumentExtractWorker) Work(ctx context.Context, job *river.Job[DocumentExtractArgs]) (retErr error) {
	ctx, span := workerTracer.Start(ctx, "job."+job.Kind,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(attribute.String("job.kind", job.Kind)),
	)
	defer func() {
		if retErr != nil {
			span.RecordError(retErr)
			span.SetStatus(codes.Error, retErr.Error())
		}
		span.End()
	}()

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

// recordJobCompleted increments the completed counter and observes duration.
func recordJobCompleted(m *observability.Metrics, queue, kind string, d time.Duration) {
	if m == nil {
		return
	}
	m.QueueJobsCompleted.WithLabelValues(queue, kind).Inc()
	m.QueueJobDuration.WithLabelValues(queue, kind).Observe(d.Seconds())
}
