package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"git.999.haus/chris/DocuMCP-go/internal/extractor"
	"git.999.haus/chris/DocuMCP-go/internal/model"
	"git.999.haus/chris/DocuMCP-go/internal/queue"
	"git.999.haus/chris/DocuMCP-go/internal/search"
)

// JobInserter inserts jobs into the queue. Defined here (where consumed).
type JobInserter interface {
	Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// maxUploadSize is the maximum allowed file size (50 MB).
const maxUploadSize = 50 * 1024 * 1024

// AllowedMIMETypes maps file extensions to their MIME types.
var AllowedMIMETypes = map[string]string{
	".pdf":  "application/pdf",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".html": "text/html",
	".htm":  "text/html",
	".md":   "text/markdown",
	".txt":  "text/plain",
}

// UploadDocumentParams holds input for uploading a file-based document.
type UploadDocumentParams struct {
	Title       string
	Description string
	FileName    string
	FileSize    int64
	Reader      io.Reader
	IsPublic    bool
	Tags        []string
	UserID      *int64
}

// DocumentPipeline orchestrates file upload, content extraction, and search
// indexing. It extends DocumentService with pipeline capabilities.
type DocumentPipeline struct {
	*DocumentService
	extractorRegistry *extractor.Registry
	indexer           *search.Indexer
	inserter          JobInserter
	storagePath       string
}

// NewDocumentPipeline creates a DocumentPipeline.
func NewDocumentPipeline(
	svc *DocumentService,
	registry *extractor.Registry,
	indexer *search.Indexer,
	inserter JobInserter,
	storagePath string,
) *DocumentPipeline {
	return &DocumentPipeline{
		DocumentService:   svc,
		extractorRegistry: registry,
		indexer:           indexer,
		inserter:          inserter,
		storagePath:       storagePath,
	}
}

// StoragePath returns the base storage directory for uploaded documents.
func (p *DocumentPipeline) StoragePath() string {
	return p.storagePath
}

// ExtractorRegistry returns the content extractor registry.
func (p *DocumentPipeline) ExtractorRegistry() *extractor.Registry {
	return p.extractorRegistry
}

// Upload stores a file, creates a DB record with status "uploaded", and
// dispatches background jobs for extraction and indexing.
func (p *DocumentPipeline) Upload(ctx context.Context, params UploadDocumentParams) (*model.Document, error) {
	if params.FileSize > maxUploadSize {
		return nil, fmt.Errorf("%w: %d bytes exceeds limit of %d", ErrFileTooLarge, params.FileSize, maxUploadSize)
	}

	ext := strings.ToLower(filepath.Ext(params.FileName))
	mimeType, ok := AllowedMIMETypes[ext]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedFileType, ext)
	}

	fileType := strings.TrimPrefix(ext, ".")
	if fileType == "htm" {
		fileType = "html"
	}
	if fileType == "txt" {
		fileType = "markdown"
	}

	// Store file to disk.
	docUUID := uuid.New().String()
	relPath := filepath.Join(fileType, docUUID+ext)
	absPath := filepath.Join(p.storagePath, relPath)

	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return nil, fmt.Errorf("creating storage directory: %w", err)
	}

	f, err := os.Create(absPath)
	if err != nil {
		return nil, fmt.Errorf("creating file %s: %w", absPath, err)
	}

	written, err := io.Copy(f, params.Reader)
	if closeErr := f.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(absPath)
		return nil, fmt.Errorf("writing uploaded file: %w", err)
	}

	// Create DB record.
	doc := &model.Document{
		UUID:     docUUID,
		Title:    params.Title,
		FileType: fileType,
		FilePath: relPath,
		FileSize: written,
		MIMEType: mimeType,
		IsPublic: params.IsPublic,
		Status:   "uploaded",
		Description: sql.NullString{
			String: params.Description,
			Valid:  params.Description != "",
		},
	}
	if params.UserID != nil {
		doc.UserID = sql.NullInt64{Int64: *params.UserID, Valid: true}
	}

	if err := p.repo.Create(ctx, doc); err != nil {
		_ = os.Remove(absPath)
		return nil, fmt.Errorf("creating document record: %w", err)
	}

	if len(params.Tags) > 0 {
		if err := p.repo.ReplaceTags(ctx, doc.ID, params.Tags); err != nil {
			return nil, fmt.Errorf("setting tags on uploaded document: %w", err)
		}
	}

	// Dispatch background extraction job.
	p.dispatchExtraction(doc.ID, docUUID)

	created, err := p.repo.FindByID(ctx, doc.ID)
	if err != nil {
		return nil, fmt.Errorf("re-fetching uploaded document: %w", err)
	}
	return created, nil
}

// ProcessDocument extracts content from a document and updates its DB record.
// This is called by the background worker.
func (p *DocumentPipeline) ProcessDocument(ctx context.Context, docID int64) error {
	doc, err := p.repo.FindByID(ctx, docID)
	if err != nil {
		return fmt.Errorf("finding document %d for processing: %w", docID, err)
	}

	absPath := filepath.Join(p.storagePath, doc.FilePath)

	ext, err := p.extractorRegistry.ForMIMEType(doc.MIMEType)
	if err != nil {
		return p.markFailed(ctx, doc, fmt.Sprintf("no extractor for %s: %v", doc.MIMEType, err))
	}

	result, err := ext.Extract(ctx, absPath)
	if err != nil {
		return p.markFailed(ctx, doc, fmt.Sprintf("extraction failed: %v", err))
	}

	// Compute content hash.
	hash := sha256.Sum256([]byte(result.Content))
	contentHash := hex.EncodeToString(hash[:])
	now := time.Now()

	doc.Content = sql.NullString{String: result.Content, Valid: true}
	doc.ContentHash = sql.NullString{String: contentHash, Valid: true}
	doc.WordCount = sql.NullInt64{Int64: int64(result.WordCount), Valid: true}
	doc.ProcessedAt = sql.NullTime{Time: now, Valid: true}
	doc.Status = "extracted"
	doc.ErrorMessage = sql.NullString{}

	if err := p.repo.Update(ctx, doc); err != nil {
		return fmt.Errorf("updating document %d after extraction: %w", docID, err)
	}

	p.logger.Info("document extracted",
		"id", docID,
		"uuid", doc.UUID,
		"word_count", result.WordCount,
	)

	// Dispatch indexing job.
	p.dispatchIndexing(doc)

	return nil
}

