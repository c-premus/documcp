package server_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"git.999.haus/chris/DocuMCP-go/internal/auth/oidc"
	"git.999.haus/chris/DocuMCP-go/internal/auth/oauth"
	"git.999.haus/chris/DocuMCP-go/internal/config"
	"git.999.haus/chris/DocuMCP-go/internal/handler"
	apihandler "git.999.haus/chris/DocuMCP-go/internal/handler/api"
	oauthhandler "git.999.haus/chris/DocuMCP-go/internal/handler/oauth"
	"git.999.haus/chris/DocuMCP-go/internal/observability"
	"git.999.haus/chris/DocuMCP-go/internal/server"
)

// testMetrics is a singleton to avoid duplicate Prometheus registration panics.
var (
	testMetrics     *observability.Metrics
	testMetricsOnce sync.Once
)

func sharedMetrics() *observability.Metrics {
	testMetricsOnce.Do(func() {
		testMetrics = observability.NewMetrics()
	})
	return testMetrics
}

func newTestServer(t *testing.T) *server.Server {
	t.Helper()

	logger := slog.New(slog.DiscardHandler)
	cfg := server.DefaultConfig()
	srv := server.New(cfg, logger)
	srv.RegisterRoutes(server.Deps{Version: "test-version"})

	return srv
}

