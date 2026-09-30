# OAuth 2.1 Flow Sequences

## Overview

DocuMCP implements an OAuth 2.1 authorization server with the following characteristics:

| Property | Value |
|----------|-------|
| Token format | `{token_id}\|{64_char_random_string}` (plain text); the server stores a versioned HMAC-SHA256 of the random portion (`v<key-version>$<hex>`) |
| Client secret hashing | bcrypt |
| PKCE | Required for every client at `/oauth/authorize` (public and confidential); S256 only |
| Token endpoint encoding | `application/x-www-form-urlencoded` only; any other `Content-Type` returns `415` |
| Resource indicators (RFC 8707) | `resource` parameter at `/oauth/authorize` and `/oauth/device/code`; must match `OAUTH_ALLOWED_RESOURCES` |
| Authorization response issuer (RFC 9207) | `iss` is added to every authorization redirect, success and error |
| Authorization code lifetime | 600 seconds (10 minutes) by default |
| Access token lifetime | 3600 seconds (1 hour) by default |
| Refresh token lifetime | 2592000 seconds (30 days) by default |
| Device code lifetime | 600 seconds (10 minutes) by default |
| Device polling interval | 5 seconds minimum; +5 seconds per `slow_down`, capped at 300 |
| Default scope (DCR) | `mcp:access mcp:read documents:read search:read zim:read templates:read services:read` |
| Registered scopes | `mcp:access`, `mcp:read`, `mcp:write`, `documents:read`, `documents:write`, `search:read`, `zim:read`, `templates:read`, `templates:write`, `services:read`, `services:write`, `admin` |
| Consent ceiling | Non-admin users can grant the default scope set only. Admins can grant every scope except `admin` and `services:write`. Neither is ever granted to an OAuth client. |
| State parameter | Required, minimum 8 characters |
| Consent nonce | UUID v4, 10-minute expiry, prevents TOCTOU attacks |
| Localhost redirect | Any port allowed for `localhost`, `127.0.0.1`, `[::1]` (RFC 8252) |

### Scopes an MCP client needs

A token is only usable at the MCP endpoint (`/documcp`) if it has:

- `mcp:access` to reach the endpoint at all.
- `mcp:read` to call read tools, and `mcp:write` to call write tools (`create_document`, `update_document`, `replace_document_content`, `delete_document`).
- A `resource` binding equal to the MCP resource URI (`{APP_URL}{DOCUMCP_ENDPOINT}`, for example `https://documcp.example.com/documcp`). A token with no `resource` is rejected unless the operator sets `OAUTH_ACCEPT_EMPTY_RESOURCE=true`.

The examples below request `mcp:access mcp:read mcp:write` and bind the token to `https://documcp.example.com/documcp`.

### Rate Limits

All limits are per client IP.

| Endpoints | Limits |
|-----------|--------|
| `POST /oauth/register` | 10/hour, 50/day |
| `POST /oauth/token`, `POST /oauth/revoke` (shared) | 30/minute, 100/hour |
| `GET /oauth/authorize`, `POST /oauth/authorize/approve`, `POST /oauth/authorize/deny` | 30/minute |
| `POST /oauth/device/code` | 30/minute |
| `POST /oauth/device`, `POST /oauth/device/approve` | 10/minute |

`GET /oauth/device` has no dedicated limit. Failed `user_code` submissions are also capped per user (`OAUTH_DEVICE_FAILURE_LIMIT` within `OAUTH_DEVICE_FAILURE_WINDOW`).

---

## 1. Discovery

### 1.1 Authorization Server Metadata (RFC 8414)

**Request:**

```http
GET /.well-known/oauth-authorization-server HTTP/1.1
Host: documcp.example.com
```

**Response:**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "issuer": "https://documcp.example.com",
  "authorization_endpoint": "https://documcp.example.com/oauth/authorize",
  "token_endpoint": "https://documcp.example.com/oauth/token",
  "revocation_endpoint": "https://documcp.example.com/oauth/revoke",
  "registration_endpoint": "https://documcp.example.com/oauth/register",
  "device_authorization_endpoint": "https://documcp.example.com/oauth/device/code",
  "response_types_supported": ["code"],
  "grant_types_supported": [
    "authorization_code",
    "refresh_token",
    "urn:ietf:params:oauth:grant-type:device_code"
  ],
  "token_endpoint_auth_methods_supported": [
    "none",
    "client_secret_basic",
    "client_secret_post"
  ],
  "code_challenge_methods_supported": ["S256"],
  "scopes_supported": [
    "admin",
    "documents:read",
    "documents:write",
    "mcp:access",
    "mcp:read",
    "mcp:write",
    "search:read",
    "services:read",
    "services:write",
    "templates:read",
    "templates:write",
    "zim:read"
  ],
  "protected_resources": ["https://documcp.example.com"],
  "resource_indicators_supported": true,
  "authorization_response_iss_parameter_supported": true
}
```

`scopes_supported` lists every registered scope, sorted. It includes `admin` and `services:write`, which the consent screen never grants to an OAuth client. For the scopes a client should actually request, use the protected resource metadata below.

### 1.2 Protected Resource Metadata (RFC 9728)

`scopes_supported` differs per protected resource. The MCP endpoint advertises only the scopes it checks. Every other path, including the root, describes the REST API and advertises every scope a bearer token can carry (all registered scopes except `admin` and `services:write`).

**Request (root):**

```http
GET /.well-known/oauth-protected-resource HTTP/1.1
Host: documcp.example.com
```

**Response:**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "resource": "https://documcp.example.com",
  "authorization_servers": ["https://documcp.example.com"],
  "scopes_supported": [
    "documents:read",
    "documents:write",
    "mcp:access",
    "mcp:read",
    "mcp:write",
    "search:read",
    "services:read",
    "templates:read",
    "templates:write",
    "zim:read"
  ],
  "bearer_methods_supported": ["header"]
}
```

