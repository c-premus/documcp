package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/c-premus/documcp/internal/model"
)

// DocumentRepo defines the repository methods the document service needs.
type DocumentRepo interface {
	FindByUUID(ctx context.Context, uuid string) (*model.Document, error)
	FindByID(ctx context.Context, id int64) (*model.Document, error)
	Create(ctx context.Context, doc *model.Document) error
	Update(ctx context.Context, doc *model.Document) error
	SoftDelete(ctx context.Context, id int64) error
	TagsForDocument(ctx context.Context, documentID int64) ([]model.DocumentTag, error)
	ReplaceTags(ctx context.Context, documentID int64, tags []string) error
	CreateVersion(ctx context.Context, version *model.DocumentVersion) error
}

// CreateDocumentParams holds the input for creating a document.
type CreateDocumentParams struct {
	Title       string
	Content     string
	FileType    string
	Description string
	IsPublic    bool
	Tags        []string
	UserID      *int64
}

// UpdateDocumentParams holds the input for updating a document.
type UpdateDocumentParams struct {
	Title       string
	Description string
	IsPublic    *bool
	Tags        []string
}

// DocumentService orchestrates document CRUD operations.
type DocumentService struct {
	repo   DocumentRepo
	logger *slog.Logger
}

// NewDocumentService creates a new DocumentService.
func NewDocumentService(repo DocumentRepo, logger *slog.Logger) *DocumentService {
	return &DocumentService{repo: repo, logger: logger}
}

// FindByUUID retrieves a document by its UUID, including its tags.
// Returns ErrNotFound when the document does not exist.
func (s *DocumentService) FindByUUID(ctx context.Context, docUUID string) (*model.Document, error) {
	doc, err := s.repo.FindByUUID(ctx, docUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("finding document by uuid: %w", err)
	}
	return doc, nil
}

// Create creates a new document with computed defaults and optional tags.
func (s *DocumentService) Create(ctx context.Context, params CreateDocumentParams) (*model.Document, error) {
	hash := sha256.Sum256([]byte(params.Content))
	contentHash := hex.EncodeToString(hash[:])
	wordCount := int64(len(strings.Fields(params.Content)))
	mimeType := mimeTypeForFileType(params.FileType)
	now := time.Now()

	doc := &model.Document{
		UUID:        uuid.New().String(),
		Title:       params.Title,
		FileType:    params.FileType,
		FilePath:    "",
		FileSize:    int64(len(params.Content)),
		MIMEType:    mimeType,
		Content:     sql.NullString{String: params.Content, Valid: true},
		ContentHash: sql.NullString{String: contentHash, Valid: true},
		WordCount:   sql.NullInt64{Int64: wordCount, Valid: true},
		ProcessedAt: sql.NullTime{Time: now, Valid: true},
		IsPublic:    params.IsPublic,
		Status:      "processed",
		Description: sql.NullString{String: params.Description, Valid: params.Description != ""},
	}

	if params.UserID != nil {
		doc.UserID = sql.NullInt64{Int64: *params.UserID, Valid: true}
	}

	if err := s.repo.Create(ctx, doc); err != nil {
		return nil, fmt.Errorf("creating document: %w", err)
	}

	if len(params.Tags) > 0 {
		if err := s.repo.ReplaceTags(ctx, doc.ID, params.Tags); err != nil {
			return nil, fmt.Errorf("setting tags on new document: %w", err)
		}
	}

	created, err := s.repo.FindByID(ctx, doc.ID)
	if err != nil {
		return nil, fmt.Errorf("re-fetching created document: %w", err)
	}

	return created, nil
}

// Update applies partial updates to an existing document identified by UUID.
func (s *DocumentService) Update(ctx context.Context, docUUID string, params UpdateDocumentParams) (*model.Document, error) {
	doc, err := s.FindByUUID(ctx, docUUID)
	if err != nil {
		return nil, fmt.Errorf("finding document for update: %w", err)
	}

	if params.Title != "" {
		doc.Title = params.Title
	}
	if params.Description != "" {
		doc.Description = sql.NullString{String: params.Description, Valid: true}
	}
	if params.IsPublic != nil {
		doc.IsPublic = *params.IsPublic
	}

	if err = s.repo.Update(ctx, doc); err != nil {
		return nil, fmt.Errorf("updating document: %w", err)
	}

	if params.Tags != nil {
		if err = s.repo.ReplaceTags(ctx, doc.ID, params.Tags); err != nil {
			return nil, fmt.Errorf("replacing tags on document: %w", err)
		}
	}

	updated, err := s.repo.FindByID(ctx, doc.ID)
	if err != nil {
		return nil, fmt.Errorf("re-fetching updated document: %w", err)
	}

	return updated, nil
}

// Delete soft-deletes a document identified by UUID.
func (s *DocumentService) Delete(ctx context.Context, docUUID string) error {
	doc, err := s.FindByUUID(ctx, docUUID)
	if err != nil {
		return fmt.Errorf("finding document for deletion: %w", err)
	}

	if err := s.repo.SoftDelete(ctx, doc.ID); err != nil {
		return fmt.Errorf("soft deleting document: %w", err)
	}

	return nil
}

// TagsForDocument returns all tags associated with a document.
func (s *DocumentService) TagsForDocument(ctx context.Context, documentID int64) ([]model.DocumentTag, error) {
	return s.repo.TagsForDocument(ctx, documentID)
}

// mimeTypeForFileType maps a file type string to its MIME type.
func mimeTypeForFileType(fileType string) string {
	switch strings.ToLower(fileType) {
	case "markdown", "md", "txt":
		return "text/markdown"
	case "html", "htm":
		return "text/html"
	case "pdf":
		return "application/pdf"
	case "docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case "xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	default:
		return "application/octet-stream"
	}
}
