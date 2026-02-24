# DocuMCP Integration Testing Guide

## Overview

This guide documents the comprehensive integration testing framework for DocuMCP, covering OAuth 2.1 authentication flows, MCP protocol compliance, tool execution, and error handling scenarios.

**Current Status (v1.8.2)**:
- 4,639+ tests total
- ~97% code coverage
- PHPStan Level 9 compliance
- Pest PHP 4.x test framework

## Test Suite Structure

```
tests/Integration/
├── MCP/
│   ├── OAuthFlowTest.php           # OAuth 2.1 + PKCE authentication flows
│   ├── McpToolExecutionTest.php    # MCP tool CRUD operations
│   ├── ProtocolComplianceTest.php  # JSON-RPC 2.0 compliance
│   └── McpErrorHandlingTest.php    # Error scenarios and edge cases
├── Auth/
│   └── OIDCConnectivityTest.php    # OIDC provider connectivity tests
├── DocumentUploadPipelineTest.php  # Document processing workflow
├── PerformanceRegressionTest.php   # Performance benchmarks
└── Support/
    ├── OAuthTestHelpers.php        # OAuth helper methods and factories
    └── McpTestHelpers.php          # MCP request and assertion helpers
```

## Quick Start

### Running All Integration Tests

```bash
# Run all MCP integration tests
./vendor/bin/pest tests/Integration/MCP/

# Run all integration tests in parallel
./vendor/bin/pest tests/Integration/ --parallel

# Run with coverage
XDEBUG_MODE=coverage ./vendor/bin/pest tests/Integration/MCP/ --coverage

# Run specific test file
./vendor/bin/pest tests/Integration/MCP/OAuthFlowTest.php

# Run with verbose output
./vendor/bin/pest tests/Integration/ --verbose
```

### Test Statistics

- **Total Tests**: 4,639+ (project-wide)
- **Integration Tests**: 100+ (MCP-specific)
- **Coverage**: ~97% overall, 70% minimum required
- **Categories**: OAuth Flow, Tool Execution, Protocol Compliance, Error Handling, OIDC Connectivity
- **Skipped Tests**: 2 (known limitations in Laravel MCP package)

## Test Categories

### 1. OAuth 2.1 Flow Tests (`OAuthFlowTest.php`)

Tests RFC 7591 (Dynamic Client Registration), RFC 7636 (PKCE), and OAuth 2.1 token lifecycle.

**Coverage:**
- Public and confidential client registration
- PKCE S256 challenge validation
- Authorization code exchange
- Token generation, revocation, and expiration
- Refresh token rotation
- MCP endpoint integration with OAuth tokens

**Example Test:**
```php
test('exchanges valid authorization code for tokens', function () {
    $authData = $this->createOAuthAuthorizationCode($this->user);

    $response = $this->postJson('/oauth/token', [
        'grant_type' => 'authorization_code',
        'code' => $authData['plain_code'],
        'redirect_uri' => 'http://localhost:8000/callback',
        'client_id' => $authData['client']->client_id,
        'code_verifier' => $authData['code_verifier'],
    ]);

    $response->assertOk();
    expect($tokens['token_type'])->toBe('Bearer');
    expect($tokens['scope'])->toBe('mcp:access');
});
```

### 2. MCP Tool Execution Tests (`McpToolExecutionTest.php`)

Tests the 16 MCP tools across four categories:

**Document Tools (5)**: `search_documents`, `read_document`, `create_document`, `update_document`, `delete_document`

**ZIM Tools (3)**: `list_zim_archives`, `search_zim`, `read_zim_article`

**Confluence Tools (3)**: `list_confluence_spaces`, `search_confluence`, `read_confluence_page`

**Git Template Tools (5)**: `list_git_templates`, `search_git_templates`, `get_template_structure`, `get_template_file`, `get_deployment_guide`

**Coverage:**
- Tool listing and schema validation
- CRUD operations on documents
- Authorization and ownership checks
- Input validation and error handling
- Concurrent operations

**Example Test:**
```php
test('denies access to other users private document', function () {
    $response = $this->mcpReadDocument($this->token, $this->otherUserDocument->uuid);

    $this->assertMcpToolError($response);
    $errorText = strtolower($response->content());
    expect($errorText)->toMatch('/not (authorized|found)|access denied|forbidden/');
});
```

### 3. Protocol Compliance Tests (`ProtocolComplianceTest.php`)

Validates JSON-RPC 2.0 and MCP specification compliance.

**Coverage:**
- JSON-RPC 2.0 message format (jsonrpc, id, result/error)
- Error codes (-32601 for method not found, etc.)
- Tool schema validation
- Content-Type headers
- Bearer token format
- Scope enforcement

**Example Test:**
```php
test('method not found returns -32601', function () {
    $response = $this->mcpRequest($this->token, [
        'jsonrpc' => '2.0',
        'id' => 1,
        'method' => 'nonexistent/method',
    ]);

    $data = $response->json();
    expect($data['error']['code'])->toBe(-32601);
});
```

### 4. Error Handling Tests (`McpErrorHandlingTest.php`)

Comprehensive error scenario coverage including security and boundary conditions.

**Coverage:**
- Authentication errors (malformed tokens, expired, revoked)
- Request payload errors (invalid JSON, missing fields)
- Tool execution errors (validation, authorization)
- Resource access errors (deleted documents, concurrent modification)
- Boundary conditions (empty queries, extreme limits)
- Security (SQL injection, XSS payloads)

