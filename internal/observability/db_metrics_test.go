package observability

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestDBStatsCollector_Describe_EmitsAllDescriptors(t *testing.T) {
	t.Parallel()

	c := &dbStatsCollector{pool: nil}
	ch := make(chan *prometheus.Desc, 10)
	c.Describe(ch)
	close(ch)

	var descs []*prometheus.Desc
	for d := range ch {
		descs = append(descs, d)
	}

	if len(descs) != 5 {
		t.Fatalf("expected 5 descriptors, got %d", len(descs))
	}

	// Verify each expected descriptor is present.
	wantSubstrings := []string{
		"db_open_connections",
		"db_in_use_connections",
		"db_idle_connections",
		"db_wait_count_total",
		"db_wait_duration_seconds_total",
	}
	for _, want := range wantSubstrings {
		found := false
		for _, d := range descs {
			if s := d.String(); contains(s, want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("descriptor containing %q not found", want)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
