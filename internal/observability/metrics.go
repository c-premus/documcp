// Package observability provides Prometheus metrics and instrumentation
// middleware for the DocuMCP application.
package observability

import (
	"database/sql"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const namespace = "documcp"

// Metrics holds all Prometheus metric collectors for the application.
type Metrics struct {
	HTTPRequestDuration   *prometheus.HistogramVec
	HTTPRequestsTotal     *prometheus.CounterVec
	HTTPActiveConnections prometheus.Gauge
	DocumentCount         prometheus.Gauge
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
		DocumentCount: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "documents",
				Help:      "Current number of indexed documents.",
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
		m.DocumentCount,
		m.SearchLatency,
		m.QueueJobsDispatched,
		m.QueueJobsCompleted,
		m.QueueJobsFailed,
		m.QueueJobDuration,
	)

	return m
}

// RegisterDBMetrics registers Prometheus gauges for database/sql.DBStats.
// The collector reads pool stats on each Prometheus scrape.
func RegisterDBMetrics(db *sql.DB) {
	prometheus.MustRegister(&dbStatsCollector{db: db})
}

// dbStatsCollector implements prometheus.Collector for database/sql pool stats.
type dbStatsCollector struct {
	db *sql.DB
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
	stats := c.db.Stats()
	ch <- prometheus.MustNewConstMetric(dbOpenDesc, prometheus.GaugeValue, float64(stats.OpenConnections))
	ch <- prometheus.MustNewConstMetric(dbInUseDesc, prometheus.GaugeValue, float64(stats.InUse))
	ch <- prometheus.MustNewConstMetric(dbIdleDesc, prometheus.GaugeValue, float64(stats.Idle))
	ch <- prometheus.MustNewConstMetric(dbWaitCountDesc, prometheus.CounterValue, float64(stats.WaitCount))
	ch <- prometheus.MustNewConstMetric(dbWaitDurDesc, prometheus.CounterValue, stats.WaitDuration.Seconds())
}

// MetricsHandler returns an http.Handler that serves Prometheus metrics
// in the standard exposition format.
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}
