-- lint-disable-file: historical migration (already deployed; see migrations/README.md "Historical record")
-- +goose Up

-- WARNING: TABLE-REWRITE MIGRATION
-- This migration takes ACCESS EXCLUSIVE on `zim_archives` for the duration
-- of the column-type cast plus the STORED-column rebuild, blocking all
-- reads and writes. The rewrite is intrinsic: `ALTER COLUMN TYPE` with a
-- USING clause and the subsequent `ADD COLUMN ... STORED` both materialize
-- a new physical relation. Neither `+goose NO TRANSACTION` nor
-- `CONCURRENTLY` helps for these shapes.
--
-- DEPLOYMENT: operators upgrading from PHP DocuMCP (or any pre-existing
-- database with populated rows) should plan a write-downtime window
-- proportional to row count. ZIM-archive rows are typically far smaller
-- than `documents`, so the window is shorter than 000013/000019, but
-- still scales with row count.
--
-- Future similar work should use the safe multi-step pattern documented in
-- `migrations/README.md` (add new typed column + dual-write + backfill in
-- batches + verify + swap + drop old) rather than an in-place type cast.

-- Convert zim_archives.tags from TEXT (holding JSON-encoded []string) to
-- native JSONB. Drop and rebuild the STORED search_vector column to
-- replace the regexp_replace('[\[\]",'']', ' ', 'g') punctuation-strip
-- hack with JSONB-native tokenization.
--
-- to_tsvector already treats JSON punctuation (`[`, `]`, `"`, `,`) as
-- token separators, so casting tags::text yields equivalent tokens without
-- the regex. The jsonb_typeof(tags) = 'array' guard rejects non-array
-- JSONB values (JSON null, object, scalar) so they don't contribute
-- literal "null" / "{}" tokens to search.
--
-- Subqueries are disallowed in generated column expressions, so the
-- jsonb_array_elements_text approach from the plan's first sketch was
-- replaced with the ::text cast + typeof guard — same outcome, IMMUTABLE-
-- compatible.

DROP INDEX IF EXISTS idx_zim_archives_search_vector;
ALTER TABLE zim_archives DROP COLUMN search_vector;

ALTER TABLE zim_archives
    ALTER COLUMN tags TYPE JSONB
    USING NULLIF(tags, '')::jsonb;

ALTER TABLE zim_archives ADD COLUMN search_vector tsvector GENERATED ALWAYS AS (
    setweight(to_tsvector('documcp_english', COALESCE(title, '')), 'A') ||
    setweight(to_tsvector('documcp_english', COALESCE(name, '')), 'A') ||
    setweight(to_tsvector('documcp_english', COALESCE(description, '')), 'B') ||
    setweight(to_tsvector('documcp_english', COALESCE(creator, '')), 'C') ||
    setweight(to_tsvector('documcp_english',
        CASE WHEN jsonb_typeof(tags) = 'array' THEN tags::text ELSE '' END
    ), 'C')
) STORED;

CREATE INDEX idx_zim_archives_search_vector ON zim_archives USING GIN (search_vector);
CREATE INDEX idx_zim_archives_tags_gin ON zim_archives USING GIN (tags);

-- +goose Down

DROP INDEX IF EXISTS idx_zim_archives_tags_gin;
DROP INDEX IF EXISTS idx_zim_archives_search_vector;
ALTER TABLE zim_archives DROP COLUMN search_vector;

ALTER TABLE zim_archives
    ALTER COLUMN tags TYPE TEXT
    USING tags::text;

ALTER TABLE zim_archives ADD COLUMN search_vector tsvector GENERATED ALWAYS AS (
    setweight(to_tsvector('documcp_english', COALESCE(title, '')), 'A') ||
    setweight(to_tsvector('documcp_english', COALESCE(name, '')), 'A') ||
    setweight(to_tsvector('documcp_english', COALESCE(description, '')), 'B') ||
    setweight(to_tsvector('documcp_english', COALESCE(creator, '')), 'C') ||
    setweight(to_tsvector('documcp_english', COALESCE(
        regexp_replace(COALESCE(tags, ''), '[\[\]",'']', ' ', 'g'), ''
    )), 'C')
) STORED;

CREATE INDEX idx_zim_archives_search_vector ON zim_archives USING GIN (search_vector);
