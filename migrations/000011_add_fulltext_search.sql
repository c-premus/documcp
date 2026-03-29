-- +goose Up

-- Enable extensions for full-text search and fuzzy matching.
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS unaccent;

-- Custom text search configuration with unaccent support.
CREATE TEXT SEARCH CONFIGURATION documcp_english (COPY = english);
ALTER TEXT SEARCH CONFIGURATION documcp_english
    ALTER MAPPING FOR asciiword, asciihword, hword_asciipart, word, hword, hword_part
    WITH unaccent, english_stem;

-- ---------------------------------------------------------------------------
-- documents: trigger-maintained tsvector (tags live in document_tags table)
-- ---------------------------------------------------------------------------
ALTER TABLE documents ADD COLUMN search_vector tsvector;
ALTER TABLE documents DROP COLUMN IF EXISTS meilisearch_indexed_at;

CREATE INDEX idx_documents_search_vector ON documents USING GIN (search_vector);
CREATE INDEX idx_documents_title_trgm ON documents USING GIN (title gin_trgm_ops);

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

-- ---------------------------------------------------------------------------
-- zim_archives: GENERATED ALWAYS STORED
-- Weights: title(A) + name(A) + description(B) + creator(C) + tags(C)
-- ---------------------------------------------------------------------------
ALTER TABLE zim_archives ADD COLUMN search_vector tsvector
    GENERATED ALWAYS AS (
        setweight(to_tsvector('documcp_english', COALESCE(title, '')), 'A') ||
        setweight(to_tsvector('documcp_english', COALESCE(name, '')), 'A') ||
        setweight(to_tsvector('documcp_english', COALESCE(description, '')), 'B') ||
        setweight(to_tsvector('documcp_english', COALESCE(creator, '')), 'C') ||
        setweight(to_tsvector('documcp_english', COALESCE(
            regexp_replace(COALESCE(tags, ''), '[\[\]",'']', ' ', 'g'), ''
        )), 'C')
    ) STORED;

ALTER TABLE zim_archives DROP COLUMN IF EXISTS meilisearch_indexed_at;

CREATE INDEX idx_zim_archives_search_vector ON zim_archives USING GIN (search_vector);
CREATE INDEX idx_zim_archives_title_trgm ON zim_archives USING GIN (title gin_trgm_ops);

-- ---------------------------------------------------------------------------
-- git_templates: GENERATED ALWAYS STORED
-- Weights: name(A) + description(B) + tags(B) + readme_content(C) + category(C) + file_paths(D)
-- ---------------------------------------------------------------------------
ALTER TABLE git_templates ADD COLUMN file_paths TEXT NULL;

ALTER TABLE git_templates ADD COLUMN search_vector tsvector
    GENERATED ALWAYS AS (
        setweight(to_tsvector('documcp_english', COALESCE(name, '')), 'A') ||
        setweight(to_tsvector('documcp_english', COALESCE(description, '')), 'B') ||
        setweight(to_tsvector('documcp_english', COALESCE(
            regexp_replace(COALESCE(tags, ''), '[\[\]",'']', ' ', 'g'), ''
        )), 'B') ||
        setweight(to_tsvector('documcp_english', COALESCE(readme_content, '')), 'C') ||
        setweight(to_tsvector('documcp_english', COALESCE(category, '')), 'C') ||
        setweight(to_tsvector('documcp_english', COALESCE(file_paths, '')), 'D')
    ) STORED;

CREATE INDEX idx_git_templates_search_vector ON git_templates USING GIN (search_vector);
CREATE INDEX idx_git_templates_name_trgm ON git_templates USING GIN (name gin_trgm_ops);

-- ---------------------------------------------------------------------------
-- git_template_files: GENERATED ALWAYS STORED
-- Weights: filename(A, humanized) + content(D, capped at 500KB)
-- ---------------------------------------------------------------------------
ALTER TABLE git_template_files ADD COLUMN search_vector tsvector
    GENERATED ALWAYS AS (
        setweight(to_tsvector('documcp_english', COALESCE(
            regexp_replace(COALESCE(filename, ''), '[-_.]', ' ', 'g'), ''
        )), 'A') ||
        setweight(to_tsvector('documcp_english', COALESCE(LEFT(content, 500000), '')), 'D')
    ) STORED;

CREATE INDEX idx_git_template_files_search_vector ON git_template_files USING GIN (search_vector);


-- +goose Down

DROP TRIGGER IF EXISTS trg_document_tags_search_vector ON document_tags;
DROP TRIGGER IF EXISTS trg_documents_search_vector ON documents;
DROP FUNCTION IF EXISTS documents_search_vector_update();

DROP INDEX IF EXISTS idx_documents_search_vector;
DROP INDEX IF EXISTS idx_documents_title_trgm;
ALTER TABLE documents DROP COLUMN IF EXISTS search_vector;
ALTER TABLE documents ADD COLUMN meilisearch_indexed_at TIMESTAMPTZ NULL;

DROP INDEX IF EXISTS idx_zim_archives_search_vector;
DROP INDEX IF EXISTS idx_zim_archives_title_trgm;
ALTER TABLE zim_archives DROP COLUMN IF EXISTS search_vector;
ALTER TABLE zim_archives ADD COLUMN meilisearch_indexed_at TIMESTAMPTZ NULL;

DROP INDEX IF EXISTS idx_git_template_files_search_vector;
ALTER TABLE git_template_files DROP COLUMN IF EXISTS search_vector;

DROP INDEX IF EXISTS idx_git_templates_search_vector;
DROP INDEX IF EXISTS idx_git_templates_name_trgm;
ALTER TABLE git_templates DROP COLUMN IF EXISTS search_vector;
ALTER TABLE git_templates DROP COLUMN IF EXISTS file_paths;

DROP TEXT SEARCH CONFIGURATION IF EXISTS documcp_english;
DROP EXTENSION IF EXISTS unaccent;
DROP EXTENSION IF EXISTS pg_trgm;
