package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	authmiddleware "git.999.haus/chris/DocuMCP-go/internal/auth/middleware"
	"git.999.haus/chris/DocuMCP-go/internal/extractor"
	"git.999.haus/chris/DocuMCP-go/internal/model"
	"git.999.haus/chris/DocuMCP-go/internal/repository"
	"git.999.haus/chris/DocuMCP-go/internal/service"
)

// documentPipeline defines the pipeline methods used by DocumentHandler.
type documentPipeline interface {
	FindByUUID(ctx context.Context, uuid string) (*model.Document, error)
	Upload(ctx context.Context, params service.UploadDocumentParams) (*model.Document, error)
	Update(ctx context.Context, docUUID string, params service.UpdateDocumentParams) (*model.Document, error)
	Delete(ctx context.Context, docUUID string) error
	StoragePath() string
	ExtractorRegistry() *extractor.Registry
}

// documentRepo defines the repository methods used by DocumentHandler.
type documentRepo interface {
	List(ctx context.Context, params repository.DocumentListParams) (*repository.DocumentListResult, error)
	FindByUUID(ctx context.Context, uuid string) (*model.Document, error)
	FindByUUIDIncludingDeleted(ctx context.Context, uuid string) (*model.Document, error)
	TagsForDocument(ctx context.Context, documentID int64) ([]model.DocumentTag, error)
	Restore(ctx context.Context, id int64) error
	PurgeSingle(ctx context.Context, id int64) (string, error)
	PurgeSoftDeleted(ctx context.Context, olderThan time.Duration) ([]repository.DocumentFilePath, error)
	ListDeleted(ctx context.Context, limit, offset int) ([]model.Document, int, error)
}

// DocumentHandler handles REST API endpoints for documents.
type DocumentHandler struct {
	pipeline documentPipeline
	repo     documentRepo
	logger   *slog.Logger
}

// NewDocumentHandler creates a new DocumentHandler.
func NewDocumentHandler(
	pipeline documentPipeline,
	repo documentRepo,
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
		tags, tagsErr := h.repo.TagsForDocument(r.Context(), doc.ID)
		if tagsErr != nil {
			h.logger.Warn("loading tags for document", "doc_id", doc.ID, "error", tagsErr)
		}
		docs = append(docs, toDocumentResponse(doc, tags))
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"data": docs,
		"meta": map[string]any{
			"total":  result.Total,
			"limit":  params.Limit,
			"offset": params.Offset,
		},
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
		if errors.Is(err, service.ErrUnsupportedFileType) || errors.Is(err, service.ErrFileTooLarge) {
			errorResponse(w, http.StatusBadRequest, err.Error())
		} else {
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

	// Check access: document must be public or owned by the authenticated user.
	if !doc.IsPublic {
		user, ok := authmiddleware.UserFromContext(r.Context())
		if !ok || !doc.UserID.Valid || user.ID != doc.UserID.Int64 {
			errorResponse(w, http.StatusForbidden, "access denied")
			return
		}
	}

	if doc.FilePath == "" {
		errorResponse(w, http.StatusNotFound, "document has no associated file")
		return
	}

	fullPath := filepath.Join(h.pipeline.StoragePath(), doc.FilePath)

	f, err := os.Open(fullPath)
	if err != nil {
		h.logger.Error("opening document file", "path", fullPath, "error", err)
		errorResponse(w, http.StatusNotFound, "file not found")
		return
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		h.logger.Error("stat document file", "path", fullPath, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to read file info")
		return
	}

	contentType := doc.MIMEType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Determine filename for Content-Disposition, sanitizing special characters.
	filename := doc.UUID + filepath.Ext(doc.FilePath)
	if doc.Title != "" {
		filename = sanitizeFilename(doc.Title) + filepath.Ext(doc.FilePath)
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))

	http.ServeContent(w, r, filename, info.ModTime(), f)
}

// analyzeResponse is the JSON representation of a document analysis result.
type analyzeResponse struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	WordCount   int      `json:"word_count"`
	ReadingTime int      `json:"reading_time"`
	Language    string   `json:"language"`
}

