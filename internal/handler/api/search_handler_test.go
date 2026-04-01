package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c-premus/documcp/internal/repository"
	"github.com/c-premus/documcp/internal/search"
)

// ---------------------------------------------------------------------------
// Mock searcher implementing documentSearcher
// ---------------------------------------------------------------------------

type mockDocumentSearcher struct {
	searchFn          func(ctx context.Context, params search.SearchParams) (*search.SearchResponse, error)
	federatedSearchFn func(ctx context.Context, params search.FederatedSearchParams) (*search.FederatedSearchResponse, error)
}

func (m *mockDocumentSearcher) Search(ctx context.Context, params search.SearchParams) (*search.SearchResponse, error) {
	if m.searchFn != nil {
		return m.searchFn(ctx, params)
	}
	return &search.SearchResponse{Hits: []search.SearchResult{}}, nil
}

func (m *mockDocumentSearcher) FederatedSearch(ctx context.Context, params search.FederatedSearchParams) (*search.FederatedSearchResponse, error) {
	if m.federatedSearchFn != nil {
		return m.federatedSearchFn(ctx, params)
	}
	return &search.FederatedSearchResponse{Hits: []search.SearchResult{}}, nil
}

// ---------------------------------------------------------------------------
// SearchHandler tests
// ---------------------------------------------------------------------------

func newTestSearchHandler() *SearchHandler {
	return &SearchHandler{
		searcher:    &mockDocumentSearcher{},
		queryLister: nil,
		suggester:   nil,
		logger:      slog.New(slog.DiscardHandler),
	}
}

// newSearchHandlerWithSearcher creates a SearchHandler with a mock searcher.
func newSearchHandlerWithSearcher(ms *mockDocumentSearcher) *SearchHandler {
	return &SearchHandler{
		searcher:    ms,
		queryLister: nil,
		suggester:   nil,
		logger:      slog.New(slog.DiscardHandler),
	}
}

// ---------------------------------------------------------------------------
// NewSearchHandler constructor test
// ---------------------------------------------------------------------------

func TestNewSearchHandler(t *testing.T) {
	t.Parallel()

	ms := &mockDocumentSearcher{}
	ql := &mockQueryLister{}
	ts := &mockTitleSuggester{}
	logger := slog.New(slog.DiscardHandler)

	h := NewSearchHandler(ms, ql, ts, logger)

	require.NotNil(t, h, "NewSearchHandler should return a non-nil handler")
	assert.Equal(t, ms, h.searcher, "searcher should be set")
	assert.Equal(t, ql, h.queryLister, "queryLister should be set")
	assert.Equal(t, ts, h.suggester, "suggester should be set")
	assert.Equal(t, logger, h.logger, "logger should be set")
}

// ---------------------------------------------------------------------------
// Search handler tests
// ---------------------------------------------------------------------------

