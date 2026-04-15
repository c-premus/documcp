package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/c-premus/documcp/internal/client/kiwix"
	"github.com/c-premus/documcp/internal/model"
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
// Mock implementations
// ---------------------------------------------------------------------------

type mockZimArchiveRepo struct {
	ListFn          func(ctx context.Context, category, language, query string, limit, offset int) ([]model.ZimArchive, error)
	CountFilteredFn func(ctx context.Context, category, language, query string) (int, error)
	FindByNameFn    func(ctx context.Context, name string) (*model.ZimArchive, error)
}

func (m *mockZimArchiveRepo) List(ctx context.Context, category, language, query string, limit, offset int) ([]model.ZimArchive, error) {
	return m.ListFn(ctx, category, language, query, limit, offset)
}

func (m *mockZimArchiveRepo) CountFiltered(ctx context.Context, category, language, query string) (int, error) {
	return m.CountFilteredFn(ctx, category, language, query)
}

func (m *mockZimArchiveRepo) FindByName(ctx context.Context, name string) (*model.ZimArchive, error) {
	return m.FindByNameFn(ctx, name)
}

type mockKiwixSearcher struct {
	SearchFn      func(ctx context.Context, archiveName, query, searchType string, limit int) ([]kiwix.SearchResult, error)
	ReadArticleFn func(ctx context.Context, archiveName, articlePath string) (*kiwix.Article, error)
}

func (m *mockKiwixSearcher) Search(ctx context.Context, archiveName, query, searchType string, limit int) ([]kiwix.SearchResult, error) {
	return m.SearchFn(ctx, archiveName, query, searchType, limit)
}

func (m *mockKiwixSearcher) ReadArticle(ctx context.Context, archiveName, articlePath string) (*kiwix.Article, error) {
	return m.ReadArticleFn(ctx, archiveName, articlePath)
}

func (m *mockKiwixSearcher) HasFulltextIndex(_ context.Context, _ string) bool {
	return false
}

// ---------------------------------------------------------------------------
// ZimHandler early-return path tests (nil kiwixClient)
// ---------------------------------------------------------------------------

// mockZimFactory implements kiwixClientFactory for tests.
type mockZimFactory struct {
	client kiwixSearcher
}

func (f *mockZimFactory) Get(_ context.Context) (kiwixSearcher, error) {
	if f.client == nil {
		return nil, errors.New("kiwix not configured")
	}
	return f.client, nil
}

func newTestZimHandler() *ZimHandler {
	return &ZimHandler{
		repo:         nil,
		kiwixFactory: nil,
		logger:       slog.New(slog.DiscardHandler),
	}
}

func newZimHandlerWithMocks(repo *mockZimArchiveRepo, kc kiwixSearcher) *ZimHandler {
	h := &ZimHandler{
		repo:   repo,
		logger: slog.New(slog.DiscardHandler),
	}
	if kc != nil {
		h.kiwixFactory = &mockZimFactory{client: kc}
	}
	return h
}

func TestZimHandler_Search_NilClient(t *testing.T) {
	t.Parallel()

	t.Run("returns 503 when kiwix client not configured", func(t *testing.T) {
		t.Parallel()

		h := newTestZimHandler()
		req := httptest.NewRequest(http.MethodGet, "/api/zim/archives/test/search?q=hello", http.NoBody)
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
		req := httptest.NewRequest(http.MethodGet, "/api/zim/archives/test/suggest?q=hello", http.NoBody)
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
		req := httptest.NewRequest(http.MethodGet, "/api/zim/archives/test/articles/page", http.NoBody)
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

		kc := &mockKiwixSearcher{}
		h := newZimHandlerWithMocks(nil, kc)
		req := httptest.NewRequest(http.MethodGet, "/api/zim/archives/test/articles/", http.NoBody)
		req = chiContext(req, map[string]string{"archive": "test", "*": ""})
		rr := httptest.NewRecorder()

		h.ReadArticle(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if msg := body["message"]; msg != "article path is required" {
			t.Errorf("message = %v, want 'article path is required'", msg)
		}
	})
}

func TestNewZimHandler(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.DiscardHandler)
	h := NewZimHandler(nil, nil, logger)

	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if h.logger != logger {
		t.Error("logger not set correctly")
	}
	if h.kiwixFactory != nil {
		t.Error("kiwixClient should be nil")
	}
}

