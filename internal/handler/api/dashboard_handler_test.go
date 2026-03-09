package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ---------------------------------------------------------------------------
// Mock counters
// ---------------------------------------------------------------------------

type mockCounter struct {
	count int
	err   error
}

func (m *mockCounter) Count(ctx context.Context) (int, error)      { return m.count, m.err }
func (m *mockCounter) CountUsers(ctx context.Context) (int, error)  { return m.count, m.err }
func (m *mockCounter) CountClients(ctx context.Context) (int, error) { return m.count, m.err }

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(nopWriter{}, nil))
}

type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) { return len(p), nil }

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestDashboardHandler_Stats_AllCounters(t *testing.T) {
	t.Parallel()

	h := NewDashboardHandler(
		&mockCounter{count: 10},
		&mockCounter{count: 5},
		&mockCounter{count: 3},
		&mockCounter{count: 2},
		&mockCounter{count: 7},
		&mockCounter{count: 4},
		&mockCounter{count: 6},
		nil, // no river client
		discardLogger(),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/stats", nil)
	rec := httptest.NewRecorder()

	h.Stats(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var envelope map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&envelope); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	resp, ok := envelope["data"].(map[string]any)
	if !ok {
		t.Fatal("expected 'data' key in response")
	}

	expected := map[string]float64{
		"documents":         10,
		"users":             5,
		"oauth_clients":     3,
		"external_services": 2,
		"zim_archives":      7,
		"confluence_spaces": 4,
		"git_templates":     6,
	}

	for key, want := range expected {
		got, ok := resp[key].(float64)
		if !ok {
			t.Errorf("key %q missing or not a number", key)
			continue
		}
		if got != want {
			t.Errorf("%s = %v, want %v", key, got, want)
		}
	}

	// No queue key when riverClient is nil.
	if _, ok := resp["queue"]; ok {
		t.Error("queue key should not be present when riverClient is nil")
	}
}

func TestDashboardHandler_Stats_NilCounters(t *testing.T) {
	t.Parallel()

	h := NewDashboardHandler(nil, nil, nil, nil, nil, nil, nil, nil, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/stats", nil)
	rec := httptest.NewRecorder()

	h.Stats(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var envelope map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&envelope); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	resp, ok := envelope["data"].(map[string]any)
	if !ok {
		t.Fatal("expected 'data' key in response")
	}

	for key, val := range resp {
		if val.(float64) != 0 {
			t.Errorf("%s = %v, want 0 for nil counter", key, val)
		}
	}
}

func TestDashboardHandler_Stats_CounterError(t *testing.T) {
	t.Parallel()

	h := NewDashboardHandler(
		&mockCounter{err: fmt.Errorf("db down")},
		&mockCounter{count: 5},
		nil, nil, nil, nil, nil,
		nil,
		discardLogger(),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/stats", nil)
	rec := httptest.NewRecorder()

	h.Stats(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var envelope map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&envelope); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	resp, ok := envelope["data"].(map[string]any)
	if !ok {
		t.Fatal("expected 'data' key in response")
	}

	// Document counter errored — should return 0.
	if resp["documents"].(float64) != 0 {
		t.Errorf("documents = %v, want 0 on error", resp["documents"])
	}

	// User counter worked.
	if resp["users"].(float64) != 5 {
		t.Errorf("users = %v, want 5", resp["users"])
	}
}

func TestDashboardHandler_Stats_ContentType(t *testing.T) {
	t.Parallel()

	h := NewDashboardHandler(nil, nil, nil, nil, nil, nil, nil, nil, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/stats", nil)
	rec := httptest.NewRecorder()

	h.Stats(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
}

func TestNewDashboardHandler(t *testing.T) {
	t.Parallel()

	h := NewDashboardHandler(nil, nil, nil, nil, nil, nil, nil, nil, discardLogger())
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
}
