# OAuth 2.1 Client Integration Guide

This guide explains how to integrate with DocuMCP's OAuth 2.1 Authorization Server to obtain access tokens for the MCP endpoint.

## Overview

DocuMCP implements OAuth 2.1 with:
- **RFC 7591** - Dynamic Client Registration
- **RFC 7636** - Proof Key for Code Exchange (PKCE, S256 required for public clients)
- **RFC 7009** - Token Revocation
- **RFC 8628** - Device Authorization Grant (for CLI tools)
- **RFC 8414** - OAuth Authorization Server Metadata Discovery
- **RFC 9728** - Protected Resource Metadata (automatic auth server discovery)

## Quick Start

### 1. Register Your Client

```bash
curl -X POST http://localhost:8080/oauth/register \
  -H "Content-Type: application/json" \
  -d '{
    "client_name": "My MCP Client",
    "redirect_uris": ["http://localhost:3000/callback"],
    "grant_types": ["authorization_code", "refresh_token"],
    "response_types": ["code"],
    "token_endpoint_auth_method": "none",
    "scope": "mcp:access documents:read search:read"
  }'
```

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
  "scope": "mcp:access documents:read search:read"
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

Redirect user to:

```
http://localhost:8080/oauth/authorize?
  response_type=code&
  client_id=YOUR_CLIENT_ID&
  redirect_uri=http://localhost:3000/callback&
  scope=mcp:access+documents:read+search:read&
  code_challenge=YOUR_CODE_CHALLENGE&
  code_challenge_method=S256&
  state=RANDOM_STATE_VALUE
```

User logs in and approves access. DocuMCP redirects to your callback:

```
http://localhost:3000/callback?
  code=AUTHORIZATION_CODE&
  state=YOUR_STATE_VALUE
```

### 4. Exchange Code for Tokens

```bash
curl -X POST http://localhost:8080/oauth/token \
  -H "Content-Type: application/json" \
  -d '{
    "grant_type": "authorization_code",
    "code": "AUTHORIZATION_CODE",
    "redirect_uri": "http://localhost:3000/callback",
    "client_id": "YOUR_CLIENT_ID",
    "code_verifier": "YOUR_CODE_VERIFIER"
  }'
```

**Response:**
```json
{
  "access_token": "64_CHARACTER_TOKEN",
  "token_type": "Bearer",
  "expires_in": 3600,
  "refresh_token": "64_CHARACTER_REFRESH_TOKEN",
  "scope": "mcp:access"
}
```

### 5. Use Token with MCP Endpoint

```bash
curl -X POST http://localhost:8080/documcp \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/list"
  }'
```

## Token Management

### Refresh Token

```bash
curl -X POST http://localhost:8080/oauth/token \
  -H "Content-Type: application/json" \
  -d '{
    "grant_type": "refresh_token",
    "refresh_token": "YOUR_REFRESH_TOKEN",
    "client_id": "YOUR_CLIENT_ID"
  }'
```

### Revoke Token

```bash
curl -X POST http://localhost:8080/oauth/revoke \
  -H "Content-Type: application/json" \
  -d '{
    "token": "YOUR_ACCESS_TOKEN",
    "client_id": "YOUR_CLIENT_ID"
  }'
```

## Device Authorization Grant (CLI Tools)

For CLI tools and devices without browsers, use RFC 8628 Device Authorization Grant. This avoids callback URL issues with dynamic port forwarding.

### 1. Request Device Code

```bash
curl -X POST http://localhost:8080/oauth/device/code \
  -H "Content-Type: application/json" \
  -d '{
    "client_id": "YOUR_CLIENT_ID",
    "scope": "mcp:access"
  }'
```

**Response:**
```json
{
  "device_code": "DEVICE_CODE",
  "user_code": "ABCD-EFGH",
  "verification_uri": "http://localhost:8080/device",
  "verification_uri_complete": "http://localhost:8080/device?user_code=ABCD-EFGH",
  "expires_in": 900,
  "interval": 5
}
```

### 2. User Authenticates

Direct user to `verification_uri_complete` or have them manually enter the `user_code` at `verification_uri`.

