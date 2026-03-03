# Phase 11: Security, Testing & Polish

You are implementing DocuMCP-go. Run `/memory-bank` first to load project context.

**Depends on**: Phases 7-10 (all previous phases complete)

## Your Task

Close all test coverage gaps, perform security review, add E2E tests, run performance benchmarks, remove the old templ+htmx admin UI, and verify CI/CD passes end-to-end. This is the final phase.

**Primary agent**: `security-reviewer`
**Secondary agents**: `test-generator`, `go-writer`, `typescript-writer`

## Coverage Targets

| Package | Current | Target | Focus |
|---------|---------|--------|-------|
| `internal/security/ssrf.go` | 0% | 90%+ | All private ranges, edge cases, DNS |
| `internal/auth/oidc/oidc.go` | 0% | 90%+ | Login, Callback, Logout, session |
| `internal/queue/` (River) | New code | 80%+ | Workers, EventBus, periodic jobs |
| `frontend/src/` | New code | 70%+ | Views, stores, composables |
| Overall Go | ~50% | 70%+ | All packages |

## Steps

### 1. SSRF Validation Tests — `internal/security/ssrf_test.go`

Test `ValidateExternalURL` and `checkIP` exhaustively. The current code in `internal/security/ssrf.go` has:
- 8 private CIDR ranges in `parsedPrivateRanges`
- Blocks: localhost, loopback, unspecified, link-local, private ranges
- Allows: http/https only
- Resolves hostnames and checks all resolved IPs

**Test cases** (table-driven with `t.Run`):

```go
package security_test

import (
    "testing"

    "git.999.haus/chris/DocuMCP-go/internal/security"
)

func TestValidateExternalURL(t *testing.T) {
    tests := []struct {
        name    string
        url     string
        wantErr bool
    }{
        // Valid URLs
        {"valid https", "https://example.com", false},
        {"valid http", "http://example.com", false},
        {"valid with port", "https://example.com:8443/path", false},
        {"valid with path", "https://example.com/some/path?q=1", false},

        // Scheme validation
        {"ftp scheme blocked", "ftp://example.com", true},
        {"file scheme blocked", "file:///etc/passwd", true},
        {"javascript scheme blocked", "javascript:alert(1)", true},
        {"no scheme", "example.com", true},
        {"empty string", "", true},

        // Localhost
        {"localhost blocked", "http://localhost", true},
        {"localhost with port", "http://localhost:8080", true},
        {"LOCALHOST uppercase", "http://LOCALHOST", true},
        {"LocalHost mixed case", "http://LocalHost", true},

        // Loopback IPs (127.0.0.0/8)
        {"127.0.0.1 blocked", "http://127.0.0.1", true},
        {"127.0.0.2 blocked", "http://127.0.0.2", true},
        {"127.255.255.255 blocked", "http://127.255.255.255", true},
        {"IPv6 loopback blocked", "http://[::1]", true},

        // Private ranges - 10.0.0.0/8
        {"10.0.0.1 blocked", "http://10.0.0.1", true},
        {"10.255.255.255 blocked", "http://10.255.255.255", true},

        // Private ranges - 172.16.0.0/12
        {"172.16.0.1 blocked", "http://172.16.0.1", true},
        {"172.31.255.255 blocked", "http://172.31.255.255", true},
        {"172.15.255.255 allowed", "http://172.15.255.255", false},  // just outside range
        {"172.32.0.1 allowed", "http://172.32.0.1", false},          // just outside range

        // Private ranges - 192.168.0.0/16
        {"192.168.0.1 blocked", "http://192.168.0.1", true},
        {"192.168.255.255 blocked", "http://192.168.255.255", true},

        // Link-local - 169.254.0.0/16
        {"169.254.0.1 blocked", "http://169.254.0.1", true},
        {"169.254.169.254 blocked (AWS metadata)", "http://169.254.169.254", true},

        // Unspecified address
        {"0.0.0.0 blocked", "http://0.0.0.0", true},
        {"[::] blocked", "http://[::]", true},

        // IPv6 private
        {"fc00:: blocked", "http://[fc00::1]", true},
        {"fd00:: blocked", "http://[fd00::1]", true},
        {"fe80:: link-local blocked", "http://[fe80::1]", true},

        // Empty/missing hostname
        {"no hostname", "http://", true},
        {"just scheme", "https:///path", true},

        // Edge cases
        {"URL with userinfo", "http://user:pass@10.0.0.1", true},
        {"decimal IP encoding", "http://2130706433", false},  // 127.0.0.1 as decimal — may or may not parse as IP
        {"hex IP encoding", "http://0x7f000001", false},       // depends on Go's net.ParseIP behavior
        {"IPv4-mapped IPv6", "http://[::ffff:127.0.0.1]", true},
        {"IPv4-mapped IPv6 private", "http://[::ffff:192.168.1.1]", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := security.ValidateExternalURL(tt.url)
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidateExternalURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
            }
        })
    }
}
```

