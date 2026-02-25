-- +goose Up
CREATE TABLE users (
    id                BIGSERIAL       PRIMARY KEY,
    name              VARCHAR(255)    NOT NULL,
    email             VARCHAR(255)    NOT NULL,
    oidc_sub          VARCHAR(255)    NULL,
    oidc_provider     VARCHAR(255)    NULL,
    email_verified_at TIMESTAMPTZ     NULL,
    is_admin          BOOLEAN         NOT NULL DEFAULT FALSE,
    password          VARCHAR(255)    NULL,
    remember_token    VARCHAR(100)    NULL,
    created_at        TIMESTAMPTZ     NULL,
    updated_at        TIMESTAMPTZ     NULL,

    CONSTRAINT users_email_unique UNIQUE (email),
    CONSTRAINT users_oidc_sub_unique UNIQUE (oidc_sub)
);

-- +goose Down
DROP TABLE IF EXISTS users;
