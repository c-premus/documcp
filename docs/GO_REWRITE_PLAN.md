# DocuMCP Go Rewrite Plan

**Version**: 1.0
**Date**: 2026-02-24
**Status**: Planning / Evaluation

> **Note (March 2026):** Confluence integration was removed from the Go rewrite scope. References to `ConfluenceClient`, Confluence tools, and `confluence_spaces` throughout this document reflect the original PHP v1.17.3 feature set and are retained for historical context only.
**Companion Document**: [REQUIREMENTS_SPECIFICATION.md](./REQUIREMENTS_SPECIFICATION.md)

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Motivation & Trade-offs](#2-motivation--trade-offs)
3. [Architecture Design](#3-architecture-design)
4. [Library Selection](#4-library-selection)
5. [Subsystem Migration Map](#5-subsystem-migration-map)
6. [Database & Migration Strategy](#6-database--migration-strategy)
7. [Feature Parity Matrix](#7-feature-parity-matrix)
8. [Effort Estimation](#8-effort-estimation)
9. [Risk Analysis](#9-risk-analysis)
10. [Migration Strategy](#10-migration-strategy)
11. [What Gets Better](#11-what-gets-better)
12. [What Gets Harder](#12-what-gets-harder)
13. [Recommended Approach](#13-recommended-approach)
14. [Decision Checklist](#14-decision-checklist)

---

## 1. Executive Summary

This document evaluates rewriting DocuMCP from Laravel 12 (PHP 8.5) to Go. The current system is production-ready at v1.17.3 with 18 MCP tools, OAuth 2.1 server, OIDC authentication, 4 Meilisearch indexes, 3 external service integrations, and 97%+ test coverage.

**Key findings**:
- Go has mature libraries for every core requirement (MCP, OAuth, OIDC, Meilisearch, document extraction)
- The biggest effort is the OAuth 2.1 server (~4-5 weeks), followed by the admin web UI (~3-4 weeks)
- The admin UI requires the most significant architectural decision: server-rendered (templ/htmx) vs separate SPA
- Total estimated effort: **16-24 weeks** for a single experienced Go developer (feature parity)
- The Go version would have significantly better performance characteristics (memory, startup, concurrency)
- The MCP server itself is the simplest part (~1-2 weeks with the official Go SDK)

---

## 2. Motivation & Trade-offs

### 2.1 Reasons to Rewrite in Go

| Benefit | Detail |
|---------|--------|
| **Performance** | ~10-50x lower memory per request; no PHP worker pool overhead; native concurrency |
| **Single binary deployment** | No PHP runtime, no Composer, no RoadRunner. One statically-linked binary + config |
| **Container size** | ~20-30 MB (scratch/distroless) vs ~200 MB (PHP Alpine) |
| **Startup time** | Milliseconds vs seconds (Octane warm-up) |
| **Concurrency model** | Goroutines are first-class; no need for Octane/RoadRunner worker abstraction |
| **Type safety** | Compile-time guarantees; no runtime type errors; no PHPStan needed |
| **Operational simplicity** | No PHP-FPM, no Octane, no RoadRunner binary management, no extension management |
| **Long-running processes** | Native support for queue workers, health checkers, scheduled tasks in-process |
| **MCP ecosystem** | Official Go SDK co-developed with Google; Go is a first-class MCP citizen |

### 2.2 Reasons NOT to Rewrite

| Risk | Detail |
|------|--------|
| **Working system** | The current system is production-ready with 97%+ coverage |
| **Rewrite cost** | 16-24 weeks of development effort |
| **Feature regression risk** | Subtle behaviors may be lost in translation |
| **Laravel ecosystem loss** | Horizon, Pulse, Livewire, Scout, Sanctum — all gone |
| **Admin UI complexity** | Livewire provides reactive UI with minimal JS; Go has no equivalent |
| **OAuth complexity** | The custom OAuth 2.1 server is ~40 files; reimplementing is non-trivial |
| **Team familiarity** | Requires Go expertise for ongoing maintenance |
| **Testing overhead** | 14,924 assertions need to be re-expressed in Go test idioms |

### 2.3 Hybrid Alternative

Instead of a full rewrite, consider:
- Keep the Laravel admin panel as-is
- Rewrite only the MCP server + REST API in Go (the hot path)
- Share the same database (PostgreSQL + Meilisearch)
- Use the Go service for MCP/API traffic, Laravel for admin/OAuth

This is discussed further in [Section 13](#13-recommended-approach).

---

## 3. Architecture Design

### 3.1 Project Structure

```
documcp/
  cmd/
    server/              # Main HTTP server entry point
    worker/              # Background job worker
    cli/                 # Admin CLI commands
  internal/
    config/              # Configuration loading (env, YAML)
    server/              # HTTP server setup, middleware, routing
    handler/             # HTTP handlers (controllers)
      api/               # REST API handlers
      oauth/             # OAuth 2.1 handlers
      admin/             # Admin UI handlers
      mcp/               # MCP endpoint handler
    service/             # Business logic (orchestration)
    action/              # Single-responsibility actions
    repository/          # Database access layer
    model/               # Domain models (structs)
    dto/                 # Data transfer objects
    extractor/           # Document content extraction
      pdf/
      docx/
      xlsx/
      html/
      markdown/
    client/              # External service clients
      kiwix/
      confluence/
      git/
    search/              # Meilisearch integration
    auth/                # Authentication & authorization
      oauth/             # OAuth 2.1 server logic
      oidc/              # OIDC client logic
      middleware/         # Auth middleware
    observability/       # Tracing, metrics, logging
    queue/               # Background job processing
  web/                   # Static assets, templates
    templates/           # HTML templates (templ or html/template)
    static/              # CSS, JS
  migrations/            # SQL migration files
  docs/
  tests/
    integration/
    unit/
  go.mod
  go.sum
  Dockerfile
  Makefile
```

### 3.2 Pattern Mapping: Laravel -> Go

| Laravel Pattern | Go Equivalent |
|----------------|---------------|
| Service-Action Pattern | Interface-based services + action functions |
| DTOs (final readonly) | Plain structs (value semantics) |
| Eloquent ORM | sqlx + squirrel (query builder) or sqlc (generated) |
| FormRequest validation | ozzo-validation or go-playground/validator |
| Middleware stack | Standard `http.Handler` middleware chain |
| Route model binding | Custom middleware or handler-level lookup |
| Livewire components | templ + htmx (server-driven) or Vue SPA |
| Laravel Scout | Direct meilisearch-go client |
| Queue jobs | In-process goroutine worker pool or asynq (Redis) |
| Events/Broadcasting | In-process event bus + optional WebSocket |
| Policies | Middleware + service-level authorization functions |
| Service providers | `fx` (Uber) or manual dependency injection |
| Artisan commands | cobra CLI framework |
| Blade templates | templ (type-safe) or html/template |
| Config/env | viper or envconfig |

### 3.3 Dependency Injection

**Option A: Manual (Recommended for this project size)**
```go
type App struct {
    Config          *config.Config
    DB              *sqlx.DB
    Meilisearch     meilisearch.ServiceManager
    Redis           *redis.Client
    DocumentService *service.DocumentService
    OAuthService    *service.OAuthService
    // ...
}

func NewApp(cfg *config.Config) (*App, error) {
    db, err := sqlx.Connect("postgres", cfg.DatabaseURL)
    // ... wire everything together
}
```

**Option B: Uber fx (if project grows)**
```go
fx.New(
    fx.Provide(config.New),
    fx.Provide(database.New),
    fx.Provide(service.NewDocumentService),
    // ...
    fx.Invoke(server.Start),
)
```

### 3.4 HTTP Router

**Recommendation**: `net/http` (Go 1.22+ has built-in path parameters) or `chi` for middleware chaining.

```go
mux := chi.NewRouter()
mux.Use(middleware.RequestID)
mux.Use(middleware.SecurityHeaders)

// MCP endpoint
mux.Post("/documcp", mcpHandler.Handle)

// OAuth endpoints
mux.Route("/oauth", func(r chi.Router) {
    r.Post("/register", oauth.Register)
    r.Post("/token", oauth.Token)
    r.Post("/revoke", oauth.Revoke)
    // ...
})

// API endpoints
mux.Route("/api", func(r chi.Router) {
    r.Use(auth.BearerToken)
    r.Get("/documents", api.ListDocuments)
    // ...
})
```

---

## 4. Library Selection

### 4.1 Core Dependencies

| Component | Library | Rationale |
|-----------|---------|-----------|
| **MCP Server** | `modelcontextprotocol/go-sdk` v1.x | Official SDK, co-developed with Google. Supports tools, prompts, resources, HTTP transport, auth middleware. |
| **HTTP Router** | `go-chi/chi` v5 | Lightweight, stdlib-compatible, excellent middleware support. |
| **Database** | `jmoiron/sqlx` + `jackc/pgx` v5 | sqlx for named queries and struct scanning; pgx for PostgreSQL driver. |
| **Migrations** | `pressly/goose` v3 | SQL-based migrations, supports up/down, versioned. |
| **Validation** | `go-playground/validator` v10 | Struct tag validation, widely adopted. |
| **Configuration** | `spf13/viper` | Env vars, YAML, defaults. Mature ecosystem. |
| **CLI** | `spf13/cobra` | Standard Go CLI framework. |

### 4.2 Authentication & OAuth

| Component | Library | Rationale |
|-----------|---------|-----------|
| **OAuth 2.1 Server** | `ory/fosite` v0.47+ | Production-grade OAuth 2.0/OIDC server. PKCE support (S256 enforcement). Device code grant requires custom handler (fosite has extension points). |
| **OIDC Client** | `coreos/go-oidc` v3 | Standard OIDC client, auto-discovery, ID token verification. |
| **JWT** | `golang-jwt/jwt` v5 | JWT creation/validation, used by fosite internally. |
| **Session** | `gorilla/sessions` | Session management for admin UI. |
| **Password Hashing** | `golang.org/x/crypto/bcrypt` | Standard bcrypt. |
| **Token Hashing** | `crypto/sha256` (stdlib) | SHA-256 for machine-generated tokens. |

**Note on fosite**: fosite does not include RFC 8628 (Device Authorization Grant) out of the box. You would need to implement a custom `DeviceCodeHandler` using fosite's extension interface. This is the most significant OAuth gap. Alternatively, implement the device flow handler from scratch (~500 lines) since it's relatively stateless.

### 4.3 Search & Content

| Component | Library | Rationale |
|-----------|---------|-----------|
| **Meilisearch** | `meilisearch/meilisearch-go` v0.28+ | Official Go client. Full API coverage. |
| **PDF Extraction** | `gen2brain/go-fitz` (MuPDF) | Most capable Go PDF library. Extracts text, HTML. Requires CGO (MuPDF C library). |
| **PDF Alternative** | `ledongthuc/pdf` | Pure Go, simpler but less capable. |
| **PDF Alternative** | Shell out to `pdftotext`/`pdftohtml` | Same approach as current Laravel (poppler-utils). Avoids CGO. |
| **DOCX** | `unidoc/unioffice` | Pure Go, reads/writes DOCX. May require commercial license for some features. |
| **DOCX Alternative** | Custom XML parser | DOCX is just ZIP + XML. ~200 lines for basic text extraction. |
| **XLSX** | `qax-os/excelize` v2 | De facto standard for Go Excel. Pure Go, excellent API. |
| **HTML to Markdown** | `JohannesKaufmann/html-to-markdown` v2 | Well-maintained HTML-to-Markdown converter. |
| **HTML Sanitization** | `microcosm-cc/bluemonday` | Safe HTML sanitization. |

### 4.4 Observability

| Component | Library | Rationale |
|-----------|---------|-----------|
| **OpenTelemetry** | `go.opentelemetry.io/otel` v1.x | Official OTEL Go SDK. First-class support. |
| **OTLP Exporter** | `go.opentelemetry.io/otel/exporters/otlp/*` | OTLP gRPC and HTTP exporters. |
| **Prometheus** | `prometheus/client_golang` | Standard Prometheus metrics. Or use OTEL metrics bridge. |
| **Structured Logging** | `log/slog` (stdlib, Go 1.21+) | Standard library structured logging. OTEL bridge available. |

### 4.5 Background Processing

| Component | Library | Rationale |
|-----------|---------|-----------|
| **Job Queue** | `hibiken/asynq` | Redis-backed task queue, similar to Horizon. Dashboard included. Retry, scheduling, priority queues. |
| **Alternative** | In-process goroutine pool | Simpler; no Redis dependency for queue. Use channels + worker goroutines. |
| **Scheduling** | `robfig/cron` v3 | Cron-like scheduler for periodic tasks. |

### 4.6 Admin UI

| Approach | Libraries | Trade-off |
|----------|-----------|-----------|
| **Server-rendered (Recommended)** | `a-h/templ` + htmx + Tailwind CSS | Type-safe templates, minimal JS, similar UX to Livewire. No separate build step. |
| **SPA** | Vue 3 + Vite (separate repo) | Best UX, but doubles the codebase. Same frontend as you'd use with any backend. |
| **Minimal** | `html/template` + htmx | No extra dependencies, but less type-safe. |

**Recommendation**: `templ` + htmx. This is the closest Go equivalent to Livewire's server-driven reactivity pattern. htmx handles partial page updates (search, filters, modals) via HTML-over-the-wire, similar to how Livewire works.

### 4.7 Git Operations

| Component | Library | Rationale |
|-----------|---------|-----------|
| **Git clone/pull** | `os/exec` (shell out to `git`) | Same approach as Laravel. Git CLI is reliable and handles all edge cases. |
| **Alternative** | `go-git/go-git` v5 | Pure Go git implementation. Avoids shell-out but heavier dependency. May not support all auth methods. |

**Recommendation**: Shell out to `git` (same as current). The `GIT_ASKPASS` credential pattern works identically.

---

## 5. Subsystem Migration Map

### 5.1 MCP Server

**Current**: Laravel MCP SDK with `DocumentationServer`, 18 tool classes, 7 prompt classes.
**Go**: Official `modelcontextprotocol/go-sdk` with `mcp.NewServer()`.

```go
server := mcp.NewServer(&mcp.Implementation{
    Name:    "DocuMCP",
    Version: "2.0.0",
}, nil)

// Register tools
mcp.AddTool(server, &mcp.Tool{
    Name:        "search_documents",
    Description: "Full-text search across documents",
    InputSchema: searchDocumentsSchema(),
}, searchDocumentsHandler)

// Register prompts
server.AddPrompt(&mcp.Prompt{
    Name:        "document_analysis",
    Description: "Analyze documents",
    Arguments:   documentAnalysisArgs(),
}, documentAnalysisHandler)

// HTTP handler with auth middleware
handler := mcp.NewStreamableHTTPHandler(
    func(r *http.Request) *mcp.Server { return server },
    nil,
)
mux.Handle("/documcp", authMiddleware(handler))
```

**Migration complexity**: LOW. The official Go SDK has nearly identical concepts. The 18 tool handlers translate 1:1 as Go functions. Conditional registration via the `shouldRegister` pattern becomes a runtime check before `mcp.AddTool()`.

### 5.2 OAuth 2.1 Server

**Current**: Custom implementation (40+ files): controller, 13 actions, 15 DTOs, 5 models, 8 form requests.
**Go**: `ory/fosite` provides the framework; custom storage and handlers needed.

**Key implementation tasks**:
1. Implement fosite's `Storage` interface for PostgreSQL (client, token, code, session storage)
2. Implement authorization code handler with PKCE enforcement
3. Implement refresh token handler
4. Implement custom Device Authorization Grant handler (not in fosite)
5. Implement RFC 7591 dynamic client registration (not in fosite)
6. Implement RFC 9728 Protected Resource Metadata endpoint
7. Implement RFC 8414 Authorization Server Metadata endpoint
8. Token hashing service (SHA-256 for tokens, bcrypt for secrets)
9. Consent screen with nonce protection

**Migration complexity**: HIGH. This is the most complex subsystem. fosite provides the OAuth framework but RFC 8628 device flow and RFC 7591 registration need custom implementation.

### 5.3 Document Processing Pipeline

**Current**: Upload -> Store -> ExtractContent (job) -> Index (job). 5 extractors, 3 queue jobs.
**Go**: Same pipeline, different libraries.

```go
// Extractor interface
type Extractor interface {
    Extract(ctx context.Context, filePath string) (*ExtractedContent, error)
    Supports(mimeType string) bool
}

// Implementations
type PDFExtractor struct{}       // Shell out to pdftohtml/pdftotext
type DOCXExtractor struct{}      // unidoc/unioffice or custom XML
type XLSXExtractor struct{}      // excelize
type HTMLExtractor struct{}      // html-to-markdown
type MarkdownExtractor struct{}  // pass-through
```

**Migration complexity**: MEDIUM. PDF extraction via shell-out is identical. DOCX/XLSX use different libraries but same logic. The queue jobs become asynq tasks or goroutine workers.

### 5.4 External Service Clients

**Current**: KiwixServeClient, ConfluenceClient, GitTemplateClient with caching.
**Go**: Standard `net/http` client with similar caching.

```go
type KiwixClient struct {
    httpClient *http.Client
    baseURL    string
    cache      *cache.Cache  // ristretto or bigcache
}
```

**Migration complexity**: MEDIUM. The HTTP client logic is straightforward. The Confluence HTML-to-Markdown converter and Kiwix OPDS XML parser need reimplementation. Git template SSRF protection and GIT_ASKPASS credential handling translate directly.

### 5.5 Admin Web UI

**Current**: 15 Livewire components with real-time search, modals, file uploads, sorting, pagination.
**Go**: templ + htmx + Tailwind.

```go
// templ component
templ DocumentList(docs []model.Document, filters Filters) {
    <div id="document-list">
        <input hx-get="/admin/documents" hx-trigger="keyup changed delay:300ms"
               hx-target="#document-list" name="search" value={filters.Search}/>
        for _, doc := range docs {
            @DocumentRow(doc)
        }
    </div>
}
```

**Migration complexity**: HIGH. This is the largest rewrite effort. Each Livewire component needs to become a templ+htmx equivalent. Features like file upload with progress, modals, real-time search, and toast notifications require careful htmx patterns. No automatic state management like Livewire provides.

### 5.6 Search System

**Current**: Laravel Scout + Meilisearch with federated multi-search.
**Go**: Direct `meilisearch-go` client.

```go
// Federated search
results, err := client.MultiSearch(&meilisearch.MultiSearchRequest{
    Federation: &meilisearch.MultiSearchFederation{Limit: limit, Offset: offset},
    Queries: []meilisearch.SearchRequest{
        {IndexUID: "documents", Query: query, Filter: docFilter},
        {IndexUID: "git_templates", Query: query, Filter: gitFilter},
        // ...
    },
})
```

**Migration complexity**: LOW. The meilisearch-go client has equivalent API. Index configuration is identical.

---

## 6. Database & Migration Strategy

### 6.1 Schema Compatibility

The existing PostgreSQL schema is language-agnostic. A Go rewrite can use the **same database** with zero schema changes.

**Migration tool**: `pressly/goose` for Go-managed migrations (SQL files, identical to current migrations).

### 6.2 ORM vs Query Builder

**Recommendation**: sqlx + handwritten SQL (not an ORM).

```go
// Model
type Document struct {
    ID          int64          `db:"id"`
    UUID        string         `db:"uuid"`
    Title       string         `db:"title"`
    Description sql.NullString `db:"description"`
    FileType    string         `db:"file_type"`
    UserID      sql.NullInt64  `db:"user_id"`
    IsPublic    bool           `db:"is_public"`
    Status      string         `db:"status"`
    CreatedAt   time.Time      `db:"created_at"`
    DeletedAt   sql.NullTime   `db:"deleted_at"`
}

// Repository
func (r *DocumentRepo) FindByUUID(ctx context.Context, uuid string) (*Document, error) {
    var doc Document
    err := r.db.GetContext(ctx, &doc,
        `SELECT * FROM documents WHERE uuid = $1 AND deleted_at IS NULL`, uuid)
    return &doc, err
}
```

**Why not GORM**: GORM adds magic and implicit behavior. sqlx is explicit, predictable, and faster. The existing SQL queries translate directly.

### 6.3 Soft Deletes

Implement as a repository-level concern:

```go
func (r *DocumentRepo) SoftDelete(ctx context.Context, id int64) error {
    _, err := r.db.ExecContext(ctx,
        `UPDATE documents SET deleted_at = NOW() WHERE id = $1`, id)
    return err
}

func (r *DocumentRepo) baseQuery() string {
    return `SELECT * FROM documents WHERE deleted_at IS NULL`
}
```

### 6.4 Zero-Downtime Transition

If migrating incrementally, both Laravel and Go can connect to the same PostgreSQL and Meilisearch instances simultaneously. No schema migration needed.

---

## 7. Feature Parity Matrix

| Feature | Laravel Status | Go Difficulty | Notes |
|---------|---------------|---------------|-------|
| MCP Server (18 tools) | Complete | Easy | Official Go SDK, 1:1 mapping |
| MCP Prompts (7) | Complete | Easy | Direct translation |
| OAuth 2.1 (auth code + PKCE) | Complete | Medium | fosite handles most of this |
| OAuth 2.1 (device flow) | Complete | Hard | Custom implementation needed |
| OAuth 2.1 (client registration) | Complete | Medium | Custom implementation needed |
| RFC 9728 PRM | Complete | Easy | Simple HTTP handler |
| RFC 8414 Metadata | Complete | Easy | Simple HTTP handler |
| OIDC Authentication | Complete | Easy | coreos/go-oidc |
| Document Upload & Storage | Complete | Easy | Standard file I/O |
| PDF Extraction | Complete | Easy | Shell out to poppler-utils (same) |
| DOCX Extraction | Complete | Medium | Different library, same concepts |
| XLSX Extraction | Complete | Easy | excelize is excellent |
| HTML Extraction | Complete | Easy | html-to-markdown library |
| Meilisearch Indexing | Complete | Easy | Official Go client |
| Federated Search | Complete | Easy | multiSearch API identical |
| ZIM/Kiwix Client | Complete | Medium | Rewrite HTTP client + OPDS parser |
| Confluence Client | Complete | Medium | Rewrite HTTP client + HTML converter |
| Git Template Client | Complete | Medium | Same git CLI approach |
| Admin Dashboard | Complete | Hard | templ+htmx rewrite of 15 components |
| Document List + Upload | Complete | Hard | File upload + progress + modals |
| User Management | Complete | Medium | CRUD with htmx |
| OAuth Client Management | Complete | Hard | Complex modals, one-time secret display |
| Service Management | Complete | Medium | CRUD + health checks |
| ZIM/Confluence/Git Managers | Complete | Hard | Multiple complex admin pages |
| Background Jobs | Complete | Easy | asynq or goroutine workers |
| Scheduled Tasks | Complete | Easy | robfig/cron |
| OpenTelemetry | Complete | Easy | Go OTEL SDK is excellent |
| Prometheus Metrics | Complete | Easy | prometheus/client_golang |
| Health Checks | Complete | Easy | Simple HTTP handlers |
| Security Headers | Complete | Easy | Middleware |
| Rate Limiting | Complete | Easy | Chi middleware or tollbooth |
| WebSocket Events | Complete | Medium | gorilla/websocket or nhooyr/websocket |
| Test Coverage (97%) | Complete | Hard | Re-express all 14,924 assertions |

---

## 8. Effort Estimation

### 8.1 By Subsystem

| Subsystem | Estimated Weeks | Confidence |
|-----------|----------------|------------|
| Project scaffolding, config, DB, DI | 1 | High |
| MCP Server (18 tools, 7 prompts) | 1.5-2 | High |
| OAuth 2.1 Server (all RFCs) | 4-5 | Medium |
| OIDC Authentication | 0.5-1 | High |
| Document Processing Pipeline | 1.5-2 | High |
| REST API (all endpoints) | 1.5-2 | High |
| Meilisearch Integration | 1 | High |
| External Service Clients (ZIM/Confluence/Git) | 2-3 | Medium |
| Admin Web UI (templ+htmx) | 3-4 | Low |
| Background Jobs + Scheduling | 0.5-1 | High |
| Observability (OTEL + Prometheus + Logging) | 1 | High |
| Security (headers, SSRF, path traversal) | 0.5 | High |
| Testing | 3-4 | Medium |
| Documentation + Deployment | 1 | High |
| **Total** | **21-30** | |
| **Realistic with buffer** | **24-32** | |

### 8.2 Critical Path

```
Week 1-2:   Scaffolding + Database + Config + Auth middleware
Week 3-6:   OAuth 2.1 Server (longest single item)
Week 3-4:   MCP Server (parallel with OAuth)
Week 5-6:   Document Pipeline + Search (parallel with OAuth)
Week 7-8:   External Service Clients
Week 9-10:  REST API endpoints
Week 11-14: Admin Web UI
Week 15-16: Observability + Security hardening
Week 17-20: Testing + Integration testing
Week 21-22: Documentation + Deployment + Smoke testing
```

### 8.3 Lines of Code Estimate

| Current (PHP) | Estimated (Go) | Ratio |
|---------------|----------------|-------|
| ~25,000 LOC (app/) | ~18,000-22,000 LOC | 0.7-0.9x |
| ~15,000 LOC (tests/) | ~12,000-16,000 LOC | 0.8-1.1x |

Go code tends to be slightly more verbose for error handling but has no annotations, no docblocks, and no framework boilerplate. Total LOC is typically similar or slightly less.

---

## 9. Risk Analysis

### 9.1 High Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| **OAuth 2.1 compliance gaps** | Auth failures for MCP clients | Extensive conformance testing against the current system. Test with Claude.ai, mcp-remote, and device flow clients. |
| **Admin UI feature regression** | Admins lose productivity | Build a feature checklist from current Livewire components. Acceptance test each page. |
| **CGO dependency (PDF)** | Build complexity, cross-compile issues | Use shell-out to poppler-utils (same as current) instead of go-fitz. Avoids CGO entirely. |
| **fosite Device Flow gap** | RFC 8628 not supported natively | Implement as custom handler. The device flow is relatively self-contained (~500 LOC). |

### 9.2 Medium Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| **Meilisearch Go client gaps** | Missing features vs PHP client | meilisearch-go is the official client; feature parity is strong. Test federated search early. |
| **templ+htmx learning curve** | Slower UI development | Prototype one complex page (DocumentList) early to validate the pattern. |
| **OIDC edge cases** | Auth failures with specific providers | coreos/go-oidc is battle-tested. Test against your specific OIDC provider early. |
| **Confluence HTML conversion** | Formatting differences | Port the regex-based converter directly. Test against sample pages. |

### 9.3 Low Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| **Database compatibility** | Schema differences | Same PostgreSQL, same schema. Zero risk. |
| **Meilisearch integration** | Index configuration | Same Meilisearch instance, same indexes. |
| **Git template operations** | Credential handling | Same git CLI, same GIT_ASKPASS pattern. |

---

## 10. Migration Strategy

### 10.1 Option A: Big Bang Rewrite

**Approach**: Rewrite everything in Go, deploy when complete.

**Pros**: Clean architecture, no hybrid complexity, single codebase.
**Cons**: Long time to production, high risk of regression, no incremental value.

**Timeline**: 24-32 weeks before first production deployment.

### 10.2 Option B: Strangler Fig (Recommended for risk-averse)

**Approach**: Incrementally migrate endpoints from Laravel to Go behind a reverse proxy.

**Phase 1** (Weeks 1-6): Go MCP server + auth middleware
- Deploy Go service alongside Laravel
- Traefik routes `/documcp` to Go, everything else to Laravel
- Both services share PostgreSQL + Meilisearch + Redis
- Validate MCP tools work identically

**Phase 2** (Weeks 7-12): Go REST API
- Migrate `/api/*` endpoints to Go
- Laravel continues serving admin panel + OAuth

**Phase 3** (Weeks 13-20): Go OAuth server
- Migrate OAuth endpoints to Go
- This is the hardest phase; requires careful token compatibility

**Phase 4** (Weeks 21-28): Go admin UI
- Migrate admin panel (or keep Laravel for admin)
- Decommission Laravel

**Pros**: Incremental value, lower risk, can validate each phase.
**Cons**: Temporary operational complexity (two services), shared state management.

### 10.3 Option C: Go API + Keep Laravel Admin (Pragmatic)

**Approach**: Rewrite MCP server + REST API in Go. Keep Laravel for admin UI and OAuth.

**Pros**: Gets the performance benefits where they matter (MCP/API hot path). Keeps the working admin UI.
**Cons**: Two codebases long-term. Divergent patterns.

**Timeline**: 12-16 weeks for Go MCP/API service.

---

## 11. What Gets Better

### 11.1 Performance

| Metric | Laravel + Octane | Go |
|--------|-----------------|-----|
| Memory per worker | ~50-80 MB | ~5-15 MB |
| Cold start | 2-5 seconds | 10-50 ms |
| Requests/second (search) | ~2,000 | ~10,000-20,000 |
| Token validation | ~1ms (SHA-256) | ~0.1ms |
| Container image | ~200 MB | ~20-30 MB |

### 11.2 Operational

- **Single binary**: No PHP, no Composer, no RoadRunner, no npm
- **Static linking**: Deploy to scratch/distroless containers
- **Native concurrency**: Goroutines for parallel search, concurrent health checks
- **Built-in HTTP server**: No need for Octane/RoadRunner abstraction
- **Cross-compilation**: Build for any OS/arch from any host

### 11.3 Development

- **Compile-time safety**: No PHPStan needed; the compiler catches type errors
- **Faster tests**: Go tests run 2-5x faster than PHPUnit/Pest
- **No framework version churn**: Go standard library is stable; no Laravel 12->13 upgrades
- **Better concurrency primitives**: Channels, goroutines, context cancellation

---

## 12. What Gets Harder

### 12.1 Admin UI

Livewire provides reactive server-driven components with near-zero JavaScript. The Go ecosystem has nothing equivalent. templ+htmx is the closest pattern but requires more manual work for:
- File upload with progress indicators
- Complex modal state management
- Real-time search with debouncing
- Toast notifications
- Multi-step forms (OAuth consent, device verification)

### 12.2 Database Interactions

Eloquent provides: eager loading, scopes, soft deletes, casts, events, model factories, and relationship traversal. In Go, all of these are explicit code. Expect 30-50% more code for database operations.

### 12.3 Validation

Laravel FormRequests provide declarative validation with custom messages, conditional rules, and authorization checks. Go validators are less expressive and require more boilerplate.

### 12.4 Testing

- No test factories (model factories) — need custom builder functions
- No automatic database transactions per test — need explicit setup/teardown
- No `RefreshDatabase` trait — need migration management in test setup
- No HTTP test helpers (`$this->getJson()`) — use `httptest.NewRecorder()`

### 12.5 Ecosystem

- No Horizon equivalent (asynq has a basic dashboard)
- No Pulse equivalent (custom or external monitoring)
- No Scout abstraction (direct Meilisearch client)
- No Sanctum (implement token auth directly)

---

## 13. Recommended Approach

Based on the analysis, here are three viable paths ranked by pragmatism:

### Path 1: Full Go Rewrite (Best long-term, highest short-term cost)

**When to choose**: You want maximum performance, operational simplicity, and are willing to invest 24-32 weeks. You have Go experience or are committed to building it.

**Priority order**: OAuth -> MCP -> API -> Pipeline -> Clients -> Admin -> Tests

### Path 2: Go API + Laravel Admin (Best balance)

**When to choose**: You want Go performance for the hot path (MCP/API) but don't want to rewrite the admin UI. Good if the admin panel rarely changes.

**Split**:
- **Go service**: MCP endpoint, REST API, background workers, health checks, metrics
- **Laravel service**: Admin panel, OAuth server, OIDC login
- **Shared**: PostgreSQL, Meilisearch, Redis

**Timeline**: 12-16 weeks for Go service.

### Path 3: Stay on Laravel (Lowest risk)

**When to choose**: The current system meets all performance requirements, and the rewrite effort doesn't justify the benefits.

**Enhance instead**: Upgrade to PHP 8.5 JIT, optimize Octane workers, add Redis caching to hot paths.

### Decision Matrix

| Factor | Full Go | Go API + Laravel | Stay Laravel |
|--------|---------|-----------------|-------------|
| Performance gain | Highest | High (hot path) | None |
| Effort | 24-32 weeks | 12-16 weeks | 0 |
| Risk | Medium-High | Low-Medium | None |
| Operational simplicity | Best | Moderate (2 services) | Current |
| Long-term maintenance | Best | Mixed | Current |
| Admin UI quality | Rebuild effort | Keep working UI | Keep working UI |

---

## 14. Decision Checklist

Before proceeding, answer these questions:

- [ ] **Performance need**: Is the current system hitting performance limits? (If not, Path 3 may suffice)
- [ ] **Deployment simplicity**: Is container size / startup time a meaningful concern?
- [ ] **Go expertise**: Do you have Go experience, or is this also a learning investment?
- [ ] **Admin UI frequency**: How often does the admin UI change? (Rarely = keep Laravel)
- [ ] **OAuth complexity**: Are you comfortable reimplementing OAuth 2.1 with device flow?
- [ ] **Timeline**: Can you afford 24-32 weeks without new features?
- [ ] **Testing commitment**: Are you prepared to re-express 14,924 test assertions?
- [ ] **MCP SDK maturity**: Has the official Go MCP SDK reached v1.x stable? (Yes, as of Feb 2026)

---

## Appendix A: Key File Counts (Current Laravel)

| Category | Files | Go Estimate |
|----------|-------|-------------|
| MCP Tools | 18 | 18 handler functions |
| MCP Prompts | 7 | 7 handler functions |
| Actions | ~50 | ~50 functions or methods |
| DTOs | ~45 | ~45 structs |
| Controllers | ~15 | ~15 handler files |
| Middleware | 8 | 8 middleware functions |
| Models | ~15 | ~15 model structs |
| Migrations | ~33 | Reuse SQL files |
| FormRequests | ~25 | ~25 validation structs |
| Livewire Components | 15 | 15 templ+htmx pages |
| Jobs | 3 | 3 worker functions |
| Commands | 15 | 15 cobra commands |
| Tests | ~100 files | ~80-100 test files |

## Appendix B: Go Module Dependencies (Estimated go.mod)

```
module github.com/yourorg/documcp

go 1.23

require (
    // MCP
    github.com/modelcontextprotocol/go-sdk v1.2.0

    // HTTP
    github.com/go-chi/chi/v5 v5.x.x

    // Database
    github.com/jmoiron/sqlx v1.x.x
    github.com/jackc/pgx/v5 v5.x.x
    github.com/pressly/goose/v3 v3.x.x

    // Auth
    github.com/ory/fosite v0.47.x
    github.com/coreos/go-oidc/v3 v3.x.x
    github.com/golang-jwt/jwt/v5 v5.x.x
    github.com/gorilla/sessions v1.x.x

    // Search
    github.com/meilisearch/meilisearch-go v0.28.x

    // Document Processing
    github.com/qax-os/excelize/v2 v2.x.x
    github.com/JohannesKaufmann/html-to-markdown/v2 v2.x.x
    github.com/microcosm-cc/bluemonday v1.x.x

    // Observability
    go.opentelemetry.io/otel v1.x.x
    go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.x.x
    github.com/prometheus/client_golang v1.x.x

    // Background Processing
    github.com/hibiken/asynq v0.x.x
    github.com/robfig/cron/v3 v3.x.x

    // UI
    github.com/a-h/templ v0.x.x

    // CLI & Config
    github.com/spf13/cobra v1.x.x
    github.com/spf13/viper v1.x.x

    // Validation
    github.com/go-playground/validator/v10 v10.x.x

    // Cache
    github.com/dgraph-io/ristretto v0.x.x

    // Redis
    github.com/redis/go-redis/v9 v9.x.x
)
```

---

*End of Go Rewrite Plan*
