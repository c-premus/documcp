package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authmiddleware "git.999.haus/chris/DocuMCP-go/internal/auth/middleware"
	"git.999.haus/chris/DocuMCP-go/internal/extractor"
	"git.999.haus/chris/DocuMCP-go/internal/model"
	"git.999.haus/chris/DocuMCP-go/internal/repository"
	"git.999.haus/chris/DocuMCP-go/internal/service"
)

// ---------------------------------------------------------------------------
// Mock repository implementing service.DocumentRepo (used by DocumentService)
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
// Mock pipeline implementing documentPipeline interface
// ---------------------------------------------------------------------------

type mockPipeline struct {
	findByUUIDFn       func(ctx context.Context, uuid string) (*model.Document, error)
	uploadFn           func(ctx context.Context, params service.UploadDocumentParams) (*model.Document, error)
	updateFn           func(ctx context.Context, docUUID string, params service.UpdateDocumentParams) (*model.Document, error)
	deleteFn           func(ctx context.Context, docUUID string) error
	storagePathVal     string
	extractorRegistryV *extractor.Registry
}

func (m *mockPipeline) FindByUUID(ctx context.Context, uuid string) (*model.Document, error) {
	if m.findByUUIDFn != nil {
		return m.findByUUIDFn(ctx, uuid)
	}
	return nil, nil
}

func (m *mockPipeline) Upload(ctx context.Context, params service.UploadDocumentParams) (*model.Document, error) {
	if m.uploadFn != nil {
		return m.uploadFn(ctx, params)
	}
	return nil, errors.New("not implemented")
}

func (m *mockPipeline) Update(ctx context.Context, docUUID string, params service.UpdateDocumentParams) (*model.Document, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, docUUID, params)
	}
	return nil, errors.New("not implemented")
}

func (m *mockPipeline) Delete(ctx context.Context, docUUID string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, docUUID)
	}
	return nil
}

func (m *mockPipeline) StoragePath() string {
	return m.storagePathVal
}

func (m *mockPipeline) ExtractorRegistry() *extractor.Registry {
	return m.extractorRegistryV
}

// ---------------------------------------------------------------------------
// Mock handler-level repo implementing documentRepo interface
// ---------------------------------------------------------------------------

type mockHandlerRepo struct {
	listFn                       func(ctx context.Context, params repository.DocumentListParams) (*repository.DocumentListResult, error)
	findByUUIDFn                 func(ctx context.Context, uuid string) (*model.Document, error)
	findByUUIDIncludingDeletedFn func(ctx context.Context, uuid string) (*model.Document, error)
	tagsForDocumentFn            func(ctx context.Context, documentID int64) ([]model.DocumentTag, error)
	restoreFn                    func(ctx context.Context, id int64) error
	purgeSingleFn                func(ctx context.Context, id int64) (string, error)
	purgeSoftDeletedFn           func(ctx context.Context, olderThan time.Duration) ([]repository.DocumentFilePath, error)
	listDeletedFn                func(ctx context.Context, limit, offset int, userID *int64) ([]model.Document, int, error)
}

func (m *mockHandlerRepo) List(ctx context.Context, params repository.DocumentListParams) (*repository.DocumentListResult, error) {
	if m.listFn != nil {
		return m.listFn(ctx, params)
	}
	return &repository.DocumentListResult{}, nil
}

func (m *mockHandlerRepo) FindByUUID(ctx context.Context, uuid string) (*model.Document, error) {
	if m.findByUUIDFn != nil {
		return m.findByUUIDFn(ctx, uuid)
	}
	return nil, sql.ErrNoRows
}

func (m *mockHandlerRepo) FindByUUIDIncludingDeleted(ctx context.Context, uuid string) (*model.Document, error) {
	if m.findByUUIDIncludingDeletedFn != nil {
		return m.findByUUIDIncludingDeletedFn(ctx, uuid)
	}
	return nil, sql.ErrNoRows
}

func (m *mockHandlerRepo) TagsForDocument(ctx context.Context, documentID int64) ([]model.DocumentTag, error) {
	if m.tagsForDocumentFn != nil {
		return m.tagsForDocumentFn(ctx, documentID)
	}
	return nil, nil
}

func (m *mockHandlerRepo) Restore(ctx context.Context, id int64) error {
	if m.restoreFn != nil {
		return m.restoreFn(ctx, id)
	}
	return nil
}

func (m *mockHandlerRepo) PurgeSingle(ctx context.Context, id int64) (string, error) {
	if m.purgeSingleFn != nil {
		return m.purgeSingleFn(ctx, id)
	}
	return "", nil
}

func (m *mockHandlerRepo) PurgeSoftDeleted(ctx context.Context, olderThan time.Duration) ([]repository.DocumentFilePath, error) {
	if m.purgeSoftDeletedFn != nil {
		return m.purgeSoftDeletedFn(ctx, olderThan)
	}
	return nil, nil
}

func (m *mockHandlerRepo) ListDeleted(ctx context.Context, limit, offset int, userID *int64) ([]model.Document, int, error) {
	if m.listDeletedFn != nil {
		return m.listDeletedFn(ctx, limit, offset, userID)
	}
	return nil, 0, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func testLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
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

// newDocumentHandlerForTest creates a DocumentHandler backed by a mock service-level
// repo. The pipeline is a real *service.DocumentPipeline backed by the mock.
// The handler repo is a mockHandlerRepo with TagsForDocument wired through.
func newDocumentHandlerForTest(mockRepo *mockDocumentRepo) *DocumentHandler {
	docService := service.NewDocumentService(mockRepo, testLogger())
	pipeline := service.NewDocumentPipeline(docService, nil, nil, nil, "")
	return &DocumentHandler{
		pipeline: pipeline,
		repo: &mockHandlerRepo{
			tagsForDocumentFn: mockRepo.tagsForDocumentFn,
		},
		logger: testLogger(),
	}
}

// newTestHandler creates a DocumentHandler using mock pipeline and mock repo directly.
func newTestHandler(p *mockPipeline, r *mockHandlerRepo) *DocumentHandler {
	return &DocumentHandler{
		pipeline: p,
		repo:     r,
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

		req := httptest.NewRequest(http.MethodGet, "/api/documents/nonexistent", http.NoBody)
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
				return nil, errors.New("connection refused")
			},
		}
		h := newDocumentHandlerForTest(mock)

		req := httptest.NewRequest(http.MethodGet, "/api/documents/abc-123", http.NoBody)
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
				return errors.New("database write error")
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

		req := httptest.NewRequest(http.MethodDelete, "/api/documents/abc-123", http.NoBody)
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

		req := httptest.NewRequest(http.MethodDelete, "/api/documents/nonexistent", http.NoBody)
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
				return errors.New("database error")
			},
		}
		h := newDocumentHandlerForTest(mock)

		req := httptest.NewRequest(http.MethodDelete, "/api/documents/abc-123", http.NoBody)
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
		name      string
		status    int
		message   string
		wantError string
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

// ---------------------------------------------------------------------------
// Mock extractor for Analyze handler tests
// ---------------------------------------------------------------------------

type mockExtractor struct {
	extractFn func(ctx context.Context, filePath string) (*extractor.ExtractedContent, error)
	mimeTypes []string
}

func (m *mockExtractor) Extract(ctx context.Context, filePath string) (*extractor.ExtractedContent, error) {
	if m.extractFn != nil {
		return m.extractFn(ctx, filePath)
	}
	return &extractor.ExtractedContent{}, nil
}

