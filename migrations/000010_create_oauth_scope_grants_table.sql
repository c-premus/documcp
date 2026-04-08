-- +goose Up
CREATE TABLE oauth_client_scope_grants (
    id          BIGSERIAL    PRIMARY KEY,
    client_id   BIGINT       NOT NULL,
    scope       TEXT         NOT NULL,
    granted_by  BIGINT       NOT NULL,
    granted_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    expires_at  TIMESTAMPTZ  NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_scope_grants_client
        FOREIGN KEY (client_id) REFERENCES oauth_clients (id) ON DELETE CASCADE,
    CONSTRAINT fk_scope_grants_user
        FOREIGN KEY (granted_by) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT uq_scope_grants_client_user
        UNIQUE (client_id, granted_by)
);

CREATE INDEX idx_scope_grants_client_id ON oauth_client_scope_grants (client_id);
CREATE INDEX idx_scope_grants_expires_at ON oauth_client_scope_grants (expires_at)
    WHERE expires_at IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS oauth_client_scope_grants;