### 1.3 Protected Resource Metadata with Path Suffix (RFC 9728)

**Request (MCP endpoint):**

```http
GET /.well-known/oauth-protected-resource/documcp HTTP/1.1
Host: documcp.example.com
```

**Response:**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "resource": "https://documcp.example.com/documcp",
  "authorization_servers": ["https://documcp.example.com"],
  "scopes_supported": ["mcp:access", "mcp:read", "mcp:write"],
  "bearer_methods_supported": ["header"]
}
```

---

## 2. Client Registration (RFC 7591)

Two settings control `POST /oauth/register`:

| Setting | Default | Effect |
|---------|---------|--------|
| `OAUTH_REGISTRATION_ENABLED` | `true` | `false` returns `404 Not Found` for every registration request. |
| `OAUTH_REGISTRATION_REQUIRE_AUTH` | `true` | `true`: the caller must send the admin panel session cookie (`documcp_session`) of an admin user. Bearer tokens are not checked. `false`: anyone can register, with the restrictions below. |

When `OAUTH_REGISTRATION_REQUIRE_AUTH=false`, the server constrains every registration:

- `urn:ietf:params:oauth:grant-type:device_code` in `grant_types` is rejected with `400 invalid_client_metadata`.
- `token_endpoint_auth_method` is forced to `none` (public client, no secret).
- `scope` is ignored and replaced with the default scope set.

### 2.1 Register a Confidential Client

Confidential clients authenticate at the token endpoint using a client secret. The server returns `client_secret` only once during registration. This requires `OAUTH_REGISTRATION_REQUIRE_AUTH=true` and an admin session.

**Request:**

```http
POST /oauth/register HTTP/1.1
Host: documcp.example.com
Content-Type: application/json
Cookie: documcp_session=<admin-session-cookie>

{
  "client_name": "My Backend Service",
  "redirect_uris": ["https://myapp.example.com/callback"],
  "grant_types": ["authorization_code", "refresh_token"],
  "response_types": ["code"],
  "token_endpoint_auth_method": "client_secret_basic",
  "scope": "mcp:access mcp:read mcp:write",
  "software_id": "my-backend-service",
  "software_version": "1.0.0"
}
```

**Response:**

```http
HTTP/1.1 201 Created
Content-Type: application/json

{
  "client_id": "550e8400-e29b-41d4-a716-446655440000",
  "client_secret": "a1b2c3d4...64_random_chars...z9y8x7w6",
  "client_id_issued_at": 1700000000,
  "client_name": "My Backend Service",
  "redirect_uris": ["https://myapp.example.com/callback"],
  "grant_types": ["authorization_code", "refresh_token"],
  "response_types": ["code"],
  "token_endpoint_auth_method": "client_secret_basic",
  "scope": "mcp:access mcp:read mcp:write"
}
```

Client secrets do not expire. The response omits `client_secret_expires_at` because its value is `0` and the field is serialized with `omitempty`.

### 2.2 Register a Public Client

Public clients set `token_endpoint_auth_method` to `"none"` and do not receive a `client_secret`. This example omits `scope`, so the server assigns the default scope set.

**Request:**

```http
POST /oauth/register HTTP/1.1
Host: documcp.example.com
Content-Type: application/json
Cookie: documcp_session=<admin-session-cookie>

{
  "client_name": "My MCP CLI Tool",
  "redirect_uris": ["http://localhost:3334/callback"],
  "grant_types": ["authorization_code", "refresh_token"],
  "response_types": ["code"],
  "token_endpoint_auth_method": "none"
}
```

Omit the `Cookie` header when `OAUTH_REGISTRATION_REQUIRE_AUTH=false`.

**Response:**

```http
HTTP/1.1 201 Created
Content-Type: application/json

