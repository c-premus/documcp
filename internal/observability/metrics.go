// Package observability provides Prometheus metrics and instrumentation
// middleware for the DocuMCP application.
package observability

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
)

const namespace = "documcp"

// Metrics holds all Prometheus metric collectors for the application.
type Metrics struct {
	HTTPRequestDuration   *prometheus.HistogramVec
	HTTPRequestsTotal     *prometheus.CounterVec
	HTTPActiveConnections prometheus.Gauge
	SearchLatency         *prometheus.HistogramVec

	// Queue metrics
	QueueJobsDispatched *prometheus.CounterVec
	QueueJobsCompleted  *prometheus.CounterVec
	QueueJobsFailed     *prometheus.CounterVec
	QueueJobDuration    *prometheus.HistogramVec
}

// NewMetrics creates and registers all Prometheus metrics with the default
// registerer. It panics if registration fails, which is appropriate at
// application startup.
func NewMetrics() *Metrics {
	m := &Metrics{
		HTTPRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: "http",
				Name:      "request_duration_seconds",
				Help:      "Duration of HTTP requests in seconds.",
				Buckets:   []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
			},
			[]string{"method", "route", "status_code"},
		),
		HTTPRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: "http",
				Name:      "requests_total",
				Help:      "Total number of HTTP requests.",
			},
			[]string{"method", "route", "status_code"},
		),
		HTTPActiveConnections: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "http",
				Name:      "active_connections",
				Help:      "Number of active HTTP connections currently being served.",
			},
		),
		SearchLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: "search",
				Name:      "latency_seconds",
				Help:      "Latency of search operations in seconds.",
				Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
			},
			[]string{"index"},
		),
		QueueJobsDispatched: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: "queue",
				Name:      "jobs_dispatched_total",
				Help:      "Total number of jobs dispatched to the queue.",
			},
			[]string{"queue", "job_kind"},
		),
		QueueJobsCompleted: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: "queue",
				Name:      "jobs_completed_total",
				Help:      "Total number of jobs completed successfully.",
			},
			[]string{"queue", "job_kind"},
		),
		QueueJobsFailed: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: "queue",
				Name:      "jobs_failed_total",
				Help:      "Total number of jobs that failed.",
			},
			[]string{"queue", "job_kind"},
		),
		QueueJobDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: "queue",
				Name:      "job_duration_seconds",
				Help:      "Duration of job execution in seconds.",
				Buckets:   []float64{.1, .25, .5, 1, 2.5, 5, 10, 30, 60, 120, 300},
			},
			[]string{"queue", "job_kind"},
		),
	}

	prometheus.MustRegister(
		m.HTTPRequestDuration,
		m.HTTPRequestsTotal,
		m.HTTPActiveConnections,
		m.SearchLatency,
		m.QueueJobsDispatched,
		m.QueueJobsCompleted,
		m.QueueJobsFailed,
		m.QueueJobDuration,
	)

	return m
}

// RegisterDocumentCount registers a gauge that queries the database for the
// current non-deleted document count on each Prometheus scrape.
func RegisterDocumentCount(pool *pgxpool.Pool) {
	prometheus.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "documents",
			Help:      "Current number of indexed documents.",
		},
		func() float64 {
			var count int
			err := pool.QueryRow(context.Background(),
				`SELECT COUNT(*) FROM documents WHERE deleted_at IS NULL`).Scan(&count)
			if err != nil {
				return 0
			}
			return float64(count)
		},
	))
}

// RegisterMCPSessionGauge registers a gauge that reports the number of
// active MCP sessions held by this replica. Sampled on each Prometheus
// scrape by calling source().
//
// Because sessions live in-memory per replica, operators can divide
// `max(documcp_mcp_active_sessions)` by `min(documcp_mcp_active_sessions)`
// to spot sticky-session hot-spotting across replicas behind a load
// balancer with cookie-based affinity.
func RegisterMCPSessionGauge(source func() int) {
	prometheus.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "mcp_active_sessions",
			Help:      "Number of MCP sessions currently held by this replica.",
		},
		func() float64 { return float64(source()) },
	))
}

// RegisterDBMetrics registers Prometheus gauges for pgxpool connection stats.
// The collector reads pool stats on each Prometheus scrape.
func RegisterDBMetrics(pool *pgxpool.Pool) {
	prometheus.MustRegister(&dbStatsCollector{pool: pool})
}