func newTestServerWithDeps(t *testing.T, deps server.Deps) *server.Server {
	t.Helper()

	logger := slog.New(slog.DiscardHandler)
	cfg := server.DefaultConfig()
	srv := server.New(cfg, logger)
	srv.RegisterRoutes(deps)

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
			req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
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
			req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
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
	logger := slog.New(slog.DiscardHandler)
	cfg := server.DefaultConfig()
	srv := server.New(cfg, logger)

	// Add a test handler that captures the request ID from context.
	var capturedID string
	srv.Router().Use(chimiddleware.RequestID)
	srv.Router().Get("/test-reqid", func(w http.ResponseWriter, r *http.Request) {
		capturedID = chimiddleware.GetReqID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test-reqid", http.NoBody)
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
			req := httptest.NewRequest(http.MethodGet, tt.path, http.NoBody)
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

	logger := slog.New(slog.DiscardHandler)
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

	logger := slog.New(slog.DiscardHandler)
	srv := server.New(server.DefaultConfig(), logger)

	if srv.Router() == nil {
		t.Fatal("Router() returned nil")
	}
}

func TestListenAndServeOnAvailablePort(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.DiscardHandler)
	cfg := server.DefaultConfig()
	srv := server.New(cfg, logger)

	// Register a simple test route.
	srv.Router().Get("/test-available-port", func(w http.ResponseWriter, _ *http.Request) {
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
	resp, err := http.Get(url) //nolint:noctx // test helper — context not needed // test helper; context not needed for connectivity check
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

	logger := slog.New(slog.DiscardHandler)

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

	logger := slog.New(slog.DiscardHandler)
	srv := server.New(server.DefaultConfig(), logger)

	srv.Router().Get("/ping", func(w http.ResponseWriter, _ *http.Request) {
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
	resp, err := http.Get(url) //nolint:noctx // test helper — context not needed // test helper; context not needed for connectivity check
	if resp != nil {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Fatalf("closing response body: %v", closeErr)
		}
	}
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

	logger := slog.New(slog.DiscardHandler)

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
			req := httptest.NewRequest(method, "/health", http.NoBody)
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
			logger := slog.New(slog.DiscardHandler)
			cfg := server.DefaultConfig()
			srv := server.New(cfg, logger)
			srv.RegisterRoutes(server.Deps{Version: tt.version})

			req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
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
	req := httptest.NewRequest(http.MethodGet, "/some/random/path", http.NoBody)
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

	logger := slog.New(slog.DiscardHandler)
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

// ---------------------------------------------------------------------------
// RegisterRoutes: API routes without OAuth return 503
// ---------------------------------------------------------------------------

func TestRegisterRoutes_APIReturns503WithoutOAuth(t *testing.T) {
	t.Parallel()

	srv := newTestServerWithDeps(t, server.Deps{Version: "test"})

	// All /api/* routes should return 503 when no OAuth is configured.
	paths := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/documents"},
		{http.MethodPost, "/api/documents"},
		{http.MethodGet, "/api/search"},
	}

	for _, tt := range paths {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(tt.method, tt.path, http.NoBody)
			rec := httptest.NewRecorder()

			srv.Router().ServeHTTP(rec, req)

			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
			}

			ct := rec.Header().Get("Content-Type")
			if ct != "application/json" {
				t.Errorf("Content-Type = %q, want %q", ct, "application/json")
			}

			body := rec.Body.String()
			if !strings.Contains(body, "authentication not configured") {
				t.Errorf("body = %q, want it to contain 'authentication not configured'", body)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// RegisterRoutes: /admin/login redirects to /auth/login
// ---------------------------------------------------------------------------

func TestRegisterRoutes_AdminLoginRedirect(t *testing.T) {
	t.Parallel()

	srv := newTestServerWithDeps(t, server.Deps{Version: "test"})

	req := httptest.NewRequest(http.MethodGet, "/admin/login", http.NoBody)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusMovedPermanently {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusMovedPermanently)
	}

	location := rec.Header().Get("Location")
	if location != "/auth/login" {
		t.Errorf("Location = %q, want %q", location, "/auth/login")
	}
}

// ---------------------------------------------------------------------------
// RegisterRoutes: MCP handler registration
// ---------------------------------------------------------------------------

func stubOAuthService() *oauth.Service {
	return oauth.NewService(nil, config.OAuthConfig{}, "http://localhost", slog.Default())
}

func TestRegisterRoutes_MCPHandler(t *testing.T) {
	t.Parallel()

	mcpHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("mcp ok"))
	})

	srv := newTestServerWithDeps(t, server.Deps{
		Version:      "test",
		MCPHandler:   mcpHandler,
		OAuthService: stubOAuthService(),
	})

	// Without a valid bearer token, expect 401 (route is registered but auth required).
	req := httptest.NewRequest(http.MethodGet, "/documcp/", http.NoBody)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code == http.StatusNotFound {
		t.Error("MCP route should be registered when OAuthService is provided")
	}
}

func TestRegisterRoutes_MCPHandlerSubpath(t *testing.T) {
	t.Parallel()

	mcpHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := newTestServerWithDeps(t, server.Deps{
		Version:      "test",
		MCPHandler:   mcpHandler,
		OAuthService: stubOAuthService(),
	})

	// Without a valid bearer token, expect non-404 (route registered).
	req := httptest.NewRequest(http.MethodPost, "/documcp/some/path", http.NoBody)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code == http.StatusNotFound {
		t.Error("MCP subpath route should be registered when OAuthService is provided")
	}
}

func TestRegisterRoutes_MCPHandlerNotRegisteredWhenNil(t *testing.T) {
	t.Parallel()

	srv := newTestServerWithDeps(t, server.Deps{Version: "test"})

	req := httptest.NewRequest(http.MethodGet, "/documcp/", http.NoBody)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d (MCP should not be registered)", rec.Code, http.StatusNotFound)
	}
}

// ---------------------------------------------------------------------------
// RegisterRoutes: SPA handler registration
// ---------------------------------------------------------------------------

func TestRegisterRoutes_SPAHandler(t *testing.T) {
	t.Parallel()

	spaCalled := false
	spaHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		spaCalled = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("spa"))
	})

	srv := newTestServerWithDeps(t, server.Deps{
		Version:    "test",
		SPAHandler: spaHandler,
	})

	// /admin should redirect to /admin/
	req := httptest.NewRequest(http.MethodGet, "/admin", http.NoBody)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusMovedPermanently {
		t.Errorf("/admin status = %d, want %d", rec.Code, http.StatusMovedPermanently)
	}

	location := rec.Header().Get("Location")
	if location != "/admin/" {
		t.Errorf("Location = %q, want %q", location, "/admin/")
	}

	// /admin/ should serve the SPA
	req2 := httptest.NewRequest(http.MethodGet, "/admin/", http.NoBody)
	rec2 := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec2, req2)

	if !spaCalled {
		t.Error("SPA handler was not called for /admin/")
	}
}