### 3. Poll for Token

Poll the token endpoint at the specified `interval` (minimum 5 seconds):

```bash
curl -X POST http://localhost:8080/oauth/token \
  -H "Content-Type: application/json" \
  -d '{
    "grant_type": "urn:ietf:params:oauth:grant-type:device_code",
    "device_code": "DEVICE_CODE",
    "client_id": "YOUR_CLIENT_ID"
  }'
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
  "access_token": "64_CHARACTER_TOKEN",
  "token_type": "Bearer",
  "expires_in": 3600,
  "refresh_token": "64_CHARACTER_REFRESH_TOKEN",
  "scope": "mcp:access"
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
# Register client for device flow
claude mcp add documcp -- npx -y mcp-remote http://localhost:8080/documcp 3334 --allow-http
```

The fixed port 3334 avoids VS Code port forwarding issues.

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

Default scopes for new registrations: `mcp:access documents:read search:read zim:read templates:read services:read`

## MCP Tools Available

After authentication, you can use these tools:

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
      "include_content": true,
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

1. **Always use PKCE with S256** - Required for public clients, plain method rejected
2. **Validate state parameter** - Prevent CSRF attacks
3. **Store tokens securely** - Never expose in URLs or logs
4. **Implement token rotation** - Use refresh tokens to get new access tokens
5. **Revoke on logout** - Clean up tokens when user logs out
6. **Validate redirect URIs** - Only registered URIs are allowed
7. **Use Device Authorization Grant for CLI** - Avoids callback URL issues
8. **Respect rate limits** - Token endpoint: 30/min, registration: 10/hour

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

```json
{
  "error": "Invalid or expired token"
}
```

Or JSON-RPC errors:
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
    this.baseUrl = 'http://localhost:8080';
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
      `scope=mcp:access+documents:read+search:read&` +
      `code_challenge=${challenge}&` +
      `code_challenge_method=S256&` +
      `state=${state}`;

    window.location.href = url;
  }

  async handleCallback(code, state) {
    // Verify state
    if (state !== sessionStorage.getItem('oauth_state')) {
      throw new Error('State mismatch');
    }

    const verifier = sessionStorage.getItem('pkce_verifier');

    const response = await fetch(`${this.baseUrl}/oauth/token`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        grant_type: 'authorization_code',
        code,
        redirect_uri: this.redirectUri,
        client_id: this.clientId,
        code_verifier: verifier
      })
    });

    return response.json();
  }

  async callTool(accessToken, toolName, args) {
    const response = await fetch(`${this.baseUrl}/documcp`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${accessToken}`,
        'Content-Type': 'application/json'
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

## Performance Expectations

Based on performance testing:

| Operation | Average Time |
|-----------|-------------|
| OAuth client registration | ~21ms |
| Token exchange | ~20-25ms |
| tools/list | ~16ms |
| search_documents | ~29ms |
| read_document | ~18ms |

All operations complete well under 500ms, suitable for real-time MCP integrations.

## Rate Limits

| Endpoint | Limit |
|----------|-------|
| Token (`/oauth/token`) | 30/min, 100/hour |
| Registration (`/oauth/register`) | 10/hour, 50/day |
| Authorization (`/oauth/authorize`) | 30/min |
| Device Authorization (`/oauth/device/code`) | 30/min |
| Device Verification (`/device`) | 5/min, 30/hour |

## Support

- MCP Protocol: [Model Context Protocol](https://modelcontextprotocol.io)
- OAuth 2.1: [RFC 6749](https://datatracker.ietf.org/doc/html/rfc6749)
- PKCE: [RFC 7636](https://datatracker.ietf.org/doc/html/rfc7636)
- Dynamic Registration: [RFC 7591](https://datatracker.ietf.org/doc/html/rfc7591)
- Device Authorization: [RFC 8628](https://datatracker.ietf.org/doc/html/rfc8628)
- Server Metadata: [RFC 8414](https://datatracker.ietf.org/doc/html/rfc8414)
- Protected Resource Metadata: [RFC 9728](https://datatracker.ietf.org/doc/html/rfc9728)