**DNS rebinding test**: Test that hostnames resolving to private IPs are blocked. Use a test DNS resolver or mock `net.LookupHost` if possible. If not mockable, test with known hostnames that resolve to specific IPs, or document the limitation.

**Boundary tests**: Test IPs at the exact boundary of each CIDR range (first blocked IP, last blocked IP, first allowed IP after the range).

### 2. OIDC Auth Tests — `internal/auth/oidc/oidc_test.go`

Test the OIDC handler's `Login`, `Callback`, and `Logout` methods. The code is at `internal/auth/oidc/oidc.go`.

**Key interfaces to mock**:
```go
// UserRepo — defined in oidc.go
type UserRepo interface {
    FindUserByOIDCSub(ctx context.Context, sub string) (*model.User, error)
    FindUserByEmail(ctx context.Context, email string) (*model.User, error)
    CreateUser(ctx context.Context, user *model.User) error
    UpdateUser(ctx context.Context, user *model.User) error
}
```

**Mock OIDC provider**: Use `httptest.NewServer` to create a mock OIDC provider that serves:
- `GET /.well-known/openid-configuration` — discovery document
- `GET /certs` — JWKS endpoint (use `go-jose` to generate test keys)
- `POST /token` — token endpoint (return test ID token)

**Test cases**:

```go
func TestLogin(t *testing.T) {
    // Setup mock OIDC provider, create Handler via oidc.New()
    t.Run("redirects to OIDC provider", func(t *testing.T) {
        // GET /auth/login → 302 redirect to provider authorize URL
        // Verify state parameter saved in session
        // Verify redirect_uri is correct
    })
    t.Run("preserves safe redirect param", func(t *testing.T) {
        // GET /auth/login?redirect=/admin/documents → state includes redirect
    })
    t.Run("ignores unsafe redirect", func(t *testing.T) {
        // GET /auth/login?redirect=//evil.com → redirect not preserved
        // GET /auth/login?redirect=\\evil.com → redirect not preserved
    })
}

func TestCallback(t *testing.T) {
    t.Run("exchanges code and creates session", func(t *testing.T) {
        // GET /auth/callback?code=VALID&state=VALID
        // Verify session contains user_id, is_admin, user_email
        // Verify redirect to /admin
    })
    t.Run("rejects invalid state", func(t *testing.T) {
        // GET /auth/callback?code=VALID&state=WRONG → error
    })
    t.Run("rejects missing code", func(t *testing.T) {
        // GET /auth/callback?state=VALID → error
    })
    t.Run("creates new user on first login", func(t *testing.T) {
        // Mock UserRepo returns not found for sub and email
        // Verify CreateUser called with correct fields
        // Verify is_admin defaults to false
    })
    t.Run("links existing user by email", func(t *testing.T) {
        // Mock UserRepo returns not found for sub but found for email
        // Verify UpdateUser called to set OIDC sub
    })
    t.Run("finds existing user by sub", func(t *testing.T) {
        // Mock UserRepo returns user for sub
        // No create/update called
    })
    t.Run("regenerates session to prevent fixation", func(t *testing.T) {
        // Verify old session values are cleared
        // Verify new session has user data
    })
}

func TestLogout(t *testing.T) {
    t.Run("clears session and redirects", func(t *testing.T) {
        // POST /auth/logout with valid session
        // Verify session MaxAge = -1
        // Verify redirect to /
    })
}

func TestIsSafeRedirect(t *testing.T) {
    // Test the isSafeRedirect helper (may need to export or test via Login)
    tests := []struct {
        redirect string
        safe     bool
    }{
        {"/admin", true},
        {"/admin/documents", true},
        {"//evil.com", false},
        {"\\\\evil.com", false},
        {"https://evil.com", false},
        {"", false},
        {"javascript:alert(1)", false},
    }
}
```

