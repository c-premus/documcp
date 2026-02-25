-- +goose Up
CREATE TABLE search_queries (
    id            BIGSERIAL   PRIMARY KEY,
    user_id       BIGINT      NULL,
    query         TEXT        NOT NULL,
    results_count INTEGER     NOT NULL,
    filters       TEXT        NULL,
    created_at    TIMESTAMPTZ NULL,
    updated_at    TIMESTAMPTZ NULL,

    CONSTRAINT search_queries_user_id_foreign
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE SET NULL
);

CREATE INDEX idx_search_queries_user_id ON search_queries (user_id);
CREATE INDEX idx_search_queries_created_at ON search_queries (created_at);

-- +goose Down
DROP TABLE IF EXISTS search_queries;
