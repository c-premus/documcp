package server_test

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
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"git.999.haus/chris/DocuMCP-go/internal/handler"
	"git.999.haus/chris/DocuMCP-go/internal/server"
)

func newTestServer(t *testing.T) *server.Server {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := server.DefaultConfig()
	srv := server.New(cfg, logger)
	srv.RegisterRoutes(server.Deps{Version: "test-version"})

	return srv
}

func TestHealthEndpoint(t *testing.T) {
	srv := newTestServer(t)

	tests := []struct {
		name       string
		wantStatus int
		wantBody   handler.HealthResponse
	}{
		{
			name:       "returns 200 with correct JSON",
			wantStatus: http.StatusOK,
			wantBody: handler.HealthResponse{
				Status:  "ok",
				Version: "test-version",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			rec := httptest.NewRecorder()

			srv.Router().ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			var got handler.HealthResponse
			if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
				t.Fatalf("decoding response body: %v", err)
			}

			if got != tt.wantBody {
				t.Errorf("body = %+v, want %+v", got, tt.wantBody)
			}

			contentType := rec.Header().Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
			}
		})
	}
}

func TestSecurityHeaders(t *testing.T) {
	srv := newTestServer(t)

	tests := []struct {
		header string
		want   string
	}{
		{"X-Frame-Options", "DENY"},
		{"X-Content-Type-Options", "nosniff"},
		{"X-XSS-Protection", "0"},
		{"Referrer-Policy", "strict-origin-when-cross-origin"},
		{"Permissions-Policy", "camera=(), microphone=(), geolocation=()"},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			rec := httptest.NewRecorder()

			srv.Router().ServeHTTP(rec, req)

			got := rec.Header().Get(tt.header)
			if got != tt.want {
				t.Errorf("%s = %q, want %q", tt.header, got, tt.want)
			}
		})
	}
}

func TestRequestID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := server.DefaultConfig()
	srv := server.New(cfg, logger)

	// Add a test handler that captures the request ID from context.
	var capturedID string
	srv.Router().Use(chimiddleware.RequestID)
	srv.Router().Get("/test-reqid", func(w http.ResponseWriter, r *http.Request) {
		capturedID = chimiddleware.GetReqID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test-reqid", nil)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if capturedID == "" {
		t.Error("request ID in context is empty, want a non-empty value")
	}
}

