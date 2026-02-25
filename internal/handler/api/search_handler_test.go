package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
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
		searcher: nil, // will cause panic if Search/FederatedSearch reaches Meilisearch call
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
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
