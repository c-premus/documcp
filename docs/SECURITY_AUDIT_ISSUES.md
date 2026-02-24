# Security Audit Issues

Generated from comprehensive security assessment on 2026-01-20.

Assessment conducted by 4 specialized agents: Cybersecurity, Developer, Operations, Architecture.

---

## Issue Template Format

Each issue below includes:
- **Title**: Copy as issue title
- **Labels**: Suggested labels (create if needed)
- **Priority**: P0 (Critical) through P3 (Low)
- **Body**: Copy as issue description

---

## P0 - CRITICAL ISSUES

### ~~Issue 1: Client Secret Regeneration Not Hashed~~ (FIXED)

**Status:** FIXED

**Labels:** `security`, `bug`, `P0-critical`

**Title:** `[SECURITY] Client secret regeneration stores plaintext instead of bcrypt hash`

**Location:** `app/Services/OAuthService.php` (lines 144-163)

**Fix:** The `regenerateClientSecret()` method now uses `Hash::make()` to hash client secrets before storage, matching the pattern in `RegisterOAuthClientAction`.

```php
public function regenerateClientSecret(string $clientId): ?OAuthClient
{
    // Generate new secret and hash it before storing (security best practice)
    $plainTextSecret = Str::random(64);
    $client->update(['client_secret' => Hash::make($plainTextSecret)]);

    // Store the plain-text secret temporarily for one-time return
    $client->setAttribute('plain_text_secret', $plainTextSecret);

    return $client;
}
```

---

### Issue 2: OAuth Session State TOCTOU Vulnerability

**Labels:** `security`, `bug`, `P0-critical`

**Title:** `[SECURITY] OAuth authorization flow vulnerable to TOCTOU race condition in multi-tab scenario`

**Body:**
```markdown
## Summary
The OAuth authorization flow stores pending requests in session and validates with `session()->pull()`. This creates a time-of-check-time-of-use (TOCTOU) race condition allowing authorization code theft across browser tabs.

## Location
- **File:** `app/Http/Controllers/Auth/OAuthAuthorizationServerController.php`
- **Lines:** 284-293 (store), 360-405 (validate)

## Attack Scenario
1. User opens consent screen, session stores `oauth_pending_request`
2. User opens another tab, approves the same client → session is pulled and deleted
3. First tab still has the form, can reuse the same POST data
4. Attacker in first tab receives authorization code intended for second tab

## Current Code
```php
session()->put('oauth_pending_request', [
    'client_id' => $client->client_id,
    'state' => $state,
    'timestamp' => now()->timestamp,
]);
```

## Fix
Implement nonce-based validation:
```php
session()->put('oauth_pending_request', [
    'nonce' => Str::uuid()->toString(),  // Add unique nonce
    'client_id' => $client->client_id,
    'state' => $state,
    'timestamp' => now()->timestamp,
]);

// In approval form, include nonce as hidden field
// In authorizeApprove(), verify nonce matches before processing
```

## Impact
- **Severity:** CRITICAL
- Authorization code theft possible across browser tabs
- Violates OAuth 2.1 security requirements

## References
- RFC 6819 - OAuth 2.0 Threat Model
- OWASP Session Management Cheat Sheet
```

---

### ~~Issue 3: OIDC Clock Skew Tolerance Too Strict~~ (FIXED)

**Status:** FIXED

**Labels:** `security`, `bug`, `P0-critical`

**Title:** `[SECURITY] OIDC token validation clock skew tolerance too strict (60s vs 300s standard)`

**Location:** `app/Actions/OAuth/ValidateOIDCTokenAction.php` (lines 49-103)

**Fix:** Clock skew tolerance increased to 300 seconds (5 minutes) per JWT RFC 7519. The JWT library's `$leeway` is set, and explicit checks for `iat` and `nbf` claims use 300-second tolerance.

```php
// Clock skew tolerance: 5 minutes per JWT RFC 7519
// Set leeway before decoding so JWT library applies it to iat/nbf/exp checks
JWT::$leeway = 300;

// Decode and verify JWT using the parsed key set
$decoded = JWT::decode($dto->token, $keys);

// ... additional validation ...

// Validate issued-at time (not in future beyond clock skew)
if (isset($decoded->iat) && $decoded->iat > time() + 300) {
    throw new \Exception('Token issued in the future');
}

// Validate not-before claim (with clock skew tolerance)
if (isset($decoded->nbf) && $decoded->nbf > time() + 300) {
    throw new \Exception('Token not yet valid');
}
```

