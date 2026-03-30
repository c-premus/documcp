-- +goose Up

-- Enable extensions for full-text search and fuzzy matching.
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS unaccent;

-- Custom text search configuration with unaccent support.
CREATE TEXT SEARCH CONFIGURATION documcp_english (COPY = english);
ALTER TEXT SEARCH CONFIGURATION documcp_english
    ALTER MAPPING FOR asciiword, asciihword, hword_asciipart, word, hword, hword_part
    WITH unaccent, english_stem;

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
    search_vector          tsvector        NULL,
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
CREATE INDEX idx_documents_search_vector ON documents USING GIN (search_vector);
CREATE INDEX idx_documents_title_trgm ON documents USING GIN (title gin_trgm_ops);

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

-- +goose StatementBegin
-- Trigger function: builds weighted vector from title(A), description(B), tags(B), content(D).
CREATE OR REPLACE FUNCTION documents_search_vector_update() RETURNS trigger AS $$
DECLARE
    tag_text TEXT;
    doc_id   BIGINT;
    doc_title TEXT;
    doc_desc  TEXT;
    doc_content TEXT;
BEGIN
    -- Determine which document to update.
    IF TG_TABLE_NAME = 'document_tags' THEN
        doc_id := COALESCE(NEW.document_id, OLD.document_id);
        SELECT title, COALESCE(description, ''), COALESCE(content, '')
          INTO doc_title, doc_desc, doc_content
          FROM documents WHERE id = doc_id;
        IF NOT FOUND THEN RETURN NEW; END IF;
    ELSE
        doc_id := NEW.id;
        doc_title := NEW.title;
        doc_desc := COALESCE(NEW.description, '');
        doc_content := COALESCE(NEW.content, '');
    END IF;

    -- Gather tags.
    SELECT COALESCE(string_agg(tag, ' '), '') INTO tag_text
      FROM document_tags WHERE document_id = doc_id;

    -- Build weighted tsvector.
    UPDATE documents SET search_vector =
        setweight(to_tsvector('documcp_english', COALESCE(doc_title, '')), 'A') ||
        setweight(to_tsvector('documcp_english', doc_desc), 'B') ||
        setweight(to_tsvector('documcp_english', tag_text), 'B') ||
        setweight(to_tsvector('documcp_english', doc_content), 'D')
    WHERE id = doc_id;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trg_documents_search_vector
    AFTER INSERT OR UPDATE OF title, description, content ON documents
    FOR EACH ROW EXECUTE FUNCTION documents_search_vector_update();

CREATE TRIGGER trg_document_tags_search_vector
    AFTER INSERT OR UPDATE OR DELETE ON document_tags
    FOR EACH ROW EXECUTE FUNCTION documents_search_vector_update();

-- +goose Down
DROP TRIGGER IF EXISTS trg_document_tags_search_vector ON document_tags;
DROP TRIGGER IF EXISTS trg_documents_search_vector ON documents;
DROP FUNCTION IF EXISTS documents_search_vector_update();
DROP TABLE IF EXISTS document_tags;
DROP TABLE IF EXISTS document_versions;
DROP TABLE IF EXISTS documents;
DROP TEXT SEARCH CONFIGURATION IF EXISTS documcp_english;
DROP EXTENSION IF EXISTS unaccent;
DROP EXTENSION IF EXISTS pg_trgm;
