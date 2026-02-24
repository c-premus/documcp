# Prometheus Metrics

DocuMCP v1.4.0

## Overview

DocuMCP exposes application metrics in Prometheus format for monitoring and observability. Metrics are collected using the `promphp/prometheus_client_php` library with Redis storage backend.

## Metrics Endpoint

**Endpoint**: `GET /metrics`
**Authentication**: Bearer token via `INTERNAL_API_TOKEN` env var (optional)
**Content-Type**: `text/plain; version=0.0.4`

When `INTERNAL_API_TOKEN` is set, requests must include `Authorization: Bearer <token>`.
When not configured, the endpoint remains publicly accessible (backward compatible).

## Available Metrics

### Document Upload Metrics

**Metric**: `documcp_documents_uploaded_total`
**Type**: Counter
**Description**: Total number of documents uploaded
**Labels**:
- `type`: Document file type (pdf, docx, xlsx, html, md)

**Example**:
```
# HELP documcp_documents_uploaded_total Total number of documents uploaded
# TYPE documcp_documents_uploaded_total counter
documcp_documents_uploaded_total{type="pdf"} 142
documcp_documents_uploaded_total{type="docx"} 87
documcp_documents_uploaded_total{type="xlsx"} 23
```

### Document Search Metrics

**Metric**: `documcp_document_search_duration_seconds`
**Type**: Histogram
**Description**: Document search duration in seconds
**Buckets**: 0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1.0, 2.5, 5.0, 7.5, 10.0

**Example**:
```
# HELP documcp_document_search_duration_seconds Document search duration in seconds
# TYPE documcp_document_search_duration_seconds histogram
documcp_document_search_duration_seconds_bucket{le="0.005"} 0
documcp_document_search_duration_seconds_bucket{le="0.01"} 0
documcp_document_search_duration_seconds_bucket{le="0.025"} 15
documcp_document_search_duration_seconds_bucket{le="0.05"} 142
documcp_document_search_duration_seconds_bucket{le="0.1"} 387
documcp_document_search_duration_seconds_bucket{le="+Inf"} 421
documcp_document_search_duration_seconds_sum 21.453
documcp_document_search_duration_seconds_count 421
```

**Metric**: `documcp_document_search_results`
**Type**: Histogram
**Description**: Number of search results returned
**Buckets**: Same as duration (0.005 - 10.0)

**Example**:
```
# HELP documcp_document_search_results Number of search results returned
# TYPE documcp_document_search_results histogram
documcp_document_search_results_bucket{le="0.005"} 0
documcp_document_search_results_bucket{le="0.01"} 0
documcp_document_search_results_bucket{le="0.025"} 12
documcp_document_search_results_bucket{le="0.05"} 45
documcp_document_search_results_bucket{le="0.1"} 98
documcp_document_search_results_bucket{le="+Inf"} 421
documcp_document_search_results_sum 5832
documcp_document_search_results_count 421
```

### Document Deletion Metrics

**Metric**: `documcp_documents_deleted_total`
**Type**: Counter
**Description**: Total number of documents deleted
**Labels**:
- `type`: Document file type (pdf, docx, xlsx, html, md)

**Example**:
```
# HELP documcp_documents_deleted_total Total number of documents deleted
# TYPE documcp_documents_deleted_total counter
documcp_documents_deleted_total{type="pdf"} 12
documcp_documents_deleted_total{type="docx"} 8
```

### Authentication Metrics

**Metric**: `documcp_auth_failures_total`
**Type**: Counter
**Description**: Total number of authentication failures
**Labels**:
- `reason`: Failure reason (invalid_token, expired, revoked, invalid_client, etc.)
- `client`: Client identifier (client_id or 'unknown')

**Example**:
```
# HELP documcp_auth_failures_total Total number of authentication failures
# TYPE documcp_auth_failures_total counter
documcp_auth_failures_total{reason="invalid_token",client="unknown"} 5
documcp_auth_failures_total{reason="expired",client="my-app"} 12
documcp_auth_failures_total{reason="revoked",client="cli-tool"} 3
```

### HTTP Request Metrics

