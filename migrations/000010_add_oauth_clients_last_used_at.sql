-- +goose Up
ALTER TABLE oauth_clients ADD COLUMN IF NOT EXISTS last_used_at TIMESTAMPTZ NULL;

-- +goose Down
ALTER TABLE oauth_clients DROP COLUMN IF EXISTS last_used_at;
