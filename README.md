# DocuMCP

A documentation server that exposes knowledge bases through the [Model Context Protocol](https://modelcontextprotocol.io/) (MCP), enabling AI agents to search, read, and manage documentation.

DocuMCP gives AI agents structured access to your documentation via MCP tools and prompts. It handles document ingestion, full-text search, and OAuth 2.1 authorization. Written in Go for single-binary deployment with low resource usage.

## Features

- **MCP Server** -- 15 tools and 6 prompts via the official [Go MCP SDK](https://github.com/modelcontextprotocol/go-sdk). Search, read, create, update, and delete documents. Federated search across documents, ZIM archives, and Git templates in a single query.
- **OAuth 2.1 Authorization Server** -- PKCE, device authorization ([RFC 8628](https://datatracker.ietf.org/doc/html/rfc8628)), dynamic client registration ([RFC 7591](https://datatracker.ietf.org/doc/html/rfc7591)), and [RFC 9728](https://datatracker.ietf.org/doc/html/rfc9728) Protected Resource Metadata for automatic discovery.
- **Document Pipeline** -- Upload PDF, DOCX, XLSX, HTML, or Markdown. Text is extracted, indexed via PostgreSQL full-text search, and searchable within seconds.
- **External Integrations** -- Kiwix ZIM archives (federated article search) and Git template repositories.
- **Background Jobs** -- [River](https://riverqueue.com/) Postgres-native job queue with 11 worker types, 3 priority queues, and 8 periodic schedules.
- **Admin UI** -- Vue 3 + TypeScript SPA for managing documents, users, OAuth clients, external services, and queue status.
- **Observability** -- OpenTelemetry tracing (OTLP), Prometheus metrics (15 collectors), and structured logging with `slog`.
- **OIDC Authentication** -- User login via any OpenID Connect provider.

## Quick Start

Docker Compose is the fastest way to run DocuMCP. The stack includes the application, PostgreSQL 17, and Traefik v3.4.

1. Create a `.env` file:

```bash
cat > .env <<EOF
DB_DATABASE=documcp
DB_USERNAME=documcp
DB_PASSWORD=change-me-db-password
OAUTH_SESSION_SECRET=change-me-session-secret-at-least-32-bytes
APP_URL=https://documcp.example.com
TRAEFIK_HOST=documcp.example.com
ACME_EMAIL=admin@example.com
EOF
```

2. Start the stack:

```bash
docker compose up -d
```

3. The application is available at `https://documcp.example.com` (or `http://localhost:8080` without Traefik).

See `docs/OAUTH_CLIENT_GUIDE.md` for connecting AI agents and CLI tools.

## Development

### Prerequisites

- Go 1.26.1
- Node.js 22 (frontend)
- PostgreSQL (with `pg_trgm` and `unaccent` extensions)
- `poppler-utils` (PDF text extraction)

A devcontainer configuration is included for VS Code with all dependencies pre-installed.

### Build and Run

```bash
go build -o bin/documcp ./cmd/documcp    # Build binary

# Cobra subcommands:
go run ./cmd/documcp serve --with-worker # HTTP server + queue workers (dev default)
go run ./cmd/documcp serve               # HTTP server only (River insert-only)
go run ./cmd/documcp worker              # Queue workers only + health endpoint (:9090)
go run ./cmd/documcp migrate             # Run database migrations and exit
go run ./cmd/documcp version             # Print version info
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
golangci-lint run                    # Lint (v2.11.3)
```

### Frontend

```bash
cd frontend
npm ci
npm run build              # OpenAPI codegen + vue-tsc + Vite build -> web/frontend/dist/
npm run dev                # Dev server with HMR
npm run test               # Vitest
npm run lint               # vue-tsc + ESLint
```

## Architecture

```
cmd/documcp/             Entry point (serve, worker, migrate, version)
internal/
  action/                Single-responsibility business actions
  auth/oauth/            OAuth 2.1 server (PKCE, device flow, dynamic registration)
  auth/oidc/             OIDC client for user authentication
  client/kiwix/          ZIM archive reader (Kiwix)
  client/git/            Git template repository sync
  crypto/                AES-256-GCM encryption for secrets at rest
  database/              PostgreSQL connection and migrations (goose)
  dto/                   Data transfer objects
  extractor/             Text extraction (PDF, DOCX, XLSX, HTML, Markdown)
  handler/api/           REST API handlers
  handler/mcp/           MCP tool and prompt handlers
  handler/oauth/         OAuth endpoint handlers
  model/                 Domain models
  observability/         Tracing, metrics, structured logging
  queue/                 River job queue (workers, events, periodic jobs)
  repository/            Data access layer (pgx, handwritten SQL)
  search/                Full-text search (tsvector/tsquery + pg_trgm)
  server/                HTTP server setup and routing (chi v5)
  service/               Business logic orchestration
  stringutil/            Shared string utilities
frontend/                Vue 3 + TypeScript SPA source (admin panel)
web/frontend/            Embedded SPA (//go:embed dist/)
migrations/              SQL migration files (goose)
docs/contracts/          OpenAPI spec, MCP contract, database schema
```

The application uses a single Cobra binary with `serve`, `worker`, and `migrate` subcommands for independent scaling. A shared Foundation holds dependencies (database pool, repositories, search). ServerApp handles HTTP; WorkerApp handles River queue processing. Repositories use `pgxpool.Pool` directly, services accept repository interfaces, and handlers accept services. Background jobs run via River, a Postgres-native job queue.

## MCP Tools

| Tool | Description |
|------|-------------|
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

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DB_HOST` | Yes | -- | PostgreSQL host |
| `DB_PORT` | No | `5432` | PostgreSQL port |
| `DB_DATABASE` | Yes | -- | Database name |
| `DB_USERNAME` | Yes | -- | Database user |
| `DB_PASSWORD` | Yes | -- | Database password |
| `DB_SSLMODE` | No | `require` | PostgreSQL SSL mode |
| `OIDC_PROVIDER_URL` | No | -- | OpenID Connect provider URL |
| `OIDC_CLIENT_ID` | No | -- | OIDC client ID |
| `OIDC_CLIENT_SECRET` | No | -- | OIDC client secret |
| `OIDC_REDIRECT_URI` | No | -- | OIDC callback URL |
| `OAUTH_SESSION_SECRET` | Yes | -- | Session secret (min 32 bytes); derives CSRF and token HMAC keys via HKDF |
| `ENCRYPTION_KEY` | No | -- | 32-byte key for AES-256-GCM encryption of stored Git tokens |
| `SERVER_HOST` | No | `0.0.0.0` | Listen address |
| `SERVER_PORT` | No | `8080` | Listen port |
| `STORAGE_DRIVER` | No | `local` | File storage driver |
| `STORAGE_BASE_PATH` | No | -- | Base path for local file storage |
| `OTEL_ENABLED` | No | `false` | Enable OpenTelemetry tracing |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | No | -- | OTLP exporter endpoint |
| `INTERNAL_API_TOKEN` | No | -- | Token for internal API endpoints |
| `APP_URL` | No | `http://localhost` | Public application URL |
| `TRUSTED_PROXIES` | No | -- | CIDR ranges for trusted reverse proxies |
| `KIWIX_FEDERATED_SEARCH_TIMEOUT` | No | `3s` | Deadline for Kiwix fan-out during unified search |
| `KIWIX_FEDERATED_MAX_ARCHIVES` | No | `10` | Max archives to search in parallel |
| `KIWIX_FEDERATED_PER_ARCHIVE_LIMIT` | No | `3` | Max results per archive |
| `QUEUE_HIGH_WORKERS` | No | `10` | River queue concurrency for high-priority jobs |
| `QUEUE_DEFAULT_WORKERS` | No | `5` | River queue concurrency for default jobs |
| `QUEUE_LOW_WORKERS` | No | `2` | River queue concurrency for low-priority jobs |
| `WORKER_HEALTH_PORT` | No | `9090` | Health endpoint port for worker-only mode |

See `.env.example` for all ~60 configurable variables with defaults.

## License

[MIT](LICENSE)
