package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	authmiddleware "github.com/c-premus/documcp/internal/auth/middleware"
	"github.com/c-premus/documcp/internal/dto"
	"github.com/c-premus/documcp/internal/extractor"
	"github.com/c-premus/documcp/internal/model"
	"github.com/c-premus/documcp/internal/repository"
	"github.com/c-premus/documcp/internal/service"
	"github.com/c-premus/documcp/internal/storage"
)

// documentPipeline defines the pipeline methods used by DocumentHandler.
type documentPipeline interface {
	FindByUUID(ctx context.Context, uuid string) (*model.Document, error)
	Upload(ctx context.Context, params service.UploadDocumentParams) (*model.Document, error)
	ReplaceContent(ctx context.Context, docUUID string, params service.ReplaceContentParams) (*model.Document, error)
	Update(ctx context.Context, docUUID string, params service.UpdateDocumentParams) (*model.Document, error)
	Delete(ctx context.Context, docUUID string) error
	ExtractorRegistry() *extractor.Registry
}

// documentRepo defines the repository methods used by DocumentHandler.
type documentRepo interface {
	List(ctx context.Context, params repository.DocumentListParams) (*repository.DocumentListResult, error)
	FindByUUID(ctx context.Context, uuid string) (*model.Document, error)
	FindByUUIDIncludingDeleted(ctx context.Context, uuid string) (*model.Document, error)
	TagsForDocument(ctx context.Context, documentID int64) ([]model.DocumentTag, error)
	TagsForDocuments(ctx context.Context, documentIDs []int64) (map[int64][]model.DocumentTag, error)
	Restore(ctx context.Context, id int64) error
	PurgeSingle(ctx context.Context, id int64) (string, error)
	PurgeSoftDeleted(ctx context.Context, olderThan time.Duration) ([]repository.DocumentFilePath, error)
	ListDeleted(ctx context.Context, limit, offset int, userID *int64) ([]model.Document, int, error)
	ListDistinctTags(ctx context.Context, prefix string, limit int, userID *int64) ([]string, error)
}

const maxUploadBodySize = 50*1024*1024 + 1024 // 50 MiB + metadata overhead

// DocumentHandler handles REST API endpoints for documents.
type DocumentHandler struct {
	pipeline      documentPipeline
	repo          documentRepo
	blob          storage.Blob
	workerTempDir string
	logger        *slog.Logger
}

// NewDocumentHandler creates a new DocumentHandler. blob is the shared
// document blob store; workerTempDir is a node-local scratch directory
// used by the Analyze endpoint to stage uploaded files for extraction.
func NewDocumentHandler(
	pipeline documentPipeline,
	repo documentRepo,
	blob storage.Blob,
	workerTempDir string,
	logger *slog.Logger,
) *DocumentHandler {
	return &DocumentHandler{
		pipeline:      pipeline,
		repo:          repo,
		blob:          blob,
		workerTempDir: workerTempDir,
		logger:        logger,
	}
}

// documentResponse is the JSON representation of a document.
type documentResponse struct {
	UUID        string   `json:"uuid"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	FileType    string   `json:"file_type"`
	FileSize    int64    `json:"file_size"`
	MIMEType    string   `json:"mime_type"`
	WordCount   int64    `json:"word_count,omitempty"`
	IsPublic    bool     `json:"is_public"`
	HasFile     bool     `json:"has_file"`
	Status      string   `json:"status"`
	ContentHash string   `json:"content_hash,omitempty"`
	Content     string   `json:"content,omitempty"`
	Tags        []string `json:"tags"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
	ProcessedAt string   `json:"processed_at,omitempty"`
}

// List handles GET /api/documents — list documents with pagination and filters.
func (h *DocumentHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r, 50, 100)

	params := repository.DocumentListParams{
		FileType: r.URL.Query().Get("file_type"),
		Status:   model.DocumentStatus(r.URL.Query().Get("status")),
		Query:    r.URL.Query().Get("q"),
		Limit:    limit,
		Offset:   offset,
		OrderBy:  r.URL.Query().Get("sort"),
		OrderDir: r.URL.Query().Get("order"),
	}

	// Non-admin users can only see their own documents and public documents.
	user, _ := authmiddleware.UserFromContext(r.Context())
	if user != nil && !user.IsAdmin {
		params.OwnerOrPublic = &user.ID
	}

	result, err := h.repo.List(r.Context(), params)
	if err != nil {
		h.logger.Error("listing documents", "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to list documents")
		return
	}

	// Batch-load tags for all documents in a single query (avoids N+1).
	docIDs := make([]int64, len(result.Documents))
	for i := range result.Documents {
		docIDs[i] = result.Documents[i].ID
	}
	tagsByDoc, tagsErr := h.repo.TagsForDocuments(r.Context(), docIDs)
	if tagsErr != nil {
		h.logger.Warn("batch-loading tags", "error", tagsErr)
		tagsByDoc = map[int64][]model.DocumentTag{}
	}

	docs := make([]documentResponse, 0, len(result.Documents))
	for i := range result.Documents {
		doc := &result.Documents[i]
		docs = append(docs, toDocumentResponse(doc, tagsByDoc[doc.ID]))
	}

	jsonResponse(w, http.StatusOK, listResponse(docs, result.Total, params.Limit, params.Offset))
}

