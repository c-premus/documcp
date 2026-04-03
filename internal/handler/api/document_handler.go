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

	authmiddleware "github.com/c-premus/documcp/internal/auth/middleware"
	"github.com/c-premus/documcp/internal/extractor"
	"github.com/c-premus/documcp/internal/model"
	"github.com/c-premus/documcp/internal/repository"
	"github.com/c-premus/documcp/internal/security"
	"github.com/c-premus/documcp/internal/service"
)

// safeStoragePath validates that filePath resolves inside storagePath,
// preventing path-traversal attacks via manipulated DB values.
func safeStoragePath(storagePath, filePath string) (string, error) {
	return security.SafeStoragePath(storagePath, filePath)
}

// documentPipeline defines the pipeline methods used by DocumentHandler.
type documentPipeline interface {
	FindByUUID(ctx context.Context, uuid string) (*model.Document, error)
	Upload(ctx context.Context, params service.UploadDocumentParams) (*model.Document, error)
	ReplaceContent(ctx context.Context, docUUID string, params service.ReplaceContentParams) (*model.Document, error)
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
	TagsForDocuments(ctx context.Context, documentIDs []int64) (map[int64][]model.DocumentTag, error)
	Restore(ctx context.Context, id int64) error
	PurgeSingle(ctx context.Context, id int64) (string, error)
	PurgeSoftDeleted(ctx context.Context, olderThan time.Duration) ([]repository.DocumentFilePath, error)
	ListDeleted(ctx context.Context, limit, offset int, userID *int64) ([]model.Document, int, error)
	ListDistinctTags(ctx context.Context, prefix string, limit int) ([]string, error)
}

