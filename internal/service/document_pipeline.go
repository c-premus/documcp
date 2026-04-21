// Package service implements business logic and orchestration.
package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/c-premus/documcp/internal/extractor"
	"github.com/c-premus/documcp/internal/model"
	"github.com/c-premus/documcp/internal/queue"
	"github.com/c-premus/documcp/internal/storage"
)

// JobInserter inserts jobs into the queue. Defined here (where consumed).
// NOTE: An identical interface exists in internal/queue/recovery.go (same "define where consumed" idiom).
type JobInserter interface {
	Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// defaultMaxUploadSize is the maximum allowed file size (50 MB).
const defaultMaxUploadSize = 50 * 1024 * 1024

// AllowedMIMETypes maps file extensions to their MIME types.
var AllowedMIMETypes = map[string]string{
	".pdf":  "application/pdf",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".html": "text/html",
	".htm":  "text/html",
	".md":   "text/markdown",
	".txt":  "text/plain",
	".epub": "application/epub+zip",
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

// ReplaceContentParams holds input for replacing a document's file content.
type ReplaceContentParams struct {
	FileName string
	FileSize int64
	Reader   io.Reader
	UserID   *int64
}

// DocumentPipeline orchestrates file upload, content extraction, and search
// indexing. It extends DocumentService with pipeline capabilities.
//
// Files flow through a Blob abstraction so the pipeline works against either
// a local filesystem or S3-compatible object storage. For extraction, which
// requires libraries like ledongthuc/pdf that take a filesystem path,
// ProcessDocument stages the blob to a node-local temp file under
// workerTempDir and cleans up after extraction.
type DocumentPipeline struct {
	*DocumentService
	extractorRegistry *extractor.Registry
	inserter          JobInserter
	blob              storage.Blob
	workerTempDir     string
	maxUploadSize     int64
}

// NewDocumentPipeline creates a DocumentPipeline. blob is the backing blob
// store; workerTempDir is a node-local scratch directory used to stage blob
// reads before calling path-based extractors.
func NewDocumentPipeline(
	svc *DocumentService,
	registry *extractor.Registry,
	inserter JobInserter,
	blob storage.Blob,
	workerTempDir string,
	maxUploadSize int64,
) *DocumentPipeline {
	if maxUploadSize <= 0 {
		maxUploadSize = defaultMaxUploadSize
	}
	return &DocumentPipeline{
		DocumentService:   svc,
		extractorRegistry: registry,
		inserter:          inserter,
		blob:              blob,
		workerTempDir:     workerTempDir,
		maxUploadSize:     maxUploadSize,
	}
}

// Delete soft-deletes a document by UUID.
func (p *DocumentPipeline) Delete(ctx context.Context, docUUID string) error {
	return p.DocumentService.Delete(ctx, docUUID)
}

// deleteOrphanBlob best-effort removes a blob whose DB record failed to
// persist. We log on error rather than swallow silently — orphan keys
// otherwise become invisible and leak storage.
func (p *DocumentPipeline) deleteOrphanBlob(ctx context.Context, key, reason string) {
	if err := p.blob.Delete(ctx, key); err != nil {
		p.logger.Warn("orphaned blob cleanup failed",
			"key", key, "reason", reason, "error", err)
	}
}

// Blob returns the blob store backing uploaded documents. Used by HTTP
// handlers that need to stream content directly (download, range GET).
func (p *DocumentPipeline) Blob() storage.Blob {
	return p.blob
}

// WorkerTempDir returns the node-local scratch directory used for staging
// blob reads during extraction.
func (p *DocumentPipeline) WorkerTempDir() string {
	return p.workerTempDir
}

// ExtractorRegistry returns the content extractor registry.
func (p *DocumentPipeline) ExtractorRegistry() *extractor.Registry {
	return p.extractorRegistry
}

// Upload stores a file, creates a DB record with status "uploaded", and
// dispatches background jobs for extraction and indexing.
func (p *DocumentPipeline) Upload(ctx context.Context, params UploadDocumentParams) (*model.Document, error) {
	if err := validateTags(params.Tags); err != nil {
		return nil, err
	}
	if params.FileSize > p.maxUploadSize {
		return nil, fmt.Errorf("%w: %d bytes exceeds limit of %d", ErrFileTooLarge, params.FileSize, p.maxUploadSize)
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

	// Store file to the blob store. Key format is {fileType}/{uuid}{ext} —
	// a forward-slash key that the filesystem backend maps to a relative
	// path and the S3 backend uses as an object key.
	docUUID := uuid.New().String()
	key := path.Join(fileType, docUUID+ext)

	w, err := p.blob.NewWriter(ctx, key, &storage.WriterOpts{ContentType: mimeType})
	if err != nil {
		return nil, fmt.Errorf("opening blob writer for %s: %w", key, err)
	}

	written, err := io.Copy(w, io.LimitReader(params.Reader, p.maxUploadSize+1))
	if closeErr := w.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		p.deleteOrphanBlob(ctx, key, "upload write error")
		return nil, fmt.Errorf("writing uploaded blob: %w", err)
	}
	if written > p.maxUploadSize {
		p.deleteOrphanBlob(ctx, key, "upload size exceeded")
		return nil, fmt.Errorf("%w: actual bytes %d exceeds limit of %d", ErrFileTooLarge, written, p.maxUploadSize)
	}

	// Create DB record.
	doc := &model.Document{
		UUID:     docUUID,
		Title:    params.Title,
		FileType: fileType,
		FilePath: key,
		FileSize: written,
		MIMEType: mimeType,
		IsPublic: params.IsPublic,
		Status:   model.DocumentStatusUploaded,
		Description: sql.NullString{
			String: params.Description,
			Valid:  params.Description != "",
		},
	}
	if params.UserID != nil {
		doc.UserID = sql.NullInt64{Int64: *params.UserID, Valid: true}
	}

	if err = p.repo.Create(ctx, doc); err != nil {
		p.deleteOrphanBlob(ctx, key, "document record create failed")
		return nil, fmt.Errorf("creating document record: %w", err)
	}

	if len(params.Tags) > 0 {
		if err = p.repo.ReplaceTags(ctx, doc.ID, params.Tags); err != nil {
			return nil, fmt.Errorf("setting tags on uploaded document: %w", err)
		}
	}

	// Dispatch background extraction job.
	var userID int64
	if params.UserID != nil {
		userID = *params.UserID
	}
	if dispatchErr := p.dispatchExtraction(ctx, doc.ID, docUUID, userID); dispatchErr != nil {
		return nil, fmt.Errorf("uploading document: %w", dispatchErr)
	}

	created, err := p.repo.FindByID(ctx, doc.ID)
	if err != nil {
		return nil, fmt.Errorf("re-fetching uploaded document: %w", err)
	}
	return created, nil
}

// ReplaceContent replaces a document's file content without changing metadata
// (title, description, tags, visibility). The old file is removed from disk,
// a new file is written, and a background extraction job is dispatched.
func (p *DocumentPipeline) ReplaceContent(ctx context.Context, docUUID string, params ReplaceContentParams) (*model.Document, error) {
	doc, err := p.FindByUUID(ctx, docUUID)
	if err != nil {
		return nil, fmt.Errorf("finding document for content replacement: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(params.FileName))
	mimeType, ok := AllowedMIMETypes[ext]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedFileType, ext)
	}

	if params.FileSize > p.maxUploadSize {
		return nil, fmt.Errorf("%w: %d bytes exceeds limit of %d", ErrFileTooLarge, params.FileSize, p.maxUploadSize)
	}

	fileType := strings.TrimPrefix(ext, ".")
	if fileType == "htm" {
		fileType = "html"
	}
	if fileType == "txt" {
		fileType = "markdown"
	}

	// Remove old blob (best-effort). Blob.Delete is already idempotent on
	// missing keys, so there's no need to pre-check existence.
	if doc.FilePath != "" {
		if removeErr := p.blob.Delete(ctx, doc.FilePath); removeErr != nil {
			p.logger.Warn("removing old blob during content replacement",
				"key", doc.FilePath, "error", removeErr)
		}
	}

	// Write new blob. Key format matches Upload: {fileType}/{uuid}{ext}.
	key := path.Join(fileType, doc.UUID+ext)

	w, err := p.blob.NewWriter(ctx, key, &storage.WriterOpts{ContentType: mimeType})
	if err != nil {
		return nil, fmt.Errorf("opening blob writer for %s: %w", key, err)
	}

	written, err := io.Copy(w, io.LimitReader(params.Reader, p.maxUploadSize+1))
	if closeErr := w.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		p.deleteOrphanBlob(ctx, key, "replace write error")
		return nil, fmt.Errorf("writing replacement blob: %w", err)
	}
	if written > p.maxUploadSize {
		p.deleteOrphanBlob(ctx, key, "replace size exceeded")
		return nil, fmt.Errorf("%w: actual bytes %d exceeds limit of %d", ErrFileTooLarge, written, p.maxUploadSize)
	}

	// Reset document fields for re-processing. Clears metadata so the worker
	// repopulates from the re-extracted result.
	doc.FilePath = key
	doc.FileSize = written
	doc.FileType = fileType
	doc.MIMEType = mimeType
	doc.Status = model.DocumentStatusUploaded
	doc.Content = sql.NullString{}
	doc.ContentHash = sql.NullString{}
	doc.WordCount = sql.NullInt64{}
	doc.ProcessedAt = sql.NullTime{}
	doc.ErrorMessage = sql.NullString{}
	doc.Metadata = nil

	if err = p.repo.Update(ctx, doc); err != nil {
		return nil, fmt.Errorf("updating document after content replacement: %w", err)
	}

	if dispatchErr := p.dispatchExtraction(ctx, doc.ID, doc.UUID, doc.UserID.Int64); dispatchErr != nil {
		return nil, fmt.Errorf("replacing document content: %w", dispatchErr)
	}

	updated, err := p.repo.FindByID(ctx, doc.ID)
	if err != nil {
		return nil, fmt.Errorf("re-fetching document after content replacement: %w", err)
	}
	return updated, nil
}

// ProcessDocument extracts content from a document and updates its DB record.
// This is called by the background worker.
//
// Extractors (PDF, DOCX, XLSX) take a filesystem path rather than an
// io.Reader, so the blob is first staged to a node-local temp file under
// workerTempDir. The temp file is removed after extraction completes or
// fails — under normal operation it lives only for the duration of the job.
func (p *DocumentPipeline) ProcessDocument(ctx context.Context, docID int64) error {
	doc, err := p.repo.FindByID(ctx, docID)
	if err != nil {
		return fmt.Errorf("finding document %d for processing: %w", docID, err)
	}

	ext, err := p.extractorRegistry.ForMIMEType(doc.MIMEType)
	if err != nil {
		return p.markFailed(ctx, doc, fmt.Sprintf("no extractor for %s: %v", doc.MIMEType, err))
	}

	localPath, cleanup, err := p.stageBlobToTemp(ctx, doc.FilePath)
	if err != nil {
		return p.markFailed(ctx, doc, fmt.Sprintf("staging blob for extraction: %v", err))
	}
	defer cleanup()

	result, err := ext.Extract(ctx, localPath)
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
	doc.Status = model.DocumentStatusIndexed
	doc.ErrorMessage = sql.NullString{}

	// Persist extractor-emitted metadata (title, creator, subjects, page count,
	// sheet names, etc.) to the JSONB column. Non-fatal on marshal error:
	// extraction success must not depend on the metadata map round-tripping,
	// and json.Marshal on map[string]any can fail for exotic value types.
	if len(result.Metadata) > 0 {
		// Sanitize uploader-controlled string values (titles, creator,
		// description, EPUB subjects, etc.) before persisting them to the
		// JSONB column — see sanitizeMetadataMap for the threat model.
		if meta, marshalErr := json.Marshal(sanitizeMetadataMap(result.Metadata)); marshalErr == nil {
			doc.Metadata = meta
		} else {
			p.logger.Warn("marshaling extracted metadata",
				"doc_uuid", doc.UUID, "error", marshalErr)
			doc.Metadata = nil
		}
	} else {
		doc.Metadata = nil
	}

	if err := p.repo.Update(ctx, doc); err != nil {
		return fmt.Errorf("updating document %d after extraction: %w", docID, err)
	}

	p.logger.Info("document processed",
		"id", docID,
		"uuid", doc.UUID,
		"word_count", result.WordCount,
	)

	return nil
}

// dispatchExtraction enqueues a document extraction job via River.
func (p *DocumentPipeline) dispatchExtraction(ctx context.Context, docID int64, docUUID string, userID int64) error {
	if p.inserter == nil {
		return nil
	}

	if _, err := p.inserter.Insert(ctx, queue.DocumentExtractArgs{
		DocumentID: docID,
		DocUUID:    docUUID,
		UserID:     userID,
	}, nil); err != nil {
		return fmt.Errorf("dispatching extraction job for document %s: %w", docUUID, err)
	}
	return nil
}

// markFailed updates a document's status to "failed" with an error message.
func (p *DocumentPipeline) markFailed(ctx context.Context, doc *model.Document, errMsg string) error {
	doc.Status = model.DocumentStatusFailed
	doc.ErrorMessage = sql.NullString{String: errMsg, Valid: true}
	if err := p.repo.Update(ctx, doc); err != nil {
		return fmt.Errorf("marking document %d as failed: %w", doc.ID, err)
	}
	p.logger.Error("document processing failed", "id", doc.ID, "uuid", doc.UUID, "error", errMsg)
	return fmt.Errorf("document %d: %s", doc.ID, errMsg)
}

// stageBlobToTemp downloads the blob at key into a temp file under
// workerTempDir and returns the temp path plus a cleanup function. The
// cleanup is safe to defer and is a no-op if the temp file was never
// successfully created.
//
// Extractors that require a seekable filesystem file (PDF, DOCX, XLSX)
// cannot consume an io.Reader directly — this staging step bridges the gap
// between the blob store and those libraries.
func (p *DocumentPipeline) stageBlobToTemp(ctx context.Context, key string) (tmpPath string, cleanup func(), err error) {
	noop := func() {}

	if p.workerTempDir == "" {
		return "", noop, errors.New("worker temp dir is not configured")
	}

	// Preserve the extension so extractors that inspect the filename
	// (e.g. html/markdown fallbacks) keep working.
	tmp, err := os.CreateTemp(p.workerTempDir, "extract-*"+filepath.Ext(key))
	if err != nil {
		return "", noop, fmt.Errorf("creating temp file: %w", err)
	}
	removeTmp := func() { _ = os.Remove(tmp.Name()) }

	r, err := p.blob.NewReader(ctx, key)
	if err != nil {
		_ = tmp.Close()
		removeTmp()
		return "", noop, fmt.Errorf("opening blob reader for %s: %w", key, err)
	}
	defer func() { _ = r.Close() }()

	if _, err := io.Copy(tmp, r); err != nil {
		_ = tmp.Close()
		removeTmp()
		return "", noop, fmt.Errorf("staging blob %s: %w", key, err)
	}
	if err := tmp.Close(); err != nil {
		removeTmp()
		return "", noop, fmt.Errorf("closing temp file: %w", err)
	}
	return tmp.Name(), removeTmp, nil
}
