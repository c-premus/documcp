-- +goose Up
CREATE TABLE git_templates (
    id                BIGSERIAL     PRIMARY KEY,
    uuid              UUID          NOT NULL,
    name              VARCHAR(255)  NOT NULL,
    slug              VARCHAR(255)  NOT NULL,
    description       TEXT          NULL,
    repository_url    VARCHAR(500)  NOT NULL,
    branch            VARCHAR(100)  NOT NULL DEFAULT 'main',
    git_token         TEXT          NULL,
    readme_content    TEXT          NULL,
    manifest          TEXT          NULL,
    category          VARCHAR(50)   NULL,
    tags              TEXT          NULL,
    user_id           BIGINT        NULL,
    is_public         BOOLEAN       NOT NULL DEFAULT FALSE,
    is_enabled        BOOLEAN       NOT NULL DEFAULT TRUE,
    status            VARCHAR(50)   NOT NULL DEFAULT 'pending',
    error_message     TEXT          NULL,
    last_synced_at    TIMESTAMPTZ   NULL,
    last_commit_sha   VARCHAR(40)   NULL,
    file_count        INTEGER       NOT NULL DEFAULT 0,
    total_size_bytes  BIGINT        NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ   NULL,
    updated_at        TIMESTAMPTZ   NULL,
    deleted_at        TIMESTAMPTZ   NULL,

    CONSTRAINT git_templates_uuid_unique UNIQUE (uuid),
    CONSTRAINT git_templates_slug_unique UNIQUE (slug),
    CONSTRAINT git_templates_user_id_foreign
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE SET NULL
);

CREATE INDEX idx_git_templates_user_id ON git_templates (user_id);
CREATE INDEX idx_git_templates_category ON git_templates (category);
CREATE INDEX idx_git_templates_status ON git_templates (status);
CREATE INDEX idx_git_templates_is_public ON git_templates (is_public);
CREATE INDEX idx_git_templates_is_enabled ON git_templates (is_enabled);
CREATE INDEX idx_git_templates_created_at ON git_templates (created_at);

CREATE TABLE git_template_files (
    id              BIGSERIAL     PRIMARY KEY,
    uuid            UUID          NOT NULL,
    git_template_id BIGINT        NOT NULL,
    path            VARCHAR(500)  NOT NULL,
    filename        VARCHAR(255)  NOT NULL,
    extension       VARCHAR(50)   NULL,
    content         TEXT          NULL,
    is_compressed   BOOLEAN       NOT NULL DEFAULT FALSE,
    size_bytes      BIGINT        NOT NULL DEFAULT 0,
    content_hash    VARCHAR(64)   NULL,
    is_essential    BOOLEAN       NOT NULL DEFAULT FALSE,
    variables       TEXT          NULL,
    created_at      TIMESTAMPTZ   NULL,
    updated_at      TIMESTAMPTZ   NULL,

    CONSTRAINT git_template_files_uuid_unique UNIQUE (uuid),
    CONSTRAINT git_template_files_git_template_id_path_unique UNIQUE (git_template_id, path),
    CONSTRAINT git_template_files_git_template_id_foreign
        FOREIGN KEY (git_template_id) REFERENCES git_templates (id) ON DELETE CASCADE
);

CREATE INDEX idx_git_template_files_git_template_id ON git_template_files (git_template_id);
CREATE INDEX idx_git_template_files_path ON git_template_files (path);
CREATE INDEX idx_git_template_files_is_essential ON git_template_files (is_essential);

-- +goose Down
DROP TABLE IF EXISTS git_template_files;
DROP TABLE IF EXISTS git_templates;
