# Phase 2B: OAuth 2.1 Server + OIDC Authentication

You are implementing DocuMCP-go. Run `/memory-bank` first to load project context.

## Your Task

Implement the full OAuth 2.1 authorization server and OIDC authentication. This is the most complex subsystem.

## Reference Documents

Read these first:
- `docs/contracts/oauth-flows.md` — all OAuth flow diagrams
- `docs/contracts/openapi.yaml` — OAuth endpoint specifications
- `docs/contracts/error-catalog.yaml` — OAuth error responses
- `docs/contracts/database-schema.sql` — OAuth tables (already migrated)

## Steps

### 1. Add dependencies

```bash
go get github.com/ory/fosite
go get github.com/coreos/go-oidc/v3
go get github.com/golang-jwt/jwt/v5
go get github.com/gorilla/sessions
go get golang.org/x/crypto
```

### 2. OAuth Storage — `internal/auth/oauth/storage.go`

Implement fosite's storage interfaces backed by PostgreSQL:
- `fosite.ClientManager` — lookup OAuth clients from `oauth_clients` table
- `fosite.AuthorizeCodeStorage` — store/retrieve auth codes
- `fosite.AccessTokenStorage` — store/retrieve access tokens
- `fosite.RefreshTokenStorage` — store/retrieve refresh tokens

Token security:
- Client secrets: bcrypt (low entropy)
- OAuth tokens: SHA-256 hash for storage, `{token_id}|{64_char_random}` format for O(1) lookup

### 3. OAuth Repository — `internal/repository/oauth_repository.go`

CRUD operations for all OAuth tables:
- `oauth_clients` — create, find by client_id, list, delete
- `oauth_authorization_codes` — create, find by code, revoke
- `oauth_access_tokens` — create, find by token hash, revoke, cleanup expired
- `oauth_refresh_tokens` — create, find by token hash, revoke
- `oauth_device_codes` — create, find by device_code, find by user_code, update status

### 4. OAuth Provider — `internal/auth/oauth/provider.go`

Configure fosite OAuth 2.1 provider:
- Authorization Code Grant with PKCE enforcement (S256 only)
- Refresh Token Grant
- Token revocation (RFC 7009)
- Token introspection (if needed)

### 5. Device Authorization Grant — `internal/auth/oauth/device.go`

Custom implementation of RFC 8628 (fosite doesn't include this):
- `POST /oauth/device/code` — issue device_code + user_code
- `POST /oauth/token` (grant_type=urn:ietf:params:oauth:grant-type:device_code) — poll for token
- `GET /oauth/device` — user verification page
- `POST /oauth/device` — user approves/denies
- Polling with slow_down enforcement
- Device code expiration

### 6. Dynamic Client Registration — `internal/auth/oauth/registration.go`

Custom implementation of RFC 7591:
- `POST /oauth/register` — register new client
- Validate redirect_uris, grant_types, response_types
- Generate client_id and optional client_secret
- Return registration response with client credentials

### 7. OAuth Handlers — `internal/handler/oauth/`

HTTP handlers for all OAuth endpoints:
- `GET /oauth/authorize` — authorization endpoint (show consent screen)
- `POST /oauth/authorize` — process consent
- `POST /oauth/token` — token endpoint
- `POST /oauth/revoke` — token revocation
- `POST /oauth/register` — dynamic registration
- `POST /oauth/device/code` — device authorization
- `GET /oauth/device` — device verification page
- `POST /oauth/device` — device verification submission
- `GET /.well-known/oauth-authorization-server` — RFC 8414 metadata
- `GET /.well-known/oauth-protected-resource` — RFC 9728 PRM

### 8. OIDC Authentication — `internal/auth/oidc/`

For admin login via external identity provider:
- `GET /auth/login` — redirect to OIDC provider
- `GET /auth/callback` — handle OIDC callback, create/update user, set session
- `POST /auth/logout` — clear session
- Auto-discover provider config via `.well-known/openid-configuration`
- Create user on first login (name, email, oidc_sub, oidc_provider)

### 9. Auth Middleware — `internal/auth/middleware/`

- `BearerToken` — validate OAuth access token from Authorization header
- `SessionAuth` — validate admin session cookie
- `RequireAdmin` — check user.is_admin

### 10. Wire and test

- Update `internal/app/app.go` with OAuth provider, OIDC client, session store
- Update `internal/server/routes.go` to mount all OAuth/auth routes
- Write table-driven tests for each handler, storage method, and middleware
- Test token hashing (SHA-256 for tokens, bcrypt for secrets)

```bash
go build ./...
go test ./...
golangci-lint run
```

## Rules

- Follow patterns from CLAUDE.md
- Match OAuth flows from `docs/contracts/oauth-flows.md` exactly
- Match error responses from `docs/contracts/error-catalog.yaml`
- Consent screen can be a simple HTML page for now (templ comes later)
- Commit after each major milestone using `/commit`
