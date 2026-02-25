package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"git.999.haus/chris/DocuMCP-go/internal/handler"
)

func TestHealthHandler(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := handler.NewHealthHandler(tt.version)

			req := httptest.NewRequest(http.MethodGet, "/health", nil)
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