// ---------------------------------------------------------------------------
// ZimHandler.List tests
// ---------------------------------------------------------------------------

func TestZimHandler_List(t *testing.T) {
	t.Parallel()

	t.Run("returns archives with default per_page", func(t *testing.T) {
		t.Parallel()

		repo := &mockZimArchiveRepo{
			CountFilteredFn: func(_ context.Context, _, _, _ string) (int, error) {
				return 2, nil
			},
			ListFn: func(_ context.Context, category, language, query string, limit, offset int) ([]model.ZimArchive, error) {
				if category != "" {
					t.Errorf("category = %q, want empty", category)
				}
				if limit != 50 {
					t.Errorf("limit = %d, want 50", limit)
				}
				if offset != 0 {
					t.Errorf("offset = %d, want 0", offset)
				}
				return []model.ZimArchive{
					{UUID: "z1", Name: "wikipedia_en", Title: "Wikipedia", Language: "eng"},
					{UUID: "z2", Name: "wiktionary_en", Title: "Wiktionary", Language: "eng"},
				}, nil
			},
		}
		h := newZimHandlerWithMocks(repo, nil)
		req := httptest.NewRequest(http.MethodGet, "/api/zim/archives", http.NoBody)
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
		if len(data) != 2 {
			t.Errorf("data length = %d, want 2", len(data))
		}

		meta := body["meta"].(map[string]any)
		if total := meta["total"].(float64); total != 2 {
			t.Errorf("meta.total = %v, want 2", total)
		}
	})

	t.Run("passes category language and query filters", func(t *testing.T) {
		t.Parallel()

		repo := &mockZimArchiveRepo{
			CountFilteredFn: func(_ context.Context, _, _, _ string) (int, error) {
				return 0, nil
			},
			ListFn: func(_ context.Context, category, language, query string, limit, _ int) ([]model.ZimArchive, error) {
				if category != "wikipedia" {
					t.Errorf("category = %q, want wikipedia", category)
				}
				if language != "eng" {
					t.Errorf("language = %q, want eng", language)
				}
				if query != "science" {
					t.Errorf("query = %q, want science", query)
				}
				if limit != 5 {
					t.Errorf("limit = %d, want 5", limit)
				}
				return []model.ZimArchive{}, nil
			},
		}
		h := newZimHandlerWithMocks(repo, nil)
		req := httptest.NewRequest(http.MethodGet, "/api/zim/archives?category=wikipedia&language=eng&query=science&per_page=5", http.NoBody)
		rr := httptest.NewRecorder()

		h.List(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})

	t.Run("returns 500 when count returns error", func(t *testing.T) {
		t.Parallel()

		repo := &mockZimArchiveRepo{
			CountFilteredFn: func(_ context.Context, _, _, _ string) (int, error) {
				return 0, errors.New("db failure")
			},
		}
		h := newZimHandlerWithMocks(repo, nil)
		req := httptest.NewRequest(http.MethodGet, "/api/zim/archives", http.NoBody)
		rr := httptest.NewRecorder()

		h.List(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if msg := body["message"]; msg != "failed to count ZIM archives" {
			t.Errorf("message = %v, want 'failed to count ZIM archives'", msg)
		}
	})

	t.Run("returns 500 when list returns error", func(t *testing.T) {
		t.Parallel()

		repo := &mockZimArchiveRepo{
			CountFilteredFn: func(_ context.Context, _, _, _ string) (int, error) {
				return 0, nil
			},
			ListFn: func(_ context.Context, _, _, _ string, _, _ int) ([]model.ZimArchive, error) {
				return nil, errors.New("db failure")
			},
		}
		h := newZimHandlerWithMocks(repo, nil)
		req := httptest.NewRequest(http.MethodGet, "/api/zim/archives", http.NoBody)
		rr := httptest.NewRecorder()

		h.List(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if msg := body["message"]; msg != "failed to list ZIM archives" {
			t.Errorf("message = %v, want 'failed to list ZIM archives'", msg)
		}
	})

	t.Run("returns empty array when no archives exist", func(t *testing.T) {
		t.Parallel()

		repo := &mockZimArchiveRepo{
			CountFilteredFn: func(_ context.Context, _, _, _ string) (int, error) {
				return 0, nil
			},
			ListFn: func(_ context.Context, _, _, _ string, _, _ int) ([]model.ZimArchive, error) {
				return []model.ZimArchive{}, nil
			},
		}
		h := newZimHandlerWithMocks(repo, nil)
		req := httptest.NewRequest(http.MethodGet, "/api/zim/archives", http.NoBody)
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

		repo := &mockZimArchiveRepo{
			CountFilteredFn: func(_ context.Context, _, _, _ string) (int, error) {
				return 0, nil
			},
			ListFn: func(_ context.Context, _, _, _ string, limit, _ int) ([]model.ZimArchive, error) {
				if limit != 50 {
					t.Errorf("limit = %d, want 50 (default)", limit)
				}
				return []model.ZimArchive{}, nil
			},
		}
		h := newZimHandlerWithMocks(repo, nil)
		req := httptest.NewRequest(http.MethodGet, "/api/zim/archives?per_page=-10", http.NoBody)
		rr := httptest.NewRecorder()

		h.List(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})
}

// ---------------------------------------------------------------------------
// ZimHandler.Show tests
// ---------------------------------------------------------------------------

func TestZimHandler_Show(t *testing.T) {
	t.Parallel()

	t.Run("returns archive when found", func(t *testing.T) {
		t.Parallel()

		repo := &mockZimArchiveRepo{
			FindByNameFn: func(_ context.Context, name string) (*model.ZimArchive, error) {
				if name != "wikipedia_en" {
					t.Errorf("name = %q, want wikipedia_en", name)
				}
				return &model.ZimArchive{
					UUID:         "zim-1",
					Name:         "wikipedia_en",
					Title:        "Wikipedia (English)",
					Language:     "eng",
					ArticleCount: 6_000_000,
					FileSize:     90 * 1024 * 1024 * 1024,
				}, nil
			},
		}
		h := newZimHandlerWithMocks(repo, nil)
		req := httptest.NewRequest(http.MethodGet, "/api/zim/archives/wikipedia_en", http.NoBody)
		req = chiContext(req, map[string]string{"archive": "wikipedia_en"})
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
		if data["uuid"] != "zim-1" {
			t.Errorf("uuid = %v, want zim-1", data["uuid"])
		}
		if data["name"] != "wikipedia_en" {
			t.Errorf("name = %v, want wikipedia_en", data["name"])
		}
		if data["title"] != "Wikipedia (English)" {
			t.Errorf("title = %v, want Wikipedia (English)", data["title"])
		}
	})

	t.Run("returns 404 when archive not found", func(t *testing.T) {
		t.Parallel()

		repo := &mockZimArchiveRepo{
			FindByNameFn: func(_ context.Context, _ string) (*model.ZimArchive, error) {
				return nil, fmt.Errorf("not found: %w", sql.ErrNoRows)
			},
		}
		h := newZimHandlerWithMocks(repo, nil)
		req := httptest.NewRequest(http.MethodGet, "/api/zim/archives/nonexistent", http.NoBody)
		req = chiContext(req, map[string]string{"archive": "nonexistent"})
		rr := httptest.NewRecorder()

		h.Show(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if msg := body["message"]; msg != "ZIM archive not found" {
			t.Errorf("message = %v, want 'ZIM archive not found'", msg)
		}
	})

	t.Run("returns 500 on unexpected error", func(t *testing.T) {
		t.Parallel()

		repo := &mockZimArchiveRepo{
			FindByNameFn: func(_ context.Context, _ string) (*model.ZimArchive, error) {
				return nil, errors.New("connection timeout")
			},
		}
		h := newZimHandlerWithMocks(repo, nil)
		req := httptest.NewRequest(http.MethodGet, "/api/zim/archives/test", http.NoBody)
		req = chiContext(req, map[string]string{"archive": "test"})
		rr := httptest.NewRecorder()

		h.Show(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})
}

// ---------------------------------------------------------------------------
// ZimHandler.Search tests (fulltext search with kiwix client)
// ---------------------------------------------------------------------------

func TestZimHandler_Search(t *testing.T) {
	t.Parallel()

	t.Run("returns 400 when q param is missing", func(t *testing.T) {
		t.Parallel()

		kc := &mockKiwixSearcher{}
		h := newZimHandlerWithMocks(nil, kc)
		req := httptest.NewRequest(http.MethodGet, "/api/zim/archives/wiki/search", http.NoBody)
		req = chiContext(req, map[string]string{"archive": "wiki"})
		rr := httptest.NewRecorder()

		h.Search(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if msg := body["message"]; msg != "query parameter 'q' is required" {
			t.Errorf("message = %v, want 'query parameter 'q' is required'", msg)
		}
	})

	t.Run("returns results from kiwix client", func(t *testing.T) {
		t.Parallel()

		kc := &mockKiwixSearcher{
			SearchFn: func(_ context.Context, archiveName, query, searchType string, limit int) ([]kiwix.SearchResult, error) {
				if archiveName != "wikipedia_en" {
					t.Errorf("archiveName = %q, want wikipedia_en", archiveName)
				}
				if query != "golang" {
					t.Errorf("query = %q, want golang", query)
				}
				if searchType != "fulltext" {
					t.Errorf("searchType = %q, want fulltext", searchType)
				}
				if limit != 10 {
					t.Errorf("limit = %d, want 10 (default)", limit)
				}
				return []kiwix.SearchResult{
					{Title: "Go (programming language)", Path: "A/Go_(programming_language)", Snippet: "Go is a language", Score: 0.95},
					{Title: "Golang", Path: "A/Golang", Snippet: "Redirect", Score: 0.80},
				}, nil
			},
		}
		h := newZimHandlerWithMocks(nil, kc)
		req := httptest.NewRequest(http.MethodGet, "/api/zim/archives/wikipedia_en/search?q=golang", http.NoBody)
		req = chiContext(req, map[string]string{"archive": "wikipedia_en"})
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
		if len(data) != 2 {
			t.Fatalf("data length = %d, want 2", len(data))
		}

		first := data[0].(map[string]any)
		if first["title"] != "Go (programming language)" {
			t.Errorf("first title = %v, want Go (programming language)", first["title"])
		}
		if first["path"] != "A/Go_(programming_language)" {
			t.Errorf("first path = %v, want A/Go_(programming_language)", first["path"])
		}
		if first["snippet"] != "Go is a language" {
			t.Errorf("first snippet = %v, want 'Go is a language'", first["snippet"])
		}

		meta := body["meta"].(map[string]any)
		if meta["archive"] != "wikipedia_en" {
			t.Errorf("meta.archive = %v, want wikipedia_en", meta["archive"])
		}
		if meta["query"] != "golang" {
			t.Errorf("meta.query = %v, want golang", meta["query"])
		}
		if total := meta["total"].(float64); total != 2 {
			t.Errorf("meta.total = %v, want 2", total)
		}
	})

	t.Run("returns empty results when kiwix client returns error", func(t *testing.T) {
		t.Parallel()

		kc := &mockKiwixSearcher{
			SearchFn: func(_ context.Context, _, _, _ string, _ int) ([]kiwix.SearchResult, error) {
				return nil, errors.New("connection refused")
			},
		}
		h := newZimHandlerWithMocks(nil, kc)
		req := httptest.NewRequest(http.MethodGet, "/api/zim/archives/wiki/search?q=test", http.NoBody)
		req = chiContext(req, map[string]string{"archive": "wiki"})
		rr := httptest.NewRecorder()

		h.Search(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		data := body["data"].([]any)
		if len(data) != 0 {
			t.Errorf("expected empty data, got %d items", len(data))
		}
	})

	t.Run("respects explicit limit param", func(t *testing.T) {
		t.Parallel()

		var capturedLimit int
		kc := &mockKiwixSearcher{
			SearchFn: func(_ context.Context, _, _, _ string, limit int) ([]kiwix.SearchResult, error) {
				capturedLimit = limit
				return []kiwix.SearchResult{}, nil
			},
		}
		h := newZimHandlerWithMocks(nil, kc)
		req := httptest.NewRequest(http.MethodGet, "/api/zim/archives/wiki/search?q=test&limit=25", http.NoBody)
		req = chiContext(req, map[string]string{"archive": "wiki"})
		rr := httptest.NewRecorder()

		h.Search(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		if capturedLimit != 25 {
			t.Errorf("limit passed to kiwix = %d, want 25", capturedLimit)
		}
	})

	t.Run("clamps limit exceeding 100 to 100", func(t *testing.T) {
		t.Parallel()

		var capturedLimit int
		kc := &mockKiwixSearcher{
			SearchFn: func(_ context.Context, _, _, _ string, limit int) ([]kiwix.SearchResult, error) {
				capturedLimit = limit
				return []kiwix.SearchResult{}, nil
			},
		}
		h := newZimHandlerWithMocks(nil, kc)
		req := httptest.NewRequest(http.MethodGet, "/api/zim/archives/wiki/search?q=test&limit=500", http.NoBody)
		req = chiContext(req, map[string]string{"archive": "wiki"})
		rr := httptest.NewRecorder()

		h.Search(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		if capturedLimit != 100 {
			t.Errorf("limit passed to kiwix = %d, want 100 (clamped)", capturedLimit)
		}
	})
}

// ---------------------------------------------------------------------------
// ZimHandler.Suggest tests (autocomplete suggestions)
// ---------------------------------------------------------------------------

func TestZimHandler_Suggest(t *testing.T) {
	t.Parallel()

	t.Run("returns 400 when q param is missing", func(t *testing.T) {
		t.Parallel()

		kc := &mockKiwixSearcher{}
		h := newZimHandlerWithMocks(nil, kc)
		req := httptest.NewRequest(http.MethodGet, "/api/zim/archives/wiki/suggest", http.NoBody)
		req = chiContext(req, map[string]string{"archive": "wiki"})
		rr := httptest.NewRecorder()

		h.Suggest(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if msg := body["message"]; msg != "query parameter 'q' is required and must be at least 2 characters" {
			t.Errorf("message = %v, want minimum length error", msg)
		}
	})

	t.Run("returns 400 when q is only 1 character", func(t *testing.T) {
		t.Parallel()

		kc := &mockKiwixSearcher{}
		h := newZimHandlerWithMocks(nil, kc)
		req := httptest.NewRequest(http.MethodGet, "/api/zim/archives/wiki/suggest?q=a", http.NoBody)
		req = chiContext(req, map[string]string{"archive": "wiki"})
		rr := httptest.NewRecorder()

		h.Suggest(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if msg := body["message"]; msg != "query parameter 'q' is required and must be at least 2 characters" {
			t.Errorf("message = %v, want minimum length error", msg)
		}
	})

	t.Run("returns suggestions from kiwix client", func(t *testing.T) {
		t.Parallel()

		kc := &mockKiwixSearcher{
			SearchFn: func(_ context.Context, archiveName, query, searchType string, limit int) ([]kiwix.SearchResult, error) {
				if archiveName != "wikipedia_en" {
					t.Errorf("archiveName = %q, want wikipedia_en", archiveName)
				}
				if query != "go" {
					t.Errorf("query = %q, want go", query)
				}
				if searchType != "suggest" {
					t.Errorf("searchType = %q, want suggest", searchType)
				}
				if limit != 10 {
					t.Errorf("limit = %d, want 10 (default)", limit)
				}
				return []kiwix.SearchResult{
					{Title: "Go (game)", Path: "A/Go_(game)"},
					{Title: "Go (programming language)", Path: "A/Go_(programming_language)"},
				}, nil
			},
		}
		h := newZimHandlerWithMocks(nil, kc)
		req := httptest.NewRequest(http.MethodGet, "/api/zim/archives/wikipedia_en/suggest?q=go", http.NoBody)
		req = chiContext(req, map[string]string{"archive": "wikipedia_en"})
		rr := httptest.NewRecorder()

		h.Suggest(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}

		data := body["data"].([]any)
		if len(data) != 2 {
			t.Fatalf("data length = %d, want 2", len(data))
		}

		first := data[0].(map[string]any)
		if first["title"] != "Go (game)" {
			t.Errorf("first title = %v, want Go (game)", first["title"])
		}
		if first["path"] != "A/Go_(game)" {
			t.Errorf("first path = %v, want A/Go_(game)", first["path"])
		}

		meta := body["meta"].(map[string]any)
		if meta["archive"] != "wikipedia_en" {
			t.Errorf("meta.archive = %v, want wikipedia_en", meta["archive"])
		}
		if meta["query"] != "go" {
			t.Errorf("meta.query = %v, want go", meta["query"])
		}
		if total := meta["total"].(float64); total != 2 {
			t.Errorf("meta.total = %v, want 2", total)
		}
	})

	t.Run("returns empty results when kiwix client returns error", func(t *testing.T) {
		t.Parallel()

		kc := &mockKiwixSearcher{
			SearchFn: func(_ context.Context, _, _, _ string, _ int) ([]kiwix.SearchResult, error) {
				return nil, errors.New("timeout")
			},
		}
		h := newZimHandlerWithMocks(nil, kc)
		req := httptest.NewRequest(http.MethodGet, "/api/zim/archives/wiki/suggest?q=test", http.NoBody)
		req = chiContext(req, map[string]string{"archive": "wiki"})
		rr := httptest.NewRecorder()

		h.Suggest(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		data := body["data"].([]any)
		if len(data) != 0 {
			t.Errorf("expected empty data, got %d items", len(data))
		}
	})

	t.Run("clamps limit exceeding 50 to 50", func(t *testing.T) {
		t.Parallel()

		var capturedLimit int
		kc := &mockKiwixSearcher{
			SearchFn: func(_ context.Context, _, _, _ string, limit int) ([]kiwix.SearchResult, error) {
				capturedLimit = limit
				return []kiwix.SearchResult{}, nil
			},
		}
		h := newZimHandlerWithMocks(nil, kc)
		req := httptest.NewRequest(http.MethodGet, "/api/zim/archives/wiki/suggest?q=test&limit=200", http.NoBody)
		req = chiContext(req, map[string]string{"archive": "wiki"})
		rr := httptest.NewRecorder()

		h.Suggest(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		if capturedLimit != 50 {
			t.Errorf("limit passed to kiwix = %d, want 50 (clamped)", capturedLimit)
		}
	})
}

// ---------------------------------------------------------------------------
// ZimHandler.ReadArticle tests (article retrieval with kiwix client)
// ---------------------------------------------------------------------------

func TestZimHandler_ReadArticle(t *testing.T) {
	t.Parallel()

	t.Run("returns article from kiwix client", func(t *testing.T) {
		t.Parallel()

		kc := &mockKiwixSearcher{
			ReadArticleFn: func(_ context.Context, archiveName, articlePath string) (*kiwix.Article, error) {
				if archiveName != "wikipedia_en" {
					t.Errorf("archiveName = %q, want wikipedia_en", archiveName)
				}
				if articlePath != "A/Go_(programming_language)" {
					t.Errorf("articlePath = %q, want A/Go_(programming_language)", articlePath)
				}
				return &kiwix.Article{
					Title:    "Go (programming language)",
					Content:  "Go is a statically typed, compiled language.",
					MIMEType: "text/html; charset=utf-8",
				}, nil
			},
		}
		h := newZimHandlerWithMocks(nil, kc)
		req := httptest.NewRequest(http.MethodGet, "/api/zim/archives/wikipedia_en/articles/A/Go_(programming_language)", http.NoBody)
		req = chiContext(req, map[string]string{"archive": "wikipedia_en", "*": "A/Go_(programming_language)"})
		rr := httptest.NewRecorder()

		h.ReadArticle(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}

		data := body["data"].(map[string]any)
		if data["archive_name"] != "wikipedia_en" {
			t.Errorf("archive_name = %v, want wikipedia_en", data["archive_name"])
		}
		if data["path"] != "A/Go_(programming_language)" {
			t.Errorf("path = %v, want A/Go_(programming_language)", data["path"])
		}
		if data["title"] != "Go (programming language)" {
			t.Errorf("title = %v, want Go (programming language)", data["title"])
		}
		if data["content"] != "Go is a statically typed, compiled language." {
			t.Errorf("content = %v, want 'Go is a statically typed, compiled language.'", data["content"])
		}
		if data["mime_type"] != "text/html; charset=utf-8" {
			t.Errorf("mime_type = %v, want 'text/html; charset=utf-8'", data["mime_type"])
		}
	})

	t.Run("returns 500 when kiwix client returns error", func(t *testing.T) {
		t.Parallel()

		kc := &mockKiwixSearcher{
			ReadArticleFn: func(_ context.Context, _, _ string) (*kiwix.Article, error) {
				return nil, errors.New("article not found in archive")
			},
		}
		h := newZimHandlerWithMocks(nil, kc)
		req := httptest.NewRequest(http.MethodGet, "/api/zim/archives/wiki/articles/A/Missing", http.NoBody)
		req = chiContext(req, map[string]string{"archive": "wiki", "*": "A/Missing"})
		rr := httptest.NewRecorder()

		h.ReadArticle(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if msg := body["message"]; msg != "failed to read article" {
			t.Errorf("message = %v, want 'failed to read article'", msg)
		}
	})
}
