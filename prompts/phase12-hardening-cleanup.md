# Phase 12: Hardening & Cleanup

You are implementing DocuMCP-go. Run `/memory-bank` first to load project context.

**Depends on**: Phases 1-11 (all previous phases complete)

## Your Task

Remove dead code, fix security issues found during the Phase 11 assessment, and harden the application for production deployment. This phase has three sections: cleanup, security fixes, and hardening.

**Primary agent**: `go-writer`
**Secondary agents**: `security-reviewer`, `typescript-writer`

## Section A: Dead Code Removal

### 1. Remove `RollbackMigration` — `internal/database/migrate.go`

Delete the `RollbackMigration` function (exported but never called anywhere in the codebase). It wraps `goose.Down()` but nothing invokes it — not CLI, not tests, not handlers.

### 2. Remove `AccessTokenFromContext` — `internal/auth/middleware/middleware.go`

Delete the `AccessTokenFromContext` function (exported but never referenced). The codebase uses `UserFromContext` instead. Remove any associated context key if it becomes unused after deletion.

### 3. Remove Redis Configuration — `internal/config/config.go`

Redis is configured but never used. Remove:
- The `RedisConfig` struct definition
- The `Redis RedisConfig` field from `Config` struct
- Redis defaults in `setDefaults()` (host, port, password, db)
- Redis population in `Load()` (the block reading `REDIS_HOST`, `REDIS_PORT`, etc.)
- Any Redis-related environment variable documentation

**Verify**: Run `go build ./...` and `go test ./...` after each removal to confirm nothing breaks.

**Commit checkpoint**: `chore: remove dead code (RollbackMigration, AccessTokenFromContext, RedisConfig)`

---

## Section B: Security Fixes

### 4. SSRF Validation on External Service Create/Update — HIGH

**Problem**: `ExternalServiceService.Create()` and `ExternalServiceService.Update()` accept a `BaseURL` without SSRF validation. An attacker with API access can store a URL pointing to internal services (e.g., `http://169.254.169.254`), which health checks or sync operations will then request.

**File**: `internal/service/external_service_service.go`

**Fix**: Add `security.ValidateExternalURL(params.BaseURL)` at the top of both `Create()` and `Update()` methods, before any database operation:

```go
import "git.999.haus/chris/DocuMCP-go/internal/security"

func (s *ExternalServiceService) Create(ctx context.Context, params CreateExternalServiceParams) (*model.ExternalService, error) {
    if err := security.ValidateExternalURL(params.BaseURL); err != nil {
        return nil, fmt.Errorf("base URL validation: %w", err)
    }
    // ... existing code
}

func (s *ExternalServiceService) Update(ctx context.Context, uuid string, params UpdateExternalServiceParams) (*model.ExternalService, error) {
    if params.BaseURL != "" {
        if err := security.ValidateExternalURL(params.BaseURL); err != nil {
            return nil, fmt.Errorf("base URL validation: %w", err)
        }
    }
    // ... existing code
}
```

**Tests**: Add test cases in the service test file:
- Create with private IP URL returns error
- Create with valid public URL succeeds
- Update with private IP URL returns error
- Update with valid public URL succeeds
- Update with empty BaseURL skips validation (no change)

### 5. SSRF Validation on Git Template Create/Update — HIGH

**Problem**: `POST /api/git-templates` and `PUT /api/git-templates/{uuid}` accept a `repository_url` without server-side SSRF validation. The `POST /api/admin/git-templates/validate-url` endpoint exists but is only a client-side convenience — not enforced on create/update.

**File**: `internal/handler/api/git_template_handler.go`

**Fix**: In the `Create` and `Update` handler methods, call the git client's URL validation before persisting:

```go
import "git.999.haus/chris/DocuMCP-go/internal/security"

// In Create handler, after parsing request body:
if err := security.ValidateExternalURL(body.RepositoryURL); err != nil {
    respondError(w, http.StatusBadRequest, "Invalid repository URL: "+err.Error())
    return
}

// In Update handler, if RepositoryURL is being changed:
if body.RepositoryURL != "" {
    if err := security.ValidateExternalURL(body.RepositoryURL); err != nil {
        respondError(w, http.StatusBadRequest, "Invalid repository URL: "+err.Error())
        return
    }
}
```

**Tests**: Add handler test cases:
- Create with `http://10.0.0.1/repo.git` returns 400
- Create with `https://github.com/user/repo.git` succeeds
- Update with private IP returns 400

