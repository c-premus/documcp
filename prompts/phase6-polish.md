# Phase 6: Observability, Testing, CI/CD, Deployment

You are implementing DocuMCP-go. Run `/memory-bank` first to load project context.

## Your Task

Add observability, comprehensive tests, CI/CD pipeline, and deployment configuration.

## Steps

### 1. OpenTelemetry — `internal/observability/`

- Tracer provider with OTLP HTTP exporter
- Middleware to create spans for HTTP requests
- Span instrumentation in service layer
- slog bridge to attach trace IDs to log entries

### 2. Prometheus Metrics

- HTTP request duration histogram
- Request counter by method/path/status
- Active connections gauge
- Document count gauge
- Search latency histogram
- `/metrics` endpoint

### 3. Comprehensive Tests

Target 70%+ coverage:
- Unit tests for all services, actions, repositories (mock DB)
- Integration tests for database operations (use testcontainers or test DB)
- HTTP handler tests for all endpoints
- OAuth flow integration tests
- MCP tool handler tests
- Extractor tests with sample files

### 4. CI/CD — `.github/workflows/` or `.forgejo/workflows/`

- Lint (golangci-lint)
- Test (go test -race -cover)
- Build (go build)
- Docker image build
- Release workflow (semantic versioning)

### 5. Deployment

- `Dockerfile` — multi-stage build, distroless/scratch base
- `docker-compose.yml` — full stack (app, PostgreSQL, Meilisearch, Redis)
- Traefik configuration for reverse proxy
- Health check endpoints

### 6. Final verification

```bash
go build ./...
go test -race -cover ./...
golangci-lint run
docker build -t documcp .
```

Commit and create release using `/commit` and `/release-forgejo`.
