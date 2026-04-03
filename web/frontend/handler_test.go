package frontend_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/c-premus/documcp/web/frontend"
)

func TestHandler(t *testing.T) {
	t.Parallel()

	handler := frontend.Handler()

	t.Run("handler is not nil", func(t *testing.T) {
		t.Parallel()
		if handler == nil {
			t.Fatal("expected Handler() to return a non-nil http.Handler")
		}
	})

	tests := []struct {
		name            string
		path            string
		wantStatus      int
		wantSubstring   string
		wantContentType string
	}{
		{
			name:            "root returns index.html with 200",
			path:            "/",
			wantStatus:      http.StatusOK,
			wantSubstring:   "<!doctype html>",
			wantContentType: "text/html",
		},
		{
			name:            "explicit index.html redirects to clean URL",
			path:            "/index.html",
			wantStatus:      http.StatusMovedPermanently,
			wantSubstring:   "",
			wantContentType: "",
		},
		{
			name:            "known static file favicon.ico returns 200",
			path:            "/favicon.ico",
			wantStatus:      http.StatusOK,
			wantSubstring:   "",
			wantContentType: "image/",
		},
		{
			name:            "known static file theme-init.js returns 200",
			path:            "/theme-init.js",
			wantStatus:      http.StatusOK,
			wantSubstring:   "",
			wantContentType: "javascript",
		},
		{
			name:            "unknown path falls back to index.html",
			path:            "/admin/dashboard",
			wantStatus:      http.StatusOK,
			wantSubstring:   "<!doctype html>",
			wantContentType: "text/html",
		},
		{
			name:            "deep unknown path falls back to index.html",
			path:            "/docs/search/results",
			wantStatus:      http.StatusOK,
			wantSubstring:   "<!doctype html>",
			wantContentType: "text/html",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, tt.path, http.NoBody)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			if tt.wantSubstring != "" {
				body := rec.Body.String()
				if !strings.Contains(body, tt.wantSubstring) {
					t.Errorf("body does not contain %q; got %q", tt.wantSubstring, body[:min(len(body), 200)])
				}
			}

			if tt.wantContentType != "" {
				ct := rec.Header().Get("Content-Type")
				if !strings.Contains(ct, tt.wantContentType) {
					t.Errorf("Content-Type = %q, want it to contain %q", ct, tt.wantContentType)
				}
			}
		})
	}
}

func TestFaviconHandler(t *testing.T) {
	t.Parallel()

	handler := frontend.FaviconHandler()

	t.Run("returns favicon with 200", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/favicon.ico", http.NoBody)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		ct := rec.Header().Get("Content-Type")
		if !strings.Contains(ct, "image/") {
			t.Errorf("Content-Type = %q, want it to contain %q", ct, "image/")
		}

		if rec.Body.Len() == 0 {
			t.Error("expected non-empty response body")
		}
	})
}
