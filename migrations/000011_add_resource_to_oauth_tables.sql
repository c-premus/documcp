-- RFC 8707 Resource Indicators for OAuth 2.0 — audience binding.
--
-- Every authorization code, access token, and device code now carries the
-- resource URI it was minted for. The audience-validating middleware
-- (internal/auth/middleware/middleware.go BearerTokenWithAudience) rejects
-- any bearer token whose `resource` column does not match the expected
-- audience for the route (/documcp vs /api). NULL means "legacy token,
-- minted before audience binding shipped" and is treated identically to a
-- mismatch: 401 with WWW-Authenticate: Bearer error="invalid_token",
-- error_description="audience mismatch". There is no grandfathering window
-- — clients must re-auth after this migration lands.
--
-- The allowlist of acceptable values is seeded from APP_URL and
-- APP_URL+DOCUMCP_ENDPOINT at config load (see internal/config + the
-- OAUTH_ALLOWED_RESOURCES override). Validation + canonicalization lives in
-- internal/auth/oauth/resource.go (ValidateResource).

-- +goose Up
ALTER TABLE oauth_authorization_codes ADD COLUMN resource TEXT NULL;
ALTER TABLE oauth_access_tokens       ADD COLUMN resource TEXT NULL;
ALTER TABLE oauth_device_codes        ADD COLUMN resource TEXT NULL;

-- +goose Down
ALTER TABLE oauth_authorization_codes DROP COLUMN IF EXISTS resource;
ALTER TABLE oauth_access_tokens       DROP COLUMN IF EXISTS resource;
ALTER TABLE oauth_device_codes        DROP COLUMN IF EXISTS resource;