---

### Issue 4: No Emergency Token Revocation Capability

**Labels:** `security`, `operations`, `P0-critical`, `documentation`

**Title:** `[SECURITY] No emergency token revocation procedure for mass compromise scenarios`

**Body:**
```markdown
## Summary
There is no documented or implemented procedure for emergency mass token revocation if a security breach occurs (e.g., Redis breach, database compromise).

## Current State
- Individual tokens can be revoked via `RevokeTokenAction`
- Token pruning removes expired tokens
- No global revocation mechanism exists

## Attack Scenario
1. Attacker gains access to Redis/database
2. Attacker extracts all access/refresh tokens
3. No way to invalidate all tokens simultaneously
4. Attacker maintains access for up to 30 days (refresh token lifetime)

## Proposed Solution

### Option A: Revocation Epoch (Recommended)
Add a global revocation epoch to the system:
```php
// In config or database
'token_revocation_epoch' => 1704067200, // Unix timestamp

// In token validation
if ($token->created_at->timestamp < config('oauth.revocation_epoch')) {
    throw new \Exception('Token revoked by global policy');
}
```

### Option B: Emergency Revoke Command
```bash
php artisan oauth:revoke-all --confirm
# Marks all tokens as revoked
# Optionally: only tokens issued before a certain date
```

## Deliverables
1. Implement global revocation mechanism
2. Create `docs/INCIDENT_RESPONSE.md` with emergency procedures
3. Add Artisan command for emergency revocation
4. Document recovery process

## Impact
- **Severity:** CRITICAL (Operational)
- No way to respond to mass token compromise
- Attackers can maintain access for extended periods
```

---

## P1 - HIGH ISSUES

### ~~Issue 5: Device Code Verification Rate Limiting Insufficient~~ (FIXED)

**Status:** FIXED

**Labels:** `security`, `enhancement`, `P1-high`

**Title:** `[SECURITY] Device code verification allows brute force (10/min insufficient for 34.5-bit entropy)`

**Location:** `app/Providers/AppServiceProvider.php` (lines 82-87)

**Fix:** Device verification now uses layered rate limiting with stricter limits: 5 attempts per minute and 30 per hour. Combined with the 8-character base-20 user codes (34.5 bits entropy), brute force is impractical.

```php
// RFC 8628: Device verification page (stricter for brute force protection)
// Layered limits: 5/min + 30/hour to prevent user code enumeration
RateLimiter::for('oauth-device-verify', function (Request $request) {
    return [
        Limit::perMinute(5)->by($request->ip()),
        Limit::perHour(30)->by($request->ip()),
    ];
});
```

**Risk Mitigation:**
- 5 attempts/min = 300/hour max
- 30/hour hard cap prevents sustained attacks
- 8-char base-20 codes = 25.6 billion combinations
- At 30 attempts/hour: ~97 million years to enumerate

---

### Issue 6: Missing Soft-Delete Checks in DocumentPolicy

**Labels:** `security`, `bug`, `P1-high`

**Title:** `[SECURITY] DocumentPolicy doesn't check for soft-deleted documents`

**Body:**
```markdown
## Summary
`DocumentPolicy` doesn't verify if a document is soft-deleted before authorizing access. Users who owned a document before deletion may get inconsistent behavior.

## Location
- **File:** `app/Policies/DocumentPolicy.php`
- **All methods**

## Current Code
```php
public function view(?User $user, Document $document): Response
{
    if ($document->is_public) {
        return Response::allow();
    }
    // No check for $document->trashed()
}
```

## Expected Code
```php
public function view(?User $user, Document $document): Response
{
    if ($document->trashed()) {
        return Response::denyAsNotFound();
    }

    if ($document->is_public) {
        return Response::allow();
    }
    // ...
}
```

## Impact
- **Severity:** HIGH
- Soft-deleted documents may be accessible
- Inconsistent authorization behavior
- Information disclosure risk

## Fix
1. Add `trashed()` check to all policy methods
2. Add test coverage for soft-deleted document access
3. Verify admin bypass still works for restore operations
```