**Metric**: `documcp_http_requests_total`
**Type**: Counter
**Description**: Total HTTP requests
**Labels**:
- `method`: HTTP method (GET, POST, PUT, DELETE)
- `path`: Request path
- `status`: HTTP status code (200, 404, 500, etc.)

**Example**:
```
# HELP documcp_http_requests_total Total HTTP requests
# TYPE documcp_http_requests_total counter
documcp_http_requests_total{method="GET",path="/api/documents",status="200"} 1523
documcp_http_requests_total{method="POST",path="/api/documents",status="201"} 142
documcp_http_requests_total{method="DELETE",path="/api/documents/{id}",status="200"} 12
```

**Metric**: `documcp_http_request_duration_seconds`
**Type**: Histogram
**Description**: HTTP request duration in seconds
**Labels**:
- `method`: HTTP method
- `path`: Request path
**Buckets**: 0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1.0, 2.5, 5.0, 7.5, 10.0

**Example**:
```
# HELP documcp_http_request_duration_seconds HTTP request duration in seconds
# TYPE documcp_http_request_duration_seconds histogram
documcp_http_request_duration_seconds_bucket{method="GET",path="/api/documents",le="0.005"} 0
documcp_http_request_duration_seconds_bucket{method="GET",path="/api/documents",le="0.01"} 12
documcp_http_request_duration_seconds_bucket{method="GET",path="/api/documents",le="0.025"} 421
documcp_http_request_duration_seconds_bucket{method="GET",path="/api/documents",le="+Inf"} 1523
documcp_http_request_duration_seconds_sum{method="GET",path="/api/documents"} 15.234
documcp_http_request_duration_seconds_count{method="GET",path="/api/documents"} 1523
```

## Configuration

### Redis Storage

Metrics are stored in Redis for persistence and multi-process support:

```php
// app/Services/Metrics/PrometheusService.php
Redis::setDefaultOptions([
    'host' => config('database.redis.default.host', '127.0.0.1'),
    'port' => config('database.redis.default.port', 6379),
    'password' => config('database.redis.default.password'),
    'database' => config('database.redis.default.database', 0),
]);
```

### Namespace

All metrics are prefixed with the application name from `config('app.name')` (default: `documcp`).

## Prometheus Configuration

Add DocuMCP to your Prometheus scrape configuration:

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'documcp'
    scrape_interval: 15s
    scrape_timeout: 10s
    metrics_path: '/metrics'
    # Include bearer_token when INTERNAL_API_TOKEN is configured
    bearer_token: '<your INTERNAL_API_TOKEN value>'
    static_configs:
      - targets: ['documcp.your-domain.com']
```

### Scraping via Docker Network

If Prometheus scrapes directly from the Docker network:

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'documcp'
    scrape_interval: 15s
    scrape_timeout: 10s
    metrics_path: '/metrics'
    bearer_token: '<your INTERNAL_API_TOKEN value>'
    static_configs:
      - targets: ['documcp:8000']  # Internal Docker service name + Octane port
```

## Grafana Dashboards

The Grafana dashboard is defined as TypeScript source in `grafana/` using the **Grafana Foundation SDK**.

**Generate from source**:
```bash
cd grafana && npm install && npm run generate
```
This outputs `dist/documcp.json`.

**Import via Grafana UI**:
1. Navigate to **Dashboards** → **Import**
2. Upload `grafana/dist/documcp.json`
3. Select your Prometheus, Tempo, and Loki data sources
4. Click **Import**

**Dashboard Sections** (UID: `documcp-observability-v2`):
- RED metrics (request rate, error rate, latency percentiles)
- Top routes and slowest routes (P95)
- Dependencies (Redis, SQL, external HTTP)
- Application internals (bootstrap, views, Livewire, MCP/Kiwix)
- Cross-service topology (hop latency, edge rates)
- Trace explorer (recent traces via Tempo, service map)
- Logs (volume by level, recent logs via Loki)

## Adding Custom Metrics

### 1. Add Method to DocumentMetrics

```php
// app/Services/Metrics/DocumentMetrics.php
public function customMetric(string $label): void
{
    $this->prometheus->incCounter(
        'custom_metric_total',
        'Description of custom metric',
        ['label' => $label]
    );
}
```

### 2. Instrument Your Code

