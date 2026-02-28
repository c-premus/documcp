package api

import (
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

	"git.999.haus/chris/DocuMCP-go/internal/model"
	"git.999.haus/chris/DocuMCP-go/internal/repository"
	"git.999.haus/chris/DocuMCP-go/internal/service"
)

// DocumentHandler handles REST API endpoints for documents.
type DocumentHandler struct {
	pipeline *service.DocumentPipeline
	repo     *repository.DocumentRepository
	logger   *slog.Logger
}

// NewDocumentHandler creates a new DocumentHandler.
func NewDocumentHandler(
	pipeline *service.DocumentPipeline,
	repo *repository.DocumentRepository,
	logger *slog.Logger,
) *DocumentHandler {
	return &DocumentHandler{
		pipeline: pipeline,
		repo:     repo,
		logger:   logger,
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
	Status      string   `json:"status"`
	ContentHash string   `json:"content_hash,omitempty"`
	Tags        []string `json:"tags"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
	ProcessedAt string   `json:"processed_at,omitempty"`
}

// List handles GET /api/documents — list documents with pagination and filters.
func (h *DocumentHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	params := repository.DocumentListParams{
		FileType: r.URL.Query().Get("file_type"),
		Status:   r.URL.Query().Get("status"),
		Query:    r.URL.Query().Get("q"),
		Limit:    limit,
		Offset:   offset,
		OrderBy:  r.URL.Query().Get("sort"),
		OrderDir: r.URL.Query().Get("order"),
	}

	result, err := h.repo.List(r.Context(), params)
	if err != nil {
		h.logger.Error("listing documents", "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to list documents")
		return
	}

	docs := make([]documentResponse, 0, len(result.Documents))
	for i := range result.Documents {
		doc := &result.Documents[i]
		tags, _ := h.repo.TagsForDocument(r.Context(), doc.ID)
		docs = append(docs, toDocumentResponse(doc, tags))
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"data":   docs,
		"total":  result.Total,
		"limit":  params.Limit,
		"offset": params.Offset,
	})
}

// Show handles GET /api/documents/{uuid} — get a single document.
func (h *DocumentHandler) Show(w http.ResponseWriter, r *http.Request) {
	docUUID := chi.URLParam(r, "uuid")

	doc, err := h.pipeline.FindByUUID(r.Context(), docUUID)
	if err != nil {
		h.logger.Error("finding document", "uuid", docUUID, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to find document")
		return
	}
	if doc == nil {
		errorResponse(w, http.StatusNotFound, "document not found")
		return
	}

	tags, _ := h.repo.TagsForDocument(r.Context(), doc.ID)
	jsonResponse(w, http.StatusOK, map[string]any{
		"data": toDocumentResponse(doc, tags),
	})
}

// Upload handles POST /api/documents — upload a new document.
func (h *DocumentHandler) Upload(w http.ResponseWriter, r *http.Request) {
	// Limit request body size.
	r.Body = http.MaxBytesReader(w, r.Body, 50*1024*1024+1024) // 50MB + metadata overhead

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

	doc, err := h.pipeline.Upload(r.Context(), params)
	if err != nil {
		h.logger.Error("uploading document", "error", err)
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrUnsupportedFileType) || errors.Is(err, service.ErrFileTooLarge) {
			status = http.StatusBadRequest
		}
		errorResponse(w, status, err.Error())
		return
	}

	tags2, _ := h.repo.TagsForDocument(r.Context(), doc.ID)
	jsonResponse(w, http.StatusCreated, map[string]any{
		"data":    toDocumentResponse(doc, tags2),
		"message": "Document uploaded and queued for processing.",
	})
}

// Update handles PUT /api/documents/{uuid} — update document metadata.
func (h *DocumentHandler) Update(w http.ResponseWriter, r *http.Request) {
	docUUID := chi.URLParam(r, "uuid")

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
		if errors.Is(err, service.ErrNotFound) {
			errorResponse(w, http.StatusNotFound, "document not found")
			return
		}
		errorResponse(w, http.StatusInternalServerError, "failed to update document")
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
func toDocumentResponse(doc *model.Document, tags []model.DocumentTag) documentResponse {
	tagNames := make([]string, len(tags))
	for i, t := range tags {
		tagNames[i] = t.Tag
	}

	return documentResponse{
		UUID:        doc.UUID,
		Title:       doc.Title,
		Description: doc.Description.String,
		FileType:    doc.FileType,
		FileSize:    doc.FileSize,
		MIMEType:    doc.MIMEType,
		WordCount:   doc.WordCount.Int64,
		IsPublic:    doc.IsPublic,
		Status:      doc.Status,
		ContentHash: doc.ContentHash.String,
		Tags:        tagNames,
		CreatedAt:   formatTime(doc.CreatedAt),
		UpdatedAt:   formatTime(doc.UpdatedAt),
		ProcessedAt: formatTime(doc.ProcessedAt),
	}
}

func formatTime(t sql.NullTime) string {
	if !t.Valid {
		return ""
	}
	return t.Time.Format(time.RFC3339)
}
