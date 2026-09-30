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

Logs and Prometheus metrics are always on: every process writes logs to stdout, registers the metrics, and serves `/metrics`. Tracing and error tracking are disabled by default and activate only when their configuration is present (`OTEL_ENABLED=true`, `SENTRY_DSN`).

## Tracing (OpenTelemetry)

Package: `internal/observability/tracer.go`, `middleware.go`

DocuMCP exports traces over OTLP HTTP. A custom `observability.Tracing()` middleware (not `otelhttp` or `otelchi`) creates a new root server span for each request, records any incoming trace context as a span link, and injects the new span's context into response headers.

### Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `OTEL_ENABLED` | `false` | Enable tracing |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | -- | OTLP HTTP endpoint (e.g., `tempo:4318`) |
| `OTEL_SERVICE_NAME` | `documcp` | `service.name` resource attribute |
| `OTEL_INSECURE` | `false` | Use HTTP instead of HTTPS for OTLP. If the endpoint includes a scheme (`http://...`), the scheme decides. |
| `OTEL_SAMPLE_RATE` | `1.0` | Trace sampling rate (0.0-1.0). See [Sampling](#sampling). |
| `OTEL_ENVIRONMENT` | -- | `deployment.environment` resource attribute |
| `OTEL_SERVICE_VERSION` | -- | `service.version` (falls back to build ldflags) |

### Propagation

W3C TraceContext and Baggage are registered as the global propagator.

Inbound HTTP requests always start a **new root span**. If the request carries a valid `traceparent`, the middleware does not use it as the parent. It attaches the incoming span context to the new span as a **span link** instead. The response carries the `traceparent` of DocuMCP's own span, not the caller's.

This is intentional. Behind a `Cloudflare Tunnel -> Traefik -> DocuMCP` chain, the upstream parent span often never reaches Tempo, which produces orphan traces ("root span not yet received"). The consequence: DocuMCP's trace is not joined to the proxy's trace. To find the caller's trace, query the link, for example TraceQL `link.traceId`, when both ends export to the same backend.

Outbound calls (Kiwix, OIDC) do propagate context: `otelhttp` injects `traceparent` into those requests.

### Sampling

The sampler is `AlwaysSample()` unless `0 < OTEL_SAMPLE_RATE < 1`, in which case it is `TraceIDRatioBased(OTEL_SAMPLE_RATE)`. Both `1.0` and `0` select `AlwaysSample()`. Setting `0` does **not** turn tracing off; use `OTEL_ENABLED=false` for that. Values outside 0.0-1.0 fail config validation.

Neither sampler is wrapped in `ParentBased`, so upstream sampling flags are ignored. Because inbound requests start new root traces anyway (see [Propagation](#propagation)), a proxy that sends `sampled=0` cannot suppress DocuMCP's traces.

### Resource Attributes

Three attributes are set on the tracer resource:

- `service.name` -- always present, from `OTEL_SERVICE_NAME`
- `service.version` -- from `OTEL_SERVICE_VERSION`, falls back to version embedded via build ldflags
- `deployment.environment` -- from `OTEL_ENVIRONMENT`, omitted when not set

### Span Details

**Naming:** Uses chi's `RoutePattern()` for low-cardinality span names. A request to `/api/documents/abc-123` produces a span named `GET /api/documents/{uuid}`, not `GET /api/documents/abc-123`. When chi matches no route pattern, the span keeps its initial name, `<METHOD> <raw path>`.

**Status:** HTTP 5xx responses set span status to `codes.Error`. Other status codes leave the span unset.

**Attributes:**

| Attribute | Example | When set |
|-----------|---------|----------|
| `http.request.method` | `GET` | Always |
| `url.path` | `/api/documents/abc-123` | Always |
| `http.response.status_code` | `200` | Always |
| `http.response_content_length` | `4096` | Always (bytes written) |
| `http.request.body.size` | `1024` | Only when the request `Content-Length` is greater than 0 |
| `http.route` | `/api/documents/{uuid}` | Only when chi matched a route pattern |

### Middleware Position

The tracing middleware is mounted globally, and only when `OTEL_ENABLED=true`. It runs after `RequestID`, `RealIP`, `SafeRecoverer`, and `SecurityHeaders`. It runs before `RequestLogger` (so log lines get `trace_id`/`span_id`), the Prometheus metrics middleware, and the remaining application middleware.

Not every request is traced. The middleware skips any path starting with `/health` (`/health`, `/health/ready`) and `/metrics`, so probes and scrapes produce no HTTP spans.

Because `SafeRecoverer` runs outside the tracing middleware, a handler panic ends the span without a status code attribute or error status. `SafeRecoverer` still logs the panic and reports it to Sentry when Sentry is configured.

### External Connection Tracing

In addition to the HTTP server middleware, DocuMCP instruments its main outbound connections so traces show the full request lifecycle -- not just the server span.

Two dependencies are deliberately left uninstrumented: a small "bare" PostgreSQL pool and a "bare" Redis client. They serve readiness probes, the `documcp_ready` and `documcp_river_leader_active` gauges, and (for Redis) distributed rate limiting. Traffic on these clients produces no spans.

**PostgreSQL (otelpgx)**

Every query on the main pool creates a span named by the low-cardinality operation (`SELECT`, `INSERT`, `UPDATE`, `DELETE`), with no `query`/`prepare` prefix. The `db.query.text` attribute contains the full query text. No manual instrumentation is needed in repository code.

The tracer is created with `otelpgx.NewTracer()` and no options. Since otelpgx v0.12.0, this naming is the default, which keeps span names low-cardinality in Tempo's span metrics (one series per operation instead of one per unique SQL query). To get the full SQL in the span name back, use the opt-in `WithFullSQLInSpanName()`.

Package: `github.com/exaring/otelpgx`, configured in `internal/database/pgxpool.go`.

**Redis (redisotel)**

Every command on the main Redis client creates a span (`SET`, `GET`, `PUBLISH`, etc.). This covers Pub/Sub event delivery and application queries. Rate limiting and readiness pings use the bare client and are not traced.

Package: `github.com/redis/go-redis/extra/redisotel/v9`, configured in `internal/app/foundation.go`.

**Kiwix HTTP Client (otelhttp)**

Outbound HTTP requests to Kiwix Serve instances create `SPAN_KIND_CLIENT` spans named `HTTP <METHOD>` (for example `HTTP GET`), with `http.request.method`, `url.full`, `server.address`, and `http.response.status_code` attributes. The `traceparent` header is injected automatically for cross-service correlation.

Package: `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp`, configured in `internal/client/kiwix/client.go`.

**OIDC HTTP Client (otelhttp)**

All outbound OIDC calls (discovery, JWKS, token exchange) go through an `otelhttp` transport and produce the same `HTTP <METHOD>` client spans as Kiwix. These are infrequent, login-only operations.

Package: `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp`, configured in `internal/auth/oidc/oidc.go`.

**Git Operations (manual spans)**

Git operations create spans via `otel.Tracer("documcp/git")`:

| Span | Kind | Attributes |
|------|------|------------|
| `git.sync` | Internal | `git.template_id`, `git.url`, `git.branch`; on success also `git.commit_sha`, `git.file_count`, `git.total_bytes` |
| `git.clone` | `SPAN_KIND_CLIENT` | `git.url`, `git.branch` |
| `git.pull` | `SPAN_KIND_CLIENT` | `git.dir`; `git.already_up_to_date` when nothing changed |

Package: `go.opentelemetry.io/otel/trace`, instrumented in `internal/client/git/sync.go` (`git.sync`) and `internal/client/git/client.go` (`git.clone`, `git.pull`).

### Background Job Spans

River does not carry trace context from the code that enqueues a job to the worker that runs it. Each job run therefore starts a new root span from the `documcp/worker` tracer, named `job.<kind>` (for example `job.document_extract`), with kind Internal and a `job.kind` attribute. A job that returns an error records it on the span and sets status `codes.Error`. Database, Redis, HTTP, and Git spans created during the job are children of this span.

Package: `internal/queue/workers.go` and `internal/queue/scheduler_workers.go`.

## Metrics (Prometheus)

Package: `internal/observability/metrics.go`, `river_leader.go`

Metrics are always registered and `/metrics` is always served; there is no switch to disable them. DocuMCP exposes 22 application metrics at `GET /metrics` on the main HTTP port. In worker-only mode (`documcp worker`), the same endpoint is served on `WORKER_HEALTH_PORT` (default `9090`). When `INTERNAL_API_TOKEN` is set, the endpoint requires `Authorization: Bearer <token>` in both modes. See [PROMETHEUS_METRICS.md](PROMETHEUS_METRICS.md) for the full metric listing, PromQL examples, and scrape configuration.

Metrics at a glance:

- **3 HTTP metrics** -- request count, duration histogram, in-flight requests
- **1 search metric** -- search latency by index
- **3 application gauges** -- document count, readiness (`documcp_ready`), River leader present (`documcp_river_leader_active`)
- **1 security metric** -- OAuth token replays detected, by type
- **4 queue metrics** -- jobs dispatched, completed, failed, duration
- **5 database metrics** -- connection pool stats collected from `pgxpool.Stat()` via a custom `prometheus.Collector`
- **5 Redis metrics** -- connection pool stats collected from `redis.PoolStats()` via a custom `prometheus.Collector`

The endpoint also exposes the Go client library's default `go_*` and `process_*` metrics.

## Structured Logging (slog)

DocuMCP uses Go's standard library `log/slog`.

### Format

- **Production and staging** (`APP_ENV=production` or `APP_ENV=staging`): JSON
- **Any other value** (for example `development`, `testing`): text

### Trace Correlation

When tracing is enabled, the logger adds `trace_id` and `span_id` fields to log entries written with a context that carries an active span (the `slog` `...Context` methods, such as `InfoContext`). The per-request `request completed` log uses this path, so it links to its trace in Grafana. Log calls made without a context, such as `logger.Warn(...)`, do not get these fields.

### HTTP Request Logs

Every HTTP request, except requests to `/health*` and `/metrics`, produces one `request completed` log line with `method`, `path`, `status`, `duration`, `client_ip`, and `request_id`. The `client_ip` field is resolved by the RealIP middleware. It honors `X-Forwarded-For` / `X-Real-IP` only when the direct connection comes from a network listed in `TRUSTED_PROXIES`; otherwise it is the TCP peer address.

### Auth Failure Logs

Auth failures are logged at WARN level with consistent prefixes for filtering. One exception: a session rejected for exceeding its absolute lifetime logs at INFO.

| Prefix | Context |
|--------|---------|
| `"auth failed: "` | Token/session auth failures. Includes `client_ip`, `path`, `method`. |
| `"oauth token failed: "` | OAuth token endpoint failures. Includes `client_ip`, `client_id`. |

Device flow `authorization_pending` responses are excluded from logging. These are normal polling behavior, not abuse indicators.

## Error Tracking (Sentry / GlitchTip)

Package: `internal/observability/sentry.go`

DocuMCP uses the `getsentry/sentry-go` SDK for error tracking. The backend is compatible with self-hosted GlitchTip (a Sentry-compatible alternative). The frontend uses `@sentry/vue`.

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

The auth middleware calls `SetUser(ctx, id)` to tag Sentry events with the authenticated user's internal numeric ID. Email, IP address, and username are deliberately not transmitted — `BeforeSend` defensively scrubs `event.User.Email`, `event.User.IPAddress`, and `event.User.Username` even if a future caller (e.g. `sentryhttp.New`) populates them through a different code path. Only the opaque user ID survives the scrub, sufficient for correlating multiple events from the same user without exposing PII.

### Context-Aware Capture

`CaptureException(ctx, err)` uses the Sentry hub from the request context when available. This ensures events inherit the correct scope (user, tags, breadcrumbs).

### Lifecycle

`InitSentry()` follows the same pattern as `InitTracer()`: it returns a flush function that the Foundation stores and calls during shutdown. Sentry flushes before the tracer closes, so in-flight error events can still include trace context.

## Grafana Dashboard

The dashboard is defined as TypeScript code using `@grafana/grafana-foundation-sdk` in the `grafana/` directory. Alert rules are defined in `grafana/src/alerts/rules.ts`.

### Generating the Dashboard and Alerts

```bash
cd grafana && npm ci && npm run generate
```

This writes two files to the `dist/` directory at the **repository root** (not `grafana/dist/`):

- `dist/documcp.json` -- the dashboard (UID `documcp-observability-v2`)
- `dist/alerts/documcp.json` -- Grafana unified-alerting provisioning file

CI regenerates both and fails if the checked-in JSON differs from the TypeScript source (`git diff --exit-code dist/documcp.json dist/alerts/documcp.json`).

### Alert Rules

The alerts file defines one rule group (`documcp`, folder `DocuMCP`, evaluated every 30s) with two rules:

| UID | Title | Fires when | For | Severity |
|-----|-------|------------|-----|----------|
| `documcp_readiness_failing` | `DocuMCP — readiness failing` | `documcp_ready == 0` | 2m | critical |
| `documcp_no_river_leader` | `DocuMCP — no River leader` | `documcp_river_leader_active == 0` | 5m | critical |

Both rules also fire on no data or query errors (`noDataState` and `execErrState` are `Alerting`), so a DocuMCP instance that stops exposing `/metrics` triggers them. The rules reference the Prometheus datasource UID `prometheus`. If your datasource has a different UID, edit the generated file before provisioning.

### Panel Groups

The dashboard has 8 panel groups across 3 datasources (Prometheus, Tempo, Loki):

| Group | Datasource | What It Shows |
|-------|------------|---------------|
| RED Metrics | Prometheus (via Tempo span metrics) | Request rate, error rate, latency percentiles |
| Routes | Prometheus (via Tempo span metrics) | Per-route request table, slowest routes bar gauge |
| Dependencies | Prometheus (via Tempo span metrics) | SQL rate/latency (filtered by `db_system="postgresql"`), Redis command rate/latency (`db_system="redis"`), outbound HTTP and Git operation rates and latency. The "HTTP (Kiwix)" series matches all `HTTP <METHOD>` client spans, so it also includes OIDC calls. The Git series matches client spans only (`git.clone`, `git.pull`), not `git.sync`. |
| Connection Pools & App Metrics | Prometheus (native) | DB pool, DB wait, Redis pool, active connections, document count, HTTP rate, HTTP latency (P50/P95/P99), search latency |
| Queue Operations | Prometheus (native) | Job rate by kind (dispatched/completed/failed), job duration P95 |
| Cross-Service Topology | Prometheus (via Tempo service graph) | Hop latency, edge request/error rates |
| Traces | Tempo | Recent server and client traces with drill-down links, service map |
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

Copy the generated JSON files to Grafana's file-based provisioning directories during deployment: the dashboard to a dashboards provider path, and `dist/alerts/documcp.json` to `provisioning/alerting/`.

## Putting It Together

### Minimum Setup (Logs Only)

No configuration needed. Set `APP_ENV=production` (or `staging`) for JSON output, then point Alloy/Promtail at stdout.

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

The endpoint is always on. Configure Prometheus to scrape `/metrics` (on `WORKER_HEALTH_PORT` for worker-only processes). See [PROMETHEUS_METRICS.md](PROMETHEUS_METRICS.md) for the scrape config.

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

# Metrics endpoint protection (required when APP_ENV=production)
INTERNAL_API_TOKEN=your-secret-token

# Logging
APP_ENV=production
```
