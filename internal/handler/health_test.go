package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/c-premus/documcp/internal/handler"
)

func TestHealthHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		version     string
		wantStatus  int
		wantVersion string
	}{
		{
			name:        "returns version string",
			version:     "1.0.0",
			wantStatus:  http.StatusOK,
			wantVersion: "1.0.0",
		},
		{
			name:        "handles empty version",
			version:     "",
			wantStatus:  http.StatusOK,
			wantVersion: "",
		},
		{
			name:        "handles dev version",
			version:     "dev-abc123",
			wantStatus:  http.StatusOK,
			wantVersion: "dev-abc123",
		},
		{
			name:        "handles semver with pre-release",
			version:     "2.1.0-beta.3+build.456",
			wantStatus:  http.StatusOK,
			wantVersion: "2.1.0-beta.3+build.456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := handler.NewHealthHandler(tt.version)

			req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			var got handler.HealthResponse
			if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
				t.Fatalf("decoding response: %v", err)
			}

			if got.Status != "ok" {
				t.Errorf("status field = %q, want %q", got.Status, "ok")
			}

			if got.Version != tt.wantVersion {
				t.Errorf("version = %q, want %q", got.Version, tt.wantVersion)
			}
		})
	}
}

func TestHealthHandler_ContentType(t *testing.T) {
	t.Parallel()

	h := handler.NewHealthHandler("1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
}

func TestHealthHandler_ResponseBody_IsValidJSON(t *testing.T) {
	t.Parallel()

	h := handler.NewHealthHandler("1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}

	// Verify only expected keys are present.
	expectedKeys := map[string]bool{"status": true, "version": true}
	for key := range raw {
		if !expectedKeys[key] {
			t.Errorf("unexpected key %q in response", key)
		}
	}
	for key := range expectedKeys {
		if _, ok := raw[key]; !ok {
			t.Errorf("missing expected key %q in response", key)
		}
	}
}

func TestHealthHandler_HTTPMethods(t *testing.T) {
	t.Parallel()

	// The handler serves any HTTP method; it always returns 200.
	// This documents the current behavior.
	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodHead,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			t.Parallel()

			h := handler.NewHealthHandler("1.0.0")
			req := httptest.NewRequest(method, "/health", http.NoBody)
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("method %s: status = %d, want %d", method, rec.Code, http.StatusOK)
			}
		})
	}
}

func TestNewHealthHandler(t *testing.T) {
	t.Parallel()

	h := handler.NewHealthHandler("test-version")
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
}
