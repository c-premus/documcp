-- +goose Up
CREATE TABLE confluence_spaces (
    id                  BIGSERIAL     PRIMARY KEY,
    uuid                UUID          NOT NULL,
    confluence_id       VARCHAR(255)  NOT NULL,
    key                 VARCHAR(255)  NOT NULL,
    name                VARCHAR(255)  NOT NULL,
    description         TEXT          NULL,
    type                VARCHAR(255)  NOT NULL DEFAULT 'global',
    status              VARCHAR(255)  NOT NULL DEFAULT 'current',
    homepage_id         VARCHAR(255)  NULL,
    icon_url            TEXT          NULL,
    external_service_id BIGINT        NULL,
    is_enabled          BOOLEAN       NOT NULL DEFAULT TRUE,
    is_searchable       BOOLEAN       NOT NULL DEFAULT TRUE,
    last_synced_at      TIMESTAMPTZ   NULL,
    created_at          TIMESTAMPTZ   NULL,
    updated_at          TIMESTAMPTZ   NULL,

    CONSTRAINT confluence_spaces_uuid_unique UNIQUE (uuid),
    CONSTRAINT confluence_spaces_key_unique UNIQUE (key),
    CONSTRAINT confluence_spaces_external_service_id_foreign
        FOREIGN KEY (external_service_id) REFERENCES external_services (id) ON DELETE SET NULL
);

CREATE INDEX idx_confluence_spaces_confluence_id ON confluence_spaces (confluence_id);
CREATE INDEX idx_confluence_spaces_key ON confluence_spaces (key);
CREATE INDEX idx_confluence_spaces_type ON confluence_spaces (type);
CREATE INDEX idx_confluence_spaces_status ON confluence_spaces (status);
CREATE INDEX idx_confluence_spaces_is_enabled_is_searchable ON confluence_spaces (is_enabled, is_searchable);
CREATE INDEX idx_confluence_spaces_external_service_id ON confluence_spaces (external_service_id);

-- +goose Down
DROP TABLE IF EXISTS confluence_spaces;