---

### Issue 7: OAuth State Parameter Not Validated in Redirect

**Labels:** `security`, `bug`, `P1-high`

**Title:** `[SECURITY] OAuth state parameter appended to redirect without format validation`

**Body:**
```markdown
## Summary
When redirecting after OAuth authorization approval, the state parameter is appended without validating its format, potentially enabling state injection attacks.

## Location
- **File:** `app/Http/Controllers/Auth/OAuthAuthorizationServerController.php`
- **Lines:** 437-443

## Current Code
```php
if ($submittedState) {
    $stateString = is_string($submittedState) ? $submittedState : '';
    $redirectUrl .= '&state=' . $stateString;
}
```

## Expected Code
```php
if ($submittedState) {
    $stateString = is_string($submittedState) ? $submittedState : '';

    // Validate state format
    if (strlen($stateString) > 500 || !preg_match('/^[a-zA-Z0-9._~()\'-]+$/', $stateString)) {
        abort(400, 'Invalid state parameter format');
    }

    $redirectUrl .= '&state=' . urlencode($stateString);
}
```

## Impact
- **Severity:** HIGH
- State parameter injection possible
- Could enable CSRF if combined with redirect URI bypass
```

---

### ~~Issue 8: OIDC Audience Array Format Not Validated~~ (FIXED)

**Status:** FIXED

**Labels:** `security`, `bug`, `P1-high`

**Title:** `[SECURITY] OIDC token audience claim array format not validated`

**Location:** `app/Actions/OAuth/ValidateOIDCTokenAction.php` (lines 73-92)

**Fix:** Audience validation now checks for empty arrays and validates each audience value for type, length, and format.

```php
$audiences = is_array($decoded->aud) ? $decoded->aud : [$decoded->aud];

// Validate audience format
if (empty($audiences)) {
    throw new \Exception('Empty audience claim');
}

foreach ($audiences as $aud) {
    if (!is_string($aud) || strlen($aud) === 0 || strlen($aud) > 255) {
        throw new \Exception('Invalid audience value format');
    }
}

if (! in_array($clientId, $audiences, true)) {
    throw new \Exception('Invalid audience. Expected: ' . $clientId);
}
```

---

### ~~Issue 9: No Audit Log for Admin Status Changes~~ (FIXED)

**Status:** FIXED

**Labels:** `security`, `operations`, `P1-high`

**Title:** `[SECURITY] Admin status changes via OIDC groups not logged`

**Location:** `app/Services/OAuthService.php` (lines 99-124)

**Fix:** The `getOrCreateUser()` method now tracks previous admin status and logs all changes with full context.

```php
// Query existing user to track admin status changes
$existingUser = User::where('oidc_sub', $tokenData->sub)->first();
$previousAdminStatus = $existingUser !== null ? $existingUser->is_admin : false;

// Use updateOrCreate to ensure admin status is re-evaluated on every login
$user = User::updateOrCreate(
    ['oidc_sub' => $tokenData->sub],
    [
        'name' => $tokenData->name ?? $tokenData->email,
        'email' => $tokenData->email,
        'oidc_provider' => $tokenData->iss,
        'is_admin' => $isAdmin,
    ]
);

// Audit log admin status changes (security-critical event)
if ($previousAdminStatus !== $isAdmin) {
    Log::warning('Admin status changed via OIDC login', [
        'user_id' => $user->id,
        'email' => $user->email,
        'oidc_sub' => $tokenData->sub,
        'previous_admin' => $previousAdminStatus,
        'new_admin' => $isAdmin,
        'oidc_groups' => $tokenData->groups ?? [],
    ]);
}
```

---

## P2 - MEDIUM ISSUES

### ~~Issue 10: No FormRequest Validation for OAuth Endpoints~~ (FIXED)

**Status:** FIXED

**Labels:** `code-quality`, `enhancement`, `P2-medium`

**Title:** `[CODE] OAuth endpoints use manual validation instead of FormRequest classes`

**Location:** `app/Http/Controllers/Auth/OAuthAuthorizationServerController.php`