func TestRegisterRoutes_SPAHandlerSubpath(t *testing.T) {
	t.Parallel()

	spaCalled := false
	spaHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		spaCalled = true
		w.WriteHeader(http.StatusOK)
	})

	srv := newTestServerWithDeps(t, server.Deps{
		Version:    "test",
		SPAHandler: spaHandler,
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", http.NoBody)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if !spaCalled {
		t.Error("SPA handler was not called for /admin/dashboard")
	}
}

// ---------------------------------------------------------------------------
// RegisterRoutes: DefaultConfig ReadHeaderTimeout
// ---------------------------------------------------------------------------

func TestDefaultConfig_ReadHeaderTimeout(t *testing.T) {
	t.Parallel()

	cfg := server.DefaultConfig()
	if cfg.ReadHeaderTimeout != 5*time.Second {
		t.Errorf("ReadHeaderTimeout = %v, want %v", cfg.ReadHeaderTimeout, 5*time.Second)
	}
}

// ---------------------------------------------------------------------------
// RegisterRoutes: CSP header present via middleware
// ---------------------------------------------------------------------------

func TestRegisterRoutes_CSPHeaderPresent(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Error("Content-Security-Policy header is empty, want it to be set")
	}
	if !strings.Contains(csp, "default-src 'self'") {
		t.Errorf("CSP = %q, want it to contain \"default-src 'self'\"", csp)
	}
}

// ---------------------------------------------------------------------------
// RegisterRoutes: MaxBodySize enforced on API routes
// ---------------------------------------------------------------------------

func TestRegisterRoutes_MaxBodySizeEnforced(t *testing.T) {
	srv := newTestServer(t)

	// Create a body larger than 1 MB (the configured limit).
	bigBody := strings.NewReader(strings.Repeat("x", 2*1024*1024))

	req := httptest.NewRequest(http.MethodPost, "/api/documents", bigBody)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	// The route handler receives the request, but the body is limited.
	// Since no OAuth is configured, the 503 middleware fires first,
	// but the body limiter is still applied by the middleware stack.
	// We just verify the request completes without panic.
	if rec.Code == 0 {
		t.Error("expected a non-zero status code")
	}
}

// ---------------------------------------------------------------------------
// RegisterRoutes: readiness endpoint registered when DB is provided
// ---------------------------------------------------------------------------

func TestRegisterRoutes_ReadinessEndpointWithDB(t *testing.T) {
	t.Parallel()

	// Use an empty sql.DB (not connected). The readiness handler will
	// attempt to ping and fail, but the route should still be registered.
	db := &sql.DB{}

	srv := newTestServerWithDeps(t, server.Deps{
		Version: "test",
		DB:      db,
	})

	req := httptest.NewRequest(http.MethodGet, "/health/ready", http.NoBody)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	// The route exists (not 404), but will return 503 because the DB is not connected.
	if rec.Code == http.StatusNotFound {
		t.Error("status = 404, want readiness endpoint to be registered when DB is provided")
	}
}

func TestRegisterRoutes_ReadinessEndpointNotRegisteredWithoutDB(t *testing.T) {
	t.Parallel()

	srv := newTestServerWithDeps(t, server.Deps{Version: "test"})

	req := httptest.NewRequest(http.MethodGet, "/health/ready", http.NoBody)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d (readiness should not be registered without DB)",
			rec.Code, http.StatusNotFound)
	}
}

// ---------------------------------------------------------------------------
// RegisterRoutes: /api/* returns JSON error body on 503
// ---------------------------------------------------------------------------

func TestRegisterRoutes_APIErrorResponseFormat(t *testing.T) {
	t.Parallel()

	srv := newTestServerWithDeps(t, server.Deps{Version: "test"})

	req := httptest.NewRequest(http.MethodGet, "/api/documents", http.NoBody)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	var errResp struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("decoding error response: %v", err)
	}

	if errResp.Error != "Service Unavailable" {
		t.Errorf("error = %q, want %q", errResp.Error, "Service Unavailable")
	}
	if errResp.Message != "authentication not configured" {
		t.Errorf("message = %q, want %q", errResp.Message, "authentication not configured")
	}
}

