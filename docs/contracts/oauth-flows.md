# OAuth 2.1 Flow Sequences

## Overview

DocuMCP implements an OAuth 2.1 authorization server with the following characteristics:

| Property | Value |
|----------|-------|
| Token format | `{token_id}\|{64_char_random_string}` (plain text); SHA-256 hash of random portion stored |
| Client secret hashing | bcrypt (human-chosen / low-entropy) |
| PKCE | Mandatory for public clients; S256 only (plain method removed) |
| Authorization code lifetime | 600 seconds (10 minutes) |
| Access token lifetime | 3600 seconds (1 hour) |
| Refresh token lifetime | 2592000 seconds (30 days) |
| Device code lifetime | 600 seconds (10 minutes) |
| Device polling interval | 5 seconds (doubles on slow_down) |
| Default scope | `mcp:access` |
| Available scopes | `mcp:access`, `mcp:read`, `mcp:write` |
| State parameter | Required, minimum 8 characters |
| Consent nonce | UUID v4, 10-minute expiry, prevents TOCTOU attacks |
| Localhost redirect | Any port allowed for `localhost`, `127.0.0.1`, `[::1]` (RFC 8252) |

### Rate Limits

| Limiter | Limits |
|---------|--------|
| `oauth-register` | 10/hour per IP, 50/day per IP |
| `oauth-token` | 30/minute per IP, 100/hour per IP |
| `oauth-authorize` | 30/minute per IP |
| `oauth-device` | 30/minute per IP |
| `oauth-device-verify` | 5/minute per IP, 30/hour per IP |

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
  "scopes_supported": ["mcp:access", "mcp:read", "mcp:write"],
  "protected_resources": ["https://documcp.example.com"]
}
```

**Error -- metadata disabled:**

```http
HTTP/1.1 404 Not Found
```

### 1.2 Protected Resource Metadata (RFC 9728)

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
  "scopes_supported": ["mcp:access", "mcp:read", "mcp:write"],
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

### 2.1 Register a Confidential Client

Confidential clients authenticate at the token endpoint using a client secret. The server returns `client_secret` only once during registration.

**Request:**

```http
POST /oauth/register HTTP/1.1
Host: documcp.example.com
Content-Type: application/json
Authorization: Bearer <admin-session-cookie>

{
  "client_name": "My Backend Service",
  "redirect_uris": ["https://myapp.example.com/callback"],
  "grant_types": ["authorization_code", "refresh_token"],
  "response_types": ["code"],
  "token_endpoint_auth_method": "client_secret_basic",
  "scope": "mcp:access",
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
  "client_secret_expires_at": 0,
  "client_name": "My Backend Service",
  "redirect_uris": ["https://myapp.example.com/callback"],
  "grant_types": ["authorization_code", "refresh_token"],
  "response_types": ["code"],
  "token_endpoint_auth_method": "client_secret_basic",
  "scope": "mcp:access"
}
```

### 2.2 Register a Public Client

Public clients set `token_endpoint_auth_method` to `"none"` and do not receive a `client_secret`. They must use PKCE (S256) for all authorization requests.

**Request:**

```http
POST /oauth/register HTTP/1.1
Host: documcp.example.com
Content-Type: application/json
Authorization: Bearer <admin-session-cookie>

{
  "client_name": "My MCP CLI Tool",
  "redirect_uris": ["http://localhost:3334/callback"],
  "grant_types": ["authorization_code", "refresh_token"],
  "response_types": ["code"],
  "token_endpoint_auth_method": "none",
  "scope": "mcp:access"
}
```

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
  "scope": "mcp:access"
}
```

Note: No `client_secret` or `client_secret_expires_at` fields in the response.

### 2.3 Registration Validation Rules

| Field | Rules |
|-------|-------|
| `client_name` | Required, string, max 255 |
| `redirect_uris` | Required, array, min 1 element |
| `redirect_uris.*` | Required, valid URL |
| `grant_types` | Nullable, array; each must be one of: `authorization_code`, `refresh_token`, `urn:ietf:params:oauth:grant-type:device_code` |
| `response_types` | Nullable, array; each must be: `code` |
| `scope` | Nullable, string |
| `token_endpoint_auth_method` | Nullable; one of: `none`, `client_secret_basic`, `client_secret_post` |
| `software_id` | Nullable, string, max 255 |
| `software_version` | Nullable, string, max 100 |

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
    &scope=mcp%3Aaccess
    &code_challenge=E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM
    &code_challenge_method=S256 HTTP/1.1