{
  "client_id": "660e8400-e29b-41d4-a716-446655440001",
  "client_id_issued_at": 1700000000,
  "client_name": "My MCP CLI Tool",
  "redirect_uris": ["http://localhost:3334/callback"],
  "grant_types": ["authorization_code", "refresh_token"],
  "response_types": ["code"],
  "token_endpoint_auth_method": "none",
  "scope": "mcp:access mcp:read documents:read search:read zim:read templates:read services:read"
}
```

Note: No `client_secret` or `client_secret_expires_at` fields in the response.

The default scope set includes `mcp:read` but not `mcp:write`. A client registered with defaults can call read tools only. It can still request `mcp:write` at `/oauth/authorize`; see section 3.2 for how the server narrows the requested scope.

### 2.3 Registration Validation Rules

| Field | Rules |
|-------|-------|
| `client_name` | Required, string, max 255 |
| `redirect_uris` | Required, array, 1 to 10 elements |
| `redirect_uris.*` | Valid absolute URL; must use `https` unless the host is a loopback address |
| `grant_types` | Optional, array; each must be one of: `authorization_code`, `refresh_token`, `urn:ietf:params:oauth:grant-type:device_code`. Default: `["authorization_code"]` |
| `response_types` | Optional, array; each must be: `code`. Default: `["code"]` |
| `scope` | Optional, space-delimited string of registered scopes. Default: the default scope set. An unknown scope returns `400 invalid_client_metadata`. |
| `token_endpoint_auth_method` | Optional; one of: `none`, `client_secret_basic`, `client_secret_post`. Default: `none` |
| `software_id` | Optional, string, max 255 |
| `software_version` | Optional, string, max 100 |

The default `grant_types` does not include `refresh_token`. Register it explicitly if the client needs to refresh tokens.

**Error -- missing required field:**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "invalid_client_metadata",
  "error_description": "The client name field is required."
}
```

**Error -- registration disabled:**

```http
HTTP/1.1 404 Not Found
```

**Error -- unauthenticated (when auth required):**

```http
HTTP/1.1 401 Unauthorized
```

**Error -- non-admin user (when auth required):**

```http
HTTP/1.1 403 Forbidden
```

---

## 3. Authorization Code + PKCE (OAuth 2.1)

### 3.1 Generate PKCE Parameters

The client generates a `code_verifier` (43-128 characters from the unreserved URI character set) and derives the `code_challenge`:

```
code_verifier  = random_string(43..128 chars, charset: [A-Z][a-z][0-9]-._~)
code_challenge = BASE64URL(SHA256(code_verifier))
```

Example:

```
code_verifier  = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk..."
code_challenge = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
```

### 3.2 Authorization Request

The client redirects the user's browser to the authorization endpoint.

**Request:**

```http
GET /oauth/authorize?response_type=code
    &client_id=660e8400-e29b-41d4-a716-446655440001
    &redirect_uri=http%3A%2F%2Flocalhost%3A3334%2Fcallback
    &state=xyzABC123_random_state
    &scope=mcp%3Aaccess%20mcp%3Aread%20mcp%3Awrite
    &code_challenge=E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM
    &code_challenge_method=S256
    &resource=https%3A%2F%2Fdocumcp.example.com%2Fdocumcp HTTP/1.1
Host: documcp.example.com
Cookie: documcp_session=<session-cookie>
```

#### Authorization Request Validation Rules

| Parameter | Rules |
|-----------|-------|
| `response_type` | Required; must be `code` |
| `client_id` | Required, string; must be a registered client |
| `redirect_uri` | Required; must match a registered redirect URI (see 7.2) |
| `state` | Required, string, min 8 characters |
| `scope` | Optional, space-delimited. Narrowed to what the signed-in user may grant (see below). If omitted, the code and token carry no scope, and the MCP endpoint rejects the token with `403 insufficient_scope`. |
| `code_challenge` | Required for every client |
| `code_challenge_method` | Required for every client; must be `S256` |
| `resource` | Optional at this endpoint. If present, must exactly match an entry in `OAUTH_ALLOWED_RESOURCES` (default: `{APP_URL}` and `{APP_URL}{DOCUMCP_ENDPOINT}`), or the server returns `400 invalid_target`. If omitted, the token has no audience binding and `/documcp` and `/api` reject it unless `OAUTH_ACCEPT_EMPTY_RESOURCE=true`. |

**Scope narrowing.** The server intersects the requested scope with the signed-in user's consent ceiling:

- Non-admin user: the default scope set (`mcp:access mcp:read documents:read search:read zim:read templates:read services:read`). A request for `mcp:access mcp:read mcp:write` becomes `mcp:access mcp:read`.
- Admin user: every scope except `admin` and `services:write`.

If nothing is left after narrowing, the server returns `400 invalid_scope`. The consent screen shows the narrowed scope. The example below assumes an admin user.

**Response -- consent screen (user is authenticated):**

```http
HTTP/1.1 200 OK
Content-Type: text/html

<!-- Consent screen with client info, scopes, nonce hidden field -->
<form method="POST" action="/oauth/authorize/approve">
  <input type="hidden" name="client_id" value="660e8400-...">
  <input type="hidden" name="redirect_uri" value="http://localhost:3334/callback">
  <input type="hidden" name="state" value="xyzABC123_random_state">
  <input type="hidden" name="scope" value="mcp:access mcp:read mcp:write">
  <input type="hidden" name="code_challenge" value="E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM">
  <input type="hidden" name="code_challenge_method" value="S256">
  <input type="hidden" name="nonce" value="550e8400-e29b-41d4-a716-446655440000">
  ...
  <button type="submit" formaction="/oauth/authorize/deny">Deny</button>
</form>
```

The server stores the following in the session:

```json
{
  "nonce": "550e8400-e29b-41d4-a716-446655440000",
  "client_id": "660e8400-...",
  "state": "xyzABC123_random_state",
  "redirect_uri": "http://localhost:3334/callback",
  "code_challenge": "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM",
  "code_challenge_method": "S256",
  "scope": "mcp:access mcp:read mcp:write",
  "resource": "https://documcp.example.com/documcp",
  "timestamp": 1700000000
}
```

