-- lint-disable-file: historical migration (already deployed; see migrations/README.md "Historical record")
-- +goose Up

-- WARNING: TABLE-REWRITE MIGRATION
-- This migration takes ACCESS EXCLUSIVE on `documents` and
-- `document_versions` for the duration of the column-type cast, blocking
-- all reads and writes. The rewrite is intrinsic: `ALTER COLUMN TYPE`
-- with a USING clause rebuilds every row regardless of whether the
-- existing values are NULL — Postgres has to materialize the new column
-- type into the heap. Neither the `NO TRANSACTION` directive nor
-- `CONCURRENTLY` helps for this shape.
--
-- DEPLOYMENT: operators upgrading from PHP DocuMCP (or any pre-existing
-- database with populated rows) should plan a write-downtime window
-- proportional to total row count across both tables. On a small tenant
-- this is seconds; on millions of rows it can be minutes.
--
-- Future similar work should use the safe multi-step pattern documented in
-- `migrations/README.md` (add new typed column + dual-write + backfill in
-- batches + verify + swap + drop old) rather than an in-place type cast.

-- Convert documents.metadata and document_versions.metadata from TEXT to
-- JSONB. These columns have always held JSON payloads but were stored as
-- TEXT, preventing native JSON path queries and GIN indexing. The
-- extraction pipeline wires metadata persistence in the same release
-- (see document_pipeline.go:ProcessDocument), so rows created from
-- v0.22.0 onward will carry extractor-emitted Dublin Core / PDF / DOCX /
-- XLSX / HTML / Markdown metadata.
--
-- Pre-v0.22.0 rows are always NULL (the pipeline previously dropped
-- result.Metadata on the floor). The USING NULLIF(..., '') cast is
-- defensive for any stray writes.
--
-- No search_vector changes: documents.search_vector does not reference
-- the metadata column today. A follow-up release will extend the STORED
-- expression with JSONB-path extraction (title / creator / subjects /
-- description weighted into the tsvector).

ALTER TABLE documents
    ALTER COLUMN metadata TYPE JSONB
    USING NULLIF(metadata, '')::jsonb;

ALTER TABLE document_versions
    ALTER COLUMN metadata TYPE JSONB
    USING NULLIF(metadata, '')::jsonb;

-- +goose Down

ALTER TABLE document_versions
    ALTER COLUMN metadata TYPE TEXT
    USING metadata::text;

ALTER TABLE documents
    ALTER COLUMN metadata TYPE TEXT
    USING metadata::text;
