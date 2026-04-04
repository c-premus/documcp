package queue

import (
	"context"
	"log/slog"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/c-premus/documcp/internal/model"
)

// JobInserter inserts jobs into the queue.
// NOTE: An identical interface exists in internal/service/document_pipeline.go (same "define where consumed" idiom).
type JobInserter interface {
	Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// StuckDocument represents a document in an intermediate processing state.
type StuckDocument struct {
	ID   int64
	UUID string
}

// DocumentStatusFinder finds documents by processing status.
type DocumentStatusFinder interface {
	FindByStatus(ctx context.Context, status model.DocumentStatus) ([]StuckDocument, error)
}

// RecoverStuckDocuments re-dispatches jobs for documents stuck in "uploaded"
// or "extracted" states. Call after the River client starts.
func RecoverStuckDocuments(ctx context.Context, inserter JobInserter, finder DocumentStatusFinder, logger *slog.Logger) {
	if finder == nil || inserter == nil {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Re-dispatch extraction for documents stuck in "uploaded" state.
	uploaded, err := finder.FindByStatus(ctx, model.DocumentStatusUploaded)
	if err != nil {
		logger.Error("finding stuck uploaded documents", "error", err)
	} else {
		for _, doc := range uploaded {
			if _, insertErr := inserter.Insert(ctx, DocumentExtractArgs{
				DocumentID: doc.ID,
				DocUUID:    doc.UUID,
			}, nil); insertErr != nil {
				logger.Error("re-dispatching extraction for stuck document",
					"doc_id", doc.ID, "uuid", doc.UUID, "error", insertErr)
			} else {
				logger.Info("re-dispatched extraction for stuck document",
					"doc_id", doc.ID, "uuid", doc.UUID)
			}
		}
	}

	// Re-dispatch extraction for documents stuck in "extracted" state
	// (with PostgreSQL FTS, extraction now directly sets status to "indexed").
	extracted, err := finder.FindByStatus(ctx, model.DocumentStatus("extracted")) // legacy status for recovery of old data
	if err != nil {
		logger.Error("finding stuck extracted documents", "error", err)
	} else {
		for _, doc := range extracted {
			if _, insertErr := inserter.Insert(ctx, DocumentExtractArgs{
				DocumentID: doc.ID,
				DocUUID:    doc.UUID,
			}, nil); insertErr != nil {
				logger.Error("re-dispatching extraction for stuck document",
					"doc_id", doc.ID, "uuid", doc.UUID, "error", insertErr)
			} else {
				logger.Info("re-dispatched extraction for stuck document",
					"doc_id", doc.ID, "uuid", doc.UUID)
			}
		}
	}
}
