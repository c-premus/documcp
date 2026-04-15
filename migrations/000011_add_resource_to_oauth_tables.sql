-- +goose Up
ALTER TABLE oauth_authorization_codes ADD COLUMN resource TEXT NULL;
ALTER TABLE oauth_access_tokens       ADD COLUMN resource TEXT NULL;
ALTER TABLE oauth_device_codes        ADD COLUMN resource TEXT NULL;

-- +goose Down
ALTER TABLE oauth_authorization_codes DROP COLUMN IF EXISTS resource;
ALTER TABLE oauth_access_tokens       DROP COLUMN IF EXISTS resource;
ALTER TABLE oauth_device_codes        DROP COLUMN IF EXISTS resource;
