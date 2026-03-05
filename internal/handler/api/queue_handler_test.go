package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// QueueHandler tests
//
// The QueueHandler wraps a *queue.RiverClient whose methods require a live
// Postgres connection (River stores job state in the database). We cannot
// mock the underlying river.Client[pgx.Tx] because it is a concrete struct,
// not an interface.
//
// These tests focus on handler-level logic that runs BEFORE the River client
// is called: constructor sanity, URL parameter parsing, and error responses.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// NewQueueHandler tests
// ---------------------------------------------------------------------------

func TestNewQueueHandler(t *testing.T) {
	t.Parallel()

	t.Run("creates handler with nil river client", func(t *testing.T) {
		t.Parallel()

		h := NewQueueHandler(nil, testLogger())

		assert.NotNil(t, h, "NewQueueHandler should return a non-nil handler")
	})
}

// ---------------------------------------------------------------------------
// RetryFailed handler tests
// ---------------------------------------------------------------------------

func TestQueueHandler_RetryFailed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		idParam    string
		wantStatus int
		wantMsg    string
	}{
		{
			name:       "returns 400 for non-numeric id",
			idParam:    "abc",
			wantStatus: http.StatusBadRequest,
			wantMsg:    "invalid job id",
		},
		{
			name:       "returns 400 for empty id",
			idParam:    "",
			wantStatus: http.StatusBadRequest,
			wantMsg:    "invalid job id",
		},
		{
			name:       "returns 400 for floating point id",
			idParam:    "3.14",
			wantStatus: http.StatusBadRequest,
			wantMsg:    "invalid job id",
		},
		{
			name:       "returns 400 for negative overflow id",
			idParam:    "99999999999999999999",
			wantStatus: http.StatusBadRequest,
			wantMsg:    "invalid job id",
		},
		{
			name:       "returns 400 for special characters",
			idParam:    "1;DROP TABLE jobs",
			wantStatus: http.StatusBadRequest,
			wantMsg:    "invalid job id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := NewQueueHandler(nil, testLogger())
			// Use a clean URL; the id param is injected via chiContext.
			req := httptest.NewRequest(http.MethodPost, "/api/admin/queue/failed/0/retry", nil)
			req = chiContext(req, map[string]string{"id": tt.idParam})
			rr := httptest.NewRecorder()

			h.RetryFailed(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)
			assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

			var body map[string]any
			err := json.NewDecoder(rr.Body).Decode(&body)
			require.NoError(t, err, "response should be valid JSON")
			assert.Equal(t, tt.wantMsg, body["message"])
			assert.Equal(t, http.StatusText(tt.wantStatus), body["error"])
		})
	}
}

// ---------------------------------------------------------------------------
// DeleteFailed handler tests
// ---------------------------------------------------------------------------

func TestQueueHandler_DeleteFailed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		idParam    string
		wantStatus int
		wantMsg    string
	}{
		{
			name:       "returns 400 for non-numeric id",
			idParam:    "xyz",
			wantStatus: http.StatusBadRequest,
			wantMsg:    "invalid job id",
		},
		{
			name:       "returns 400 for empty id",
			idParam:    "",
			wantStatus: http.StatusBadRequest,
			wantMsg:    "invalid job id",
		},
		{
			name:       "returns 400 for floating point id",
			idParam:    "1.5",
			wantStatus: http.StatusBadRequest,
			wantMsg:    "invalid job id",
		},
		{
			name:       "returns 400 for negative overflow id",
			idParam:    "99999999999999999999",
			wantStatus: http.StatusBadRequest,
			wantMsg:    "invalid job id",
		},
		{
			name:       "returns 400 for whitespace id",
			idParam:    "  ",
			wantStatus: http.StatusBadRequest,
			wantMsg:    "invalid job id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := NewQueueHandler(nil, testLogger())
			// Use a clean URL; the id param is injected via chiContext.
			req := httptest.NewRequest(http.MethodDelete, "/api/admin/queue/failed/0", nil)
			req = chiContext(req, map[string]string{"id": tt.idParam})
			rr := httptest.NewRecorder()

			h.DeleteFailed(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)
			assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

			var body map[string]any
			err := json.NewDecoder(rr.Body).Decode(&body)
			require.NoError(t, err, "response should be valid JSON")
			assert.Equal(t, tt.wantMsg, body["message"])
			assert.Equal(t, http.StatusText(tt.wantStatus), body["error"])
		})
	}
}

// ---------------------------------------------------------------------------
// Stats handler tests
// ---------------------------------------------------------------------------

func TestQueueHandler_Stats(t *testing.T) {
	t.Parallel()

	t.Run("panics with nil river client", func(t *testing.T) {
		t.Parallel()

		// When riverClient is nil, calling Stats will dereference a nil pointer.
		// This verifies the handler does not silently swallow the nil — it
		// requires a properly initialized RiverClient.
		h := NewQueueHandler(nil, testLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/admin/queue/stats", nil)
		rr := httptest.NewRecorder()

		assert.Panics(t, func() {
			h.Stats(rr, req)
		}, "Stats should panic when riverClient is nil")
	})
}

// ---------------------------------------------------------------------------
// ListFailed handler tests
// ---------------------------------------------------------------------------

func TestQueueHandler_ListFailed(t *testing.T) {
	t.Parallel()

	t.Run("panics with nil river client", func(t *testing.T) {
		t.Parallel()

		h := NewQueueHandler(nil, testLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/admin/queue/failed", nil)
		rr := httptest.NewRecorder()

		assert.Panics(t, func() {
			h.ListFailed(rr, req)
		}, "ListFailed should panic when riverClient is nil")
	})
}

// ---------------------------------------------------------------------------
// Error response format tests
// ---------------------------------------------------------------------------

func TestQueueHandler_ErrorResponseFormat(t *testing.T) {
	t.Parallel()

	t.Run("error response contains error and message fields", func(t *testing.T) {
		t.Parallel()

		h := NewQueueHandler(nil, testLogger())
		req := httptest.NewRequest(http.MethodPost, "/api/admin/queue/failed/bad/retry", nil)
		req = chiContext(req, map[string]string{"id": "bad"})
		rr := httptest.NewRecorder()

		h.RetryFailed(rr, req)

		var body map[string]any
		err := json.NewDecoder(rr.Body).Decode(&body)
		require.NoError(t, err)

		assert.Contains(t, body, "error", "response should have 'error' key")
		assert.Contains(t, body, "message", "response should have 'message' key")
		assert.Equal(t, "Bad Request", body["error"])
		assert.Equal(t, "invalid job id", body["message"])
	})

	t.Run("content type is application/json for error responses", func(t *testing.T) {
		t.Parallel()

		h := NewQueueHandler(nil, testLogger())
		req := httptest.NewRequest(http.MethodDelete, "/api/admin/queue/failed/notanumber", nil)
		req = chiContext(req, map[string]string{"id": "notanumber"})
		rr := httptest.NewRecorder()

		h.DeleteFailed(rr, req)

		assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	})
}
