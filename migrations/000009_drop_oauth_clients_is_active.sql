-- lint-disable-file: historical migration (already deployed; see migrations/README.md "Historical record")
-- +goose Up
DROP INDEX IF EXISTS idx_oauth_clients_is_active;
ALTER TABLE oauth_clients DROP COLUMN IF EXISTS is_active;

-- +goose Down
ALTER TABLE oauth_clients ADD COLUMN is_active BOOLEAN NOT NULL DEFAULT TRUE;
CREATE INDEX idx_oauth_clients_is_active ON oauth_clients (is_active);
