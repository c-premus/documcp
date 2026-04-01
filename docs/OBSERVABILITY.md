# Observability

## Overview

DocuMCP produces traces, metrics, structured logs, and error reports. Each signal flows to a dedicated backend, and Grafana queries all three for a unified view.

```
DocuMCP App
  ├── OTLP HTTP ──> Tempo (traces) ──> Prometheus (span metrics + service graph)
  ├── /metrics ──> Prometheus (native app metrics)
  ├── stdout JSON ──> Alloy/Promtail ──> Loki (logs)
  └── Sentry SDK ──> GlitchTip (errors, optional)

Grafana reads from: Prometheus, Loki, Tempo
```

All four signals are optional. Each subsystem is disabled by default and activates only when its configuration is present.

## Tracing (OpenTelemetry)

Package: `internal/observability/tracer.go`, `middleware.go`

DocuMCP exports traces over OTLP HTTP. A custom `observability.Tracing()` middleware (not `otelhttp` or `otelchi`) creates server spans, extracts trace context from incoming headers, and injects it into response headers.

### Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `OTEL_ENABLED` | `false` | Enable tracing |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | -- | OTLP HTTP endpoint (e.g., `tempo:4318`) |
| `OTEL_SERVICE_NAME` | `documcp` | `service.name` resource attribute |
| `OTEL_INSECURE` | `false` | Use HTTP instead of HTTPS for OTLP |
| `OTEL_SAMPLE_RATE` | `1.0` | Trace sampling rate (0.0-1.0) |
| `OTEL_ENVIRONMENT` | -- | `deployment.environment` resource attribute |
| `OTEL_SERVICE_VERSION` | -- | `service.version` (falls back to build ldflags) |

### Propagation

W3C TraceContext and Baggage. The middleware reads `traceparent`/`tracestate` from incoming requests and writes them back on responses.

### Sampling

The default sampler is `AlwaysSample()`. When `OTEL_SAMPLE_RATE` is set below `1.0`, the sampler switches to `TraceIDRatioBased`.

**Important:** `AlwaysSample()` ignores upstream proxy sampling decisions. If a reverse proxy like Traefik sets a sampling flag in `traceparent`, DocuMCP overrides it and samples the trace anyway. This is intentional -- the application owns its sampling policy.

### Resource Attributes

Three attributes are set on the tracer resource:

- `service.name` -- always present, from `OTEL_SERVICE_NAME`
- `service.version` -- from `OTEL_SERVICE_VERSION`, falls back to version embedded via build ldflags
- `deployment.environment` -- from `OTEL_ENVIRONMENT`, omitted when not set

### Span Details

**Naming:** Uses chi's `RoutePattern()` for low-cardinality span names. A request to `/api/documents/abc-123` produces a span named `GET /api/documents/{uuid}`, not `GET /api/documents/abc-123`.

**Status:** HTTP 5xx responses set span status to `codes.Error`. Other status codes leave the span unset.

**Attributes on every span:**

| Attribute | Example |
|-----------|---------|
| `http.request.method` | `GET` |
| `url.path` | `/api/documents/abc-123` |
| `http.response.status_code` | `200` |
| `http.route` | `/api/documents/{uuid}` |
| `http.request.body.size` | `1024` |
| `http.response_content_length` | `4096` |

### Middleware Position

The tracing middleware is mounted at the router root, before all application middleware. Every route is traced.

## Metrics (Prometheus)

Package: `internal/observability/metrics.go`

DocuMCP exposes 14 application metrics at `GET /metrics`. When `INTERNAL_API_TOKEN` is set, the endpoint requires `Authorization: Bearer <token>`. See [PROMETHEUS_METRICS.md](PROMETHEUS_METRICS.md) for the full metric listing, PromQL examples, and scrape configuration.

Metrics at a glance:

- **3 HTTP metrics** -- request count, duration histogram, active connections
- **1 search metric** -- search latency by index
- **1 application metric** -- document count gauge
- **4 queue metrics** -- jobs dispatched, completed, failed, duration
- **5 database metrics** -- connection pool stats collected from `pgxpool.Stat()` via a custom `prometheus.Collector`

## Structured Logging (slog)

DocuMCP uses Go's standard library `log/slog`.

### Format

- **Production** (`APP_ENV=production`): JSON
- **Development** (any other value): text

### Trace Correlation

When tracing is enabled, `trace_id` and `span_id` fields are injected into every log entry. This links logs to their corresponding traces in Grafana.

### HTTP Request Logs

Every HTTP request log includes a `client_ip` field, resolved by the RealIP middleware (respects `X-Forwarded-For` / `X-Real-IP` behind a reverse proxy).

### Auth Failure Logs

Auth failures are logged at WARN level with consistent prefixes for filtering:

| Prefix | Context |
|--------|---------|
| `"auth failed: "` | Token/session auth failures. Includes `client_ip`, `path`, `method`. |
| `"oauth token failed: "` | OAuth token endpoint failures. Includes `client_ip`, `client_id`. |