```php
// In your controller or service
public function __construct(
    private DocumentMetrics $metrics
) {}

public function someMethod(): void
{
    $this->metrics->customMetric('value');
}
```

### 3. Available Metric Types

**Counter** (monotonically increasing):
```php
$this->prometheus->incCounter('metric_name', 'Description', ['label' => 'value'], 1);
```

**Gauge** (can go up or down):
```php
$this->prometheus->setGauge('metric_name', 'Description', 42.5, ['label' => 'value']);
```

**Histogram** (observations with buckets):
```php
$this->prometheus->observeHistogram('metric_name', 'Description', 0.123, ['label' => 'value']);
```

## Querying Metrics

### PromQL Examples

**Document upload rate (per minute)**:
```promql
rate(documcp_documents_uploaded_total[5m]) * 60
```

**95th percentile search duration**:
```promql
histogram_quantile(0.95, rate(documcp_document_search_duration_seconds_bucket[5m]))
```

**Average search results count**:
```promql
rate(documcp_document_search_results_sum[5m]) / rate(documcp_document_search_results_count[5m])
```

**HTTP request rate by status code**:
```promql
sum(rate(documcp_http_requests_total[5m])) by (status)
```

**Error rate (percentage)**:
```promql
sum(rate(documcp_http_requests_total{status=~"5.."}[5m])) / sum(rate(documcp_http_requests_total[5m])) * 100
```

**Authentication failure rate by reason**:
```promql
sum(rate(documcp_auth_failures_total[5m])) by (reason)
```

**Total auth failures in last hour**:
```promql
sum(increase(documcp_auth_failures_total[1h]))
```

## Troubleshooting

### Metrics endpoint returns empty

**Cause**: No metrics have been recorded yet.
**Solution**: Generate some activity (upload document, search, etc.) and refresh.

### Prometheus can't scrape /metrics

**Cause**: Firewall, authentication, or network issue.
**Solution**:
1. Verify endpoint is accessible: `curl http://documcp.your-domain.com/metrics`
2. Check Traefik labels in `docker-compose.yml`
3. Verify Prometheus scrape config targets correct hostname
4. Check Docker network connectivity

### Redis connection errors in logs

**Cause**: Redis unavailable or misconfigured.
**Solution**:
1. Verify Redis is running: `docker ps | grep redis`
2. Check Redis connection in `.env`:
   ```env
   REDIS_HOST=redis
   REDIS_PASSWORD=null
   REDIS_PORT=6379
   REDIS_DB=0
   ```
3. Test connection: `docker exec documcp_app redis-cli -h redis ping`

### High cardinality warnings

**Cause**: Too many unique label combinations.
**Solution**: Avoid high-cardinality labels like:
- User IDs (use aggregated metrics)
- UUIDs
- Timestamps
- IP addresses

**Good labels**: file_type, status, method, endpoint
**Bad labels**: user_id, document_uuid, ip_address

## Best Practices

### Label Cardinality

Keep label cardinality low (< 100 unique values per label):
- ✅ `file_type`: pdf, docx, xlsx (5 values)
- ✅ `status`: indexed, processing, failed (3 values)
- ❌ `document_id`: UUID (unbounded cardinality)

### Metric Naming

Follow Prometheus conventions:
- Use `_total` suffix for counters: `documents_uploaded_total`
- Use `_seconds` suffix for durations: `search_duration_seconds`
- Use base units: seconds (not milliseconds), bytes (not kilobytes)
- Use snake_case: `http_request_duration_seconds`

### Histogram Buckets

Default buckets: `[0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1.0, 2.5, 5.0, 7.5, 10.0]`

Adjust buckets based on your latency distribution:
```php
$histogram = $this->registry->getOrRegisterHistogram(
    $this->namespace,
    $name,
    $help,
    array_keys($labels),
    [0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0] // Custom buckets
);
```

## References

- [Prometheus Documentation](https://prometheus.io/docs/introduction/overview/)
- [Prometheus Best Practices](https://prometheus.io/docs/practices/naming/)
- [promphp/prometheus_client_php](https://github.com/PromPHP/prometheus_client_php)
- [Grafana Dashboards](https://grafana.com/docs/grafana/latest/dashboards/)