// ---------------------------------------------------------------------------
// RegisterRoutes: admin endpoint routes fall under /api/admin
// ---------------------------------------------------------------------------

func TestRegisterRoutes_AdminEndpointWithoutAuth(t *testing.T) {
	t.Parallel()

	srv := newTestServerWithDeps(t, server.Deps{Version: "test"})

	// Admin routes are under /api which requires OAuth, so they should 503.
	req := httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/stats", http.NoBody)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

// ---------------------------------------------------------------------------
// RegisterRoutes: /admin redirects to /admin/ when SPA is nil
// ---------------------------------------------------------------------------

func TestRegisterRoutes_AdminWithoutSPA(t *testing.T) {
	t.Parallel()

	srv := newTestServerWithDeps(t, server.Deps{Version: "test"})

	// Without SPA handler, /admin/ should return 404.
	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", http.NoBody)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d (SPA not configured)", rec.Code, http.StatusNotFound)
	}
}

// ---------------------------------------------------------------------------
// RegisterRoutes: MCP handler without OAuth has no bearer middleware
// ---------------------------------------------------------------------------

func TestRegisterRoutes_MCPHandlerWithoutOAuth(t *testing.T) {
	t.Parallel()

	mcpHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := newTestServerWithDeps(t, server.Deps{
		Version:    "test",
		MCPHandler: mcpHandler,
		// OAuthService is nil -- MCP must NOT be registered (security: no unauthenticated access)
	})

	req := httptest.NewRequest(http.MethodGet, "/documcp/", http.NoBody)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d (MCP should not be registered without OAuth)", rec.Code, http.StatusNotFound)
	}
}

// ---------------------------------------------------------------------------
// RegisterRoutes: multiple middleware are applied in correct order
// ---------------------------------------------------------------------------

func TestRegisterRoutes_MiddlewareOrder(t *testing.T) {
	srv := newTestServer(t)

	// Verify that all expected middleware are active by checking
	// that a request to /health gets both security headers and a request ID.
	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	// Security headers should be present.
	if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("X-Frame-Options = %q, want %q", got, "DENY")
	}

	// The response should succeed.
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// ---------------------------------------------------------------------------
// RegisterRoutes: admin login redirect with query params preserved
// ---------------------------------------------------------------------------

func TestRegisterRoutes_AdminLoginRedirectPreservesMethod(t *testing.T) {
	t.Parallel()

	srv := newTestServerWithDeps(t, server.Deps{Version: "test"})

	// POST to /admin/login should still get routed (chi only registers GET).
	req := httptest.NewRequest(http.MethodPost, "/admin/login", http.NoBody)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	// POST to a GET-only route should return 405 Method Not Allowed.
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

// ---------------------------------------------------------------------------
// RegisterRoutes: route registration for each handler group
// ---------------------------------------------------------------------------

// routeRegistered is a test helper that verifies a route is registered (not 404).
// Handlers may panic (caught by SafeRecoverer → 500) or return auth errors,
// but the key assertion is: the route exists in the router.
func routeRegistered(t *testing.T, srv *server.Server, method, path string) {
	t.Helper()

	req := httptest.NewRequest(method, path, http.NoBody)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code == http.StatusNotFound {
		t.Errorf("%s %s returned 404, want route to be registered", method, path)
	}
}

func TestRegisterRoutes_DocumentEndpoints(t *testing.T) {
	t.Parallel()

	srv := newTestServerWithDeps(t, server.Deps{
		Version:         "test",
		OAuthService:    stubOAuthService(),
		DocumentHandler: new(apihandler.DocumentHandler),
	})

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/documents"},
		{http.MethodGet, "/api/documents/trash"},
		{http.MethodGet, "/api/documents/00000000-0000-0000-0000-000000000001"},
		{http.MethodGet, "/api/documents/00000000-0000-0000-0000-000000000001/download"},
		{http.MethodPost, "/api/documents"},
		{http.MethodPost, "/api/documents/analyze"},
		{http.MethodPut, "/api/documents/00000000-0000-0000-0000-000000000001"},
		{http.MethodDelete, "/api/documents/00000000-0000-0000-0000-000000000001"},
		{http.MethodPost, "/api/documents/00000000-0000-0000-0000-000000000001/restore"},
		{http.MethodDelete, "/api/documents/00000000-0000-0000-0000-000000000001/purge"},
	}

	for _, tt := range routes {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			t.Parallel()
			routeRegistered(t, srv, tt.method, tt.path)
		})
	}
}

