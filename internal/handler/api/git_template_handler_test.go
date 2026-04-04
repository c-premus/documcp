package api

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c-premus/documcp/internal/archive"
	"github.com/c-premus/documcp/internal/model"
	"github.com/c-premus/documcp/internal/queue"
)

// ---------------------------------------------------------------------------
// slugifyTemplateName tests
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// toGitTemplateResponse tests
// ---------------------------------------------------------------------------

func TestToGitTemplateResponse(t *testing.T) {
	t.Parallel()

	t.Run("maps all required fields", func(t *testing.T) {
		t.Parallel()

		gt := &model.GitTemplate{
			UUID:           "uuid-123",
			Name:           "My Template",
			Slug:           "my-template",
			RepositoryURL:  "https://github.com/user/repo",
			Branch:         "main",
			IsPublic:       true,
			Status:         "synced",
			FileCount:      10,
			TotalSizeBytes: 2048,
		}

		resp := toGitTemplateResponse(gt)

		if resp.UUID != "uuid-123" {
			t.Errorf("UUID = %q, want uuid-123", resp.UUID)
		}
		if resp.Name != "My Template" {
			t.Errorf("Name = %q, want My Template", resp.Name)
		}
		if resp.Slug != "my-template" {
			t.Errorf("Slug = %q, want my-template", resp.Slug)
		}
		if resp.RepositoryURL != "https://github.com/user/repo" {
			t.Errorf("RepositoryURL = %q, want https://github.com/user/repo", resp.RepositoryURL)
		}
		if resp.Branch != "main" {
			t.Errorf("Branch = %q, want main", resp.Branch)
		}
		if !resp.IsPublic {
			t.Error("IsPublic = false, want true")
		}
		if resp.Status != "synced" {
			t.Errorf("Status = %q, want synced", resp.Status)
		}
		if resp.FileCount != 10 {
			t.Errorf("FileCount = %d, want 10", resp.FileCount)
		}
		if resp.TotalSizeBytes != 2048 {
			t.Errorf("TotalSizeBytes = %d, want 2048", resp.TotalSizeBytes)
		}
	})

	t.Run("maps optional NullString fields when valid", func(t *testing.T) {
		t.Parallel()

		gt := &model.GitTemplate{
			UUID:          "uuid-1",
			Name:          "Test",
			Slug:          "test",
			RepositoryURL: "https://example.com",
			Branch:        "main",
			Status:        "synced",
			Description:   sql.NullString{String: "A description", Valid: true},
			Category:      sql.NullString{String: "devops", Valid: true},
			ErrorMessage:  sql.NullString{String: "sync failed", Valid: true},
			LastCommitSHA: sql.NullString{String: "abc123def", Valid: true},
		}

		resp := toGitTemplateResponse(gt)

		if resp.Description != "A description" {
			t.Errorf("Description = %q, want A description", resp.Description)
		}
		if resp.Category != "devops" {
			t.Errorf("Category = %q, want devops", resp.Category)
		}
		if resp.ErrorMessage != "sync failed" {
			t.Errorf("ErrorMessage = %q, want sync failed", resp.ErrorMessage)
		}
		if resp.LastCommitSHA != "abc123def" {
			t.Errorf("LastCommitSHA = %q, want abc123def", resp.LastCommitSHA)
		}
	})

	t.Run("null optional fields produce empty strings", func(t *testing.T) {
		t.Parallel()

		gt := &model.GitTemplate{
			UUID:          "uuid-2",
			Name:          "Minimal",
			Slug:          "minimal",
			RepositoryURL: "https://example.com",
			Branch:        "main",
			Status:        "pending",
		}

		resp := toGitTemplateResponse(gt)

		if resp.Description != "" {
			t.Errorf("Description = %q, want empty", resp.Description)
		}
		if resp.Category != "" {
			t.Errorf("Category = %q, want empty", resp.Category)
		}
		if resp.ErrorMessage != "" {
			t.Errorf("ErrorMessage = %q, want empty", resp.ErrorMessage)
		}
		if resp.LastSyncedAt != "" {
			t.Errorf("LastSyncedAt = %q, want empty", resp.LastSyncedAt)
		}
		if resp.LastCommitSHA != "" {
			t.Errorf("LastCommitSHA = %q, want empty", resp.LastCommitSHA)
		}
		if resp.CreatedAt != "" {
			t.Errorf("CreatedAt = %q, want empty", resp.CreatedAt)
		}
		if resp.UpdatedAt != "" {
			t.Errorf("UpdatedAt = %q, want empty", resp.UpdatedAt)
		}
	})

	t.Run("timestamps formatted as RFC3339 when valid", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC)
		gt := &model.GitTemplate{
			UUID:          "uuid-3",
			Name:          "Timed",
			Slug:          "timed",
			RepositoryURL: "https://example.com",
			Branch:        "main",
			Status:        "synced",
			LastSyncedAt:  sql.NullTime{Time: now, Valid: true},
			CreatedAt:     sql.NullTime{Time: now, Valid: true},
			UpdatedAt:     sql.NullTime{Time: now, Valid: true},
		}

		resp := toGitTemplateResponse(gt)
		want := "2025-06-15T14:30:00Z"

		if resp.LastSyncedAt != want {
			t.Errorf("LastSyncedAt = %q, want %q", resp.LastSyncedAt, want)
		}
		if resp.CreatedAt != want {
			t.Errorf("CreatedAt = %q, want %q", resp.CreatedAt, want)
		}
		if resp.UpdatedAt != want {
			t.Errorf("UpdatedAt = %q, want %q", resp.UpdatedAt, want)
		}
	})

	t.Run("tags parsed from JSON when valid", func(t *testing.T) {
		t.Parallel()

		gt := &model.GitTemplate{
			UUID:          "uuid-4",
			Name:          "Tagged",
			Slug:          "tagged",
			RepositoryURL: "https://example.com",
			Branch:        "main",
			Status:        "synced",
			Tags:          sql.NullString{String: `["go","docker","k8s"]`, Valid: true},
		}

		resp := toGitTemplateResponse(gt)

		if len(resp.Tags) != 3 {
			t.Fatalf("Tags length = %d, want 3", len(resp.Tags))
		}
		if resp.Tags[0] != "go" || resp.Tags[1] != "docker" || resp.Tags[2] != "k8s" {
			t.Errorf("Tags = %v, want [go docker k8s]", resp.Tags)
		}
	})

	t.Run("null tags produce empty slice not nil", func(t *testing.T) {
		t.Parallel()

		gt := &model.GitTemplate{
			UUID:          "uuid-5",
			Name:          "No Tags",
			Slug:          "no-tags",
			RepositoryURL: "https://example.com",
			Branch:        "main",
			Status:        "synced",
		}

		resp := toGitTemplateResponse(gt)

		if resp.Tags == nil {
			t.Fatal("Tags should be empty slice, not nil")
		}
		if len(resp.Tags) != 0 {
			t.Errorf("Tags length = %d, want 0", len(resp.Tags))
		}
	})

	t.Run("invalid tags JSON falls back to empty slice", func(t *testing.T) {
		t.Parallel()

		gt := &model.GitTemplate{
			UUID:          "uuid-6",
			Name:          "Bad Tags",
			Slug:          "bad-tags",
			RepositoryURL: "https://example.com",
			Branch:        "main",
			Status:        "synced",
			Tags:          sql.NullString{String: "not json", Valid: true},
		}

		resp := toGitTemplateResponse(gt)

		if resp.Tags == nil {
			t.Fatal("Tags should be empty slice, not nil")
		}
		if len(resp.Tags) != 0 {
			t.Errorf("Tags length = %d, want 0 (fallback for invalid JSON)", len(resp.Tags))
		}
	})

	t.Run("zero file count and size", func(t *testing.T) {
		t.Parallel()

		gt := &model.GitTemplate{
			UUID:          "uuid-7",
			Name:          "Empty",
			Slug:          "empty",
			RepositoryURL: "https://example.com",
			Branch:        "main",
			Status:        "pending",
			FileCount:     0,
		}

		resp := toGitTemplateResponse(gt)

		if resp.FileCount != 0 {
			t.Errorf("FileCount = %d, want 0", resp.FileCount)
		}
		if resp.TotalSizeBytes != 0 {
			t.Errorf("TotalSizeBytes = %d, want 0", resp.TotalSizeBytes)
		}
	})
}

// ---------------------------------------------------------------------------
// buildTemplateArchiveZip tests
// ---------------------------------------------------------------------------