Host: documcp.example.com
Cookie: <session-cookie>
```

#### Authorization Request Validation Rules

| Parameter | Rules |
|-----------|-------|
| `response_type` | Required; must be `code` |
| `client_id` | Required, string |
| `redirect_uri` | Required, valid URL |
| `state` | Required, string, min 8 characters |
| `scope` | Nullable, string |
| `code_challenge` | Nullable, string (required for public clients) |
| `code_challenge_method` | Nullable; must be `S256` (required for public clients) |

**Response -- consent screen (user is authenticated):**

```http
HTTP/1.1 200 OK
Content-Type: text/html

<!-- Consent screen with client info, scopes, nonce hidden field -->
<form method="POST" action="/oauth/authorize/approve">
  <input type="hidden" name="client_id" value="660e8400-...">
  <input type="hidden" name="redirect_uri" value="http://localhost:3334/callback">
  <input type="hidden" name="state" value="xyzABC123_random_state">
  <input type="hidden" name="scope" value="mcp:access">
  <input type="hidden" name="code_challenge" value="E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM">
  <input type="hidden" name="code_challenge_method" value="S256">
  <input type="hidden" name="nonce" value="550e8400-e29b-41d4-a716-446655440000">
  ...
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
  "timestamp": 1700000000
}
```

**Response -- user not authenticated:**

```http
HTTP/1.1 302 Found
Location: /login?redirect=<full_authorize_url>
```

### 3.3 Authorization Approval

The user submits the consent form.

**Request:**

```http
POST /oauth/authorize/approve HTTP/1.1
Host: documcp.example.com
Content-Type: application/json
Cookie: <session-cookie>

{
  "client_id": "660e8400-e29b-41d4-a716-446655440001",
  "redirect_uri": "http://localhost:3334/callback",
  "scope": "mcp:access",
  "state": "xyzABC123_random_state",
  "code_challenge": "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM",
  "code_challenge_method": "S256",
  "nonce": "550e8400-e29b-41d4-a716-446655440000"
}
```

#### Approval Validation Rules

| Parameter | Rules |
|-----------|-------|
| `client_id` | Required, must exist in `oauth_clients` |
| `redirect_uri` | Required, valid URL |
| `scope` | Nullable, string |
| `state` | Nullable, string |
| `code_challenge` | Nullable, string |
| `code_challenge_method` | Nullable; must be `S256` |
| `nonce` | Required, string, valid UUID |

The server validates:
1. Pending request exists in session (prevents stale tab attacks).
2. Nonce matches (prevents TOCTOU race conditions).
3. `client_id` matches pending request.
4. `state` matches pending request.
5. Timestamp is within 10 minutes (rejects expired requests).
6. State parameter format is safe (alphanumeric + `._~()'-`, max 500 characters).

**Response -- success:**

```http
HTTP/1.1 302 Found
Location: http://localhost:3334/callback?code=42%7CaBcDeF...64chars...&state=xyzABC123_random_state
```

The authorization code is in `{id}|{64_char_random}` format, URL-encoded.

### 3.4 Token Exchange

The client exchanges the authorization code for tokens at the token endpoint.

**Request (public client with PKCE):**

```http
POST /oauth/token HTTP/1.1
Host: documcp.example.com
Content-Type: application/json

{
  "grant_type": "authorization_code",
  "code": "42|aBcDeF...64chars...",
  "client_id": "660e8400-e29b-41d4-a716-446655440001",
  "redirect_uri": "http://localhost:3334/callback",
  "code_verifier": "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk..."
}
```

**Request (confidential client):**

```http
POST /oauth/token HTTP/1.1
Host: documcp.example.com
Content-Type: application/json

{
  "grant_type": "authorization_code",
  "code": "42|aBcDeF...64chars...",
  "client_id": "550e8400-e29b-41d4-a716-446655440000",
  "client_secret": "a1b2c3d4...64_random_chars...z9y8x7w6",
  "redirect_uri": "https://myapp.example.com/callback",
  "code_verifier": "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk..."
}
```

