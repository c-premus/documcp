# DocuMCP

[![CI](https://github.com/c-premus/documcp/actions/workflows/ci.yaml/badge.svg)](https://github.com/c-premus/documcp/actions/workflows/ci.yaml)
[![Release](https://img.shields.io/github/v/tag/c-premus/documcp?label=release)](https://github.com/c-premus/documcp/tags)
[![Image Size](https://img.shields.io/docker/image-size/cpremus/documcp?sort=semver&label=image%20size)](https://hub.docker.com/r/cpremus/documcp)
[![Go Version](https://img.shields.io/github/go-mod/go-version/c-premus/documcp)](https://go.dev/)
[![License](https://img.shields.io/github/license/c-premus/documcp)](LICENSE)

A documentation server that exposes knowledge bases through the [Model Context Protocol](https://modelcontextprotocol.io/) (MCP), enabling AI agents to search, read, and manage documentation.

DocuMCP gives AI agents structured access to your documentation via MCP tools and prompts. It handles document ingestion, full-text search, and OAuth 2.1 authorization. Written in Go and shipped as a distroless container (~45 MB).

## Features

- **MCP Server** -- 17 tools and 6 prompts via the official [Go MCP SDK](https://github.com/modelcontextprotocol/go-sdk), stateless Streamable HTTP. Search, read, create, update, replace, and delete documents. Federated search across documents, ZIM archives, and Git templates.
- **OAuth 2.1 Authorization Server** -- PKCE, device authorization ([RFC 8628](https://datatracker.ietf.org/doc/html/rfc8628)), dynamic client registration ([RFC 7591](https://datatracker.ietf.org/doc/html/rfc7591)), and [RFC 9728](https://datatracker.ietf.org/doc/html/rfc9728) Protected Resource Metadata for automatic discovery.
- **Document Pipeline** -- Upload PDF, DOCX, XLSX, HTML, EPUB, or Markdown. Text is extracted, indexed via PostgreSQL full-text search, and searchable within seconds.
- **External Integrations** -- Kiwix ZIM archives (federated article search) and Git template repositories.
- **Background Jobs** -- [River](https://riverqueue.com/) Postgres-native job queue with 8 worker types, 3 queues (`high`, `default`, `low`), and 7 periodic schedules (enabled with `SCHEDULER_ENABLED=true`).
- **Admin UI** -- Vue 3 + TypeScript SPA for managing documents, users, OAuth clients, external services, and queue status.
- **Observability** -- OpenTelemetry tracing with automatic instrumentation for database queries (otelpgx), Redis commands (redisotel), and outbound HTTP (otelhttp). Prometheus metrics covering HTTP, database pool, Redis pool, search, queue, and OAuth token replay. Structured logging with `slog` (trace/span ID injection). Optional Sentry/GlitchTip error tracking. See [docs/OBSERVABILITY.md](docs/OBSERVABILITY.md) for architecture and configuration.
- **OIDC Authentication** -- User login via any OpenID Connect provider.

## Quick Start

Docker Compose is the fastest way to run DocuMCP. The stack includes the application, PostgreSQL 18, Redis 8, and Traefik v3.7.

1. Create a `.env` file:

```bash
cat > .env <<EOF
# Database
DB_DATABASE=documcp
DB_USERNAME=documcp
DB_PASSWORD=$(openssl rand -hex 24)

# Redis
REDIS_PASSWORD=$(openssl rand -hex 24)

# Secrets
OAUTH_SESSION_SECRET=$(openssl rand -base64 32)
ENCRYPTION_KEY=$(openssl rand -hex 32)
INTERNAL_API_TOKEN=$(openssl rand -hex 32)
HKDF_SALT=$(openssl rand -base64 24)

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

Migrations run automatically on first start. The compose file passes the whole `.env` to the app, so any variable from `.env.example` can be added there. It also turns on the job scheduler (`SCHEDULER_ENABLED=true`) and connects to the bundled Postgres without TLS (`DB_SSLMODE=disable`). Set `DB_SSLMODE=require` if you point `DB_HOST` at an external database.

3. The application is available at `https://documcp.example.com` via Traefik (ports 80/443), and directly on port 8080 of the host. To run without Traefik, remove the `traefik` service; to keep Traefik as the only entry point, remove the `ports` mapping from the `app` service.

> **OIDC required for login:** The admin panel authenticates users via an OpenID Connect provider. Set `OIDC_PROVIDER_URL`, `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET`, `OIDC_REDIRECT_URI`, and one of `OIDC_ADMIN_GROUPS` or `OIDC_BOOTSTRAP_ADMIN_EMAIL` in your `.env` file. See `.env.example` for the full list.

See [docs/OAUTH_CLIENT_GUIDE.md](docs/OAUTH_CLIENT_GUIDE.md) for connecting AI agents and CLI tools. MCP clients that don't yet implement RFC 8707 resource indicators (e.g. Open WebUI) can be supported via the opt-in `OAUTH_ACCEPT_EMPTY_RESOURCE` flag — see [Accepting non-RFC-8707 clients](docs/CONFIGURATION.md#accepting-non-rfc-8707-clients-oauth_accept_empty_resource).

## Development

### Prerequisites

- Go 1.27.1 (the `toolchain` directive in `go.mod`)
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
go run ./cmd/documcp rekey               # Re-encrypt secrets after rotating ENCRYPTION_KEY
go run ./cmd/documcp version             # Print version info
go run ./cmd/documcp health              # Check readiness (for Docker healthchecks)
```

### Test

```bash
go test ./...                        # Unit tests
go test -race ./...                  # With race detection
go test -cover ./...                 # With coverage
go test -tags integration ./...      # Unit + integration tests (needs Docker for testcontainers)
```

### Code Quality

```bash
gofmt -w .                           # Format
goimports -w .                       # Fix imports
golangci-lint run                    # Lint (v2.14.0)
```

### Frontend

```bash
cd frontend
npm ci
npm run build              # vue-tsc + Vite build -> web/frontend/dist/
npm run dev                # Dev server with HMR
npm run test               # Vitest
npm run test:coverage      # Vitest with coverage
npm run lint               # vue-tsc -b + ESLint
npm run format:check       # Prettier
```

## Architecture

```
cmd/documcp/             Entry point (serve, worker, migrate, rekey, version, health)
internal/
  app/                   App lifecycle (Foundation + ServerApp + WorkerApp)
  archive/               Shared zip/tar.gz builder with path-traversal guard
  auth/middleware/       Bearer-token, scope, and RFC 8707 audience middleware
  auth/oauth/            OAuth 2.1 server (PKCE, device flow, dynamic registration)
  auth/oidc/             OIDC client for user authentication
  auth/scope/            Scope registry and consent ceilings
  auth/session/          Redis-backed browser sessions
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
  storage/               Blob storage abstraction (local filesystem + S3 via aws-sdk-go-v2)
  stringutil/            Shared string utilities
  testutil/              Test helpers and fixtures
frontend/                Vue 3 + TypeScript SPA source (admin panel)
web/frontend/            Embedded SPA (//go:embed dist/)
migrations/              SQL migration files (goose)
docs/contracts/          OpenAPI spec, MCP contract, OAuth flow reference
```

The application uses a single Cobra binary. `serve` and `worker` scale independently; `migrate`, `rekey`, `version`, and `health` are one-shot commands. A shared Foundation holds dependencies (database pool, Redis client, repositories, search). ServerApp handles HTTP; WorkerApp handles River queue processing. Redis provides distributed rate limiting across server instances and cross-instance SSE event delivery via Pub/Sub. Repositories use `pgxpool.Pool` directly, services accept repository interfaces, and handlers accept services. Background jobs run via River, a Postgres-native job queue.

## MCP Tools

| Tool | Description |
|------|-------------|
| `list_documents` | List documents with optional filters |
| `search_documents` | Full-text search across documents |
| `read_document` | Retrieve document content by UUID |
| `create_document` | Create a new document |
| `update_document` | Modify document metadata |
| `replace_document_content` | Replace the body of an inline Markdown or HTML document |
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

Read tools need the `mcp:read` scope and write tools need `mcp:write`, on top of `mcp:access` for the endpoint itself. ZIM and Git template tools are always registered; they return empty results until a Kiwix server or Git template repository is configured in the admin panel.

## Configuration

DocuMCP is configured through environment variables. `.env.example` is the authoritative list; [docs/CONFIGURATION.md](docs/CONFIGURATION.md) groups every variable by concern (Application / Server / TLS / Database / Redis / OIDC / OAuth / Storage / Kiwix / Git Templates / Security / Queue Workers / Lifecycle / MCP Server / Observability / Scheduler) with defaults and descriptions.

The minimum set needed to start:

| Variable | Why |
|----------|-----|
| `APP_URL` | Public application URL. Seeds the RFC 8707 resource indicator allowlist and the OIDC `post_logout_redirect_uri`. |
| `DB_HOST`, `DB_DATABASE`, `DB_USERNAME`, `DB_PASSWORD` | PostgreSQL connection. |
| `REDIS_ADDR` | Distributed rate limiting and cross-replica SSE/control-bus. See [docs/REDIS.md](docs/REDIS.md) for ACL requirements. |
| `OAUTH_SESSION_SECRET`, `HKDF_SALT`, `ENCRYPTION_KEY`, `INTERNAL_API_TOKEN` | Per-deployment secrets. `ENCRYPTION_KEY` must be 64 hex characters (`openssl rand -hex 32`). `OAUTH_SESSION_SECRET` must be at least 32 characters in production (`openssl rand -base64 32`). `HKDF_SALT` is required in every environment and must be at least 16 characters (`openssl rand -base64 24`). `INTERNAL_API_TOKEN` protects `/metrics`. All four are required when `APP_ENV=production`. |
| `OIDC_PROVIDER_URL`, `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET`, `OIDC_REDIRECT_URI` | OpenID Connect provider — DocuMCP has no local login. |
| `OIDC_ADMIN_GROUPS` _or_ `OIDC_BOOTSTRAP_ADMIN_EMAIL` | At least one is required when OIDC is configured, so an admin can ever exist. `OIDC_ADMIN_GROUPS` is re-evaluated on every login; `OIDC_BOOTSTRAP_ADMIN_EMAIL` fires once, on user creation, for providers without group claims. |

## Running multiple replicas

DocuMCP is designed to scale horizontally behind a load balancer. Two things have to be true:

1. **Shared storage** -- set `STORAGE_DRIVER=s3` and point it at any S3-compatible service. The filesystem driver is node-local and will not work with more than one replica.
2. **At least one worker replica** -- uploaded documents are extracted by queue workers, and scheduled jobs (Kiwix and Git template sync, OAuth token and expired scope-grant cleanup, orphaned-file cleanup, soft-delete purge, external-service health checks, search-query retention) run via River's periodic-job enqueuer. River elects a single leader across the cluster via the `river_leader` table, and only the leader enqueues periodic jobs. If every replica runs in insert-only `serve` mode (without `--with-worker`), nothing is extracted and scheduled jobs never fire. Run at least one `serve --with-worker` or `worker` replica, and set `SCHEDULER_ENABLED=true` on it.

The MCP endpoint (`/documcp`) runs in Streamable HTTP stateless mode: every request creates a temporary session that closes after the response, so any replica can serve any request without sticky affinity. `GET /documcp` returns `405 Method Not Allowed` -- we don't offer a standalone SSE stream because our tools are pure request/response (no sampling, elicitation, or server-initiated messages). The REST and admin surfaces are stateless against Postgres and also require no affinity.

Cross-replica cache invalidation is already handled: admin edits to Kiwix external services publish a message on a dedicated Redis pub/sub channel (`documcp:control:cache.kiwix.invalidate`) that all replicas subscribe to. Other caches are read-through against Postgres and don't need invalidation.

Per-replica health is reported at `/health/ready`, which checks Postgres and Redis and needs no token (the container healthcheck calls it). Worker-only replicas serve the same check on `WORKER_HEALTH_PORT` (default 9090).

## Documentation

| Document | Description |
|----------|-------------|
| [Changelog](CHANGELOG.md) | Per-release changes (auto-generated from conventional-commit subjects) |
| [Contributing](CONTRIBUTING.md) | How to send PRs, commit message format, dual-repo flow |
| [Security](SECURITY.md) | Reporting vulnerabilities, security model |
| [Configuration](docs/CONFIGURATION.md) | Every environment variable, grouped by concern |
| [OAuth Client Guide](docs/OAUTH_CLIENT_GUIDE.md) | Connecting AI agents, CLI tools, and Claude.ai |
| [Operations](docs/OPERATIONS.md) | Backup / restore (Postgres + blob store), readiness monitoring, alert rules |
| [Observability](docs/OBSERVABILITY.md) | Tracing, metrics, logging, error tracking, Grafana dashboard |
| [Prometheus Metrics](docs/PROMETHEUS_METRICS.md) | Metric listing, PromQL examples, scrape configuration |
| [Redis](docs/REDIS.md) | ACL requirements, client architecture, troubleshooting |
| [Migrations](migrations/README.md) | Migration safety rules enforced in CI |
| [OpenAPI Spec](docs/contracts/openapi.yaml) | REST API specification |
| [MCP Contract](docs/contracts/mcp-contract.json) | MCP tools and prompts schema |
| [OAuth Flows](docs/contracts/oauth-flows.md) | OAuth 2.1 flow diagrams |

## License

[MIT](LICENSE)