**Response -- user not authenticated:**

```http
HTTP/1.1 302 Found
Location: /auth/login?redirect=<url-encoded /oauth/authorize?... request URI>
```

### 3.3 Authorization Approval

The user submits the consent form.

**Request:**

```http
POST /oauth/authorize/approve HTTP/1.1
Host: documcp.example.com
Content-Type: application/x-www-form-urlencoded
Cookie: documcp_session=<session-cookie>

client_id=660e8400-e29b-41d4-a716-446655440001&redirect_uri=http%3A%2F%2Flocalhost%3A3334%2Fcallback&scope=mcp%3Aaccess+mcp%3Aread+mcp%3Awrite&state=xyzABC123_random_state&code_challenge=E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM&code_challenge_method=S256&nonce=550e8400-e29b-41d4-a716-446655440000
```

The endpoint also accepts the same fields as a JSON object when `Content-Type` is exactly `application/json`.

#### Approval Validation Rules

| Parameter | Rules |
|-----------|-------|
| `client_id` | Required; must equal the pending request |
| `redirect_uri` | Must equal the pending request |
| `scope` | Must equal the pending (narrowed) scope |
| `state` | Must equal the pending request |
| `code_challenge` | Must equal the pending request |
| `code_challenge_method` | Must equal the pending request |
| `nonce` | Required; must equal the pending request |

The server validates:
1. Pending request exists in session (prevents stale tab attacks).
2. Nonce matches (prevents TOCTOU race conditions).
3. `client_id` matches pending request.
4. `state` matches pending request.
5. Timestamp is within 10 minutes (rejects expired requests).
6. State parameter format is safe (alphanumeric + `._~()'-`, max 500 characters).
7. `redirect_uri`, `scope`, `code_challenge`, and `code_challenge_method` match the pending request. The code is issued from the session values, not the POST body.

On approval the server records a time-bounded scope grant for the client (`OAUTH_SCOPE_GRANT_TTL`).

**Response -- success:**

The server responds `200 OK` with an HTML page that redirects in JavaScript, not an HTTP `302`. Safari and embedded browsers do not reliably follow cross-origin redirects from popup windows. The redirect target is:

```
http://localhost:3334/callback?code=42%7CaBcDeF...64chars...&iss=https%3A%2F%2Fdocumcp.example.com&state=xyzABC123_random_state
```

The authorization code is in `{id}|{64_char_random}` format, URL-encoded. `iss` (RFC 9207) is always present and equals the `issuer` from the authorization server metadata. Clients should compare it before redeeming the code.

**Response -- denied (`POST /oauth/authorize/deny`):**

The deny endpoint takes only `nonce`. It clears the pending request and redirects the same way to:

```
http://localhost:3334/callback?error=access_denied&error_description=The+resource+owner+denied+the+request&iss=https%3A%2F%2Fdocumcp.example.com&state=xyzABC123_random_state
```

### 3.4 Token Exchange

The client exchanges the authorization code for tokens at the token endpoint.

The token endpoint accepts only `application/x-www-form-urlencoded` bodies (RFC 6749 §3.2). A JSON body returns `415 Unsupported Media Type` with `invalid_request`.

**Request (public client with PKCE):**

```http
POST /oauth/token HTTP/1.1
Host: documcp.example.com
Content-Type: application/x-www-form-urlencoded

grant_type=authorization_code&code=42%7CaBcDeF...64chars...&client_id=660e8400-e29b-41d4-a716-446655440001&redirect_uri=http%3A%2F%2Flocalhost%3A3334%2Fcallback&code_verifier=dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk...
```

**Request (confidential client, `client_secret_basic`):**

```http
POST /oauth/token HTTP/1.1
Host: documcp.example.com
Content-Type: application/x-www-form-urlencoded
Authorization: Basic <base64(client_id:client_secret)>

grant_type=authorization_code&code=42%7CaBcDeF...64chars...&redirect_uri=https%3A%2F%2Fmyapp.example.com%2Fcallback&code_verifier=dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk...
```

For `client_secret_post`, send `client_id` and `client_secret` in the body instead. Sending credentials in both the `Authorization` header and the body returns `400 invalid_request`.

**Response:**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "access_token": "99|xYzAbC...64chars...",
  "token_type": "Bearer",
  "expires_in": 3600,
  "refresh_token": "100|pQrStU...64chars...",
  "scope": "mcp:access mcp:read mcp:write"
}
```

The authorization code is revoked after successful exchange (one-time use). Presenting a used code again revokes every token issued from it.

The access token inherits the `resource` captured at `/oauth/authorize`. The token scope is the approved scope intersected with the client's effective scope (its registered scope plus unexpired consent grants).

#### Token Request Validation Rules (authorization_code)

| Parameter | Rules |
|-----------|-------|
| `grant_type` | Required; `authorization_code` |
| `client_id` | Required (body or HTTP Basic) |
| `client_secret` | Required for confidential clients (body or HTTP Basic) |
| `code` | Required |
| `redirect_uri` | Required; must equal the value used at `/oauth/authorize` |
| `code_verifier` | Required when a `code_challenge` was sent (always, since PKCE is mandatory). Sending one when no challenge was stored is rejected. |
| `resource` | Optional. If present, must equal the `resource` sent at `/oauth/authorize`. |

The client must have `authorization_code` in its registered `grant_types`, or the server returns `400 unauthorized_client`.

---

## 4. Refresh Token

The client exchanges a refresh token for a new access token and new refresh token. The old access token and old refresh token are both revoked (token rotation).

**Request:**

```http
POST /oauth/token HTTP/1.1
Host: documcp.example.com
Content-Type: application/x-www-form-urlencoded

