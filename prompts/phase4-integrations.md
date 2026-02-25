# Phase 4: External Service Clients + REST API

You are implementing DocuMCP-go. Run `/memory-bank` first to load project context.

## Your Task

Implement the three external service clients (ZIM/Kiwix, Confluence, Git templates) and the remaining REST API endpoints.

## Reference Documents

- `docs/REQUIREMENTS_SPECIFICATION.md` — external service requirements
- `docs/contracts/openapi.yaml` — all REST API endpoints
- `docs/contracts/database-schema.sql` — external_services, zim_archives, confluence_spaces, git_templates tables

## Steps

### 1. ZIM/Kiwix Client — `internal/client/kiwix/`

HTTP client for Kiwix Serve instances:
- Parse OPDS catalog feed (XML) to list archives
- Search within archives (fulltext + suggest)
- Read article content (HTML -> plain text conversion)
- Health check endpoint
- Tiered caching (in-memory with TTL)
- Sync archives to local DB + Meilisearch

### 2. Confluence Client — `internal/client/confluence/`

HTTP client for Confluence REST API:
- List spaces (with type/query filters)
- Search pages (CQL or simple query)
- Read page content (convert Confluence HTML to markdown)
- Health check
- Caching
- Sync spaces to local DB + Meilisearch

### 3. Git Template Client — `internal/client/git/`

Git operations for template repositories:
- Clone/pull repositories (shell out to `git` with `GIT_ASKPASS` for credentials)
- Parse template structure (file tree, manifest, variables)
- Read individual files with variable substitution
- Generate deployment guides
- Create downloadable archives (zip/tar.gz)
- SSRF protection on repository URLs
- Sync to local DB + Meilisearch

### 4. External Service Management — `internal/service/external_service_service.go`

- CRUD for external services
- Health check orchestration (periodic background checks)
- Circuit breaker pattern (track consecutive_failures, error_count)
- Environment-managed services (is_env_managed flag)

### 5. REST API — `internal/handler/api/`

Implement remaining endpoints from `docs/contracts/openapi.yaml`:
- External services CRUD
- ZIM archive management
- Confluence space management
- Git template management
- User management (admin-only)
- OAuth client management (admin-only)

### 6. Wire and test

```bash
go build ./...
go test ./...
golangci-lint run
```

Commit after each major milestone using `/commit`.