**Response:**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "access_token": "99|xYzAbC...64chars...",
  "token_type": "Bearer",
  "expires_in": 3600,
  "refresh_token": "100|pQrStU...64chars...",
  "scope": "mcp:access"
}
```

The authorization code is revoked after successful exchange (one-time use).

#### Token Request Validation Rules (authorization_code)

| Parameter | Rules |
|-----------|-------|
| `grant_type` | Required; must be `authorization_code` |
| `client_id` | Required, string |
| `client_secret` | Nullable, string (required for confidential clients) |
| `code` | Required (when `grant_type` is `authorization_code`), string |
| `redirect_uri` | Required (when `grant_type` is `authorization_code`), valid URL |
| `code_verifier` | Nullable, string (required when `code_challenge` was used) |

---

## 4. Refresh Token

The client exchanges a refresh token for a new access token and new refresh token. The old access token and old refresh token are both revoked (token rotation).

**Request:**

```http
POST /oauth/token HTTP/1.1
Host: documcp.example.com
Content-Type: application/json

{
  "grant_type": "refresh_token",
  "refresh_token": "100|pQrStU...64chars...",
  "client_id": "660e8400-e29b-41d4-a716-446655440001"
}
```

For confidential clients, include `client_secret`:

```http
POST /oauth/token HTTP/1.1
Host: documcp.example.com
Content-Type: application/json

{
  "grant_type": "refresh_token",
  "refresh_token": "100|pQrStU...64chars...",
  "client_id": "550e8400-e29b-41d4-a716-446655440000",
  "client_secret": "a1b2c3d4...64_random_chars...z9y8x7w6",
  "scope": "mcp:access"
}
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
  "scope": "mcp:access"
}
```

#### Token Request Validation Rules (refresh_token)

| Parameter | Rules |
|-----------|-------|
| `grant_type` | Required; must be `refresh_token` |
| `client_id` | Required, string |
| `client_secret` | Nullable, string (required for confidential clients) |
| `refresh_token` | Required (when `grant_type` is `refresh_token`), string |
| `scope` | Nullable, string (cannot exceed original scope) |

---

## 5. Device Authorization (RFC 8628)

The device authorization flow is designed for input-constrained devices (CLI tools, smart displays) that cannot easily handle browser redirects or type long URLs. This eliminates the callback issues commonly seen with mcp-remote.

### 5.1 Device Authorization Request

The device requests authorization by providing its `client_id`. The client must have `urn:ietf:params:oauth:grant-type:device_code` in its `grant_types`.

**Request:**

```http
POST /oauth/device/code HTTP/1.1
Host: documcp.example.com
Content-Type: application/json

{
  "client_id": "660e8400-e29b-41d4-a716-446655440001",
  "scope": "mcp:access"
}
```

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
| `scope` | Nullable, string |

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
Location: /login?redirect=/oauth/device?user_code=BCDF-GHJK
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
| `user_code` | Required, string, max 9 |
| `approve` | Required; must be `approve` or `deny` |

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
Content-Type: application/json

{
  "grant_type": "urn:ietf:params:oauth:grant-type:device_code",
  "device_code": "55|dEvIcEcOdE...64chars...",
  "client_id": "660e8400-e29b-41d4-a716-446655440001"
}
```

For confidential clients, include `client_secret`.

#### Token Request Validation Rules (device_code)

| Parameter | Rules |
|-----------|-------|
| `grant_type` | Required; must be `urn:ietf:params:oauth:grant-type:device_code` |
| `client_id` | Required, string |
| `client_secret` | Nullable, string (required for confidential clients) |
| `device_code` | Required (when `grant_type` is `urn:ietf:params:oauth:grant-type:device_code`), string |

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

The server increases the `interval` by 5 seconds on each `slow_down` response. The device must use the new interval.

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
  "scope": "mcp:access"
}
```

The device code is marked as `exchanged` and cannot be reused.

---

## 6. Token Revocation (RFC 7009)

Tokens can be revoked by the client. Per RFC 7009, the endpoint always returns 200 OK even if the token was not found (to prevent token scanning attacks).

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
| `client_secret` | Nullable, string (required for confidential clients) |
| `token_type_hint` | Nullable; must be `access_token` or `refresh_token` |

When revoking a refresh token (with `token_type_hint: "refresh_token"`), the associated access token is also revoked.

---

## 7. Error Cases

### 7.1 Missing PKCE for Public Clients

Public clients (`token_endpoint_auth_method: "none"`) must provide PKCE parameters.

**Missing `code_challenge`:**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "invalid_request",
  "error_description": "PKCE code_challenge required for public clients"
}
```