// Analyze handles POST /api/documents/analyze — extract and analyze an uploaded file.
func (h *DocumentHandler) Analyze(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		errorResponse(w, http.StatusBadRequest, "file is required")
		return
	}
	defer func() { _ = file.Close() }()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	mimeType, ok := service.AllowedMIMETypes[ext]
	if !ok {
		errorResponse(w, http.StatusBadRequest, fmt.Sprintf("unsupported file type: %q", ext))
		return
	}

	// Write to a temp file for extraction.
	tmpFile, err := os.CreateTemp("", "analyze-*"+ext)
	if err != nil {
		h.logger.Error("creating temp file for analysis", "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to process file")
		return
	}
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
	}()

	if _, err := io.Copy(tmpFile, file); err != nil {
		h.logger.Error("writing temp file for analysis", "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to process file")
		return
	}
	_ = tmpFile.Close()

	ext2, err := h.pipeline.ExtractorRegistry().ForMIMEType(mimeType)
	if err != nil {
		errorResponse(w, http.StatusUnprocessableEntity, fmt.Sprintf("no extractor for type: %s", mimeType))
		return
	}

	result, err := ext2.Extract(r.Context(), tmpFile.Name())
	if err != nil {
		h.logger.Error("extracting content for analysis", "error", err)
		errorResponse(w, http.StatusInternalServerError, "content extraction failed")
		return
	}

	content := result.Content
	wordCount := len(strings.Fields(content))
	readingTime := max(wordCount/200, 1)

	title := strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename))

	resp := analyzeResponse{
		Title:       title,
		Description: firstParagraph(content),
		Tags:        extractKeywords(content),
		WordCount:   wordCount,
		ReadingTime: readingTime,
		Language:    detectLanguage(content),
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"data": resp,
	})
}

// firstParagraph returns the first non-empty paragraph of content, capped at 500 characters.
func firstParagraph(content string) string {
	paragraphs := strings.Split(content, "\n\n")
	for _, p := range paragraphs {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			if len(trimmed) > 500 {
				return trimmed[:500]
			}
			return trimmed
		}
	}
	return ""
}

// extractKeywords returns the top 5 most frequent non-stop words from content.
func extractKeywords(content string) []string {
	stopWords := map[string]struct{}{
		"the": {}, "a": {}, "an": {}, "and": {}, "or": {}, "but": {},
		"in": {}, "on": {}, "at": {}, "to": {}, "for": {}, "of": {},
		"with": {}, "by": {}, "from": {}, "is": {}, "it": {}, "that": {},
		"this": {}, "was": {}, "are": {}, "be": {}, "has": {}, "have": {},
		"had": {}, "not": {}, "no": {}, "do": {}, "does": {}, "did": {},
		"will": {}, "would": {}, "could": {}, "should": {}, "may": {},
		"might": {}, "can": {}, "shall": {}, "as": {}, "if": {}, "then": {},
		"than": {}, "so": {}, "up": {}, "out": {}, "about": {}, "into": {},
		"over": {}, "after": {}, "before": {}, "between": {}, "under": {},
		"again": {}, "there": {}, "here": {}, "when": {}, "where": {},
		"why": {}, "how": {}, "all": {}, "each": {}, "every": {}, "both": {},
		"few": {}, "more": {}, "most": {}, "other": {}, "some": {}, "such": {},
		"only": {}, "own": {}, "same": {}, "also": {}, "just": {}, "because": {},
		"its": {}, "i": {}, "me": {}, "my": {}, "we": {}, "our": {}, "you": {},
		"your": {}, "he": {}, "him": {}, "his": {}, "she": {}, "her": {},
		"they": {}, "them": {}, "their": {}, "what": {}, "which": {}, "who": {},
		"whom": {}, "been": {}, "being": {}, "were": {},
	}

	freq := make(map[string]int)
	for _, word := range strings.Fields(content) {
		w := strings.ToLower(strings.Trim(word, ".,;:!?\"'()[]{}"))
		if len(w) < 3 {
			continue
		}
		if _, stop := stopWords[w]; stop {
			continue
		}
		freq[w]++
	}

	type wordCount struct {
		word  string
		count int
	}
	ranked := make([]wordCount, 0, len(freq))
	for w, c := range freq {
		ranked = append(ranked, wordCount{word: w, count: c})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].count != ranked[j].count {
			return ranked[i].count > ranked[j].count
		}
		return ranked[i].word < ranked[j].word
	})

	limit := min(5, len(ranked))
	keywords := make([]string, limit)
	for i := 0; i < limit; i++ {
		keywords[i] = ranked[i].word
	}
	return keywords
}

