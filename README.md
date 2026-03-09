# DocuMCP

A documentation server that exposes knowledge bases through the [Model Context Protocol](https://modelcontextprotocol.io/) (MCP).

DocuMCP gives AI agents structured access to your documentation via MCP tools and prompts. It handles document ingestion, full-text search, OAuth 2.1 authorization, and integrates with external sources like Confluence, ZIM archives, and Git repositories. This is a Go rewrite of the original [PHP/Laravel version](https://github.com/chris/DocuMCP), targeting single-binary deployment with lower resource usage.

## Features

- **MCP Server** -- 18 tools and 7 prompts via the official Go MCP SDK. Agents can search, read, create, update, and delete documents.
- **OAuth 2.1 Authorization Server** -- PKCE, device authorization (RFC 8628), dynamic client registration (RFC 7591), and RFC 9728 Protected Resource Metadata for automatic discovery.
- **Document Pipeline** -- Upload PDF, DOCX, XLSX, HTML, or Markdown. Text is extracted, indexed in Meilisearch, and searchable within seconds.
- **Federated Search** -- Query across four Meilisearch indexes (documents, ZIM archives, Confluence spaces, Git templates) in a single request.
- **External Integrations** -- Sync content from Confluence, Kiwix ZIM archives, and Git template repositories.
- **Admin UI** -- Vue 3 SPA for managing documents, users, OAuth clients, and external services.
- **Observability** -- OpenTelemetry tracing, Prometheus metrics (9 collectors), and structured logging with `slog`.
- **OIDC Authentication** -- User login via any OpenID Connect provider.

## Quick Start

Docker Compose is the fastest way to run DocuMCP. The stack includes the application, PostgreSQL 17, Meilisearch v1.12, and Traefik v3.4.

1. Clone the repository and create a `.env` file:

```bash
git clone https://github.com/chris/DocuMCP-go.git
cd DocuMCP-go

cat > .env <<EOF
DB_DATABASE=documcp
DB_USERNAME=documcp
DB_PASSWORD=change-me-db-password
MEILI_MASTER_KEY=change-me-meili-key
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

## Development

### Prerequisites

- Go 1.25
- Node.js 22 (frontend build)
- PostgreSQL
- Meilisearch
- `poppler-utils` (PDF text extraction)

A devcontainer configuration is included for VS Code with all dependencies pre-installed.

### Build

```bash
go build ./...
```

### Test

```bash
go test ./...              # All tests
go test -race ./...        # With race detection
go test -cover ./...       # With coverage
```

### Code Quality

```bash
gofmt -w .                 # Format
goimports -w .             # Fix imports
golangci-lint run          # Lint (v2.9.0)
```

### Frontend

```bash
cd frontend
npm ci
npm run build              # Production build -> web/frontend/dist
npm run dev                # Dev server with HMR
```

## Architecture

```
cmd/server/              Entry point
internal/
  auth/oauth/            OAuth 2.1 server (PKCE, device flow, token management)
  auth/oidc/             OIDC client for user authentication
  client/confluence/     Confluence API client and sync
  client/kiwix/          ZIM archive reader and sync
  client/git/            Git template repository sync
  crypto/                AES-256-GCM encryption for secrets at rest
  database/              PostgreSQL connection and migrations (goose)
  dto/                   Data transfer objects
  extractor/             Text extraction (PDF, DOCX, XLSX, HTML, Markdown)
  handler/mcp/           MCP tool and prompt handlers
  handler/oauth/         OAuth endpoint handlers
  model/                 Domain models
  observability/         Tracing, metrics, structured logging
  repository/            Data access layer (sqlx, handwritten SQL)
  search/                Meilisearch client and indexer
  server/                HTTP server setup and routing (chi v5)
  service/               Business logic
  testutil/              Test helpers and builders
frontend/                Vue 3 + TypeScript SPA (admin panel)
migrations/              SQL migration files (goose)
docs/contracts/          OpenAPI spec, MCP contract, database schema
```

The application uses constructor injection throughout. Repositories accept `sqlx.DB`, services accept repository interfaces, and handlers accept services. Background jobs (document extraction, search indexing, periodic sync) run via River, a Postgres-native job queue.

## Configuration

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DB_HOST` | Yes | -- | PostgreSQL host |
| `DB_PORT` | No | `5432` | PostgreSQL port |
| `DB_DATABASE` | Yes | -- | Database name |
| `DB_USERNAME` | Yes | -- | Database user |
| `DB_PASSWORD` | Yes | -- | Database password |
| `DB_SSLMODE` | No | `require` | PostgreSQL SSL mode |
| `MEILISEARCH_HOST` | Yes | -- | Meilisearch URL |
| `MEILISEARCH_KEY` | Yes | -- | Meilisearch master key |
| `OIDC_PROVIDER_URL` | No | -- | OpenID Connect provider URL |
| `OIDC_CLIENT_ID` | No | -- | OIDC client ID |
| `OIDC_CLIENT_SECRET` | No | -- | OIDC client secret |
| `OIDC_REDIRECT_URI` | No | -- | OIDC callback URL |
| `OAUTH_SESSION_SECRET` | Yes | -- | Session secret (min 32 bytes); also derives CSRF and token HMAC keys via HKDF |
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

For a complete configuration reference, see `docs/contracts/configuration-schema.yaml`.

## License

TBD
