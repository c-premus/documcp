# Prometheus Metrics

## Overview

DocuMCP exposes application metrics in Prometheus format at `GET /metrics`. Metrics are collected using `prometheus/client_golang`. All metrics use the `documcp` namespace.

When `INTERNAL_API_TOKEN` is set, the endpoint requires `Authorization: Bearer <token>`. When not configured, the endpoint is publicly accessible.

## HTTP Metrics

**`documcp_http_requests_total`** (Counter)
Total number of HTTP requests.
Labels: `method`, `route`, `status_code`

**`documcp_http_request_duration_seconds`** (Histogram)
Duration of HTTP requests in seconds.
Labels: `method`, `route`, `status_code`
Buckets: 5ms, 10ms, 25ms, 50ms, 100ms, 250ms, 500ms, 1s, 2.5s, 5s, 10s

**`documcp_http_active_connections`** (Gauge)
Number of active HTTP connections currently being served.

## Search Metrics

**`documcp_search_latency_seconds`** (Histogram)
Latency of search operations in seconds.
Labels: `index`
Buckets: 1ms, 5ms, 10ms, 25ms, 50ms, 100ms, 250ms, 500ms, 1s

**`documcp_search_reconciliation_actions_total`** (Counter)
Total number of search index reconciliation actions.
Labels: `index`, `action`

## Application Metrics

**`documcp_documents`** (Gauge)
Current number of indexed documents.

## Queue Metrics

**`documcp_queue_jobs_dispatched_total`** (Counter)
Total number of jobs dispatched to the queue.
Labels: `queue`, `job_kind`

**`documcp_queue_jobs_completed_total`** (Counter)
Total number of jobs completed successfully.
Labels: `queue`, `job_kind`

**`documcp_queue_jobs_failed_total`** (Counter)
Total number of jobs that failed.
Labels: `queue`, `job_kind`

**`documcp_queue_job_duration_seconds`** (Histogram)
Duration of job execution in seconds.
Labels: `queue`, `job_kind`
Buckets: 100ms, 250ms, 500ms, 1s, 2.5s, 5s, 10s, 30s, 60s, 120s, 300s

## Database Connection Pool Metrics

**`documcp_db_open_connections`** (Gauge)
Number of open connections.

**`documcp_db_in_use_connections`** (Gauge)
Number of connections in use.

**`documcp_db_idle_connections`** (Gauge)
Number of idle connections.

**`documcp_db_wait_count_total`** (Counter)
Total number of connections waited for.

**`documcp_db_wait_duration_seconds_total`** (Counter)
Total time waited for connections.

## Prometheus Configuration

```yaml
scrape_configs:
  - job_name: 'documcp'
    scrape_interval: 15s
    metrics_path: '/metrics'
    # Include when INTERNAL_API_TOKEN is configured
    bearer_token: '<your INTERNAL_API_TOKEN value>'
    static_configs:
      - targets: ['documcp:8080']
```

## PromQL Examples

```promql
# Request rate per minute
rate(documcp_http_requests_total[5m]) * 60

# 95th percentile request latency
histogram_quantile(0.95, rate(documcp_http_request_duration_seconds_bucket[5m]))

# Error rate (5xx)
sum(rate(documcp_http_requests_total{status_code=~"5.."}[5m])) / sum(rate(documcp_http_requests_total[5m])) * 100

# Search latency by index (P95)
histogram_quantile(0.95, sum(rate(documcp_search_latency_seconds_bucket[5m])) by (le, index))

# Job failure rate by kind
sum(rate(documcp_queue_jobs_failed_total[5m])) by (job_kind)

# Active database connections
documcp_db_in_use_connections
```