const maxUploadBodySize = 50*1024*1024 + 1024 // 50 MiB + metadata overhead

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
		Status:   r.URL.Query().Get("status"),
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
	includeContent := r.URL.Query().Get("include_content") == "true"
	jsonResponse(w, http.StatusOK, map[string]any{
		"data": toDocumentResponse(doc, tags, includeContent),
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

// ReplaceContent handles POST /api/documents/{uuid}/content — replace document file content.
func (h *DocumentHandler) ReplaceContent(w http.ResponseWriter, r *http.Request) {
	docUUID := chi.URLParam(r, "uuid")

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
		if errors.Is(err, service.ErrUnsupportedFileType) || errors.Is(err, service.ErrFileTooLarge) {
			errorResponse(w, http.StatusBadRequest, err.Error())
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
// Pass includeContent=true to populate the Content field (omitted by default — can be large).
func toDocumentResponse(doc *model.Document, tags []model.DocumentTag, includeContent ...bool) documentResponse {
	tagNames := make([]string, len(tags))
	for i, t := range tags {
		tagNames[i] = t.Tag
	}

	resp := documentResponse{
		UUID:        doc.UUID,
		Title:       doc.Title,
		Description: nullStringValue(doc.Description),
		FileType:    doc.FileType,
		FileSize:    doc.FileSize,
		MIMEType:    doc.MIMEType,
		WordCount:   nullInt64Value(doc.WordCount),
		IsPublic:    doc.IsPublic,
		HasFile:     doc.FilePath != "",
		Status:      doc.Status,
		ContentHash: nullStringValue(doc.ContentHash),
		Tags:        tagNames,
		CreatedAt:   nullTimeToString(doc.CreatedAt),
		UpdatedAt:   nullTimeToString(doc.UpdatedAt),
		ProcessedAt: nullTimeToString(doc.ProcessedAt),
	}

	if len(includeContent) > 0 && includeContent[0] && doc.Content.Valid {
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
		errorResponse(w, http.StatusForbidden, "access denied")
		return
	}

	if doc.FilePath == "" {
		// No file on disk — serve content from DB if available (e.g. markdown/html created via API).
		if !doc.Content.Valid || doc.Content.String == "" {
			errorResponse(w, http.StatusNotFound, "document has no associated file")
			return
		}
		filename := sanitizeFilename(doc.Title) + "." + doc.FileType
		w.Header().Set("Content-Type", doc.MIMEType)
		w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
		w.Header().Set("Content-Length", strconv.Itoa(len(doc.Content.String)))
		_, _ = w.Write([]byte(doc.Content.String))
		return
	}

	safePath, err := safeStoragePath(h.pipeline.StoragePath(), doc.FilePath)
	if err != nil {
		h.logger.Error("path traversal check failed", "file_path", doc.FilePath, "error", err)
		errorResponse(w, http.StatusForbidden, "access denied")
		return
	}

	f, err := os.Open(safePath)
	if err != nil {
		h.logger.Error("opening document file", "path", safePath, "error", err)
		errorResponse(w, http.StatusNotFound, "file not found")
		return
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		h.logger.Error("stat document file", "path", safePath, "error", err)
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
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
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
	r.Body = http.MaxBytesReader(w, r.Body, 50<<20)
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

	if _, err = io.Copy(tmpFile, file); err != nil {
		h.logger.Error("writing temp file for analysis", "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to process file")
		return
	}
	_ = tmpFile.Close()

	ext2, err := h.pipeline.ExtractorRegistry().ForMIMEType(mimeType)
	if err != nil {
		errorResponse(w, http.StatusUnprocessableEntity, "no extractor for type: "+mimeType)
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

	// Derive title: extractor metadata > first H1 > filename.
	title := metadataString(result.Metadata, "title", "Title")
	if title == "" {
		title = firstHeading(content)
	}
	if title == "" {
		title = strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename))
	}

	// Derive description: extractor metadata > first non-heading paragraph.
	description := metadataString(result.Metadata, "description")
	if description == "" {
		description = firstParagraph(content)
	}

	// Derive tags: headings first, fall back to keyword frequency.
	tags := extractHeadingTags(content)
	if len(tags) < 3 {
		tags = extractKeywords(content)
	}

	resp := analyzeResponse{
		Title:       title,
		Description: description,
		Tags:        tags,
		WordCount:   wordCount,
		ReadingTime: readingTime,
		Language:    detectLanguage(content),
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"data": resp,
	})
}

// metadataString returns the first non-empty string value found under the given
// keys in the extractor metadata map. Keys are tried in order (e.g. "title",
// "Title") to handle format-specific casing differences.
func metadataString(metadata map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := metadata[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

// firstHeading returns the text of the first ATX heading (# Title) in content.
func firstHeading(content string) string {
	for line := range strings.SplitSeq(content, "\n") {
		if after, found := strings.CutPrefix(line, "# "); found {
			t := strings.TrimSpace(after)
			if t != "" {
				return t
			}
		}
	}
	return ""
}

// extractHeadingTags extracts unique tag suggestions from ## and ### headings.
// Returns at most 5 tags, lowercased and trimmed.
func extractHeadingTags(content string) []string {
	seen := make(map[string]bool)
	var tags []string
	for line := range strings.SplitSeq(content, "\n") {
		var heading string
		if after, ok := strings.CutPrefix(line, "### "); ok {
			heading = after
		} else if after, ok := strings.CutPrefix(line, "## "); ok {
			heading = after
		}
		if heading == "" {
			continue
		}
		tag := strings.ToLower(strings.TrimSpace(heading))
		// Skip very short or markdown-artifact headings.
		if len(tag) < 3 || strings.HasPrefix(tag, "#") {
			continue
		}
		if !seen[tag] {
			seen[tag] = true
			tags = append(tags, tag)
			if len(tags) >= 5 {
				break
			}
		}
	}
	return tags
}

// firstParagraph returns the first non-empty, non-heading paragraph of content,
// capped at 500 characters.
func firstParagraph(content string) string {
	for p := range strings.SplitSeq(content, "\n\n") {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		runes := []rune(trimmed)
		if len(runes) > 500 {
			return string(runes[:500])
		}
		return trimmed
	}
	return ""
}

// stopWords is the set of common English words excluded from keyword extraction.
var stopWords = map[string]struct{}{
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

// extractKeywords returns the top 5 most frequent non-stop words from content.
func extractKeywords(content string) []string {
	freq := make(map[string]int)
	for word := range strings.FieldsSeq(content) {
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
	for i := range limit {
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

	if err = h.repo.Restore(r.Context(), doc.ID); err != nil {
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
		safePath, pathErr := safeStoragePath(h.pipeline.StoragePath(), filePath)
		if pathErr != nil {
			h.logger.Error("path traversal check failed during purge", "file_path", filePath, "error", pathErr)
		} else if err := os.Remove(safePath); err != nil && !os.IsNotExist(err) {
			h.logger.Error("removing file after purge", "path", safePath, "error", err)
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
			safePath, pathErr := safeStoragePath(h.pipeline.StoragePath(), p.FilePath)
			if pathErr != nil {
				h.logger.Error("path traversal check failed during bulk purge", "file_path", p.FilePath, "error", pathErr)
				continue
			}
			if err := os.Remove(safePath); err != nil && !os.IsNotExist(err) {
				h.logger.Error("removing file after bulk purge", "path", safePath, "error", err)
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
	limit, offset := parsePagination(r, 50, 100)

	// Non-admin users can only see their own deleted documents.
	var userID *int64
	if user, _ := authmiddleware.UserFromContext(r.Context()); user != nil && !user.IsAdmin {
		userID = &user.ID
	}

	docs, total, err := h.repo.ListDeleted(r.Context(), limit, offset, userID)
	if err != nil {
		h.logger.Error("listing deleted documents", "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to list deleted documents")
		return
	}

	// Batch-load tags for all documents in a single query (avoids N+1).
	docIDs := make([]int64, len(docs))
	for i := range docs {
		docIDs[i] = docs[i].ID
	}
	tagsByDoc, tagsErr := h.repo.TagsForDocuments(r.Context(), docIDs)
	if tagsErr != nil {
		h.logger.Warn("batch-loading tags", "error", tagsErr)
		tagsByDoc = map[int64][]model.DocumentTag{}
	}

	responses := make([]documentResponse, 0, len(docs))
	for i := range docs {
		doc := &docs[i]
		responses = append(responses, toDocumentResponse(doc, tagsByDoc[doc.ID]))
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"data": responses,
		"meta": map[string]any{
			"total": total,
		},
	})
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

	tags, err := h.repo.ListDistinctTags(r.Context(), q, limit)
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
		if r == '"' || r == '\\' || r < 32 || r == 127 || r == '/' || r == ':' {
			return '_'
		}
		return r
	}, name)
}

// detectLanguage is a placeholder that returns "en".
func detectLanguage(_ string) string {
	return "en"
}
