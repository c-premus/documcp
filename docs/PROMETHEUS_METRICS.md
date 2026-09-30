# Prometheus Metrics

## Overview

DocuMCP exposes 22 application metrics in Prometheus format at `GET /metrics`. Metrics are collected using `prometheus/client_golang` and registered on the default registry. All application metrics use the `documcp` namespace.

Metrics are always registered and the endpoint is always served. There is no setting to turn them off.

Because the handler is `promhttp.Handler()` on the default registry, the endpoint also exposes the client library's standard Go runtime (`go_*`) and process (`process_*`) metrics. These are not counted in the 22 and are not documented here.

### Where the endpoint is served

| Mode | Address |
|------|---------|
| `documcp serve` (with or without `--with-worker`) | Main HTTP port (`SERVER_PORT`, default `8080`) |
| `documcp worker` | Health server on `WORKER_HEALTH_PORT` (default `9090`) |

Both modes register the same 22 metrics. In worker-only mode, the HTTP metrics never receive observations because the health server has no metrics middleware.

### Authentication

When `INTERNAL_API_TOKEN` is set, the endpoint requires `Authorization: Bearer <token>`. This applies in both serve and worker mode. When the token is not set, the endpoint is publicly accessible and DocuMCP logs a warning at startup. `INTERNAL_API_TOKEN` is required when `APP_ENV=production`.

## HTTP Metrics

Recorded by the metrics middleware in serve mode. Requests to `/health*` and `/metrics` are counted too.

**`documcp_http_requests_total`** (Counter)
Total number of HTTP requests.
Labels: `method`, `route`, `status_code`

**`documcp_http_request_duration_seconds`** (Histogram)
Duration of HTTP requests in seconds.
Labels: `method`, `route`, `status_code`
Buckets: 5ms, 10ms, 25ms, 50ms, 100ms, 250ms, 500ms, 1s, 2.5s, 5s, 10s

The `route` label is chi's matched route pattern (for example `/api/documents/{uuid}`), or `unmatched` when no pattern matched.

**`documcp_http_active_connections`** (Gauge)
Number of HTTP requests currently being served. The gauge is incremented per request, not per TCP connection, so it measures in-flight requests. Long-lived SSE streams count for as long as they stay open.

## Search Metrics

**`documcp_search_latency_seconds`** (Histogram)
Latency of search operations in seconds. Only successful searches are observed.
Labels: `index` (`documents`, `zim_archives`, `git_templates`, or `federated` for the cross-source search used by `unified_search` and the REST API; the federated value covers the database query only, not the Kiwix fan-out)
Buckets: 1ms, 5ms, 10ms, 25ms, 50ms, 100ms, 250ms, 500ms, 1s

## Application Metrics

These gauges are computed on every Prometheus scrape.

**`documcp_documents`** (Gauge)
Number of documents that are not soft-deleted, in any processing status. Runs `SELECT COUNT(*) FROM documents WHERE deleted_at IS NULL` on the main (traced) pool. Reports `0` if the query fails.

**`documcp_ready`** (Gauge)
`1` when both PostgreSQL and Redis respond to Ping within 2 seconds, `0` otherwise. Uses the uninstrumented pool and Redis client, so scrapes produce no spans. This is the same check as the `/health/ready` endpoint.

**`documcp_river_leader_active`** (Gauge)
`1` when the `river_leader` table holds a non-expired row, `0` otherwise (including when the query fails). A value of `0` means no process holds River leadership, so periodic jobs (OAuth token cleanup, orphaned file cleanup, soft-delete purge, and others) are not running. The usual cause is a deployment with no `worker` process and no `serve --with-worker`.

## Security Metrics

**`documcp_oauth_token_replay_total`** (Counter)
Number of detected OAuth token replays. A non-zero rate suggests a stolen authorization code or refresh token. On detection, DocuMCP revokes every token descended from the same authorization code.
Labels: `type`

| `type` value | Meaning |
|--------------|---------|
| `authcode` | An already-used (revoked) authorization code was presented again |
| `refresh` | A revoked refresh token was presented again, for example after rotation |

## Queue Metrics

**`documcp_queue_jobs_dispatched_total`** (Counter)
Total number of jobs dispatched to the queue through DocuMCP's River client wrapper. Periodic jobs that River schedules on its own are not counted.
Labels: `queue`, `job_kind`

**`documcp_queue_jobs_completed_total`** (Counter)
Total number of jobs completed successfully.
Labels: `queue`, `job_kind`

**`documcp_queue_jobs_failed_total`** (Counter)
Total number of failed job attempts. Each failed attempt of a retried job is counted. Jobs that panic are logged but not counted here.
Labels: `queue`, `job_kind`

**`documcp_queue_job_duration_seconds`** (Histogram)
Duration of job execution in seconds. Only successful runs are observed.
Labels: `queue`, `job_kind`
Buckets: 100ms, 250ms, 500ms, 1s, 2.5s, 5s, 10s, 30s, 60s, 120s, 300s

## Database Connection Pool Metrics

These metrics are collected from `pgxpool.Stat()` on each Prometheus scrape. They describe the main (instrumented) pool only, not the small uninstrumented pool used for readiness and leader checks.

**`documcp_db_open_connections`** (Gauge)
Number of open connections (`TotalConns`).

**`documcp_db_in_use_connections`** (Gauge)
Number of connections in use (`AcquiredConns`).

**`documcp_db_idle_connections`** (Gauge)
Number of idle connections (`IdleConns`).

**`documcp_db_wait_count_total`** (Counter)
Number of acquires that had to wait because the pool had no idle connection (pgx `EmptyAcquireCount`). This includes waits for a new connection to be opened, not only waits caused by a full pool.

**`documcp_db_wait_duration_seconds_total`** (Counter)
Total time spent in successful acquires, in seconds (pgx `AcquireDuration`). Despite the name, this includes fast acquires that did not wait.

## Redis Connection Pool Metrics

These metrics are collected from `redis.PoolStats()` on each Prometheus scrape. They describe the main (instrumented) Redis client only. The separate uninstrumented client used for rate limiting and readiness checks is not included.

**`documcp_redis_pool_hits_total`** (Counter)
Total number of times a connection was found in the pool.

**`documcp_redis_pool_misses_total`** (Counter)
Total number of times a connection was not found in the pool.

**`documcp_redis_pool_timeouts_total`** (Counter)
Total number of times a wait for a connection timed out.

**`documcp_redis_active_connections`** (Gauge)
Number of connections in use, computed as `TotalConns - IdleConns`.

**`documcp_redis_idle_connections`** (Gauge)
Number of idle connections in the pool.

## Prometheus Configuration

```yaml
scrape_configs:
  - job_name: 'documcp'
    scrape_interval: 15s
    metrics_path: '/metrics'
    # Include when INTERNAL_API_TOKEN is configured
    bearer_token: '<your INTERNAL_API_TOKEN value>'
    static_configs:
      # "app" is the service name in the bundled docker-compose.yml.
      # For worker-only processes, scrape WORKER_HEALTH_PORT (default 9090) instead.
      - targets: ['app:8080']
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

# Redis pool hit rate
rate(documcp_redis_pool_hits_total[5m]) / (rate(documcp_redis_pool_hits_total[5m]) + rate(documcp_redis_pool_misses_total[5m])) * 100

# Redis pool timeouts (should be 0)
rate(documcp_redis_pool_timeouts_total[5m])

# Not ready (DB or Redis ping failing)
documcp_ready == 0

# No River leader (periodic jobs not running)
documcp_river_leader_active == 0

# OAuth token replays by type
sum(rate(documcp_oauth_token_replay_total[5m])) by (type)
```