**Fix:** The OAuth authorization server controller now uses existing FormRequest classes instead of inline `Validator::make()` calls. All 9 methods with manual validation were refactored to use type-hinted FormRequest classes:

- `register()` → `RegisterOAuthClientRequest`
- `authorize()` → `AuthorizeRequest`
- `token()` → `TokenRequest`
- `deviceAuthorization()` → `DeviceAuthorizationRequest`
- `revoke()` → `RevokeRequest`
- `introspect()` → `IntrospectRequest`
- `userinfo()` → Uses bearer token validation (no request body)
- `authorizeApprove()` → `AuthorizeApproveRequest`
- `deviceVerify()` → `DeviceVerifyRequest`

```php
// Before (manual validation)
$validator = Validator::make($request->all(), [
    'client_name' => 'required|string|max:255',
]);

// After (FormRequest)
public function register(RegisterOAuthClientRequest $request): JsonResponse
{
    $validated = $request->validated();
    // Type-safe, validated input
}
```

---

### Issue 11: Authorization Code One-Time Use Not DB-Enforced

**Labels:** `security`, `enhancement`, `P2-medium`

**Title:** `[SECURITY] Authorization code one-time use enforced in application code, not database`

**Body:**
```markdown
## Summary
Authorization code revocation is done in application code, creating a TOCTOU race window where two simultaneous requests could both redeem the same code.

## Location
- **File:** `app/Actions/OAuth/ExchangeAuthorizationCodeAction.php`
- **Line:** 122

## Current Code
```php
// Both requests could pass this check simultaneously
if ($authCode->revoked) {
    throw new \Exception('Authorization code already used');
}

// ... generate tokens ...

$authCode->update(['revoked' => true]);
```

## Fix
Add database-level constraint:
```php
// Migration
Schema::table('oauth_authorization_codes', function (Blueprint $table) {
    // Partial unique index: only one non-revoked code per code value
    $table->unique(['code', 'revoked'], 'oauth_codes_unique_active');
});

// Or use database locking
$authCode = OAuthAuthorizationCode::where('code_hash', $codeHash)
    ->lockForUpdate()
    ->first();
```

## Impact
- **Severity:** MEDIUM
- Race condition window for code replay
- Two tokens could be issued for same authorization
```

---

### Issue 12: Rate Limit Documentation Mismatch

**Labels:** `documentation`, `bug`, `P2-medium`

**Title:** `[DOCS] OAuth registration rate limits don't match documentation`

**Body:**
```markdown
## Summary
`.env.production.example` documents rate limits as "3/hr, 10/day" but code implements 10/hr, 50/day.

## Locations
- **Documentation:** `.env.production.example` line 234
- **Code:** `app/Providers/AppServiceProvider.php` line 65

## Documentation Says
```
# Intentional: rate-limited (3/hr, 10/day)
```

## Code Implements
```php
Limit::perHour(10)->by($request->ip()),
Limit::perDay(50)->by($request->ip()),
```

## Fix
Either:
1. Update documentation to match code (10/hr, 50/day)
2. Or update code to match documented limits (3/hr, 10/day)

## Impact
- **Severity:** MEDIUM
- Confusion about actual security posture
- Could allow 5x more registrations than expected
```

---

### Issue 13: Missing Authentication Failure Metrics

**Labels:** `observability`, `enhancement`, `P2-medium`

**Title:** `[OPS] No Prometheus metrics for authentication failures`

**Body:**
```markdown
## Summary
Failed OAuth/OIDC token validations don't increment Prometheus metrics, making it impossible to detect attack patterns or set up alerting.

## Location
- **File:** `app/Services/Metrics/PrometheusService.php`
- **Missing:** Auth failure counters

## Proposed Metrics
```php
// Add to PrometheusService
public function incrementAuthFailure(string $reason, string $client): void
{
    $this->incCounter(
        'documcp_auth_failures_total',
        'Authentication failures',
        ['reason' => $reason, 'client' => $client]
    );
}

// Usage in LaravelMcpAuth.php
if (!$user) {
    app(PrometheusService::class)->incrementAuthFailure('invalid_token', 'mcp');
    return null;
}
```

## Proposed Alert Rules
```yaml
- alert: AuthenticationFailureRateHigh
  expr: increase(documcp_auth_failures_total[5m]) > 20
  for: 1m
  labels:
    severity: warning
  annotations:
    summary: High authentication failure rate detected
