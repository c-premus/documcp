-- +goose NO TRANSACTION
--
-- This migration adds two indexes on tables that already contain rows in any
-- non-greenfield deployment (`search_queries` is populated continuously after
-- migration 000004; `documents` is populated by user uploads after 000002).
-- Building the indexes inline would take an ACCESS EXCLUSIVE lock for the
-- duration of the build, blocking all writes to both tables.
--
-- `+goose NO TRANSACTION` lets us use `CREATE INDEX CONCURRENTLY`, which
-- builds the index without blocking writes (it takes only a SHARE UPDATE
-- EXCLUSIVE lock that is compatible with INSERT/UPDATE/DELETE). The trade-off
-- is that statements run in autocommit, so a partial failure leaves a partial
-- state. `IF NOT EXISTS` guards make re-running the migration safe — re-runs
-- pick up wherever the previous attempt failed. See `migrations/README.md`
-- for the safe-migration convention.

-- +goose Up
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_search_queries_query_lower ON search_queries (LOWER(query));
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_documents_title_pattern ON documents (title text_pattern_ops)
    WHERE deleted_at IS NULL AND is_public = true;

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS idx_documents_title_pattern;
DROP INDEX CONCURRENTLY IF EXISTS idx_search_queries_query_lower;