grant_type=refresh_token&refresh_token=100%7CpQrStU...64chars...&client_id=660e8400-e29b-41d4-a716-446655440001
```

For confidential clients, add client authentication (HTTP Basic or `client_secret` in the body). This example also narrows the scope to read-only:

```http
POST /oauth/token HTTP/1.1
Host: documcp.example.com
Content-Type: application/x-www-form-urlencoded

grant_type=refresh_token&refresh_token=100%7CpQrStU...64chars...&client_id=550e8400-e29b-41d4-a716-446655440000&client_secret=a1b2c3d4...64_random_chars...z9y8x7w6&scope=mcp%3Aaccess+mcp%3Aread
```

**Response:**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "access_token": "101|nEwToKeN...64chars...",
  "token_type": "Bearer",
  "expires_in": 3600,
  "refresh_token": "102|nEwReFrEsH...64chars...",
  "scope": "mcp:access mcp:read mcp:write"
}
```

The new access token keeps the original `resource` binding. Presenting a refresh token that was already rotated revokes every token in the same grant.

#### Token Request Validation Rules (refresh_token)

| Parameter | Rules |
|-----------|-------|
| `grant_type` | Required; `refresh_token` |
| `client_id` | Required (body or HTTP Basic) |
| `client_secret` | Required for confidential clients (body or HTTP Basic) |
| `refresh_token` | Required |
| `scope` | Optional; must be a subset of the original scope |
| `resource` | Optional; must equal the original token's `resource` |

The client must have `refresh_token` in its registered `grant_types`, or the server returns `400 unauthorized_client`.

---

## 5. Device Authorization (RFC 8628)

The device authorization flow is designed for input-constrained devices (CLI tools, smart displays) that cannot easily handle browser redirects or type long URLs. This eliminates the callback issues commonly seen with mcp-remote.

### 5.1 Device Authorization Request

The device requests authorization by providing its `client_id`. The client must have `urn:ietf:params:oauth:grant-type:device_code` in its `grant_types`. Unauthenticated registration cannot add that grant type (see section 2).

**Request:**

```http
POST /oauth/device/code HTTP/1.1
Host: documcp.example.com
Content-Type: application/x-www-form-urlencoded

client_id=660e8400-e29b-41d4-a716-446655440001&scope=mcp%3Aaccess+mcp%3Aread+mcp%3Awrite&resource=https%3A%2F%2Fdocumcp.example.com%2Fdocumcp
```

This endpoint also accepts a JSON body with the same fields.

**Response:**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "device_code": "55|dEvIcEcOdE...64chars...",
  "user_code": "BCDF-GHJK",
  "verification_uri": "https://documcp.example.com/oauth/device",
  "verification_uri_complete": "https://documcp.example.com/oauth/device?user_code=BCDF-GHJK",
  "expires_in": 600,
  "interval": 5
}
```

The `user_code` follows XXXX-XXXX format using a base-20 character set (`BCDFGHJKLMNPQRSTVWXZ`) that excludes vowels (to prevent accidental profanity) and confusing characters (`0`, `1`, `O`, `I`).

#### Device Authorization Validation Rules

| Parameter | Rules |
|-----------|-------|
| `client_id` | Required, string |
| `scope` | Optional, space-delimited registered scopes. Narrowed to the approving user's consent ceiling at approval time (same rules as section 3.2). If omitted, the token carries no scope and cannot call MCP tools. |
| `resource` | Optional. If present, must exactly match an entry in `OAUTH_ALLOWED_RESOURCES`, or the server returns `400 invalid_target`. This is the only place to bind a device-flow token to an audience: the device-code token request ignores `resource`. Without it, `/documcp` and `/api` reject the token unless `OAUTH_ACCEPT_EMPTY_RESOURCE=true`. |

### 5.2 User Verification

The user navigates to the `verification_uri` (or scans a QR code with `verification_uri_complete`).

**Step 1 -- Verification page (may include pre-filled code via query parameter):**

```http
GET /oauth/device?user_code=BCDF-GHJK HTTP/1.1
Host: documcp.example.com
Cookie: <session-cookie>
```

```http
HTTP/1.1 200 OK
Content-Type: text/html

<!-- Form to enter user code -->
```

If the user is not authenticated, they are redirected to login first:

```http
HTTP/1.1 302 Found
Location: /auth/login?redirect=%2Foauth%2Fdevice%3Fuser_code%3DBCDF-GHJK
```

**Step 2 -- Submit user code:**

```http
POST /oauth/device HTTP/1.1
Host: documcp.example.com
Content-Type: application/x-www-form-urlencoded
Cookie: <session-cookie>