```

## Impact
- **Severity:** MEDIUM
- Cannot detect brute force attacks
- No alerting on authentication anomalies
```

---

### Issue 14: Health Check Missing OIDC Provider Status

**Labels:** `observability`, `enhancement`, `P2-medium`

**Title:** `[OPS] Deep health check doesn't verify OIDC provider connectivity`

**Body:**
```markdown
## Summary
`/api/health/deep` checks database, Redis, and Meilisearch but not OIDC provider connectivity. If the OIDC provider is down, health check still returns "healthy".

## Location
- **File:** `app/Services/HealthCheckService.php`

## Proposed Fix
```php
private function checkOidcProvider(): array
{
    $start = microtime(true);

    try {
        $discoveryUrl = config('documcp.auth.oidc.discovery_url');
        $response = Http::timeout(5)->get($discoveryUrl);

        return [
            'status' => $response->successful() ? 'healthy' : 'unhealthy',
            'latency_ms' => round((microtime(true) - $start) * 1000, 2),
        ];
    } catch (\Exception $e) {
        return [
            'status' => 'unhealthy',
            'error' => 'OIDC provider unreachable',
        ];
    }
}
```

## Impact
- **Severity:** MEDIUM
- Load balancers won't detect auth system failures
- Users may experience login failures without operational awareness
```

---

### Issue 15: Device Code Polling Interval Not Bounded

**Labels:** `security`, `bug`, `P2-medium`

**Title:** `[SECURITY] Device code polling interval increases unboundedly on violations`

**Body:**
```markdown
## Summary
When a client polls too fast, the polling interval is incremented by 5 seconds with no maximum bound. Continuous violations could make the interval exceed the device code lifetime.

## Location
- **File:** `app/Models/OAuthDeviceCode.php`
- **Lines:** 182-192

## Current Code
```php
if ($tooFast) {
    $data['interval'] = $this->interval + 5;  // No max!
}
```

## Expected Code
```php
if ($tooFast) {
    $data['interval'] = min($this->interval + 5, 300);  // Cap at 5 minutes
}
```

## Impact
- **Severity:** MEDIUM
- Denial-of-service against device authorization flow
- Legitimate clients could become unable to complete flow
```

---

### Issue 16: SQL LIKE Wildcards Not Escaped in Search

**Labels:** `security`, `bug`, `P2-medium`

**Title:** `[SECURITY] LIKE wildcards not escaped in OAuth client search`

**Body:**
```markdown
## Summary
The OAuth client search doesn't escape LIKE wildcards (`%`, `_`), allowing users to craft search patterns that match unintended records.

## Location
- **File:** `app/Livewire/Admin/OAuthClientList.php`
- **Line:** 111

## Current Code
```php
->whereRaw('LOWER(client_name) LIKE ?', ['%' . strtolower($this->search) . '%'])
```

## Expected Code
```php
$escapedSearch = str_replace(
    ['%', '_', '\\'],
    ['\\%', '\\_', '\\\\'],
    strtolower($this->search)
);
->whereRaw('LOWER(client_name) LIKE ?', ['%' . $escapedSearch . '%'])
```

## Impact
- **Severity:** MEDIUM
- Information disclosure risk
- Users can craft search patterns matching unintended records
```

---

## P3 - LOW ISSUES

### Issue 17: Bearer Token Case Sensitivity

**Labels:** `enhancement`, `P3-low`

**Title:** `[ENHANCEMENT] Bearer token extraction is case-sensitive`

**Body:**
```markdown
## Summary
Laravel's `bearerToken()` method is case-sensitive. While RFC 6750 specifies lowercase "Bearer", robust implementations accept any case.

## Location
- **File:** `app/Http/Middleware/LaravelMcpAuth.php`
- **Line:** 32

## Impact
- **Severity:** LOW
- Minor UX issue for non-standard clients
- Not a security vulnerability
```

---

### Issue 18: Client Secret Expiration Not Validated

**Labels:** `enhancement`, `P3-low`

**Title:** `[ENHANCEMENT] Client secret expiration field not validated during authentication`

