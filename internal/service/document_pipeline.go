// Package service implements business logic and orchestration.
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

	"github.com/c-premus/documcp/internal/extractor"
	"github.com/c-premus/documcp/internal/model"
	"github.com/c-premus/documcp/internal/queue"
	"github.com/c-premus/documcp/internal/security"
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
type DocumentPipeline struct {
	*DocumentService
	extractorRegistry *extractor.Registry
	inserter          JobInserter
	storagePath       string
	maxUploadSize     int64
}

// NewDocumentPipeline creates a DocumentPipeline.
func NewDocumentPipeline(
	svc *DocumentService,
	registry *extractor.Registry,
	inserter JobInserter,
	storagePath string,
	maxUploadSize int64,
) *DocumentPipeline {
	if maxUploadSize <= 0 {
		maxUploadSize = defaultMaxUploadSize
	}
	return &DocumentPipeline{
		DocumentService:   svc,
		extractorRegistry: registry,
		inserter:          inserter,
		storagePath:       storagePath,
		maxUploadSize:     maxUploadSize,
	}
}

// Delete soft-deletes a document by UUID.
func (p *DocumentPipeline) Delete(ctx context.Context, docUUID string) error {
	return p.DocumentService.Delete(ctx, docUUID)
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

	// Store file to disk.
	docUUID := uuid.New().String()
	relPath := filepath.Join(fileType, docUUID+ext)
	absPath := filepath.Join(p.storagePath, relPath)

	if err := os.MkdirAll(filepath.Dir(absPath), 0o750); err != nil {
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
		_ = os.Remove(absPath)
		return nil, fmt.Errorf("creating document record: %w", err)
	}

	if len(params.Tags) > 0 {
		if err = p.repo.ReplaceTags(ctx, doc.ID, params.Tags); err != nil {
			return nil, fmt.Errorf("setting tags on uploaded document: %w", err)
		}
	}

	// Dispatch background extraction job.
	if dispatchErr := p.dispatchExtraction(ctx, doc.ID, docUUID); dispatchErr != nil {
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

	// Remove old file from disk (best-effort).
	if doc.FilePath != "" {
		oldPath, pathErr := security.SafeStoragePath(p.storagePath, doc.FilePath)
		if pathErr != nil {
			p.logger.Warn("unsafe old file path during content replacement",
				"file_path", doc.FilePath, "error", pathErr)
		} else if removeErr := os.Remove(oldPath); removeErr != nil && !os.IsNotExist(removeErr) {
			p.logger.Warn("removing old file during content replacement",
				"path", oldPath, "error", removeErr)
		}
	}

	// Write new file to disk.
	relPath := filepath.Join(fileType, doc.UUID+ext)
	fullPath := filepath.Join(p.storagePath, relPath)

	if err = os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
		return nil, fmt.Errorf("creating storage directory: %w", err)
	}

	f, err := os.Create(fullPath)
	if err != nil {
		return nil, fmt.Errorf("creating file %s: %w", fullPath, err)
	}

	written, err := io.Copy(f, params.Reader)
	if closeErr := f.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(fullPath)
		return nil, fmt.Errorf("writing replacement file: %w", err)
	}

	// Reset document fields for re-processing.
	doc.FilePath = relPath
	doc.FileSize = written
	doc.FileType = fileType
	doc.MIMEType = mimeType
	doc.Status = model.DocumentStatusUploaded
	doc.Content = sql.NullString{}
	doc.ContentHash = sql.NullString{}
	doc.WordCount = sql.NullInt64{}
	doc.ProcessedAt = sql.NullTime{}
	doc.ErrorMessage = sql.NullString{}

	if err = p.repo.Update(ctx, doc); err != nil {
		return nil, fmt.Errorf("updating document after content replacement: %w", err)
	}

	if dispatchErr := p.dispatchExtraction(ctx, doc.ID, doc.UUID); dispatchErr != nil {
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
func (p *DocumentPipeline) ProcessDocument(ctx context.Context, docID int64) error {
	doc, err := p.repo.FindByID(ctx, docID)
	if err != nil {
		return fmt.Errorf("finding document %d for processing: %w", docID, err)
	}

	absPath, err := security.SafeStoragePath(p.storagePath, doc.FilePath)
	if err != nil {
		return p.markFailed(ctx, doc, fmt.Sprintf("unsafe file path: %v", err))
	}

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
	doc.Status = model.DocumentStatusIndexed
	doc.ErrorMessage = sql.NullString{}

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
func (p *DocumentPipeline) dispatchExtraction(ctx context.Context, docID int64, docUUID string) error {
	if p.inserter == nil {
		return nil
	}

	if _, err := p.inserter.Insert(ctx, queue.DocumentExtractArgs{
		DocumentID: docID,
		DocUUID:    docUUID,
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
