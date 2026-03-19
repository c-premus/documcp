package search

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/meilisearch/meilisearch-go"
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

// ListIndexedDocumentUUIDs returns a set of all document UUIDs currently in the search index.
func (ix *Indexer) ListIndexedDocumentUUIDs(ctx context.Context) (map[string]bool, error) {
	idx := ix.client.ms.Index(IndexDocuments)
	uuids := make(map[string]bool)

	const pageSize int64 = 1000
	offset := int64(0)

	for {
		var result meilisearch.DocumentsResult
		err := idx.GetDocumentsWithContext(ctx, &meilisearch.DocumentsQuery{
			Fields: []string{"uuid"},
			Limit:  pageSize,
			Offset: offset,
		}, &result)
		if err != nil {
			return nil, fmt.Errorf("listing indexed document uuids at offset %d: %w", offset, err)
		}

		for _, hit := range result.Results {
			if raw, ok := hit["uuid"]; ok {
				var uuid string
				if err := json.Unmarshal(raw, &uuid); err == nil {
					uuids[uuid] = true
				}
			}
		}

		offset += int64(len(result.Results))
		if offset >= result.Total {
			break
		}
	}

	ix.logger.Debug("listed indexed document uuids", "count", len(uuids))
	return uuids, nil
}

// Searcher returns a Searcher backed by this indexer's client.
func (ix *Indexer) Searcher() *Searcher {
	return NewSearcher(ix.client, ix.logger)
}

//nolint:godot // ---------------------------------------------------------------------------
// ZIM archive indexing.
//nolint:godot // ---------------------------------------------------------------------------

// ZimArchiveRecord represents a ZIM archive to be indexed in Meilisearch.
type ZimArchiveRecord struct {
	UUID         string   `json:"uuid"`
	Name         string   `json:"name"`
	Title        string   `json:"title"`
	Description  string   `json:"description,omitempty"`
	Language     string   `json:"language"`
	Category     string   `json:"category,omitempty"`
	Creator      string   `json:"creator,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	ArticleCount int64    `json:"article_count"`
}

// IndexZimArchive adds or updates a ZIM archive in the zim_archives index.
func (ix *Indexer) IndexZimArchive(ctx context.Context, rec ZimArchiveRecord) error {
	idx := ix.client.ms.Index(IndexZimArchives)
	task, err := idx.AddDocumentsWithContext(ctx, []ZimArchiveRecord{rec}, nil)
	if err != nil {
		return fmt.Errorf("indexing ZIM archive %s: %w", rec.UUID, err)
	}

	ix.logger.Debug("ZIM archive indexed", "uuid", rec.UUID, "task_uid", task.TaskUID)
	return nil
}

// DeleteZimArchive removes a ZIM archive from the index by UUID.
func (ix *Indexer) DeleteZimArchive(ctx context.Context, uuid string) error {
	idx := ix.client.ms.Index(IndexZimArchives)
	task, err := idx.DeleteDocumentWithContext(ctx, uuid, nil)
	if err != nil {
		return fmt.Errorf("deleting ZIM archive %s from index: %w", uuid, err)
	}

	ix.logger.Debug("ZIM archive removed from index", "uuid", uuid, "task_uid", task.TaskUID)
	return nil
}

// IndexZimArchiveBatch adds or updates multiple ZIM archives in the zim_archives index.
func (ix *Indexer) IndexZimArchiveBatch(ctx context.Context, recs []ZimArchiveRecord) error {
	if len(recs) == 0 {
		return nil
	}

	idx := ix.client.ms.Index(IndexZimArchives)
	task, err := idx.AddDocumentsWithContext(ctx, recs, nil)
	if err != nil {
		return fmt.Errorf("batch indexing %d ZIM archives: %w", len(recs), err)
	}

	ix.logger.Info("ZIM archives batch indexed", "count", len(recs), "task_uid", task.TaskUID)
	return nil
}

//nolint:godot // ---------------------------------------------------------------------------
// Confluence space indexing.
//nolint:godot // ---------------------------------------------------------------------------

// ConfluenceSpaceRecord represents a Confluence space to be indexed.
type ConfluenceSpaceRecord struct {
	UUID              string `json:"uuid"`
	ConfluenceID      string `json:"confluence_id"`
	Key               string `json:"key"`
	Name              string `json:"name"`
	Description       string `json:"description,omitempty"`
	Type              string `json:"type"`
	Status            string `json:"status,omitempty"`
	ExternalServiceID int64  `json:"external_service_id"`
	IsEnabled         bool   `json:"is_enabled"`
	SoftDeleted       bool   `json:"__soft_deleted"`
}

// IndexConfluenceSpace adds or updates a Confluence space in the confluence_spaces index.
func (ix *Indexer) IndexConfluenceSpace(ctx context.Context, rec ConfluenceSpaceRecord) error {
	idx := ix.client.ms.Index(IndexConfluenceSpaces)
	task, err := idx.AddDocumentsWithContext(ctx, []ConfluenceSpaceRecord{rec}, nil)
	if err != nil {
		return fmt.Errorf("indexing Confluence space %s: %w", rec.UUID, err)
	}

	ix.logger.Debug("Confluence space indexed", "uuid", rec.UUID, "task_uid", task.TaskUID)
	return nil
}

// DeleteConfluenceSpace removes a Confluence space from the index by UUID.
func (ix *Indexer) DeleteConfluenceSpace(ctx context.Context, uuid string) error {
	idx := ix.client.ms.Index(IndexConfluenceSpaces)
	task, err := idx.DeleteDocumentWithContext(ctx, uuid, nil)
	if err != nil {
		return fmt.Errorf("deleting Confluence space %s from index: %w", uuid, err)
	}

	ix.logger.Debug("Confluence space removed from index", "uuid", uuid, "task_uid", task.TaskUID)
	return nil
}

// SoftDeleteConfluenceSpace marks a Confluence space as soft-deleted in the index
// rather than removing it. Soft-deleted records are filtered out via __soft_deleted=false.
func (ix *Indexer) SoftDeleteConfluenceSpace(ctx context.Context, uuid string) error {
	idx := ix.client.ms.Index(IndexConfluenceSpaces)
	record := map[string]any{
		"uuid":           uuid,
		"__soft_deleted": true,
	}
	task, err := idx.AddDocumentsWithContext(ctx, []map[string]any{record}, nil)
	if err != nil {
		return fmt.Errorf("soft-deleting Confluence space %s in index: %w", uuid, err)
	}

	ix.logger.Debug("Confluence space soft-deleted in index", "uuid", uuid, "task_uid", task.TaskUID)
	return nil
}

//nolint:godot // ---------------------------------------------------------------------------
// Git template indexing.
//nolint:godot // ---------------------------------------------------------------------------

// GitTemplateRecord represents a Git template to be indexed.
type GitTemplateRecord struct {
	UUID          string   `json:"uuid"`
	Name          string   `json:"name"`
	Slug          string   `json:"slug"`
	Description   string   `json:"description,omitempty"`
	ReadmeContent string   `json:"readme_content,omitempty"`
	Category      string   `json:"category,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	UserID        *int64   `json:"user_id,omitempty"`
	IsPublic      bool     `json:"is_public"`
	Status        string   `json:"status"`
	SoftDeleted   bool     `json:"__soft_deleted"`
}