**Missing `code_challenge_method`:**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "invalid_request",
  "error_description": "PKCE code_challenge_method required for public clients"
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

Confidential clients (`token_endpoint_auth_method: "client_secret_basic"` or `"client_secret_post"`) are not required to use PKCE but may do so optionally.

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
HTTP/1.1 500 Internal Server Error
Content-Type: application/json

{
  "error": "server_error",
  "error_description": "An internal error occurred while processing the token request"
}
```

Note: The server returns a generic `server_error` for all token exchange failures to prevent information leakage (RFC 9700). This includes invalid codes, expired codes, revoked codes, and redirect URI mismatches.

### 7.4 Invalid PKCE Code Verifier

When the `code_verifier` does not match the `code_challenge` stored with the authorization code:

```http
HTTP/1.1 500 Internal Server Error
Content-Type: application/json

{
  "error": "server_error",
  "error_description": "An internal error occurred while processing the token request"
}
```

### 7.5 PKCE Downgrade Attack Prevention (RFC 9700)

If a `code_verifier` is submitted during token exchange but the original authorization request did not include a `code_challenge`, the server rejects the request. This prevents an attacker from intercepting an authorization code and attempting to exchange it with their own PKCE verifier.

```http
HTTP/1.1 500 Internal Server Error
Content-Type: application/json

{
  "error": "server_error",
  "error_description": "An internal error occurred while processing the token request"
}
```

### 7.6 Invalid Client Credentials

For confidential clients, providing an incorrect `client_secret`:

```http
HTTP/1.1 500 Internal Server Error
Content-Type: application/json

{
  "error": "server_error",
  "error_description": "An internal error occurred while processing the token request"
}
```

### 7.7 Invalid or Non-Existent Client

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "invalid_client",
  "error_description": "Client not found or inactive"
}
```

### 7.8 Unsupported Grant Type

When the `grant_type` passes validation but is not implemented by the server:

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "unsupported_grant_type",
  "error_description": "Grant type client_credentials is not supported"
}
```

When the `grant_type` is not in the allowed list at all:

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "invalid_request",
  "error_description": "The selected grant type is invalid."
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
HTTP/1.1 500 Internal Server Error
Content-Type: application/json

{
  "error": "server_error",
  "error_description": "An internal error occurred while processing the token request"
}
```

### 7.11 Refresh Token Client Mismatch

When a refresh token is used with a different client than the one that originally obtained it:

```http
HTTP/1.1 500 Internal Server Error
Content-Type: application/json

{
  "error": "server_error",
  "error_description": "An internal error occurred while processing the token request"
}
```

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

**Invalid or inactive client:**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "invalid_client",
  "error_description": "Invalid or inactive client"
}
```

**Client does not support device_code grant:**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "invalid_client",
  "error_description": "Client does not support device_code grant type"
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
Retry-After: 60

{
  "message": "Too Many Attempts."
}
```

---

## Appendix A: Token Format

All bearer tokens (access tokens, refresh tokens, authorization codes, device codes) use the format:

```
{database_id}|{64_character_random_string}
```

- The `database_id` prefix enables O(1) lookup by primary key.
- Only the SHA-256 hash of the 64-character random string is stored in the database.
- The full plain-text token is returned to the client exactly once (at creation time).
- Legacy bcrypt-hashed tokens are supported during migration (detected by `$2y$` prefix).

## Appendix B: PKCE Reference

```
code_verifier:  43-128 characters from unreserved URI character set
                [A-Z] [a-z] [0-9] - . _ ~

code_challenge: BASE64URL(SHA256(ASCII(code_verifier)))
                = rtrim(strtr(base64_encode(hash('sha256', code_verifier, true)), '+/', '-_'), '=')

Verification:   hash_equals(stored_challenge, BASE64URL(SHA256(submitted_verifier)))
```

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
| OAuth actions (auth code, refresh, PKCE) | `internal/auth/oauth/actions.go` |
| Token generation and hashing | `internal/auth/oauth/token.go` |
| PKCE validation | `internal/auth/oauth/pkce.go` |
| Device code actions | `internal/auth/oauth/device.go` |
| Redirect URI validation | `internal/auth/oauth/redirect.go` |
| OAuth configuration | `internal/config/config.go` (OAuthConfig) |
| Routes | `internal/server/routes.go` |
