# OAuth 2.1 Client Integration Guide

This guide explains how to integrate with DocuMCP's OAuth 2.1 Authorization Server to obtain access tokens for the MCP endpoint.

## Overview

DocuMCP implements OAuth 2.1 with:
- **RFC 7591** - Dynamic Client Registration
- **RFC 7636** - Proof Key for Code Exchange (PKCE, S256, always required)
- **RFC 7009** - Token Revocation
- **RFC 8628** - Device Authorization Grant (for CLI tools)
- **RFC 8414** - OAuth Authorization Server Metadata Discovery
- **RFC 8707** - Resource Indicators (tokens are bound to the resource they were issued for)
- **RFC 9207** - `iss` parameter on every authorization response
- **RFC 9728** - Protected Resource Metadata (automatic auth server discovery)

Most MCP clients (Claude.ai, Claude Code, mcp-remote) do all of this for you once they are given the MCP endpoint URL. The manual steps below are for writing your own client or debugging one.

The examples use `https://documcp.example.com` as the server URL (`APP_URL`) and `https://documcp.example.com/documcp` as the MCP resource.

## Quick Start

### 1. Register Your Client

```bash
curl -X POST https://documcp.example.com/oauth/register \
  -H "Content-Type: application/json" \
  -d '{
    "client_name": "My MCP Client",
    "redirect_uris": ["http://localhost:3000/callback"],
    "grant_types": ["authorization_code", "refresh_token"],
    "response_types": ["code"],
    "token_endpoint_auth_method": "none"
  }'
```

Registration is controlled by two settings:

- `OAUTH_REGISTRATION_REQUIRE_AUTH=true` (the default): only a signed-in admin can register clients. The request must carry the admin panel's session cookie; without it the endpoint returns 401. Admins can request any scope and grant type.
- `OAUTH_REGISTRATION_REQUIRE_AUTH=false`: anyone can register, which is what Claude.ai and other clients that self-register need. These registrations are restricted to public clients (`token_endpoint_auth_method` is forced to `none`), get the default scopes whatever they ask for, and cannot use the device-code grant.

`OAUTH_REGISTRATION_ENABLED=false` turns the endpoint off entirely.

**Response:**
```json
{
  "client_id": "uuid-format-client-id",
  "client_id_issued_at": 1763407584,
  "client_name": "My MCP Client",
  "redirect_uris": ["http://localhost:3000/callback"],
  "grant_types": ["authorization_code", "refresh_token"],
  "response_types": ["code"],
  "token_endpoint_auth_method": "none",
  "scope": "mcp:access mcp:read documents:read search:read zim:read templates:read services:read"
}
```

Save the `client_id` for subsequent requests.

### 2. Generate PKCE Challenge

```bash
# Generate code verifier (43-128 characters, URL-safe)
code_verifier=$(openssl rand -base64 32 | tr -d /=+ | cut -c -43)

# Generate S256 challenge
code_challenge=$(echo -n "$code_verifier" | openssl dgst -sha256 -binary | base64 | tr '+/' '-_' | tr -d '=')

echo "Code Verifier: $code_verifier"
echo "Code Challenge: $code_challenge"
```

### 3. Authorization Request

Redirect the user to (line breaks added for readability):

```
https://documcp.example.com/oauth/authorize?
  response_type=code&
  client_id=YOUR_CLIENT_ID&
  redirect_uri=http://localhost:3000/callback&
  scope=mcp:access+mcp:read&
  resource=https://documcp.example.com/documcp&
  code_challenge=YOUR_CODE_CHALLENGE&
  code_challenge_method=S256&
  state=RANDOM_STATE_VALUE
```