### 6. XSS in Autocomplete `highlightPrefix` — MEDIUM

**Problem**: The `highlightPrefix` function in the search handler wraps matched text in `<em>` tags without HTML-escaping the title. If a document title contains `<script>` or other HTML, it passes through raw.

**File**: `internal/handler/api/search_handler.go`

**Fix**: HTML-escape both segments of the title before wrapping:

```go
import "html"

func highlightPrefix(title, prefix string) string {
    if len(prefix) == 0 || len(prefix) > len(title) {
        return html.EscapeString(title)
    }
    if !strings.EqualFold(title[:len(prefix)], prefix) {
        return html.EscapeString(title)
    }
    return "<em>" + html.EscapeString(title[:len(prefix)]) + "</em>" + html.EscapeString(title[len(prefix):])
}
```

**Tests**: Add test cases:
- Title with `<script>` tag is escaped
- Title with `&` is escaped to `&amp;`
- Normal title still highlights correctly
- Empty prefix returns escaped title

### 7. Open Redirect — URL-Encode Redirect Parameter — MEDIUM

**Problem**: `SessionAuth` middleware constructs redirect URLs using `r.URL.RequestURI()` without URL-encoding, which can break parsing or enable edge-case open redirect.

**File**: `internal/auth/middleware/middleware.go`

**Fix**: URL-encode the redirect value:

```go
import "net/url"

// In SessionAuth, where the redirect is constructed:
http.Redirect(w, r, "/auth/login?redirect="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound)
```

Apply this to all locations in the middleware that construct redirect URLs with `r.URL.RequestURI()`.

### 8. Content-Disposition Header Injection — MEDIUM

**Problem**: The document download handler constructs a `Content-Disposition` header using the document title without sanitizing special characters (quotes, newlines, non-ASCII).

**File**: `internal/handler/api/document_handler.go`

**Fix**: Sanitize the filename:

```go
func sanitizeFilename(name string) string {
    return strings.Map(func(r rune) rune {
        if r == '"' || r == '\\' || r < 32 {
            return '_'
        }
        return r
    }, name)
}

// In Download handler:
filename := sanitizeFilename(doc.Title) + filepath.Ext(doc.FilePath)
w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
```

**Tests**: Add test cases for `sanitizeFilename`:
- Title with quotes: `my "doc"` becomes `my _doc_`
- Title with backslash: `my\doc` becomes `my_doc`
- Title with newline: `my\ndoc` becomes `my_doc`
- Normal title passes through unchanged

**Commit checkpoint**: `fix(security): SSRF validation on create/update, XSS in autocomplete, open redirect, header injection`

---

## Section C: Hardening

### 9. Production Config Validation — MEDIUM

**Problem**: Missing `OAUTH_SESSION_SECRET` in production causes sessions to reset on every restart (random secret generated at startup).

**File**: `internal/config/config.go`

**Fix**: In `Validate()`, add a production-mode check:

```go
func (c *Config) Validate() []string {
    var errs []string
    // ... existing checks ...

    if c.App.Env == "production" && c.OAuth.SessionSecret == "" {
        errs = append(errs, "OAUTH_SESSION_SECRET is required in production")
    }

    return errs
}
```

### 10. Inject Logger into QueueHandler — LOW

**Problem**: `QueueHandler` uses global `slog.Error()` instead of an injected logger, bypassing structured logging configuration (trace correlation, JSON formatting).

**File**: `internal/handler/api/queue_handler.go`

**Fix**: Add `logger *slog.Logger` to the `QueueHandler` struct and constructor. Replace all `slog.Error(...)` calls with `h.logger.ErrorContext(ctx, ...)`.

```go
type QueueHandler struct {
    client queueClient
    logger *slog.Logger
}

func NewQueueHandler(client queueClient, logger *slog.Logger) *QueueHandler {
    return &QueueHandler{client: client, logger: logger}
}
```

Update the constructor call in `app.go` to pass the logger.

### 11. No Object-Level Access Control on API — MEDIUM

**Problem**: Any authenticated user can modify/delete any document, external service, or git template — there are no ownership checks on mutating operations. Only the `Download` handler checks ownership.

**File**: Multiple handler files in `internal/handler/api/`

**Fix**: This is a design decision that depends on the intended access model. The PHP version may have had the same behavior (admin-only API access via OAuth scopes). Evaluate and choose one approach:

**Option A — Admin-only writes (simplest, matches PHP behavior if applicable)**:
Wrap all mutating API routes (`POST`, `PUT`, `DELETE` on documents, external-services, git-templates) with `RequireAdmin` middleware in `routes.go`. Read-only operations remain available to all authenticated users.

**Option B — Ownership-based access control**:
Add an ownership check in each mutating handler:
```go
if doc.UserID != userID && !user.IsAdmin {
    respondError(w, http.StatusForbidden, "You do not have permission to modify this resource")
    return
}
```

**Decision needed**: Check the PHP version's behavior. If the API was admin-only (OAuth clients are always admin-scoped), Option A is correct. Document the decision.

**Tests**: Whichever option is chosen, add test cases verifying:
- Non-admin user cannot delete another user's document
- Admin user can delete any document
- Non-admin user can read any public document

### 12. CSP: Remove `unsafe-inline` from `script-src` — LOW

**Problem**: `Content-Security-Policy` allows `'unsafe-inline'` for scripts, weakening XSS protection.

**File**: `internal/server/middleware.go`

**Fix**: The Vue SPA (built by Vite) should not need inline scripts. Remove `'unsafe-inline'` from `script-src`:

```go
"script-src 'self'"
```

**Verify**: After removing, load the admin SPA in a browser and check the browser console for CSP violations. If Vite injects inline scripts in production builds, either:
- Configure Vite to avoid inline scripts (preferred)
- Use CSP nonces (more complex)

### 13. SSRF: DNS Rebinding Protection — LOW

**Problem**: `ValidateExternalURL` resolves hostnames via `net.LookupHost()` but the actual HTTP request happens later — an attacker can use DNS rebinding to make the initial lookup resolve to a public IP, then switch to a private IP.

**File**: `internal/security/ssrf.go`

**Fix**: Create a custom `http.Transport` with a `DialContext` that re-validates resolved IPs at connection time. Apply this transport to all external HTTP clients (Kiwix, Confluence, Git).

```go
// SafeTransport returns an http.Transport that blocks connections to private IPs.
func SafeTransport() *http.Transport {
    return &http.Transport{
        DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
            host, port, err := net.SplitHostPort(addr)
            if err != nil {
                return nil, fmt.Errorf("invalid address: %w", err)
            }
            ips, err := net.DefaultResolver.LookupHost(ctx, host)
            if err != nil {
                return nil, fmt.Errorf("DNS resolution failed: %w", err)
            }
            for _, ipStr := range ips {
                ip := net.ParseIP(ipStr)
                if ip == nil {
                    continue
                }
                if err := checkIP(ip); err != nil {
                    return nil, fmt.Errorf("blocked connection to %s: %w", addr, err)
                }
            }
            // Connect to the first valid IP
            dialer := &net.Dialer{Timeout: 10 * time.Second}
            return dialer.DialContext(ctx, network, addr)
        },
    }
}
```

Then use `&http.Client{Transport: security.SafeTransport()}` in external service clients.

**Tests**: Unit test `SafeTransport` with a test server on localhost (should be blocked).

**Commit checkpoint**: `fix(security): production config validation, logger injection, access control, CSP hardening, DNS rebinding protection`

---

## Commit Checkpoints Summary

1. `chore: remove dead code (RollbackMigration, AccessTokenFromContext, RedisConfig)`
2. `fix(security): SSRF validation on create/update, XSS in autocomplete, open redirect, header injection`
3. `fix(security): production config validation, logger injection, access control, CSP hardening, DNS rebinding protection`

Use `/commit` after each checkpoint.

## Verification

After all changes:

```bash
# Build
go build ./...

# Test
go test -race ./...

# Lint
golangci-lint run

# Frontend (should be unaffected)
cd frontend && npx vue-tsc --noEmit && npx vitest run

# Docker
docker build -t documcp .
```

## Final State

After this phase completes:
- Zero dead code (no unused exports, no vestigial config)
- SSRF validation enforced on all user-supplied URLs at the service/handler layer
- XSS prevention in autocomplete responses
- Open redirect mitigated via URL encoding
- Content-Disposition header injection prevented
- Production config validation enforces required secrets
- Consistent structured logging in all handlers
- Object-level access control on mutating API endpoints
- CSP tightened (no `unsafe-inline`)
- DNS rebinding protection via connection-time IP validation
