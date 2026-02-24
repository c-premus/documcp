# DocuMCP Local Testing Guide

This guide provides step-by-step instructions for testing DocuMCP in your local environment with real infrastructure.

## Prerequisites

Before starting, ensure you have:
- PHP 8.4+ with required extensions (bcmath, exif, gd, intl, opcache, pcntl, pdo_pgsql, pgsql, redis, zip, sockets)
- Composer 2.x
- Node.js 24+ and NPM
- Access to infrastructure services (PostgreSQL 18, Redis 8, Meilisearch v1.33)
- Xdebug (for coverage reports)

## Quick Start

### 1. Start the Application

```bash
# Install dependencies (if not already done)
composer install
npm install && npm run build

# Verify environment configuration
php artisan config:clear
php artisan cache:clear

# Run database migrations
php artisan migrate

# Start the development server
php artisan octane:start --host=0.0.0.0 --port=8000
```

Application available at: http://localhost:8000

### 2. Run Automated Tests

```bash
# Run all tests (uses SQLite in-memory, no production database access)
./vendor/bin/pest

# Run tests in parallel (3-5x faster)
./vendor/bin/pest --parallel

# Run with coverage (minimum 70% required)
XDEBUG_MODE=coverage ./vendor/bin/pest --coverage --min=70

# Run specific test groups
./vendor/bin/pest --group=arch        # Architecture tests
./vendor/bin/pest tests/Unit/         # Unit tests only
./vendor/bin/pest tests/Integration/  # Integration tests only
```

### 3. Start Queue Worker

```bash
# Start Horizon for queue processing
php artisan horizon

# Or simple queue worker
php artisan queue:work --tries=3 --timeout=60
```

Horizon dashboard: http://localhost:8000/horizon

## Testing Checklist

### Database Connectivity

```bash
php artisan tinker
>>> DB::connection()->getPdo()
>>> Schema::hasTable('documents')
>>> Schema::hasTable('users')
>>> Schema::hasTable('oauth_clients')
```

Expected: Connection object returned, all tables exist.

### Redis Connectivity

```bash
php artisan tinker
>>> Cache::put('test', 'value', 60)
>>> Cache::get('test')
>>> Cache::forget('test')
```

Expected: Returns 'value', no connection errors.

### Meilisearch Connectivity

```bash
php artisan tinker
>>> $client = app(\Meilisearch\Client::class)
>>> $client->health()
>>> $client->getIndexes()
```

Expected: Health status OK, indexes listed.

### OIDC Connectivity

```bash
php artisan tinker
>>> $url = config('documcp.auth.oidc.issuer')
>>> Http::get($url . '.well-known/openid-configuration')->json()
```

Expected: OIDC configuration with endpoints, issuer, supported grant types.

## User Authentication Testing

### OIDC SSO Login

1. Navigate to http://localhost:8000/login
2. Click "Sign in with OIDC" button
3. You'll be redirected to Authentik (auth.999.haus)
4. Enter your credentials
5. Authorize DocuMCP application
6. Redirected back to http://localhost:8000/auth/oidc/callback
7. Logged in and redirected to dashboard

**Expected Flow:**
- Login page shows OIDC button
- Authentik handles authentication
- Callback processes authorization code
- User record created/updated in database
- Session established

### Local Password Login

```bash
# Create test user
php artisan tinker
>>> User::factory()->create([
...   'email' => 'test@example.com',
...   'password' => bcrypt('password123'),
...   'is_admin' => true
... ])
```

Login with email: test@example.com, password: password123

## MCP Endpoint Testing

### Authentication

Obtain a Bearer token via:
1. OIDC flow (user login provides token)
2. OAuth 2.1 client flow (see OAuth testing section)

### Basic MCP Request

```bash
# List tools
curl -X POST http://localhost:8000/documcp \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/list"
  }'
```

Expected response (16 tools total):
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "tools": [
      {"name": "search_documents", "description": "...", "inputSchema": {...}},
      {"name": "read_document", "description": "...", "inputSchema": {...}},
      {"name": "create_document", "description": "...", "inputSchema": {...}},
      {"name": "update_document", "description": "...", "inputSchema": {...}},
      {"name": "delete_document", "description": "...", "inputSchema": {...}},
      {"name": "list_zim_archives", "description": "...", "inputSchema": {...}},
      {"name": "search_zim", "description": "...", "inputSchema": {...}},
      {"name": "read_zim_article", "description": "...", "inputSchema": {...}},
      {"name": "list_confluence_spaces", "description": "...", "inputSchema": {...}},
      {"name": "search_confluence", "description": "...", "inputSchema": {...}},
      {"name": "read_confluence_page", "description": "...", "inputSchema": {...}},
      {"name": "list_git_templates", "description": "...", "inputSchema": {...}},
      {"name": "search_git_templates", "description": "...", "inputSchema": {...}},
      {"name": "get_template_structure", "description": "...", "inputSchema": {...}},
      {"name": "get_template_file", "description": "...", "inputSchema": {...}},
      {"name": "get_deployment_guide", "description": "...", "inputSchema": {...}}
    ]
  }
}
```

### Search Documents

```bash
curl -X POST http://localhost:8000/documcp \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/call",
    "params": {
      "name": "search_documents",
      "arguments": {
        "query": "your search term",
        "include_content": true
      }
    }
  }'
