-- +goose Up

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
