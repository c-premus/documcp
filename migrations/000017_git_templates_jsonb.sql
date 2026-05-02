-- lint-disable-file: historical migration (already deployed; see migrations/README.md "Historical record")
-- +goose Up

-- WARNING: TABLE-REWRITE MIGRATION
-- This migration takes ACCESS EXCLUSIVE on `git_templates` for the duration
-- of two column-type casts plus the STORED-column rebuild, blocking all
-- reads and writes. The rewrite is intrinsic: each `ALTER COLUMN TYPE`
-- with a USING clause and the `ADD COLUMN ... STORED` all materialize a
-- new physical relation. Neither `+goose NO TRANSACTION` nor
-- `CONCURRENTLY` helps for these shapes.
--
-- DEPLOYMENT: operators upgrading from PHP DocuMCP (or any pre-existing
-- database with populated rows) should plan a write-downtime window
-- proportional to row count × average README content size. Most git
-- template tenants have low row counts; the window is typically small
-- but grows with `readme_content` heap size.
--
-- Future similar work should use the safe multi-step pattern documented in
-- `migrations/README.md` (add new typed column + dual-write + backfill in
-- batches + verify + swap + drop old) rather than an in-place type cast.

-- Convert git_templates.tags and git_templates.manifest from TEXT to
-- JSONB. Drop and rebuild the STORED search_vector to replace the
-- regexp_replace punctuation-strip hack with a JSONB-native expression
-- matching the pattern used in migration 000016 for zim_archives.
--
-- tags: writer is service/git_template_service.go — json.Marshal([]string)
-- into the column. All existing rows are either NULL or a valid JSON
-- array of strings.
--
-- manifest: no production writer exists. All rows are NULL. Type change
-- is a free tightening bundled with the same-table search_vector rebuild.
--
-- file_paths: intentionally NOT migrated. client/git/sync.go:buildFilePaths
-- writes a space-separated TEXT string, not JSON. Converting would
-- require changing the writer first. Out of scope; the real fix is to
-- drop the column in favor of git_template_files.path joins.

DROP INDEX IF EXISTS idx_git_templates_search_vector;
ALTER TABLE git_templates DROP COLUMN search_vector;

ALTER TABLE git_templates
    ALTER COLUMN tags TYPE JSONB
    USING NULLIF(tags, '')::jsonb;

ALTER TABLE git_templates
    ALTER COLUMN manifest TYPE JSONB
    USING NULLIF(manifest, '')::jsonb;

ALTER TABLE git_templates ADD COLUMN search_vector tsvector GENERATED ALWAYS AS (
    setweight(to_tsvector('documcp_english', COALESCE(name, '')), 'A') ||
    setweight(to_tsvector('documcp_english', COALESCE(description, '')), 'B') ||
    setweight(to_tsvector('documcp_english',
        CASE WHEN jsonb_typeof(tags) = 'array' THEN tags::text ELSE '' END
    ), 'B') ||
    setweight(to_tsvector('documcp_english', COALESCE(readme_content, '')), 'C') ||
    setweight(to_tsvector('documcp_english', COALESCE(category, '')), 'C') ||
    setweight(to_tsvector('documcp_english', COALESCE(file_paths, '')), 'D')
) STORED;

CREATE INDEX idx_git_templates_search_vector ON git_templates USING GIN (search_vector);
CREATE INDEX idx_git_templates_tags_gin ON git_templates USING GIN (tags);

-- +goose Down

DROP INDEX IF EXISTS idx_git_templates_tags_gin;
DROP INDEX IF EXISTS idx_git_templates_search_vector;
ALTER TABLE git_templates DROP COLUMN search_vector;

ALTER TABLE git_templates
    ALTER COLUMN manifest TYPE TEXT
    USING manifest::text;

ALTER TABLE git_templates
    ALTER COLUMN tags TYPE TEXT
    USING tags::text;

ALTER TABLE git_templates ADD COLUMN search_vector tsvector GENERATED ALWAYS AS (
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