user_code=BCDF-GHJK
```

User code lookup is case-insensitive and works with or without the dash separator.

| Validation Rule | Value |
|-----------------|-------|
| `user_code` | Required, string, max 9 characters |

On success, the server stores `device_code_pending` in the session and renders the consent screen:

```http
HTTP/1.1 200 OK
Content-Type: text/html

<!-- Consent screen showing client name, requested scopes, approve/deny buttons -->
```

**Step 3 -- Approve or deny:**

```http
POST /oauth/device/approve HTTP/1.1
Host: documcp.example.com
Content-Type: application/x-www-form-urlencoded
Cookie: <session-cookie>

user_code=BCDF-GHJK&approve=approve
```

| Parameter | Rules |
|-----------|-------|
| `user_code` | Required; must match the pending session value |
| `approve` | `approve` approves; any other value (the form sends `deny`) denies |

The server validates:
1. Pending device authorization exists in session.
2. User code matches the session value (case-insensitive).
3. Timestamp is within 10 minutes.

**Response -- approved:**

```http
HTTP/1.1 200 OK
Content-Type: text/html

<!-- "Authorization successful! You can close this window and return to your device." -->
```

**Response -- denied:**

```http
HTTP/1.1 200 OK
Content-Type: text/html

<!-- "Authorization denied. You can close this window." -->
```

### 5.3 Device Token Polling

While the user authorizes on the browser, the device polls the token endpoint.

**Request:**

```http
POST /oauth/token HTTP/1.1
Host: documcp.example.com
Content-Type: application/x-www-form-urlencoded

grant_type=urn%3Aietf%3Aparams%3Aoauth%3Agrant-type%3Adevice_code&device_code=55%7CdEvIcEcOdE...64chars...&client_id=660e8400-e29b-41d4-a716-446655440001
```

For confidential clients, add client authentication (HTTP Basic or `client_secret` in the body).

#### Token Request Validation Rules (device_code)

| Parameter | Rules |
|-----------|-------|
| `grant_type` | Required; `urn:ietf:params:oauth:grant-type:device_code` |
| `client_id` | Required (body or HTTP Basic) |
| `client_secret` | Required for confidential clients (body or HTTP Basic) |
| `device_code` | Required |

A `resource` parameter on this request is ignored. The token uses the `resource` sent to `/oauth/device/code`.

#### Polling Response: Authorization Pending

User has not yet authorized or denied.

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "authorization_pending",
  "error_description": "The authorization request is still pending"
}
```

The device should wait `interval` seconds before polling again.

#### Polling Response: Slow Down

Device is polling faster than the allowed `interval`.

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "slow_down",
  "error_description": "Polling too fast. Increase interval to 10 seconds"
}
```

The server increases the `interval` by 5 seconds on each `slow_down` response, up to 300 seconds. The device must use the new interval.

#### Polling Response: Access Denied

User denied the authorization request.

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "access_denied",
  "error_description": "The user denied the authorization request"
}
```

The device should stop polling and inform the user.

#### Polling Response: Expired Token

Device code has expired (after 600 seconds).

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "expired_token",
  "error_description": "The device code has expired"
}
```

The device must restart the flow from Step 5.1.

#### Polling Response: Success

User has authorized the device.

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "access_token": "201|aCcEsStOkEn...64chars...",
  "token_type": "Bearer",
  "expires_in": 3600,
  "refresh_token": "202|rEfReShToKeN...64chars...",
  "scope": "mcp:access mcp:read mcp:write"
}
```

The device code is marked as `exchanged` and cannot be reused.

---

## 6. Token Revocation (RFC 7009)

Tokens can be revoked by the client. Per RFC 7009, the endpoint returns 200 OK even if the token was not found (to prevent token scanning attacks). The one exception is failed client authentication, which returns `401 invalid_client`.

The endpoint accepts a JSON body (shown below) or `application/x-www-form-urlencoded`. Client credentials must be in the body; HTTP Basic is not read here.

**Request (access token):**

```http
POST /oauth/revoke HTTP/1.1
Host: documcp.example.com
Content-Type: application/json

{
  "token": "99|xYzAbC...64chars...",
  "client_id": "660e8400-e29b-41d4-a716-446655440001",
  "token_type_hint": "access_token"
}
```

**Request (refresh token, confidential client):**

```http
POST /oauth/revoke HTTP/1.1
Host: documcp.example.com
Content-Type: application/json

{
  "token": "100|pQrStU...64chars...",
  "client_id": "550e8400-e29b-41d4-a716-446655440000",
  "client_secret": "a1b2c3d4...64_random_chars...z9y8x7w6",
  "token_type_hint": "refresh_token"
}
```

**Response (success, token found and revoked):**

```http
HTTP/1.1 200 OK
Content-Type: application/json

[]
```

**Response (success, token not found -- identical per RFC 7009):**

```http
HTTP/1.1 200 OK
Content-Type: application/json

[]
```

#### Revocation Validation Rules

| Parameter | Rules |
|-----------|-------|
| `token` | Required, string |
| `client_id` | Required, string |
| `client_secret` | Required for confidential clients |
| `token_type_hint` | Optional; must be `access_token` or `refresh_token`. Without a hint, the server tries both. |

