-- lint-disable-file: historical migration (already deployed; see migrations/README.md "Historical record")
-- +goose Up

-- WARNING: TABLE-REWRITE MIGRATION
-- This migration takes ACCESS EXCLUSIVE on `documents` for the duration of
-- the STORED-column rebuild, blocking all reads and writes. The rewrite
-- is intrinsic: dropping `search_vector` and adding it back with the
-- extended JSONB-path expression forces Postgres to materialize a new
-- physical relation with the recomputed values for every row. Neither
-- the `NO TRANSACTION` directive nor `CONCURRENTLY` helps for this shape.
--
-- DEPLOYMENT: operators upgrading from PHP DocuMCP (or any pre-existing
-- database with populated documents) should plan a write-downtime window
-- proportional to row count × average content size. On a small tenant
-- this is seconds; on millions of rows of multi-MB content it can be
-- minutes. This is the second `documents` rewrite (after 000013) — the
-- two could have been combined had the metadata-extension design been
-- in scope at the same time.
--
-- Future similar work should use the safe multi-step pattern documented in
-- `migrations/README.md` (add nullable column + trigger + backfill in
-- batches + verify + swap) rather than this in-place rewrite shape.

-- Extend the STORED documents.search_vector generated column to index the
-- JSONB metadata column landed by migration 000015. Closes the
-- "EPUB-only content-header baking" duplication that existed because,
-- pre-v0.22.0, the EPUB extractor was the only path that baked Dublin
-- Core title/creator/subjects/description into result.Content purely so
-- FTS could reach the metadata at all. The other five extractors (PDF,
-- DOCX, XLSX, HTML, Markdown) returned metadata only via result.Metadata
-- and their metadata was invisible to search.
--
-- With migrations 000015-000018 now persisting result.Metadata to
-- documents.metadata JSONB, this migration reads four Dublin Core-ish
-- JSONB paths (title / creator / subjects / description) directly into
-- the tsvector. The follow-up code change drops buildMetadataHeader
-- from the EPUB extractor since the header hack is no longer needed.
--
-- Pattern: matches migration 000016 (zim_archives) and 000017
-- (git_templates). STORED generated column expressions must be IMMUTABLE
-- and disallow subqueries, so jsonb_array_elements_text is not usable
-- here. Instead, for the subjects array we cast the JSONB sub-element
-- to text and let to_tsvector tokenize across the JSON punctuation
-- (`[`, `]`, `"`, `,`), guarded by a jsonb_typeof check so non-array
-- values contribute nothing. For scalar fields (title / creator /
-- description) we use metadata ->> 'key', which already coerces to text
-- and yields NULL for non-string values — COALESCE handles that.
--
-- Weights match the 000013-era shape (title=A, description=B) and mirror
-- zim_archives conventions (creator=B elevated from C on the reasoning
-- that a document's author is more load-bearing than an archive's;
-- subjects=C matching zim tags).

DROP INDEX IF EXISTS idx_documents_search_vector;
ALTER TABLE documents DROP COLUMN search_vector;

ALTER TABLE documents ADD COLUMN search_vector tsvector GENERATED ALWAYS AS (
    setweight(to_tsvector('documcp_english', COALESCE(title, '')), 'A') ||
    setweight(to_tsvector('documcp_english', COALESCE(description, '')), 'B') ||
    setweight(to_tsvector('documcp_english', COALESCE(tags_text, '')), 'B') ||
    setweight(to_tsvector('documcp_english', COALESCE(content, '')), 'D') ||
    setweight(to_tsvector('documcp_english', COALESCE(metadata ->> 'title', '')), 'A') ||
    setweight(to_tsvector('documcp_english', COALESCE(metadata ->> 'creator', '')), 'B') ||
    setweight(to_tsvector('documcp_english', COALESCE(metadata ->> 'description', '')), 'B') ||
    setweight(to_tsvector('documcp_english',
        CASE WHEN jsonb_typeof(metadata -> 'subjects') = 'array'
             THEN (metadata -> 'subjects')::text
             ELSE ''
        END
    ), 'C')
) STORED;

CREATE INDEX idx_documents_search_vector ON documents USING GIN (search_vector);

-- +goose Down

DROP INDEX IF EXISTS idx_documents_search_vector;
ALTER TABLE documents DROP COLUMN search_vector;

ALTER TABLE documents ADD COLUMN search_vector tsvector GENERATED ALWAYS AS (
    setweight(to_tsvector('documcp_english', COALESCE(title, '')), 'A') ||
    setweight(to_tsvector('documcp_english', COALESCE(description, '')), 'B') ||
    setweight(to_tsvector('documcp_english', COALESCE(tags_text, '')), 'B') ||
    setweight(to_tsvector('documcp_english', COALESCE(content, '')), 'D')
) STORED;

CREATE INDEX idx_documents_search_vector ON documents USING GIN (search_vector);