func TestSearchHandler_Search(t *testing.T) {
	t.Parallel()

	t.Run("returns 400 when query parameter q is missing", func(t *testing.T) {
		t.Parallel()

		h := newTestSearchHandler()
		req := httptest.NewRequest(http.MethodGet, "/api/search", http.NoBody)
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

	t.Run("returns 400 when q is empty string", func(t *testing.T) {
		t.Parallel()

		h := newTestSearchHandler()
		req := httptest.NewRequest(http.MethodGet, "/api/search?q=", http.NoBody)
		rr := httptest.NewRecorder()

		h.Search(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("response includes error field for bad request", func(t *testing.T) {
		t.Parallel()

		h := newTestSearchHandler()
		req := httptest.NewRequest(http.MethodGet, "/api/search", http.NoBody)
		rr := httptest.NewRecorder()

		h.Search(rr, req)

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if _, ok := body["error"]; !ok {
			t.Error("response missing 'error' key")
		}
	})

	t.Run("content type is application/json", func(t *testing.T) {
		t.Parallel()

		h := newTestSearchHandler()
		req := httptest.NewRequest(http.MethodGet, "/api/search", http.NoBody)
		rr := httptest.NewRecorder()

		h.Search(rr, req)

		if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
	})

	t.Run("accepts all valid file_type values and returns 200", func(t *testing.T) {
		t.Parallel()

		validTypes := []string{"pdf", "docx", "xlsx", "html", "markdown"}
		for _, ft := range validTypes {
			t.Run(ft, func(t *testing.T) {
				t.Parallel()

				var capturedFileType string
				ms := &mockDocumentSearcher{
					searchFn: func(_ context.Context, params search.SearchParams) (*search.SearchResponse, error) {
						capturedFileType = params.FileType
						return &search.SearchResponse{Hits: []search.SearchResult{}}, nil
					},
				}
				h := newSearchHandlerWithSearcher(ms)
				req := httptest.NewRequest(http.MethodGet, "/api/search?q=test&file_type="+ft, http.NoBody)
				rr := httptest.NewRecorder()

				h.Search(rr, req)

				assert.Equal(t, http.StatusOK, rr.Code)
				assert.Equal(t, ft, capturedFileType, "file_type should be passed to searcher")
			})
		}
	})

	t.Run("returns 400 for invalid file_type filter", func(t *testing.T) {
		t.Parallel()

		h := newTestSearchHandler()
		req := httptest.NewRequest(http.MethodGet, "/api/search?q=test&file_type=exe", http.NoBody)
		rr := httptest.NewRecorder()

		h.Search(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if msg := body["message"]; msg != "invalid file_type filter" {
			t.Errorf("message = %v, want 'invalid file_type filter'", msg)
		}
	})

	t.Run("returns 200 with search results on happy path", func(t *testing.T) {
		t.Parallel()

		ms := &mockDocumentSearcher{
			searchFn: func(_ context.Context, params search.SearchParams) (*search.SearchResponse, error) {
				assert.Equal(t, "golang tutorial", params.Query)
				assert.Equal(t, search.IndexDocuments, params.IndexUID)
				assert.Equal(t, int64(20), params.Limit)
				assert.Equal(t, int64(0), params.Offset)
				return &search.SearchResponse{
					Hits: []search.SearchResult{
						{UUID: "doc-1", Title: "Go Tutorial", Description: "Learn Go", Source: "document", Score: 0.9},
						{UUID: "doc-2", Title: "Golang Basics", Source: "document", Score: 0.7},
					},
					EstimatedTotal:   2,
					ProcessingTimeMs: 42,
				}, nil
			},
		}
		h := newSearchHandlerWithSearcher(ms)
		req := httptest.NewRequest(http.MethodGet, "/api/search?q=golang+tutorial", http.NoBody)
		rr := httptest.NewRecorder()

		h.Search(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

		var body map[string]any
		err := json.NewDecoder(rr.Body).Decode(&body)
		require.NoError(t, err)

		data, ok := body["data"].([]any)
		require.True(t, ok, "expected 'data' key with array value")
		assert.Len(t, data, 2)

		first := data[0].(map[string]any)
		assert.Equal(t, "doc-1", first["uuid"])
		assert.Equal(t, "Go Tutorial", first["title"])

		meta, ok := body["meta"].(map[string]any)
		require.True(t, ok, "expected 'meta' key")
		assert.Equal(t, "golang tutorial", meta["query"])
		assert.InEpsilon(t, float64(2), meta["total"], 1e-9)
		assert.InEpsilon(t, float64(42), meta["processing_time_ms"], 1e-9)
		assert.InEpsilon(t, float64(20), meta["limit"], 1e-9)
		assert.Equal(t, float64(0), meta["offset"])
	})

	t.Run("returns 500 when searcher returns error", func(t *testing.T) {
		t.Parallel()

		ms := &mockDocumentSearcher{
			searchFn: func(_ context.Context, _ search.SearchParams) (*search.SearchResponse, error) {
				return nil, errors.New("database unavailable")
			},
		}
		h := newSearchHandlerWithSearcher(ms)
		req := httptest.NewRequest(http.MethodGet, "/api/search?q=test", http.NoBody)
		rr := httptest.NewRecorder()

		h.Search(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		var body map[string]any
		err := json.NewDecoder(rr.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, "search failed", body["message"])
		assert.Equal(t, "Internal Server Error", body["error"])
	})

	t.Run("passes custom limit and offset to searcher", func(t *testing.T) {
		t.Parallel()

		var capturedLimit, capturedOffset int64
		ms := &mockDocumentSearcher{
			searchFn: func(_ context.Context, params search.SearchParams) (*search.SearchResponse, error) {
				capturedLimit = params.Limit
				capturedOffset = params.Offset
				return &search.SearchResponse{Hits: []search.SearchResult{}}, nil
			},
		}
		h := newSearchHandlerWithSearcher(ms)
		req := httptest.NewRequest(http.MethodGet, "/api/search?q=test&limit=50&offset=10", http.NoBody)
		rr := httptest.NewRecorder()

		h.Search(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, int64(50), capturedLimit)
		assert.Equal(t, int64(10), capturedOffset)
	})

	t.Run("clamps limit to maximum 100", func(t *testing.T) {
		t.Parallel()

		var capturedLimit int64
		ms := &mockDocumentSearcher{
			searchFn: func(_ context.Context, params search.SearchParams) (*search.SearchResponse, error) {
				capturedLimit = params.Limit
				return &search.SearchResponse{Hits: []search.SearchResult{}}, nil
			},
		}
		h := newSearchHandlerWithSearcher(ms)
		req := httptest.NewRequest(http.MethodGet, "/api/search?q=test&limit=999", http.NoBody)
		rr := httptest.NewRecorder()

		h.Search(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, int64(100), capturedLimit, "limit should be clamped to 100")
	})

	t.Run("uses default limit 20 when limit is zero", func(t *testing.T) {
		t.Parallel()

		var capturedLimit int64
		ms := &mockDocumentSearcher{
			searchFn: func(_ context.Context, params search.SearchParams) (*search.SearchResponse, error) {
				capturedLimit = params.Limit
				return &search.SearchResponse{Hits: []search.SearchResult{}}, nil
			},
		}
		h := newSearchHandlerWithSearcher(ms)
		req := httptest.NewRequest(http.MethodGet, "/api/search?q=test&limit=0", http.NoBody)
		rr := httptest.NewRecorder()

		h.Search(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, int64(20), capturedLimit, "limit=0 should default to 20")
	})

	t.Run("always sets IndexUID to documents", func(t *testing.T) {
		t.Parallel()

		var capturedIndex string
		ms := &mockDocumentSearcher{
			searchFn: func(_ context.Context, params search.SearchParams) (*search.SearchResponse, error) {
				capturedIndex = params.IndexUID
				return &search.SearchResponse{Hits: []search.SearchResult{}}, nil
			},
		}
		h := newSearchHandlerWithSearcher(ms)
		req := httptest.NewRequest(http.MethodGet, "/api/search?q=test", http.NoBody)
		rr := httptest.NewRecorder()

		h.Search(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, search.IndexDocuments, capturedIndex)
	})

	t.Run("returns empty data array when no results found", func(t *testing.T) {
		t.Parallel()

		ms := &mockDocumentSearcher{
			searchFn: func(_ context.Context, _ search.SearchParams) (*search.SearchResponse, error) {
				return &search.SearchResponse{
					Hits:             []search.SearchResult{},
					EstimatedTotal:   0,
					ProcessingTimeMs: 3,
				}, nil
			},
		}
		h := newSearchHandlerWithSearcher(ms)
		req := httptest.NewRequest(http.MethodGet, "/api/search?q=nonexistent", http.NoBody)
		rr := httptest.NewRecorder()

		h.Search(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		var body map[string]any
		err := json.NewDecoder(rr.Body).Decode(&body)
		require.NoError(t, err)
		data, ok := body["data"].([]any)
		require.True(t, ok)
		assert.Empty(t, data)
	})

	t.Run("does not pass file_type when not specified", func(t *testing.T) {
		t.Parallel()

		var capturedFileType string
		ms := &mockDocumentSearcher{
			searchFn: func(_ context.Context, params search.SearchParams) (*search.SearchResponse, error) {
				capturedFileType = params.FileType
				return &search.SearchResponse{Hits: []search.SearchResult{}}, nil
			},
		}
		h := newSearchHandlerWithSearcher(ms)
		req := httptest.NewRequest(http.MethodGet, "/api/search?q=test", http.NoBody)
		rr := httptest.NewRecorder()

		h.Search(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Empty(t, capturedFileType, "file_type should be empty when not specified")
	})

	t.Run("negative offset is clamped to zero", func(t *testing.T) {
		t.Parallel()

		var capturedOffset int64
		ms := &mockDocumentSearcher{
			searchFn: func(_ context.Context, params search.SearchParams) (*search.SearchResponse, error) {
				capturedOffset = params.Offset
				return &search.SearchResponse{Hits: []search.SearchResult{}}, nil
			},
		}
		h := newSearchHandlerWithSearcher(ms)
		req := httptest.NewRequest(http.MethodGet, "/api/search?q=test&offset=-5", http.NoBody)
		rr := httptest.NewRecorder()

		h.Search(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, int64(0), capturedOffset, "negative offset should be clamped to 0")
	})
}

// ---------------------------------------------------------------------------
// FederatedSearch handler tests
// ---------------------------------------------------------------------------

func TestSearchHandler_FederatedSearch(t *testing.T) {
	t.Parallel()

	t.Run("returns 400 when query parameter q is missing", func(t *testing.T) {
		t.Parallel()

		h := newTestSearchHandler()
		req := httptest.NewRequest(http.MethodGet, "/api/search/unified", http.NoBody)
		rr := httptest.NewRecorder()

		h.FederatedSearch(rr, req)

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

	t.Run("returns 400 when q is empty string", func(t *testing.T) {
		t.Parallel()

		h := newTestSearchHandler()
		req := httptest.NewRequest(http.MethodGet, "/api/search/unified?q=", http.NoBody)
		rr := httptest.NewRecorder()

		h.FederatedSearch(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("response includes error and message fields", func(t *testing.T) {
		t.Parallel()

		h := newTestSearchHandler()
		req := httptest.NewRequest(http.MethodGet, "/api/search/unified", http.NoBody)
		rr := httptest.NewRecorder()

		h.FederatedSearch(rr, req)

		var body map[string]any
		err := json.NewDecoder(rr.Body).Decode(&body)
		require.NoError(t, err)
		assert.Contains(t, body, "error", "response should have 'error' key")
		assert.Equal(t, "query parameter 'q' is required", body["message"])
	})

	t.Run("response format is JSON on error", func(t *testing.T) {
		t.Parallel()

		h := newTestSearchHandler()
		req := httptest.NewRequest(http.MethodGet, "/api/search/unified", http.NoBody)
		rr := httptest.NewRecorder()

		h.FederatedSearch(rr, req)

		if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("response is not valid JSON: %v", err)
		}
	})

	t.Run("limit zero is clamped to default 20", func(t *testing.T) {
		t.Parallel()

		var capturedLimit int64
		ms := &mockDocumentSearcher{
			federatedSearchFn: func(_ context.Context, params search.FederatedSearchParams) (*search.FederatedSearchResponse, error) {
				capturedLimit = params.Limit
				return &search.FederatedSearchResponse{Hits: []search.SearchResult{}}, nil
			},
		}
		h := newSearchHandlerWithSearcher(ms)
		req := httptest.NewRequest(http.MethodGet, "/api/search/unified?q=test&limit=0", http.NoBody)
		rr := httptest.NewRecorder()

		h.FederatedSearch(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, int64(20), capturedLimit, "limit=0 should be clamped to default 20")
	})

	t.Run("limit over 100 is clamped to 100", func(t *testing.T) {
		t.Parallel()

		var capturedLimit int64
		ms := &mockDocumentSearcher{
			federatedSearchFn: func(_ context.Context, params search.FederatedSearchParams) (*search.FederatedSearchResponse, error) {
				capturedLimit = params.Limit
				return &search.FederatedSearchResponse{Hits: []search.SearchResult{}}, nil
			},
		}
		h := newSearchHandlerWithSearcher(ms)
		req := httptest.NewRequest(http.MethodGet, "/api/search/unified?q=test&limit=500", http.NoBody)
		rr := httptest.NewRecorder()

		h.FederatedSearch(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, int64(100), capturedLimit, "limit=500 should be clamped to 100")
	})

	t.Run("types=document maps to documents index", func(t *testing.T) {
		t.Parallel()

		var capturedIndexes []string
		ms := &mockDocumentSearcher{
			federatedSearchFn: func(_ context.Context, params search.FederatedSearchParams) (*search.FederatedSearchResponse, error) {
				capturedIndexes = params.Indexes
				return &search.FederatedSearchResponse{Hits: []search.SearchResult{}}, nil
			},
		}
		h := newSearchHandlerWithSearcher(ms)
		req := httptest.NewRequest(http.MethodGet, "/api/search/unified?q=test&types=document", http.NoBody)
		rr := httptest.NewRecorder()

		h.FederatedSearch(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, []string{search.IndexDocuments}, capturedIndexes)
	})

	t.Run("types=zim_archive maps to zim archives index", func(t *testing.T) {
		t.Parallel()

		var capturedIndexes []string
		ms := &mockDocumentSearcher{
			federatedSearchFn: func(_ context.Context, params search.FederatedSearchParams) (*search.FederatedSearchResponse, error) {
				capturedIndexes = params.Indexes
				return &search.FederatedSearchResponse{Hits: []search.SearchResult{}}, nil
			},
		}
		h := newSearchHandlerWithSearcher(ms)
		req := httptest.NewRequest(http.MethodGet, "/api/search/unified?q=test&types=zim_archive", http.NoBody)
		rr := httptest.NewRecorder()

		h.FederatedSearch(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, []string{search.IndexZimArchives}, capturedIndexes)
	})

	t.Run("types=git_template maps to git templates index", func(t *testing.T) {
		t.Parallel()

		var capturedIndexes []string
		ms := &mockDocumentSearcher{
			federatedSearchFn: func(_ context.Context, params search.FederatedSearchParams) (*search.FederatedSearchResponse, error) {
				capturedIndexes = params.Indexes
				return &search.FederatedSearchResponse{Hits: []search.SearchResult{}}, nil
			},
		}
		h := newSearchHandlerWithSearcher(ms)
		req := httptest.NewRequest(http.MethodGet, "/api/search/unified?q=test&types=git_template", http.NoBody)
		rr := httptest.NewRecorder()

		h.FederatedSearch(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, []string{search.IndexGitTemplates}, capturedIndexes)
	})

	t.Run("multiple types in comma-separated list are all mapped", func(t *testing.T) {
		t.Parallel()

		var capturedIndexes []string
		ms := &mockDocumentSearcher{
			federatedSearchFn: func(_ context.Context, params search.FederatedSearchParams) (*search.FederatedSearchResponse, error) {
				capturedIndexes = params.Indexes
				return &search.FederatedSearchResponse{Hits: []search.SearchResult{}}, nil
			},
		}
		h := newSearchHandlerWithSearcher(ms)
		req := httptest.NewRequest(http.MethodGet,
			"/api/search/unified?q=test&types=document,zim_archive,git_template", http.NoBody)
		rr := httptest.NewRecorder()

		h.FederatedSearch(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, []string{search.IndexDocuments, search.IndexZimArchives, search.IndexGitTemplates}, capturedIndexes)
	})

	t.Run("unknown type is silently ignored", func(t *testing.T) {
		t.Parallel()

		var capturedIndexes []string
		ms := &mockDocumentSearcher{
			federatedSearchFn: func(_ context.Context, params search.FederatedSearchParams) (*search.FederatedSearchResponse, error) {
				capturedIndexes = params.Indexes
				return &search.FederatedSearchResponse{Hits: []search.SearchResult{}}, nil
			},
		}
		h := newSearchHandlerWithSearcher(ms)
		req := httptest.NewRequest(http.MethodGet, "/api/search/unified?q=test&types=unknown_type", http.NoBody)
		rr := httptest.NewRecorder()

		h.FederatedSearch(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Empty(t, capturedIndexes, "unknown types should be ignored, resulting in empty indexes")
	})

	t.Run("returns search results with correct response structure", func(t *testing.T) {
		t.Parallel()

		ms := &mockDocumentSearcher{
			federatedSearchFn: func(_ context.Context, _ search.FederatedSearchParams) (*search.FederatedSearchResponse, error) {
				return &search.FederatedSearchResponse{
					Hits: []search.SearchResult{
						{UUID: "abc-123", Title: "Go Guide", Description: "A guide", Source: "document", Score: 0.95},
						{UUID: "def-456", Title: "Docker Intro", Source: "zim_archive", Score: 0.80},
					},
					EstimatedTotal:   2,
					ProcessingTimeMs: 15,
				}, nil
			},
		}
		h := newSearchHandlerWithSearcher(ms)
		req := httptest.NewRequest(http.MethodGet, "/api/search/unified?q=go&limit=10&offset=5", http.NoBody)
		rr := httptest.NewRecorder()

		h.FederatedSearch(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var body map[string]any
		err := json.NewDecoder(rr.Body).Decode(&body)
		require.NoError(t, err)

		data, ok := body["data"].([]any)
		require.True(t, ok)
		assert.Len(t, data, 2)

		meta, ok := body["meta"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "go", meta["query"])
		assert.InEpsilon(t, float64(2), meta["total"], 1e-9)
		assert.InEpsilon(t, float64(15), meta["processing_time_ms"], 1e-9)
		assert.InEpsilon(t, float64(10), meta["limit"], 1e-9)
		assert.InEpsilon(t, float64(5), meta["offset"], 1e-9)
	})

	t.Run("returns 500 when searcher fails", func(t *testing.T) {
		t.Parallel()

		ms := &mockDocumentSearcher{
			federatedSearchFn: func(_ context.Context, _ search.FederatedSearchParams) (*search.FederatedSearchResponse, error) {
				return nil, errors.New("connection refused")
			},
		}
		h := newSearchHandlerWithSearcher(ms)
		req := httptest.NewRequest(http.MethodGet, "/api/search/unified?q=test", http.NoBody)
		rr := httptest.NewRecorder()

		h.FederatedSearch(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		var body map[string]any
		err := json.NewDecoder(rr.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, "search failed", body["message"])
	})

	t.Run("no types parameter results in nil indexes", func(t *testing.T) {
		t.Parallel()

		var capturedIndexes []string
		ms := &mockDocumentSearcher{
			federatedSearchFn: func(_ context.Context, params search.FederatedSearchParams) (*search.FederatedSearchResponse, error) {
				capturedIndexes = params.Indexes
				return &search.FederatedSearchResponse{Hits: []search.SearchResult{}}, nil
			},
		}
		h := newSearchHandlerWithSearcher(ms)
		req := httptest.NewRequest(http.MethodGet, "/api/search/unified?q=test", http.NoBody)
		rr := httptest.NewRecorder()

		h.FederatedSearch(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Nil(t, capturedIndexes, "no types param should pass nil indexes to searcher")
	})
}

// ---------------------------------------------------------------------------
// Mock implementations for searchQueryLister and titleSuggester
// ---------------------------------------------------------------------------

type mockQueryLister struct {
	popularQueriesFn func(ctx context.Context, limit int) ([]repository.PopularQuery, error)
}

func (m *mockQueryLister) PopularQueries(ctx context.Context, limit int) ([]repository.PopularQuery, error) {
	if m.popularQueriesFn != nil {
		return m.popularQueriesFn(ctx, limit)
	}
	return nil, nil
}

type mockTitleSuggester struct {
	suggestTitlesFn func(ctx context.Context, prefix string, limit int) ([]repository.TitleSuggestion, error)
}

func (m *mockTitleSuggester) SuggestTitles(ctx context.Context, prefix string, limit int) ([]repository.TitleSuggestion, error) {
	if m.suggestTitlesFn != nil {
		return m.suggestTitlesFn(ctx, prefix, limit)
	}
	return nil, nil
}

// newSearchHandlerWithMocks creates a SearchHandler with the given mocks.
// The searcher field is nil (only safe for Popular/Autocomplete tests).
func newSearchHandlerWithMocks(ql searchQueryLister, ts titleSuggester) *SearchHandler {
	return &SearchHandler{
		searcher:    nil,
		queryLister: ql,
		suggester:   ts,
		logger:      slog.New(slog.DiscardHandler),
	}
}

// ---------------------------------------------------------------------------
// Popular handler tests
// ---------------------------------------------------------------------------

func TestSearchHandler_Popular(t *testing.T) {
	t.Parallel()

	t.Run("returns 200 with popular queries using default limit", func(t *testing.T) {
		t.Parallel()

		var capturedLimit int
		ql := &mockQueryLister{
			popularQueriesFn: func(_ context.Context, limit int) ([]repository.PopularQuery, error) {
				capturedLimit = limit
				return []repository.PopularQuery{
					{Query: "golang", Count: 42},
					{Query: "docker", Count: 30},
					{Query: "kubernetes", Count: 25},
				}, nil
			},
		}
		h := newSearchHandlerWithMocks(ql, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/search/popular", http.NoBody)
		rr := httptest.NewRecorder()

		h.Popular(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, 10, capturedLimit, "default limit should be 10")

		var body map[string]any
		err := json.NewDecoder(rr.Body).Decode(&body)
		require.NoError(t, err)
		dataRaw, ok := body["data"].([]any)
		require.True(t, ok, "expected 'data' key with array value")
		assert.Len(t, dataRaw, 3)
		first := dataRaw[0].(map[string]any)
		assert.Equal(t, "golang", first["query"])
		assert.InEpsilon(t, float64(42), first["count"], 1e-9)
	})

	t.Run("respects custom limit parameter", func(t *testing.T) {
		t.Parallel()

		var capturedLimit int
		ql := &mockQueryLister{
			popularQueriesFn: func(_ context.Context, limit int) ([]repository.PopularQuery, error) {
				capturedLimit = limit
				return []repository.PopularQuery{
					{Query: "test", Count: 10},
				}, nil
			},
		}
		h := newSearchHandlerWithMocks(ql, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/search/popular?limit=25", http.NoBody)
		rr := httptest.NewRecorder()

		h.Popular(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, 25, capturedLimit, "custom limit should be respected")
	})

	t.Run("passes explicit limit=5 to query lister", func(t *testing.T) {
		t.Parallel()

		var capturedLimit int
		ql := &mockQueryLister{
			popularQueriesFn: func(_ context.Context, limit int) ([]repository.PopularQuery, error) {
				capturedLimit = limit
				return []repository.PopularQuery{
					{Query: "go", Count: 5},
				}, nil
			},
		}
		h := newSearchHandlerWithMocks(ql, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/search/popular?limit=5", http.NoBody)
		rr := httptest.NewRecorder()

		h.Popular(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, 5, capturedLimit, "explicit limit=5 should be passed through")
	})

	t.Run("caps limit at 50", func(t *testing.T) {
		t.Parallel()

		var capturedLimit int
		ql := &mockQueryLister{
			popularQueriesFn: func(_ context.Context, limit int) ([]repository.PopularQuery, error) {
				capturedLimit = limit
				return nil, nil
			},
		}
		h := newSearchHandlerWithMocks(ql, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/search/popular?limit=200", http.NoBody)
		rr := httptest.NewRecorder()

		h.Popular(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, 50, capturedLimit, "limit should be capped at 50")
	})

	t.Run("uses default limit for zero value", func(t *testing.T) {
		t.Parallel()

		var capturedLimit int
		ql := &mockQueryLister{
			popularQueriesFn: func(_ context.Context, limit int) ([]repository.PopularQuery, error) {
				capturedLimit = limit
				return nil, nil
			},
		}
		h := newSearchHandlerWithMocks(ql, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/search/popular?limit=0", http.NoBody)
		rr := httptest.NewRecorder()

		h.Popular(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, 10, capturedLimit, "zero limit should fall back to default 10")
	})

	t.Run("uses default limit for negative value", func(t *testing.T) {
		t.Parallel()

		var capturedLimit int
		ql := &mockQueryLister{
			popularQueriesFn: func(_ context.Context, limit int) ([]repository.PopularQuery, error) {
				capturedLimit = limit
				return nil, nil
			},
		}
		h := newSearchHandlerWithMocks(ql, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/search/popular?limit=-5", http.NoBody)
		rr := httptest.NewRecorder()

		h.Popular(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, 10, capturedLimit, "negative limit should fall back to default 10")
	})

	t.Run("returns 500 when query lister fails", func(t *testing.T) {
		t.Parallel()

		ql := &mockQueryLister{
			popularQueriesFn: func(_ context.Context, _ int) ([]repository.PopularQuery, error) {
				return nil, errors.New("database connection lost")
			},
		}
		h := newSearchHandlerWithMocks(ql, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/search/popular", http.NoBody)
		rr := httptest.NewRecorder()

		h.Popular(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		var body map[string]any
		err := json.NewDecoder(rr.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, "failed to fetch popular queries", body["message"])
	})

	t.Run("uses default limit for non-numeric limit param", func(t *testing.T) {
		t.Parallel()

		var capturedLimit int
		ql := &mockQueryLister{
			popularQueriesFn: func(_ context.Context, limit int) ([]repository.PopularQuery, error) {
				capturedLimit = limit
				return nil, nil
			},
		}
		h := newSearchHandlerWithMocks(ql, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/search/popular?limit=abc", http.NoBody)
		rr := httptest.NewRecorder()

		h.Popular(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, 10, capturedLimit, "non-numeric limit should fall back to default 10")
	})

	t.Run("response content-type is application/json", func(t *testing.T) {
		t.Parallel()

		ql := &mockQueryLister{
			popularQueriesFn: func(_ context.Context, _ int) ([]repository.PopularQuery, error) {
				return []repository.PopularQuery{}, nil
			},
		}
		h := newSearchHandlerWithMocks(ql, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/search/popular", http.NoBody)
		rr := httptest.NewRecorder()

		h.Popular(rr, req)

		assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	})

	t.Run("returns empty array when no popular queries exist", func(t *testing.T) {
		t.Parallel()

		ql := &mockQueryLister{
			popularQueriesFn: func(_ context.Context, _ int) ([]repository.PopularQuery, error) {
				return []repository.PopularQuery{}, nil
			},
		}
		h := newSearchHandlerWithMocks(ql, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/search/popular", http.NoBody)
		rr := httptest.NewRecorder()

		h.Popular(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var body map[string]any
		err := json.NewDecoder(rr.Body).Decode(&body)
		require.NoError(t, err)
		dataRaw, ok := body["data"].([]any)
		require.True(t, ok, "expected 'data' key with array value")
		assert.Empty(t, dataRaw)
	})
}

// ---------------------------------------------------------------------------
// Autocomplete handler tests
// ---------------------------------------------------------------------------

func TestSearchHandler_Autocomplete(t *testing.T) {
	t.Parallel()

	t.Run("returns 400 when query parameter is missing", func(t *testing.T) {
		t.Parallel()

		h := newSearchHandlerWithMocks(nil, &mockTitleSuggester{})

		req := httptest.NewRequest(http.MethodGet, "/api/search/autocomplete", http.NoBody)
		rr := httptest.NewRecorder()

		h.Autocomplete(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		var body map[string]any
		err := json.NewDecoder(rr.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, "query parameter must be between 2 and 100 characters", body["message"])
	})

	t.Run("returns 400 when query is too short (1 char)", func(t *testing.T) {
		t.Parallel()

		h := newSearchHandlerWithMocks(nil, &mockTitleSuggester{})

		req := httptest.NewRequest(http.MethodGet, "/api/search/autocomplete?query=a", http.NoBody)
		rr := httptest.NewRecorder()

		h.Autocomplete(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		var body map[string]any
		err := json.NewDecoder(rr.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, "query parameter must be between 2 and 100 characters", body["message"])
	})

	t.Run("returns 400 when query exceeds 100 characters", func(t *testing.T) {
		t.Parallel()

		h := newSearchHandlerWithMocks(nil, &mockTitleSuggester{})

		req := httptest.NewRequest(http.MethodGet, "/api/search/autocomplete?query="+
			strings.Repeat("x", 101), http.NoBody)
		rr := httptest.NewRecorder()

		h.Autocomplete(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("returns 200 with suggestions and highlighted titles", func(t *testing.T) {
		t.Parallel()

		ts := &mockTitleSuggester{
			suggestTitlesFn: func(_ context.Context, prefix string, limit int) ([]repository.TitleSuggestion, error) {
				assert.Equal(t, "go", prefix)
				assert.Equal(t, 5, limit, "default limit should be 5")
				return []repository.TitleSuggestion{
					{UUID: "uuid-1", Title: "Golang Guide"},
					{UUID: "uuid-2", Title: "Go Patterns"},
					{UUID: "uuid-3", Title: "Getting Started"},
				}, nil
			},
		}
		h := newSearchHandlerWithMocks(nil, ts)

		req := httptest.NewRequest(http.MethodGet, "/api/search/autocomplete?query=go", http.NoBody)
		rr := httptest.NewRecorder()

		h.Autocomplete(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var body map[string]any
		err := json.NewDecoder(rr.Body).Decode(&body)
		require.NoError(t, err)
		dataRaw, ok := body["data"].([]any)
		require.True(t, ok, "expected 'data' key with array value")
		require.Len(t, dataRaw, 3)

		r0 := dataRaw[0].(map[string]any)
		assert.Equal(t, "uuid-1", r0["uuid"])
		assert.Equal(t, "Golang Guide", r0["title"])
		assert.Equal(t, "<em>Go</em>lang Guide", r0["highlighted_title"])

		r1 := dataRaw[1].(map[string]any)
		assert.Equal(t, "uuid-2", r1["uuid"])
		assert.Equal(t, "Go Patterns", r1["title"])
		assert.Equal(t, "<em>Go</em> Patterns", r1["highlighted_title"])

		// "Getting Started" does not start with "go", so no highlight
		r2 := dataRaw[2].(map[string]any)
		assert.Equal(t, "uuid-3", r2["uuid"])
		assert.Equal(t, "Getting Started", r2["title"])
		assert.Equal(t, "Getting Started", r2["highlighted_title"])
	})

	t.Run("respects custom limit parameter", func(t *testing.T) {
		t.Parallel()

		var capturedLimit int
		ts := &mockTitleSuggester{
			suggestTitlesFn: func(_ context.Context, _ string, limit int) ([]repository.TitleSuggestion, error) {
				capturedLimit = limit
				return nil, nil
			},
		}
		h := newSearchHandlerWithMocks(nil, ts)

		req := httptest.NewRequest(http.MethodGet, "/api/search/autocomplete?query=te&limit=8", http.NoBody)
		rr := httptest.NewRecorder()

		h.Autocomplete(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, 8, capturedLimit)
	})

	t.Run("caps limit at 10", func(t *testing.T) {
		t.Parallel()

		var capturedLimit int
		ts := &mockTitleSuggester{
			suggestTitlesFn: func(_ context.Context, _ string, limit int) ([]repository.TitleSuggestion, error) {
				capturedLimit = limit
				return nil, nil
			},
		}
		h := newSearchHandlerWithMocks(nil, ts)

		req := httptest.NewRequest(http.MethodGet, "/api/search/autocomplete?query=te&limit=100", http.NoBody)
		rr := httptest.NewRecorder()

		h.Autocomplete(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, 10, capturedLimit, "limit should be capped at 10")
	})

	t.Run("uses default limit of 5 when not specified", func(t *testing.T) {
		t.Parallel()

		var capturedLimit int
		ts := &mockTitleSuggester{
			suggestTitlesFn: func(_ context.Context, _ string, limit int) ([]repository.TitleSuggestion, error) {
				capturedLimit = limit
				return nil, nil
			},
		}
		h := newSearchHandlerWithMocks(nil, ts)

		req := httptest.NewRequest(http.MethodGet, "/api/search/autocomplete?query=test", http.NoBody)
		rr := httptest.NewRecorder()

		h.Autocomplete(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, 5, capturedLimit, "default limit should be 5")
	})

	t.Run("uses default limit for non-numeric limit param", func(t *testing.T) {
		t.Parallel()

		var capturedLimit int
		ts := &mockTitleSuggester{
			suggestTitlesFn: func(_ context.Context, _ string, limit int) ([]repository.TitleSuggestion, error) {
				capturedLimit = limit
				return nil, nil
			},
		}
		h := newSearchHandlerWithMocks(nil, ts)

		req := httptest.NewRequest(http.MethodGet, "/api/search/autocomplete?query=test&limit=xyz", http.NoBody)
		rr := httptest.NewRecorder()

		h.Autocomplete(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, 5, capturedLimit, "non-numeric limit should fall back to default 5")
	})

	t.Run("response content-type is application/json", func(t *testing.T) {
		t.Parallel()

		ts := &mockTitleSuggester{
			suggestTitlesFn: func(_ context.Context, _ string, _ int) ([]repository.TitleSuggestion, error) {
				return []repository.TitleSuggestion{}, nil
			},
		}
		h := newSearchHandlerWithMocks(nil, ts)

		req := httptest.NewRequest(http.MethodGet, "/api/search/autocomplete?query=test", http.NoBody)
		rr := httptest.NewRecorder()

		h.Autocomplete(rr, req)

		assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	})

	t.Run("returns 500 when suggester fails", func(t *testing.T) {
		t.Parallel()

		ts := &mockTitleSuggester{
			suggestTitlesFn: func(_ context.Context, _ string, _ int) ([]repository.TitleSuggestion, error) {
				return nil, errors.New("database timeout")
			},
		}
		h := newSearchHandlerWithMocks(nil, ts)

		req := httptest.NewRequest(http.MethodGet, "/api/search/autocomplete?query=test", http.NoBody)
		rr := httptest.NewRecorder()

		h.Autocomplete(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		var body map[string]any
		err := json.NewDecoder(rr.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, "failed to fetch suggestions", body["message"])
	})

	t.Run("returns empty array when no suggestions found", func(t *testing.T) {
		t.Parallel()

		ts := &mockTitleSuggester{
			suggestTitlesFn: func(_ context.Context, _ string, _ int) ([]repository.TitleSuggestion, error) {
				return []repository.TitleSuggestion{}, nil
			},
		}
		h := newSearchHandlerWithMocks(nil, ts)

		req := httptest.NewRequest(http.MethodGet, "/api/search/autocomplete?query=zzz", http.NoBody)
		rr := httptest.NewRecorder()

		h.Autocomplete(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var body map[string]any
		err := json.NewDecoder(rr.Body).Decode(&body)
		require.NoError(t, err)
		dataRaw, ok := body["data"].([]any)
		require.True(t, ok, "expected 'data' key with array value")
		assert.Empty(t, dataRaw)
	})

	t.Run("accepts maximum valid query length of 100", func(t *testing.T) {
		t.Parallel()

		query100 := strings.Repeat("a", 100)
		ts := &mockTitleSuggester{
			suggestTitlesFn: func(_ context.Context, prefix string, _ int) ([]repository.TitleSuggestion, error) {
				assert.Equal(t, query100, prefix)
				return []repository.TitleSuggestion{}, nil
			},
		}
		h := newSearchHandlerWithMocks(nil, ts)

		req := httptest.NewRequest(http.MethodGet, "/api/search/autocomplete?query="+query100, http.NoBody)
		rr := httptest.NewRecorder()

		h.Autocomplete(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code, "query with exactly 100 chars should be accepted")
	})

	t.Run("accepts minimum valid query length of 2", func(t *testing.T) {
		t.Parallel()

		ts := &mockTitleSuggester{
			suggestTitlesFn: func(_ context.Context, prefix string, _ int) ([]repository.TitleSuggestion, error) {
				assert.Equal(t, "ab", prefix)
				return []repository.TitleSuggestion{
					{UUID: "uuid-1", Title: "About Us"},
				}, nil
			},
		}
		h := newSearchHandlerWithMocks(nil, ts)

		req := httptest.NewRequest(http.MethodGet, "/api/search/autocomplete?query=ab", http.NoBody)
		rr := httptest.NewRecorder()

		h.Autocomplete(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

// ---------------------------------------------------------------------------
// highlightPrefix tests
// ---------------------------------------------------------------------------

func TestHighlightPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		title  string
		prefix string
		want   string
	}{
		{
			name:   "wraps matching prefix in em tags",
			title:  "Golang Guide",
			prefix: "Go",
			want:   "<em>Go</em>lang Guide",
		},
		{
			name:   "returns original title when prefix does not match",
			title:  "Docker Tutorial",
			prefix: "Go",
			want:   "Docker Tutorial",
		},
		{
			name:   "case-insensitive match preserves original case",
			title:  "Golang Guide",
			prefix: "go",
			want:   "<em>Go</em>lang Guide",
		},
		{
			name:   "returns original title for empty prefix",
			title:  "Golang Guide",
			prefix: "",
			want:   "Golang Guide",
		},
		{
			name:   "prefix longer than title returns title unchanged",
			title:  "Go",
			prefix: "Golang",
			want:   "Go",
		},
		{
			name:   "exact match wraps entire title",
			title:  "Go",
			prefix: "Go",
			want:   "<em>Go</em>",
		},
		{
			name:   "full title match case-insensitive",
			title:  "HELLO",
			prefix: "hello",
			want:   "<em>HELLO</em>",
		},
		{
			name:   "single character prefix match",
			title:  "Apple",
			prefix: "A",
			want:   "<em>A</em>pple",
		},
		{
			name:   "prefix with spaces",
			title:  "Getting Started Guide",
			prefix: "Getting St",
			want:   "<em>Getting St</em>arted Guide",
		},
		{
			name:   "non-matching single character",
			title:  "Banana",
			prefix: "C",
			want:   "Banana",
		},
		// XSS prevention tests
		{
			name:   "script tag in title and prefix is escaped",
			title:  "<script>alert(1)</script>",
			prefix: "<script>",
			want:   "<em>&lt;script&gt;</em>alert(1)&lt;/script&gt;",
		},
		{
			name:   "ampersand in title and prefix is escaped",
			title:  "R&D Report",
			prefix: "R&D",
			want:   "<em>R&amp;D</em> Report",
		},
		{
			name:   "normal title still highlights correctly",
			title:  "Hello World",
			prefix: "Hello",
			want:   "<em>Hello</em> World",
		},
		{
			name:   "empty prefix returns escaped title",
			title:  "<b>bold</b>",
			prefix: "",
			want:   "&lt;b&gt;bold&lt;/b&gt;",
		},
		{
			name:   "prefix longer than title returns escaped title",
			title:  "A&B",
			prefix: "A&B is long",
			want:   "A&amp;B",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := highlightPrefix(tt.title, tt.prefix)
			assert.Equal(t, tt.want, got)
		})
	}
}