**Body:**
```markdown
## Summary
The `client_secret_expires_at` field exists but is not checked during client authentication. If an admin sets an expiry, it's ignored.

## Location
- **File:** `app/Actions/OAuth/ExchangeAuthorizationCodeAction.php`
- **Lines:** 37-44

## Impact
- **Severity:** LOW
- Only affects optional secret expiration feature
- Secrets don't expire by default (per OAuth 2.1)
```

---

### Issue 19: OIDC Code Exchange in Controller Instead of Action

**Labels:** `code-quality`, `P3-low`

**Title:** `[CODE] OIDC code exchange logic should be extracted to Action class`

**Body:**
```markdown
## Summary
The OIDC code-for-token exchange is implemented directly in `LoginController` instead of following the Action pattern used elsewhere.

## Location
- **File:** `app/Http/Controllers/Auth/LoginController.php`
- **Lines:** 217-254

## Expected
Create `app/Actions/OAuth/ExchangeOIDCCodeAction.php` to maintain consistency with the Service-Action pattern.

## Impact
- **Severity:** LOW
- Code consistency
- Testability improvement
```

---

### Issue 20: Prometheus Metrics Endpoint Publicly Accessible

**Labels:** `security`, `enhancement`, `P3-low`

**Title:** `[SECURITY] Prometheus metrics endpoint is publicly accessible`

**Body:**
```markdown
## Summary
The `/metrics` endpoint is accessible without authentication, potentially exposing infrastructure details.

## Location
- **File:** `routes/web.php`
- **Line:** 10

## Current State
Standard practice for Prometheus scraping, but could leak:
- Document upload counts
- Search patterns
- Infrastructure information

## Proposed Options
1. Restrict by IP (only Prometheus scraper)
2. Require bearer token
3. Document as intentional and acceptable risk

## Impact
- **Severity:** LOW
- Standard monitoring practice
- Minimal sensitive data exposure
```

---

## Summary Table

| Issue | Priority | Type | Status |
|-------|----------|------|--------|
| Client secret hashing | P0 | Bug | FIXED |
| OAuth session TOCTOU | P0 | Bug | Open |
| OIDC clock skew | P0 | Bug | FIXED |
| Emergency revocation | P0 | Feature | Open |
| Device code rate limit | P1 | Enhancement | FIXED |
| Soft-delete checks | P1 | Bug | Open |
| State parameter validation | P1 | Bug | Open |
| OIDC audience validation | P1 | Bug | FIXED |
| Admin audit logging | P1 | Enhancement | FIXED |
| FormRequest classes | P2 | Refactor | FIXED |
| Auth code DB constraint | P2 | Enhancement | Open |
| Rate limit docs | P2 | Docs | Open |
| Auth failure metrics | P2 | Enhancement | Open |
| OIDC health check | P2 | Enhancement | Open |
| Polling interval cap | P2 | Bug | Open |
| LIKE escaping | P2 | Bug | Open |
| Bearer case sensitivity | P3 | Enhancement | Open |
| Secret expiration | P3 | Enhancement | Open |
| OIDC code exchange action | P3 | Refactor | Open |
| Metrics endpoint | P3 | Enhancement | Open |

**Fixed in Security Hardening Phase (P0-P1):**
- Issue 1: Client secret hashing (bcrypt)
- Issue 3: OIDC clock skew (300s tolerance)
- Issue 5: Device code rate limiting (5/min, 30/hour)
- Issue 8: OIDC audience validation (format checks)
- Issue 9: Admin audit logging (OIDC group changes)

**Fixed in Code Quality Audit Phase (P2):**
- Issue 10: OAuth FormRequest classes (all 9 controller methods)

**Fixed in Security Hardening Phase (Additional):**
- Symlink traversal protection (Git Templates)
- MIME type validation (document uploads)
- Meilisearch filter injection prevention
- SSRF protection (Git URLs)
- ZIM path traversal prevention
- XXE hardening (Office documents)
- Token sanitization (Git errors)
- Path canonicalization (Git Templates)
- Variable substitution security

---

## Fixed Issues (Security Hardening Phase)

The following security issues were identified during audits and fixed in the security hardening phase (January 2026).