func TestNotFound(t *testing.T) {
	srv := newTestServer(t)

	tests := []struct {
		name string
		path string
	}{
		{"unknown root path", "/nonexistent"},
		{"unknown nested path", "/foo/bar/baz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			srv.Router().ServeHTTP(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := server.DefaultConfig()

	tests := []struct {
		name string
		got  any
		want any
	}{
		{"Host", cfg.Host, "0.0.0.0"},
		{"Port", cfg.Port, 8080},
		{"ReadTimeout", cfg.ReadTimeout, 5 * time.Second},
		{"WriteTimeout", cfg.WriteTimeout, 10 * time.Second},
		{"IdleTimeout", cfg.IdleTimeout, 120 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.got != tt.want {
				t.Errorf("DefaultConfig().%s = %v, want %v", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestNew_SetsAddr(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := server.Config{
		Host: "127.0.0.1",
		Port: 9999,
	}
	srv := server.New(cfg, logger)

	wantAddr := "127.0.0.1:9999"
	if got := srv.Addr(); got != wantAddr {
		t.Errorf("Addr() = %q, want %q", got, wantAddr)
	}
}

func TestNew_RouterNotNil(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := server.New(server.DefaultConfig(), logger)

	if srv.Router() == nil {
		t.Fatal("Router() returned nil")
	}
}

func TestListenAndServeOnAvailablePort(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := server.DefaultConfig()
	srv := server.New(cfg, logger)

	// Register a simple test route.
	srv.Router().Get("/test-available-port", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	ln, err := srv.ListenAndServeOnAvailablePort()
	if err != nil {
		t.Fatalf("ListenAndServeOnAvailablePort() returned unexpected error: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	// Verify the listener is actually listening.
	addr := ln.Addr().String()
	if addr == "" {
		t.Fatal("listener address is empty")
	}

	// The server address should be updated to match the listener.
	if srv.Addr() != addr {
		t.Errorf("Addr() = %q, want %q (should match listener)", srv.Addr(), addr)
	}

	// Verify we can actually make an HTTP request to the server.
	url := fmt.Sprintf("http://%s/test-available-port", addr)
	resp, err := http.Get(url) //nolint:noctx
	if err != nil {
		t.Fatalf("HTTP GET %s returned unexpected error: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("response status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading response body: %v", err)
	}
	if string(body) != "ok" {
		t.Errorf("response body = %q, want %q", string(body), "ok")
	}
}

func TestListenAndServeOnAvailablePort_RandomPort(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Create two servers; they should get different ports.
	srv1 := server.New(server.DefaultConfig(), logger)
	srv2 := server.New(server.DefaultConfig(), logger)

	ln1, err := srv1.ListenAndServeOnAvailablePort()
	if err != nil {
		t.Fatalf("srv1.ListenAndServeOnAvailablePort() returned unexpected error: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv1.Shutdown(ctx)
	}()

	ln2, err := srv2.ListenAndServeOnAvailablePort()
	if err != nil {
		t.Fatalf("srv2.ListenAndServeOnAvailablePort() returned unexpected error: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv2.Shutdown(ctx)
	}()

	if ln1.Addr().String() == ln2.Addr().String() {
		t.Errorf("two servers got the same address: %s", ln1.Addr().String())
	}
}

func TestShutdown_GracefulStop(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := server.New(server.DefaultConfig(), logger)

	srv.Router().Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	_, err := srv.ListenAndServeOnAvailablePort()
	if err != nil {
		t.Fatalf("ListenAndServeOnAvailablePort() returned unexpected error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = srv.Shutdown(ctx)
	if err != nil {
		t.Fatalf("Shutdown() returned unexpected error: %v", err)
	}

	// After shutdown, new requests should fail.
	url := fmt.Sprintf("http://%s/ping", srv.Addr())
	_, err = http.Get(url) //nolint:noctx
	if err == nil {
		t.Error("expected error connecting to shut-down server, got nil")
	}
}

func TestAddr_Format(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		host string
		port int
		want string
	}{
		{"default", "0.0.0.0", 8080, "0.0.0.0:8080"},
		{"localhost", "127.0.0.1", 3000, "127.0.0.1:3000"},
		{"empty host", "", 8080, ":8080"},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := server.Config{
				Host: tt.host,
				Port: tt.port,
			}
			srv := server.New(cfg, logger)

			if got := srv.Addr(); got != tt.want {
				t.Errorf("Addr() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHealthEndpoint_MethodNotAllowed(t *testing.T) {
	srv := newTestServer(t)

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/health", nil)
			rec := httptest.NewRecorder()

			srv.Router().ServeHTTP(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s /health status = %d, want %d", method, rec.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}

func TestHealthEndpoint_WithDifferentVersions(t *testing.T) {
	tests := []struct {
		version string
	}{
		{"1.0.0"},
		{""},
		{"v2.3.4-beta"},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			cfg := server.DefaultConfig()
			srv := server.New(cfg, logger)
			srv.RegisterRoutes(server.Deps{Version: tt.version})

			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			rec := httptest.NewRecorder()

			srv.Router().ServeHTTP(rec, req)

			var got handler.HealthResponse
			if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
				t.Fatalf("decoding response body: %v", err)
			}

			if got.Version != tt.version {
				t.Errorf("Version = %q, want %q", got.Version, tt.version)
			}
			if got.Status != "ok" {
				t.Errorf("Status = %q, want %q", got.Status, "ok")
			}
		})
	}
}

func TestSecurityHeaders_PresentOnAllEndpoints(t *testing.T) {
	srv := newTestServer(t)

	// Security headers should be on non-existent paths too.
	req := httptest.NewRequest(http.MethodGet, "/some/random/path", nil)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	// Even on 404 responses, security headers should be present.
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want %q on 404 response", got, "nosniff")
	}
	if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("X-Frame-Options = %q, want %q on 404 response", got, "DENY")
	}
}

func TestListenAndServeOnAvailablePort_PortIsNonZero(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := server.New(server.DefaultConfig(), logger)

	ln, err := srv.ListenAndServeOnAvailablePort()
	if err != nil {
		t.Fatalf("ListenAndServeOnAvailablePort() returned unexpected error: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	addr := ln.Addr().String()
	// Should NOT be :0 -- it should have a real port assigned.
	if strings.HasSuffix(addr, ":0") {
		t.Errorf("listener address %q ends with :0, expected an assigned port", addr)
	}
}