Device flow `authorization_pending` responses are excluded from logging. These are normal polling behavior, not abuse indicators.

## Error Tracking (Sentry / GlitchTip)

Package: `internal/observability/sentry.go`

DocuMCP uses the `getsentry/sentry-go` SDK (v0.44.1) for error tracking. The backend is compatible with self-hosted GlitchTip (a Sentry-compatible alternative). The frontend uses `@sentry/vue`.

### Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `SENTRY_DSN` | -- | Sentry/GlitchTip DSN (empty = disabled) |
| `SENTRY_ENVIRONMENT` | `APP_ENV` value | Environment tag |
| `SENTRY_RELEASE` | build version | Release tag |
| `SENTRY_SAMPLE_RATE` | `1.0` | Error sample rate (0.0-1.0) |
| `VITE_SENTRY_DSN` | -- | Frontend Sentry DSN (empty = disabled) |

When `SENTRY_DSN` is empty, the SDK is not initialized. All capture calls become no-ops.

### Tracing Separation

`tracesSampleRate` is set to `0`. OpenTelemetry handles distributed tracing. Sentry is used for error tracking only.

### Panic Recovery

The `SafeRecoverer` middleware captures panics via `sentry.RecoverWithContext()`. This replaces Go's default panic-and-crash behavior with structured error reporting.

### User Context

The auth middleware calls `SetUser(ctx, id, email)` to tag Sentry events with the authenticated user. Errors reported after authentication include user identity.

### Context-Aware Capture

`CaptureException(ctx, err)` uses the Sentry hub from the request context when available. This ensures events inherit the correct scope (user, tags, breadcrumbs).

### Lifecycle

`InitSentry()` follows the same pattern as `InitTracer()`: it returns a flush function that the Foundation stores and calls during shutdown. Sentry flushes before the tracer closes, so in-flight error events can still include trace context.

## Grafana Dashboard

The dashboard is defined as TypeScript code using `@grafana/grafana-foundation-sdk` in the `grafana/` directory.

### Generating the Dashboard

```bash
cd grafana && npm run generate
```

This produces `dist/documcp.json`. CI validates that the checked-in JSON matches the TypeScript source (`git diff --exit-code dist/documcp.json`).

### Panel Groups

The dashboard has 7 panel groups across 3 datasources (Prometheus, Tempo, Loki):

| Group | Datasource | What It Shows |
|-------|------------|---------------|
| RED Metrics | Prometheus (via Tempo span metrics) | Request rate, error rate, latency percentiles |
| Routes | Prometheus (via Tempo span metrics) | Per-route request table, slowest routes bar gauge |
| Dependencies | Prometheus (via Tempo span metrics) | SQL query rate/latency, external HTTP calls |
| Go Runtime | Prometheus (native) | DB pool, wait time, active connections, document count, HTTP rate, search latency, MCP/Kiwix operations |
| Cross-Service Topology | Prometheus (via Tempo service graph) | Hop latency, edge request/error rates |
| Traces | Tempo | Recent traces table with drill-down links, service map |
| Logs | Loki | Log volume by level, recent logs with trace correlation |

### Service Identifiers

The dashboard queries use hardcoded service identifiers that must match the observability stack:

| Query Context | Identifier |
|---------------|------------|
| Tempo span metrics (PromQL) | `service="documcp"` |
| Loki log queries (LogQL) | `service_name="documcp-app"` |
| Tempo TraceQL | `resource.service.name = "documcp"` |

The `documcp` value comes from the OTEL `service.name` resource attribute. The `documcp-app` value comes from container/Alloy labels. These are not configurable in the dashboard itself.

### Deployment

The generated JSON is copied to Grafana's file-based provisioning directory during deployment.

## Putting It Together

### Minimum Setup (Logs Only)

No configuration needed. Set `APP_ENV=production` for JSON output, then point Alloy/Promtail at stdout.

### Add Tracing

```bash
OTEL_ENABLED=true
OTEL_EXPORTER_OTLP_ENDPOINT=tempo:4318
OTEL_INSECURE=true
```

### Add Error Tracking

```bash
SENTRY_DSN=https://key@glitchtip.example.com/1
```

### Add Metrics Scraping

Configure Prometheus to scrape `/metrics`. See [PROMETHEUS_METRICS.md](PROMETHEUS_METRICS.md) for the scrape config.

### Full Stack

```bash
# Tracing
OTEL_ENABLED=true
OTEL_EXPORTER_OTLP_ENDPOINT=tempo:4318
OTEL_INSECURE=true
OTEL_SERVICE_NAME=documcp
OTEL_ENVIRONMENT=production
OTEL_SAMPLE_RATE=1.0

# Error tracking
SENTRY_DSN=https://key@glitchtip.example.com/1
SENTRY_ENVIRONMENT=production

# Metrics endpoint protection
INTERNAL_API_TOKEN=your-secret-token

# Logging
APP_ENV=production
```
