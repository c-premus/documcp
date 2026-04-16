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
	"github.com/c-premus/documcp/internal/repository"
)

// DocumentRepo defines the repository methods the document service needs.
type DocumentRepo interface {
	FindByUUID(ctx context.Context, uuid string) (*model.Document, error)
	FindByUUIDIncludingDeleted(ctx context.Context, uuid string) (*model.Document, error)
	FindByID(ctx context.Context, id int64) (*model.Document, error)
	Create(ctx context.Context, doc *model.Document) error
	Update(ctx context.Context, doc *model.Document) error
	SoftDelete(ctx context.Context, id int64) error
	Restore(ctx context.Context, id int64) error
	PurgeSingle(ctx context.Context, id int64) (string, error)
	PurgeSoftDeleted(ctx context.Context, olderThan time.Duration) ([]repository.DocumentFilePath, error)
	List(ctx context.Context, params repository.DocumentListParams) (*repository.DocumentListResult, error)
	ListDeleted(ctx context.Context, limit, offset int, userID *int64) ([]model.Document, int, error)
	ListDistinctTags(ctx context.Context, prefix string, limit int, userID *int64) ([]string, error)
	TagsForDocument(ctx context.Context, documentID int64) ([]model.DocumentTag, error)
	TagsForDocuments(ctx context.Context, documentIDs []int64) (map[int64][]model.DocumentTag, error)
	ReplaceTags(ctx context.Context, documentID int64, tags []string) error
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
	if err := validateTags(params.Tags); err != nil {
		return nil, err
	}

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
		Status:      model.DocumentStatusIndexed,
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
	if err := validateTags(params.Tags); err != nil {
		return nil, err
	}

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

// TagsForDocuments batch-loads tags for the given document IDs.
func (s *DocumentService) TagsForDocuments(ctx context.Context, documentIDs []int64) (map[int64][]model.DocumentTag, error) {
	return s.repo.TagsForDocuments(ctx, documentIDs)
}

// List returns a paginated slice of documents matching params. Visibility
// filtering (OwnerOrPublic) is the caller's responsibility — set on params.
func (s *DocumentService) List(ctx context.Context, params repository.DocumentListParams) (*repository.DocumentListResult, error) {
	return s.repo.List(ctx, params)
}

// ListDeleted returns a paginated slice of soft-deleted documents. When
// userID is non-nil, only documents owned by that user are returned.
func (s *DocumentService) ListDeleted(ctx context.Context, limit, offset int, userID *int64) ([]model.Document, int, error) {
	return s.repo.ListDeleted(ctx, limit, offset, userID)
}

// ListDistinctTags returns distinct tag strings matching the prefix, scoped
// to the user's visible documents when userID is non-nil (non-admin caller).
func (s *DocumentService) ListDistinctTags(ctx context.Context, prefix string, limit int, userID *int64) ([]string, error) {
	return s.repo.ListDistinctTags(ctx, prefix, limit, userID)
}

// FindByUUIDIncludingDeleted returns a document by UUID, including
// soft-deleted rows. Returns ErrNotFound when the UUID does not exist.
// Used by ownership checks on trash operations and by restore/purge.
func (s *DocumentService) FindByUUIDIncludingDeleted(ctx context.Context, docUUID string) (*model.Document, error) {
	doc, err := s.repo.FindByUUIDIncludingDeleted(ctx, docUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("finding document (including deleted) by uuid: %w", err)
	}
	return doc, nil
}

// Restore reinstates a soft-deleted document identified by UUID and returns
// the refreshed row. Returns ErrNotFound when the document does not exist,
// ErrNotDeleted when the document is not soft-deleted.
func (s *DocumentService) Restore(ctx context.Context, docUUID string) (*model.Document, error) {
	doc, err := s.FindByUUIDIncludingDeleted(ctx, docUUID)
	if err != nil {
		return nil, fmt.Errorf("finding document for restore: %w", err)
	}
	if !doc.DeletedAt.Valid {
		return nil, ErrNotDeleted
	}
	if err = s.repo.Restore(ctx, doc.ID); err != nil {
		return nil, fmt.Errorf("restoring document: %w", err)
	}
	restored, err := s.repo.FindByUUID(ctx, docUUID)
	if err != nil {
		return nil, fmt.Errorf("re-fetching restored document: %w", err)
	}
	return restored, nil
}

// PurgeSingle permanently deletes a soft-deleted document and returns the
// stored file path (empty when the document had no blob). Callers are
// responsible for deleting the backing blob after a successful purge.
func (s *DocumentService) PurgeSingle(ctx context.Context, docUUID string) (string, error) {
	doc, err := s.FindByUUIDIncludingDeleted(ctx, docUUID)
	if err != nil {
		return "", fmt.Errorf("finding document for purge: %w", err)
	}
	filePath, err := s.repo.PurgeSingle(ctx, doc.ID)
	if err != nil {
		return "", fmt.Errorf("purging document: %w", err)
	}
	return filePath, nil
}

// PurgeSoftDeleted permanently deletes all soft-deleted documents whose
// deleted_at is older than olderThan. Returns the file paths for callers
// to clean up the corresponding blobs.
func (s *DocumentService) PurgeSoftDeleted(ctx context.Context, olderThan time.Duration) ([]repository.DocumentFilePath, error) {
	return s.repo.PurgeSoftDeleted(ctx, olderThan)
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
