-- +goose Up
CREATE TABLE external_services (
    id                    BIGSERIAL     PRIMARY KEY,
    uuid                  UUID          NOT NULL,
    name                  VARCHAR(255)  NOT NULL,
    slug                  VARCHAR(255)  NOT NULL,
    type                  VARCHAR(255)  NOT NULL,
    base_url              VARCHAR(255)  NOT NULL,
    api_key               TEXT          NULL,
    config                TEXT          NULL,
    priority              INTEGER       NOT NULL DEFAULT 0,
    status                VARCHAR(255)  NOT NULL DEFAULT 'unknown',
    last_check_at         TIMESTAMPTZ   NULL,
    last_latency_ms       INTEGER       NULL,
    error_count           INTEGER       NOT NULL DEFAULT 0,
    consecutive_failures  INTEGER       NOT NULL DEFAULT 0,
    last_error            TEXT          NULL,
    last_error_at         TIMESTAMPTZ   NULL,
    is_enabled            BOOLEAN       NOT NULL DEFAULT TRUE,
    is_env_managed        BOOLEAN       NOT NULL DEFAULT FALSE,
    created_at            TIMESTAMPTZ   NULL,
    updated_at            TIMESTAMPTZ   NULL,

    CONSTRAINT external_services_uuid_unique UNIQUE (uuid),
    CONSTRAINT external_services_slug_unique UNIQUE (slug)
);

CREATE INDEX idx_external_services_type_is_enabled ON external_services (type, is_enabled);
CREATE INDEX idx_external_services_type_priority ON external_services (type, priority);
CREATE INDEX idx_external_services_status ON external_services (status);

-- +goose Down
DROP TABLE IF EXISTS external_services;
