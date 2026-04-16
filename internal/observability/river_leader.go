package observability

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

// leaderProbeTimeout caps each scrape's query budget. River's leader
// keepalive cadence is faster than any reasonable scrape interval, so
// a short timeout is sufficient.
const leaderProbeTimeout = 2 * time.Second

// LeaderQuerier is the subset of pgxpool.Pool needed to run the leader
// query. Declared as an interface so the collector logic is testable
// without a live pool.
type LeaderQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// riverLeaderActive returns 1 when river_leader holds a row whose
// expires_at is in the future, 0 otherwise (including all error cases).
// Query failures degrade to 0 — we prefer a false alarm over a silent
// "leader still active" claim when the database is unreachable.
func riverLeaderActive(ctx context.Context, db LeaderQuerier) float64 {
	var count int
	if err := db.QueryRow(ctx,
		`SELECT COUNT(*) FROM river_leader WHERE expires_at > now()`).Scan(&count); err != nil {
		return 0
	}
	if count == 0 {
		return 0
	}
	return 1
}

// RegisterRiverLeaderGauge exposes documcp_river_leader_active as a
// self-collecting gauge. On every Prometheus scrape it queries
// river_leader for a non-expired row. A value of 0 means no replica
// currently holds the River leadership lease — periodic jobs (OAuth
// token cleanup, orphan file cleanup, soft-delete purge) are not firing
// until a worker replica takes leadership. Pair with a Grafana alert on
// "gauge == 0 for 5m" to catch the "deployed two serve replicas, no
// worker" footgun.
//
// Pass the uninstrumented pool so the scrape path stays trace-free.
func RegisterRiverLeaderGauge(db *pgxpool.Pool) {
	prometheus.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "river",
			Name:      "leader_active",
			Help:      "1 when a non-expired River leader exists (periodic jobs are firing), 0 otherwise.",
		},
		func() float64 {
			ctx, cancel := context.WithTimeout(context.Background(), leaderProbeTimeout)
			defer cancel()
			return riverLeaderActive(ctx, db)
		},
	))
}