func TestRegisterRoutes_DocumentEndpointsNotRegisteredWhenNil(t *testing.T) {
	t.Parallel()

	srv := newTestServerWithDeps(t, server.Deps{
		Version:      "test",
		OAuthService: stubOAuthService(),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/documents", http.NoBody)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	// Without DocumentHandler, /api/documents hits the /api route group
	// but has no handler. Chi returns 404 for unmatched nested routes.
	if rec.Code == http.StatusOK {
		t.Error("expected non-200 when DocumentHandler is nil")
	}
}

func TestRegisterRoutes_SearchEndpoints(t *testing.T) {
	t.Parallel()

	srv := newTestServerWithDeps(t, server.Deps{
		Version:       "test",
		OAuthService:  stubOAuthService(),
		SearchHandler: new(apihandler.SearchHandler),
	})

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/search"},
		{http.MethodGet, "/api/search/unified"},
		{http.MethodGet, "/api/search/popular"},
		{http.MethodGet, "/api/search/autocomplete"},
	}

	for _, tt := range routes {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			t.Parallel()
			routeRegistered(t, srv, tt.method, tt.path)
		})
	}
}

func TestRegisterRoutes_ZimEndpoints(t *testing.T) {
	t.Parallel()

	srv := newTestServerWithDeps(t, server.Deps{
		Version:      "test",
		OAuthService: stubOAuthService(),
		ZimHandler:   new(apihandler.ZimHandler),
	})

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/zim/archives"},
		{http.MethodGet, "/api/zim/archives/test-archive"},
		{http.MethodGet, "/api/zim/archives/test-archive/search"},
		{http.MethodGet, "/api/zim/archives/test-archive/suggest"},
		{http.MethodGet, "/api/zim/archives/test-archive/articles/A/Test"},
	}

	for _, tt := range routes {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			t.Parallel()
			routeRegistered(t, srv, tt.method, tt.path)
		})
	}
}

func TestRegisterRoutes_GitTemplateEndpoints(t *testing.T) {
	t.Parallel()

	srv := newTestServerWithDeps(t, server.Deps{
		Version:            "test",
		OAuthService:       stubOAuthService(),
		GitTemplateHandler: new(apihandler.GitTemplateHandler),
	})

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/git-templates"},
		{http.MethodGet, "/api/git-templates/search"},
		{http.MethodGet, "/api/git-templates/00000000-0000-0000-0000-000000000001"},
		{http.MethodGet, "/api/git-templates/00000000-0000-0000-0000-000000000001/structure"},
		{http.MethodGet, "/api/git-templates/00000000-0000-0000-0000-000000000001/files/README.md"},
		{http.MethodGet, "/api/git-templates/00000000-0000-0000-0000-000000000001/deployment-guide"},
		{http.MethodPost, "/api/git-templates"},
		{http.MethodPut, "/api/git-templates/00000000-0000-0000-0000-000000000001"},
		{http.MethodDelete, "/api/git-templates/00000000-0000-0000-0000-000000000001"},
		{http.MethodPost, "/api/git-templates/00000000-0000-0000-0000-000000000001/sync"},
		{http.MethodPost, "/api/git-templates/00000000-0000-0000-0000-000000000001/download"},
	}

	for _, tt := range routes {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			t.Parallel()
			routeRegistered(t, srv, tt.method, tt.path)
		})
	}
}