### 3. River Queue Integration Tests

Use `testcontainers-go` (already a dependency) to run tests against a real Postgres instance:

**`internal/queue/river_integration_test.go`** (build tag `//go:build integration`):
```go
func TestRiverIntegration(t *testing.T) {
    // Start Postgres testcontainer
    // Create pgxpool
    // Run River migrations
    // Create RiverClient with test workers

    t.Run("insert and process extraction job", func(t *testing.T) {
        // Insert DocumentExtractArgs
        // Wait for worker to process
        // Verify DocumentProcessor.ProcessDocument was called
    })

    t.Run("failed job retries with backoff", func(t *testing.T) {
        // Insert job that fails
        // Verify retry after configured delay
        // Verify max 3 attempts
    })

    t.Run("periodic jobs execute on schedule", func(t *testing.T) {
        // Register periodic job with short interval
        // Verify it fires
    })

    t.Run("event bus publishes on job lifecycle", func(t *testing.T) {
        // Subscribe to EventBus
        // Insert and complete a job
        // Verify dispatched + completed events received
    })
}
```

**Unit tests** for non-integration code:
- `internal/queue/events_test.go` — EventBus subscribe/publish/unsubscribe
- `internal/queue/jobs_test.go` — Kind() and InsertOpts() return values
- `internal/queue/periodic_test.go` — BuildPeriodicJobs with empty/non-empty schedules

### 4. Vue Component Tests

Ensure all Phase 9 and 10 components have Vitest tests. Use `@vue/test-utils` with `mount` or `shallowMount`. Mock API calls with `vi.fn()`.

**Priority test files** (if not already created):
- `frontend/src/views/__tests__/DashboardView.spec.ts`
- `frontend/src/views/__tests__/DocumentListView.spec.ts`
- `frontend/src/views/__tests__/DocumentDetailView.spec.ts`
- `frontend/src/components/documents/__tests__/UploadModal.spec.ts`
- `frontend/src/components/documents/__tests__/ContentViewer.spec.ts`
- `frontend/src/components/oauth/__tests__/SecretDisplayModal.spec.ts`
- `frontend/src/components/shared/__tests__/TreeNode.spec.ts`
- `frontend/src/composables/__tests__/useSSE.spec.ts`
- `frontend/src/stores/__tests__/auth.spec.ts`
- `frontend/src/stores/__tests__/documents.spec.ts`

### 5. E2E Tests — Playwright

Set up Playwright for end-to-end testing of critical user flows.

**`frontend/playwright.config.ts`**:
```ts
import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './e2e',
  use: {
    baseURL: 'http://localhost:8080',
  },
  webServer: {
    command: 'cd .. && go run ./cmd/server',
    port: 8080,
    reuseExistingServer: true,
  },
})
```

**`frontend/e2e/`** test files:

```
e2e/
  auth.spec.ts          — Login redirect, session persistence, logout
  documents.spec.ts     — Upload document, view detail, soft-delete, restore
  oauth-clients.spec.ts — Create client, view secret, revoke
```

**Note**: E2E tests require a running Go server with a real database. These may only run in CI with Docker services or be marked as skippable in local dev.

Install Playwright:
```bash
cd frontend
npm install -D @playwright/test
npx playwright install chromium
```

### 6. Security Review

Run the `security-reviewer` agent on all code from Phases 7-10. Focus areas:

**Go code**:
- `internal/queue/` — verify no SQL injection in River job args, no sensitive data in job payloads
- `internal/handler/api/sse_handler.go` — verify SSE endpoint requires auth, no XSS in event data
- `internal/handler/api/queue_handler.go` — verify admin-only access, no mass data exposure
- `internal/handler/api/auth_handler.go` — verify session validation, no user enumeration
- `web/frontend/handler.go` — verify SPA handler doesn't serve files outside dist/

**Frontend code**:
- `ContentViewer.vue` — verify DOMPurify sanitization, no XSS from document content
- `SecretDisplayModal.vue` — verify secret not logged or persisted in state after modal closes
- Auth store — verify credentials not stored in localStorage
- SSE composable — verify no sensitive data exposed via events
- API client — verify auth headers not leaked in error messages

