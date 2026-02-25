package server_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

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
