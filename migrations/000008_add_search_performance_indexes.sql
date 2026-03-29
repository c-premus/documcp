-- +goose Up
CREATE INDEX IF NOT EXISTS idx_search_queries_query_lower ON search_queries (LOWER(query));
CREATE INDEX IF NOT EXISTS idx_documents_title_pattern ON documents (title text_pattern_ops)
    WHERE deleted_at IS NULL AND is_public = true;

-- +goose Down
DROP INDEX IF EXISTS idx_documents_title_pattern;
DROP INDEX IF EXISTS idx_search_queries_query_lower;
