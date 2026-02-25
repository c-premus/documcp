package search

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// DocumentRecord represents a document to be indexed in Meilisearch.
// Field names and JSON tags match the PHP version's index schema.
type DocumentRecord struct {
	UUID        string   `json:"uuid"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Content     string   `json:"content,omitempty"`
	FileType    string   `json:"file_type"`
	Tags        []string `json:"tags,omitempty"`
	Status      string   `json:"status"`
	UserID      *int64   `json:"user_id,omitempty"`
	IsPublic    bool     `json:"is_public"`
	WordCount   int      `json:"word_count,omitempty"`
	CreatedAt   string   `json:"created_at,omitempty"`
	UpdatedAt   string   `json:"updated_at,omitempty"`

	// Soft delete marker for Meilisearch filtering.
	SoftDeleted bool `json:"__soft_deleted"`
}

// Indexer handles adding, updating, and removing documents from Meilisearch indexes.
type Indexer struct {
	client *Client
	logger *slog.Logger
}

// NewIndexer creates an Indexer backed by the given Client.
func NewIndexer(client *Client, logger *slog.Logger) *Indexer {
	return &Indexer{client: client, logger: logger}
}

// IndexDocument adds or updates a single document in the documents index.
func (ix *Indexer) IndexDocument(ctx context.Context, doc DocumentRecord) error {
	idx := ix.client.ms.Index(IndexDocuments)
	task, err := idx.AddDocumentsWithContext(ctx, []DocumentRecord{doc}, nil)
	if err != nil {
		return fmt.Errorf("indexing document %s: %w", doc.UUID, err)
	}

	ix.logger.Debug("document indexed", "uuid", doc.UUID, "task_uid", task.TaskUID)
	return nil
}

// DeleteDocument removes a document from the documents index by UUID.
func (ix *Indexer) DeleteDocument(ctx context.Context, uuid string) error {
	idx := ix.client.ms.Index(IndexDocuments)
	task, err := idx.DeleteDocumentWithContext(ctx, uuid, nil)
	if err != nil {
		return fmt.Errorf("deleting document %s from index: %w", uuid, err)
	}

	ix.logger.Debug("document removed from index", "uuid", uuid, "task_uid", task.TaskUID)
	return nil
}

// SoftDeleteDocument marks a document as soft-deleted in the index rather than
// removing it. This matches the PHP version's behavior where deleted documents
// are filtered out via __soft_deleted=false.
func (ix *Indexer) SoftDeleteDocument(ctx context.Context, uuid string) error {
	idx := ix.client.ms.Index(IndexDocuments)
	record := map[string]any{
		"uuid":           uuid,
		"__soft_deleted": true,
	}
	task, err := idx.AddDocumentsWithContext(ctx, []map[string]any{record}, nil)
	if err != nil {
		return fmt.Errorf("soft-deleting document %s in index: %w", uuid, err)
	}

	ix.logger.Debug("document soft-deleted in index", "uuid", uuid, "task_uid", task.TaskUID)
	return nil
}

// IndexBatch adds or updates multiple documents in the documents index.
func (ix *Indexer) IndexBatch(ctx context.Context, docs []DocumentRecord) error {
	if len(docs) == 0 {
		return nil
	}

	idx := ix.client.ms.Index(IndexDocuments)
	task, err := idx.AddDocumentsWithContext(ctx, docs, nil)
	if err != nil {
		return fmt.Errorf("batch indexing %d documents: %w", len(docs), err)
	}

	ix.logger.Info("documents batch indexed", "count", len(docs), "task_uid", task.TaskUID)
	return nil
}

// WaitForTask blocks until a Meilisearch task completes.
func (ix *Indexer) WaitForTask(ctx context.Context, taskUID int64) error {
	_, err := ix.client.ms.WaitForTaskWithContext(ctx, taskUID, 500*time.Millisecond)
	if err != nil {
		return fmt.Errorf("waiting for task %d: %w", taskUID, err)
	}
	return nil
}

// Searcher returns a Searcher backed by this indexer's client.
func (ix *Indexer) Searcher() *Searcher {
	return NewSearcher(ix.client, ix.logger)
}
