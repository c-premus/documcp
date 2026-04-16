package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/c-premus/documcp/internal/handler"
)

// stubPinger implements handler.DependencyPinger for testing.
type stubPinger struct {
	err error
}

func (s *stubPinger) Ping(_ context.Context) error {
	return s.err
}

func TestReadinessHandler_NilDB(t *testing.T) {
	t.Parallel()

	h := handler.NewReadinessHandler("1.0.0", nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/ready", http.NoBody)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp handler.ReadinessResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	if resp.Status != "ready" {
		t.Errorf("status = %q, want %q", resp.Status, "ready")
	}
	if resp.Version != "1.0.0" {
		t.Errorf("version = %q, want %q", resp.Version, "1.0.0")
	}
	if len(resp.Services) != 0 {
		t.Errorf("services should be empty when DB is nil, got %v", resp.Services)
	}
}

func TestReadinessHandler_DBHealthy(t *testing.T) {
	t.Parallel()

	h := handler.NewReadinessHandler("2.0.0", &stubPinger{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/ready", http.NoBody)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp handler.ReadinessResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	if resp.Status != "ready" {
		t.Errorf("status = %q, want %q", resp.Status, "ready")
	}
	if resp.Services["postgres"] != "healthy" {
		t.Errorf("postgres = %q, want %q", resp.Services["postgres"], "healthy")
	}
}

func TestReadinessHandler_DBUnhealthy(t *testing.T) {
	t.Parallel()

	h := handler.NewReadinessHandler("3.0.0", &stubPinger{err: errors.New("connection refused")}, nil)

	req := httptest.NewRequest(http.MethodGet, "/ready", http.NoBody)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	var resp handler.ReadinessResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	if resp.Status != "not_ready" {
		t.Errorf("status = %q, want %q", resp.Status, "not_ready")
	}
	if resp.Services["postgres"] != "unhealthy" {
		t.Errorf("postgres = %q, want %q", resp.Services["postgres"], "unhealthy")
	}
}

func TestReadinessHandler_RedisUnhealthy(t *testing.T) {
	t.Parallel()

	h := handler.NewReadinessHandler("4.0.0",
		&stubPinger{},
		&stubPinger{err: errors.New("redis down")},
	)

	req := httptest.NewRequest(http.MethodGet, "/ready", http.NoBody)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	var resp handler.ReadinessResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	if resp.Services["postgres"] != "healthy" {
		t.Errorf("postgres = %q, want %q", resp.Services["postgres"], "healthy")
	}
	if resp.Services["redis"] != "unhealthy" {
		t.Errorf("redis = %q, want %q", resp.Services["redis"], "unhealthy")
	}
}

func TestReadinessHandler_ContentType(t *testing.T) {
	t.Parallel()

	h := handler.NewReadinessHandler("1.0.0", nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/ready", http.NoBody)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
}
