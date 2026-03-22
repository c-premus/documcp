package observability_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"git.999.haus/chris/DocuMCP-go/internal/observability"
)

func TestNewMetrics(t *testing.T) {
	// Use a custom registry to avoid polluting the default and to allow
	// repeated test runs.
	reg := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = reg
	prometheus.DefaultGatherer = reg

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = prometheus.NewRegistry()
		prometheus.DefaultGatherer = prometheus.NewRegistry()
	})

	m := observability.NewMetrics()

	if m.HTTPRequestDuration == nil {
		t.Fatal("HTTPRequestDuration is nil")
	}
	if m.HTTPRequestsTotal == nil {
		t.Fatal("HTTPRequestsTotal is nil")
	}
	if m.HTTPActiveConnections == nil {
		t.Fatal("HTTPActiveConnections is nil")
	}
	if m.DocumentCount == nil {
		t.Fatal("DocumentCount is nil")
	}
	if m.SearchLatency == nil {
		t.Fatal("SearchLatency is nil")
	}

	// Verify metrics are gathered without error.
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gathering metrics: %v", err)
	}

	wantNames := map[string]bool{
		"documcp_http_active_connections": false,
		"documcp_documents":               false,
	}

	for _, f := range families {
		if _, ok := wantNames[f.GetName()]; ok {
			wantNames[f.GetName()] = true
		}
	}

	for name, found := range wantNames {
		if !found {
			t.Errorf("metric %q not found in gathered families", name)
		}
	}
}

func TestMetricsHandler(t *testing.T) {
	h := observability.MetricsHandler()
	if h == nil {
		t.Fatal("MetricsHandler returned nil")
	}

	req := httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") && !strings.Contains(ct, "application/openmetrics-text") {
		t.Errorf("Content-Type = %q, want text/plain or openmetrics", ct)
	}
}
