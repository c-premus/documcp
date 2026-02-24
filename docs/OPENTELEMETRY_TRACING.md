# OpenTelemetry Distributed Tracing

This document describes how distributed tracing is implemented in DocuMCP using OpenTelemetry.

## Overview

DocuMCP uses [OpenTelemetry](https://opentelemetry.io/) to provide distributed tracing across HTTP requests, database queries, queue jobs, and external API calls. Traces are exported to Grafana Tempo for visualization and analysis.

The implementation uses [keepsuit/laravel-opentelemetry](https://github.com/keepsuit/laravel-opentelemetry) v2.0, which provides automatic instrumentation for Laravel components plus a convenient API for manual tracing.

### What Tracing Provides

- **Request flow visualization**: See how requests flow through the application
- **Performance bottleneck identification**: Find slow database queries, API calls, or processing steps
- **Error correlation**: Link exceptions to specific traces and spans
- **Cross-service tracing**: Follow requests across queue jobs and external services
- **Log correlation**: Match log entries to specific traces using trace IDs

## Configuration

### Environment Variables

Add these to your `.env` file:

```env
# Master switch for OpenTelemetry
OTEL_ENABLED=true

# Service name (appears in trace UI)
OTEL_SERVICE_NAME=documcp

# Traces exporter
# Development: console (outputs to stdout)
# Testing: memory (no output, exercises code paths)
# Production: otlp (sends to Grafana Alloy)
OTEL_TRACES_EXPORTER=otlp

# OTLP endpoint (required when using otlp exporter)
OTEL_EXPORTER_OTLP_ENDPOINT=http://alloy:4318
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf

# Sampling configuration
# 1.0 = sample everything (development)
# 0.1 = sample 10% (production)
OTEL_TRACES_SAMPLER_ARG=1.0

# Grafana URL for trace visualization links in admin dashboard
GRAFANA_URL=http://grafana:3000
```

### Exporter Options

| Exporter | Use Case | Description |
|----------|----------|-------------|
| `otlp` | Production | Sends traces to Grafana Alloy via OTLP HTTP protocol |
| `console` | Development | Outputs trace data to stdout for debugging |
| `memory` | Testing | Creates spans in memory, discarded at request end |

**Note**: Use `memory` instead of `null` for testing. In v2.0, `null` is a true no-op exporter (was in-memory in v1.x). Additionally, Laravel's `env()` converts the string `'null'` to PHP `NULL`, breaking config resolution.

### Sampler Types

| Sampler | Description |
|---------|-------------|
| `always_on` | Sample every request (default) |
| `always_off` | Disable sampling |
| `traceidratio` | Sample a percentage of requests |

For production, use `traceidratio` with `OTEL_TRACES_SAMPLER_ARG` set between 0.1 (10%) and 0.5 (50%) depending on traffic volume.

### User Context

v2.0 adds authenticated user context to traces. When enabled, `user.id` is attached to active spans.

```env
OTEL_USER_CONTEXT=true  # default
```

Only `user.id` is added — email and name are excluded to comply with OTEL attribute guidelines prohibiting PII in traces.

### Worker Mode Detection

v2.0 auto-detects long-running processes (Octane, Horizon, queue workers) and optimizes span flushing and metric collection.

```env
OTEL_WORKER_MODE_FLUSH_AFTER_EACH_ITERATION=false  # default
OTEL_WORKER_MODE_COLLECT_INTERVAL=60  # seconds
```

Built-in detectors: `OctaneWorkerModeDetector`, `QueueWorkerModeDetector`.

### Tail Sampling

v2.0 supports tail sampling — making sampling decisions based on complete trace data (errors, slow traces) rather than at trace start.

```env
OTEL_TRACES_TAIL_SAMPLING_ENABLED=false  # default, enable in production when ready
```

Rules:
- **ErrorsRule**: Keeps traces containing spans with `STATUS_ERROR`
- **SlowTraceRule**: Keeps traces exceeding a duration threshold (default 2000ms)

Tail sampling only works for single-service deployments. Use an OTEL Collector for distributed tail sampling.

### Scout Instrumentation

v2.0 adds automatic instrumentation for Laravel Scout (Meilisearch) operations:

```env
OTEL_INSTRUMENTATION_SCOUT=true  # default
```

Traces `search()`, `paginate()`, `update()`, and `delete()` operations.

### Console Command Tracing (v2.0 Behavior Change)

v2.0 changed console instrumentation from an exclude-list to an include-list. Only explicitly listed commands are traced:

```php
// config/opentelemetry.php
Instrumentation\ConsoleInstrumentation::class => [
    'enabled' => true,
    'commands' => [
        'migrate*',
        'queue:work',
        'horizon',
        'zim:*',
        'documcp:*',
    ],
],
```

Wildcards are supported (e.g., `migrate*` matches `migrate:fresh`, `migrate:reset`).

### Sensitive Query Parameters

v2.0 adds redaction of sensitive URL query parameters in traces:

```php
'sensitive_query_parameters' => ['token', 'code', 'state', 'nonce'],
```

Configured for both HTTP server and client instrumentation.

## Trace Flow

A typical request creates this span hierarchy:

```
HTTP Request (root span)
├── Database Query (automatic)
├── Database Query (automatic)
├── mcp.search_documents (manual)
│   └── search.meilisearch (manual)
│       └── HTTP Client Request (automatic)
└── Queue Job Dispatch (automatic)
```

### Automatic Instrumentation

The Laravel OpenTelemetry SDK automatically traces:

| Component | Span Name Pattern | Example |
|-----------|------------------|---------|
| HTTP Server | `{METHOD} {route}` | `POST /documcp` |
| HTTP Client | `{METHOD} {url}` | `GET https://api.example.com/v1/search` |
| Database | `{OPERATION}` | `SELECT`, `INSERT`, `UPDATE`, `DELETE` |
| Redis | `{COMMAND}` | `GET`, `SET`, `DEL`, `LPUSH` |
| Queue | `send {queue}` / `process {queue}` | `send default`, `process high` |
| Cache | SDK-managed | Cache get/put/forget operations |
| Events | SDK-managed | Laravel event dispatch |
| Views | SDK-managed | Blade template rendering |
| Livewire | SDK-managed | Livewire component updates |
| Console | Opt-in (listed commands only) | See [Console Command Tracing](#console-command-tracing-v20-behavior-change) |

### Manual Instrumentation

Services and MCP tools add custom spans using the `App\Support\Tracing` helper.

## Instrumented Components

### Services

**SearchService** (`app/Services/SearchService.php`):
- `search.meilisearch` - Full-text document search
- `search.meilisearch_facets` - Faceted search with filters
- `search.autocomplete` - Search suggestions

**KiwixServeClient** (`app/Services/Zim/KiwixServeClient.php`):
- `kiwix.get_entries` - Fetch ZIM archive catalog
- `kiwix.get_all_entries` - Fetch all catalog entries
- `kiwix.suggest` - Article title autocomplete
- `kiwix.search` - Full-text search within archive
- `kiwix.get_article` - Retrieve article content

**ConfluenceClient** (`app/Services/Atlassian/ConfluenceClient.php`):
- `confluence.get_spaces` - List Confluence spaces
- `confluence.get_space` - Get single space details
- `confluence.search` - CQL search for pages
- `confluence.get_page` - Retrieve page content
- `confluence.get_page_markdown` - Get page as Markdown

**GitTemplateClient** (`app/Services/GitTemplate/GitTemplateClient.php`):
- `git_template.sync` - Clone or pull repository
- `git_template.extract_files` - Extract files from repository
- `git_template.apply_variables` - Variable substitution in templates

### MCP Tools

All 18 MCP tools are traced with the span name pattern `mcp.{tool_name}`:

| Tool | Span Name |
|------|-----------|
| unified_search | `mcp.unified_search` |
| search_documents | `mcp.search_documents` |
| create_document | `mcp.create_document` |
| read_document | `mcp.read_document` |
| update_document | `mcp.update_document` |
| delete_document | `mcp.delete_document` |
| list_zim_archives | `mcp.list_zim_archives` |
| search_zim | `mcp.search_zim` |
| read_zim_article | `mcp.read_zim_article` |
| list_confluence_spaces | `mcp.list_confluence_spaces` |
| search_confluence | `mcp.search_confluence` |
| read_confluence_page | `mcp.read_confluence_page` |
| list_git_templates | `mcp.list_git_templates` |
| search_git_templates | `mcp.search_git_templates` |
| get_deployment_guide | `mcp.get_deployment_guide` |
| get_template_structure | `mcp.get_template_structure` |
| get_template_file | `mcp.get_template_file` |
| download_template | `mcp.download_template` |

Each MCP tool span includes these standard attributes:
- `mcp.tool` - Tool identifier
- `mcp.user_id` - Authenticated user ID (if available)

Plus tool-specific attributes like `mcp.search.query`, `mcp.search.results_count`, etc.

### Queue Jobs

All jobs extending `BaseJob` are traced automatically:

```php
// BaseJob wraps process() in a trace span
Tracing::trace('job.process', fn () => $this->process(), $this->getSpanAttributes());
```

Standard job attributes:
- `job.name` - Job class name
- `job.attempt` - Current attempt number
- `job.queue` - Queue name

Document processing jobs add:
- `document.id` - Document UUID

## Viewing Traces in Grafana Tempo

### Accessing Tempo

1. Open Grafana at `http://your-grafana:3000`
2. Navigate to Explore (compass icon)
3. Select "Tempo" as the data source

### Filtering by Service

Use the TraceQL query builder with:
- Service name filter: `service.name = "documcp"`

### Example TraceQL Queries

```traceql
# All MCP tool calls
{span.name =~ "mcp.*"}

# Search operations
{span.name =~ "search.*"}

# Failed spans only
{status = error}

# Spans longer than 500ms
{duration > 500ms}

# MCP search with specific query
{span.name = "mcp.search_documents" && span.mcp.search.query =~ ".*kubernetes.*"}

# Queue job execution
{span.name = "job.process"}

# External service calls
{span.name =~ "kiwix.*" || span.name =~ "confluence.*"}
```

### Reading Trace Waterfalls

Click any trace ID to see the full waterfall view:

1. **Root span** - The HTTP request that started the trace
2. **Child spans** - Nested operations within the request
3. **Attributes** - Key-value pairs on each span
4. **Events** - Timestamped occurrences within a span
5. **Errors** - Exceptions recorded on spans (shown in red)

## Adding Tracing to New Code

### Using the Tracing Helper

The `App\Support\Tracing` class provides a static API for tracing operations.

**Basic trace:**

```php
use App\Support\Tracing;

$result = Tracing::trace('operation.name', function () {
    return doSomething();
});
```

**With attributes:**

```php
$result = Tracing::trace('operation.name', function () use ($param) {
    // Add attributes at start of operation
    Tracing::addAttributes([
        'operation.param' => $param,
    ]);

    $result = doSomething();

    // Add result attributes
    Tracing::addAttributes([
        'operation.result_count' => count($result),
    ]);

    return $result;
});
```

**With initial attributes:**

```php
$result = Tracing::trace('operation.name', function () {
    return doSomething();
}, [
    'operation.param' => $param,
]);
```

**Recording exceptions:**

```php
try {
    doRiskyThing();
} catch (\Exception $e) {
    Tracing::recordException($e);
    throw $e;
}
```

**Adding events:**

```php
Tracing::trace('operation.name', function () {
    Tracing::addEvent('checkpoint.reached', [
        'items_processed' => 50,
    ]);

    // continue processing...
});
```

### Span Naming Convention

Use `{service}.{operation}` format:

- `search.meilisearch` - Meilisearch search operation
- `kiwix.get_article` - Kiwix article retrieval
- `confluence.search` - Confluence CQL search
- `mcp.create_document` - MCP tool invocation
- `job.process` - Queue job processing

### Attribute Naming Convention

- Use dot notation: `mcp.tool`, `search.query`, `confluence.page_id`
- Prefix with domain: `search.`, `kiwix.`, `confluence.`, `mcp.`, `job.`
- Truncate long strings to 200-300 characters to prevent trace bloat
- Never log credentials, tokens, or sensitive data

### Graceful Degradation

All `Tracing` methods are no-ops when OTEL is disabled:

```php
// These do nothing when OTEL_ENABLED=false
Tracing::trace('operation', fn () => $result);  // Just returns $result
Tracing::addAttributes(['key' => 'value']);     // No-op
Tracing::recordException($e);                    // No-op
```

This means you can add tracing code without impacting non-traced environments.

## Production Recommendations

### Sampling Strategy

```env
# Development: sample everything
OTEL_TRACES_SAMPLER_ARG=1.0

# Staging: sample 50%
OTEL_TRACES_SAMPLER_ARG=0.5

# Production (low traffic): sample 25%
OTEL_TRACES_SAMPLER_ARG=0.25

# Production (high traffic): sample 10%
OTEL_TRACES_SAMPLER_ARG=0.1
```

Balance trace coverage against storage costs and performance overhead.

### Tail Sampling Strategy

For production, consider enabling tail sampling to reduce telemetry volume while preserving important traces:

```env
OTEL_TRACES_TAIL_SAMPLING_ENABLED=true
OTEL_TRACES_TAIL_SAMPLING_SLOW_TRACES_THRESHOLD_MS=2000
```

This keeps all error traces and traces slower than 2 seconds, regardless of head sampling rate.

### Grafana Alloy Configuration

Alloy receives OTLP traces and forwards them to Tempo. Required configuration:

```hcl
// Receive OTLP traces
otelcol.receiver.otlp "default" {
  http {
    endpoint = "0.0.0.0:4318"
  }
  grpc {
    endpoint = "0.0.0.0:4317"
  }
  output {
    traces = [otelcol.processor.batch.default.input]
  }
}

// Batch traces for efficiency
otelcol.processor.batch "default" {
  timeout = "5s"
  send_batch_size = 1000
  output {
    traces = [otelcol.exporter.otlp.tempo.input]
  }
}

// Export to Tempo
otelcol.exporter.otlp "tempo" {
  client {
    endpoint = "tempo:4317"
    tls {
      insecure = true  // Use TLS in production
    }
  }
}
```

### Tempo Configuration

Ensure Tempo is configured to receive OTLP:

```yaml
# tempo.yaml
distributor:
  receivers:
    otlp:
      protocols:
        http:
        grpc:
```

## Troubleshooting

### Traces Not Appearing

1. **Check OTEL is enabled:**
   ```bash
   php artisan tinker
   >>> config('opentelemetry.enabled')
   => true
   ```

2. **Verify OTLP endpoint is reachable:**
   ```bash
   curl -v http://alloy:4318/v1/traces
   ```

3. **Check exporter setting:**
   ```bash
   grep OTEL_TRACES_EXPORTER .env
   # Should be 'otlp' for production
   ```

4. **Review Alloy logs for errors:**
   ```bash
   docker logs alloy
   ```

### Missing Spans

1. **Ensure Tracing import is present:**
   ```php
   use App\Support\Tracing;
   ```

2. **Verify code is wrapped in trace:**
   ```php
   return Tracing::trace('operation', function () {
       // your code here
   });
   ```

3. **Check for exceptions that might skip trace:**
   Exceptions thrown inside `Tracing::trace()` are recorded and re-thrown, but if an exception occurs before `trace()` is called, no span is created.

### Partial Traces (Missing Child Spans)

1. **Queue jobs not traced:**
   - Ensure queue worker is running
   - Check `QueueInstrumentation` is enabled in config

2. **Async operations:**
   - HTTP client calls outside the trace context won't be linked
   - Dispatch queue jobs within the trace span

### High Cardinality Issues

Avoid dynamic span names that create high cardinality:

```php
// BAD: Creates unique span for each document
Tracing::trace("document.{$documentId}", fn () => ...);

// GOOD: Use attributes for identifiers
Tracing::trace('document.read', function () use ($documentId) {
    Tracing::addAttributes(['document.id' => $documentId]);
    // ...
});
```

### Performance Impact

If tracing causes performance issues:

1. Reduce sample rate
2. Disable verbose instrumentations (views, cache, events)
3. Check for spans in tight loops

## Admin Dashboard Widget

The admin dashboard includes a "Distributed Tracing" widget showing:

- **Tracing status**: Enabled/disabled indicator
- **Service name**: Configured `OTEL_SERVICE_NAME`
- **OTLP endpoint**: Where traces are sent
- **Sampler type**: Current sampling strategy
- **Sample rate**: Percentage of requests traced
- **Grafana link**: Direct link to Tempo explore (when `GRAFANA_URL` configured)

The widget provides hints when tracing is disabled or Grafana URL is not set.

## Related Documentation

- [Grafana Alloy Logging](GRAFANA_ALLOY_LOGGING.md) - Structured logging with Loki
- [Prometheus Metrics](PROMETHEUS_METRICS.md) - Application metrics
- [Health Checks](HEALTH_CHECKS.md) - Service health endpoints