**Example Test:**
```php
test('read_document with SQL injection attempt is safe', function () {
    $maliciousUuid = "'; DROP TABLE documents; --";

    $response = $this->mcpReadDocument($this->token, $maliciousUuid);

    $this->assertMcpToolError($response);
    $this->assertDatabaseHas('users', ['id' => $this->user->id]);
});
```

## Test Helpers

### OAuthTestHelpers Trait

```php
// Generate PKCE code verifier and challenge
$verifier = $this->generateCodeVerifier();
$challenge = $this->generateS256Challenge($verifier);

// Create OAuth access token (programmatic)
$tokenData = $this->createOAuthAccessToken($user);
$plainToken = $tokenData['plain_token'];

// Create authorization code with PKCE
$authData = $this->createOAuthAuthorizationCode($user);

// Create complete token set (access + refresh)
$tokenSet = $this->createOAuthTokenSet($user);

// Create expired or revoked tokens
$expired = $this->createExpiredOAuthAccessToken($user);
$revoked = $this->createRevokedOAuthAccessToken($user);

// Register OAuth client via RFC 7591
$clientData = $this->registerOAuthClient(['client_name' => 'My Client']);

// Exchange authorization code
$tokens = $this->exchangeAuthorizationCode([
    'code' => $plainCode,
    'client_id' => $clientId,
    'code_verifier' => $verifier,
    'redirect_uri' => 'http://localhost:8000/callback',
]);
```

### McpTestHelpers Trait

```php
// Make JSON-RPC 2.0 request
$response = $this->mcpRequest($token, [
    'jsonrpc' => '2.0',
    'id' => 1,
    'method' => 'tools/list',
]);

// List available tools
$response = $this->mcpListTools($token);

// Call specific tools
$response = $this->mcpSearchDocuments($token, 'query', ['limit' => 10]);
$response = $this->mcpReadDocument($token, $uuid);
$response = $this->mcpCreateDocument($token, ['title' => 'Doc', 'content' => '...', 'file_type' => 'markdown']);
$response = $this->mcpUpdateDocument($token, $uuid, ['title' => 'New Title']);
$response = $this->mcpDeleteDocument($token, $uuid);

// Assertions
$data = $this->assertMcpSuccess($response);     // Valid JSON-RPC success
$data = $this->assertMcpError($response);       // Valid JSON-RPC error
$data = $this->assertMcpToolSuccess($response); // Tool executed successfully
$data = $this->assertMcpToolError($response);   // Tool returned error (isError=true)

// Extract tool content
$content = $this->extractMcpToolContent($response);
```

## Running Tests in CI/CD

### Forgejo Actions

The CI/CD pipeline (`.forgejo/workflows/ci.yml`) includes:

1. **Static Analysis** - PHPStan Level 9 + Laravel Pint
2. **Unit Tests** - With coverage reporting (70% minimum)
3. **Integration Tests** - Separate job for MCP tests
4. **Architecture Tests** - Service-Action pattern validation
5. **Security Scan** - Vulnerability checks

### Devcontainer Integration Environment

The devcontainer provides a complete testing environment with all services:

```bash
# Services started automatically by devcontainer:
# - PostgreSQL 18 (localhost:5432)
# - Redis 8 (localhost:6379)
# - Meilisearch v1.33 (localhost:7700)
# - Mailhog (localhost:8025)

# Run all tests (uses SQLite in-memory, safe for development)
./vendor/bin/pest

# Run in parallel for faster execution
./vendor/bin/pest --parallel

# Run with coverage
XDEBUG_MODE=coverage ./vendor/bin/pest --coverage --min=70
```

## Known Limitations

1. **Null Request ID** - Laravel MCP package doesn't handle null IDs (JSON-RPC notification semantics)
2. **Refresh Token** - Requires client validation fix in RefreshAccessTokenAction
3. **OAuth Error Codes** - Controller returns 500 for business logic exceptions (should be 400 with specific error codes per RFC)

## Best Practices

### Writing New Tests

1. Use helper traits for consistent setup
2. Test both success and error paths
3. Validate response structure, not just status codes
4. Include ownership and authorization checks
5. Test boundary conditions (empty, max limits, invalid types)

### Test Organization

```php
describe('Feature Category', function () {
    test('specific behavior', function () {
        // Arrange
        $tokenData = $this->createOAuthAccessToken($this->user);

        // Act
        $response = $this->mcpListTools($tokenData['plain_token']);

        // Assert
        $this->assertMcpSuccess($response);
    });
});
```

### Security Testing

Always test:
- SQL injection attempts
- XSS payloads (for JSON APIs, ensure proper encoding)
- Authorization boundaries (can't access other users' private data)
- Token expiration and revocation
- Invalid input handling

## Coverage Requirements

| Component | Minimum | Target |
|-----------|---------|--------|
| Overall | 70% | 85% |
| Models | 85% | 95% |
| Controllers | 80% | 90% |
| Actions | 90% | 95% |
| Jobs | 75% | 85% |
| Services | 80% | 90% |

### Category Coverage Goals

- OAuth Flow: 100% of registration, authorization, token lifecycle
- MCP Tools: 100% of CRUD operations with authorization
- Protocol: 100% JSON-RPC 2.0 message format
- Error Handling: All major error categories covered

## Future Enhancements

1. **Browser Automation** - Playwright tests for full OAuth consent flow
2. **Load Testing** - k6 scripts for performance validation
3. **MCP Inspector** - Interactive protocol debugging
4. **Metrics Collection** - Custom MCP metrics for monitoring
5. **E2E Tests** - Complete user journey testing
