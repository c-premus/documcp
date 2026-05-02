package app

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/c-premus/documcp/internal/config"
	"github.com/c-premus/documcp/internal/observability"
)

// newTestFoundation returns a *Foundation populated with the minimum fields
// newHealthServer reads. BarePgxPool and BareRedisClient stay nil — callers
// must not exercise /readyz or /health/ready in tests built on this helper.
func newTestFoundation(t *testing.T, token string, metrics *observability.Metrics) (*Foundation, *bytes.Buffer) {
	t.Helper()
	var logBuf bytes.Buffer
	cfg := &config.Config{}
	cfg.App.InternalAPIToken = token
	return &Foundation{
		Config:  cfg,
		Logger:  slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})),
		Metrics: metrics,
	}, &logBuf
}

// hitRoute issues a fake GET to mux for path and returns the recorder.
func hitRoute(t *testing.T, srv *http.Server, path, authHeader string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)
	return w
}

func TestNewHealthServer_LivenessAliases(t *testing.T) {
	t.Parallel()
	f, _ := newTestFoundation(t, "", nil)
	srv := newHealthServer(0, f)

	for _, path := range []string{"/healthz", "/health"} {
		w := hitRoute(t, srv, path, "")
		if w.Code != http.StatusOK {
			t.Errorf("GET %s: code = %d, want 200", path, w.Code)
		}
		if got := strings.TrimSpace(w.Body.String()); got != "ok" {
			t.Errorf("GET %s: body = %q, want %q", path, got, "ok")
		}
	}
}

func TestNewHealthServer_ReadinessAliasesRouted(t *testing.T) {
	t.Parallel()
	f, _ := newTestFoundation(t, "", nil)
	srv := newHealthServer(0, f)

	// We only verify routing: /readyz and /health/ready resolve to a non-nil,
	// non-default-NotFound handler. Calling them with a nil BarePgxPool would
	// panic, so we use mux.Handler(req) introspection instead.
	mux, ok := srv.Handler.(*http.ServeMux)
	if !ok {
		t.Fatalf("server handler is %T, want *http.ServeMux", srv.Handler)
	}
	for _, path := range []string{"/readyz", "/health/ready"} {
		req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
		_, pattern := mux.Handler(req)
		if pattern == "" {
			t.Errorf("GET %s: no pattern matched (route not registered)", path)
		}
	}
}

func TestNewHealthServer_MetricsRequiresToken(t *testing.T) {
	t.Parallel()
	f, _ := newTestFoundation(t, "secret-token", &observability.Metrics{})
	srv := newHealthServer(0, f)

	// No Authorization header → 401.
	w := hitRoute(t, srv, "/metrics", "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("GET /metrics without bearer: code = %d, want 401", w.Code)
	}

	// Wrong token → 401.
	w = hitRoute(t, srv, "/metrics", "Bearer wrong-token")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("GET /metrics with wrong bearer: code = %d, want 401", w.Code)
	}

	// Correct token → 200 + Prometheus content type.
	w = hitRoute(t, srv, "/metrics", "Bearer secret-token")
	if w.Code != http.StatusOK {
		t.Errorf("GET /metrics with correct bearer: code = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/plain") {
		t.Errorf("GET /metrics: Content-Type = %q, want text/plain prefix", ct)
	}
}

func TestNewHealthServer_MetricsUnauthenticatedWhenTokenEmpty(t *testing.T) {
	t.Parallel()
	f, logBuf := newTestFoundation(t, "", &observability.Metrics{})
	srv := newHealthServer(0, f)

	w := hitRoute(t, srv, "/metrics", "")
	if w.Code != http.StatusOK {
		t.Errorf("GET /metrics with empty token: code = %d, want 200 (unauthenticated exposure)", w.Code)
	}

	logs := logBuf.String()
	if !strings.Contains(logs, "metrics endpoint exposed without authentication") {
		t.Errorf("expected WARN log about unauthenticated metrics, got logs:\n%s", logs)
	}
}

func TestNewHealthServer_MetricsAbsentWhenMetricsNil(t *testing.T) {
	t.Parallel()
	f, _ := newTestFoundation(t, "secret", nil)
	srv := newHealthServer(0, f)

	w := hitRoute(t, srv, "/metrics", "")
	if w.Code != http.StatusNotFound {
		t.Errorf("GET /metrics with nil Metrics: code = %d, want 404", w.Code)
	}
}

