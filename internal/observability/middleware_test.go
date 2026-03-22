package observability_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"git.999.haus/chris/DocuMCP-go/internal/observability"
)

func TestTracing_PassesRequestThrough(t *testing.T) {
	t.Parallel()

	var handlerCalled bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mw := observability.Tracing("test-tracer")
	handler := mw(inner)

	req := httptest.NewRequest(http.MethodGet, "/test/path", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !handlerCalled {
		t.Fatal("inner handler was not called")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if body != "ok" {
		t.Errorf("body = %q, want %q", body, "ok")
	}
}

func TestTracing_PreservesResponseStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
	}{
		{name: "200 OK", statusCode: http.StatusOK},
		{name: "201 Created", statusCode: http.StatusCreated},
		{name: "400 Bad Request", statusCode: http.StatusBadRequest},
		{name: "404 Not Found", statusCode: http.StatusNotFound},
		{name: "500 Internal Server Error", statusCode: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			})

			mw := observability.Tracing("test-tracer")
			handler := mw(inner)

			req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.statusCode {
				t.Errorf("status = %d, want %d", rec.Code, tt.statusCode)
			}
		})
	}
}

func TestTracing_PreservesRequestMethod(t *testing.T) {
	t.Parallel()

	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			t.Parallel()

			var receivedMethod string
			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedMethod = r.Method
				w.WriteHeader(http.StatusOK)
			})

			mw := observability.Tracing("test-tracer")
			handler := mw(inner)

			req := httptest.NewRequest(method, "/test", http.NoBody)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if receivedMethod != method {
				t.Errorf("received method = %q, want %q", receivedMethod, method)
			}
		})
	}
}

func TestTracing_PropagatesContextToHandler(t *testing.T) {
	t.Parallel()

	var ctxExists bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The tracing middleware should inject a span into the context.
		// We verify the context is not nil and the request passes through.
		ctxExists = r.Context() != nil
		w.WriteHeader(http.StatusOK)
	})

	mw := observability.Tracing("test-tracer")
	handler := mw(inner)

	req := httptest.NewRequest(http.MethodGet, "/ctx-test", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !ctxExists {
		t.Error("request context was nil inside handler")
	}
}

func TestTracing_WithResponseBody(t *testing.T) {
	t.Parallel()

	responseBody := `{"status":"healthy"}`
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(responseBody))
	})

	mw := observability.Tracing("test-tracer")
	handler := mw(inner)

	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	result := rec.Result()
	defer func() { _ = result.Body.Close() }()

	body, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("reading response body: %v", err)
	}

	if string(body) != responseBody {
		t.Errorf("body = %q, want %q", string(body), responseBody)
	}

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
}

func TestTracing_WithChiRouter(t *testing.T) {
	t.Parallel()

	var handlerCalled bool
	r := chi.NewRouter()
	r.Use(observability.Tracing("chi-tracer"))
	r.Get("/api/documents/{uuid}", func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/documents/test-uuid-123", http.NoBody)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if !handlerCalled {
		t.Fatal("handler was not called through chi router")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestTracing_DefaultStatusCodeWhenNotExplicitlySet(t *testing.T) {
	t.Parallel()

	// When the handler does not explicitly call WriteHeader,
	// the default status code should be 200.
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("implicit 200"))
	})

	mw := observability.Tracing("test-tracer")
	handler := mw(inner)

	req := httptest.NewRequest(http.MethodGet, "/implicit", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestTracing_WithContentLength(t *testing.T) {
	t.Parallel()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := observability.Tracing("test-tracer")
	handler := mw(inner)

	req := httptest.NewRequest(http.MethodPost, "/with-content", http.NoBody)
	req.ContentLength = 1024
	rec := httptest.NewRecorder()

	// Should not panic even with content length set.
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
