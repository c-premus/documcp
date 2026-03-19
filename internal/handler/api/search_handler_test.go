package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"git.999.haus/chris/DocuMCP-go/internal/repository"
)

// ---------------------------------------------------------------------------
// SearchHandler tests
//
// The SearchHandler depends on *search.Searcher which wraps a concrete
// *search.Client (Meilisearch). Since we cannot mock the Meilisearch
// ServiceManager without a real server, we test the early-return error paths
// (parameter validation) and the response format logic.
// ---------------------------------------------------------------------------

func newTestSearchHandler() *SearchHandler {
	return &SearchHandler{
		searcher:    nil, // will cause panic if Search/FederatedSearch reaches Meilisearch call
		queryLister: nil,
		suggester:   nil,
		logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

// ---------------------------------------------------------------------------
// Search handler tests
// ---------------------------------------------------------------------------

func TestSearchHandler_Search(t *testing.T) {
	t.Parallel()

	t.Run("returns 400 when query parameter q is missing", func(t *testing.T) {
		t.Parallel()

		h := newTestSearchHandler()
		req := httptest.NewRequest(http.MethodGet, "/api/search", nil)
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
		req := httptest.NewRequest(http.MethodGet, "/api/search?q=", nil)
		rr := httptest.NewRecorder()

		h.Search(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("response includes error field for bad request", func(t *testing.T) {
		t.Parallel()

		h := newTestSearchHandler()
		req := httptest.NewRequest(http.MethodGet, "/api/search", nil)
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
		req := httptest.NewRequest(http.MethodGet, "/api/search", nil)
		rr := httptest.NewRecorder()

		h.Search(rr, req)

		if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
	})

	t.Run("returns 400 for invalid file_type filter", func(t *testing.T) {
		t.Parallel()

		h := newTestSearchHandler()
		req := httptest.NewRequest(http.MethodGet, "/api/search?q=test&file_type=exe", nil)
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
}

// ---------------------------------------------------------------------------
// FederatedSearch handler tests
// ---------------------------------------------------------------------------

func TestSearchHandler_FederatedSearch(t *testing.T) {
	t.Parallel()

	t.Run("returns 400 when query parameter q is missing", func(t *testing.T) {
		t.Parallel()

		h := newTestSearchHandler()
		req := httptest.NewRequest(http.MethodGet, "/api/search/unified", nil)
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
		req := httptest.NewRequest(http.MethodGet, "/api/search/unified?q=", nil)
		rr := httptest.NewRecorder()

		h.FederatedSearch(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("response format is JSON on error", func(t *testing.T) {
		t.Parallel()

		h := newTestSearchHandler()
		req := httptest.NewRequest(http.MethodGet, "/api/search/unified", nil)
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
		logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
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

		req := httptest.NewRequest(http.MethodGet, "/api/search/popular", nil)
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

		req := httptest.NewRequest(http.MethodGet, "/api/search/popular?limit=25", nil)
		rr := httptest.NewRecorder()

		h.Popular(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, 25, capturedLimit, "custom limit should be respected")
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

		req := httptest.NewRequest(http.MethodGet, "/api/search/popular?limit=200", nil)
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

		req := httptest.NewRequest(http.MethodGet, "/api/search/popular?limit=0", nil)
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

		req := httptest.NewRequest(http.MethodGet, "/api/search/popular?limit=-5", nil)
		rr := httptest.NewRecorder()

		h.Popular(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, 10, capturedLimit, "negative limit should fall back to default 10")
	})

	t.Run("returns 500 when query lister fails", func(t *testing.T) {
		t.Parallel()

		ql := &mockQueryLister{
			popularQueriesFn: func(_ context.Context, _ int) ([]repository.PopularQuery, error) {
				return nil, fmt.Errorf("database connection lost")
			},
		}
		h := newSearchHandlerWithMocks(ql, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/search/popular", nil)
		rr := httptest.NewRecorder()

		h.Popular(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		var body map[string]any
		err := json.NewDecoder(rr.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, "failed to fetch popular queries", body["message"])
	})

	t.Run("returns empty array when no popular queries exist", func(t *testing.T) {
		t.Parallel()

		ql := &mockQueryLister{
			popularQueriesFn: func(_ context.Context, _ int) ([]repository.PopularQuery, error) {
				return []repository.PopularQuery{}, nil
			},
		}
		h := newSearchHandlerWithMocks(ql, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/search/popular", nil)
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

		req := httptest.NewRequest(http.MethodGet, "/api/search/autocomplete", nil)
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

		req := httptest.NewRequest(http.MethodGet, "/api/search/autocomplete?query=a", nil)
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
			strings.Repeat("x", 101), nil)
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

		req := httptest.NewRequest(http.MethodGet, "/api/search/autocomplete?query=go", nil)
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

		req := httptest.NewRequest(http.MethodGet, "/api/search/autocomplete?query=te&limit=8", nil)
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

		req := httptest.NewRequest(http.MethodGet, "/api/search/autocomplete?query=te&limit=100", nil)
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

		req := httptest.NewRequest(http.MethodGet, "/api/search/autocomplete?query=test", nil)
		rr := httptest.NewRecorder()

		h.Autocomplete(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, 5, capturedLimit, "default limit should be 5")
	})

	t.Run("returns 500 when suggester fails", func(t *testing.T) {
		t.Parallel()

		ts := &mockTitleSuggester{
			suggestTitlesFn: func(_ context.Context, _ string, _ int) ([]repository.TitleSuggestion, error) {
				return nil, fmt.Errorf("database timeout")
			},
		}
		h := newSearchHandlerWithMocks(nil, ts)

		req := httptest.NewRequest(http.MethodGet, "/api/search/autocomplete?query=test", nil)
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

		req := httptest.NewRequest(http.MethodGet, "/api/search/autocomplete?query=zzz", nil)
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

		req := httptest.NewRequest(http.MethodGet, "/api/search/autocomplete?query=ab", nil)
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