// IndexGitTemplate adds or updates a Git template in the git_templates index.
func (ix *Indexer) IndexGitTemplate(ctx context.Context, rec GitTemplateRecord) error {
	idx := ix.client.ms.Index(IndexGitTemplates)
	task, err := idx.AddDocumentsWithContext(ctx, []GitTemplateRecord{rec}, nil)
	if err != nil {
		return fmt.Errorf("indexing Git template %s: %w", rec.UUID, err)
	}

	ix.logger.Debug("Git template indexed", "uuid", rec.UUID, "task_uid", task.TaskUID)
	return nil
}

// DeleteGitTemplate removes a Git template from the index by UUID.
func (ix *Indexer) DeleteGitTemplate(ctx context.Context, uuid string) error {
	idx := ix.client.ms.Index(IndexGitTemplates)
	task, err := idx.DeleteDocumentWithContext(ctx, uuid, nil)
	if err != nil {
		return fmt.Errorf("deleting Git template %s from index: %w", uuid, err)
	}

	ix.logger.Debug("Git template removed from index", "uuid", uuid, "task_uid", task.TaskUID)
	return nil
}

// SoftDeleteGitTemplate marks a Git template as soft-deleted in the index
// rather than removing it. Soft-deleted records are filtered out via __soft_deleted=false.
func (ix *Indexer) SoftDeleteGitTemplate(ctx context.Context, uuid string) error {
	idx := ix.client.ms.Index(IndexGitTemplates)
	record := map[string]any{
		"uuid":           uuid,
		"__soft_deleted": true,
	}
	task, err := idx.AddDocumentsWithContext(ctx, []map[string]any{record}, nil)
	if err != nil {
		return fmt.Errorf("soft-deleting Git template %s in index: %w", uuid, err)
	}

	ix.logger.Debug("Git template soft-deleted in index", "uuid", uuid, "task_uid", task.TaskUID)
	return nil
}
