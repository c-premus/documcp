package observability_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"git.999.haus/chris/DocuMCP-go/internal/observability"
)

// newTestMetrics creates Metrics registered against a fresh registry so tests
// do not interfere with each other.
func newTestMetrics(t *testing.T) *observability.Metrics {
	t.Helper()

	reg := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = reg
	prometheus.DefaultGatherer = reg

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = prometheus.NewRegistry()
		prometheus.DefaultGatherer = prometheus.NewRegistry()
	})

	return observability.NewMetrics()
}

func TestMetricsMiddleware_RecordsRequestMetrics(t *testing.T) {
	m := newTestMetrics(t)

	r := chi.NewRouter()
	r.Use(observability.MetricsMiddleware(m))
	r.Get("/api/documents/{uuid}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/documents/abc-123", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Verify request counter incremented.
	var metric dto.Metric
	if err := m.HTTPRequestsTotal.WithLabelValues("GET", "/api/documents/{uuid}", "200").Write(&metric); err != nil {
		t.Fatalf("writing metric: %v", err)
	}

	if got := metric.GetCounter().GetValue(); got != 1 {
		t.Errorf("HTTPRequestsTotal = %v, want 1", got)
	}

	// Verify duration was observed (count should be 1).
	var histMetric dto.Metric
	observer := m.HTTPRequestDuration.WithLabelValues("GET", "/api/documents/{uuid}", "200")
	if err := observer.(prometheus.Metric).Write(&histMetric); err != nil {
		t.Fatalf("writing histogram metric: %v", err)
	}

	if got := histMetric.GetHistogram().GetSampleCount(); got != 1 {
		t.Errorf("HTTPRequestDuration sample count = %d, want 1", got)
	}
}

func TestMetricsMiddleware_ActiveConnectionsReturnToZero(t *testing.T) {
	m := newTestMetrics(t)

	r := chi.NewRouter()
	r.Use(observability.MetricsMiddleware(m))
	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	// After the request completes, active connections should be back to zero.
	var metric dto.Metric
	if err := m.HTTPActiveConnections.Write(&metric); err != nil {
		t.Fatalf("writing metric: %v", err)
	}

	if got := metric.GetGauge().GetValue(); got != 0 {
		t.Errorf("HTTPActiveConnections = %v, want 0", got)
	}
}

func TestMetricsMiddleware_UnmatchedRoute(t *testing.T) {
	m := newTestMetrics(t)

	r := chi.NewRouter()
	r.Use(observability.MetricsMiddleware(m))
	r.Get("/known", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Request a path that does not match any route.
	req := httptest.NewRequest(http.MethodGet, "/unknown-path", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	// chi returns 404 for unmatched routes; the middleware should use
	// "unmatched" for the route label.
	var metric dto.Metric
	if err := m.HTTPRequestsTotal.WithLabelValues("GET", "unmatched", "404").Write(&metric); err != nil {
		t.Fatalf("writing metric: %v", err)
	}

	if got := metric.GetCounter().GetValue(); got != 1 {
		t.Errorf("HTTPRequestsTotal for unmatched = %v, want 1", got)
	}
}