```

### Create Document

```bash
curl -X POST http://localhost:8000/documcp \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 3,
    "method": "tools/call",
    "params": {
      "name": "create_document",
      "arguments": {
        "title": "Test Document",
        "content": "# Test Content\n\nThis is a test document.",
        "file_type": "markdown"
      }
    }
  }'
```

## OAuth 2.1 Server Testing

### Register OAuth Client

```bash
curl -X POST http://localhost:8000/oauth/register \
  -H "Content-Type: application/json" \
  -d '{
    "client_name": "Test Client",
    "redirect_uris": ["http://localhost:3000/callback"],
    "grant_types": ["authorization_code", "refresh_token"],
    "response_types": ["code"],
    "token_endpoint_auth_method": "none",
    "scope": "mcp:access"
  }'
```

Save the `client_id` from the response.

### Generate PKCE Challenge

```bash
# Generate code verifier (43-128 characters)
code_verifier=$(openssl rand -base64 32 | tr -d /=+ | cut -c -43)
echo "Code Verifier: $code_verifier"

# Generate S256 challenge
code_challenge=$(echo -n $code_verifier | openssl dgst -sha256 -binary | base64 | tr +/ -_ | tr -d =)
echo "Code Challenge: $code_challenge"
```

### Authorization Request

Open in browser:
```
http://localhost:8000/oauth/authorize?
  response_type=code
  &client_id=YOUR_CLIENT_ID
  &redirect_uri=http://localhost:3000/callback
  &scope=mcp:access
  &code_challenge=YOUR_CODE_CHALLENGE
  &code_challenge_method=S256
  &state=random_state_value
```

After consent, you'll receive an authorization code in the redirect.

### Exchange Code for Token

```bash
curl -X POST http://localhost:8000/oauth/token \
  -H "Content-Type: application/json" \
  -d '{
    "grant_type": "authorization_code",
    "code": "YOUR_AUTH_CODE",
    "redirect_uri": "http://localhost:3000/callback",
    "client_id": "YOUR_CLIENT_ID",
    "code_verifier": "YOUR_CODE_VERIFIER"
  }'
```

Response includes `access_token` and `refresh_token`.

## Document Management Testing

### Upload Document via Admin Panel

1. Login as admin user
2. Navigate to Documents section
3. Click "Upload Document"
4. Select a test file (PDF, DOCX, MD, HTML, XLSX)
5. Fill in metadata (title, description)
6. Submit

**Watch for:**
- File stored in `storage/app/documents/`
- Document record in database
- Queue job dispatched
- Text extraction completed
- Meilisearch index updated

### Verify in Database

```bash
php artisan tinker
>>> Document::latest()->first()
>>> Document::latest()->first()->extracted_text
```

### Verify in Meilisearch

```bash
php artisan tinker
>>> $client = app(\Meilisearch\Client::class)
>>> $index = $client->index('documents')
>>> $index->search('your term')
```

## Troubleshooting

### Common Issues

**Connection Refused to Redis**
```bash
# Check Redis is running
redis-cli -h bordertown ping
# Should return PONG
```

**Meilisearch Not Found**
```bash
# Test connectivity
curl http://bordertown:7700/health
```

**OIDC Callback Error**
- Verify `OIDC_REDIRECT_URI` matches Authentik configuration
- Check Laravel logs: `tail -f storage/logs/laravel.log`

**Queue Jobs Not Processing**
```bash
# Ensure Horizon is running
php artisan horizon

# Check failed jobs
php artisan queue:failed
```

**Database Connection Failed**
```bash
# Test PostgreSQL connection
psql -h bordertown -U docu-mcp -d docu-mcp
```

### Log Locations

- Laravel logs: `storage/logs/laravel.log`
- Horizon dashboard: http://localhost:8000/horizon
- Queue metrics: Horizon → Metrics tab

### Reset Environment

```bash
# Clear all caches
php artisan cache:clear
php artisan config:clear
php artisan route:clear
php artisan view:clear

# Reset database
php artisan migrate:fresh

# Reindex Meilisearch
php artisan scout:flush "App\Models\Document"
php artisan scout:import "App\Models\Document"
```

## Performance Monitoring

### Response Times

```bash
# Test MCP endpoint latency
time curl -s -X POST http://localhost:8000/documcp \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' > /dev/null

# Expected: < 500ms
```

### Queue Health

Monitor in Horizon dashboard:
- Jobs per minute
- Wait times
- Failed jobs
- Worker status

### Memory Usage

```bash
# Monitor PHP memory
php artisan tinker
>>> memory_get_usage(true) / 1024 / 1024
# Returns MB used
```

## Next Steps

After local testing is complete:

1. **Document findings** - Note any issues or edge cases
2. **Performance baseline** - Record response times
3. **Security review** - Verify authentication enforcement
4. **Production preparation** - Plan deployment checklist
5. **Monitoring setup** - Configure alerting and metrics
