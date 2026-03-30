-- +goose Up
CREATE TABLE oauth_clients (
    id                         BIGSERIAL     PRIMARY KEY,
    client_id                  VARCHAR(255)  NOT NULL,
    client_secret              VARCHAR(255)  NULL,
    client_secret_expires_at   TIMESTAMPTZ   NULL,
    client_name                VARCHAR(255)  NOT NULL,
    software_id                VARCHAR(255)  NULL,
    software_version           VARCHAR(100)  NULL,
    redirect_uris              TEXT          NOT NULL,
    grant_types                TEXT          NOT NULL,
    response_types             TEXT          NOT NULL,
    token_endpoint_auth_method VARCHAR(50)   NOT NULL DEFAULT 'none',
    scope                      VARCHAR(500)  NULL,
    user_id                    BIGINT        NULL,
    is_active                  BOOLEAN       NOT NULL DEFAULT TRUE,
    last_used_at               TIMESTAMPTZ   NULL,
    created_at                 TIMESTAMPTZ   NULL,
    updated_at                 TIMESTAMPTZ   NULL,

    CONSTRAINT oauth_clients_client_id_unique UNIQUE (client_id),
    CONSTRAINT oauth_clients_user_id_foreign
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

CREATE INDEX idx_oauth_clients_user_id ON oauth_clients (user_id);
CREATE INDEX idx_oauth_clients_is_active ON oauth_clients (is_active);

CREATE TABLE oauth_authorization_codes (
    id                    BIGSERIAL     PRIMARY KEY,
    code                  VARCHAR(128)  NOT NULL,
    client_id             BIGINT        NOT NULL,
    user_id               BIGINT        NULL,
    redirect_uri          TEXT          NOT NULL,
    scope                 TEXT          NULL,
    code_challenge        VARCHAR(128)  NULL,
    code_challenge_method VARCHAR(10)   NULL,
    expires_at            TIMESTAMPTZ   NOT NULL,
    revoked               BOOLEAN       NOT NULL DEFAULT FALSE,
    created_at            TIMESTAMPTZ   NULL,
    updated_at            TIMESTAMPTZ   NULL,

    CONSTRAINT oauth_authorization_codes_code_unique UNIQUE (code),
    CONSTRAINT oauth_authorization_codes_client_id_foreign
        FOREIGN KEY (client_id) REFERENCES oauth_clients (id) ON DELETE CASCADE,
    CONSTRAINT oauth_authorization_codes_user_id_foreign
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

CREATE INDEX idx_oauth_authorization_codes_code ON oauth_authorization_codes (code);
CREATE INDEX idx_oauth_authorization_codes_client_id ON oauth_authorization_codes (client_id);
CREATE INDEX idx_oauth_authorization_codes_expires_at_revoked ON oauth_authorization_codes (expires_at, revoked);

CREATE TABLE oauth_access_tokens (
    id         BIGSERIAL     PRIMARY KEY,
    token      VARCHAR(255)  NOT NULL,
    client_id  BIGINT        NOT NULL,
    user_id    BIGINT        NULL,
    scope      TEXT          NULL,
    expires_at TIMESTAMPTZ   NOT NULL,
    revoked    BOOLEAN       NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ   NULL,
    updated_at TIMESTAMPTZ   NULL,

    CONSTRAINT oauth_access_tokens_token_unique UNIQUE (token),
    CONSTRAINT oauth_access_tokens_client_id_foreign
        FOREIGN KEY (client_id) REFERENCES oauth_clients (id) ON DELETE CASCADE,
    CONSTRAINT oauth_access_tokens_user_id_foreign
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

CREATE INDEX idx_oauth_access_tokens_token ON oauth_access_tokens (token);
CREATE INDEX idx_oauth_access_tokens_client_id ON oauth_access_tokens (client_id);
CREATE INDEX idx_oauth_access_tokens_user_id ON oauth_access_tokens (user_id);
CREATE INDEX idx_oauth_access_tokens_expires_at_revoked ON oauth_access_tokens (expires_at, revoked);

CREATE TABLE oauth_refresh_tokens (
    id              BIGSERIAL     PRIMARY KEY,
    token           VARCHAR(255)  NOT NULL,
    access_token_id BIGINT        NOT NULL,
    expires_at      TIMESTAMPTZ   NOT NULL,
    revoked         BOOLEAN       NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ   NULL,
    updated_at      TIMESTAMPTZ   NULL,

    CONSTRAINT oauth_refresh_tokens_token_unique UNIQUE (token),
    CONSTRAINT oauth_refresh_tokens_access_token_id_foreign
        FOREIGN KEY (access_token_id) REFERENCES oauth_access_tokens (id) ON DELETE CASCADE
);

CREATE INDEX idx_oauth_refresh_tokens_token ON oauth_refresh_tokens (token);
CREATE INDEX idx_oauth_refresh_tokens_access_token_id ON oauth_refresh_tokens (access_token_id);
CREATE INDEX idx_oauth_refresh_tokens_expires_at_revoked ON oauth_refresh_tokens (expires_at, revoked);

CREATE TABLE oauth_device_codes (
    id                          BIGSERIAL     PRIMARY KEY,
    device_code                 VARCHAR(255)  NOT NULL,
    user_code                   VARCHAR(9)    NOT NULL,
    client_id                   BIGINT        NOT NULL,
    user_id                     BIGINT        NULL,
    scope                       TEXT          NULL,
    verification_uri            VARCHAR(512)  NOT NULL,
    verification_uri_complete   VARCHAR(512)  NULL,
    interval                    INTEGER       NOT NULL DEFAULT 5,
    last_polled_at              TIMESTAMPTZ   NULL,
    status                      VARCHAR(20)   NOT NULL DEFAULT 'pending',
    expires_at                  TIMESTAMPTZ   NOT NULL,
    created_at                  TIMESTAMPTZ   NULL,
    updated_at                  TIMESTAMPTZ   NULL,

    CONSTRAINT oauth_device_codes_user_code_unique UNIQUE (user_code),
    CONSTRAINT oauth_device_codes_client_id_foreign
        FOREIGN KEY (client_id) REFERENCES oauth_clients (id) ON DELETE CASCADE,
    CONSTRAINT oauth_device_codes_user_id_foreign
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

CREATE INDEX idx_oauth_device_codes_client_id ON oauth_device_codes (client_id);
CREATE INDEX idx_oauth_device_codes_user_id ON oauth_device_codes (user_id);
CREATE INDEX idx_oauth_device_codes_status_expires_at ON oauth_device_codes (status, expires_at);

-- +goose Down
DROP TABLE IF EXISTS oauth_device_codes;
DROP TABLE IF EXISTS oauth_refresh_tokens;
DROP TABLE IF EXISTS oauth_access_tokens;
DROP TABLE IF EXISTS oauth_authorization_codes;
DROP TABLE IF EXISTS oauth_clients;