// IndexDocument indexes a document in Meilisearch and updates its DB record.
func (p *DocumentPipeline) IndexDocument(ctx context.Context, doc *model.Document) error {
	if p.indexer == nil {
		return nil
	}

	tags, err := p.repo.TagsForDocument(ctx, doc.ID)
	if err != nil {
		return fmt.Errorf("loading tags for indexing document %d: %w", doc.ID, err)
	}

	tagNames := make([]string, len(tags))
	for i, t := range tags {
		tagNames[i] = t.Tag
	}

	record := search.DocumentRecord{
		UUID:        doc.UUID,
		Title:       doc.Title,
		Description: doc.Description.String,
		Content:     doc.Content.String,
		FileType:    doc.FileType,
		Tags:        tagNames,
		Status:      doc.Status,
		IsPublic:    doc.IsPublic,
		WordCount:   int(doc.WordCount.Int64),
		SoftDeleted: false,
	}
	if doc.UserID.Valid {
		uid := doc.UserID.Int64
		record.UserID = &uid
	}
	if doc.CreatedAt.Valid {
		record.CreatedAt = doc.CreatedAt.Time.Format(time.RFC3339)
	}
	if doc.UpdatedAt.Valid {
		record.UpdatedAt = doc.UpdatedAt.Time.Format(time.RFC3339)
	}

	if err := p.indexer.IndexDocument(ctx, record); err != nil {
		return p.markIndexFailed(ctx, doc, fmt.Sprintf("indexing failed: %v", err))
	}

	now := time.Now()
	doc.Status = "indexed"
	doc.MeilisearchIndexedAt = sql.NullTime{Time: now, Valid: true}
	doc.ErrorMessage = sql.NullString{}

	if err := p.repo.Update(ctx, doc); err != nil {
		return fmt.Errorf("updating document %d after indexing: %w", doc.ID, err)
	}

	p.logger.Info("document indexed", "id", doc.ID, "uuid", doc.UUID)
	return nil
}

// dispatchExtraction enqueues a document extraction job via River.
func (p *DocumentPipeline) dispatchExtraction(docID int64, docUUID string) {
	if p.inserter == nil {
		return
	}

	if _, err := p.inserter.Insert(context.Background(), queue.DocumentExtractArgs{
		DocumentID: docID,
		DocUUID:    docUUID,
	}, nil); err != nil {
		p.logger.Error("failed to dispatch extraction job", "doc_id", docID, "uuid", docUUID, "error", err)
	}
}

// dispatchIndexing enqueues a document indexing job via River.
func (p *DocumentPipeline) dispatchIndexing(doc *model.Document) {
	if p.inserter == nil || p.indexer == nil {
		return
	}

	if _, err := p.inserter.Insert(context.Background(), queue.DocumentIndexArgs{
		DocumentID: doc.ID,
		DocUUID:    doc.UUID,
	}, nil); err != nil {
		p.logger.Error("failed to dispatch indexing job", "doc_id", doc.ID, "uuid", doc.UUID, "error", err)
	}
}

// IndexDocumentByID fetches a document by ID and indexes it in Meilisearch.
// Used by the River DocumentIndexWorker.
func (p *DocumentPipeline) IndexDocumentByID(ctx context.Context, docID int64) error {
	doc, err := p.repo.FindByID(ctx, docID)
	if err != nil {
		return fmt.Errorf("finding document %d for indexing: %w", docID, err)
	}
	return p.IndexDocument(ctx, doc)
}

// markFailed updates a document's status to "failed" with an error message.
func (p *DocumentPipeline) markFailed(ctx context.Context, doc *model.Document, errMsg string) error {
	doc.Status = "failed"
	doc.ErrorMessage = sql.NullString{String: errMsg, Valid: true}
	if err := p.repo.Update(ctx, doc); err != nil {
		return fmt.Errorf("marking document %d as failed: %w", doc.ID, err)
	}
	p.logger.Error("document processing failed", "id", doc.ID, "uuid", doc.UUID, "error", errMsg)
	return fmt.Errorf("document %d: %s", doc.ID, errMsg)
}

// markIndexFailed updates a document's status to "index_failed".
func (p *DocumentPipeline) markIndexFailed(ctx context.Context, doc *model.Document, errMsg string) error {
	doc.Status = "index_failed"
	doc.ErrorMessage = sql.NullString{String: errMsg, Valid: true}
	if err := p.repo.Update(ctx, doc); err != nil {
		return fmt.Errorf("marking document %d as index_failed: %w", doc.ID, err)
	}
	p.logger.Error("document indexing failed", "id", doc.ID, "uuid", doc.UUID, "error", errMsg)
	return fmt.Errorf("document %d: %s", doc.ID, errMsg)
}
