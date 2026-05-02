-- lint-disable-file: historical migration (already deployed; see migrations/README.md "Historical record")
-- +goose Up

-- WARNING: TABLE-REWRITE MIGRATION
-- This migration takes ACCESS EXCLUSIVE on `documents` for the duration of
-- the STORED-column rebuild, blocking all reads and writes. The rewrite is
-- intrinsic: dropping `search_vector` and adding it back as a STORED
-- generated column forces Postgres to materialize a new physical relation
-- with the recomputed values for every row. Neither `+goose NO TRANSACTION`
-- nor `CONCURRENTLY` helps for this shape.
--
-- DEPLOYMENT: operators upgrading from PHP DocuMCP (or any pre-existing
-- database with populated documents) should plan a write-downtime window
-- proportional to row count × average content size. On a small tenant
-- this is seconds; on millions of rows of multi-MB content it can be
-- minutes.
--
-- Future similar work should use the safe multi-step pattern documented in
-- `migrations/README.md` (add nullable column + trigger + backfill in
-- batches + verify + swap) rather than this in-place rewrite shape.

-- Replace the trigger-driven `documents.search_vector` with a STORED generated
-- column. Tags move from `document_tags` (separate table, read via trigger)
-- into `documents.tags_text` (space-joined on the same row), letting the
-- generated column be deterministic over row columns only.
--
-- ReplaceTags() in Go is responsible for keeping `tags_text` in sync with the
-- `document_tags` rows within the same transaction.

ALTER TABLE documents ADD COLUMN tags_text TEXT NULL;

UPDATE documents d SET tags_text = sub.joined
FROM (
    SELECT document_id, string_agg(tag, ' ' ORDER BY tag) AS joined
    FROM document_tags
    GROUP BY document_id
) sub
WHERE d.id = sub.document_id;

DROP TRIGGER IF EXISTS trg_document_tags_search_vector ON document_tags;
DROP TRIGGER IF EXISTS trg_documents_search_vector ON documents;
DROP FUNCTION IF EXISTS documents_search_vector_update();

DROP INDEX IF EXISTS idx_documents_search_vector;
ALTER TABLE documents DROP COLUMN search_vector;

ALTER TABLE documents ADD COLUMN search_vector tsvector GENERATED ALWAYS AS (
    setweight(to_tsvector('documcp_english', COALESCE(title, '')), 'A') ||
    setweight(to_tsvector('documcp_english', COALESCE(description, '')), 'B') ||
    setweight(to_tsvector('documcp_english', COALESCE(tags_text, '')), 'B') ||
    setweight(to_tsvector('documcp_english', COALESCE(content, '')), 'D')
) STORED;

CREATE INDEX idx_documents_search_vector ON documents USING GIN (search_vector);

-- +goose Down

DROP INDEX IF EXISTS idx_documents_search_vector;
ALTER TABLE documents DROP COLUMN search_vector;
ALTER TABLE documents ADD COLUMN search_vector tsvector NULL;
CREATE INDEX idx_documents_search_vector ON documents USING GIN (search_vector);

ALTER TABLE documents DROP COLUMN tags_text;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION documents_search_vector_update() RETURNS trigger AS $$
DECLARE
    tag_text TEXT;
    doc_id   BIGINT;
    doc_title TEXT;
    doc_desc  TEXT;
    doc_content TEXT;
BEGIN
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

    SELECT COALESCE(string_agg(tag, ' '), '') INTO tag_text
      FROM document_tags WHERE document_id = doc_id;

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