- `scope`: `mcp:access` lets the token reach the MCP endpoint at all; each tool then also checks `mcp:read` (search, list, read) or `mcp:write` (create, update, replace, delete). A token with only `mcp:access` can list tools but every tool call fails with an insufficient-scope error. See [Available Scopes](#available-scopes).
- `resource`: the MCP endpoint URL ([RFC 8707](https://datatracker.ietf.org/doc/html/rfc8707)). The token is bound to it, and `/documcp` rejects tokens bound to anything else or to nothing. Operators can accept tokens without a resource for older clients with `OAUTH_ACCEPT_EMPTY_RESOURCE=true`; see [Configuration](CONFIGURATION.md#accepting-non-rfc-8707-clients-oauth_accept_empty_resource).

The user signs in and approves access. DocuMCP redirects to your callback:

```
http://localhost:3000/callback?
  code=AUTHORIZATION_CODE&
  state=YOUR_STATE_VALUE&
  iss=https://documcp.example.com
```

Check that `state` matches what you sent and that `iss` is the server you started with ([RFC 9207](https://datatracker.ietf.org/doc/html/rfc9207)). Error redirects carry `iss` too.

### 4. Exchange Code for Tokens

The token endpoint only accepts `application/x-www-form-urlencoded` bodies ([RFC 6749 §3.2](https://datatracker.ietf.org/doc/html/rfc6749#section-3.2)); JSON gets `415 Unsupported Media Type`.

```bash
curl -X POST https://documcp.example.com/oauth/token \
  -d grant_type=authorization_code \
  -d code=AUTHORIZATION_CODE \
  -d redirect_uri=http://localhost:3000/callback \
  -d client_id=YOUR_CLIENT_ID \
  -d code_verifier=YOUR_CODE_VERIFIER \
  -d resource=https://documcp.example.com/documcp
```

`resource` is optional here, but if you send it, it must match the one from the authorization request. Confidential clients authenticate with HTTP Basic (`-u CLIENT_ID:CLIENT_SECRET`) or with `client_secret` in the body, not both.

**Response:**
```json
{
  "access_token": "ACCESS_TOKEN",
  "token_type": "Bearer",
  "expires_in": 3600,
  "refresh_token": "REFRESH_TOKEN",
  "scope": "mcp:access mcp:read"
}
```

The granted `scope` can be narrower than the requested one: it is limited to what the approving user may delegate (see [Available Scopes](#available-scopes)).

### 5. Use Token with MCP Endpoint

```bash
curl -X POST https://documcp.example.com/documcp \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/list"
  }'
```

## Token Management

### Refresh Token

```bash
curl -X POST https://documcp.example.com/oauth/token \
  -d grant_type=refresh_token \
  -d refresh_token=YOUR_REFRESH_TOKEN \
  -d client_id=YOUR_CLIENT_ID
```

Refresh tokens rotate: each refresh returns a new one and invalidates the old. Presenting a refresh token (or authorization code) a second time is treated as theft, and every token descended from the same authorization is revoked.

### Revoke Token

The revocation endpoint accepts a form-encoded or JSON body.

```bash
curl -X POST https://documcp.example.com/oauth/revoke \
  -d token=YOUR_ACCESS_TOKEN \
  -d client_id=YOUR_CLIENT_ID
```

## Device Authorization Grant (CLI Tools)

For CLI tools and devices without browsers, use the RFC 8628 Device Authorization Grant. This avoids callback URL issues with dynamic port forwarding. The client must be registered with the `urn:ietf:params:oauth:grant-type:device_code` grant type, which requires an admin registration (see step 1).

### 1. Request Device Code

This endpoint accepts a form-encoded or JSON body.

```bash
curl -X POST https://documcp.example.com/oauth/device/code \
  -d client_id=YOUR_CLIENT_ID \
  -d "scope=mcp:access mcp:read" \
  -d resource=https://documcp.example.com/documcp
```

**Response:**
```json
{
  "device_code": "DEVICE_CODE",
  "user_code": "ABCD-EFGH",
  "verification_uri": "https://documcp.example.com/oauth/device",
  "verification_uri_complete": "https://documcp.example.com/oauth/device?user_code=ABCD-EFGH",
  "expires_in": 600,
  "interval": 5
}
```

`expires_in` follows `OAUTH_DEVICE_CODE_LIFETIME` (default 10 minutes).

### 2. User Authenticates

Direct the user to `verification_uri_complete`, or have them enter the `user_code` at `verification_uri`. Repeated wrong codes are rate limited per IP and per user.

### 3. Poll for Token

Poll the token endpoint at the specified `interval` (minimum 5 seconds):

```bash
curl -X POST https://documcp.example.com/oauth/token \
  -d grant_type=urn:ietf:params:oauth:grant-type:device_code \
  -d device_code=DEVICE_CODE \
  -d client_id=YOUR_CLIENT_ID
```

**Pending Response (keep polling):**
```json
{
  "error": "authorization_pending",
  "error_description": "The authorization request is still pending"
}
```

**Success Response:**
```json
{
  "access_token": "ACCESS_TOKEN",
  "token_type": "Bearer",
  "expires_in": 3600,
  "refresh_token": "REFRESH_TOKEN",
  "scope": "mcp:access mcp:read"
}
```

**Rate Limit Response (slow down polling):**
```json
{
  "error": "slow_down",
  "error_description": "You are polling too frequently"
}
```

### Example: Using with Claude Code

```bash
claude mcp add documcp -- npx -y mcp-remote https://documcp.example.com/documcp 3334
```

The fixed port 3334 avoids VS Code port forwarding issues. For a plain-HTTP development server, add `--allow-http`.

## Available Scopes

| Scope | Description |
|-------|-------------|
| `mcp:access` | MCP endpoint access |
| `mcp:read` | MCP read operations |
| `mcp:write` | MCP write operations |
| `documents:read` | Read documents |
| `documents:write` | Write/modify documents |
| `search:read` | Search functionality |
| `zim:read` | Read ZIM archives |
| `templates:read` | Read Git templates |
| `templates:write` | Write/modify templates |
| `services:read` | Read external services |
| `services:write` | Write/modify services |
| `admin` | Admin access |

`mcp:access` admits a request to the MCP endpoint. Inside it, every tool checks `mcp:read` or `mcp:write`. The other scopes gate the REST API only.

Default scopes for new registrations: `mcp:access mcp:read documents:read search:read zim:read templates:read services:read`

What a user can delegate to a client at consent time depends on who they are:

- **Non-admin users** can grant the default scopes plus `mcp:write`. Their clients can create documents and update, replace, or delete the documents that user owns; anyone else's documents return "document not found". REST write scopes stay admin-only.
- **Admins** can grant every scope except `admin` and `services:write`, which never leave the server.

A client never receives more than its registered scopes plus scopes a user has approved for it. Those approvals expire after `OAUTH_SCOPE_GRANT_TTL`.

## MCP Tools Available

After authentication, you can call the tools below. The first two need `mcp:read`; the rest need `mcp:write`. `tools/list` returns all 17 tools with their input schemas, and [docs/contracts/mcp-contract.json](contracts/mcp-contract.json) documents them.

### 1. search_documents

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "search_documents",
    "arguments": {
      "query": "OAuth security",
      "file_type": "markdown",
      "include_snippets": true,
      "limit": 10
    }
  }
}
```

### 2. read_document

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "read_document",
    "arguments": {
      "uuid": "document-uuid-here"
    }
  }
}
```

### 3. create_document

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "create_document",
    "arguments": {
      "title": "New Document",
      "content": "Document content here",
      "file_type": "markdown"
    }
  }
}
```

### 4. update_document

```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "tools/call",
  "params": {
    "name": "update_document",
    "arguments": {
      "uuid": "document-uuid-here",
      "description": "Updated description"
    }
  }
}
```

### 5. delete_document

```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "method": "tools/call",
  "params": {
    "name": "delete_document",
    "arguments": {
      "uuid": "document-uuid-here"
    }
  }
}
```

## Security Best Practices

1. **Always use PKCE with S256** - Required for every client; the `plain` method is rejected
2. **Validate state parameter** - Prevent CSRF attacks
3. **Store tokens securely** - Never expose in URLs or logs
4. **Implement token rotation** - Use refresh tokens to get new access tokens
5. **Revoke on logout** - Clean up tokens when user logs out
6. **Validate redirect URIs** - Only registered URIs are allowed
7. **Use Device Authorization Grant for CLI** - Avoids callback URL issues
8. **Respect rate limits** - See [Rate Limits](#rate-limits)

## Error Responses

### OAuth Errors

```json
{
  "error": "invalid_grant",
  "error_description": "The authorization code has expired"
}
```

Common errors:
- `invalid_client` - Client ID not recognized
- `invalid_grant` - Code expired or invalid PKCE verifier
- `invalid_request` - Missing required parameters

### MCP Errors

A missing, expired, or wrong-audience token gets `401` with a JSON body and a `WWW-Authenticate` header that points at the protected resource metadata:

```
WWW-Authenticate: Bearer resource_metadata="https://documcp.example.com/.well-known/oauth-protected-resource/documcp"
```

```json
{
  "error": "Unauthorized",
  "message": "Bearer token required"
}
```

A token that reaches the endpoint but lacks a tool's scope gets a normal tool result with `isError: true` and a message such as `mcp:read scope required for document search`. Protocol-level problems come back as JSON-RPC errors:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32601,
    "message": "Method not found"
  }
}
```

## Claude Code / Claude Desktop

Use [mcp-remote](https://www.npmjs.com/package/mcp-remote) to bridge stdio-based MCP clients to DocuMCP's HTTP endpoint:

### Claude Code

```bash
claude mcp add documcp -- npx -y mcp-remote https://documcp.example.com/documcp
```

### Claude Desktop

Add to your Claude Desktop configuration (`claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "documcp": {
      "command": "npx",
      "args": ["-y", "mcp-remote", "https://documcp.example.com/documcp"]
    }
  }
}
```

OAuth authorization happens automatically via browser popup on first connection.

## Example Client Implementation (JavaScript)

```javascript
class DocuMCPClient {
  constructor(clientId, redirectUri) {
    this.clientId = clientId;
    this.redirectUri = redirectUri;
    this.baseUrl = 'https://documcp.example.com';
    this.resource = `${this.baseUrl}/documcp`;
  }

  generateRandomString(length) {
    const array = new Uint8Array(length);
    crypto.getRandomValues(array);
    return btoa(String.fromCharCode(...array))
      .replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
      .slice(0, length);
  }

  async sha256Base64(value) {
    const data = new TextEncoder().encode(value);
    const hash = await crypto.subtle.digest('SHA-256', data);
    return btoa(String.fromCharCode(...new Uint8Array(hash)))
      .replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
  }

  async generatePKCE() {
    const verifier = this.generateRandomString(43);
    const challenge = await this.sha256Base64(verifier);
    return { verifier, challenge };
  }

  async authorize() {
    const { verifier, challenge } = await this.generatePKCE();
    const state = this.generateRandomString(32);

    // Store for later verification
    sessionStorage.setItem('pkce_verifier', verifier);
    sessionStorage.setItem('oauth_state', state);

    const url = `${this.baseUrl}/oauth/authorize?` +
      `response_type=code&` +
      `client_id=${this.clientId}&` +
      `redirect_uri=${encodeURIComponent(this.redirectUri)}&` +
      `scope=${encodeURIComponent('mcp:access mcp:read')}&` +
      `resource=${encodeURIComponent(this.resource)}&` +
      `code_challenge=${challenge}&` +
      `code_challenge_method=S256&` +
      `state=${state}`;

    window.location.href = url;
  }

  async handleCallback(code, state, iss) {
    // Verify state and issuer (RFC 9207)
    if (state !== sessionStorage.getItem('oauth_state')) {
      throw new Error('State mismatch');
    }
    if (iss !== this.baseUrl) {
      throw new Error('Issuer mismatch');
    }

    const verifier = sessionStorage.getItem('pkce_verifier');

    // The token endpoint only accepts form-encoded bodies.
    const response = await fetch(`${this.baseUrl}/oauth/token`, {
      method: 'POST',
      body: new URLSearchParams({
        grant_type: 'authorization_code',
        code,
        redirect_uri: this.redirectUri,
        client_id: this.clientId,
        code_verifier: verifier,
        resource: this.resource
      })
    });

    return response.json();
  }

  async callTool(accessToken, toolName, args) {
    const response = await fetch(`${this.baseUrl}/documcp`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${accessToken}`,
        'Content-Type': 'application/json',
        'Accept': 'application/json, text/event-stream'
      },
      body: JSON.stringify({
        jsonrpc: '2.0',
        id: Date.now(),
        method: 'tools/call',
        params: {
          name: toolName,
          arguments: args
        }
      })
    });

    return response.json();
  }
}
```

## Rate Limits

Per client IP:

| Endpoint | Limit |
|----------|-------|
| Token and revocation (`/oauth/token`, `/oauth/revoke`) | 30/min and 100/hour |
| Registration (`/oauth/register`) | 10/hour and 50/day |
| Authorization and consent (`/oauth/authorize*`) | 30/min |
| Device authorization (`/oauth/device/code`) | 30/min |
| Device verification form (`POST /oauth/device*`) | 10/min |

## Support

- MCP Protocol: [Model Context Protocol](https://modelcontextprotocol.io)
- OAuth 2.0: [RFC 6749](https://datatracker.ietf.org/doc/html/rfc6749)
- Resource Indicators: [RFC 8707](https://datatracker.ietf.org/doc/html/rfc8707)
- Authorization Server Issuer Identification: [RFC 9207](https://datatracker.ietf.org/doc/html/rfc9207)
- PKCE: [RFC 7636](https://datatracker.ietf.org/doc/html/rfc7636)
- Dynamic Registration: [RFC 7591](https://datatracker.ietf.org/doc/html/rfc7591)
- Device Authorization: [RFC 8628](https://datatracker.ietf.org/doc/html/rfc8628)
- Server Metadata: [RFC 8414](https://datatracker.ietf.org/doc/html/rfc8414)
- Protected Resource Metadata: [RFC 9728](https://datatracker.ietf.org/doc/html/rfc9728)
