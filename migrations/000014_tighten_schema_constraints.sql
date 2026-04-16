-- +goose Up

-- CHECK constraints reify enum-like columns that were previously stored as
-- freeform VARCHAR. Valid values match the Go-side typed-string constants.

-- documents.status — scrub any pre-typed-constants legacy values, then constrain.
UPDATE documents SET status = 'failed'
WHERE status NOT IN ('pending', 'uploaded', 'indexed', 'failed');

ALTER TABLE documents
    ADD CONSTRAINT documents_status_check
    CHECK (status IN ('pending', 'uploaded', 'indexed', 'failed'));

-- git_templates.status — GitTemplateStatus {pending, synced, failed}.
UPDATE git_templates SET status = 'failed'
WHERE status NOT IN ('pending', 'synced', 'failed');

ALTER TABLE git_templates
    ADD CONSTRAINT git_templates_status_check
    CHECK (status IN ('pending', 'synced', 'failed'));

-- oauth_device_codes.status — DeviceCodeStatus {pending, authorized, exchanged, denied}.
ALTER TABLE oauth_device_codes
    ADD CONSTRAINT oauth_device_codes_status_check
    CHECK (status IN ('pending', 'authorized', 'exchanged', 'denied'));

-- external_services.status — ExternalServiceStatus {unknown, healthy, unhealthy}.
ALTER TABLE external_services
    ADD CONSTRAINT external_services_status_check
    CHECK (status IN ('unknown', 'healthy', 'unhealthy'));

-- code_challenge_method — PKCE handler only accepts S256 (authorize.go:109).
ALTER TABLE oauth_authorization_codes
    ADD CONSTRAINT oauth_authorization_codes_code_challenge_method_check
    CHECK (code_challenge_method IS NULL OR code_challenge_method = 'S256');

-- token_endpoint_auth_method — advertised in wellknown.go: none, basic, post.
ALTER TABLE oauth_clients
    ADD CONSTRAINT oauth_clients_token_endpoint_auth_method_check
    CHECK (token_endpoint_auth_method IN ('none', 'client_secret_basic', 'client_secret_post'));

-- Drop indexes that duplicate their UNIQUE constraint's implicit index.
-- Postgres picks the unique index; these redundant B-trees only slow writes.
DROP INDEX IF EXISTS idx_oauth_access_tokens_token;
DROP INDEX IF EXISTS idx_oauth_authorization_codes_code;
DROP INDEX IF EXISTS idx_oauth_refresh_tokens_token;

-- users.oidc_sub uniqueness needs oidc_provider in the key: two providers can
-- each emit the same `sub` value and must not collide on insert.
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_oidc_sub_unique;
ALTER TABLE users
    ADD CONSTRAINT users_oidc_provider_sub_unique UNIQUE (oidc_provider, oidc_sub);

-- +goose Down

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_oidc_provider_sub_unique;
ALTER TABLE users ADD CONSTRAINT users_oidc_sub_unique UNIQUE (oidc_sub);

CREATE INDEX idx_oauth_refresh_tokens_token ON oauth_refresh_tokens (token);
CREATE INDEX idx_oauth_authorization_codes_code ON oauth_authorization_codes (code);
CREATE INDEX idx_oauth_access_tokens_token ON oauth_access_tokens (token);

ALTER TABLE oauth_clients DROP CONSTRAINT IF EXISTS oauth_clients_token_endpoint_auth_method_check;
ALTER TABLE oauth_authorization_codes DROP CONSTRAINT IF EXISTS oauth_authorization_codes_code_challenge_method_check;
ALTER TABLE external_services DROP CONSTRAINT IF EXISTS external_services_status_check;
ALTER TABLE oauth_device_codes DROP CONSTRAINT IF EXISTS oauth_device_codes_status_check;
ALTER TABLE git_templates DROP CONSTRAINT IF EXISTS git_templates_status_check;
ALTER TABLE documents DROP CONSTRAINT IF EXISTS documents_status_check;