func (m *mockExtractor) Supports(mimeType string) bool {
	return slices.Contains(m.mimeTypes, mimeType)
}

// newDocumentHandlerWithExtractor creates a DocumentHandler with a pipeline
// that has a working ExtractorRegistry backed by the given mock extractor.
func newDocumentHandlerWithExtractor(mockRepo *mockDocumentRepo, ext extractor.Extractor) *DocumentHandler {
	docService := service.NewDocumentService(mockRepo, testLogger())
	registry := extractor.NewRegistry(ext)
	pipeline := service.NewDocumentPipeline(docService, registry, nil, nil, "")
	return &DocumentHandler{
		pipeline: pipeline,
		repo:     &mockHandlerRepo{},
		logger:   testLogger(),
	}
}

// ---------------------------------------------------------------------------
// Analyze handler tests
// ---------------------------------------------------------------------------

func TestDocumentHandler_Analyze(t *testing.T) {
	t.Parallel()

	t.Run("returns 400 for unsupported file type", func(t *testing.T) {
		t.Parallel()

		ext := &mockExtractor{mimeTypes: []string{"text/markdown"}}
		h := newDocumentHandlerWithExtractor(&mockDocumentRepo{}, ext)

		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		part, err := writer.CreateFormFile("file", "malware.exe")
		require.NoError(t, err)
		_, _ = part.Write([]byte("malicious content"))
		_ = writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/documents/analyze", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rr := httptest.NewRecorder()

		h.Analyze(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		body := decodeJSONBody(t, rr.Body)
		msg, ok := body["message"].(string)
		assert.True(t, ok)
		assert.Contains(t, msg, "unsupported file type")
		assert.Contains(t, msg, ".exe")
	})

	t.Run("returns 400 when no file provided", func(t *testing.T) {
		t.Parallel()

		ext := &mockExtractor{mimeTypes: []string{"text/markdown"}}
		h := newDocumentHandlerWithExtractor(&mockDocumentRepo{}, ext)

		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		_ = writer.WriteField("title", "test")
		_ = writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/documents/analyze", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rr := httptest.NewRecorder()

		h.Analyze(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		body := decodeJSONBody(t, rr.Body)
		assert.Equal(t, "file is required", body["message"])
	})

	t.Run("returns 400 for invalid multipart form", func(t *testing.T) {
		t.Parallel()

		ext := &mockExtractor{mimeTypes: []string{"text/markdown"}}
		h := newDocumentHandlerWithExtractor(&mockDocumentRepo{}, ext)

		req := httptest.NewRequest(http.MethodPost, "/api/documents/analyze",
			strings.NewReader("not multipart data"))
		req.Header.Set("Content-Type", "text/plain")
		rr := httptest.NewRecorder()

		h.Analyze(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("returns 200 with analysis for valid markdown file", func(t *testing.T) {
		t.Parallel()

		content := "Golang is a statically typed programming language. " +
			"Golang was designed at Google by Robert Griesemer. " +
			"Golang is popular for building scalable services."

		ext := &mockExtractor{
			mimeTypes: []string{"text/markdown"},
			extractFn: func(_ context.Context, _ string) (*extractor.ExtractedContent, error) {
				return &extractor.ExtractedContent{
					Content:   content,
					WordCount: len(strings.Fields(content)),
				}, nil
			},
		}
		h := newDocumentHandlerWithExtractor(&mockDocumentRepo{}, ext)

		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		part, err := writer.CreateFormFile("file", "readme.md")
		require.NoError(t, err)
		_, _ = part.Write([]byte(content))
		_ = writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/documents/analyze", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rr := httptest.NewRecorder()

		h.Analyze(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		body := decodeJSONBody(t, rr.Body)
		data, ok := body["data"].(map[string]any)
		require.True(t, ok, "expected data key in response")

		assert.Equal(t, "readme", data["title"])
		assert.NotEmpty(t, data["description"])
		assert.Equal(t, "en", data["language"])

		wordCount, ok := data["word_count"].(float64)
		require.True(t, ok)
		assert.Greater(t, wordCount, float64(0))

		readingTime, ok := data["reading_time"].(float64)
		require.True(t, ok)
		assert.GreaterOrEqual(t, readingTime, float64(1))

		tags, ok := data["tags"].([]any)
		require.True(t, ok)
		assert.NotEmpty(t, tags)
	})

	t.Run("returns 422 when no extractor available for MIME type", func(t *testing.T) {
		t.Parallel()

		// Create an extractor that does NOT support text/markdown.
		ext := &mockExtractor{mimeTypes: []string{"application/pdf"}}
		h := newDocumentHandlerWithExtractor(&mockDocumentRepo{}, ext)

		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		part, err := writer.CreateFormFile("file", "document.md")
		require.NoError(t, err)
		_, _ = part.Write([]byte("some markdown content"))
		_ = writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/documents/analyze", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rr := httptest.NewRecorder()

		h.Analyze(rr, req)

		assert.Equal(t, http.StatusUnprocessableEntity, rr.Code)
		body := decodeJSONBody(t, rr.Body)
		msg, ok := body["message"].(string)
		assert.True(t, ok)
		assert.Contains(t, msg, "no extractor for type")
	})

	t.Run("returns 500 when extraction fails", func(t *testing.T) {
		t.Parallel()

		ext := &mockExtractor{
			mimeTypes: []string{"text/markdown"},
			extractFn: func(_ context.Context, _ string) (*extractor.ExtractedContent, error) {
				return nil, errors.New("extraction engine failure")
			},
		}
		h := newDocumentHandlerWithExtractor(&mockDocumentRepo{}, ext)

		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		part, err := writer.CreateFormFile("file", "broken.md")
		require.NoError(t, err)
		_, _ = part.Write([]byte("content"))
		_ = writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/documents/analyze", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rr := httptest.NewRecorder()

		h.Analyze(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		body := decodeJSONBody(t, rr.Body)
		assert.Equal(t, "content extraction failed", body["message"])
	})

	t.Run("reading time is at least 1 minute for short content", func(t *testing.T) {
		t.Parallel()

		ext := &mockExtractor{
			mimeTypes: []string{"text/markdown"},
			extractFn: func(_ context.Context, _ string) (*extractor.ExtractedContent, error) {
				return &extractor.ExtractedContent{
					Content:   "Short.",
					WordCount: 1,
				}, nil
			},
		}
		h := newDocumentHandlerWithExtractor(&mockDocumentRepo{}, ext)

		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		part, err := writer.CreateFormFile("file", "tiny.md")
		require.NoError(t, err)
		_, _ = part.Write([]byte("Short."))
		_ = writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/documents/analyze", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rr := httptest.NewRecorder()

		h.Analyze(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		body := decodeJSONBody(t, rr.Body)
		data := body["data"].(map[string]any)
		assert.InEpsilon(t, float64(1), data["reading_time"], 1e-9)
	})
}

// ---------------------------------------------------------------------------
// Download handler tests
// ---------------------------------------------------------------------------

func TestDocumentHandler_Download(t *testing.T) {
	t.Parallel()

	t.Run("returns 404 when document not found", func(t *testing.T) {
		t.Parallel()

		repo := &mockHandlerRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
				return nil, sql.ErrNoRows
			},
		}
		h := newTestHandler(&mockPipeline{}, repo)

		req := httptest.NewRequest(http.MethodGet, "/api/documents/nonexistent/download", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "nonexistent"})
		rr := httptest.NewRecorder()

		h.Download(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
		body := decodeJSONBody(t, rr.Body)
		assert.Equal(t, "document not found", body["message"])
	})

	t.Run("returns 500 when repo returns non-ErrNoRows error", func(t *testing.T) {
		t.Parallel()

		repo := &mockHandlerRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
				return nil, errors.New("connection refused")
			},
		}
		h := newTestHandler(&mockPipeline{}, repo)

		req := httptest.NewRequest(http.MethodGet, "/api/documents/abc/download", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "abc"})
		rr := httptest.NewRecorder()

		h.Download(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		body := decodeJSONBody(t, rr.Body)
		assert.Equal(t, "failed to find document", body["message"])
	})

	t.Run("returns 403 for private document without authenticated user", func(t *testing.T) {
		t.Parallel()

		doc := newTestDocument("priv-uuid")
		doc.IsPublic = false
		doc.UserID = sql.NullInt64{Int64: 42, Valid: true}

		repo := &mockHandlerRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
				return doc, nil
			},
		}
		h := newTestHandler(&mockPipeline{}, repo)

		req := httptest.NewRequest(http.MethodGet, "/api/documents/priv-uuid/download", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "priv-uuid"})
		rr := httptest.NewRecorder()

		h.Download(rr, req)

		assert.Equal(t, http.StatusForbidden, rr.Code)
		body := decodeJSONBody(t, rr.Body)
		assert.Equal(t, "access denied", body["message"])
	})

	t.Run("returns 403 for private document when non-owner is authenticated", func(t *testing.T) {
		t.Parallel()

		doc := newTestDocument("priv-uuid-2")
		doc.IsPublic = false
		doc.UserID = sql.NullInt64{Int64: 42, Valid: true}

		repo := &mockHandlerRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
				return doc, nil
			},
		}
		h := newTestHandler(&mockPipeline{}, repo)

		user := &model.User{ID: 99, Name: "Stranger"}
		req := httptest.NewRequest(http.MethodGet, "/api/documents/priv-uuid-2/download", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "priv-uuid-2"})
		ctx := context.WithValue(req.Context(), authmiddleware.UserContextKey, user)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		h.Download(rr, req)

		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("returns 404 when document has no associated file", func(t *testing.T) {
		t.Parallel()

		doc := newTestDocument("no-file-uuid")
		doc.IsPublic = true
		doc.FilePath = ""

		repo := &mockHandlerRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
				return doc, nil
			},
		}
		h := newTestHandler(&mockPipeline{}, repo)

		req := httptest.NewRequest(http.MethodGet, "/api/documents/no-file-uuid/download", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "no-file-uuid"})
		rr := httptest.NewRecorder()

		h.Download(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
		body := decodeJSONBody(t, rr.Body)
		assert.Equal(t, "document has no associated file", body["message"])
	})

	t.Run("returns 404 when file does not exist on disk", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		doc := newTestDocument("missing-file-uuid")
		doc.IsPublic = true
		doc.FilePath = "markdown/nonexistent.md"

		repo := &mockHandlerRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
				return doc, nil
			},
		}
		h := newTestHandler(&mockPipeline{storagePathVal: tmpDir}, repo)

		req := httptest.NewRequest(http.MethodGet, "/api/documents/missing-file-uuid/download", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "missing-file-uuid"})
		rr := httptest.NewRecorder()

		h.Download(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
		body := decodeJSONBody(t, rr.Body)
		assert.Equal(t, "file not found", body["message"])
	})

	t.Run("serves public document file with correct headers", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		subDir := filepath.Join(tmpDir, "markdown")
		require.NoError(t, os.MkdirAll(subDir, 0o750))

		fileContent := "# Hello World\n\nThis is test content."
		require.NoError(t, os.WriteFile(filepath.Join(subDir, "test.md"), []byte(fileContent), 0o600))

		doc := newTestDocument("dl-uuid-1")
		doc.IsPublic = true
		doc.FilePath = "markdown/test.md"
		doc.MIMEType = "text/markdown"
		doc.Title = "Test Document"

		repo := &mockHandlerRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
				return doc, nil
			},
		}
		h := newTestHandler(&mockPipeline{storagePathVal: tmpDir}, repo)

		req := httptest.NewRequest(http.MethodGet, "/api/documents/dl-uuid-1/download", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "dl-uuid-1"})
		rr := httptest.NewRecorder()

		h.Download(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "text/markdown", rr.Header().Get("Content-Type"))
		assert.Contains(t, rr.Header().Get("Content-Disposition"), "Test Document.md")
		assert.Equal(t, fileContent, rr.Body.String())
	})

	t.Run("owner can download private document", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		subDir := filepath.Join(tmpDir, "markdown")
		require.NoError(t, os.MkdirAll(subDir, 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(subDir, "private.md"), []byte("secret"), 0o600))

		doc := newTestDocument("owner-dl-uuid")
		doc.IsPublic = false
		doc.UserID = sql.NullInt64{Int64: 42, Valid: true}
		doc.FilePath = "markdown/private.md"

		repo := &mockHandlerRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
				return doc, nil
			},
		}
		h := newTestHandler(&mockPipeline{storagePathVal: tmpDir}, repo)

		user := &model.User{ID: 42, Name: "Owner"}
		req := httptest.NewRequest(http.MethodGet, "/api/documents/owner-dl-uuid/download", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "owner-dl-uuid"})
		ctx := context.WithValue(req.Context(), authmiddleware.UserContextKey, user)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		h.Download(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "secret", rr.Body.String())
	})

	t.Run("uses UUID as filename when title is empty", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		subDir := filepath.Join(tmpDir, "markdown")
		require.NoError(t, os.MkdirAll(subDir, 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(subDir, "test.md"), []byte("content"), 0o600))

		doc := newTestDocument("notitle-uuid")
		doc.IsPublic = true
		doc.Title = ""
		doc.FilePath = "markdown/test.md"

		repo := &mockHandlerRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
				return doc, nil
			},
		}
		h := newTestHandler(&mockPipeline{storagePathVal: tmpDir}, repo)

		req := httptest.NewRequest(http.MethodGet, "/api/documents/notitle-uuid/download", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "notitle-uuid"})
		rr := httptest.NewRecorder()

		h.Download(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Header().Get("Content-Disposition"), "notitle-uuid.md")
	})

	t.Run("defaults to application/octet-stream when MIME type is empty", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		subDir := filepath.Join(tmpDir, "markdown")
		require.NoError(t, os.MkdirAll(subDir, 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(subDir, "test.md"), []byte("data"), 0o600))

		doc := newTestDocument("notype-uuid")
		doc.IsPublic = true
		doc.MIMEType = ""
		doc.FilePath = "markdown/test.md"

		repo := &mockHandlerRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
				return doc, nil
			},
		}
		h := newTestHandler(&mockPipeline{storagePathVal: tmpDir}, repo)

		req := httptest.NewRequest(http.MethodGet, "/api/documents/notype-uuid/download", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "notype-uuid"})
		rr := httptest.NewRecorder()

		h.Download(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/octet-stream", rr.Header().Get("Content-Type"))
	})
}