Revoking either half of a pair revokes the other: an access token takes its refresh token with it, and a refresh token takes its access token. A client can only revoke its own tokens.

**Response (client authentication failed):**

```http
HTTP/1.1 401 Unauthorized
Content-Type: application/json
WWW-Authenticate: Basic realm="oauth"

{
  "error": "invalid_client",
  "error_description": "Client authentication failed"
}
```

---

## 7. Error Cases

### 7.1 Missing PKCE

Every client, public or confidential, must send PKCE parameters to `/oauth/authorize`.

**Missing `code_challenge`:**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "invalid_request",
  "error_description": "PKCE code_challenge required"
}
```

**Missing `code_challenge_method`:**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "invalid_request",
  "error_description": "PKCE code_challenge_method required"
}
```

**Using `plain` method (only S256 is allowed):**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "invalid_request",
  "error_description": "The selected code challenge method is invalid."
}
```

### 7.2 Invalid Redirect URI

The `redirect_uri` must exactly match a registered URI. For localhost/loopback URIs (`localhost`, `127.0.0.1`, `[::1]`), any port is allowed per RFC 8252 Section 7.3, but the scheme and path must match.

**Non-matching URI:**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "invalid_request",
  "error_description": "Invalid redirect_uri"
}
```

**Localhost port flexibility examples:**

| Registered URI | Request URI | Result |
|---------------|-------------|--------|
| `http://localhost/callback` | `http://localhost:8080/callback` | Allowed |
| `http://127.0.0.1/callback` | `http://127.0.0.1:54321/callback` | Allowed |
| `http://[::1]/callback` | `http://[::1]:12345/callback` | Allowed |
| `http://localhost/callback` | `https://localhost/callback` | Rejected (scheme mismatch) |
| `http://localhost/callback` | `http://localhost:8080/other-path` | Rejected (path mismatch) |
| `https://example.com:443/callback` | `https://example.com:8443/callback` | Rejected (non-loopback) |

### 7.3 Invalid or Expired Authorization Code

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "invalid_grant",
  "error_description": "The authorization code is invalid, expired, or has already been used"
}
```

The server returns this same response for every failed `authorization_code` exchange so the client learns nothing about the cause. This includes invalid, expired, or reused codes, a `redirect_uri` mismatch, a `resource` mismatch, an unknown `client_id`, and the cases in 7.4 to 7.6. The specific reason is logged server-side.

### 7.4 Invalid PKCE Code Verifier

When the `code_verifier` is missing or does not match the `code_challenge` stored with the authorization code, the server returns the `invalid_grant` response from 7.3.

### 7.5 PKCE Downgrade Attack Prevention (RFC 9700)

If a `code_verifier` is submitted during token exchange but the authorization code has no stored `code_challenge`, the server rejects the request with the `invalid_grant` response from 7.3. This prevents an attacker from intercepting an authorization code and exchanging it with their own PKCE verifier.

### 7.6 Invalid Client Credentials

At the token endpoint, an incorrect `client_secret` for a confidential client returns the `invalid_grant` response from 7.3 (`authorization_code`) or 7.10 (`refresh_token`). For the device-code grant it returns:

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "invalid_client",
  "error_description": "Invalid client credentials"
}
```

At `/oauth/revoke` it returns `401 invalid_client` (see section 6).

### 7.7 Invalid or Non-Existent Client

At `/oauth/authorize`:

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "invalid_client",
  "error_description": "Client not found or inactive"
}
```

### 7.8 Unsupported Grant Type

When `grant_type` is not one of the three supported values:

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "unsupported_grant_type",
  "error_description": "Grant type client_credentials is not supported"
}
```

When the grant type is supported but not in the client's registered `grant_types`:

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "unauthorized_client",
  "error_description": "This client is not authorized for the authorization_code grant type"
}
```

**Wrong `Content-Type` at the token endpoint:**

```http
HTTP/1.1 415 Unsupported Media Type
Content-Type: application/json
Accept: application/x-www-form-urlencoded

{
  "error": "invalid_request",
  "error_description": "The token endpoint requires Content-Type: application/x-www-form-urlencoded"
}
```

### 7.9 Missing Required Token Fields

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "invalid_request",
  "error_description": "The code field is required when grant type is authorization_code."
}
```

### 7.10 Expired or Revoked Refresh Token

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "invalid_grant",
  "error_description": "The refresh token is invalid, expired, or has been revoked"
}
```

The server returns this same response for every failed `refresh_token` request, including a `scope` wider than the original grant and a `resource` that differs from the original token.

### 7.11 Refresh Token Client Mismatch

When a refresh token is used with a different client than the one that originally obtained it, the server returns the `invalid_grant` response from 7.10.

### 7.12 Consent Session Errors

**No pending request in session (stale tab):**

```http
HTTP/1.1 400 Bad Request

No pending OAuth request. Please restart the authorization flow.
```

**Nonce mismatch (potential TOCTOU attack):**

```http
HTTP/1.1 400 Bad Request

Invalid authorization request. Please restart the authorization flow.
```

**Client ID mismatch (multiple tabs open):**

```http
HTTP/1.1 400 Bad Request

