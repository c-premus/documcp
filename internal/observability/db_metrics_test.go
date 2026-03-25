package observability_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/c-premus/documcp/internal/observability"
)

// stubConnector implements driver.Connector to create a *sql.DB without a real
// database. The resulting DB cannot execute queries but exposes valid DBStats.
type stubConnector struct{}

func (stubConnector) Connect(context.Context) (driver.Conn, error) {
	return nil, sql.ErrConnDone
}

func (stubConnector) Driver() driver.Driver {
	return nil
}

// newStubDB returns a *sql.DB backed by a no-op connector. This is sufficient
// for reading pool stats without hitting a real database.
func newStubDB() *sql.DB {
	return sql.OpenDB(stubConnector{})
}

// withFreshRegistry swaps the default prometheus registerer/gatherer to a fresh
// registry for the duration of the test. Tests using this helper must NOT use
// t.Parallel() because they mutate package-level globals.
func withFreshRegistry(t *testing.T) *prometheus.Registry {
	t.Helper()

	reg := prometheus.NewRegistry()
	origRegisterer := prometheus.DefaultRegisterer
	origGatherer := prometheus.DefaultGatherer
	prometheus.DefaultRegisterer = reg
	prometheus.DefaultGatherer = reg

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = origRegisterer
		prometheus.DefaultGatherer = origGatherer
	})

	return reg
}

func TestRegisterDBMetrics_DoesNotPanic(t *testing.T) {
	// Not parallel: mutates prometheus.DefaultRegisterer.
	reg := withFreshRegistry(t)

	db := newStubDB()
	defer func() { _ = db.Close() }()

	// RegisterDBMetrics uses MustRegister, so any panic here will fail the test.
	observability.RegisterDBMetrics(db)

	// Verify registration actually happened by gathering.
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gathering metrics: %v", err)
	}

	if len(families) == 0 {
		t.Error("expected at least one metric family after registration")
	}
}

func TestDBStatsCollector_Describe(t *testing.T) {
	// Not parallel: mutates prometheus.DefaultRegisterer.
	reg := withFreshRegistry(t)

	db := newStubDB()
	defer func() { _ = db.Close() }()

	observability.RegisterDBMetrics(db)

	// Gather triggers Describe and Collect on all registered collectors.
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gathering metrics: %v", err)
	}

	wantDescs := map[string]bool{
		"documcp_db_open_connections":            false,
		"documcp_db_in_use_connections":          false,
		"documcp_db_idle_connections":            false,
		"documcp_db_wait_count_total":            false,
		"documcp_db_wait_duration_seconds_total": false,
	}

	for _, f := range families {
		if _, ok := wantDescs[f.GetName()]; ok {
			wantDescs[f.GetName()] = true
		}
	}

	for name, found := range wantDescs {
		if !found {
			t.Errorf("metric %q not found in gathered families", name)
		}
	}
}

func TestDBStatsCollector_Collect(t *testing.T) {
	// Not parallel: mutates prometheus.DefaultRegisterer.
	reg := withFreshRegistry(t)

	db := newStubDB()
	defer func() { _ = db.Close() }()

	observability.RegisterDBMetrics(db)

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gathering metrics: %v", err)
	}

	// Build a lookup from metric name to value for gauge/counter metrics.
	metricValues := make(map[string]float64)
	for _, f := range families {
		name := f.GetName()
		if len(f.GetMetric()) > 0 {
			m := f.GetMetric()[0]
			switch {
			case m.GetGauge() != nil:
				metricValues[name] = m.GetGauge().GetValue()
			case m.GetCounter() != nil:
				metricValues[name] = m.GetCounter().GetValue()
			}
		}
	}

	// A freshly-opened stub DB should have zero stats across the board.
	tests := []struct {
		name string
		want float64
	}{
		{name: "documcp_db_open_connections", want: 0},
		{name: "documcp_db_in_use_connections", want: 0},
		{name: "documcp_db_idle_connections", want: 0},
		{name: "documcp_db_wait_count_total", want: 0},
		{name: "documcp_db_wait_duration_seconds_total", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Subtests read from pre-computed map; safe to parallelize.
			t.Parallel()

			got, ok := metricValues[tt.name]
			if !ok {
				t.Fatalf("metric %q not found", tt.name)
			}
			if got != tt.want {
				t.Errorf("%s = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestDBStatsCollector_MetricTypes(t *testing.T) {
	// Not parallel: mutates prometheus.DefaultRegisterer.
	reg := withFreshRegistry(t)

	db := newStubDB()
	defer func() { _ = db.Close() }()

	observability.RegisterDBMetrics(db)

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gathering metrics: %v", err)
	}

	wantTypes := map[string]dto.MetricType{
		"documcp_db_open_connections":            dto.MetricType_GAUGE,
		"documcp_db_in_use_connections":          dto.MetricType_GAUGE,
		"documcp_db_idle_connections":            dto.MetricType_GAUGE,
		"documcp_db_wait_count_total":            dto.MetricType_COUNTER,
		"documcp_db_wait_duration_seconds_total": dto.MetricType_COUNTER,
	}

	for _, f := range families {
		name := f.GetName()
		if wantType, ok := wantTypes[name]; ok {
			if got := f.GetType(); got != wantType {
				t.Errorf("metric %q type = %v, want %v", name, got, wantType)
			}
			delete(wantTypes, name)
		}
	}

	for name := range wantTypes {
		t.Errorf("metric %q not found in gathered families", name)
	}
}