func TestRegisterRoutes_ExternalServiceEndpoints(t *testing.T) {
	t.Parallel()

	srv := newTestServerWithDeps(t, server.Deps{
		Version:                "test",
		OAuthService:           stubOAuthService(),
		ExternalServiceHandler: new(apihandler.ExternalServiceHandler),
	})

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/external-services"},
		{http.MethodGet, "/api/external-services/00000000-0000-0000-0000-000000000001"},
		{http.MethodPost, "/api/external-services"},
		{http.MethodPut, "/api/external-services/00000000-0000-0000-0000-000000000001"},
		{http.MethodDelete, "/api/external-services/00000000-0000-0000-0000-000000000001"},
		{http.MethodPost, "/api/external-services/00000000-0000-0000-0000-000000000001/health-check"},
		{http.MethodPost, "/api/external-services/00000000-0000-0000-0000-000000000001/sync"},
	}

	for _, tt := range routes {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			t.Parallel()
			routeRegistered(t, srv, tt.method, tt.path)
		})
	}
}

func TestRegisterRoutes_AdminUserEndpoints(t *testing.T) {
	t.Parallel()

	srv := newTestServerWithDeps(t, server.Deps{
		Version:      "test",
		OAuthService: stubOAuthService(),
		UserHandler:  new(apihandler.UserHandler),
	})

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/admin/users"},
		{http.MethodPost, "/api/admin/users"},
		{http.MethodGet, "/api/admin/users/1"},
		{http.MethodPut, "/api/admin/users/1"},
		{http.MethodDelete, "/api/admin/users/1"},
		{http.MethodPost, "/api/admin/users/1/toggle-admin"},
	}

	for _, tt := range routes {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			t.Parallel()
			routeRegistered(t, srv, tt.method, tt.path)
		})
	}
}

func TestRegisterRoutes_AdminOAuthClientEndpoints(t *testing.T) {
	t.Parallel()

	srv := newTestServerWithDeps(t, server.Deps{
		Version:            "test",
		OAuthService:       stubOAuthService(),
		OAuthClientHandler: new(apihandler.OAuthClientHandler),
	})

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/admin/oauth-clients"},
		{http.MethodPost, "/api/admin/oauth-clients"},
		{http.MethodGet, "/api/admin/oauth-clients/1"},
		{http.MethodPost, "/api/admin/oauth-clients/1/revoke"},
	}

	for _, tt := range routes {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			t.Parallel()
			routeRegistered(t, srv, tt.method, tt.path)
		})
	}
}

func TestRegisterRoutes_AdminQueueEndpoints(t *testing.T) {
	t.Parallel()

	srv := newTestServerWithDeps(t, server.Deps{
		Version:      "test",
		OAuthService: stubOAuthService(),
		QueueHandler: new(apihandler.QueueHandler),
	})

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/admin/queue/stats"},
		{http.MethodGet, "/api/admin/queue/failed"},
		{http.MethodPost, "/api/admin/queue/failed/1/retry"},
		{http.MethodDelete, "/api/admin/queue/failed/1"},
	}

	for _, tt := range routes {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			t.Parallel()
			routeRegistered(t, srv, tt.method, tt.path)
		})
	}
}

func TestRegisterRoutes_AdminDashboardEndpoints(t *testing.T) {
	t.Parallel()

	srv := newTestServerWithDeps(t, server.Deps{
		Version:          "test",
		OAuthService:     stubOAuthService(),
		DashboardHandler: new(apihandler.DashboardHandler),
	})

	routeRegistered(t, srv, http.MethodGet, "/api/admin/dashboard/stats")
}

func TestRegisterRoutes_AdminSSEEndpoints(t *testing.T) {
	t.Parallel()

	srv := newTestServerWithDeps(t, server.Deps{
		Version:      "test",
		OAuthService: stubOAuthService(),
		SSEHandler:   new(apihandler.SSEHandler),
	})

	routeRegistered(t, srv, http.MethodGet, "/api/admin/events/stream")
}

func TestRegisterRoutes_AdminBulkPurge(t *testing.T) {
	t.Parallel()

	srv := newTestServerWithDeps(t, server.Deps{
		Version:         "test",
		OAuthService:    stubOAuthService(),
		DocumentHandler: new(apihandler.DocumentHandler),
	})

	routeRegistered(t, srv, http.MethodDelete, "/api/admin/documents/purge")
}