### Fixed 1: Symlink Traversal Protection

**Status:** FIXED

**Location:** `app/Services/GitTemplate/GitTemplateClient.php` (lines 191-199 in `extractFiles()`)

**Fix:** The `extractFiles()` method now skips symlinks using `is_link()` check before processing files. This prevents attackers from using symlinks in git templates to read arbitrary files outside the repository.

```php
// Security: Skip symlinks to prevent arbitrary file read attacks
if (is_link($file->getPathname())) {
    Log::warning('Skipping symlink in template', [
        'template_id' => $template->id,
        'path' => $file->getPathname(),
    ]);
    continue;
}
```

---

### Fixed 2: MIME Type Validation

**Status:** FIXED

**Location:** `app/Services/DocumentService.php` (lines 37-73 in `getUploadValidationRules()`)

**Fix:** The `getUploadValidationRules()` method now validates MIME types alongside file extensions. This prevents attackers from uploading malicious files by renaming them with allowed extensions.

```php
$mimeTypes = [
    'application/pdf',
    'application/msword',
    'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
    // ... additional MIME types
];

return [
    'file' => [
        'required',
        'file',
        "max:{$maxSize}",
        "extensions:{$extensions}",
        'mimetypes:' . implode(',', $mimeTypes),
    ],
];
```

---

### Fixed 3: Search Filter Injection

**Status:** FIXED

**Location:** `app/Actions/Search/SearchDocumentsAction.php` (lines 32-44 in `execute()`)

**Fix:** The `execute()` method now quotes filter values using `addslashes()` to prevent Meilisearch filter injection attacks.

```php
if ($dto->fileType) {
    // Security: Quote the value to prevent Meilisearch filter injection
    $filters[] = "file_type = '" . addslashes($dto->fileType) . "'";
}

if ($dto->tags !== null && $dto->tags !== []) {
    // Security: Quote tag values to prevent Meilisearch filter injection
    $tagFilters = array_map(
        fn ($tag) => "tags = '" . addslashes($tag) . "'",
        $dto->tags
    );
    $filters[] = '(' . implode(' OR ', $tagFilters) . ')';
}
```

---

### Fixed 4: SSRF Protection

**Status:** FIXED

**Location:** `app/Http/Requests/RegisterGitTemplateRequest.php` (lines 48-82 in `validateNotInternalUrl()`)

**Fix:** The `validateNotInternalUrl()` method validates that repository URLs do not point to internal or private IP addresses. This prevents Server-Side Request Forgery (SSRF) attacks.

```php
private function validateNotInternalUrl(string $url, Closure $fail): void
{
    // Block localhost variants
    if (in_array($host, ['localhost', '127.0.0.1', '::1', '0.0.0.0'], true)) {
        $fail('Repository URL cannot point to localhost.');
        return;
    }

    // Block .local domains
    if (str_ends_with($host, '.local') || str_ends_with($host, '.localhost')) {
        $fail('Repository URL cannot point to local network domains.');
        return;
    }

    // Resolve hostname and check for private IPs
    $ip = gethostbyname($host);
    if ($ip !== $host) {
        if (filter_var($ip, FILTER_VALIDATE_IP, FILTER_FLAG_NO_PRIV_RANGE | FILTER_FLAG_NO_RES_RANGE) === false) {
            $fail('Repository URL cannot point to private or reserved IP ranges.');
        }
    }
}
```

---

### Fixed 5: ZIM Path Traversal

**Status:** FIXED

**Location:** `app/Services/Zim/KiwixServeClient.php` (lines 501-514 in `getArticle()`, lines 540-553 in `getRawContent()`)

**Fix:** The `getArticle()` and `getRawContent()` methods now block path traversal attempts by checking for `..` sequences and null bytes. Hidden files are also blocked.

```php
// Security: Block path traversal attempts
if (str_contains($path, '..') || str_contains($path, "\0")) {
    throw new RuntimeException('Invalid article path: path traversal detected');
}

// Security: Block access to hidden files/directories
if (preg_match('/^\./', $path) || str_contains($path, '/.')) {
    throw new RuntimeException('Invalid article path: hidden files not allowed');
}
```

---

### Fixed 6: XXE Hardening