// dbStatsCollector implements prometheus.Collector for pgxpool stats.
type dbStatsCollector struct {
	pool *pgxpool.Pool
}

var (
	dbOpenDesc      = prometheus.NewDesc(namespace+"_db_open_connections", "Number of open connections.", nil, nil)
	dbInUseDesc     = prometheus.NewDesc(namespace+"_db_in_use_connections", "Number of connections in use.", nil, nil)
	dbIdleDesc      = prometheus.NewDesc(namespace+"_db_idle_connections", "Number of idle connections.", nil, nil)
	dbWaitCountDesc = prometheus.NewDesc(namespace+"_db_wait_count_total", "Total number of connections waited for.", nil, nil)
	dbWaitDurDesc   = prometheus.NewDesc(namespace+"_db_wait_duration_seconds_total", "Total time waited for connections.", nil, nil)
)

// Describe sends the metric descriptors to the provided channel.
func (c *dbStatsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- dbOpenDesc
	ch <- dbInUseDesc
	ch <- dbIdleDesc
	ch <- dbWaitCountDesc
	ch <- dbWaitDurDesc
}

// Collect gathers and sends current database connection pool metrics.
func (c *dbStatsCollector) Collect(ch chan<- prometheus.Metric) {
	stats := c.pool.Stat()
	ch <- prometheus.MustNewConstMetric(dbOpenDesc, prometheus.GaugeValue, float64(stats.TotalConns()))
	ch <- prometheus.MustNewConstMetric(dbInUseDesc, prometheus.GaugeValue, float64(stats.AcquiredConns()))
	ch <- prometheus.MustNewConstMetric(dbIdleDesc, prometheus.GaugeValue, float64(stats.IdleConns()))
	ch <- prometheus.MustNewConstMetric(dbWaitCountDesc, prometheus.CounterValue, float64(stats.EmptyAcquireCount()))
	ch <- prometheus.MustNewConstMetric(dbWaitDurDesc, prometheus.CounterValue, stats.AcquireDuration().Seconds())
}

// RegisterRedisMetrics registers Prometheus gauges for Redis connection pool stats.
// The collector reads pool stats on each Prometheus scrape.
func RegisterRedisMetrics(client *redis.Client) {
	prometheus.MustRegister(&redisStatsCollector{client: client})
}

// redisStatsCollector implements prometheus.Collector for Redis pool stats.
type redisStatsCollector struct {
	client *redis.Client
}

var (
	redisHitsDesc     = prometheus.NewDesc(namespace+"_redis_pool_hits_total", "Total number of times a connection was found in the pool.", nil, nil)
	redisMissesDesc   = prometheus.NewDesc(namespace+"_redis_pool_misses_total", "Total number of times a connection was not found in the pool.", nil, nil)
	redisTimeoutsDesc = prometheus.NewDesc(namespace+"_redis_pool_timeouts_total", "Total number of times a wait for a connection timed out.", nil, nil)
	redisActiveDesc   = prometheus.NewDesc(namespace+"_redis_active_connections", "Number of active connections in the pool.", nil, nil)
	redisIdleDesc     = prometheus.NewDesc(namespace+"_redis_idle_connections", "Number of idle connections in the pool.", nil, nil)
)

// Describe sends the metric descriptors to the provided channel.
func (c *redisStatsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- redisHitsDesc
	ch <- redisMissesDesc
	ch <- redisTimeoutsDesc
	ch <- redisActiveDesc
	ch <- redisIdleDesc
}

// Collect gathers and sends current Redis connection pool metrics.
func (c *redisStatsCollector) Collect(ch chan<- prometheus.Metric) {
	stats := c.client.PoolStats()
	ch <- prometheus.MustNewConstMetric(redisHitsDesc, prometheus.CounterValue, float64(stats.Hits))
	ch <- prometheus.MustNewConstMetric(redisMissesDesc, prometheus.CounterValue, float64(stats.Misses))
	ch <- prometheus.MustNewConstMetric(redisTimeoutsDesc, prometheus.CounterValue, float64(stats.Timeouts))
	ch <- prometheus.MustNewConstMetric(redisActiveDesc, prometheus.GaugeValue, float64(stats.TotalConns-stats.IdleConns))
	ch <- prometheus.MustNewConstMetric(redisIdleDesc, prometheus.GaugeValue, float64(stats.IdleConns))
}

// MetricsHandler returns an http.Handler that serves Prometheus metrics
// in the standard exposition format.
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}