// Restore handles POST /api/documents/{uuid}/restore — restore a soft-deleted document.
func (h *DocumentHandler) Restore(w http.ResponseWriter, r *http.Request) {
	docUUID := chi.URLParam(r, "uuid")

	doc, err := h.repo.FindByUUIDIncludingDeleted(r.Context(), docUUID)
	if err != nil {
		h.logger.Error("finding document for restore", "uuid", docUUID, "error", err)
		if errors.Is(err, sql.ErrNoRows) {
			errorResponse(w, http.StatusNotFound, "document not found")
			return
		}
		errorResponse(w, http.StatusInternalServerError, "failed to find document")
		return
	}

	if !doc.DeletedAt.Valid {
		errorResponse(w, http.StatusBadRequest, "document is not deleted")
		return
	}

	if err := h.repo.Restore(r.Context(), doc.ID); err != nil {
		h.logger.Error("restoring document", "uuid", docUUID, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to restore document")
		return
	}

	// Re-fetch to get updated timestamps.
	doc, err = h.repo.FindByUUID(r.Context(), docUUID)
	if err != nil {
		h.logger.Error("fetching restored document", "uuid", docUUID, "error", err)
		errorResponse(w, http.StatusInternalServerError, "document restored but failed to fetch updated record")
		return
	}

	tags, _ := h.repo.TagsForDocument(r.Context(), doc.ID)
	jsonResponse(w, http.StatusOK, map[string]any{
		"message": "Document restored successfully.",
		"data":    toDocumentResponse(doc, tags),
	})
}

// Purge handles DELETE /api/documents/{uuid}/purge — permanently delete a document.
func (h *DocumentHandler) Purge(w http.ResponseWriter, r *http.Request) {
	docUUID := chi.URLParam(r, "uuid")

	doc, err := h.repo.FindByUUIDIncludingDeleted(r.Context(), docUUID)
	if err != nil {
		h.logger.Error("finding document for purge", "uuid", docUUID, "error", err)
		if errors.Is(err, sql.ErrNoRows) {
			errorResponse(w, http.StatusNotFound, "document not found")
			return
		}
		errorResponse(w, http.StatusInternalServerError, "failed to find document")
		return
	}

	filePath, err := h.repo.PurgeSingle(r.Context(), doc.ID)
	if err != nil {
		h.logger.Error("purging document", "uuid", docUUID, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to purge document")
		return
	}

	if filePath != "" {
		fullPath := filepath.Join(h.pipeline.StoragePath(), filePath)
		if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
			h.logger.Error("removing file after purge", "path", fullPath, "error", err)
		}
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"message": "Document permanently deleted.",
	})
}

// BulkPurge handles DELETE /api/admin/documents/purge — purge all soft-deleted documents older than a threshold.
func (h *DocumentHandler) BulkPurge(w http.ResponseWriter, r *http.Request) {
	days := 30
	if d := r.URL.Query().Get("older_than_days"); d != "" {
		parsed, err := strconv.Atoi(d)
		if err != nil || parsed < 0 {
			errorResponse(w, http.StatusBadRequest, "older_than_days must be a non-negative integer")
			return
		}
		days = parsed
	}

	olderThan := time.Duration(days) * 24 * time.Hour

	paths, err := h.repo.PurgeSoftDeleted(r.Context(), olderThan)
	if err != nil {
		h.logger.Error("bulk purging documents", "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to purge documents")
		return
	}

	for _, p := range paths {
		if p.FilePath != "" {
			fullPath := filepath.Join(h.pipeline.StoragePath(), p.FilePath)
			if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
				h.logger.Error("removing file after bulk purge", "path", fullPath, "error", err)
			}
		}
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"message": fmt.Sprintf("Purged %d documents.", len(paths)),
		"count":   len(paths),
	})
}

// ListDeleted handles GET /api/documents/trash — list soft-deleted documents.
func (h *DocumentHandler) ListDeleted(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	docs, total, err := h.repo.ListDeleted(r.Context(), limit, offset)
	if err != nil {
		h.logger.Error("listing deleted documents", "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to list deleted documents")
		return
	}

	responses := make([]documentResponse, 0, len(docs))
	for i := range docs {
		doc := &docs[i]
		tags, tagsErr := h.repo.TagsForDocument(r.Context(), doc.ID)
		if tagsErr != nil {
			h.logger.Warn("loading tags for document", "doc_id", doc.ID, "error", tagsErr)
		}
		responses = append(responses, toDocumentResponse(doc, tags))
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"data": responses,
		"meta": map[string]any{
			"total": total,
		},
	})
}

// sanitizeFilename removes characters that could cause header injection in
// Content-Disposition values (quotes, backslashes, control characters).
func sanitizeFilename(name string) string {
	return strings.Map(func(r rune) rune {
		if r == '"' || r == '\\' || r < 32 {
			return '_'
		}
		return r
	}, name)
}

// detectLanguage is a placeholder that returns "en".
func detectLanguage(_ string) string {
	return "en"
}