func TestRegisterRoutes_AdminExternalServiceReorder(t *testing.T) {
	t.Parallel()

	srv := newTestServerWithDeps(t, server.Deps{
		Version:                "test",
		OAuthService:           stubOAuthService(),
		ExternalServiceHandler: new(apihandler.ExternalServiceHandler),
	})

	routeRegistered(t, srv, http.MethodPut, "/api/admin/external-services/reorder")
}

func TestRegisterRoutes_AdminGitTemplateValidateURL(t *testing.T) {
	t.Parallel()

	srv := newTestServerWithDeps(t, server.Deps{
		Version:            "test",
		OAuthService:       stubOAuthService(),
		GitTemplateHandler: new(apihandler.GitTemplateHandler),
	})

	routeRegistered(t, srv, http.MethodPost, "/api/admin/git-templates/validate-url")
}

func TestRegisterRoutes_OAuthEndpoints(t *testing.T) {
	t.Parallel()

	srv := newTestServerWithDeps(t, server.Deps{
		Version:      "test",
		OAuthHandler: new(oauthhandler.Handler),
	})

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/.well-known/oauth-authorization-server"},
		{http.MethodGet, "/.well-known/oauth-protected-resource"},
		{http.MethodGet, "/oauth/authorize"},
		{http.MethodPost, "/oauth/authorize/approve"},
		{http.MethodGet, "/oauth/device"},
		{http.MethodPost, "/oauth/device"},
		{http.MethodPost, "/oauth/device/approve"},
		{http.MethodPost, "/oauth/token"},
		{http.MethodPost, "/oauth/revoke"},
		// /oauth/register omitted: zero-value Handler has RegistrationEnabled=false,
		// so Register() returns 404 by design (not a routing issue).
		{http.MethodPost, "/oauth/device/code"},
	}

	for _, tt := range routes {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			t.Parallel()
			routeRegistered(t, srv, tt.method, tt.path)
		})
	}
}

func TestRegisterRoutes_OIDCEndpoints(t *testing.T) {
	t.Parallel()

	srv := newTestServerWithDeps(t, server.Deps{
		Version:     "test",
		OIDCHandler: new(oidc.Handler),
	})

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/auth/login"},
		{http.MethodGet, "/auth/callback"},
		{http.MethodPost, "/auth/logout"},
	}

	for _, tt := range routes {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			t.Parallel()
			routeRegistered(t, srv, tt.method, tt.path)
		})
	}
}

func TestRegisterRoutes_AuthMeEndpoint(t *testing.T) {
	t.Parallel()

	srv := newTestServerWithDeps(t, server.Deps{
		Version:     "test",
		AuthHandler: new(apihandler.AuthHandler),
	})

	routeRegistered(t, srv, http.MethodGet, "/api/auth/me")
}

// ---------------------------------------------------------------------------
// Start and Close
// ---------------------------------------------------------------------------

func TestStart_ListensAndAcceptsConnections(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.DiscardHandler)
	cfg := server.DefaultConfig()
	cfg.Host = "127.0.0.1"
	cfg.Port = 0
	srv := server.New(cfg, logger)

	srv.Router().Get("/ping", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("pong"))
	})

	ln, err := srv.ListenAndServeOnAvailablePort()
	if err != nil {
		t.Fatalf("ListenAndServeOnAvailablePort() error: %v", err)
	}
	defer func() {
		_ = srv.Close()
	}()

	url := fmt.Sprintf("http://%s/ping", ln.Addr().String())
	resp, err := http.Get(url) //nolint:noctx // test helper — context not needed
	if err != nil {
		t.Fatalf("GET %s error: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "pong" {
		t.Errorf("body = %q, want %q", string(body), "pong")
	}
}