// Show handles GET /api/documents/{uuid} — get a single document.
func (h *DocumentHandler) Show(w http.ResponseWriter, r *http.Request) {
	docUUID := chi.URLParam(r, "uuid")

	doc, err := h.pipeline.FindByUUID(r.Context(), docUUID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			errorResponse(w, http.StatusNotFound, "document not found")
			return
		}
		h.logger.Error("finding document", "uuid", docUUID, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to find document")
		return
	}

	// Non-admin users can only see their own documents or public documents.
	user, _ := authmiddleware.UserFromContext(r.Context())
	if !canAccessDocument(user, doc) {
		errorResponse(w, http.StatusNotFound, "document not found")
		return
	}

	tags, _ := h.repo.TagsForDocument(r.Context(), doc.ID)
	resp := toDocumentResponse(doc, tags)
	if r.URL.Query().Get("include_content") == "true" {
		resp = toDocumentResponseWithContent(doc, tags)
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"data": resp,
	})
}

// Upload handles POST /api/documents — upload a new document.
func (h *DocumentHandler) Upload(w http.ResponseWriter, r *http.Request) {
	// Limit request body size.
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBodySize)

	if err := r.ParseMultipartForm(10 * 1024 * 1024); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		errorResponse(w, http.StatusBadRequest, "file is required")
		return
	}
	defer func() { _ = file.Close() }()

	title := r.FormValue("title")
	if title == "" {
		title = strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename))
		if title == "" {
			title = header.Filename
		}
	}

	var tags []string
	if tagsStr := r.FormValue("tags"); tagsStr != "" {
		tags = strings.Split(tagsStr, ",")
		for i := range tags {
			tags[i] = strings.TrimSpace(tags[i])
		}
	}

	isPublic := r.FormValue("is_public") == "true" || r.FormValue("is_public") == "1"

	params := service.UploadDocumentParams{
		Title:       title,
		Description: r.FormValue("description"),
		FileName:    header.Filename,
		FileSize:    header.Size,
		Reader:      file,
		IsPublic:    isPublic,
		Tags:        tags,
	}

	// Set the owner from the authenticated user context.
	if uploadUser, ok := authmiddleware.UserFromContext(r.Context()); ok {
		params.UserID = &uploadUser.ID
	}

	doc, err := h.pipeline.Upload(r.Context(), params)
	if err != nil {
		h.logger.Error("uploading document", "error", err)
		switch {
		case errors.Is(err, service.ErrUnsupportedFileType):
			errorResponse(w, http.StatusBadRequest, "unsupported file type")
		case errors.Is(err, service.ErrFileTooLarge):
			errorResponse(w, http.StatusBadRequest, "file exceeds maximum upload size")
		case errors.Is(err, service.ErrTagValidation):
			errorResponse(w, http.StatusBadRequest, err.Error())
		default:
			errorResponse(w, http.StatusInternalServerError, "failed to process document upload")
		}
		return
	}

	tags2, _ := h.repo.TagsForDocument(r.Context(), doc.ID)
	jsonResponse(w, http.StatusCreated, map[string]any{
		"data":    toDocumentResponse(doc, tags2),
		"message": "Document uploaded and queued for processing.",
	})
}

