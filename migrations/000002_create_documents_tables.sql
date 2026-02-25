-- +goose Up
CREATE TABLE documents (
    id                     BIGSERIAL       PRIMARY KEY,
    uuid                   UUID            NOT NULL,
    title                  VARCHAR(255)    NOT NULL,
    description            TEXT            NULL,
    file_type              VARCHAR(50)     NOT NULL,
    file_path              VARCHAR(500)    NOT NULL,
    file_size              BIGINT          NOT NULL,
    mime_type              VARCHAR(100)    NOT NULL,
    url                    VARCHAR(500)    NULL,
    content                TEXT            NULL,
    content_hash           VARCHAR(64)     NULL,
    metadata               TEXT            NULL,
    processed_at           TIMESTAMPTZ     NULL,
    word_count             INTEGER         NULL,
    user_id                BIGINT          NULL,
    is_public              BOOLEAN         NOT NULL DEFAULT FALSE,
    status                 VARCHAR(50)     NOT NULL DEFAULT 'processing',
    error_message          TEXT            NULL,
    meilisearch_indexed_at TIMESTAMPTZ     NULL,
    created_at             TIMESTAMPTZ     NULL,
    updated_at             TIMESTAMPTZ     NULL,
    deleted_at             TIMESTAMPTZ     NULL,

    CONSTRAINT documents_uuid_unique UNIQUE (uuid),
    CONSTRAINT documents_user_id_foreign
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE SET NULL
);

COMMENT ON COLUMN documents.status IS 'Valid values: uploaded, processing, extracted, indexed, index_failed, failed';
COMMENT ON COLUMN documents.metadata IS 'Extraction metadata: images, structure, formatting';
COMMENT ON COLUMN documents.processed_at IS 'Timestamp when document extraction completed';

CREATE INDEX idx_documents_user_id ON documents (user_id);
CREATE INDEX idx_documents_file_type ON documents (file_type);
CREATE INDEX idx_documents_status ON documents (status);
CREATE INDEX idx_documents_created_at ON documents (created_at);
CREATE INDEX idx_documents_is_public ON documents (is_public);
CREATE INDEX idx_documents_deleted_at ON documents (deleted_at);
CREATE INDEX idx_documents_content_hash ON documents (content_hash);
CREATE INDEX idx_documents_user_id_created_at ON documents (user_id, created_at);
CREATE INDEX idx_documents_status_created_at ON documents (status, created_at);
CREATE INDEX idx_documents_is_public_created_at ON documents (is_public, created_at);
CREATE INDEX idx_documents_file_type_created_at ON documents (file_type, created_at);

CREATE TABLE document_versions (
    id          BIGSERIAL     PRIMARY KEY,
    document_id BIGINT        NOT NULL,
    version     INTEGER       NOT NULL,
    file_path   VARCHAR(500)  NOT NULL,
    content     TEXT          NULL,
    metadata    TEXT          NULL,
    created_at  TIMESTAMPTZ   NULL,
    updated_at  TIMESTAMPTZ   NULL,

    CONSTRAINT document_versions_document_id_foreign
        FOREIGN KEY (document_id) REFERENCES documents (id) ON DELETE CASCADE
);

COMMENT ON COLUMN document_versions.metadata IS 'Extraction metadata: images, structure, formatting';

CREATE INDEX idx_document_versions_document_id_version ON document_versions (document_id, version);

CREATE TABLE document_tags (
    id          BIGSERIAL     PRIMARY KEY,
    document_id BIGINT        NOT NULL,
    tag         VARCHAR(100)  NOT NULL,
    created_at  TIMESTAMPTZ   NULL,
    updated_at  TIMESTAMPTZ   NULL,

    CONSTRAINT document_tags_document_id_foreign
        FOREIGN KEY (document_id) REFERENCES documents (id) ON DELETE CASCADE,
    CONSTRAINT document_tags_document_id_tag_unique UNIQUE (document_id, tag)
);

CREATE INDEX idx_document_tags_document_id ON document_tags (document_id);
CREATE INDEX idx_document_tags_tag ON document_tags (tag);

-- +goose Down
DROP TABLE IF EXISTS document_tags;
DROP TABLE IF EXISTS document_versions;
DROP TABLE IF EXISTS documents;