OAuth request mismatch. This may happen if you have multiple authorization tabs open. Please close all tabs and try again.
```

**State mismatch:**

```http
HTTP/1.1 400 Bad Request

OAuth state mismatch. Please restart the authorization flow.
```

**Request expired (older than 10 minutes):**

```http
HTTP/1.1 400 Bad Request

OAuth request expired. Please restart the authorization flow.
```

**Invalid state parameter format (injection prevention):**

```http
HTTP/1.1 400 Bad Request

Invalid state parameter format
```

State parameters must use only safe characters (`[a-zA-Z0-9._~()'-]`) and be at most 500 characters.

### 7.13 Device Flow Errors

**Invalid or inactive client (`/oauth/device/code`):**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "invalid_client",
  "error_description": "invalid or inactive client"
}
```

**Client does not support device_code grant (`/oauth/device/code`):**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "invalid_client",
  "error_description": "client does not support the requested grant type"
}
```

**Unknown client at the token endpoint (device-code grant):**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "invalid_client",
  "error_description": "Invalid client"
}
```

**Invalid device code:**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "invalid_grant",
  "error_description": "Invalid device code"
}
```

**Device code already exchanged (one-time use):**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "invalid_grant",
  "error_description": "Device code has already been used"
}
```

**Wrong client for device code:**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "invalid_grant",
  "error_description": "Invalid device code"
}
```

### 7.14 Rate Limiting

When rate limits are exceeded, the server returns:

```http
HTTP/1.1 429 Too Many Requests
Content-Type: application/json
Retry-After: <seconds>

{
  "error": "Too Many Requests",
  "message": "Rate limit exceeded. See the Retry-After header for when to retry."
}
```

### 7.15 Resource and Scope Errors

**`resource` not in `OAUTH_ALLOWED_RESOURCES` (`/oauth/authorize`, `/oauth/device/code`):**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "invalid_target",
  "error_description": "The requested resource is not recognized."
}
```

**No requested scope is within the user's consent ceiling (`/oauth/authorize`):**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "invalid_scope",
  "error_description": "None of the requested scopes are available to your account."
}
```

In the device flow, the same condition renders an HTML error page on `POST /oauth/device` instead.

---

## Appendix A: Token Format

All bearer tokens (access tokens, refresh tokens, authorization codes, device codes) use the format:

```
{database_id}|{64_character_random_string}
```

- The `database_id` prefix enables O(1) lookup by primary key.
- The random portion uses `[A-Za-z0-9]`.
- Only an HMAC-SHA256 of the 64-character random string is stored, prefixed with the key version (`v<version>$<hex>`). The HMAC key is derived from `OAUTH_SESSION_SECRET`. During a key rotation, tokens hashed under `OAUTH_SESSION_SECRET_PREVIOUS` still verify.
- The full plain-text token is returned to the client exactly once (at creation time).

## Appendix B: PKCE Reference

```
code_verifier:  43-128 characters from unreserved URI character set (RFC 7636)
                [A-Z] [a-z] [0-9] - . _ ~

code_challenge: BASE64URL(SHA256(ASCII(code_verifier)))   -- base64url, no padding

Verification:   constant-time compare of stored_challenge and
                BASE64URL(SHA256(submitted_verifier))
```

The server does not check the verifier's length or character set; it only checks that the computed challenge matches.

Only S256 is supported. The `plain` method is rejected.

## Appendix C: Source File Reference

| Component | Path |
|-----------|------|
| OAuth handler (shared) | `internal/handler/oauth/handler.go` |
| Authorize handler | `internal/handler/oauth/authorize.go` |
| Token handler | `internal/handler/oauth/token.go` |
| Revoke handler | `internal/handler/oauth/revoke.go` |
| Registration handler | `internal/handler/oauth/register.go` |
| Device flow handler | `internal/handler/oauth/device_flow.go` |
| Discovery metadata (RFC 8414, RFC 9728) | `internal/handler/oauth/wellknown.go` |
| Consent and device HTML templates | `internal/handler/oauth/templates.go` |
| OAuth service, token issuance, client auth | `internal/auth/oauth/service.go` |
| Client registration | `internal/auth/oauth/client_registration.go` |
| Authorization code issue and exchange | `internal/auth/oauth/authorization_code.go` |
| Refresh token rotation | `internal/auth/oauth/token_refresh.go` |
| Token revocation | `internal/auth/oauth/token_revocation.go` |
| Device authorization, approval, polling | `internal/auth/oauth/device_authorization.go` |
| Device user code generation | `internal/auth/oauth/device.go` |
| Device verification failure limiter | `internal/auth/oauth/device_failures.go` |
| Token generation and hashing | `internal/auth/oauth/token.go` |
| PKCE validation | `internal/auth/oauth/pkce.go` |
| Resource indicator validation (RFC 8707) | `internal/auth/oauth/resource.go` |
| Redirect URI validation | `internal/auth/oauth/redirect.go` |
| Scope definitions and consent ceilings | `internal/auth/scope/scope.go` |
| Bearer, audience, and scope middleware | `internal/auth/middleware/middleware.go` |
| OAuth configuration | `internal/config/config.go` (OAuthConfig) |
| Routes | `internal/server/routes.go` |