// ---------------------------------------------------------------------------
// firstParagraph tests
// ---------------------------------------------------------------------------

func TestFirstParagraph(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "returns first paragraph from multi-paragraph text",
			content: "First paragraph here.\n\nSecond paragraph.\n\nThird paragraph.",
			want:    "First paragraph here.",
		},
		{
			name:    "returns single paragraph as-is",
			content: "Only one paragraph without any double newlines.",
			want:    "Only one paragraph without any double newlines.",
		},
		{
			name:    "returns empty string for empty input",
			content: "",
			want:    "",
		},
		{
			name:    "truncates long first paragraph to 500 characters",
			content: strings.Repeat("A", 600) + "\n\nSecond paragraph.",
			want:    strings.Repeat("A", 500),
		},
		{
			name:    "returns exactly 500 chars when first paragraph is 500 chars",
			content: strings.Repeat("B", 500) + "\n\nSecond.",
			want:    strings.Repeat("B", 500),
		},
		{
			name:    "returns full text when first paragraph is under 500 chars",
			content: strings.Repeat("C", 499) + "\n\nSecond.",
			want:    strings.Repeat("C", 499),
		},
		{
			name:    "skips leading empty paragraphs",
			content: "\n\n\n\nActual first paragraph.\n\nSecond.",
			want:    "Actual first paragraph.",
		},
		{
			name:    "trims whitespace from first paragraph",
			content: "  \t First paragraph with whitespace  \t \n\nSecond.",
			want:    "First paragraph with whitespace",
		},
		{
			name:    "returns empty for whitespace-only content",
			content: "   \n\n   \n\n   ",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := firstParagraph(tt.content)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ---------------------------------------------------------------------------
// extractKeywords tests
// ---------------------------------------------------------------------------

func TestExtractKeywords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "extracts top keywords from normal text",
			content: "golang golang golang programming programming testing",
			want:    []string{"golang", "programming", "testing"},
		},
		{
			name:    "returns empty slice for empty text",
			content: "",
			want:    []string{},
		},
		{
			name: "filters out stop words",
			content: "the and but with from this that was are " +
				"have been not will would could should",
			want: []string{},
		},
		{
			name:    "filters words shorter than 3 characters",
			content: "go is an ok it do no",
			want:    []string{},
		},
		{
			name:    "limits to 5 keywords maximum",
			content: "alpha alpha beta beta gamma gamma delta delta epsilon epsilon zeta zeta eta eta",
			want:    []string{"alpha", "beta", "delta", "epsilon", "eta"},
		},
		{
			name:    "handles punctuation stripping",
			content: "hello, world! hello. world? testing: testing; programming (code) [test]",
			want:    []string{"hello", "testing", "world", "code", "programming"},
		},
		{
			name:    "converts words to lowercase",
			content: "Golang GOLANG golang Programming PROGRAMMING",
			want:    []string{"golang", "programming"},
		},
		{
			name:    "returns fewer than 5 when not enough unique words",
			content: "kubernetes kubernetes docker",
			want:    []string{"kubernetes", "docker"},
		},
		{
			name:    "sorts by count descending then alphabetically",
			content: "zebra zebra apple apple mango",
			want:    []string{"apple", "zebra", "mango"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := extractKeywords(tt.content)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ---------------------------------------------------------------------------
// detectLanguage tests
// ---------------------------------------------------------------------------

func TestDetectLanguage(t *testing.T) {
	t.Parallel()

	t.Run("returns en for any input", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "en", detectLanguage("Hello world"))
	})

	t.Run("returns en for empty string", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "en", detectLanguage(""))
	})

	t.Run("returns en for non-English text", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "en", detectLanguage("Bonjour le monde"))
	})

	t.Run("returns en for unicode content", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "en", detectLanguage("日本語テキスト"))
	})
}