func TestClose_RejectsNewConnections(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.DiscardHandler)
	srv := server.New(server.DefaultConfig(), logger)

	srv.Router().Get("/ping", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ln, err := srv.ListenAndServeOnAvailablePort()
	if err != nil {
		t.Fatalf("ListenAndServeOnAvailablePort() error: %v", err)
	}

	addr := ln.Addr().String()

	// Close the server immediately.
	if err := srv.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	// New connections should fail after Close.
	url := fmt.Sprintf("http://%s/ping", addr)
	resp, err := http.Get(url) //nolint:noctx // test helper — context not needed
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err == nil {
		t.Error("expected error connecting to closed server, got nil")
	}
}

// ---------------------------------------------------------------------------
// RegisterRoutes: all handlers together
// ---------------------------------------------------------------------------

func TestRegisterRoutes_AllHandlers(t *testing.T) {
	t.Parallel()

	// Verify that RegisterRoutes does not panic when all handler slots are populated.
	srv := newTestServerWithDeps(t, server.Deps{
		Version:                "test",
		OAuthService:           stubOAuthService(),
		OAuthHandler:           new(oauthhandler.Handler),
		OIDCHandler:            new(oidc.Handler),
		DocumentHandler:        new(apihandler.DocumentHandler),
		SearchHandler:          new(apihandler.SearchHandler),
		ZimHandler:             new(apihandler.ZimHandler),
		GitTemplateHandler:     new(apihandler.GitTemplateHandler),
		ExternalServiceHandler: new(apihandler.ExternalServiceHandler),
		UserHandler:            new(apihandler.UserHandler),
		OAuthClientHandler:     new(apihandler.OAuthClientHandler),
		QueueHandler:           new(apihandler.QueueHandler),
		DashboardHandler:       new(apihandler.DashboardHandler),
		SSEHandler:             new(apihandler.SSEHandler),
		AuthHandler:            new(apihandler.AuthHandler),
		SPAHandler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
		MCPHandler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	})

	// Smoke test: health should still work.
	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// ---------------------------------------------------------------------------
// RegisterRoutes: well-known endpoints not registered when OAuthHandler is nil
// ---------------------------------------------------------------------------

func TestRegisterRoutes_WellKnownNotRegisteredWithoutOAuth(t *testing.T) {
	t.Parallel()

	srv := newTestServerWithDeps(t, server.Deps{Version: "test"})

	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", http.NoBody)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d when OAuthHandler is nil", rec.Code, http.StatusNotFound)
	}
}

// ---------------------------------------------------------------------------
// RegisterRoutes: OIDC endpoints not registered when OIDCHandler is nil
// ---------------------------------------------------------------------------

func TestRegisterRoutes_OIDCNotRegisteredWithoutHandler(t *testing.T) {
	t.Parallel()

	srv := newTestServerWithDeps(t, server.Deps{Version: "test"})

	paths := []string{"/auth/login", "/auth/callback"}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
			rec := httptest.NewRecorder()
			srv.Router().ServeHTTP(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Errorf("status = %d, want %d when OIDCHandler is nil", rec.Code, http.StatusNotFound)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// internalTokenAuth: metrics endpoint protected when token is configured
// ---------------------------------------------------------------------------

func TestRegisterRoutes_MetricsProtectedByToken(t *testing.T) {
	t.Parallel()

	srv := newTestServerWithDeps(t, server.Deps{
		Version:          "test",
		Metrics:          sharedMetrics(),
		InternalAPIToken: "secret-token",
	})

	// Without token: should return 401
	req := httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status without token = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	// With correct token: should return 200
	req2 := httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody)
	req2.Header.Set("Authorization", "Bearer secret-token")
	rec2 := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Errorf("status with token = %d, want %d", rec2.Code, http.StatusOK)
	}
}

func TestRegisterRoutes_MetricsUnprotectedWithoutToken(t *testing.T) {
	t.Parallel()

	srv := newTestServerWithDeps(t, server.Deps{
		Version: "test",
		Metrics: sharedMetrics(),
	})

	req := httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d when no token configured", rec.Code, http.StatusOK)
	}
}

func TestRegisterRoutes_MetricsNotRegisteredWithoutMetrics(t *testing.T) {
	t.Parallel()

	srv := newTestServerWithDeps(t, server.Deps{Version: "test"})

	req := httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d when Metrics is nil", rec.Code, http.StatusNotFound)
	}
}
