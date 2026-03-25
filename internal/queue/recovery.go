package queue

import (
	"context"
	"log/slog"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// JobInserter inserts jobs into the queue.
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
	FindByStatus(ctx context.Context, status string) ([]StuckDocument, error)
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
	uploaded, err := finder.FindByStatus(ctx, "uploaded")
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

	// Re-dispatch indexing for documents stuck in "extracted" state.
	extracted, err := finder.FindByStatus(ctx, "extracted")
	if err != nil {
		logger.Error("finding stuck extracted documents", "error", err)
	} else {
		for _, doc := range extracted {
			if _, insertErr := inserter.Insert(ctx, DocumentIndexArgs{
				DocumentID: doc.ID,
				DocUUID:    doc.UUID,
			}, nil); insertErr != nil {
				logger.Error("re-dispatching indexing for stuck document",
					"doc_id", doc.ID, "uuid", doc.UUID, "error", insertErr)
			} else {
				logger.Info("re-dispatched indexing for stuck document",
					"doc_id", doc.ID, "uuid", doc.UUID)
			}
		}
	}
}