func TestBuildTemplateArchiveZip(t *testing.T) {
	t.Parallel()

	t.Run("creates valid zip with single entry", func(t *testing.T) {
		t.Parallel()

		entries := []archive.Entry{
			{Path: "README.md", Content: "# Hello"},
		}

		var buf bytes.Buffer
		err := archive.BuildZip(&buf, entries)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		if err != nil {
			t.Fatalf("opening zip: %v", err)
		}

		if len(reader.File) != 1 {
			t.Fatalf("zip file count = %d, want 1", len(reader.File))
		}
		if reader.File[0].Name != "README.md" {
			t.Errorf("file name = %q, want README.md", reader.File[0].Name)
		}

		rc, err := reader.File[0].Open()
		if err != nil {
			t.Fatalf("opening file in zip: %v", err)
		}
		defer func() { _ = rc.Close() }()

		data, _ := io.ReadAll(rc)
		if string(data) != "# Hello" {
			t.Errorf("file content = %q, want %q", string(data), "# Hello")
		}
	})

	t.Run("creates zip with multiple entries", func(t *testing.T) {
		t.Parallel()

		entries := []archive.Entry{
			{Path: "file1.txt", Content: "content1"},
			{Path: "dir/file2.txt", Content: "content2"},
			{Path: "dir/sub/file3.txt", Content: "content3"},
		}

		var buf bytes.Buffer
		err := archive.BuildZip(&buf, entries)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		if err != nil {
			t.Fatalf("opening zip: %v", err)
		}

		if len(reader.File) != 3 {
			t.Fatalf("zip file count = %d, want 3", len(reader.File))
		}

		wantNames := []string{"file1.txt", "dir/file2.txt", "dir/sub/file3.txt"}
		for i, f := range reader.File {
			if f.Name != wantNames[i] {
				t.Errorf("file[%d] name = %q, want %q", i, f.Name, wantNames[i])
			}
		}
	})

	t.Run("creates valid zip with empty entries", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		err := archive.BuildZip(&buf, []archive.Entry{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		if err != nil {
			t.Fatalf("opening empty zip: %v", err)
		}

		if len(reader.File) != 0 {
			t.Errorf("zip file count = %d, want 0", len(reader.File))
		}
	})

	t.Run("handles entry with empty content", func(t *testing.T) {
		t.Parallel()

		entries := []archive.Entry{
			{Path: "empty.txt", Content: ""},
		}

		var buf bytes.Buffer
		err := archive.BuildZip(&buf, entries)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		if err != nil {
			t.Fatalf("opening zip: %v", err)
		}

		rc, _ := reader.File[0].Open()
		defer func() { _ = rc.Close() }()

		data, _ := io.ReadAll(rc)
		if len(data) != 0 {
			t.Errorf("file content length = %d, want 0", len(data))
		}
	})

	t.Run("preserves large content", func(t *testing.T) {
		t.Parallel()

		bigContent := strings.Repeat("x", 100_000)
		entries := []archive.Entry{
			{Path: "big.txt", Content: bigContent},
		}

		var buf bytes.Buffer
		err := archive.BuildZip(&buf, entries)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		if err != nil {
			t.Fatalf("opening zip: %v", err)
		}

		rc, _ := reader.File[0].Open()
		defer func() { _ = rc.Close() }()

		data, _ := io.ReadAll(rc)
		if len(data) != 100_000 {
			t.Errorf("file content length = %d, want 100000", len(data))
		}
	})
}

// ---------------------------------------------------------------------------
// buildTemplateArchiveTarGz tests
// ---------------------------------------------------------------------------

func TestBuildTemplateArchiveTarGz(t *testing.T) {
	t.Parallel()

	t.Run("creates valid tar.gz with single entry", func(t *testing.T) {
		t.Parallel()

		entries := []archive.Entry{
			{Path: "README.md", Content: "# Hello"},
		}

		var buf bytes.Buffer
		err := archive.BuildTarGz(&buf, entries)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		gr, err := gzip.NewReader(&buf)
		if err != nil {
			t.Fatalf("opening gzip: %v", err)
		}
		defer func() { _ = gr.Close() }()

		tr := tar.NewReader(gr)
		hdr, err := tr.Next()
		if err != nil {
			t.Fatalf("reading tar header: %v", err)
		}

		if hdr.Name != "README.md" {
			t.Errorf("file name = %q, want README.md", hdr.Name)
		}
		if hdr.Mode != 0o600 {
			t.Errorf("file mode = %o, want 0o600", hdr.Mode)
		}
		if hdr.Size != int64(len("# Hello")) {
			t.Errorf("file size = %d, want %d", hdr.Size, len("# Hello"))
		}

		data, _ := io.ReadAll(tr)
		if string(data) != "# Hello" {
			t.Errorf("file content = %q, want %q", string(data), "# Hello")
		}

		// No more entries.
		_, err = tr.Next()
		if !errors.Is(err, io.EOF) {
			t.Errorf("expected EOF after single entry, got %v", err)
		}
	})

	t.Run("creates tar.gz with multiple entries", func(t *testing.T) {
		t.Parallel()

		entries := []archive.Entry{
			{Path: "file1.txt", Content: "content1"},
			{Path: "dir/file2.txt", Content: "content2"},
		}

		var buf bytes.Buffer
		err := archive.BuildTarGz(&buf, entries)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		gr, err := gzip.NewReader(&buf)
		if err != nil {
			t.Fatalf("opening gzip: %v", err)
		}
		defer func() { _ = gr.Close() }()

		tr := tar.NewReader(gr)

		wantNames := []string{"file1.txt", "dir/file2.txt"}
		wantContents := []string{"content1", "content2"}

		for i := range wantNames {
			hdr, err := tr.Next()
			if err != nil {
				t.Fatalf("reading tar entry %d: %v", i, err)
			}
			if hdr.Name != wantNames[i] {
				t.Errorf("entry[%d] name = %q, want %q", i, hdr.Name, wantNames[i])
			}
			data, _ := io.ReadAll(tr)
			if string(data) != wantContents[i] {
				t.Errorf("entry[%d] content = %q, want %q", i, string(data), wantContents[i])
			}
		}
	})

	t.Run("creates valid tar.gz with empty entries", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		err := archive.BuildTarGz(&buf, []archive.Entry{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		gr, err := gzip.NewReader(&buf)
		if err != nil {
			t.Fatalf("opening gzip: %v", err)
		}
		defer func() { _ = gr.Close() }()

		tr := tar.NewReader(gr)
		_, err = tr.Next()
		if !errors.Is(err, io.EOF) {
			t.Errorf("expected EOF for empty archive, got %v", err)
		}
	})

	t.Run("handles entry with empty content", func(t *testing.T) {
		t.Parallel()

		entries := []archive.Entry{
			{Path: "empty.txt", Content: ""},
		}

		var buf bytes.Buffer
		err := archive.BuildTarGz(&buf, entries)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		gr, _ := gzip.NewReader(&buf)
		defer func() { _ = gr.Close() }()

		tr := tar.NewReader(gr)
		hdr, _ := tr.Next()
		if hdr.Size != 0 {
			t.Errorf("file size = %d, want 0", hdr.Size)
		}
	})

	t.Run("all entries have mode 0o600", func(t *testing.T) {
		t.Parallel()

		entries := []archive.Entry{
			{Path: "a.txt", Content: "a"},
			{Path: "b.txt", Content: "b"},
			{Path: "c.txt", Content: "c"},
		}

		var buf bytes.Buffer
		err := archive.BuildTarGz(&buf, entries)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		gr, _ := gzip.NewReader(&buf)
		defer func() { _ = gr.Close() }()

		tr := tar.NewReader(gr)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("reading tar header: %v", err)
			}
			if hdr.Mode != 0o600 {
				t.Errorf("entry %q mode = %o, want 0o600", hdr.Name, hdr.Mode)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// Mock implementation of gitTemplateRepo
// ---------------------------------------------------------------------------

type mockGitTemplateRepo struct {
	ListFn             func(ctx context.Context, category string, limit, offset int) ([]model.GitTemplate, error)
	CountFilteredFn    func(ctx context.Context, category string) (int, error)
	SearchFn           func(ctx context.Context, query, category string, limit int) ([]model.GitTemplate, error)
	FindByUUIDFn       func(ctx context.Context, uuid string) (*model.GitTemplate, error)
	CreateFn           func(ctx context.Context, tmpl *model.GitTemplate) error
	UpdateFn           func(ctx context.Context, tmpl *model.GitTemplate) error
	SoftDeleteFn       func(ctx context.Context, id int64) error
	FilesForTemplateFn func(ctx context.Context, templateID int64) ([]model.GitTemplateFile, error)
	FindFileByPathFn   func(ctx context.Context, templateID int64, path string) (*model.GitTemplateFile, error)
}

func (m *mockGitTemplateRepo) List(ctx context.Context, category string, limit, offset int) ([]model.GitTemplate, error) {
	return m.ListFn(ctx, category, limit, offset)
}

func (m *mockGitTemplateRepo) CountFiltered(ctx context.Context, category string) (int, error) {
	return m.CountFilteredFn(ctx, category)
}

func (m *mockGitTemplateRepo) Search(ctx context.Context, query, category string, limit int) ([]model.GitTemplate, error) {
	return m.SearchFn(ctx, query, category, limit)
}

func (m *mockGitTemplateRepo) FindByUUID(ctx context.Context, uuid string) (*model.GitTemplate, error) {
	return m.FindByUUIDFn(ctx, uuid)
}

func (m *mockGitTemplateRepo) Create(ctx context.Context, tmpl *model.GitTemplate) error {
	return m.CreateFn(ctx, tmpl)
}

func (m *mockGitTemplateRepo) Update(ctx context.Context, tmpl *model.GitTemplate) error {
	return m.UpdateFn(ctx, tmpl)
}

func (m *mockGitTemplateRepo) SoftDelete(ctx context.Context, id int64) error {
	return m.SoftDeleteFn(ctx, id)
}

func (m *mockGitTemplateRepo) FilesForTemplate(ctx context.Context, templateID int64) ([]model.GitTemplateFile, error) {
	return m.FilesForTemplateFn(ctx, templateID)
}

func (m *mockGitTemplateRepo) FindFileByPath(ctx context.Context, templateID int64, path string) (*model.GitTemplateFile, error) {
	return m.FindFileByPathFn(ctx, templateID, path)
}

// ---------------------------------------------------------------------------
// GitTemplateHandler early-return path tests
// ---------------------------------------------------------------------------

func newTestGitTemplateHandler() *GitTemplateHandler {
	return &GitTemplateHandler{
		repo:   nil,
		logger: slog.New(slog.DiscardHandler),
	}
}

func newGitTemplateHandlerWithMock(repo *mockGitTemplateRepo) *GitTemplateHandler {
	return &GitTemplateHandler{
		repo:   repo,
		logger: slog.New(slog.DiscardHandler),
	}
}

func TestGitTemplateHandler_ReadFile_EmptyPath(t *testing.T) {
	t.Parallel()

	t.Run("returns 400 when file path is empty", func(t *testing.T) {
		t.Parallel()

		h := newTestGitTemplateHandler()
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates/uuid-1/files/", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "uuid-1", "*": ""})
		rr := httptest.NewRecorder()

		h.ReadFile(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if msg := body["message"]; msg != "file path is required" {
			t.Errorf("message = %v, want 'file path is required'", msg)
		}
	})
}

func TestGitTemplateHandler_ReadFile(t *testing.T) {
	t.Parallel()

	// Shared template returned by FindByUUID in the "success" cases.
	validTemplate := &model.GitTemplate{ID: 42, UUID: "uuid-1", Name: "test-template"}

	// Shared file returned by FindFileByPath in the "success" cases.
	validFile := &model.GitTemplateFile{
		ID:          1,
		GitTemplateID: 42,
		Path:        "src/main.go",
		Filename:    "main.go",
		Content:     sql.NullString{String: "package main", Valid: true},
		SizeBytes:   12,
		IsEssential: true,
	}

	t.Run("returns 404 when template not found", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return nil, sql.ErrNoRows
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates/uuid-1/files/src/main.go", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "uuid-1", "*": "src/main.go"})
		rr := httptest.NewRecorder()

		h.ReadFile(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
		assert.Equal(t, "git template not found", body["message"])
	})

	t.Run("returns 500 for unexpected FindByUUID error", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return nil, errors.New("db error")
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates/uuid-1/files/src/main.go", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "uuid-1", "*": "src/main.go"})
		rr := httptest.NewRecorder()

		h.ReadFile(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("returns 404 when file not found in template", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return validTemplate, nil
			},
			FindFileByPathFn: func(_ context.Context, _ int64, _ string) (*model.GitTemplateFile, error) {
				return nil, sql.ErrNoRows
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates/uuid-1/files/missing.txt", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "uuid-1", "*": "missing.txt"})
		rr := httptest.NewRecorder()

		h.ReadFile(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
		assert.Contains(t, body["message"], "not found in template")
	})

	t.Run("returns 500 for unexpected FindFileByPath error", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return validTemplate, nil
			},
			FindFileByPathFn: func(_ context.Context, _ int64, _ string) (*model.GitTemplateFile, error) {
				return nil, errors.New("db error")
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates/uuid-1/files/src/main.go", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "uuid-1", "*": "src/main.go"})
		rr := httptest.NewRecorder()

		h.ReadFile(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("returns 200 with file content on success", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return validTemplate, nil
			},
			FindFileByPathFn: func(_ context.Context, _ int64, _ string) (*model.GitTemplateFile, error) {
				return validFile, nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates/uuid-1/files/src/main.go", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "uuid-1", "*": "src/main.go"})
		rr := httptest.NewRecorder()

		h.ReadFile(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
		data := body["data"].(map[string]any)
		assert.Equal(t, "package main", data["content"])
		assert.Equal(t, "src/main.go", data["path"])
		assert.Equal(t, "main.go", data["filename"])
	})

	t.Run("returns 400 for invalid variables JSON", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return validTemplate, nil
			},
			FindFileByPathFn: func(_ context.Context, _ int64, _ string) (*model.GitTemplateFile, error) {
				return validFile, nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		reqURL := "/api/git-templates/uuid-1/files/src/main.go?variables=" + url.QueryEscape("notjson")
		req := httptest.NewRequest(http.MethodGet, reqURL, http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "uuid-1", "*": "src/main.go"})
		rr := httptest.NewRecorder()

		h.ReadFile(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
		assert.Equal(t, "invalid variables JSON", body["message"])
	})

	t.Run("returns 200 with substituted variables", func(t *testing.T) {
		t.Parallel()

		fileWithVar := &model.GitTemplateFile{
			ID:          2,
			GitTemplateID: 42,
			Path:        "README.md",
			Filename:    "README.md",
			Content:     sql.NullString{String: "Hello, {{name}}!", Valid: true},
			SizeBytes:   17,
		}
		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return validTemplate, nil
			},
			FindFileByPathFn: func(_ context.Context, _ int64, _ string) (*model.GitTemplateFile, error) {
				return fileWithVar, nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		reqURL := "/api/git-templates/uuid-1/files/README.md?variables=" + url.QueryEscape(`{"name":"World"}`)
		req := httptest.NewRequest(http.MethodGet, reqURL, http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "uuid-1", "*": "README.md"})
		rr := httptest.NewRecorder()

		h.ReadFile(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
		data := body["data"].(map[string]any)
		assert.Equal(t, "Hello, World!", data["content"])
	})

	t.Run("response includes unresolved_variables when present", func(t *testing.T) {
		t.Parallel()

		fileWithVars := &model.GitTemplateFile{
			ID:          3,
			GitTemplateID: 42,
			Path:        "config.yml",
			Filename:    "config.yml",
			Content:     sql.NullString{String: "{{found}} {{missing}}", Valid: true},
			SizeBytes:   21,
		}
		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return validTemplate, nil
			},
			FindFileByPathFn: func(_ context.Context, _ int64, _ string) (*model.GitTemplateFile, error) {
				return fileWithVars, nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		reqURL := "/api/git-templates/uuid-1/files/config.yml?variables=" + url.QueryEscape(`{"found":"yes"}`)
		req := httptest.NewRequest(http.MethodGet, reqURL, http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "uuid-1", "*": "config.yml"})
		rr := httptest.NewRecorder()

		h.ReadFile(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
		unresolved, ok := body["unresolved_variables"].([]any)
		require.True(t, ok, "unresolved_variables key should be present")
		assert.Contains(t, unresolved, "missing")
	})

	t.Run("returns 200 without unresolved_variables key when all resolved", func(t *testing.T) {
		t.Parallel()

		fileWithVar := &model.GitTemplateFile{
			ID:          4,
			GitTemplateID: 42,
			Path:        "greeting.txt",
			Filename:    "greeting.txt",
			Content:     sql.NullString{String: "Hi {{name}}", Valid: true},
			SizeBytes:   11,
		}
		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return validTemplate, nil
			},
			FindFileByPathFn: func(_ context.Context, _ int64, _ string) (*model.GitTemplateFile, error) {
				return fileWithVar, nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		reqURL := "/api/git-templates/uuid-1/files/greeting.txt?variables=" + url.QueryEscape(`{"name":"Alice"}`)
		req := httptest.NewRequest(http.MethodGet, reqURL, http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "uuid-1", "*": "greeting.txt"})
		rr := httptest.NewRecorder()

		h.ReadFile(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
		_, hasUnresolved := body["unresolved_variables"]
		assert.False(t, hasUnresolved, "unresolved_variables key should not be present when all variables are resolved")
	})

	t.Run("passes file path to FindFileByPath", func(t *testing.T) {
		t.Parallel()

		var capturedPath string
		var capturedTemplateID int64
		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return validTemplate, nil
			},
			FindFileByPathFn: func(_ context.Context, templateID int64, path string) (*model.GitTemplateFile, error) {
				capturedTemplateID = templateID
				capturedPath = path
				return validFile, nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates/uuid-1/files/deep/nested/file.go", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "uuid-1", "*": "deep/nested/file.go"})
		rr := httptest.NewRecorder()

		h.ReadFile(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "deep/nested/file.go", capturedPath)
		assert.Equal(t, int64(42), capturedTemplateID)
	})
}

func TestGitTemplateHandler_Create_Validation(t *testing.T) {
	t.Parallel()

	t.Run("returns 400 for invalid JSON body", func(t *testing.T) {
		t.Parallel()

		h := newTestGitTemplateHandler()
		req := httptest.NewRequest(http.MethodPost, "/api/git-templates", strings.NewReader("not json"))
		rr := httptest.NewRecorder()

		h.Create(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if msg := body["message"]; msg != "invalid JSON body" {
			t.Errorf("message = %v, want 'invalid JSON body'", msg)
		}
	})

	t.Run("returns 400 when name is missing", func(t *testing.T) {
		t.Parallel()

		h := newTestGitTemplateHandler()
		req := httptest.NewRequest(http.MethodPost, "/api/git-templates",
			strings.NewReader(`{"repository_url":"https://github.com/user/repo"}`))
		rr := httptest.NewRecorder()

		h.Create(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if msg := body["message"]; msg != "name is required" {
			t.Errorf("message = %v, want 'name is required'", msg)
		}
	})

	t.Run("returns 400 when repository_url is missing", func(t *testing.T) {
		t.Parallel()

		h := newTestGitTemplateHandler()
		req := httptest.NewRequest(http.MethodPost, "/api/git-templates",
			strings.NewReader(`{"name":"My Template"}`))
		rr := httptest.NewRecorder()

		h.Create(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if msg := body["message"]; msg != "repository_url is required" {
			t.Errorf("message = %v, want 'repository_url is required'", msg)
		}
	})
}

func TestGitTemplateHandler_Download_Validation(t *testing.T) {
	t.Parallel()

	t.Run("returns 400 for invalid JSON body", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return &model.GitTemplate{ID: 1, UUID: "uuid-1", Name: "Test", Slug: "test"}, nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodPost, "/api/git-templates/uuid-1/download",
			strings.NewReader("not json"))
		req = chiContext(req, map[string]string{"uuid": "uuid-1"})
		rr := httptest.NewRecorder()

		h.Download(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})
}

func TestNewGitTemplateHandler(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.DiscardHandler)
	h := NewGitTemplateHandler(nil, nil, logger)

	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if h.logger != logger {
		t.Error("logger not set correctly")
	}
}

// ---------------------------------------------------------------------------
// GitTemplateHandler.List tests
// ---------------------------------------------------------------------------

func TestGitTemplateHandler_List(t *testing.T) {
	t.Parallel()

	t.Run("returns templates with default limit", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			CountFilteredFn: func(_ context.Context, _ string) (int, error) { return 2, nil },
			ListFn: func(_ context.Context, category string, limit, offset int) ([]model.GitTemplate, error) {
				if category != "" {
					t.Errorf("category = %q, want empty", category)
				}
				if limit != 50 {
					t.Errorf("limit = %d, want 50", limit)
				}
				if offset != 0 {
					t.Errorf("offset = %d, want 0", offset)
				}
				return []model.GitTemplate{
					{UUID: "t1", Name: "Template One", Slug: "template-one", RepositoryURL: "https://github.com/a/b", Branch: "main", Status: "synced"},
					{UUID: "t2", Name: "Template Two", Slug: "template-two", RepositoryURL: "https://github.com/c/d", Branch: "main", Status: "pending"},
				}, nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates", http.NoBody)
		rr := httptest.NewRecorder()

		h.List(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		data, ok := body["data"].([]any)
		if !ok {
			t.Fatal("data field missing or not an array")
		}
		if len(data) != 2 {
			t.Errorf("data length = %d, want 2", len(data))
		}

		meta := body["meta"].(map[string]any)
		if total := meta["total"].(float64); total != 2 {
			t.Errorf("meta.total = %v, want 2", total)
		}
	})

	t.Run("passes category filter and custom per_page", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			CountFilteredFn: func(_ context.Context, _ string) (int, error) { return 0, nil },
			ListFn: func(_ context.Context, category string, limit, offset int) ([]model.GitTemplate, error) {
				if category != "devops" {
					t.Errorf("category = %q, want devops", category)
				}
				if limit != 10 {
					t.Errorf("limit = %d, want 10", limit)
				}
				if offset != 0 {
					t.Errorf("offset = %d, want 0", offset)
				}
				return []model.GitTemplate{}, nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates?category=devops&per_page=10", http.NoBody)
		rr := httptest.NewRecorder()

		h.List(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})

	t.Run("returns 500 when count fails", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			CountFilteredFn: func(_ context.Context, _ string) (int, error) {
				return 0, errors.New("db connection lost")
			},
			ListFn: func(_ context.Context, _ string, _, _ int) ([]model.GitTemplate, error) {
				t.Error("ListFn should not be called when count fails")
				return nil, nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates", http.NoBody)
		rr := httptest.NewRecorder()

		h.List(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if msg := body["message"]; msg != "failed to count git templates" {
			t.Errorf("message = %v, want 'failed to count git templates'", msg)
		}
	})

	t.Run("returns 500 when list fails", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			CountFilteredFn: func(_ context.Context, _ string) (int, error) { return 5, nil },
			ListFn: func(_ context.Context, _ string, _, _ int) ([]model.GitTemplate, error) {
				return nil, errors.New("db connection lost")
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates", http.NoBody)
		rr := httptest.NewRecorder()

		h.List(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if msg := body["message"]; msg != "failed to list git templates" {
			t.Errorf("message = %v, want 'failed to list git templates'", msg)
		}
	})

	t.Run("returns empty array when no templates exist", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			CountFilteredFn: func(_ context.Context, _ string) (int, error) { return 0, nil },
			ListFn: func(_ context.Context, _ string, _, _ int) ([]model.GitTemplate, error) {
				return []model.GitTemplate{}, nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates", http.NoBody)
		rr := httptest.NewRecorder()

		h.List(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		data := body["data"].([]any)
		if len(data) != 0 {
			t.Errorf("data length = %d, want 0", len(data))
		}
	})

	t.Run("negative per_page defaults to 50", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			CountFilteredFn: func(_ context.Context, _ string) (int, error) { return 0, nil },
			ListFn: func(_ context.Context, _ string, limit, _ int) ([]model.GitTemplate, error) {
				if limit != 50 {
					t.Errorf("limit = %d, want 50 (default)", limit)
				}
				return []model.GitTemplate{}, nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates?per_page=-5", http.NoBody)
		rr := httptest.NewRecorder()

		h.List(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})
}

// ---------------------------------------------------------------------------
// GitTemplateHandler.Show tests
// ---------------------------------------------------------------------------

func TestGitTemplateHandler_Show(t *testing.T) {
	t.Parallel()

	t.Run("returns template when found", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, uuid string) (*model.GitTemplate, error) {
				if uuid != "tmpl-uuid-1" {
					t.Errorf("uuid = %q, want tmpl-uuid-1", uuid)
				}
				return &model.GitTemplate{
					UUID:          "tmpl-uuid-1",
					Name:          "My Template",
					Slug:          "my-template",
					RepositoryURL: "https://github.com/user/repo",
					Branch:        "main",
					Status:        "synced",
					FileCount:     5,
				}, nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates/tmpl-uuid-1", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "tmpl-uuid-1"})
		rr := httptest.NewRecorder()

		h.Show(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		data := body["data"].(map[string]any)
		if data["uuid"] != "tmpl-uuid-1" {
			t.Errorf("uuid = %v, want tmpl-uuid-1", data["uuid"])
		}
		if data["name"] != "My Template" {
			t.Errorf("name = %v, want My Template", data["name"])
		}
	})

	t.Run("returns 404 when template not found", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return nil, fmt.Errorf("finding template: %w", sql.ErrNoRows)
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates/missing", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "missing"})
		rr := httptest.NewRecorder()

		h.Show(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("returns 500 on unexpected error", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return nil, errors.New("connection reset")
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates/uuid-1", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "uuid-1"})
		rr := httptest.NewRecorder()

		h.Show(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})
}

// ---------------------------------------------------------------------------
// GitTemplateHandler.Delete tests
// ---------------------------------------------------------------------------

func TestGitTemplateHandler_Delete(t *testing.T) {
	t.Parallel()

	t.Run("deletes template successfully", func(t *testing.T) {
		t.Parallel()

		var deletedID int64
		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return &model.GitTemplate{ID: 42, UUID: "del-uuid"}, nil
			},
			SoftDeleteFn: func(_ context.Context, id int64) error {
				deletedID = id
				return nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodDelete, "/api/git-templates/del-uuid", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "del-uuid"})
		rr := httptest.NewRecorder()

		h.Delete(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		if deletedID != 42 {
			t.Errorf("deleted ID = %d, want 42", deletedID)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if msg := body["message"]; msg != "Git template deleted successfully." {
			t.Errorf("message = %v, want 'Git template deleted successfully.'", msg)
		}
	})

	t.Run("returns 404 when template not found", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return nil, fmt.Errorf("not found: %w", sql.ErrNoRows)
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodDelete, "/api/git-templates/missing", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "missing"})
		rr := httptest.NewRecorder()

		h.Delete(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("returns 500 when soft delete fails", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return &model.GitTemplate{ID: 1, UUID: "uuid-1"}, nil
			},
			SoftDeleteFn: func(_ context.Context, _ int64) error {
				return errors.New("db write failed")
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodDelete, "/api/git-templates/uuid-1", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "uuid-1"})
		rr := httptest.NewRecorder()

		h.Delete(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})

	t.Run("returns 500 when find returns unexpected error", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return nil, errors.New("timeout")
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodDelete, "/api/git-templates/uuid-1", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "uuid-1"})
		rr := httptest.NewRecorder()

		h.Delete(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})
}

// ---------------------------------------------------------------------------
// GitTemplateHandler.ValidateURL tests
// ---------------------------------------------------------------------------

func TestGitTemplateHandler_ValidateURL(t *testing.T) {
	t.Parallel()

	t.Run("returns valid true for safe URL", func(t *testing.T) {
		t.Parallel()

		h := newTestGitTemplateHandler()
		req := httptest.NewRequest(http.MethodPost, "/api/admin/git-templates/validate-url",
			strings.NewReader(`{"url":"https://github.com/user/repo"}`))
		rr := httptest.NewRecorder()

		h.ValidateURL(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if valid := body["valid"].(bool); !valid {
			t.Error("valid = false, want true")
		}
	})

	t.Run("returns valid false for SSRF-blocked URL", func(t *testing.T) {
		t.Parallel()

		h := newTestGitTemplateHandler()
		req := httptest.NewRequest(http.MethodPost, "/api/admin/git-templates/validate-url",
			strings.NewReader(`{"url":"http://127.0.0.1:8080/secret"}`))
		rr := httptest.NewRecorder()

		h.ValidateURL(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if valid := body["valid"].(bool); valid {
			t.Error("valid = true, want false for localhost URL")
		}
		if _, hasErr := body["error"]; !hasErr {
			t.Error("expected error field in response for blocked URL")
		}
	})

	t.Run("returns 400 for invalid JSON", func(t *testing.T) {
		t.Parallel()

		h := newTestGitTemplateHandler()
		req := httptest.NewRequest(http.MethodPost, "/api/admin/git-templates/validate-url",
			strings.NewReader("not json"))
		rr := httptest.NewRecorder()

		h.ValidateURL(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("returns 400 when url is empty", func(t *testing.T) {
		t.Parallel()

		h := newTestGitTemplateHandler()
		req := httptest.NewRequest(http.MethodPost, "/api/admin/git-templates/validate-url",
			strings.NewReader(`{"url":""}`))
		rr := httptest.NewRecorder()

		h.ValidateURL(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("returns valid false for file scheme URL", func(t *testing.T) {
		t.Parallel()

		h := newTestGitTemplateHandler()
		req := httptest.NewRequest(http.MethodPost, "/api/admin/git-templates/validate-url",
			strings.NewReader(`{"url":"file:///etc/passwd"}`))
		rr := httptest.NewRecorder()

		h.ValidateURL(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if valid := body["valid"].(bool); valid {
			t.Error("valid = true, want false for file:// URL")
		}
	})
}

// newGitTemplateHandlerFull creates a handler with repo and inserter mocks.
// Uses mockJobInserter from external_service_handler_test.go (same package).
func newGitTemplateHandlerFull(repo *mockGitTemplateRepo, ins gitTemplateJobInserter) *GitTemplateHandler {
	return &GitTemplateHandler{
		repo:     repo,
		inserter: ins,
		logger:   slog.New(slog.DiscardHandler),
	}
}

// ---------------------------------------------------------------------------
// Shared test fixtures
// ---------------------------------------------------------------------------

func sampleTemplate() *model.GitTemplate {
	return &model.GitTemplate{
		ID:             42,
		UUID:           "test-uuid",
		Name:           "My Template",
		Slug:           "my-template",
		RepositoryURL:  "https://github.com/user/repo",
		Branch:         "main",
		IsPublic:       true,
		IsEnabled:      true,
		Status:         "synced",
		FileCount:      2,
		TotalSizeBytes: 1024,
		Description:    sql.NullString{String: "A test template", Valid: true},
	}
}

func sampleFiles() []model.GitTemplateFile {
	return []model.GitTemplateFile{
		{
			ID:            1,
			GitTemplateID: 42,
			Path:          "README.md",
			Filename:      "README.md",
			Extension:     sql.NullString{String: ".md", Valid: true},
			Content:       sql.NullString{String: "# Hello {{project_name}}", Valid: true},
			SizeBytes:     24,
			IsEssential:   true,
			ContentHash:   sql.NullString{String: "abc123", Valid: true},
		},
		{
			ID:            2,
			GitTemplateID: 42,
			Path:          "src/main.go",
			Filename:      "main.go",
			Extension:     sql.NullString{String: ".go", Valid: true},
			Content:       sql.NullString{String: "package main\n", Valid: true},
			SizeBytes:     13,
			IsEssential:   false,
		},
	}
}

// ---------------------------------------------------------------------------
// GitTemplateHandler.Search tests
// ---------------------------------------------------------------------------

func TestGitTemplateHandler_Search(t *testing.T) {
	t.Parallel()

	t.Run("returns matching templates", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			SearchFn: func(_ context.Context, query, category string, limit int) ([]model.GitTemplate, error) {
				if query != "docker" {
					t.Errorf("query = %q, want docker", query)
				}
				if category != "devops" {
					t.Errorf("category = %q, want devops", category)
				}
				if limit != 10 {
					t.Errorf("limit = %d, want 10", limit)
				}
				return []model.GitTemplate{
					{UUID: "t1", Name: "Docker Compose", Slug: "docker-compose", RepositoryURL: "https://github.com/a/b", Branch: "main", Status: "synced"},
				}, nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates/search?q=docker&category=devops&limit=10", http.NoBody)
		rr := httptest.NewRecorder()

		h.Search(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		data := body["data"].([]any)
		if len(data) != 1 {
			t.Errorf("data length = %d, want 1", len(data))
		}
		meta := body["meta"].(map[string]any)
		if meta["query"] != "docker" {
			t.Errorf("meta.query = %v, want docker", meta["query"])
		}
		if total := meta["total"].(float64); total != 1 {
			t.Errorf("meta.total = %v, want 1", total)
		}
	})

	t.Run("uses default limit of 50 when not provided", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			SearchFn: func(_ context.Context, _ string, _ string, limit int) ([]model.GitTemplate, error) {
				if limit != 50 {
					t.Errorf("limit = %d, want 50", limit)
				}
				return []model.GitTemplate{}, nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates/search?q=test", http.NoBody)
		rr := httptest.NewRecorder()

		h.Search(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})

	t.Run("returns empty array when no results", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			SearchFn: func(_ context.Context, _ string, _ string, _ int) ([]model.GitTemplate, error) {
				return []model.GitTemplate{}, nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates/search?q=nonexistent", http.NoBody)
		rr := httptest.NewRecorder()

		h.Search(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		data := body["data"].([]any)
		if len(data) != 0 {
			t.Errorf("data length = %d, want 0", len(data))
		}
	})

	t.Run("returns 500 when repo search fails", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			SearchFn: func(_ context.Context, _ string, _ string, _ int) ([]model.GitTemplate, error) {
				return nil, errors.New("search index unavailable")
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates/search?q=test", http.NoBody)
		rr := httptest.NewRecorder()

		h.Search(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if msg := body["message"]; msg != "failed to search git templates" {
			t.Errorf("message = %v, want 'failed to search git templates'", msg)
		}
	})
}

// ---------------------------------------------------------------------------
// GitTemplateHandler.Update tests
// ---------------------------------------------------------------------------

func TestGitTemplateHandler_Update(t *testing.T) {
	t.Parallel()

	t.Run("returns 404 when template not found", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return nil, fmt.Errorf("not found: %w", sql.ErrNoRows)
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodPut, "/api/git-templates/missing", strings.NewReader(`{"name":"Updated"}`))
		req = chiContext(req, map[string]string{"uuid": "missing"})
		rr := httptest.NewRecorder()

		h.Update(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("returns 500 when find returns unexpected error", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return nil, errors.New("connection timeout")
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodPut, "/api/git-templates/uuid-1", strings.NewReader(`{"name":"Updated"}`))
		req = chiContext(req, map[string]string{"uuid": "uuid-1"})
		rr := httptest.NewRecorder()

		h.Update(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})

	t.Run("returns 400 for invalid JSON body", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return sampleTemplate(), nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodPut, "/api/git-templates/test-uuid", strings.NewReader("not json"))
		req = chiContext(req, map[string]string{"uuid": "test-uuid"})
		rr := httptest.NewRecorder()

		h.Update(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if msg := body["message"]; msg != "invalid JSON body" {
			t.Errorf("message = %v, want 'invalid JSON body'", msg)
		}
	})

	t.Run("returns 400 for SSRF-blocked repository URL", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return sampleTemplate(), nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodPut, "/api/git-templates/test-uuid",
			strings.NewReader(`{"repository_url":"http://127.0.0.1:8080/evil"}`))
		req = chiContext(req, map[string]string{"uuid": "test-uuid"})
		rr := httptest.NewRecorder()

		h.Update(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		msg, _ := body["message"].(string)
		if msg != "Invalid repository URL" {
			t.Errorf("message = %q, want %q", msg, "Invalid repository URL")
		}
	})

	t.Run("updates all provided fields successfully", func(t *testing.T) {
		t.Parallel()

		var updatedTmpl *model.GitTemplate
		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return sampleTemplate(), nil
			},
			UpdateFn: func(_ context.Context, tmpl *model.GitTemplate) error {
				updatedTmpl = tmpl
				return nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		reqBody := `{
			"name": "Updated Name",
			"description": "New desc",
			"branch": "develop",
			"category": "web",
			"tags": ["go", "api"],
			"is_public": false,
			"repository_url": "https://github.com/new/repo"
		}`
		req := httptest.NewRequest(http.MethodPut, "/api/git-templates/test-uuid", strings.NewReader(reqBody))
		req = chiContext(req, map[string]string{"uuid": "test-uuid"})
		rr := httptest.NewRecorder()

		h.Update(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		if updatedTmpl == nil {
			t.Fatal("UpdateFn was not called")
		}
		if updatedTmpl.Name != "Updated Name" {
			t.Errorf("Name = %q, want 'Updated Name'", updatedTmpl.Name)
		}
		if updatedTmpl.Slug != "updated-name" {
			t.Errorf("Slug = %q, want 'updated-name'", updatedTmpl.Slug)
		}
		if updatedTmpl.Description.String != "New desc" {
			t.Errorf("Description = %q, want 'New desc'", updatedTmpl.Description.String)
		}
		if updatedTmpl.Branch != "develop" {
			t.Errorf("Branch = %q, want 'develop'", updatedTmpl.Branch)
		}
		if updatedTmpl.Category.String != "web" {
			t.Errorf("Category = %q, want 'web'", updatedTmpl.Category.String)
		}
		if updatedTmpl.RepositoryURL != "https://github.com/new/repo" {
			t.Errorf("RepositoryURL = %q, want 'https://github.com/new/repo'", updatedTmpl.RepositoryURL)
		}
		if updatedTmpl.IsPublic {
			t.Error("IsPublic = true, want false")
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if msg := body["message"]; msg != "Git template updated successfully." {
			t.Errorf("message = %v, want 'Git template updated successfully.'", msg)
		}
	})

	t.Run("returns 500 when repo update fails", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return sampleTemplate(), nil
			},
			UpdateFn: func(_ context.Context, _ *model.GitTemplate) error {
				return errors.New("db write failed")
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodPut, "/api/git-templates/test-uuid",
			strings.NewReader(`{"name":"Updated"}`))
		req = chiContext(req, map[string]string{"uuid": "test-uuid"})
		rr := httptest.NewRecorder()

		h.Update(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if msg := body["message"]; msg != "failed to update git template" {
			t.Errorf("message = %v, want 'failed to update git template'", msg)
		}
	})

	t.Run("partial update preserves unset fields", func(t *testing.T) {
		t.Parallel()

		var updatedTmpl *model.GitTemplate
		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return sampleTemplate(), nil
			},
			UpdateFn: func(_ context.Context, tmpl *model.GitTemplate) error {
				updatedTmpl = tmpl
				return nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodPut, "/api/git-templates/test-uuid",
			strings.NewReader(`{"name":"Only Name Changed"}`))
		req = chiContext(req, map[string]string{"uuid": "test-uuid"})
		rr := httptest.NewRecorder()

		h.Update(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		if updatedTmpl.Name != "Only Name Changed" {
			t.Errorf("Name = %q, want 'Only Name Changed'", updatedTmpl.Name)
		}
		// Original branch should be preserved.
		if updatedTmpl.Branch != "main" {
			t.Errorf("Branch = %q, want 'main' (preserved)", updatedTmpl.Branch)
		}
		// Original repository URL should be preserved.
		if updatedTmpl.RepositoryURL != "https://github.com/user/repo" {
			t.Errorf("RepositoryURL = %q, want original URL", updatedTmpl.RepositoryURL)
		}
	})
}

// ---------------------------------------------------------------------------
// GitTemplateHandler.Sync tests
// ---------------------------------------------------------------------------

func TestGitTemplateHandler_Sync(t *testing.T) {
	t.Parallel()

	t.Run("returns 404 when template not found", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return nil, fmt.Errorf("not found: %w", sql.ErrNoRows)
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodPost, "/api/git-templates/missing/sync", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "missing"})
		rr := httptest.NewRecorder()

		h.Sync(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("returns 500 when find returns unexpected error", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return nil, errors.New("connection timeout")
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodPost, "/api/git-templates/uuid-1/sync", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "uuid-1"})
		rr := httptest.NewRecorder()

		h.Sync(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})

	t.Run("returns 503 when inserter is nil", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return sampleTemplate(), nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodPost, "/api/git-templates/test-uuid/sync", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "test-uuid"})
		rr := httptest.NewRecorder()

		h.Sync(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusServiceUnavailable)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if msg := body["message"]; msg != "job queue not available" {
			t.Errorf("message = %v, want 'job queue not available'", msg)
		}
	})

	t.Run("returns 500 when inserter fails", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return sampleTemplate(), nil
			},
		}
		ins := &mockJobInserter{
			insertFn: func(_ context.Context, _ river.JobArgs, _ *river.InsertOpts) (*rivertype.JobInsertResult, error) {
				return nil, errors.New("queue full")
			},
		}
		h := newGitTemplateHandlerFull(repo, ins)
		req := httptest.NewRequest(http.MethodPost, "/api/git-templates/test-uuid/sync", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "test-uuid"})
		rr := httptest.NewRecorder()

		h.Sync(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if msg := body["message"]; msg != "failed to enqueue sync job" {
			t.Errorf("message = %v, want 'failed to enqueue sync job'", msg)
		}
	})

	t.Run("returns 202 and enqueues job successfully", func(t *testing.T) {
		t.Parallel()

		var insertedArgs river.JobArgs
		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return sampleTemplate(), nil
			},
		}
		ins := &mockJobInserter{
			insertFn: func(_ context.Context, args river.JobArgs, _ *river.InsertOpts) (*rivertype.JobInsertResult, error) {
				insertedArgs = args
				return &rivertype.JobInsertResult{}, nil
			},
		}
		h := newGitTemplateHandlerFull(repo, ins)
		req := httptest.NewRequest(http.MethodPost, "/api/git-templates/test-uuid/sync", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "test-uuid"})
		rr := httptest.NewRecorder()

		h.Sync(rr, req)

		if rr.Code != http.StatusAccepted {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusAccepted)
		}

		if _, ok := insertedArgs.(queue.SyncGitTemplatesArgs); !ok {
			t.Errorf("inserted args type = %T, want queue.SyncGitTemplatesArgs", insertedArgs)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if msg := body["message"]; msg != "Sync queued" {
			t.Errorf("message = %v, want 'Sync queued'", msg)
		}
	})
}

// ---------------------------------------------------------------------------
// GitTemplateHandler.Structure tests
// ---------------------------------------------------------------------------

func TestGitTemplateHandler_Structure(t *testing.T) {
	t.Parallel()

	t.Run("returns 404 when template not found", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return nil, fmt.Errorf("not found: %w", sql.ErrNoRows)
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates/missing/structure", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "missing"})
		rr := httptest.NewRecorder()

		h.Structure(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("returns 500 when find returns unexpected error", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return nil, errors.New("db error")
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates/uuid-1/structure", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "uuid-1"})
		rr := httptest.NewRecorder()

		h.Structure(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})

	t.Run("returns 500 when files query fails", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return sampleTemplate(), nil
			},
			FilesForTemplateFn: func(_ context.Context, _ int64) ([]model.GitTemplateFile, error) {
				return nil, errors.New("files table unavailable")
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates/test-uuid/structure", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "test-uuid"})
		rr := httptest.NewRecorder()

		h.Structure(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})

	t.Run("returns file tree with essential files and variables", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return sampleTemplate(), nil
			},
			FilesForTemplateFn: func(_ context.Context, templateID int64) ([]model.GitTemplateFile, error) {
				if templateID != 42 {
					t.Errorf("templateID = %d, want 42", templateID)
				}
				return sampleFiles(), nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates/test-uuid/structure", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "test-uuid"})
		rr := httptest.NewRecorder()

		h.Structure(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		data := body["data"].(map[string]any)

		if data["uuid"] != "test-uuid" {
			t.Errorf("uuid = %v, want test-uuid", data["uuid"])
		}
		if data["name"] != "My Template" {
			t.Errorf("name = %v, want My Template", data["name"])
		}

		fileTree := data["file_tree"].([]any)
		if len(fileTree) != 2 {
			t.Fatalf("file_tree length = %d, want 2", len(fileTree))
		}
		if fileTree[0] != "README.md" {
			t.Errorf("file_tree[0] = %v, want README.md", fileTree[0])
		}
		if fileTree[1] != "src/main.go" {
			t.Errorf("file_tree[1] = %v, want src/main.go", fileTree[1])
		}

		essentialFiles := data["essential_files"].([]any)
		if len(essentialFiles) != 1 {
			t.Fatalf("essential_files length = %d, want 1", len(essentialFiles))
		}
		if essentialFiles[0] != "README.md" {
			t.Errorf("essential_files[0] = %v, want README.md", essentialFiles[0])
		}

		variables := data["variables"].([]any)
		if len(variables) != 1 {
			t.Fatalf("variables length = %d, want 1", len(variables))
		}
		if variables[0] != "project_name" {
			t.Errorf("variables[0] = %v, want project_name", variables[0])
		}

		files := data["files"].([]any)
		if len(files) != 2 {
			t.Fatalf("files length = %d, want 2", len(files))
		}
		firstFile := files[0].(map[string]any)
		if firstFile["path"] != "README.md" {
			t.Errorf("files[0].path = %v, want README.md", firstFile["path"])
		}
		if firstFile["extension"] != ".md" {
			t.Errorf("files[0].extension = %v, want .md", firstFile["extension"])
		}
		if firstFile["is_essential"] != true {
			t.Errorf("files[0].is_essential = %v, want true", firstFile["is_essential"])
		}
		if firstFile["content_hash"] != "abc123" {
			t.Errorf("files[0].content_hash = %v, want abc123", firstFile["content_hash"])
		}
	})

	t.Run("returns empty structure when template has no files", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return sampleTemplate(), nil
			},
			FilesForTemplateFn: func(_ context.Context, _ int64) ([]model.GitTemplateFile, error) {
				return []model.GitTemplateFile{}, nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates/test-uuid/structure", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "test-uuid"})
		rr := httptest.NewRecorder()

		h.Structure(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		data := body["data"].(map[string]any)
		fileTree := data["file_tree"].([]any)
		if len(fileTree) != 0 {
			t.Errorf("file_tree length = %d, want 0", len(fileTree))
		}
	})
}

// ---------------------------------------------------------------------------
// GitTemplateHandler.DeploymentGuide tests
// ---------------------------------------------------------------------------

func TestGitTemplateHandler_DeploymentGuide(t *testing.T) {
	t.Parallel()

	t.Run("returns 404 when template not found", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return nil, fmt.Errorf("not found: %w", sql.ErrNoRows)
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates/missing/deployment-guide", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "missing"})
		rr := httptest.NewRecorder()

		h.DeploymentGuide(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("returns 500 when find returns unexpected error", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return nil, errors.New("db error")
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates/uuid-1/deployment-guide", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "uuid-1"})
		rr := httptest.NewRecorder()

		h.DeploymentGuide(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})

	t.Run("returns 500 when files query fails", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return sampleTemplate(), nil
			},
			FilesForTemplateFn: func(_ context.Context, _ int64) ([]model.GitTemplateFile, error) {
				return nil, errors.New("disk error")
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates/test-uuid/deployment-guide", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "test-uuid"})
		rr := httptest.NewRecorder()

		h.DeploymentGuide(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})

	t.Run("returns guide with only essential files", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return sampleTemplate(), nil
			},
			FilesForTemplateFn: func(_ context.Context, _ int64) ([]model.GitTemplateFile, error) {
				return sampleFiles(), nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates/test-uuid/deployment-guide", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "test-uuid"})
		rr := httptest.NewRecorder()

		h.DeploymentGuide(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		data := body["data"].(map[string]any)

		if data["template_name"] != "My Template" {
			t.Errorf("template_name = %v, want My Template", data["template_name"])
		}
		if data["description"] != "A test template" {
			t.Errorf("description = %v, want 'A test template'", data["description"])
		}

		files := data["files"].([]any)
		if len(files) != 1 {
			t.Fatalf("files length = %d, want 1 (only essential)", len(files))
		}
		firstFile := files[0].(map[string]any)
		if firstFile["path"] != "README.md" {
			t.Errorf("files[0].path = %v, want README.md", firstFile["path"])
		}
		if firstFile["content"] != "# Hello {{project_name}}" {
			t.Errorf("files[0].content = %v, want '# Hello {{project_name}}'", firstFile["content"])
		}

		steps := data["steps"].([]any)
		if len(steps) != 1 {
			t.Fatalf("steps length = %d, want 1", len(steps))
		}

		unresolved := data["unresolved_variables"].([]any)
		if len(unresolved) != 0 {
			t.Errorf("unresolved_variables length = %d, want 0 (no variable substitution requested)", len(unresolved))
		}
	})

	t.Run("substitutes variables in essential file content", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return sampleTemplate(), nil
			},
			FilesForTemplateFn: func(_ context.Context, _ int64) ([]model.GitTemplateFile, error) {
				return sampleFiles(), nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet,
			`/api/git-templates/test-uuid/deployment-guide?variables={"project_name":"MyApp"}`, http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "test-uuid"})
		rr := httptest.NewRecorder()

		h.DeploymentGuide(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		data := body["data"].(map[string]any)
		files := data["files"].([]any)
		firstFile := files[0].(map[string]any)
		if firstFile["content"] != "# Hello MyApp" {
			t.Errorf("content = %v, want '# Hello MyApp'", firstFile["content"])
		}
	})

	t.Run("returns empty description when not set", func(t *testing.T) {
		t.Parallel()

		tmpl := sampleTemplate()
		tmpl.Description = sql.NullString{Valid: false}

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return tmpl, nil
			},
			FilesForTemplateFn: func(_ context.Context, _ int64) ([]model.GitTemplateFile, error) {
				return []model.GitTemplateFile{}, nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates/test-uuid/deployment-guide", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "test-uuid"})
		rr := httptest.NewRecorder()

		h.DeploymentGuide(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		data := body["data"].(map[string]any)
		if data["description"] != "" {
			t.Errorf("description = %v, want empty string", data["description"])
		}
	})
}

// ---------------------------------------------------------------------------
// GitTemplateHandler.Create extended tests
// ---------------------------------------------------------------------------

func TestGitTemplateHandler_Create(t *testing.T) {
	t.Parallel()

	t.Run("returns 400 for SSRF-blocked repository URL", func(t *testing.T) {
		t.Parallel()

		h := newTestGitTemplateHandler()
		req := httptest.NewRequest(http.MethodPost, "/api/git-templates",
			strings.NewReader(`{"name":"Evil","repository_url":"http://169.254.169.254/latest/meta-data"}`))
		rr := httptest.NewRecorder()

		h.Create(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		msg, _ := body["message"].(string)
		if msg != "Invalid repository URL" {
			t.Errorf("message = %q, want %q", msg, "Invalid repository URL")
		}
	})

	t.Run("returns 500 when repo create fails", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			CreateFn: func(_ context.Context, _ *model.GitTemplate) error {
				return errors.New("unique constraint violation")
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodPost, "/api/git-templates",
			strings.NewReader(`{"name":"Test","repository_url":"https://github.com/user/repo"}`))
		rr := httptest.NewRecorder()

		h.Create(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if msg := body["message"]; msg != "failed to create git template" {
			t.Errorf("message = %v, want 'failed to create git template'", msg)
		}
	})

	t.Run("succeeds with nil inserter without error", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			CreateFn: func(_ context.Context, _ *model.GitTemplate) error {
				return nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodPost, "/api/git-templates",
			strings.NewReader(`{"name":"My Template","repository_url":"https://github.com/user/repo"}`))
		rr := httptest.NewRecorder()

		h.Create(rr, req)

		if rr.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if msg := body["message"]; msg != "Git template created and queued for sync." {
			t.Errorf("message = %v, want 'Git template created and queued for sync.'", msg)
		}
	})

	t.Run("creates template with all fields and enqueues sync", func(t *testing.T) {
		t.Parallel()

		var createdTmpl *model.GitTemplate
		var insertCalled bool
		repo := &mockGitTemplateRepo{
			CreateFn: func(_ context.Context, tmpl *model.GitTemplate) error {
				createdTmpl = tmpl
				return nil
			},
		}
		ins := &mockJobInserter{
			insertFn: func(_ context.Context, _ river.JobArgs, _ *river.InsertOpts) (*rivertype.JobInsertResult, error) {
				insertCalled = true
				return &rivertype.JobInsertResult{}, nil
			},
		}
		h := newGitTemplateHandlerFull(repo, ins)
		reqBody := `{
			"name": "Full Template",
			"repository_url": "https://github.com/user/repo",
			"description": "A complete template",
			"branch": "develop",
			"category": "devops",
			"tags": ["go", "docker"],
			"is_public": true
		}`
		req := httptest.NewRequest(http.MethodPost, "/api/git-templates", strings.NewReader(reqBody))
		rr := httptest.NewRecorder()

		h.Create(rr, req)

		if rr.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
		}

		if createdTmpl == nil {
			t.Fatal("CreateFn was not called")
		}
		if createdTmpl.Name != "Full Template" {
			t.Errorf("Name = %q, want 'Full Template'", createdTmpl.Name)
		}
		if createdTmpl.Slug != "full-template" {
			t.Errorf("Slug = %q, want 'full-template'", createdTmpl.Slug)
		}
		if createdTmpl.Branch != "develop" {
			t.Errorf("Branch = %q, want 'develop'", createdTmpl.Branch)
		}
		if createdTmpl.Description.String != "A complete template" {
			t.Errorf("Description = %q, want 'A complete template'", createdTmpl.Description.String)
		}
		if createdTmpl.Category.String != "devops" {
			t.Errorf("Category = %q, want 'devops'", createdTmpl.Category.String)
		}
		if !createdTmpl.IsPublic {
			t.Error("IsPublic = false, want true")
		}
		if createdTmpl.Status != "pending" {
			t.Errorf("Status = %q, want 'pending'", createdTmpl.Status)
		}
		if createdTmpl.UUID == "" {
			t.Error("UUID should not be empty")
		}
		if !insertCalled {
			t.Error("inserter.Insert was not called")
		}
	})

	t.Run("defaults branch to main when not provided", func(t *testing.T) {
		t.Parallel()

		var createdTmpl *model.GitTemplate
		repo := &mockGitTemplateRepo{
			CreateFn: func(_ context.Context, tmpl *model.GitTemplate) error {
				createdTmpl = tmpl
				return nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodPost, "/api/git-templates",
			strings.NewReader(`{"name":"NoBranch","repository_url":"https://github.com/user/repo"}`))
		rr := httptest.NewRecorder()

		h.Create(rr, req)

		if rr.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
		}
		if createdTmpl.Branch != "main" {
			t.Errorf("Branch = %q, want 'main' (default)", createdTmpl.Branch)
		}
	})
}

// ---------------------------------------------------------------------------
// GitTemplateHandler.Download tests
// ---------------------------------------------------------------------------

func TestGitTemplateHandler_Download(t *testing.T) {
	t.Parallel()

	t.Run("returns 404 when template not found", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return nil, fmt.Errorf("not found: %w", sql.ErrNoRows)
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodPost, "/api/git-templates/missing/download",
			strings.NewReader(`{"format":"zip"}`))
		req = chiContext(req, map[string]string{"uuid": "missing"})
		rr := httptest.NewRecorder()

		h.Download(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("returns 400 for invalid format", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return sampleTemplate(), nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodPost, "/api/git-templates/test-uuid/download",
			strings.NewReader(`{"format":"rar"}`))
		req = chiContext(req, map[string]string{"uuid": "test-uuid"})
		rr := httptest.NewRecorder()

		h.Download(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if msg := body["message"]; msg != "format must be 'zip' or 'tar.gz'" {
			t.Errorf("message = %v, want \"format must be 'zip' or 'tar.gz'\"", msg)
		}
	})

	t.Run("returns 500 when files query fails", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return sampleTemplate(), nil
			},
			FilesForTemplateFn: func(_ context.Context, _ int64) ([]model.GitTemplateFile, error) {
				return nil, errors.New("disk error")
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodPost, "/api/git-templates/test-uuid/download",
			strings.NewReader(`{"format":"zip"}`))
		req = chiContext(req, map[string]string{"uuid": "test-uuid"})
		rr := httptest.NewRecorder()

		h.Download(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})

	t.Run("returns zip archive with correct metadata", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return sampleTemplate(), nil
			},
			FilesForTemplateFn: func(_ context.Context, _ int64) ([]model.GitTemplateFile, error) {
				return sampleFiles(), nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodPost, "/api/git-templates/test-uuid/download",
			strings.NewReader(`{"format":"zip"}`))
		req = chiContext(req, map[string]string{"uuid": "test-uuid"})
		rr := httptest.NewRecorder()

		h.Download(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		data := body["data"].(map[string]any)

		if data["format"] != "zip" {
			t.Errorf("format = %v, want zip", data["format"])
		}
		if data["filename"] != "my-template.zip" {
			t.Errorf("filename = %v, want my-template.zip", data["filename"])
		}
		if fileCount := data["file_count"].(float64); fileCount != 2 {
			t.Errorf("file_count = %v, want 2", fileCount)
		}
		if data["archive_base64"] == nil || data["archive_base64"] == "" {
			t.Error("archive_base64 should not be empty")
		}
	})

	t.Run("returns tar.gz archive with correct metadata", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return sampleTemplate(), nil
			},
			FilesForTemplateFn: func(_ context.Context, _ int64) ([]model.GitTemplateFile, error) {
				return sampleFiles(), nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodPost, "/api/git-templates/test-uuid/download",
			strings.NewReader(`{"format":"tar.gz"}`))
		req = chiContext(req, map[string]string{"uuid": "test-uuid"})
		rr := httptest.NewRecorder()

		h.Download(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		data := body["data"].(map[string]any)

		if data["format"] != "tar.gz" {
			t.Errorf("format = %v, want tar.gz", data["format"])
		}
		if data["filename"] != "my-template.tar.gz" {
			t.Errorf("filename = %v, want my-template.tar.gz", data["filename"])
		}
		if fileCount := data["file_count"].(float64); fileCount != 2 {
			t.Errorf("file_count = %v, want 2", fileCount)
		}
	})

	t.Run("defaults to zip when format not provided", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return sampleTemplate(), nil
			},
			FilesForTemplateFn: func(_ context.Context, _ int64) ([]model.GitTemplateFile, error) {
				return []model.GitTemplateFile{}, nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodPost, "/api/git-templates/test-uuid/download",
			strings.NewReader(`{}`))
		req = chiContext(req, map[string]string{"uuid": "test-uuid"})
		rr := httptest.NewRecorder()

		h.Download(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		data := body["data"].(map[string]any)
		if data["format"] != "zip" {
			t.Errorf("format = %v, want zip (default)", data["format"])
		}
	})

	t.Run("returns valid archive with no files", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return sampleTemplate(), nil
			},
			FilesForTemplateFn: func(_ context.Context, _ int64) ([]model.GitTemplateFile, error) {
				return []model.GitTemplateFile{}, nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodPost, "/api/git-templates/test-uuid/download",
			strings.NewReader(`{"format":"zip"}`))
		req = chiContext(req, map[string]string{"uuid": "test-uuid"})
		rr := httptest.NewRecorder()

		h.Download(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		data := body["data"].(map[string]any)
		if fileCount := data["file_count"].(float64); fileCount != 0 {
			t.Errorf("file_count = %v, want 0", fileCount)
		}
		if data["archive_base64"] == nil {
			t.Error("archive_base64 should not be nil for empty archive")
		}
	})

	t.Run("substitutes variables in archive content", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return sampleTemplate(), nil
			},
			FilesForTemplateFn: func(_ context.Context, _ int64) ([]model.GitTemplateFile, error) {
				return sampleFiles(), nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodPost, "/api/git-templates/test-uuid/download",
			strings.NewReader(`{"format":"zip","variables":{"project_name":"MyApp"}}`))
		req = chiContext(req, map[string]string{"uuid": "test-uuid"})
		rr := httptest.NewRecorder()

		h.Download(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		data := body["data"].(map[string]any)

		// Decode the base64 zip and verify file content has substituted variables.
		archiveB64 := data["archive_base64"].(string)
		archiveBytes, err := base64.StdEncoding.DecodeString(archiveB64)
		if err != nil {
			t.Fatalf("decoding base64: %v", err)
		}

		reader, err := zip.NewReader(bytes.NewReader(archiveBytes), int64(len(archiveBytes)))
		if err != nil {
			t.Fatalf("opening zip: %v", err)
		}

		if len(reader.File) != 2 {
			t.Fatalf("zip file count = %d, want 2", len(reader.File))
		}

		rc, err := reader.File[0].Open()
		if err != nil {
			t.Fatalf("opening zip entry: %v", err)
		}
		defer func() { _ = rc.Close() }()

		content, _ := io.ReadAll(rc)
		if string(content) != "# Hello MyApp" {
			t.Errorf("file content = %q, want '# Hello MyApp'", string(content))
		}

		unresolved := data["unresolved_variables"].([]any)
		if len(unresolved) != 0 {
			t.Errorf("unresolved_variables length = %d, want 0", len(unresolved))
		}
	})
}
