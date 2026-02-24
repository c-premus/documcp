# Grafana Alloy Logging Integration

DocuMCP v1.4.0

## Overview

DocuMCP integrates with Grafana Alloy for centralized log collection and aggregation. Logs are formatted as structured JSON and sent to Grafana Loki for querying and analysis.

**Architecture**:
```
DocuMCP (JSON logs) → Docker json-file driver → Grafana Alloy → Loki → Grafana
```

## Grafana Alloy vs Promtail

Grafana Alloy replaces Promtail and collects logs, metrics, and traces in a single agent.

| Feature | Promtail | Grafana Alloy |
|---------|----------|---------------|
| Log collection | Yes | Yes |
| Metrics collection | No | Yes |
| Traces collection | No | Yes |
| OpenTelemetry support | No | Yes |
| Service discovery | Limited | Docker, Kubernetes, file-based |
| Configuration format | YAML | River (HCL-like) |

DocuMCP uses Alloy because:
- Single agent collects logs, metrics, and traces
- Native Docker container discovery
- Built-in JSON log parsing

## DocuMCP Logging Configuration

### Structured JSON Format

All logs are formatted as structured JSON with consistent fields:

```json
{
  "timestamp": "2025-11-22T10:30:45.123456+00:00",
  "level": "INFO",
  "level_value": 200,
  "message": "Document uploaded successfully",
  "channel": "documcp",
  "context": {
    "document_id": 123,
    "user_id": 456,
    "file_type": "pdf"
  },
  "extra": {
    "request_id": "550e8400-e29b-41d4-a716-446655440000",
    "ip": "192.168.1.100",
    "method": "POST",
    "path": "api/documents"
  },
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "user_id": 456,
  "ip": "192.168.1.100",
  "trace_id": "abc123def456...",
  "span_id": "789xyz...",
  "environment": "production",
  "application": "documcp"
}
```

### Key Fields

**Top-level fields** (for easy querying):

| Field | Description |
|-------|-------------|
| `timestamp` | ISO 8601 timestamp with microseconds |
| `level` | Log level name (DEBUG, INFO, WARNING, ERROR, CRITICAL) |
| `level_value` | Numeric log level (100-600) |
| `message` | Log message |
| `channel` | Logging channel name |
| `request_id` | Unique request ID for distributed tracing |
| `user_id` | Authenticated user ID (if available) |
| `ip` | Client IP address |
| `trace_id` | OpenTelemetry trace ID (if available from external systems like Airflow) |
| `span_id` | OpenTelemetry span ID (if available) |
| `environment` | Application environment (production, staging, dev) |
| `application` | Application name (documcp) |

**Nested fields**:
- `context`: Additional context data (document_id, file_type, etc.)
- `extra`: Extra metadata from Monolog processors (request_id, user_id, ip, method, path)

### Logging Channels

DocuMCP provides two structured JSON logging channels:

**1. `json` channel** (stdout):
```php
// .env
LOG_CHANNEL=json
```

Writes to `php://stdout` for Docker log collection.

**2. `json_daily` channel** (file):
```php
// .env
LOG_CHANNEL=json_daily
LOG_DAILY_DAYS=14
```

Writes to `storage/logs/laravel-json.log` with 14-day rotation.

### Request ID Middleware

The `AssignRequestId` middleware adds unique request IDs to:
- **Request attributes**: `$request->attributes->get('request_id')`
- **Response headers**: `X-Request-ID`
- **Log context**: Automatically included in all logs for that request

Example:
```php
// All logs in this request will include the request_id
Log::info('Processing document upload', ['document_id' => 123]);
```

## Docker Logging Configuration

DocuMCP containers use the `json-file` driver with labels for Alloy scraping:

```yaml
# docker-compose.yml
services:
  app:
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"
    labels:
      - "logging=alloy"
      - "service=documcp-app"
      - "application=documcp"
      - "environment=production"
```

**Labels explanation**:
- `logging=alloy`: Marks container for Alloy log collection
- `service=documcp-app`: Service name for filtering
- `application=documcp`: Application name
- `environment=production`: Environment tag

## Grafana Alloy Configuration

### Installation

Configuration file location:

```
/etc/alloy/config.alloy
```

### Log Collection from Docker

Add this configuration to scrape DocuMCP logs:

```hcl
// config.alloy

// Discover Docker containers with logging=alloy label
discovery.docker "documcp" {
  host = "unix:///var/run/docker.sock"
  filter {
    name   = "label"
    values = ["logging=alloy"]
  }
}

// Relabel container metadata as Loki labels
discovery.relabel "documcp" {
  targets = discovery.docker.documcp.targets

  rule {
    source_labels = ["__meta_docker_container_label_service"]
    target_label  = "service"
  }
  rule {
    source_labels = ["__meta_docker_container_label_application"]
    target_label  = "application"
  }
  rule {
    source_labels = ["__meta_docker_container_label_environment"]
    target_label  = "environment"
  }
  rule {
    source_labels = ["__meta_docker_container_name"]
    target_label  = "container"
  }
}

// Read logs from discovered containers
loki.source.docker "documcp" {
  host       = "unix:///var/run/docker.sock"
  targets    = discovery.relabel.documcp.output
  forward_to = [loki.process.documcp.receiver]
}

// Parse JSON logs and extract fields
loki.process "documcp" {
  forward_to = [loki.write.loki.receiver]

  stage.json {
    expressions = {
      level       = "level",
      message     = "message",
      request_id  = "request_id",
      user_id     = "user_id",
      ip          = "ip",
      trace_id    = "trace_id",
      span_id     = "span_id",
    }
  }

  // Promote important fields to Loki labels for filtering
  stage.labels {
    values = {
      level = "level",
    }
  }
}

// Send logs to Loki
loki.write "loki" {
  endpoint {
    url = "http://loki:3100/loki/api/v1/push"
  }
}
```

### Restart Alloy

After configuration changes:

```bash
# Restart Grafana Alloy
sudo systemctl restart alloy

# Check status
sudo systemctl status alloy

# View logs
sudo journalctl -u alloy -f
```

## Querying Logs in Grafana

### LogQL Basics

Grafana Loki uses LogQL for querying. Access via **Grafana → Explore → Loki**.

### Example Queries

**All DocuMCP logs**:
```logql
{application="documcp"}
```

**Logs from specific service**:
```logql
{application="documcp", service="documcp-app"}
```

**Error logs only**:
```logql
{application="documcp"} | json | level="ERROR"
```

**Logs for specific request ID** (distributed tracing):
```logql
{application="documcp"} | json | request_id="550e8400-e29b-41d4-a716-446655440000"
```

**Logs by OpenTelemetry trace ID** (correlate with external systems like Airflow):
```logql
{application="documcp"} | json | trace_id="abc123def456..."
```

**Logs for specific user**:
```logql
{application="documcp"} | json | user_id="456"
```

**Search for specific text**:
```logql
{application="documcp"} |= "Document uploaded"
```

**Document upload logs with context**:
```logql
{application="documcp"} | json | message =~ "(?i)document.*upload" | line_format "{{.timestamp}} [{{.level}}] {{.message}} (user={{.user_id}}, doc={{.context_document_id}})"
```

**Rate of errors (per minute)**:
```logql
sum(rate({application="documcp"} | json | level="ERROR" [5m])) * 60
```

**Top 10 error messages**:
```logql
topk(10, sum by (message) (count_over_time({application="documcp"} | json | level="ERROR" [24h])))
```

**Logs with specific context field**:
```logql
{application="documcp"} | json | context_document_id > 0
```

### Advanced Filtering

**Multiple conditions**:
```logql
{application="documcp", service="documcp-app"}
| json
| level="ERROR"
| ip="192.168.1.100"
```

**Regex matching**:
```logql
{application="documcp"} | json | message =~ "(?i)(error|exception|fail)"
```

**Numeric comparisons**:
```logql
{application="documcp"} | json | user_id != "null" | level_value >= 400
```

**Time range** (use Grafana time picker or):
```logql
{application="documcp"} | json [1h]
```

## Log Aggregation Patterns

### Request Lifecycle Tracing

Track all logs for a single request using request_id:

```logql
{application="documcp"}
| json
| request_id="550e8400-e29b-41d4-a716-446655440000"
| line_format "{{.timestamp}} [{{.level}}] {{.message}}"
```

### User Activity Monitoring

All actions by a specific user:

```logql
{application="documcp"}
| json
| user_id="456"
| line_format "{{.timestamp}} {{.method}} {{.path}} - {{.message}}"
```

### Error Rate by Service

```logql
sum by (service) (rate({application="documcp"} | json | level="ERROR" [5m]))
```

### Document Processing Pipeline

Track document through upload → processing → indexing:

```logql
{application="documcp"}
| json
| context_document_id="123"
| line_format "{{.timestamp}} [{{.level}}] {{.message}}"
```

## Grafana Dashboard

### Creating Log Panels

**1. Error rate panel**:
- **Query**: `sum(rate({application="documcp"} | json | level="ERROR" [5m])) * 60`
- **Visualization**: Time series
- **Unit**: errors/min

**2. Log volume by level**:
- **Query**: `sum by (level) (count_over_time({application="documcp"} | json [5m]))`
- **Visualization**: Stacked area chart
- **Legend**: `{{level}}`