// ReplaceContent handles POST /api/documents/{uuid}/content — replace document file content.
func (h *DocumentHandler) ReplaceContent(w http.ResponseWriter, r *http.Request) {
	docUUID := chi.URLParam(r, "uuid")

	if !h.checkOwnership(r, docUUID) {
		errorResponse(w, http.StatusNotFound, "document not found")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBodySize)

	if err := r.ParseMultipartForm(10 * 1024 * 1024); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		errorResponse(w, http.StatusBadRequest, "file is required")
		return
	}
	defer func() { _ = file.Close() }()

	params := service.ReplaceContentParams{
		FileName: header.Filename,
		FileSize: header.Size,
		Reader:   file,
	}

	if user, ok := authmiddleware.UserFromContext(r.Context()); ok {
		params.UserID = &user.ID
	}

	doc, err := h.pipeline.ReplaceContent(r.Context(), docUUID, params)
	if err != nil {
		h.logger.Error("replacing document content", "uuid", docUUID, "error", err)
		if errors.Is(err, service.ErrNotFound) {
			errorResponse(w, http.StatusNotFound, "document not found")
			return
		}
		if errors.Is(err, service.ErrUnsupportedFileType) {
			errorResponse(w, http.StatusBadRequest, "unsupported file type")
			return
		}
		if errors.Is(err, service.ErrFileTooLarge) {
			errorResponse(w, http.StatusBadRequest, "file exceeds maximum upload size")
			return
		}
		errorResponse(w, http.StatusInternalServerError, "failed to replace document content")
		return
	}

	tags, _ := h.repo.TagsForDocument(r.Context(), doc.ID)
	jsonResponse(w, http.StatusOK, map[string]any{
		"data":    toDocumentResponse(doc, tags),
		"message": "Document content replaced and queued for processing.",
	})
}

// checkOwnership verifies the requesting user has access to the document.
// Admins can access any document. Non-admin users can only access their own.
func (h *DocumentHandler) checkOwnership(r *http.Request, docUUID string) bool {
	user, ok := authmiddleware.UserFromContext(r.Context())
	if !ok || user == nil {
		return false
	}
	if user.IsAdmin {
		return true
	}
	doc, err := h.repo.FindByUUIDIncludingDeleted(r.Context(), docUUID)
	if err != nil {
		return false
	}
	return doc.UserID.Valid && doc.UserID.Int64 == user.ID
}

