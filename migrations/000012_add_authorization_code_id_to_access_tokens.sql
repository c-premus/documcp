-- +goose Up
-- Link access tokens back to the authorization code that minted them so that
-- auth-code replay (security.md M1) and refresh-token reuse (M2) can revoke
-- the entire token family in one query. The column is nullable because
-- tokens issued via the device flow do not flow from an authorization code,
-- and because pre-existing tokens from earlier releases have no recorded
-- parentage.
ALTER TABLE oauth_access_tokens ADD COLUMN authorization_code_id BIGINT NULL;

ALTER TABLE oauth_access_tokens
    ADD CONSTRAINT oauth_access_tokens_authorization_code_id_foreign
    FOREIGN KEY (authorization_code_id) REFERENCES oauth_authorization_codes (id) ON DELETE SET NULL;

CREATE INDEX idx_oauth_access_tokens_authorization_code_id
    ON oauth_access_tokens (authorization_code_id)
    WHERE authorization_code_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_oauth_access_tokens_authorization_code_id;
ALTER TABLE oauth_access_tokens DROP CONSTRAINT IF EXISTS oauth_access_tokens_authorization_code_id_foreign;
ALTER TABLE oauth_access_tokens DROP COLUMN IF EXISTS authorization_code_id;