**Status:** FIXED

**Locations:**
- `app/Extractors/DocxExtractor.php` (lines 37-46 in `execute()`)
- `app/Extractors/XlsxExtractor.php` (lines 23-32 in `execute()`)

**Fix:** Both extractors now use `libxml_use_internal_errors(true)` to suppress XML parsing errors that could leak sensitive file paths. Errors are cleared after processing.

```php
// Security: Use internal errors to prevent XML parsing errors from leaking sensitive paths
$previousUseErrors = libxml_use_internal_errors(true);

try {
    $phpWord = IOFactory::load($filePath);
} finally {
    // Clear any accumulated errors and restore previous state
    libxml_clear_errors();
    libxml_use_internal_errors($previousUseErrors);
}
```

---

### Fixed 7: Token Sanitization

**Status:** FIXED

**Location:** `app/Services/GitTemplate/GitTemplateClient.php` (lines 508-539 in `sanitizeGitError()`)

**Fix:** The `sanitizeGitError()` method removes credentials from git error messages before logging or returning them. This prevents accidental exposure of git tokens.

```php
private function sanitizeGitError(string $error): string
{
    // Remove URLs that might contain tokens
    $sanitized = preg_replace('/(?:https?|git|ssh):\/\/[^\s]+/i', '[URL REDACTED]', $error);

    // Remove potential token patterns
    $sanitized = preg_replace('/(?:oauth2:|Bearer\s+|token[=:\s]+)[^\s]+/i', '[CREDENTIALS REDACTED]', $sanitized ?? $error);

    // Remove hex tokens (40+ chars)
    $sanitized = preg_replace('/[a-f0-9]{40,}/i', '[TOKEN REDACTED]', $sanitized ?? $error);

    // Remove base64-encoded credentials
    $sanitized = preg_replace('/[A-Za-z0-9+\/]{40,}={0,2}/', '[ENCODED REDACTED]', $sanitized ?? $error);

    return $sanitized ?? $error;
}
```

---

### Fixed 8: Path Canonicalization

**Status:** FIXED

**Location:** `app/Services/GitTemplate/GitTemplateClient.php` (lines 439-468 in `getRelativePath()`)

**Fix:** The `getRelativePath()` method uses `realpath()` to canonicalize paths and verify files are within the repository directory. This prevents path traversal via symbolic links or relative paths.

```php
private function getRelativePath(string $basePath, string $fullPath): string
{
    // Canonicalize both paths to prevent path traversal
    $realBase = realpath($basePath);
    $realFull = realpath($fullPath);

    // Ensure full path is within base path
    $realBase = rtrim($realBase, '/') . '/';
    if (! str_starts_with($realFull, $realBase)) {
        Log::warning('Path outside repository detected', [
            'base' => $realBase,
            'full' => $realFull,
        ]);
        return basename($fullPath);
    }

    return substr($realFull, strlen($realBase));
}
```

---

### Fixed 9: Variable Substitution Security

**Status:** FIXED

**Location:** `app/Services/GitTemplate/GitTemplateClient.php` (lines 593-606 in `applyVariables()`)

**Fix:** The `applyVariables()` method validates variable key format (alphanumeric and underscore only) and strips control characters from values before substitution.

```php
public function applyVariables(string $content, array $variables): string
{
    foreach ($variables as $key => $value) {
        // Security: Validate key format (alphanumeric + underscore only)
        if (! preg_match('/^[a-zA-Z_][a-zA-Z0-9_]*$/', $key)) {
            continue;
        }
        // Security: Strip null bytes and control characters from value
        $safeValue = preg_replace('/[\x00-\x08\x0B\x0C\x0E-\x1F]/', '', $value) ?? $value;
        $content = str_replace('{{' . $key . '}}', $safeValue, $content);
    }

    return $content;
}
```

---

## Labels to Create in Forgejo

```
security     - Security-related issues
bug          - Something isn't working
enhancement  - New feature or improvement
documentation - Documentation updates
code-quality - Code quality improvements
observability - Monitoring, logging, metrics
operations   - Operational concerns
P0-critical  - Must fix immediately
P1-high      - Fix this sprint
P2-medium    - Fix this quarter
P3-low       - Technical debt / nice to have
```
