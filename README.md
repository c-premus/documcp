# DocuMCP

[![CI](https://github.com/c-premus/documcp/actions/workflows/ci.yaml/badge.svg)](https://github.com/c-premus/documcp/actions/workflows/ci.yaml)
[![Release](https://img.shields.io/github/v/tag/c-premus/documcp?label=release)](https://github.com/c-premus/documcp/tags)
[![Image Size](https://img.shields.io/docker/image-size/cpremus/documcp?sort=semver&label=image%20size)](https://hub.docker.com/r/cpremus/documcp)
[![Go Version](https://img.shields.io/github/go-mod/go-version/c-premus/documcp)](https://go.dev/)
[![License](https://img.shields.io/github/license/c-premus/documcp)](LICENSE)

A documentation server that exposes knowledge bases through the [Model Context Protocol](https://modelcontextprotocol.io/) (MCP), enabling AI agents to search, read, and manage documentation.

DocuMCP gives AI agents structured access to your documentation via MCP tools and prompts. It handles document ingestion, full-text search, and OAuth 2.1 authorization. Written in Go for single-binary deployment with low resource usage.

## Features

- **MCP Server** -- 16 tools and 6 prompts via the official [Go MCP SDK](https://github.com/modelcontextprotocol/go-sdk). Search, read, create, update, and delete documents. Federated search across documents, ZIM archives, and Git templates in a single query.
- **OAuth 2.1 Authorization Server** -- PKCE, device authorization ([RFC 8628](https://datatracker.ietf.org/doc/html/rfc8628)), dynamic client registration ([RFC 7591](https://datatracker.ietf.org/doc/html/rfc7591)), and [RFC 9728](https://datatracker.ietf.org/doc/html/rfc9728) Protected Resource Metadata for automatic discovery.
- **Document Pipeline** -- Upload PDF, DOCX, XLSX, HTML, EPUB, or Markdown. Text is extracted, indexed via PostgreSQL full-text search, and searchable within seconds.
- **External Integrations** -- Kiwix ZIM archives (federated article search) and Git template repositories.
- **Background Jobs** -- [River](https://riverqueue.com/) Postgres-native job queue with 7 worker types, 3 priority queues, and 6 periodic schedules.
- **Admin UI** -- Vue 3 + TypeScript SPA for managing documents, users, OAuth clients, external services, and queue status.
- **Observability** -- OpenTelemetry tracing with automatic instrumentation for database queries (otelpgx), Redis commands (redisotel), and outbound HTTP (otelhttp). Prometheus metrics covering HTTP, database pool, Redis pool, search, queue, and OAuth token replay. Structured logging with `slog` (trace/span ID injection). Optional Sentry/GlitchTip error tracking. See [docs/OBSERVABILITY.md](docs/OBSERVABILITY.md) for architecture and configuration.
- **OIDC Authentication** -- User login via any OpenID Connect provider.

## Quick Start

Docker Compose is the fastest way to run DocuMCP. The stack includes the application, PostgreSQL 17, Redis 8, and Traefik v3.4.

1. Create a `.env` file:

```bash
cat > .env <<EOF
# Database
DB_DATABASE=documcp
DB_USERNAME=documcp
DB_PASSWORD=$(openssl rand -base64 32)

# Redis (see docs/REDIS.md for ACL requirements)
REDIS_ADDR=redis:6379

# Secrets
OAUTH_SESSION_SECRET=$(openssl rand -base64 32)
ENCRYPTION_KEY=$(openssl rand -base64 32)
INTERNAL_API_TOKEN=$(openssl rand -base64 32)
HKDF_SALT=$(openssl rand -base64 16)

# Public URL and TLS
APP_URL=https://documcp.example.com
TRAEFIK_HOST=documcp.example.com
ACME_EMAIL=admin@example.com
EOF
```

2. Start the stack:

```bash
docker compose up -d
```

Migrations run automatically on first start.

3. The application is available at `https://documcp.example.com` via Traefik (ports 80/443). The app listens on port 8080 internally but is not exposed to the host by default. To run without Traefik, remove the `traefik` service and add `ports: ["8080:8080"]` to the `app` service.

> **OIDC required for login:** The admin panel authenticates users via an OpenID Connect provider. Set the `OIDC_*` variables in your `.env` file -- see `.env.example` for the full list.

See [docs/OAUTH_CLIENT_GUIDE.md](docs/OAUTH_CLIENT_GUIDE.md) for connecting AI agents and CLI tools.

## Development

### Prerequisites

- Go 1.26.2
- Node.js 24 (frontend)
- PostgreSQL (with `pg_trgm` and `unaccent` extensions)
- Redis 8+ (distributed rate limiting and SSE events)

### Build and Run

```bash
go build -o bin/documcp ./cmd/documcp    # Build binary

# Cobra subcommands:
go run ./cmd/documcp serve --with-worker # HTTP server + queue workers (dev default)
go run ./cmd/documcp serve               # HTTP server only (River insert-only)
go run ./cmd/documcp worker              # Queue workers only + health endpoint (:9090)
go run ./cmd/documcp migrate             # Run database migrations and exit
go run ./cmd/documcp version             # Print version info
go run ./cmd/documcp health              # Check readiness (for Docker healthchecks)
```

### Test

```bash
go test ./...                        # All tests
go test -race ./...                  # With race detection
go test -cover ./...                 # With coverage
go test -tags integration ./...      # Integration tests (needs Docker)
```

### Code Quality

```bash
gofmt -w .                           # Format
goimports -w .                       # Fix imports
golangci-lint run                    # Lint (v2.11.4)
```

### Frontend

```bash
cd frontend
npm ci
npm run build              # vue-tsc + Vite build -> web/frontend/dist/
npm run dev                # Dev server with HMR
npm run test               # Vitest
npm run lint               # vue-tsc + ESLint
```

## Architecture

```
cmd/documcp/             Entry point (serve, worker, migrate, version, health)
internal/
  app/                   App lifecycle (Foundation + ServerApp + WorkerApp)
  archive/               Shared zip/tar.gz builder with path-traversal guard
  auth/oauth/            OAuth 2.1 server (PKCE, device flow, dynamic registration)
  auth/oidc/             OIDC client for user authentication
  client/kiwix/          ZIM archive reader (Kiwix)
  client/git/            Git template repository sync
  config/                Configuration loading (env, YAML)
  cron/                  Cron schedule definitions
  crypto/                AES-256-GCM encryption for secrets at rest
  database/              PostgreSQL connection and migrations (goose)
  dto/                   Data transfer objects
  extractor/             Text extraction (PDF, DOCX, XLSX, HTML, EPUB, Markdown)
  handler/api/           REST API handlers
  handler/mcp/           MCP tool and prompt handlers
  handler/oauth/         OAuth endpoint handlers
  model/                 Domain models
  observability/         Tracing, metrics, structured logging, Sentry
  queue/                 River job queue (workers, events, periodic jobs)
  repository/            Data access layer (pgx, handwritten SQL)
  search/                Full-text search (tsvector/tsquery + pg_trgm)
  security/              Path traversal and SSRF guards
  server/                HTTP server setup and routing (chi v5)
  service/               Business logic orchestration
  storage/               Blob storage abstraction (FSBlob + S3Blob via gocloud.dev)
  stringutil/            Shared string utilities
  testutil/              Test helpers and fixtures
frontend/                Vue 3 + TypeScript SPA source (admin panel)
web/frontend/            Embedded SPA (//go:embed dist/)
migrations/              SQL migration files (goose)
docs/contracts/          OpenAPI spec, MCP contract, database schema
```

The application uses a single Cobra binary with `serve`, `worker`, `migrate`, and `health` subcommands for independent scaling. A shared Foundation holds dependencies (database pool, Redis client, repositories, search). ServerApp handles HTTP; WorkerApp handles River queue processing. Redis provides distributed rate limiting across server instances and cross-instance SSE event delivery via Pub/Sub. Repositories use `pgxpool.Pool` directly, services accept repository interfaces, and handlers accept services. Background jobs run via River, a Postgres-native job queue.

## MCP Tools

| Tool | Description |
|------|-------------|
| `list_documents` | List documents with optional filters |
| `search_documents` | Full-text search across documents |
| `read_document` | Retrieve document content by UUID |
| `create_document` | Create a new document |
| `update_document` | Modify document metadata |
| `delete_document` | Remove a document |
| `unified_search` | Cross-source search (documents, ZIM, Git templates) |
| `list_zim_archives` | List available ZIM archives |
| `search_zim` | Search within a specific ZIM archive |
| `read_zim_article` | Retrieve a ZIM article |
| `list_git_templates` | List available Git templates |
| `search_git_templates` | Search across template READMEs |
| `get_template_structure` | View folder tree and variables |
| `get_template_file` | Retrieve a file with variable substitution |
| `get_deployment_guide` | Deployment instructions with essential files |
| `download_template` | Download template as base64-encoded archive |

ZIM and Git template tools are registered conditionally based on whether the corresponding external services are configured.

## Configuration

Every variable below is sourced from `.env.example`. "Required" means startup fails without it; production-only requirements are noted in the description. Defaults shown are what the application uses when the variable is unset.

### Application

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `APP_NAME` | No | `DocuMCP` | Display name used in logs and the admin UI |
| `APP_ENV` | No | `development` | Environment: `development`, `staging`, `production`, `testing` |
| `APP_DEBUG` | No | `false` | Enables verbose debug logging |
| `APP_URL` | No | `http://localhost` | Public application URL (also seeds the OAuth resource indicator allowlist) |
| `APP_TIMEZONE` | No | `UTC` | Server timezone |
| `INTERNAL_API_TOKEN` | Prod | -- | Bearer token guarding `/metrics` and `/health/ready`. Generate `openssl rand -hex 32` |
| `ENCRYPTION_KEY` | Prod | -- | 64-char hex (32 bytes) for AES-256-GCM encryption of stored Git tokens. Generate `openssl rand -hex 32` |

### Server

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SERVER_HOST` | No | `0.0.0.0` | Listen address |
| `SERVER_PORT` | No | `8080` | Listen port (becomes the HTTP→HTTPS redirect port when TLS is enabled) |
| `TRUSTED_PROXIES` | No | -- | CIDR ranges for trusted reverse proxies (comma-separated). Required for correct `X-Forwarded-For` handling |
| `SERVER_READ_TIMEOUT` | No | `30s` | Full request read deadline |
| `SERVER_WRITE_TIMEOUT` | No | `30s` | Response write deadline |
| `SERVER_IDLE_TIMEOUT` | No | `120s` | Keep-alive idle timeout |
| `SERVER_READ_HEADER_TIMEOUT` | No | `5s` | Header read deadline (slowloris guard) |
| `SERVER_SHUTDOWN_TIMEOUT` | No | `5s` | Graceful shutdown deadline |
| `SERVER_REQUEST_TIMEOUT` | No | `60s` | Per-request context timeout (excludes `/documcp` and SSE streams) |
| `SERVER_MAX_BODY_SIZE` | No | `1048576` | Max request body in bytes (1 MiB; uploads use a separate limit) |
| `SERVER_HSTS_MAX_AGE` | No | `63072000` | HSTS max-age in seconds (2 years; `0` disables) |
| `SERVER_SSE_HEARTBEAT_INTERVAL` | No | `15s` | Server-Sent Events keepalive interval |

### TLS (direct HTTPS, no reverse proxy)

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `TLS_ENABLED` | No | `false` | Terminate TLS directly in the Go process |
| `TLS_PORT` | No | `8443` | HTTPS listen port |
| `TLS_CERT_FILE` | No | -- | PEM certificate path (empty + TLS enabled = ephemeral self-signed) |
| `TLS_KEY_FILE` | No | -- | PEM private key path |

### Database (PostgreSQL)

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DB_HOST` | Yes | `127.0.0.1` | PostgreSQL host |
| `DB_PORT` | No | `5432` | PostgreSQL port |
| `DB_DATABASE` | Yes | `documcp` | Database name |
| `DB_USERNAME` | Yes | `documcp` | Database user |
| `DB_PASSWORD` | Prod | -- | Database password |
| `DB_SSLMODE` | No | `require` | SSL mode: `disable`, `allow`, `prefer`, `require`, `verify-ca`, `verify-full` |
| `DB_MAX_OPEN_CONNS` | No | `25` | Max pool size (raise to 40–50 for combined `serve --with-worker` mode) |
| `DB_MAX_IDLE_CONNS` | No | `5` | Max idle connections in the pool |
| `DB_MAX_LIFETIME` | No | `5m` | Max lifetime per connection |
| `DB_PGX_MIN_CONNS` | No | `5` | Minimum idle connections kept warm |
| `DB_PGX_MAX_CONN_LIFETIME` | No | `30m` | pgx-level max connection lifetime |
| `DB_PGX_MAX_CONN_IDLE_TIME` | No | `5m` | pgx-level max idle time before close |

### Redis

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `REDIS_ADDR` | Yes | `localhost:6379` | Redis address (`host:port`) |
| `REDIS_USERNAME` | No | -- | Redis 6+ ACL username (empty = default user) |
| `REDIS_PASSWORD` | No | -- | Redis password |
| `REDIS_DB` | No | `0` | Redis database number |
| `REDIS_POOL_SIZE` | No | `10` | Connection pool size (`0` = `10 * GOMAXPROCS`) |
| `REDIS_MIN_IDLE_CONNS` | No | `2` | Idle connections kept warm |
| `REDIS_CONN_MAX_IDLE_TIME` | No | `5m` | Max idle time before close |
| `REDIS_DIAL_TIMEOUT` | No | `5s` | Connection establishment timeout |
| `REDIS_READ_TIMEOUT` | No | `5s` | Socket read timeout |
| `REDIS_WRITE_TIMEOUT` | No | `5s` | Socket write timeout |
| `REDIS_MAX_RETRIES` | No | `3` | Max command retries (`0` disables) |
| `REDIS_MAX_ACTIVE_CONNS` | No | `0` | Max active connections (`0` = unlimited) |
| `REDIS_TLS_ENABLED` | No | `false` | TLS for Redis (required for cloud-managed Redis) |
| `REDIS_TLS_CA_FILE` | No | -- | Optional CA certificate (PEM); empty = system CA pool |

### OIDC Authentication

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `OIDC_PROVIDER_URL` | No | -- | OpenID Connect provider URL (enables auto-discovery) |
| `OIDC_CLIENT_ID` | No | -- | OIDC client ID |
| `OIDC_CLIENT_SECRET` | No | -- | OIDC client secret |
| `OIDC_REDIRECT_URI` | No | -- | OIDC callback URL |
| `OIDC_SCOPES` | No | `openid,profile,email` | Comma-separated requested scopes |
| `OIDC_ADMIN_GROUPS` | No | -- | Comma-separated group names that grant admin access |
| `OIDC_AUTHORIZATION_URL` | No | -- | Manual override (skips discovery) |
| `OIDC_TOKEN_URL` | No | -- | Manual override |
| `OIDC_USERINFO_URL` | No | -- | Manual override |
| `OIDC_JWKS_URL` | No | -- | Manual override |
| `OIDC_END_SESSION_URL` | No | -- | RP-Initiated Logout endpoint (auto-discovered when `OIDC_PROVIDER_URL` is set) |

### OAuth 2.1 Authorization Server

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `OAUTH_SESSION_SECRET` | Prod | -- | Session secret (min 32 bytes); derives CSRF and token HMAC keys via HKDF. Generate `openssl rand -base64 32` |
| `OAUTH_SESSION_SECRET_PREVIOUS` | No | -- | Previous session secret for key rotation |
| `OAUTH_SESSION_MAX_AGE` | No | `720h` | Session lifetime (30 days) |
| `HKDF_SALT` | Prod | `DocuMCP-go-v1` | Per-deployment salt — startup rejects the default in production |
| `OAUTH_AUTHORIZATION_CODE_LIFETIME` | No | `10m` | Authorization code TTL |
| `OAUTH_ACCESS_TOKEN_LIFETIME` | No | `1h` | Access token TTL |
| `OAUTH_REFRESH_TOKEN_LIFETIME` | No | `720h` | Refresh token TTL (30 days) |
| `OAUTH_DEVICE_CODE_LIFETIME` | No | `10m` | Device code TTL |
| `OAUTH_DEVICE_POLLING_INTERVAL` | No | `5s` | Minimum device polling interval |
| `OAUTH_REGISTRATION_ENABLED` | No | `true` | Enables RFC 7591 dynamic client registration |
| `OAUTH_REGISTRATION_REQUIRE_AUTH` | No | `true` | When `false`, anonymous registration is allowed but constrained (public clients, read-only scopes, no device_code; rate-limited) |
| `OAUTH_CLIENT_TOUCH_TIMEOUT` | No | `3s` | Timeout for fire-and-forget `last_used_at` updates |
| `OAUTH_ALLOWED_RESOURCES` | No | _derived_ | RFC 8707 resource indicator allowlist (comma-separated absolute URIs). Defaults to `[APP_URL, APP_URL+DOCUMCP_ENDPOINT]` |

### Storage

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `STORAGE_DRIVER` | No | `local` | Blob backend: `local` / `fs` (node-local), `s3` (any S3-compatible service) |
| `STORAGE_BASE_PATH` | No | `./storage` | Filesystem root — always required (workers stage git clones and extraction scratch here, even with `s3`) |
| `STORAGE_DOCUMENT_PATH` | No | `documents` | Subdirectory under `STORAGE_BASE_PATH` for the FSBlob document tree |
| `STORAGE_TEMP_PATH` | No | `tmp` | Subdirectory for transient worker scratch |
| `STORAGE_MAX_UPLOAD_SIZE` | No | `52428800` | Max upload file size in bytes (50 MiB) |
| `STORAGE_MAX_EXTRACTED_TEXT` | No | `52428800` | Max decompressed text per file in bytes (50 MiB) |
| `STORAGE_MAX_ZIP_FILES` | No | `100` | Max files in a DOCX/EPUB ZIP archive |
| `STORAGE_MAX_SHEETS` | No | `100` | Max sheets in an XLSX file |
| `STORAGE_S3_ENDPOINT` | No† | -- | S3 endpoint URL (empty = AWS default; required for R2, B2, Garage, SeaweedFS, etc.) |
| `STORAGE_S3_BUCKET` | No† | -- | Target bucket name |
| `STORAGE_S3_REGION` | No† | `us-east-1` | AWS region string (`us-east-1` is a safe placeholder for Garage/SeaweedFS) |
| `STORAGE_S3_ACCESS_KEY_ID` | No† | -- | Static access key |
| `STORAGE_S3_SECRET_ACCESS_KEY` | No† | -- | Static secret key |
| `STORAGE_S3_USE_PATH_STYLE` | No | `true` | Force path-style addressing; required for most self-hosted backends |
| `STORAGE_S3_FORCE_SSL` | No | `true` | Reject plaintext S3 endpoints at startup |

† Required when `STORAGE_DRIVER=s3`. The `s3` driver speaks the S3 API and works against AWS S3, Cloudflare R2, Backblaze B2, Wasabi, Garage, SeaweedFS, and any other S3-compatible service. Keys use the same `{file_type}/{uuid}.{ext}` layout as the filesystem driver, so switching backends requires no database migration.

### External Services: Kiwix

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `KIWIX_CACHE_TTL` | No | `1h` | TTL for the Kiwix client factory cache |
| `KIWIX_HTTP_TIMEOUT` | No | `10s` | Per-request HTTP timeout |
| `KIWIX_HEALTH_CHECK_TIMEOUT` | No | `5s` | Health probe timeout |
| `KIWIX_FEDERATED_SEARCH_TIMEOUT` | No | `3s` | Deadline for Kiwix fan-out during `unified_search` |
| `KIWIX_FEDERATED_MAX_ARCHIVES` | No | `10` | Max archives searched in parallel |
| `KIWIX_FEDERATED_PER_ARCHIVE_LIMIT` | No | `3` | Max results per archive |

### External Services: Git Templates

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `GIT_MAX_FILE_SIZE` | No | `1048576` | Max bytes per extracted file (1 MiB) |
| `GIT_MAX_TOTAL_SIZE` | No | `10485760` | Max bytes per template after extraction (10 MiB) |

### Security

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SSRF_DIALER_TIMEOUT` | No | `10s` | Timeout for the SSRF-guarded outbound HTTP dialer (Kiwix, Git, OIDC) |

### Queue Workers

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `QUEUE_HIGH_WORKERS` | No | `10` | River high-priority queue concurrency |
| `QUEUE_DEFAULT_WORKERS` | No | `5` | River default queue concurrency |
| `QUEUE_LOW_WORKERS` | No | `2` | River low-priority queue concurrency |
| `WORKER_HEALTH_PORT` | No | `9090` | Health endpoint port for worker-only mode |

### Lifecycle

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `APP_QUEUE_STOP_TIMEOUT` | No | `10s` | Deadline for River queue drain on shutdown |
| `APP_TRACER_STOP_TIMEOUT` | No | `5s` | Deadline for OpenTelemetry tracer flush on shutdown |

### MCP Server

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DOCUMCP_ENDPOINT` | No | `/documcp` | URL path the MCP server is mounted at |
| `DOCUMCP_NAME` | No | `DocuMCP` | Server name advertised in MCP `initialize` |
| `DOCUMCP_VERSION` | No | `dev` | Set automatically from the git tag via ldflags — do not override manually |

### Observability

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `OTEL_ENABLED` | No | `false` | Enable OpenTelemetry tracing |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | No | -- | OTLP HTTP exporter endpoint (e.g., `tempo:4318`) |
| `OTEL_SERVICE_NAME` | No | `documcp` | Service name in traces |
| `OTEL_INSECURE` | No | `false` | Use HTTP instead of HTTPS for the OTLP exporter |
| `OTEL_SAMPLE_RATE` | No | `1.0` | Trace sampling rate (0.0–1.0); ignores upstream sampling decisions |
| `OTEL_ENVIRONMENT` | No | -- | `deployment.environment` resource attribute |
| `SENTRY_DSN` | No | -- | Sentry/GlitchTip DSN for error tracking (empty = disabled) |
| `SENTRY_SAMPLE_RATE` | No | `1.0` | Error sample rate (0.0–1.0) |
| `VITE_SENTRY_DSN` | No | -- | Frontend Sentry DSN — read by Vite at `npm run build` time only |

### Scheduler (cron expressions, empty disables a job)

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SCHEDULER_ENABLED` | No | `false` | Master switch for periodic jobs |
| `SCHEDULER_KIWIX_SCHEDULE` | No | `0 */6 * * *` | Kiwix archive metadata refresh |
| `SCHEDULER_GIT_SCHEDULE` | No | `0 * * * *` | Git template repository sync |
| `SCHEDULER_OAUTH_CLEANUP_SCHEDULE` | No | `0 * * * *` | Expired OAuth token / scope-grant cleanup |
| `SCHEDULER_ORPHANED_FILES_SCHEDULE` | No | `0 2 * * *` | Orphan blob cleanup (no DB row) |
| `SCHEDULER_SEARCH_VERIFY_SCHEDULE` | No | `0 3 * * *` | Search-index integrity verification |
| `SCHEDULER_SOFT_DELETE_PURGE_SCHEDULE` | No | `0 4 * * *` | Permanent deletion of soft-deleted documents |
| `SCHEDULER_ZIM_CLEANUP_SCHEDULE` | No | `0 5 * * *` | Stale ZIM archive cache cleanup |
| `SCHEDULER_HEALTH_CHECK_SCHEDULE` | No | `*/15 * * * *` | External service health probing |

## Running multiple replicas

DocuMCP is designed to scale horizontally behind a load balancer. Two things have to be true:

1. **Shared storage** -- set `STORAGE_DRIVER=s3` and point it at any S3-compatible service. The filesystem driver is node-local and will not work with more than one replica.
2. **At least one worker replica** -- scheduled jobs (document extraction, soft-delete purge, orphan cleanup, OAuth token cleanup, external-service health checks, expired scope-grant cleanup) run via River's periodic-job enqueuer. River elects a single leader across the cluster via the `river_leader` table, and only the leader enqueues periodic jobs. If every replica runs in insert-only `serve` mode (without `--with-worker`), no leader is elected and scheduled jobs never fire. Run at least one `serve --with-worker` or `worker` replica.

The MCP endpoint (`/documcp`) runs in Streamable HTTP stateless mode: every request creates a temporary session that closes after the response, so any replica can serve any request without sticky affinity. `GET /documcp` returns `405 Method Not Allowed` -- we don't offer a standalone SSE stream because our tools are pure request/response (no sampling, elicitation, or server-initiated messages). The REST and admin surfaces are stateless against Postgres and also require no affinity.

Cross-replica cache invalidation is already handled: admin edits to Kiwix external services publish a message on a dedicated Redis pub/sub channel (`documcp:control:cache.kiwix.invalidate`) that all replicas subscribe to. Other caches are read-through against Postgres and don't need invalidation.

Per-replica health is reported at `/health/ready`, which checks Postgres and Redis.

## Documentation

| Document | Description |
|----------|-------------|
| [OAuth Client Guide](docs/OAUTH_CLIENT_GUIDE.md) | Connecting AI agents, CLI tools, and Claude.ai |
| [Observability](docs/OBSERVABILITY.md) | Tracing, metrics, logging, error tracking, Grafana dashboard |
| [Prometheus Metrics](docs/PROMETHEUS_METRICS.md) | Metric listing, PromQL examples, scrape configuration |
| [Redis](docs/REDIS.md) | ACL requirements, client architecture, troubleshooting |
| [OpenAPI Spec](docs/contracts/openapi.yaml) | REST API specification |
| [MCP Contract](docs/contracts/mcp-contract.json) | MCP tools and prompts schema |
| [OAuth Flows](docs/contracts/oauth-flows.md) | OAuth 2.1 flow diagrams |

## License

[MIT](LICENSE)
