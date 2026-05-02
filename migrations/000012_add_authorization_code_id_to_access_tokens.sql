-- +goose NO TRANSACTION
--
-- Adds `authorization_code_id` to `oauth_access_tokens` so auth-code replay
-- (security.md M1) and refresh-token reuse (M2) can revoke the entire token
-- family in one query. The column is nullable because tokens issued via the
-- device flow do not flow from an authorization code, and because pre-existing
-- tokens from earlier releases have no recorded parentage.
--
-- `oauth_access_tokens` is populated continuously after migration 000003.
-- `+goose NO TRANSACTION` lets us use `CREATE INDEX CONCURRENTLY`, which
-- builds the supporting partial index without blocking writes. The trade-off
-- is that statements run in autocommit. `IF NOT EXISTS` guards on the column
-- and index, plus a DO-block for the foreign-key constraint, make re-running
-- the migration safe — re-runs pick up wherever the previous attempt failed.
-- See `migrations/README.md` for the safe-migration convention.

-- +goose Up

ALTER TABLE oauth_access_tokens ADD COLUMN IF NOT EXISTS authorization_code_id BIGINT NULL;

-- +goose StatementBegin
DO $$
BEGIN
    ALTER TABLE oauth_access_tokens
        ADD CONSTRAINT oauth_access_tokens_authorization_code_id_foreign
        FOREIGN KEY (authorization_code_id) REFERENCES oauth_authorization_codes (id) ON DELETE SET NULL;
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;
-- +goose StatementEnd

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_oauth_access_tokens_authorization_code_id
    ON oauth_access_tokens (authorization_code_id)
    WHERE authorization_code_id IS NOT NULL;

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS idx_oauth_access_tokens_authorization_code_id;
ALTER TABLE oauth_access_tokens DROP CONSTRAINT IF EXISTS oauth_access_tokens_authorization_code_id_foreign;
ALTER TABLE oauth_access_tokens DROP COLUMN IF EXISTS authorization_code_id;
