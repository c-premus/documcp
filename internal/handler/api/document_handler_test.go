package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"git.999.haus/chris/DocuMCP-go/internal/model"
	"git.999.haus/chris/DocuMCP-go/internal/service"
)

// ---------------------------------------------------------------------------
// Mock repository implementing service.DocumentRepo
// ---------------------------------------------------------------------------

type mockDocumentRepo struct {
	findByUUIDFn      func(ctx context.Context, uuid string) (*model.Document, error)
	findByIDFn        func(ctx context.Context, id int64) (*model.Document, error)
	createFn          func(ctx context.Context, doc *model.Document) error
	updateFn          func(ctx context.Context, doc *model.Document) error
	softDeleteFn      func(ctx context.Context, id int64) error
	tagsForDocumentFn func(ctx context.Context, documentID int64) ([]model.DocumentTag, error)
	replaceTagsFn     func(ctx context.Context, documentID int64, tags []string) error
	createVersionFn   func(ctx context.Context, version *model.DocumentVersion) error
}

func (m *mockDocumentRepo) FindByUUID(ctx context.Context, uuid string) (*model.Document, error) {
	if m.findByUUIDFn != nil {
		return m.findByUUIDFn(ctx, uuid)
	}
	return nil, sql.ErrNoRows
}

func (m *mockDocumentRepo) FindByID(ctx context.Context, id int64) (*model.Document, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, sql.ErrNoRows
}

func (m *mockDocumentRepo) Create(ctx context.Context, doc *model.Document) error {
	if m.createFn != nil {
		return m.createFn(ctx, doc)
	}
	return nil
}

func (m *mockDocumentRepo) Update(ctx context.Context, doc *model.Document) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, doc)
	}
	return nil
}

func (m *mockDocumentRepo) SoftDelete(ctx context.Context, id int64) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, id)
	}
	return nil
}

func (m *mockDocumentRepo) TagsForDocument(ctx context.Context, documentID int64) ([]model.DocumentTag, error) {
	if m.tagsForDocumentFn != nil {
		return m.tagsForDocumentFn(ctx, documentID)
	}
	return nil, nil
}

func (m *mockDocumentRepo) ReplaceTags(ctx context.Context, documentID int64, tags []string) error {
	if m.replaceTagsFn != nil {
		return m.replaceTagsFn(ctx, documentID, tags)
	}
	return nil
}

