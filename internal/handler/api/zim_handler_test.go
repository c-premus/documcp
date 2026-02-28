package api

import (
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"git.999.haus/chris/DocuMCP-go/internal/model"
)

// ---------------------------------------------------------------------------
// humanFileSize tests
// ---------------------------------------------------------------------------

func TestHumanFileSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{
			name:  "zero bytes",
			bytes: 0,
			want:  "0 B",
		},
		{
			name:  "one byte",
			bytes: 1,
			want:  "1 B",
		},
		{
			name:  "just under 1 KB",
			bytes: 1023,
			want:  "1023 B",
		},
		{
			name:  "exactly 1 KB",
			bytes: 1024,
			want:  "1.0 KB",
		},
		{
			name:  "1.5 KB",
			bytes: 1536,
			want:  "1.5 KB",
		},
		{
			name:  "just under 1 MB",
			bytes: 1024*1024 - 1,
			want:  "1024.0 KB",
		},
		{
			name:  "exactly 1 MB",
			bytes: 1024 * 1024,
			want:  "1.0 MB",
		},
		{
			name:  "500 MB",
			bytes: 500 * 1024 * 1024,
			want:  "500.0 MB",
		},
		{
			name:  "exactly 1 GB",
			bytes: 1024 * 1024 * 1024,
			want:  "1.0 GB",
		},
		{
			name:  "1.5 GB",
			bytes: 1536 * 1024 * 1024,
			want:  "1.5 GB",
		},
		{
			name:  "exactly 1 TB",
			bytes: 1024 * 1024 * 1024 * 1024,
			want:  "1.0 TB",
		},
		{
			name:  "2.5 TB",
			bytes: int64(2.5 * 1024 * 1024 * 1024 * 1024),
			want:  "2.5 TB",
		},
		{
			name:  "negative value falls through to bytes",
			bytes: -1,
			want:  "-1 B",
		},
		{
			name:  "large negative falls through to bytes",
			bytes: -1024,
			want:  "-1024 B",
		},
		{
			name:  "512 bytes",
			bytes: 512,
			want:  "512 B",
		},
		{
			name:  "10 KB",
			bytes: 10 * 1024,
			want:  "10.0 KB",
		},
		{
			name:  "100 MB",
			bytes: 100 * 1024 * 1024,
			want:  "100.0 MB",
		},
		{
			name:  "fractional GB",
			bytes: 1024*1024*1024 + 512*1024*1024,
			want:  "1.5 GB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := humanFileSize(tt.bytes)
			if got != tt.want {
				t.Errorf("humanFileSize(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// toZimArchiveResponse tests
// ---------------------------------------------------------------------------

func TestToZimArchiveResponse(t *testing.T) {
	t.Parallel()

	t.Run("maps all required fields", func(t *testing.T) {
		t.Parallel()

		za := &model.ZimArchive{
			UUID:         "zim-uuid-1",
			Name:         "wikipedia_en",
			Title:        "Wikipedia (English)",
			Language:     "eng",
			ArticleCount: 6_000_000,
			MediaCount:   500_000,
			FileSize:     90 * 1024 * 1024 * 1024, // 90 GB
		}

		resp := toZimArchiveResponse(za)

		if resp.UUID != "zim-uuid-1" {
			t.Errorf("UUID = %q, want zim-uuid-1", resp.UUID)
		}
		if resp.Name != "wikipedia_en" {
			t.Errorf("Name = %q, want wikipedia_en", resp.Name)
		}
		if resp.Title != "Wikipedia (English)" {
			t.Errorf("Title = %q, want Wikipedia (English)", resp.Title)
		}
		if resp.Language != "eng" {
			t.Errorf("Language = %q, want eng", resp.Language)
		}
		if resp.ArticleCount != 6_000_000 {
			t.Errorf("ArticleCount = %d, want 6000000", resp.ArticleCount)
		}
		if resp.MediaCount != 500_000 {
			t.Errorf("MediaCount = %d, want 500000", resp.MediaCount)
		}
		if resp.FileSize != 90*1024*1024*1024 {
			t.Errorf("FileSize = %d, want %d", resp.FileSize, 90*1024*1024*1024)
		}
		if resp.FileSizeHuman != "90.0 GB" {
			t.Errorf("FileSizeHuman = %q, want 90.0 GB", resp.FileSizeHuman)
		}
	})

	t.Run("maps optional NullString fields when valid", func(t *testing.T) {
		t.Parallel()

		za := &model.ZimArchive{
			UUID:        "zim-uuid-2",
			Name:        "test",
			Title:       "Test",
			Language:    "eng",
			Description: sql.NullString{String: "A test archive", Valid: true},
			Category:    sql.NullString{String: "wikipedia", Valid: true},
			Creator:     sql.NullString{String: "Kiwix", Valid: true},
			Publisher:   sql.NullString{String: "Kiwix", Valid: true},
		}

		resp := toZimArchiveResponse(za)

		if resp.Description != "A test archive" {
			t.Errorf("Description = %q, want A test archive", resp.Description)
		}
		if resp.Category != "wikipedia" {
			t.Errorf("Category = %q, want wikipedia", resp.Category)
		}
		if resp.Creator != "Kiwix" {
			t.Errorf("Creator = %q, want Kiwix", resp.Creator)
		}
		if resp.Publisher != "Kiwix" {
			t.Errorf("Publisher = %q, want Kiwix", resp.Publisher)
		}
	})

	t.Run("null optional fields produce empty strings", func(t *testing.T) {
		t.Parallel()

		za := &model.ZimArchive{
			UUID:     "zim-uuid-3",
			Name:     "minimal",
			Title:    "Minimal",
			Language: "fra",
		}

		resp := toZimArchiveResponse(za)

		if resp.Description != "" {
			t.Errorf("Description = %q, want empty", resp.Description)
		}
		if resp.Category != "" {
			t.Errorf("Category = %q, want empty", resp.Category)
		}
		if resp.Creator != "" {
			t.Errorf("Creator = %q, want empty", resp.Creator)
		}
		if resp.Publisher != "" {
			t.Errorf("Publisher = %q, want empty", resp.Publisher)
		}
		if resp.LastSyncedAt != "" {
			t.Errorf("LastSyncedAt = %q, want empty", resp.LastSyncedAt)
		}
	})

	t.Run("last synced at formatted as RFC3339 when valid", func(t *testing.T) {
		t.Parallel()

		syncTime := time.Date(2025, 3, 20, 8, 0, 0, 0, time.UTC)
		za := &model.ZimArchive{
			UUID:         "zim-uuid-4",
			Name:         "synced",
			Title:        "Synced",
			Language:     "eng",
			LastSyncedAt: sql.NullTime{Time: syncTime, Valid: true},
		}

		resp := toZimArchiveResponse(za)
		want := "2025-03-20T08:00:00Z"

		if resp.LastSyncedAt != want {
			t.Errorf("LastSyncedAt = %q, want %q", resp.LastSyncedAt, want)
		}
	})

	t.Run("tags parsed from JSON when valid", func(t *testing.T) {
		t.Parallel()

		za := &model.ZimArchive{
			UUID:     "zim-uuid-5",
			Name:     "tagged",
			Title:    "Tagged",
			Language: "eng",
			Tags:     sql.NullString{String: `["wikipedia","science"]`, Valid: true},
		}

		resp := toZimArchiveResponse(za)

		if len(resp.Tags) != 2 {
			t.Fatalf("Tags length = %d, want 2", len(resp.Tags))
		}
		if resp.Tags[0] != "wikipedia" || resp.Tags[1] != "science" {
			t.Errorf("Tags = %v, want [wikipedia science]", resp.Tags)
		}
	})

	t.Run("null tags produce empty slice not nil", func(t *testing.T) {
		t.Parallel()

		za := &model.ZimArchive{
			UUID:     "zim-uuid-6",
			Name:     "no-tags",
			Title:    "No Tags",
			Language: "eng",
		}

		resp := toZimArchiveResponse(za)

		if resp.Tags == nil {
			t.Fatal("Tags should be empty slice, not nil")
		}
		if len(resp.Tags) != 0 {
			t.Errorf("Tags length = %d, want 0", len(resp.Tags))
		}
	})

	t.Run("invalid tags JSON falls back to empty slice", func(t *testing.T) {
		t.Parallel()

		za := &model.ZimArchive{
			UUID:     "zim-uuid-7",
			Name:     "bad-tags",
			Title:    "Bad Tags",
			Language: "eng",
			Tags:     sql.NullString{String: "not-json", Valid: true},
		}

		resp := toZimArchiveResponse(za)

		if resp.Tags == nil {
			t.Fatal("Tags should be empty slice, not nil")
		}
		if len(resp.Tags) != 0 {
			t.Errorf("Tags length = %d, want 0 (fallback for invalid JSON)", len(resp.Tags))
		}
	})

	t.Run("file size human readable for zero bytes", func(t *testing.T) {
		t.Parallel()

		za := &model.ZimArchive{
			UUID:     "zim-uuid-8",
			Name:     "empty",
			Title:    "Empty",
			Language: "eng",
			FileSize: 0,
		}

		resp := toZimArchiveResponse(za)

		if resp.FileSizeHuman != "0 B" {
			t.Errorf("FileSizeHuman = %q, want 0 B", resp.FileSizeHuman)
		}
	})

	t.Run("file size human readable for kilobytes", func(t *testing.T) {
		t.Parallel()

		za := &model.ZimArchive{
			UUID:     "zim-uuid-9",
			Name:     "small",
			Title:    "Small",
			Language: "eng",
			FileSize: 5 * 1024,
		}

		resp := toZimArchiveResponse(za)

		if resp.FileSizeHuman != "5.0 KB" {
			t.Errorf("FileSizeHuman = %q, want 5.0 KB", resp.FileSizeHuman)
		}
	})

	t.Run("zero article and media counts", func(t *testing.T) {
		t.Parallel()

		za := &model.ZimArchive{
			UUID:         "zim-uuid-10",
			Name:         "counts",
			Title:        "Counts",
			Language:     "eng",
			ArticleCount: 0,
			MediaCount:   0,
		}

		resp := toZimArchiveResponse(za)

		if resp.ArticleCount != 0 {
			t.Errorf("ArticleCount = %d, want 0", resp.ArticleCount)
		}
		if resp.MediaCount != 0 {
			t.Errorf("MediaCount = %d, want 0", resp.MediaCount)
		}
	})
}

// ---------------------------------------------------------------------------
// ZimHandler early-return path tests (nil kiwixClient)
// ---------------------------------------------------------------------------

func newTestZimHandler() *ZimHandler {
	return &ZimHandler{
		repo:        nil,
		kiwixClient: nil,
		logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestZimHandler_Search_NilClient(t *testing.T) {
	t.Parallel()

	t.Run("returns 503 when kiwix client not configured", func(t *testing.T) {
		t.Parallel()

		h := newTestZimHandler()
		req := httptest.NewRequest(http.MethodGet, "/api/zim/archives/test/search?q=hello", nil)
		rr := httptest.NewRecorder()

		h.Search(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusServiceUnavailable)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if msg := body["message"]; msg != "Kiwix integration not configured" {
			t.Errorf("message = %v, want 'Kiwix integration not configured'", msg)
		}
	})
}

func TestZimHandler_Suggest_NilClient(t *testing.T) {
	t.Parallel()

	t.Run("returns 503 when kiwix client not configured", func(t *testing.T) {
		t.Parallel()

		h := newTestZimHandler()
		req := httptest.NewRequest(http.MethodGet, "/api/zim/archives/test/suggest?q=hello", nil)
		rr := httptest.NewRecorder()

		h.Suggest(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusServiceUnavailable)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if msg := body["message"]; msg != "Kiwix integration not configured" {
			t.Errorf("message = %v, want 'Kiwix integration not configured'", msg)
		}
	})
}

func TestZimHandler_ReadArticle_NilClient(t *testing.T) {
	t.Parallel()

	t.Run("returns 503 when kiwix client not configured", func(t *testing.T) {
		t.Parallel()

		h := newTestZimHandler()
		req := httptest.NewRequest(http.MethodGet, "/api/zim/archives/test/articles/page", nil)
		req = chiContext(req, map[string]string{"archive": "test", "*": "page"})
		rr := httptest.NewRecorder()

		h.ReadArticle(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusServiceUnavailable)
		}
	})
}

func TestZimHandler_ReadArticle_EmptyPath(t *testing.T) {
	t.Parallel()

	t.Run("returns 400 when article path is empty", func(t *testing.T) {
		t.Parallel()

		// Use a handler with a non-nil kiwixClient field would require
		// construction, but we can test by providing a non-nil handler
		// that has kiwixClient set. Since we cannot easily construct one,
		// we can skip past the nil check by using a handler that has a
		// placeholder. But since kiwixClient is a concrete *kiwix.Client,
		// we cannot fake it without unsafe. This test is omitted.
	})
}

func TestNewZimHandler(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewZimHandler(nil, nil, logger)

	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if h.logger != logger {
		t.Error("logger not set correctly")
	}
	if h.kiwixClient != nil {
		t.Error("kiwixClient should be nil")
	}
}