**3. Recent errors table**:
- **Query**: `{application="documcp"} | json | level="ERROR"`
- **Visualization**: Logs panel
- **Fields**: timestamp, level, message, request_id, user_id

**4. Top error messages**:
- **Query**: `topk(10, sum by (message) (count_over_time({application="documcp"} | json | level="ERROR" [1h])))`
- **Visualization**: Bar chart

**5. Request latency (if you log durations)**:
- **Query**: `quantile_over_time(0.95, {application="documcp"} | json | unwrap context_duration [5m])`
- **Visualization**: Time series
- **Unit**: seconds

## Alerts

### Configuring Loki Alerts

Create alerts in Grafana for critical conditions:

**High error rate**:
```logql
sum(rate({application="documcp"} | json | level="ERROR" [5m])) > 10
```

**No logs received** (dead service):
```logql
absent_over_time({application="documcp"}[5m])
```

**Critical errors**:
```logql
count_over_time({application="documcp"} | json | level="CRITICAL" [5m]) > 0
```

**Failed document processing**:
```logql
count_over_time({application="documcp"} | json | message =~ "(?i)processing.*failed" [5m]) > 5
```

### Alert Routing

Send alerts to Apprise:

```yaml
# alertmanager.yml
route:
  receiver: 'apprise'
  routes:
    - match:
        severity: critical
      receiver: 'apprise'

receivers:
  - name: 'apprise'
    webhook_configs:
      - url: 'http://apprise:8000/notify/apprise?tag=documcp'
```

## Troubleshooting

### No logs appearing in Loki

**Check Docker labels**:
```bash
docker inspect documcp_app | grep -A 5 Labels
```

Should include:
```json
"logging=alloy"
"service=documcp-app"
```

**Check Alloy is scraping**:
```bash
# View Alloy logs
sudo journalctl -u alloy -f

# Check Alloy targets
curl http://localhost:12345/targets  # Adjust port for your setup
```

**Verify Loki connectivity**:
```bash
# Test Loki endpoint
curl http://loki:3100/ready
```

### Logs not structured (plain text)

**Check LOG_CHANNEL**:
```bash
# .env
LOG_CHANNEL=json  # or json_daily
```

**Verify formatter**:
```php
// config/logging.php
'formatter' => \App\Logging\StructuredJsonFormatter::class,
```

### Missing request_id field

**Check middleware registration**:
```php
// bootstrap/app.php
$middleware->append(\App\Http\Middleware\AssignRequestId::class);
```

**Verify middleware is running**:
```bash
# Check response headers
curl -I http://documcp.local/api/documents
# Should include: X-Request-ID: <uuid>
```

### High log volume

**Increase log level** (production):
```bash
# .env
LOG_LEVEL=error  # Only log errors and above
```

**Adjust log retention**:
```yaml
# docker-compose.yml
logging:
  options:
    max-size: "5m"    # Reduce from 10m
    max-file: "2"     # Reduce from 3
```

**Filter verbose logs in Alloy**:
```hcl
// config.alloy
loki.process "documcp" {
  stage.match {
    selector = "{level=\"DEBUG\"}"
    action   = "drop"
  }
}
```

## Best Practices

### Log Levels

Use appropriate log levels:
- **DEBUG**: Detailed debugging information (disable in production)
- **INFO**: General informational messages
- **WARNING**: Warning messages, but application continues
- **ERROR**: Error messages, request failed
- **CRITICAL**: Critical errors, service impaired

### Context Fields

Always include relevant context:
```php
Log::info('Document uploaded', [
    'document_id' => $document->id,
    'user_id' => $user->id,
    'file_type' => $document->file_type,
    'file_size' => $document->file_size,
]);
```

### Request Tracing

Use request_id for distributed tracing:
```php
$requestId = request()->attributes->get('request_id');
Log::info('Processing started', ['request_id' => $requestId]);
// ... processing ...
Log::info('Processing completed', ['request_id' => $requestId]);
```

### Performance Considerations

- **Avoid high cardinality labels**: Don't use user_id, request_id, or IP as Loki labels
- **Use JSON fields for filtering**: Extract with `| json` instead of labels
- **Limit log retention**: Configure Loki retention (default 7-30 days)
- **Use log sampling** for high-traffic endpoints

## References

- [Grafana Alloy Documentation](https://grafana.com/docs/alloy/latest/)
- [Grafana Loki Documentation](https://grafana.com/docs/loki/latest/)
- [LogQL Query Language](https://grafana.com/docs/loki/latest/logql/)
- [Docker Logging Drivers](https://docs.docker.com/config/containers/logging/configure/)
- [Laravel Logging Documentation](https://laravel.com/docs/12.x/logging)
