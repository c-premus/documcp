package api

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	authmiddleware "github.com/c-premus/documcp/internal/auth/middleware"
	"github.com/c-premus/documcp/internal/model"
)

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
		} else if err := os.Remove(safePath); err != nil && !os.IsNotExist(err) { //nolint:gosec // safePath validated by safeStoragePath above
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
			if err := os.Remove(safePath); err != nil && !os.IsNotExist(err) { //nolint:gosec // safePath validated by safeStoragePath above
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