// ---------------------------------------------------------------------------
// sanitizeFilename tests
// ---------------------------------------------------------------------------

func TestSanitizeFilename(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "replaces double quotes with underscore",
			in:   `my "doc"`,
			want: "my _doc_",
		},
		{
			name: "replaces backslash with underscore",
			in:   `my\doc`,
			want: "my_doc",
		},
		{
			name: "replaces newline with underscore",
			in:   "my\ndoc",
			want: "my_doc",
		},
		{
			name: "normal title passes through unchanged",
			in:   "report",
			want: "report",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := sanitizeFilename(tt.in)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ---------------------------------------------------------------------------
// NewDocumentHandler constructor test
// ---------------------------------------------------------------------------

func TestNewDocumentHandler(t *testing.T) {
	t.Parallel()

	t.Run("returns non-nil handler with all fields set", func(t *testing.T) {
		t.Parallel()

		p := &mockPipeline{}
		r := &mockHandlerRepo{}
		l := testLogger()

		h := NewDocumentHandler(p, r, l)

		assert.NotNil(t, h)
		assert.Equal(t, p, h.pipeline)
		assert.Equal(t, r, h.repo)
		assert.Equal(t, l, h.logger)
	})
}

// ---------------------------------------------------------------------------
// List handler tests
// ---------------------------------------------------------------------------

func TestDocumentHandler_List(t *testing.T) {
	t.Parallel()

	t.Run("returns empty list when no documents exist", func(t *testing.T) {
		t.Parallel()

		repo := &mockHandlerRepo{
			listFn: func(_ context.Context, _ repository.DocumentListParams) (*repository.DocumentListResult, error) {
				return &repository.DocumentListResult{
					Documents: []model.Document{},
					Total:     0,
				}, nil
			},
		}
		h := newTestHandler(&mockPipeline{}, repo)

		req := httptest.NewRequest(http.MethodGet, "/api/documents", http.NoBody)
		rr := httptest.NewRecorder()

		h.List(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		body := decodeJSONBody(t, rr.Body)
		data, ok := body["data"].([]any)
		require.True(t, ok)
		assert.Empty(t, data)
		meta := body["meta"].(map[string]any)
		assert.InDelta(t, float64(0), meta["total"], 0.001)
	})

	t.Run("returns documents with tags and pagination metadata", func(t *testing.T) {
		t.Parallel()

		doc1 := *newTestDocument("list-uuid-1")
		doc1.ID = 10
		doc2 := *newTestDocument("list-uuid-2")
		doc2.ID = 20
		doc2.Title = "Second Doc"

		repo := &mockHandlerRepo{
			listFn: func(_ context.Context, _ repository.DocumentListParams) (*repository.DocumentListResult, error) {
				return &repository.DocumentListResult{
					Documents: []model.Document{doc1, doc2},
					Total:     42,
				}, nil
			},
			tagsForDocumentFn: func(_ context.Context, docID int64) ([]model.DocumentTag, error) {
				if docID == 10 {
					return []model.DocumentTag{{Tag: "go"}}, nil
				}
				return nil, nil
			},
		}
		h := newTestHandler(&mockPipeline{}, repo)

		req := httptest.NewRequest(http.MethodGet, "/api/documents?limit=10&offset=5", http.NoBody)
		rr := httptest.NewRecorder()

		h.List(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		body := decodeJSONBody(t, rr.Body)
		meta := body["meta"].(map[string]any)
		assert.InEpsilon(t, float64(42), meta["total"], 1e-9)
		assert.InEpsilon(t, float64(10), meta["limit"], 1e-9)
		assert.InEpsilon(t, float64(5), meta["offset"], 1e-9)

		data, ok := body["data"].([]any)
		require.True(t, ok)
		assert.Len(t, data, 2)

		first := data[0].(map[string]any)
		assert.Equal(t, "list-uuid-1", first["uuid"])
		tags := first["tags"].([]any)
		assert.Len(t, tags, 1)
		assert.Equal(t, "go", tags[0])
	})

	t.Run("passes filter parameters to repo", func(t *testing.T) {
		t.Parallel()

		var capturedParams repository.DocumentListParams
		repo := &mockHandlerRepo{
			listFn: func(_ context.Context, params repository.DocumentListParams) (*repository.DocumentListResult, error) {
				capturedParams = params
				return &repository.DocumentListResult{}, nil
			},
		}
		h := newTestHandler(&mockPipeline{}, repo)

		req := httptest.NewRequest(http.MethodGet, "/api/documents?file_type=pdf&status=indexed&q=test&sort=title&order=asc", http.NoBody)
		rr := httptest.NewRecorder()

		h.List(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "pdf", capturedParams.FileType)
		assert.Equal(t, "indexed", capturedParams.Status)
		assert.Equal(t, "test", capturedParams.Query)
		assert.Equal(t, "title", capturedParams.OrderBy)
		assert.Equal(t, "asc", capturedParams.OrderDir)
	})

	t.Run("returns 500 when repo returns error", func(t *testing.T) {
		t.Parallel()

		repo := &mockHandlerRepo{
			listFn: func(_ context.Context, _ repository.DocumentListParams) (*repository.DocumentListResult, error) {
				return nil, errors.New("database down")
			},
		}
		h := newTestHandler(&mockPipeline{}, repo)

		req := httptest.NewRequest(http.MethodGet, "/api/documents", http.NoBody)
		rr := httptest.NewRecorder()

		h.List(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		body := decodeJSONBody(t, rr.Body)
		assert.Equal(t, "failed to list documents", body["message"])
	})

	t.Run("continues when TagsForDocument fails for a document", func(t *testing.T) {
		t.Parallel()

		doc := *newTestDocument("tags-fail-uuid")
		repo := &mockHandlerRepo{
			listFn: func(_ context.Context, _ repository.DocumentListParams) (*repository.DocumentListResult, error) {
				return &repository.DocumentListResult{
					Documents: []model.Document{doc},
					Total:     1,
				}, nil
			},
			tagsForDocumentFn: func(_ context.Context, _ int64) ([]model.DocumentTag, error) {
				return nil, errors.New("tags error")
			},
		}
		h := newTestHandler(&mockPipeline{}, repo)

		req := httptest.NewRequest(http.MethodGet, "/api/documents", http.NoBody)
		rr := httptest.NewRecorder()

		h.List(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		body := decodeJSONBody(t, rr.Body)
		data := body["data"].([]any)
		assert.Len(t, data, 1)
	})
}

// ---------------------------------------------------------------------------
// Restore handler tests
// ---------------------------------------------------------------------------

func TestDocumentHandler_Restore(t *testing.T) {
	t.Parallel()

	t.Run("restores a soft-deleted document successfully", func(t *testing.T) {
		t.Parallel()

		deletedDoc := newTestDocument("restore-uuid")
		deletedDoc.DeletedAt = sql.NullTime{Time: time.Now(), Valid: true}

		restoredDoc := newTestDocument("restore-uuid")
		restoredDoc.DeletedAt = sql.NullTime{Valid: false}

		repo := &mockHandlerRepo{
			findByUUIDIncludingDeletedFn: func(_ context.Context, _ string) (*model.Document, error) {
				return deletedDoc, nil
			},
			restoreFn: func(_ context.Context, _ int64) error {
				return nil
			},
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
				return restoredDoc, nil
			},
			tagsForDocumentFn: func(_ context.Context, _ int64) ([]model.DocumentTag, error) {
				return []model.DocumentTag{{Tag: "restored"}}, nil
			},
		}
		h := newTestHandler(&mockPipeline{}, repo)

		req := httptest.NewRequest(http.MethodPost, "/api/documents/restore-uuid/restore", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "restore-uuid"})
		rr := httptest.NewRecorder()

		h.Restore(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		body := decodeJSONBody(t, rr.Body)
		assert.Equal(t, "Document restored successfully.", body["message"])
		data := body["data"].(map[string]any)
		assert.Equal(t, "restore-uuid", data["uuid"])
	})

	t.Run("returns 404 when document not found", func(t *testing.T) {
		t.Parallel()

		repo := &mockHandlerRepo{
			findByUUIDIncludingDeletedFn: func(_ context.Context, _ string) (*model.Document, error) {
				return nil, sql.ErrNoRows
			},
		}
		h := newTestHandler(&mockPipeline{}, repo)

		req := httptest.NewRequest(http.MethodPost, "/api/documents/missing/restore", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "missing"})
		rr := httptest.NewRecorder()

		h.Restore(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
		body := decodeJSONBody(t, rr.Body)
		assert.Equal(t, "document not found", body["message"])
	})

	t.Run("returns 500 when FindByUUIDIncludingDeleted fails with non-ErrNoRows", func(t *testing.T) {
		t.Parallel()

		repo := &mockHandlerRepo{
			findByUUIDIncludingDeletedFn: func(_ context.Context, _ string) (*model.Document, error) {
				return nil, errors.New("database error")
			},
		}
		h := newTestHandler(&mockPipeline{}, repo)

		req := httptest.NewRequest(http.MethodPost, "/api/documents/err/restore", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "err"})
		rr := httptest.NewRecorder()

		h.Restore(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		body := decodeJSONBody(t, rr.Body)
		assert.Equal(t, "failed to find document", body["message"])
	})

	t.Run("returns 400 when document is not deleted", func(t *testing.T) {
		t.Parallel()

		doc := newTestDocument("not-deleted-uuid")
		doc.DeletedAt = sql.NullTime{Valid: false}

		repo := &mockHandlerRepo{
			findByUUIDIncludingDeletedFn: func(_ context.Context, _ string) (*model.Document, error) {
				return doc, nil
			},
		}
		h := newTestHandler(&mockPipeline{}, repo)

		req := httptest.NewRequest(http.MethodPost, "/api/documents/not-deleted-uuid/restore", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "not-deleted-uuid"})
		rr := httptest.NewRecorder()

		h.Restore(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		body := decodeJSONBody(t, rr.Body)
		assert.Equal(t, "document is not deleted", body["message"])
	})

	t.Run("returns 500 when Restore repo call fails", func(t *testing.T) {
		t.Parallel()

		doc := newTestDocument("restore-fail-uuid")
		doc.DeletedAt = sql.NullTime{Time: time.Now(), Valid: true}

		repo := &mockHandlerRepo{
			findByUUIDIncludingDeletedFn: func(_ context.Context, _ string) (*model.Document, error) {
				return doc, nil
			},
			restoreFn: func(_ context.Context, _ int64) error {
				return errors.New("restore failed")
			},
		}
		h := newTestHandler(&mockPipeline{}, repo)

		req := httptest.NewRequest(http.MethodPost, "/api/documents/restore-fail-uuid/restore", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "restore-fail-uuid"})
		rr := httptest.NewRecorder()

		h.Restore(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		body := decodeJSONBody(t, rr.Body)
		assert.Equal(t, "failed to restore document", body["message"])
	})

	t.Run("returns 500 when re-fetch after restore fails", func(t *testing.T) {
		t.Parallel()

		doc := newTestDocument("refetch-fail-uuid")
		doc.DeletedAt = sql.NullTime{Time: time.Now(), Valid: true}

		repo := &mockHandlerRepo{
			findByUUIDIncludingDeletedFn: func(_ context.Context, _ string) (*model.Document, error) {
				return doc, nil
			},
			restoreFn: func(_ context.Context, _ int64) error {
				return nil
			},
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
				return nil, errors.New("re-fetch failed")
			},
		}
		h := newTestHandler(&mockPipeline{}, repo)

		req := httptest.NewRequest(http.MethodPost, "/api/documents/refetch-fail-uuid/restore", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "refetch-fail-uuid"})
		rr := httptest.NewRecorder()

		h.Restore(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		body := decodeJSONBody(t, rr.Body)
		assert.Equal(t, "document restored but failed to fetch updated record", body["message"])
	})
}

// ---------------------------------------------------------------------------
// Purge handler tests
// ---------------------------------------------------------------------------

func TestDocumentHandler_Purge(t *testing.T) {
	t.Parallel()

	t.Run("purges document successfully without file", func(t *testing.T) {
		t.Parallel()

		doc := newTestDocument("purge-uuid")

		repo := &mockHandlerRepo{
			findByUUIDIncludingDeletedFn: func(_ context.Context, _ string) (*model.Document, error) {
				return doc, nil
			},
			purgeSingleFn: func(_ context.Context, _ int64) (string, error) {
				return "", nil
			},
		}
		h := newTestHandler(&mockPipeline{}, repo)

		req := httptest.NewRequest(http.MethodDelete, "/api/documents/purge-uuid/purge", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "purge-uuid"})
		rr := httptest.NewRecorder()

		h.Purge(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		body := decodeJSONBody(t, rr.Body)
		assert.Equal(t, "Document permanently deleted.", body["message"])
	})

	t.Run("purges document and removes file from disk", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		subDir := filepath.Join(tmpDir, "markdown")
		require.NoError(t, os.MkdirAll(subDir, 0o750))
		filePath := filepath.Join(subDir, "to-delete.md")
		require.NoError(t, os.WriteFile(filePath, []byte("delete me"), 0o600))

		doc := newTestDocument("purge-file-uuid")

		repo := &mockHandlerRepo{
			findByUUIDIncludingDeletedFn: func(_ context.Context, _ string) (*model.Document, error) {
				return doc, nil
			},
			purgeSingleFn: func(_ context.Context, _ int64) (string, error) {
				return "markdown/to-delete.md", nil
			},
		}
		h := newTestHandler(&mockPipeline{storagePathVal: tmpDir}, repo)

		req := httptest.NewRequest(http.MethodDelete, "/api/documents/purge-file-uuid/purge", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "purge-file-uuid"})
		rr := httptest.NewRecorder()

		h.Purge(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		_, err := os.Stat(filePath)
		assert.True(t, os.IsNotExist(err), "file should be deleted from disk")
	})

	t.Run("returns 404 when document not found", func(t *testing.T) {
		t.Parallel()

		repo := &mockHandlerRepo{
			findByUUIDIncludingDeletedFn: func(_ context.Context, _ string) (*model.Document, error) {
				return nil, sql.ErrNoRows
			},
		}
		h := newTestHandler(&mockPipeline{}, repo)

		req := httptest.NewRequest(http.MethodDelete, "/api/documents/missing/purge", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "missing"})
		rr := httptest.NewRecorder()

		h.Purge(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("returns 500 when FindByUUIDIncludingDeleted errors", func(t *testing.T) {
		t.Parallel()

		repo := &mockHandlerRepo{
			findByUUIDIncludingDeletedFn: func(_ context.Context, _ string) (*model.Document, error) {
				return nil, errors.New("db error")
			},
		}
		h := newTestHandler(&mockPipeline{}, repo)

		req := httptest.NewRequest(http.MethodDelete, "/api/documents/err/purge", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "err"})
		rr := httptest.NewRecorder()

		h.Purge(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("returns 500 when PurgeSingle fails", func(t *testing.T) {
		t.Parallel()

		doc := newTestDocument("purge-err-uuid")
		repo := &mockHandlerRepo{
			findByUUIDIncludingDeletedFn: func(_ context.Context, _ string) (*model.Document, error) {
				return doc, nil
			},
			purgeSingleFn: func(_ context.Context, _ int64) (string, error) {
				return "", errors.New("purge failed")
			},
		}
		h := newTestHandler(&mockPipeline{}, repo)

		req := httptest.NewRequest(http.MethodDelete, "/api/documents/purge-err-uuid/purge", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "purge-err-uuid"})
		rr := httptest.NewRecorder()

		h.Purge(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		body := decodeJSONBody(t, rr.Body)
		assert.Equal(t, "failed to purge document", body["message"])
	})
}

// ---------------------------------------------------------------------------
// BulkPurge handler tests
// ---------------------------------------------------------------------------

func TestDocumentHandler_BulkPurge(t *testing.T) {
	t.Parallel()

	t.Run("purges documents with default 30 days threshold", func(t *testing.T) {
		t.Parallel()

		var capturedDuration time.Duration
		repo := &mockHandlerRepo{
			purgeSoftDeletedFn: func(_ context.Context, olderThan time.Duration) ([]repository.DocumentFilePath, error) {
				capturedDuration = olderThan
				return []repository.DocumentFilePath{
					{ID: 1, UUID: "a", FilePath: ""},
					{ID: 2, UUID: "b", FilePath: ""},
				}, nil
			},
		}
		h := newTestHandler(&mockPipeline{}, repo)

		req := httptest.NewRequest(http.MethodDelete, "/api/admin/documents/purge", http.NoBody)
		rr := httptest.NewRecorder()

		h.BulkPurge(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		body := decodeJSONBody(t, rr.Body)
		assert.Equal(t, "Purged 2 documents.", body["message"])
		assert.InEpsilon(t, float64(2), body["count"], 1e-9)
		assert.Equal(t, 30*24*time.Hour, capturedDuration)
	})

	t.Run("uses custom older_than_days parameter", func(t *testing.T) {
		t.Parallel()

		var capturedDuration time.Duration
		repo := &mockHandlerRepo{
			purgeSoftDeletedFn: func(_ context.Context, olderThan time.Duration) ([]repository.DocumentFilePath, error) {
				capturedDuration = olderThan
				return nil, nil
			},
		}
		h := newTestHandler(&mockPipeline{}, repo)

		req := httptest.NewRequest(http.MethodDelete, "/api/admin/documents/purge?older_than_days=7", http.NoBody)
		rr := httptest.NewRecorder()

		h.BulkPurge(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, 7*24*time.Hour, capturedDuration)
		body := decodeJSONBody(t, rr.Body)
		assert.Equal(t, "Purged 0 documents.", body["message"])
	})

	t.Run("returns 400 for negative older_than_days", func(t *testing.T) {
		t.Parallel()

		h := newTestHandler(&mockPipeline{}, &mockHandlerRepo{})

		req := httptest.NewRequest(http.MethodDelete, "/api/admin/documents/purge?older_than_days=-1", http.NoBody)
		rr := httptest.NewRecorder()

		h.BulkPurge(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		body := decodeJSONBody(t, rr.Body)
		assert.Equal(t, "older_than_days must be a non-negative integer", body["message"])
	})

	t.Run("returns 400 for non-integer older_than_days", func(t *testing.T) {
		t.Parallel()

		h := newTestHandler(&mockPipeline{}, &mockHandlerRepo{})

		req := httptest.NewRequest(http.MethodDelete, "/api/admin/documents/purge?older_than_days=abc", http.NoBody)
		rr := httptest.NewRecorder()

		h.BulkPurge(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("returns 500 when PurgeSoftDeleted fails", func(t *testing.T) {
		t.Parallel()

		repo := &mockHandlerRepo{
			purgeSoftDeletedFn: func(_ context.Context, _ time.Duration) ([]repository.DocumentFilePath, error) {
				return nil, errors.New("purge error")
			},
		}
		h := newTestHandler(&mockPipeline{}, repo)

		req := httptest.NewRequest(http.MethodDelete, "/api/admin/documents/purge", http.NoBody)
		rr := httptest.NewRecorder()

		h.BulkPurge(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		body := decodeJSONBody(t, rr.Body)
		assert.Equal(t, "failed to purge documents", body["message"])
	})

	t.Run("removes files from disk during bulk purge", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		subDir := filepath.Join(tmpDir, "pdf")
		require.NoError(t, os.MkdirAll(subDir, 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(subDir, "a.pdf"), []byte("aaa"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(subDir, "b.pdf"), []byte("bbb"), 0o600))

		repo := &mockHandlerRepo{
			purgeSoftDeletedFn: func(_ context.Context, _ time.Duration) ([]repository.DocumentFilePath, error) {
				return []repository.DocumentFilePath{
					{ID: 1, UUID: "a", FilePath: "pdf/a.pdf"},
					{ID: 2, UUID: "b", FilePath: "pdf/b.pdf"},
				}, nil
			},
		}
		h := newTestHandler(&mockPipeline{storagePathVal: tmpDir}, repo)

		req := httptest.NewRequest(http.MethodDelete, "/api/admin/documents/purge", http.NoBody)
		rr := httptest.NewRecorder()

		h.BulkPurge(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		_, err := os.Stat(filepath.Join(subDir, "a.pdf"))
		assert.True(t, os.IsNotExist(err))
		_, err = os.Stat(filepath.Join(subDir, "b.pdf"))
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("accepts zero days threshold", func(t *testing.T) {
		t.Parallel()

		var capturedDuration time.Duration
		repo := &mockHandlerRepo{
			purgeSoftDeletedFn: func(_ context.Context, olderThan time.Duration) ([]repository.DocumentFilePath, error) {
				capturedDuration = olderThan
				return nil, nil
			},
		}
		h := newTestHandler(&mockPipeline{}, repo)

		req := httptest.NewRequest(http.MethodDelete, "/api/admin/documents/purge?older_than_days=0", http.NoBody)
		rr := httptest.NewRecorder()

		h.BulkPurge(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, time.Duration(0), capturedDuration)
	})
}

// ---------------------------------------------------------------------------
// ListDeleted handler tests
// ---------------------------------------------------------------------------

func TestDocumentHandler_ListDeleted(t *testing.T) {
	t.Parallel()

	t.Run("returns deleted documents with pagination", func(t *testing.T) {
		t.Parallel()

		doc := *newTestDocument("deleted-uuid")
		doc.DeletedAt = sql.NullTime{Time: time.Now(), Valid: true}

		repo := &mockHandlerRepo{
			listDeletedFn: func(_ context.Context, _ int, _ int, _ *int64) ([]model.Document, int, error) {
				return []model.Document{doc}, 1, nil
			},
			tagsForDocumentFn: func(_ context.Context, _ int64) ([]model.DocumentTag, error) {
				return []model.DocumentTag{{Tag: "trash"}}, nil
			},
		}
		h := newTestHandler(&mockPipeline{}, repo)

		req := httptest.NewRequest(http.MethodGet, "/api/documents/trash?limit=10&offset=0", http.NoBody)
		rr := httptest.NewRecorder()

		h.ListDeleted(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		body := decodeJSONBody(t, rr.Body)
		meta := body["meta"].(map[string]any)
		assert.InEpsilon(t, float64(1), meta["total"], 1e-9)
		data := body["data"].([]any)
		assert.Len(t, data, 1)
		first := data[0].(map[string]any)
		assert.Equal(t, "deleted-uuid", first["uuid"])
	})

	t.Run("returns empty list when no deleted documents", func(t *testing.T) {
		t.Parallel()

		repo := &mockHandlerRepo{
			listDeletedFn: func(_ context.Context, _ int, _ int, _ *int64) ([]model.Document, int, error) {
				return []model.Document{}, 0, nil
			},
		}
		h := newTestHandler(&mockPipeline{}, repo)

		req := httptest.NewRequest(http.MethodGet, "/api/documents/trash", http.NoBody)
		rr := httptest.NewRecorder()

		h.ListDeleted(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		body := decodeJSONBody(t, rr.Body)
		meta2 := body["meta"].(map[string]any)
		assert.InDelta(t, float64(0), meta2["total"], 0.001)
		data := body["data"].([]any)
		assert.Empty(t, data)
	})

	t.Run("returns 500 when ListDeleted repo fails", func(t *testing.T) {
		t.Parallel()

		repo := &mockHandlerRepo{
			listDeletedFn: func(_ context.Context, _ int, _ int, _ *int64) ([]model.Document, int, error) {
				return nil, 0, errors.New("db error")
			},
		}
		h := newTestHandler(&mockPipeline{}, repo)

		req := httptest.NewRequest(http.MethodGet, "/api/documents/trash", http.NoBody)
		rr := httptest.NewRecorder()

		h.ListDeleted(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		body := decodeJSONBody(t, rr.Body)
		assert.Equal(t, "failed to list deleted documents", body["message"])
	})

	t.Run("continues when tags fail for a deleted document", func(t *testing.T) {
		t.Parallel()

		doc := *newTestDocument("tags-fail-del")
		repo := &mockHandlerRepo{
			listDeletedFn: func(_ context.Context, _ int, _ int, _ *int64) ([]model.Document, int, error) {
				return []model.Document{doc}, 1, nil
			},
			tagsForDocumentFn: func(_ context.Context, _ int64) ([]model.DocumentTag, error) {
				return nil, errors.New("tags unavailable")
			},
		}
		h := newTestHandler(&mockPipeline{}, repo)

		req := httptest.NewRequest(http.MethodGet, "/api/documents/trash", http.NoBody)
		rr := httptest.NewRecorder()

		h.ListDeleted(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		body := decodeJSONBody(t, rr.Body)
		data := body["data"].([]any)
		assert.Len(t, data, 1)
	})
}

// ---------------------------------------------------------------------------
// Show handler — success with tags
// ---------------------------------------------------------------------------

func TestDocumentHandler_Show_Success(t *testing.T) {
	t.Parallel()

	t.Run("returns document with tags", func(t *testing.T) {
		t.Parallel()

		doc := newTestDocument("show-uuid")
		p := &mockPipeline{
			findByUUIDFn: func(_ context.Context, uuid string) (*model.Document, error) {
				if uuid == "show-uuid" {
					return doc, nil
				}
				return nil, service.ErrNotFound
			},
		}
		repo := &mockHandlerRepo{
			tagsForDocumentFn: func(_ context.Context, _ int64) ([]model.DocumentTag, error) {
				return []model.DocumentTag{{Tag: "api"}, {Tag: "test"}}, nil
			},
		}
		h := newTestHandler(p, repo)

		req := httptest.NewRequest(http.MethodGet, "/api/documents/show-uuid", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "show-uuid"})
		rr := httptest.NewRecorder()

		h.Show(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		body := decodeJSONBody(t, rr.Body)
		data := body["data"].(map[string]any)
		assert.Equal(t, "show-uuid", data["uuid"])
		tags := data["tags"].([]any)
		assert.Len(t, tags, 2)
		assert.Equal(t, "api", tags[0])
		assert.Equal(t, "test", tags[1])
	})
}

// ---------------------------------------------------------------------------
// Ownership scoping tests
// ---------------------------------------------------------------------------

func TestDocumentHandler_List_OwnershipScoping(t *testing.T) {
	t.Parallel()

	t.Run("non-admin user sets OwnerOrPublic", func(t *testing.T) {
		t.Parallel()

		var capturedParams repository.DocumentListParams
		repo := &mockHandlerRepo{
			listFn: func(_ context.Context, params repository.DocumentListParams) (*repository.DocumentListResult, error) {
				capturedParams = params
				return &repository.DocumentListResult{Documents: nil, Total: 0}, nil
			},
		}
		h := newTestHandler(&mockPipeline{}, repo)

		user := &model.User{ID: 42, IsAdmin: false}
		req := httptest.NewRequest(http.MethodGet, "/api/documents", http.NoBody)
		ctx := context.WithValue(req.Context(), authmiddleware.UserContextKey, user)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		h.List(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		require.NotNil(t, capturedParams.OwnerOrPublic, "OwnerOrPublic should be set for non-admin")
		assert.Equal(t, int64(42), *capturedParams.OwnerOrPublic)
	})

	t.Run("admin user does not set OwnerOrPublic", func(t *testing.T) {
		t.Parallel()

		var capturedParams repository.DocumentListParams
		repo := &mockHandlerRepo{
			listFn: func(_ context.Context, params repository.DocumentListParams) (*repository.DocumentListResult, error) {
				capturedParams = params
				return &repository.DocumentListResult{Documents: nil, Total: 0}, nil
			},
		}
		h := newTestHandler(&mockPipeline{}, repo)

		user := &model.User{ID: 1, IsAdmin: true}
		req := httptest.NewRequest(http.MethodGet, "/api/documents", http.NoBody)
		ctx := context.WithValue(req.Context(), authmiddleware.UserContextKey, user)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		h.List(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Nil(t, capturedParams.OwnerOrPublic, "OwnerOrPublic should be nil for admin")
	})
}

func TestDocumentHandler_Show_OwnershipScoping(t *testing.T) {
	t.Parallel()

	t.Run("non-admin can see own document", func(t *testing.T) {
		t.Parallel()

		doc := newTestDocument("own-doc")
		doc.IsPublic = false
		doc.UserID = sql.NullInt64{Int64: 42, Valid: true}

		p := &mockPipeline{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
				return doc, nil
			},
		}
		repo := &mockHandlerRepo{}
		h := newTestHandler(p, repo)

		user := &model.User{ID: 42, IsAdmin: false}
		req := httptest.NewRequest(http.MethodGet, "/api/documents/own-doc", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "own-doc"})
		ctx := context.WithValue(req.Context(), authmiddleware.UserContextKey, user)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		h.Show(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("non-admin can see public document", func(t *testing.T) {
		t.Parallel()

		doc := newTestDocument("public-doc")
		doc.IsPublic = true
		doc.UserID = sql.NullInt64{Int64: 99, Valid: true}

		p := &mockPipeline{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
				return doc, nil
			},
		}
		repo := &mockHandlerRepo{}
		h := newTestHandler(p, repo)

		user := &model.User{ID: 42, IsAdmin: false}
		req := httptest.NewRequest(http.MethodGet, "/api/documents/public-doc", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "public-doc"})
		ctx := context.WithValue(req.Context(), authmiddleware.UserContextKey, user)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		h.Show(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("non-admin gets 404 for other user private document", func(t *testing.T) {
		t.Parallel()

		doc := newTestDocument("other-doc")
		doc.IsPublic = false
		doc.UserID = sql.NullInt64{Int64: 99, Valid: true}

		p := &mockPipeline{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
				return doc, nil
			},
		}
		repo := &mockHandlerRepo{}
		h := newTestHandler(p, repo)

		user := &model.User{ID: 42, IsAdmin: false}
		req := httptest.NewRequest(http.MethodGet, "/api/documents/other-doc", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "other-doc"})
		ctx := context.WithValue(req.Context(), authmiddleware.UserContextKey, user)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		h.Show(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
		body := decodeJSONBody(t, rr.Body)
		assert.Equal(t, "document not found", body["message"])
	})

	t.Run("non-admin gets 404 for unowned private document", func(t *testing.T) {
		t.Parallel()

		doc := newTestDocument("unowned-doc")
		doc.IsPublic = false
		doc.UserID = sql.NullInt64{Valid: false} // no owner

		p := &mockPipeline{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
				return doc, nil
			},
		}
		repo := &mockHandlerRepo{}
		h := newTestHandler(p, repo)

		user := &model.User{ID: 42, IsAdmin: false}
		req := httptest.NewRequest(http.MethodGet, "/api/documents/unowned-doc", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "unowned-doc"})
		ctx := context.WithValue(req.Context(), authmiddleware.UserContextKey, user)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		h.Show(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})
}

func TestDocumentHandler_Upload_SetsUserID(t *testing.T) {
	t.Parallel()

	t.Run("sets UserID from authenticated user", func(t *testing.T) {
		t.Parallel()

		var capturedParams service.UploadDocumentParams
		p := &mockPipeline{
			uploadFn: func(_ context.Context, params service.UploadDocumentParams) (*model.Document, error) {
				capturedParams = params
				return newTestDocument("uploaded"), nil
			},
		}
		repo := &mockHandlerRepo{}
		h := newTestHandler(p, repo)

		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		part, _ := writer.CreateFormFile("file", "test.md")
		_, _ = part.Write([]byte("# Hello"))
		_ = writer.Close()

		user := &model.User{ID: 77, IsAdmin: false}
		req := httptest.NewRequest(http.MethodPost, "/api/documents", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		ctx := context.WithValue(req.Context(), authmiddleware.UserContextKey, user)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		h.Upload(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
		require.NotNil(t, capturedParams.UserID, "UserID should be set from context")
		assert.Equal(t, int64(77), *capturedParams.UserID)
	})
}

func TestDocumentHandler_ListDeleted_OwnershipScoping(t *testing.T) {
	t.Parallel()

	t.Run("non-admin passes userID to repo", func(t *testing.T) {
		t.Parallel()

		var capturedUserID *int64
		repo := &mockHandlerRepo{
			listDeletedFn: func(_ context.Context, _, _ int, userID *int64) ([]model.Document, int, error) {
				capturedUserID = userID
				return nil, 0, nil
			},
		}
		h := newTestHandler(&mockPipeline{}, repo)

		user := &model.User{ID: 42, IsAdmin: false}
		req := httptest.NewRequest(http.MethodGet, "/api/documents/trash", http.NoBody)
		ctx := context.WithValue(req.Context(), authmiddleware.UserContextKey, user)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		h.ListDeleted(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		require.NotNil(t, capturedUserID, "userID should be set for non-admin")
		assert.Equal(t, int64(42), *capturedUserID)
	})

	t.Run("admin passes nil userID to repo", func(t *testing.T) {
		t.Parallel()

		var capturedUserID *int64
		called := false
		repo := &mockHandlerRepo{
			listDeletedFn: func(_ context.Context, _, _ int, userID *int64) ([]model.Document, int, error) {
				capturedUserID = userID
				called = true
				return nil, 0, nil
			},
		}
		h := newTestHandler(&mockPipeline{}, repo)

		user := &model.User{ID: 1, IsAdmin: true}
		req := httptest.NewRequest(http.MethodGet, "/api/documents/trash", http.NoBody)
		ctx := context.WithValue(req.Context(), authmiddleware.UserContextKey, user)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		h.ListDeleted(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.True(t, called, "listDeletedFn should have been called")
		assert.Nil(t, capturedUserID, "userID should be nil for admin")
	})
}
