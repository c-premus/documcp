-- +goose Up

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