// Update handles PUT /api/documents/{uuid} — update document metadata.
func (h *DocumentHandler) Update(w http.ResponseWriter, r *http.Request) {
	docUUID := chi.URLParam(r, "uuid")

	if !h.checkOwnership(r, docUUID) {
		errorResponse(w, http.StatusNotFound, "document not found")
		return
	}

	var body struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		IsPublic    *bool    `json:"is_public"`
		Tags        []string `json:"tags"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	doc, err := h.pipeline.Update(r.Context(), docUUID, service.UpdateDocumentParams{
		Title:       body.Title,
		Description: body.Description,
		IsPublic:    body.IsPublic,
		Tags:        body.Tags,
	})
	if err != nil {
		h.logger.Error("updating document", "uuid", docUUID, "error", err)
		switch {
		case errors.Is(err, service.ErrNotFound):
			errorResponse(w, http.StatusNotFound, "document not found")
		case errors.Is(err, service.ErrTagValidation):
			errorResponse(w, http.StatusBadRequest, err.Error())
		default:
			errorResponse(w, http.StatusInternalServerError, "failed to update document")
		}
		return
	}

	tags, _ := h.repo.TagsForDocument(r.Context(), doc.ID)
	jsonResponse(w, http.StatusOK, map[string]any{
		"data":    toDocumentResponse(doc, tags),
		"message": "Document updated successfully.",
	})
}

// Delete handles DELETE /api/documents/{uuid} — soft delete a document.
func (h *DocumentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	docUUID := chi.URLParam(r, "uuid")

	if !h.checkOwnership(r, docUUID) {
		errorResponse(w, http.StatusNotFound, "document not found")
		return
	}

	if err := h.pipeline.Delete(r.Context(), docUUID); err != nil {
		h.logger.Error("deleting document", "uuid", docUUID, "error", err)
		if errors.Is(err, service.ErrNotFound) {
			errorResponse(w, http.StatusNotFound, "document not found")
			return
		}
		errorResponse(w, http.StatusInternalServerError, "failed to delete document")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"message": "Document deleted successfully.",
	})
}

// toDocumentResponse converts a model.Document and its tags to a response DTO.
// The Content field is always omitted — it can be large. Use
// toDocumentResponseWithContent to include it.
func toDocumentResponse(doc *model.Document, tags []model.DocumentTag) documentResponse {
	return documentResponse{
		UUID:        doc.UUID,
		Title:       doc.Title,
		Description: nullStringValue(doc.Description),
		FileType:    doc.FileType,
		FileSize:    doc.FileSize,
		MIMEType:    doc.MIMEType,
		WordCount:   nullInt64Value(doc.WordCount),
		IsPublic:    doc.IsPublic,
		HasFile:     doc.FilePath != "",
		Status:      string(doc.Status),
		ContentHash: nullStringValue(doc.ContentHash),
		Tags:        dto.TagNames(tags),
		CreatedAt:   dto.FormatNullTime(doc.CreatedAt),
		UpdatedAt:   dto.FormatNullTime(doc.UpdatedAt),
		ProcessedAt: dto.FormatNullTime(doc.ProcessedAt),
	}
}

// toDocumentResponseWithContent is toDocumentResponse with the Content field
// populated when the document has body content available.
func toDocumentResponseWithContent(doc *model.Document, tags []model.DocumentTag) documentResponse {
	resp := toDocumentResponse(doc, tags)
	if doc.Content.Valid {
		resp.Content = doc.Content.String
	}
	return resp
}

// Download handles GET /api/documents/{uuid}/download — serve the document file.
func (h *DocumentHandler) Download(w http.ResponseWriter, r *http.Request) {
	docUUID := chi.URLParam(r, "uuid")

	doc, err := h.repo.FindByUUID(r.Context(), docUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			errorResponse(w, http.StatusNotFound, "document not found")
			return
		}
		h.logger.Error("finding document for download", "uuid", docUUID, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to find document")
		return
	}

	// Check access: public, admin, or owner.
	user, _ := authmiddleware.UserFromContext(r.Context())
	if !canAccessDocument(user, doc) {
		errorResponse(w, http.StatusNotFound, "document not found")
		return
	}

	if doc.FilePath == "" {
		// No file on disk — serve content from DB if available (e.g. markdown/html created via API).
		if !doc.Content.Valid || doc.Content.String == "" {
			errorResponse(w, http.StatusNotFound, "document has no associated file")
			return
		}
		filename := sanitizeFilename(doc.Title) + sanitizeExtension("."+doc.FileType)
		w.Header().Set("Content-Type", doc.MIMEType)
		w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
		w.Header().Set("Content-Length", strconv.Itoa(len(doc.Content.String)))
		_, _ = w.Write([]byte(doc.Content.String))
		return
	}

	// Determine filename for Content-Disposition, sanitizing special characters.
	ext := sanitizeExtension(filepath.Ext(doc.FilePath))
	filename := doc.UUID + ext
	if doc.Title != "" {
		filename = sanitizeFilename(doc.Title) + ext
	}

	contentType := doc.MIMEType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	if err := serveBlob(w, r, h.blob, doc.FilePath, filename, contentType); err != nil {
		h.logger.Error("serving document blob",
			"uuid", doc.UUID, "key", doc.FilePath, "error", err)
		// Headers may already be sent; no point writing another error response.
	}
}

// ListTags handles GET /api/documents/tags — autocomplete tags.
func (h *DocumentHandler) ListTags(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	// Scope tags by user visibility: admins see all, non-admins see only
	// tags from public documents or their own.
	var tagUserID *int64
	if user, ok := authmiddleware.UserFromContext(r.Context()); ok && !user.IsAdmin {
		tagUserID = &user.ID
	}

	tags, err := h.repo.ListDistinctTags(r.Context(), q, limit, tagUserID)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "failed to list tags", "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to list tags")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{"data": tags})
}

// canAccessDocument returns true if the user may view the document.
// Access is granted when the document is public, the user is an admin,
// or the user owns the document.
func canAccessDocument(user *model.User, doc *model.Document) bool {
	if doc.IsPublic {
		return true
	}
	if user == nil {
		return false
	}
	return user.IsAdmin || (doc.UserID.Valid && doc.UserID.Int64 == user.ID)
}

// sanitizeFilename removes characters unsafe for Content-Disposition headers
// (quotes, backslashes, control characters including DEL, path separators).
func sanitizeFilename(name string) string {
	return strings.Map(func(r rune) rune {
		if r == '"' || r == '\\' || r < 32 || r == 127 || r == '/' || r == ':' || r == ';' {
			return '_'
		}
		return r
	}, name)
}

// sanitizeExtension validates that a file extension contains only safe characters
// (letters, digits, and the leading dot). Returns empty string if unsafe.
func sanitizeExtension(ext string) string {
	if ext == "" {
		return ""
	}
	for i, r := range ext {
		if i == 0 && r == '.' {
			continue
		}
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return ""
		}
	}
	return ext
}
