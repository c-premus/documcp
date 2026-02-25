-- +goose Up
CREATE TABLE zim_archives (
    id                     BIGSERIAL     PRIMARY KEY,
    uuid                   UUID          NOT NULL,
    name                   VARCHAR(255)  NOT NULL,
    slug                   VARCHAR(255)  NOT NULL,
    kiwix_id               VARCHAR(255)  NULL,
    title                  VARCHAR(255)  NOT NULL,
    description            TEXT          NULL,
    language               VARCHAR(10)   NOT NULL DEFAULT 'en',
    category               VARCHAR(255)  NULL,
    creator                VARCHAR(255)  NULL,
    publisher              VARCHAR(255)  NULL,
    favicon                TEXT          NULL,
    article_count          BIGINT        NOT NULL DEFAULT 0,
    media_count            BIGINT        NOT NULL DEFAULT 0,
    file_size              BIGINT        NOT NULL DEFAULT 0,
    tags                   TEXT          NULL,
    external_service_id    BIGINT        NULL,
    is_enabled             BOOLEAN       NOT NULL DEFAULT TRUE,
    is_searchable          BOOLEAN       NOT NULL DEFAULT TRUE,
    last_synced_at         TIMESTAMPTZ   NULL,
    meilisearch_indexed_at TIMESTAMPTZ   NULL,
    created_at             TIMESTAMPTZ   NULL,
    updated_at             TIMESTAMPTZ   NULL,

    CONSTRAINT zim_archives_uuid_unique UNIQUE (uuid),
    CONSTRAINT zim_archives_name_unique UNIQUE (name),
    CONSTRAINT zim_archives_slug_unique UNIQUE (slug),
    CONSTRAINT zim_archives_external_service_id_foreign
        FOREIGN KEY (external_service_id) REFERENCES external_services (id) ON DELETE SET NULL
);

CREATE INDEX idx_zim_archives_name ON zim_archives (name);
CREATE INDEX idx_zim_archives_category ON zim_archives (category);
CREATE INDEX idx_zim_archives_language ON zim_archives (language);
CREATE INDEX idx_zim_archives_is_enabled_is_searchable ON zim_archives (is_enabled, is_searchable);
CREATE INDEX idx_zim_archives_external_service_id ON zim_archives (external_service_id);

-- +goose Down
DROP TABLE IF EXISTS zim_archives;