**OWASP Top 10 checklist**:
1. A01: Broken Access Control — verify admin-only routes enforce admin check
2. A02: Cryptographic Failures — verify session secrets, token generation
3. A03: Injection — verify no SQL injection, no command injection, XSS prevention
4. A04: Insecure Design — verify SSRF protection, rate limiting
5. A05: Security Misconfiguration — verify CORS, CSP headers, debug mode
6. A07: Authentication Failures — verify session fixation prevention, brute force protection
7. A09: Logging Failures — verify sensitive data not logged

### 7. Performance Benchmarks

Add Go benchmarks for critical paths:

**`internal/security/ssrf_bench_test.go`**:
```go
func BenchmarkValidateExternalURL(b *testing.B) {
    for i := 0; i < b.N; i++ {
        security.ValidateExternalURL("https://example.com")
    }
}
```

**`internal/search/searcher_bench_test.go`** (if testcontainers available):
```go
func BenchmarkSearch(b *testing.B) { /* ... */ }
```

**`internal/extractor/` benchmarks** for each extractor type with sample files from `internal/testutil/testdata/`.

### 8. Remove Old Admin UI

**Delete files**:
- `internal/handler/admin/handler.go`
- `internal/handler/admin/deps.go`
- All files in `web/templates/` (`.templ` and `_templ.go` files):
  - `layout.templ`, `layout_templ.go`
  - `components.templ`, `components_templ.go`
  - `dashboard.templ`, `dashboard_templ.go`
  - `documents.templ`, `documents_templ.go`
  - `users.templ`, `users_templ.go`
  - `oauth_clients.templ`, `oauth_clients_templ.go`
  - `external_services.templ`, `external_services_templ.go`
  - `zim_archives.templ`, `zim_archives_templ.go`
  - `confluence_spaces.templ`, `confluence_spaces_templ.go`
  - `git_templates.templ`, `git_templates_templ.go`
  - `login.templ`, `login_templ.go`

**Modify `internal/app/app.go`**:
- Remove `adminhandler` import and `adminH` creation
- Remove `AdminHandler` from `Deps`

**Modify `internal/server/routes.go`**:
- Remove the entire `/admin` route group that uses `deps.AdminHandler`
- Remove `adminhandler` import
- Remove `AdminHandler *adminhandler.Handler` from `Deps`
- Update the Vue SPA to mount at `/admin/*` instead of `/app/*` (now that old admin is gone)
- Keep `/admin/login` redirecting to `/auth/login` for backward compatibility

**Remove dependencies from `go.mod`**:
```bash
# After deleting templ files:
go mod tidy
```

Verify `github.com/a-h/templ` and `github.com/gorilla/csrf` are no longer needed. If `gorilla/csrf` is still used by the OAuth subrouter, keep it — only remove if no longer imported.

### 9. Final CI/CD Verification

Run the full CI pipeline locally:

```bash
# Go
go build ./...
go test -race -cover ./...
golangci-lint run
go vet ./...

# Frontend
cd frontend
npx vitest run
npx vue-tsc --noEmit
npm run build

# E2E (if services available)
npx playwright test

# Docker
docker build -t documcp .

# Coverage report
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | grep total
# Target: 70%+ overall

# Security-specific coverage
go test -coverprofile=ssrf.out ./internal/security/...
go tool cover -func=ssrf.out
# Target: 90%+ for ssrf.go

go test -coverprofile=oidc.out ./internal/auth/oidc/...
go tool cover -func=oidc.out
# Target: 90%+ for oidc.go
```

### 10. Update OpenAPI Spec

If any new endpoints were added in this phase, update `docs/contracts/openapi.yaml` to include them. Then regenerate the frontend API client:

```bash
cd frontend && npm run generate-api
```

## Commit Checkpoints

1. **SSRF + OIDC tests**: `ssrf_test.go`, `oidc_test.go`, achieve 90%+ coverage on both
2. **Vue tests**: component tests, store tests, composable tests
3. **E2E tests + River integration tests**: Playwright setup, integration test with testcontainers
4. **Security review**: fix any issues found, add security-related tests
5. **Remove old admin + final CI**: delete templ/admin files, swap SPA mount point, `go mod tidy`, verify all CI green

Use `/commit` after each checkpoint.

## Final State

After this phase completes:

- Old templ+htmx admin is completely removed
- Vue SPA serves at `/admin/*` (or `/app/*`)
- All security-critical code has 90%+ test coverage
- Overall Go coverage is 70%+
- All CI jobs pass (lint, test, build for both Go and frontend)
- Docker image builds successfully with embedded frontend
- No known security vulnerabilities in new code