func (m *mockDocumentRepo) CreateVersion(ctx context.Context, version *model.DocumentVersion) error {
	if m.createVersionFn != nil {
		return m.createVersionFn(ctx, version)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestDocument(uuid string) *model.Document {
	now := sql.NullTime{Time: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC), Valid: true}
	return &model.Document{
		ID:       1,
		UUID:     uuid,
		Title:    "Test Document",
		FileType: "markdown",
		FilePath: "markdown/test.md",
		FileSize: 1024,
		MIMEType: "text/markdown",
		IsPublic: true,
		Status:   "processed",
		Description: sql.NullString{
			String: "A test document",
			Valid:  true,
		},
		WordCount: sql.NullInt64{Int64: 100, Valid: true},
		ContentHash: sql.NullString{
			String: "abc123",
			Valid:  true,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// newDocumentHandlerForTest creates a DocumentHandler backed by a mock repo.
// The handler's repo field (concrete *repository.DocumentRepository) is nil,
// so only methods routed through the pipeline's DocumentService are safe to
// call. Tests that need List or TagsForDocument at the handler level must
// exercise other code paths.
func newDocumentHandlerForTest(mockRepo *mockDocumentRepo) *DocumentHandler {
	docService := service.NewDocumentService(mockRepo, testLogger())
	pipeline := service.NewDocumentPipeline(docService, nil, nil, nil, "")
	return &DocumentHandler{
		pipeline: pipeline,
		repo:     nil, // concrete repo requires DB; not used in non-List paths
		logger:   testLogger(),
	}
}

// chiContext injects chi URL params into a request context.
func chiContext(r *http.Request, params map[string]string) *http.Request {
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func decodeJSONBody(t *testing.T, body io.Reader) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.NewDecoder(body).Decode(&result); err != nil {
		t.Fatalf("decoding JSON response: %v", err)
	}
	return result
}

// ---------------------------------------------------------------------------
// Show handler tests
// ---------------------------------------------------------------------------

func TestDocumentHandler_Show(t *testing.T) {
	t.Parallel()

	// NOTE: The Show success path (document found) calls h.repo.TagsForDocument()
	// on the concrete *repository.DocumentRepository, which requires a live
	// database connection. That path is covered by integration tests.

	t.Run("returns 404 when document not found", func(t *testing.T) {
		t.Parallel()

		mock := &mockDocumentRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
				return nil, sql.ErrNoRows
			},
		}
		h := newDocumentHandlerForTest(mock)

		req := httptest.NewRequest(http.MethodGet, "/api/documents/nonexistent", nil)
		req = chiContext(req, map[string]string{"uuid": "nonexistent"})
		rr := httptest.NewRecorder()

		h.Show(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}

		body := decodeJSONBody(t, rr.Body)
		if msg, ok := body["message"].(string); !ok || msg != "document not found" {
			t.Errorf("message = %v, want 'document not found'", body["message"])
		}
	})

	t.Run("returns 500 when repo returns error", func(t *testing.T) {
		t.Parallel()

		mock := &mockDocumentRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
				return nil, fmt.Errorf("connection refused")
			},
		}
		h := newDocumentHandlerForTest(mock)

		req := httptest.NewRequest(http.MethodGet, "/api/documents/abc-123", nil)
		req = chiContext(req, map[string]string{"uuid": "abc-123"})
		rr := httptest.NewRecorder()

		h.Show(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})
}

// ---------------------------------------------------------------------------
// Update handler tests
// ---------------------------------------------------------------------------

func TestDocumentHandler_Update(t *testing.T) {
	t.Parallel()

	t.Run("returns 400 for invalid JSON body", func(t *testing.T) {
		t.Parallel()

		mock := &mockDocumentRepo{}
		h := newDocumentHandlerForTest(mock)

		req := httptest.NewRequest(http.MethodPut, "/api/documents/abc-123",
			strings.NewReader("not json"))
		req = chiContext(req, map[string]string{"uuid": "abc-123"})
		rr := httptest.NewRecorder()

		h.Update(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
		body := decodeJSONBody(t, rr.Body)
		if msg := body["message"]; msg != "invalid JSON body" {
			t.Errorf("message = %v, want 'invalid JSON body'", msg)
		}
	})

	t.Run("returns 404 when document not found", func(t *testing.T) {
		t.Parallel()

		mock := &mockDocumentRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
				return nil, sql.ErrNoRows
			},
		}
		h := newDocumentHandlerForTest(mock)

		body := `{"title":"New Title"}`
		req := httptest.NewRequest(http.MethodPut, "/api/documents/nonexistent",
			strings.NewReader(body))
		req = chiContext(req, map[string]string{"uuid": "nonexistent"})
		rr := httptest.NewRecorder()

		h.Update(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("returns 500 when pipeline update fails", func(t *testing.T) {
		t.Parallel()

		doc := newTestDocument("abc-123")
		mock := &mockDocumentRepo{
			findByUUIDFn: func(_ context.Context, uuid string) (*model.Document, error) {
				if uuid == "abc-123" {
					return doc, nil
				}
				return nil, sql.ErrNoRows
			},
			updateFn: func(_ context.Context, _ *model.Document) error {
				return fmt.Errorf("database write error")
			},
		}
		h := newDocumentHandlerForTest(mock)

		reqBody := `{"title":"Updated Title"}`
		req := httptest.NewRequest(http.MethodPut, "/api/documents/abc-123",
			strings.NewReader(reqBody))
		req = chiContext(req, map[string]string{"uuid": "abc-123"})
		rr := httptest.NewRecorder()

		h.Update(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})

	t.Run("accepts empty JSON body", func(t *testing.T) {
		t.Parallel()

		mock := &mockDocumentRepo{}
		h := newDocumentHandlerForTest(mock)

		// Empty object is valid JSON, should proceed to pipeline.Update
		// which will fail because the doc won't be found.
		reqBody := `{}`
		req := httptest.NewRequest(http.MethodPut, "/api/documents/missing",
			strings.NewReader(reqBody))
		req = chiContext(req, map[string]string{"uuid": "missing"})
		rr := httptest.NewRecorder()

		h.Update(rr, req)

		// Doc not found (sql.ErrNoRows) via the pipeline
		if rr.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})
}

// ---------------------------------------------------------------------------
// Delete handler tests
// ---------------------------------------------------------------------------

func TestDocumentHandler_Delete(t *testing.T) {
	t.Parallel()

	t.Run("deletes document successfully", func(t *testing.T) {
		t.Parallel()

		doc := newTestDocument("abc-123")
		mock := &mockDocumentRepo{
			findByUUIDFn: func(_ context.Context, uuid string) (*model.Document, error) {
				if uuid == "abc-123" {
					return doc, nil
				}
				return nil, sql.ErrNoRows
			},
			softDeleteFn: func(_ context.Context, _ int64) error {
				return nil
			},
		}
		h := newDocumentHandlerForTest(mock)

		req := httptest.NewRequest(http.MethodDelete, "/api/documents/abc-123", nil)
		req = chiContext(req, map[string]string{"uuid": "abc-123"})
		rr := httptest.NewRecorder()

		h.Delete(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		body := decodeJSONBody(t, rr.Body)
		if msg := body["message"]; msg != "Document deleted successfully." {
			t.Errorf("message = %v, want 'Document deleted successfully.'", msg)
		}
	})

	t.Run("returns 404 when document not found", func(t *testing.T) {
		t.Parallel()

		mock := &mockDocumentRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
				return nil, sql.ErrNoRows
			},
		}
		h := newDocumentHandlerForTest(mock)

		req := httptest.NewRequest(http.MethodDelete, "/api/documents/nonexistent", nil)
		req = chiContext(req, map[string]string{"uuid": "nonexistent"})
		rr := httptest.NewRecorder()

		h.Delete(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("returns 500 when soft delete fails", func(t *testing.T) {
		t.Parallel()

		doc := newTestDocument("abc-123")
		mock := &mockDocumentRepo{
			findByUUIDFn: func(_ context.Context, uuid string) (*model.Document, error) {
				if uuid == "abc-123" {
					return doc, nil
				}
				return nil, sql.ErrNoRows
			},
			softDeleteFn: func(_ context.Context, _ int64) error {
				return fmt.Errorf("database error")
			},
		}
		h := newDocumentHandlerForTest(mock)

		req := httptest.NewRequest(http.MethodDelete, "/api/documents/abc-123", nil)
		req = chiContext(req, map[string]string{"uuid": "abc-123"})
		rr := httptest.NewRecorder()

		h.Delete(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})
}

// ---------------------------------------------------------------------------
// Upload handler tests
// ---------------------------------------------------------------------------

func TestDocumentHandler_Upload(t *testing.T) {
	t.Parallel()

	t.Run("returns 400 when no multipart form", func(t *testing.T) {
		t.Parallel()

		mock := &mockDocumentRepo{}
		h := newDocumentHandlerForTest(mock)

		req := httptest.NewRequest(http.MethodPost, "/api/documents",
			strings.NewReader("not multipart"))
		req.Header.Set("Content-Type", "text/plain")
		rr := httptest.NewRecorder()

		h.Upload(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
		body := decodeJSONBody(t, rr.Body)
		if msg := body["message"]; msg != "invalid multipart form" {
			t.Errorf("message = %v, want 'invalid multipart form'", msg)
		}
	})

	t.Run("returns 400 when file field is missing", func(t *testing.T) {
		t.Parallel()

		mock := &mockDocumentRepo{}
		h := newDocumentHandlerForTest(mock)

		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		_ = writer.WriteField("title", "Test")
		_ = writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/documents", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rr := httptest.NewRecorder()

		h.Upload(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
		body := decodeJSONBody(t, rr.Body)
		if msg := body["message"]; msg != "file is required" {
			t.Errorf("message = %v, want 'file is required'", msg)
		}
	})

	t.Run("returns 400 for unsupported file type", func(t *testing.T) {
		t.Parallel()

		mock := &mockDocumentRepo{}
		h := newDocumentHandlerForTest(mock)

		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		part, _ := writer.CreateFormFile("file", "test.exe")
		_, _ = part.Write([]byte("binary content"))
		_ = writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/documents", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rr := httptest.NewRecorder()

		h.Upload(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
		body := decodeJSONBody(t, rr.Body)
		msg, ok := body["message"].(string)
		if !ok || !strings.Contains(msg, "unsupported file type") {
			t.Errorf("message = %v, want it to contain 'unsupported file type'", body["message"])
		}
	})
}

// ---------------------------------------------------------------------------
// toDocumentResponse tests
// ---------------------------------------------------------------------------

func TestToDocumentResponse(t *testing.T) {
	t.Parallel()

	t.Run("maps all fields correctly", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
		doc := &model.Document{
			UUID:        "doc-uuid-1",
			Title:       "My Doc",
			Description: sql.NullString{String: "desc", Valid: true},
			FileType:    "pdf",
			FilePath:    "pdf/doc.pdf",
			FileSize:    2048,
			MIMEType:    "application/pdf",
			WordCount:   sql.NullInt64{Int64: 500, Valid: true},
			IsPublic:    true,
			Status:      "indexed",
			ContentHash: sql.NullString{String: "hash123", Valid: true},
			CreatedAt:   sql.NullTime{Time: now, Valid: true},
			UpdatedAt:   sql.NullTime{Time: now, Valid: true},
			ProcessedAt: sql.NullTime{Time: now, Valid: true},
		}
		tags := []model.DocumentTag{
			{Tag: "go"},
			{Tag: "testing"},
		}

		resp := toDocumentResponse(doc, tags)

		if resp.UUID != "doc-uuid-1" {
			t.Errorf("UUID = %q, want doc-uuid-1", resp.UUID)
		}
		if resp.Title != "My Doc" {
			t.Errorf("Title = %q, want My Doc", resp.Title)
		}
		if resp.Description != "desc" {
			t.Errorf("Description = %q, want desc", resp.Description)
		}
		if resp.FileType != "pdf" {
			t.Errorf("FileType = %q, want pdf", resp.FileType)
		}
		if resp.FileSize != 2048 {
			t.Errorf("FileSize = %d, want 2048", resp.FileSize)
		}
		if resp.MIMEType != "application/pdf" {
			t.Errorf("MIMEType = %q, want application/pdf", resp.MIMEType)
		}
		if resp.WordCount != 500 {
			t.Errorf("WordCount = %d, want 500", resp.WordCount)
		}
		if !resp.IsPublic {
			t.Error("IsPublic = false, want true")
		}
		if resp.Status != "indexed" {
			t.Errorf("Status = %q, want indexed", resp.Status)
		}
		if resp.ContentHash != "hash123" {
			t.Errorf("ContentHash = %q, want hash123", resp.ContentHash)
		}
		if len(resp.Tags) != 2 || resp.Tags[0] != "go" || resp.Tags[1] != "testing" {
			t.Errorf("Tags = %v, want [go testing]", resp.Tags)
		}
		wantTime := now.Format(time.RFC3339)
		if resp.CreatedAt != wantTime {
			t.Errorf("CreatedAt = %q, want %q", resp.CreatedAt, wantTime)
		}
	})

	t.Run("handles nil tags", func(t *testing.T) {
		t.Parallel()

		doc := newTestDocument("abc")
		resp := toDocumentResponse(doc, nil)

		if resp.Tags == nil {
			t.Error("Tags should be empty slice, not nil")
		}
		if len(resp.Tags) != 0 {
			t.Errorf("Tags length = %d, want 0", len(resp.Tags))
		}
	})

	t.Run("handles null optional fields", func(t *testing.T) {
		t.Parallel()

		doc := &model.Document{
			UUID:     "uuid-1",
			Title:    "Title",
			FileType: "md",
			Status:   "uploaded",
		}
		resp := toDocumentResponse(doc, nil)

		if resp.Description != "" {
			t.Errorf("Description = %q, want empty", resp.Description)
		}
		if resp.ContentHash != "" {
			t.Errorf("ContentHash = %q, want empty", resp.ContentHash)
		}
		if resp.ProcessedAt != "" {
			t.Errorf("ProcessedAt = %q, want empty", resp.ProcessedAt)
		}
		if resp.CreatedAt != "" {
			t.Errorf("CreatedAt = %q, want empty for null time", resp.CreatedAt)
		}
	})
}

// ---------------------------------------------------------------------------
// formatTime tests
// ---------------------------------------------------------------------------

func TestFormatTime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		t    sql.NullTime
		want string
	}{
		{
			name: "valid time formats as RFC3339",
			t:    sql.NullTime{Time: time.Date(2025, 3, 10, 14, 30, 0, 0, time.UTC), Valid: true},
			want: "2025-03-10T14:30:00Z",
		},
		{
			name: "invalid null time returns empty string",
			t:    sql.NullTime{Valid: false},
			want: "",
		},
		{
			name: "zero time with valid flag",
			t:    sql.NullTime{Time: time.Time{}, Valid: true},
			want: "0001-01-01T00:00:00Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := formatTime(tt.t)
			if got != tt.want {
				t.Errorf("formatTime() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// jsonResponse and errorResponse tests
// ---------------------------------------------------------------------------

func TestJsonResponse(t *testing.T) {
	t.Parallel()

	t.Run("sets content type and status", func(t *testing.T) {
		t.Parallel()

		rr := httptest.NewRecorder()
		jsonResponse(rr, http.StatusCreated, map[string]string{"key": "value"})

		if rr.Code != http.StatusCreated {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusCreated)
		}
		if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}

		body := decodeJSONBody(t, rr.Body)
		if body["key"] != "value" {
			t.Errorf("body key = %v, want value", body["key"])
		}
	})

	t.Run("encodes complex structures", func(t *testing.T) {
		t.Parallel()

		rr := httptest.NewRecorder()
		data := map[string]any{
			"items": []string{"a", "b"},
			"count": 2,
		}
		jsonResponse(rr, http.StatusOK, data)

		body := decodeJSONBody(t, rr.Body)
		items, ok := body["items"].([]any)
		if !ok || len(items) != 2 {
			t.Errorf("items = %v, want [a b]", body["items"])
		}
	})
}

func TestErrorResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		status     int
		message    string
		wantError  string
	}{
		{
			name:      "bad request",
			status:    http.StatusBadRequest,
			message:   "invalid input",
			wantError: "Bad Request",
		},
		{
			name:      "not found",
			status:    http.StatusNotFound,
			message:   "resource not found",
			wantError: "Not Found",
		},
		{
			name:      "internal server error",
			status:    http.StatusInternalServerError,
			message:   "something went wrong",
			wantError: "Internal Server Error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rr := httptest.NewRecorder()
			errorResponse(rr, tt.status, tt.message)

			if rr.Code != tt.status {
				t.Errorf("status = %d, want %d", rr.Code, tt.status)
			}

			body := decodeJSONBody(t, rr.Body)
			if body["error"] != tt.wantError {
				t.Errorf("error = %v, want %q", body["error"], tt.wantError)
			}
			if body["message"] != tt.message {
				t.Errorf("message = %v, want %q", body["message"], tt.message)
			}
		})
	}
}
