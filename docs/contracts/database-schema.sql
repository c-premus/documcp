-- ============================================================
-- DocuMCP Database Schema
-- Generated from Laravel migrations (33 migration files)
-- Cross-referenced with 15 Eloquent model files
-- PostgreSQL-native DDL
--
-- Source: /workspaces/DocuMCP/database/migrations/
-- Models: /workspaces/DocuMCP/app/Models/
-- Generated: 2026-02-24
-- ============================================================

-- Tables are ordered for foreign key dependency resolution:
--   1. Independent tables (no FK references)
--   2. Tables referencing only independent tables
--   3. Tables referencing second-tier tables

-- ============================================================
-- Table: users
-- Migration: 0001_01_01_000000_create_users_table.php
-- Modified:  2025_11_13_134731_add_oidc_fields_to_users_table.php
-- Modified:  2025_11_22_224440_add_is_admin_to_users_table.php
-- Model:     App\Models\User
-- ============================================================
CREATE TABLE users (
    id                BIGSERIAL       PRIMARY KEY,
    name              VARCHAR(255)    NOT NULL,
    email             VARCHAR(255)    NOT NULL,
    oidc_sub          VARCHAR(255)    NULL,
    oidc_provider     VARCHAR(255)    NULL,
    email_verified_at TIMESTAMPTZ     NULL,
    is_admin          BOOLEAN         NOT NULL DEFAULT FALSE,
    password          VARCHAR(255)    NULL,
    remember_token    VARCHAR(100)    NULL,
    created_at        TIMESTAMPTZ     NULL,
    updated_at        TIMESTAMPTZ     NULL,

    CONSTRAINT users_email_unique UNIQUE (email),
    CONSTRAINT users_oidc_sub_unique UNIQUE (oidc_sub)
);


-- ============================================================
-- Table: password_reset_tokens
-- Migration: 0001_01_01_000000_create_users_table.php
-- ============================================================
CREATE TABLE password_reset_tokens (
    email      VARCHAR(255)  NOT NULL,
    token      VARCHAR(255)  NOT NULL,
    created_at TIMESTAMPTZ   NULL,

    CONSTRAINT password_reset_tokens_pkey PRIMARY KEY (email)
);


-- ============================================================
-- Table: sessions
-- Migration: 0001_01_01_000000_create_users_table.php
-- ============================================================
CREATE TABLE sessions (
    id             VARCHAR(255) NOT NULL,
    user_id        BIGINT       NULL,
    ip_address     VARCHAR(45)  NULL,
    user_agent     TEXT         NULL,
    payload        TEXT         NOT NULL,
    last_activity  INTEGER      NOT NULL,

    CONSTRAINT sessions_pkey PRIMARY KEY (id),
    CONSTRAINT sessions_user_id_foreign
        FOREIGN KEY (user_id) REFERENCES users (id)
);

CREATE INDEX idx_sessions_user_id ON sessions (user_id);
CREATE INDEX idx_sessions_last_activity ON sessions (last_activity);


-- ============================================================
-- Table: cache
-- Migration: 0001_01_01_000001_create_cache_table.php
-- ============================================================
CREATE TABLE cache (
    key        VARCHAR(255)  NOT NULL,
    value      TEXT          NOT NULL,
    expiration INTEGER       NOT NULL,

    CONSTRAINT cache_pkey PRIMARY KEY (key)
);


-- ============================================================
-- Table: cache_locks
-- Migration: 0001_01_01_000001_create_cache_table.php
-- ============================================================
CREATE TABLE cache_locks (
    key        VARCHAR(255)  NOT NULL,
    owner      VARCHAR(255)  NOT NULL,
    expiration INTEGER       NOT NULL,

    CONSTRAINT cache_locks_pkey PRIMARY KEY (key)
);


-- ============================================================
-- Table: jobs
-- Migration: 0001_01_01_000002_create_jobs_table.php
-- ============================================================
CREATE TABLE jobs (
    id           BIGSERIAL  PRIMARY KEY,
    queue        VARCHAR(255)  NOT NULL,
    payload      TEXT          NOT NULL,
    attempts     SMALLINT      NOT NULL,
    reserved_at  INTEGER       NULL,
    available_at INTEGER       NOT NULL,
    created_at   INTEGER       NOT NULL
);

CREATE INDEX idx_jobs_queue ON jobs (queue);


-- ============================================================
-- Table: job_batches
-- Migration: 0001_01_01_000002_create_jobs_table.php
-- ============================================================
CREATE TABLE job_batches (
    id             VARCHAR(255)  NOT NULL,
    name           VARCHAR(255)  NOT NULL,
    total_jobs     INTEGER       NOT NULL,
    pending_jobs   INTEGER       NOT NULL,
    failed_jobs    INTEGER       NOT NULL,
    failed_job_ids TEXT          NOT NULL,
    options        TEXT          NULL,
    cancelled_at   INTEGER       NULL,
    created_at     INTEGER       NOT NULL,
    finished_at    INTEGER       NULL,

    CONSTRAINT job_batches_pkey PRIMARY KEY (id)
);


-- ============================================================
-- Table: failed_jobs
-- Migration: 0001_01_01_000002_create_jobs_table.php
-- ============================================================
CREATE TABLE failed_jobs (
    id        BIGSERIAL     PRIMARY KEY,
    uuid      VARCHAR(255)  NOT NULL,
    connection TEXT          NOT NULL,
    queue     TEXT          NOT NULL,
    payload   TEXT          NOT NULL,
    exception TEXT          NOT NULL,
    failed_at TIMESTAMPTZ   NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT failed_jobs_uuid_unique UNIQUE (uuid)
);


-- ============================================================
-- Table: documents
-- Migration: 2025_11_13_134725_create_documents_table.php
-- Modified:  2025_11_18_160330_add_error_message_to_documents_table.php
-- Modified:  2025_11_19_142835_refactor_documents_content_storage.php
--            (renamed extracted_text -> content, added metadata, processed_at)
-- Modified:  2025_11_21_014433_add_performance_indexes.php
-- Modified:  2025_11_24_000001_add_index_failed_status_to_documents_table.php
--            (status comment updated to include index_failed)
-- Modified:  2026_01_27_213225_add_content_hash_to_documents_table.php
-- Model:     App\Models\Document
-- ============================================================
CREATE TABLE documents (
    id                     BIGSERIAL       PRIMARY KEY,
    uuid                   UUID            NOT NULL,
    title                  VARCHAR(255)    NOT NULL,
    description            TEXT            NULL,
    file_type              VARCHAR(50)     NOT NULL,
    file_path              VARCHAR(500)    NOT NULL,
    file_size              BIGINT          NOT NULL,
    mime_type              VARCHAR(100)    NOT NULL,
    url                    VARCHAR(500)    NULL,
    content                TEXT            NULL,
    content_hash           VARCHAR(64)     NULL,
    metadata               TEXT            NULL,
    processed_at           TIMESTAMPTZ     NULL,
    word_count             INTEGER         NULL,
    user_id                BIGINT          NULL,
    is_public              BOOLEAN         NOT NULL DEFAULT FALSE,
    status                 VARCHAR(50)     NOT NULL DEFAULT 'processing',
    error_message          TEXT            NULL,
    meilisearch_indexed_at TIMESTAMPTZ     NULL,
    created_at             TIMESTAMPTZ     NULL,
    updated_at             TIMESTAMPTZ     NULL,
    deleted_at             TIMESTAMPTZ     NULL,

    CONSTRAINT documents_uuid_unique UNIQUE (uuid),
    CONSTRAINT documents_user_id_foreign
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE SET NULL
);

COMMENT ON COLUMN documents.status IS 'Valid values: uploaded, processing, extracted, indexed, index_failed, failed';
COMMENT ON COLUMN documents.metadata IS 'Extraction metadata: images, structure, formatting';
COMMENT ON COLUMN documents.processed_at IS 'Timestamp when document extraction completed';

-- Single-column indexes (from create migration)
CREATE INDEX idx_documents_user_id ON documents (user_id);
CREATE INDEX idx_documents_file_type ON documents (file_type);
CREATE INDEX idx_documents_status ON documents (status);
CREATE INDEX idx_documents_created_at ON documents (created_at);
CREATE INDEX idx_documents_is_public ON documents (is_public);

-- Soft delete index (from performance indexes migration)
CREATE INDEX idx_documents_deleted_at ON documents (deleted_at);

-- Content hash index (from add content_hash migration)
CREATE INDEX idx_documents_content_hash ON documents (content_hash);

-- Composite indexes (from performance indexes migration)
CREATE INDEX idx_documents_user_id_created_at ON documents (user_id, created_at);
CREATE INDEX idx_documents_status_created_at ON documents (status, created_at);
CREATE INDEX idx_documents_is_public_created_at ON documents (is_public, created_at);
CREATE INDEX idx_documents_file_type_created_at ON documents (file_type, created_at);


-- ============================================================
-- Table: document_versions
-- Migration: 2025_11_13_134726_create_document_versions_table.php
-- Modified:  2025_11_19_143215_refactor_document_versions_content_storage.php
--            (renamed extracted_text -> content, added metadata)
-- Model:     App\Models\DocumentVersion
-- ============================================================
CREATE TABLE document_versions (
    id          BIGSERIAL     PRIMARY KEY,
    document_id BIGINT        NOT NULL,
    version     INTEGER       NOT NULL,
    file_path   VARCHAR(500)  NOT NULL,
    content     TEXT          NULL,
    metadata    TEXT          NULL,
    created_at  TIMESTAMPTZ   NULL,
    updated_at  TIMESTAMPTZ   NULL,

    CONSTRAINT document_versions_document_id_foreign
        FOREIGN KEY (document_id) REFERENCES documents (id) ON DELETE CASCADE
);

COMMENT ON COLUMN document_versions.metadata IS 'Extraction metadata: images, structure, formatting';

CREATE INDEX idx_document_versions_document_id_version ON document_versions (document_id, version);


-- ============================================================
-- Table: document_tags
-- Migration: 2025_11_13_134730_create_document_tags_table.php
-- Model:     App\Models\DocumentTag
-- ============================================================
CREATE TABLE document_tags (
    id          BIGSERIAL     PRIMARY KEY,
    document_id BIGINT        NOT NULL,
    tag         VARCHAR(100)  NOT NULL,
    created_at  TIMESTAMPTZ   NULL,
    updated_at  TIMESTAMPTZ   NULL,

    CONSTRAINT document_tags_document_id_foreign
        FOREIGN KEY (document_id) REFERENCES documents (id) ON DELETE CASCADE,
    CONSTRAINT document_tags_document_id_tag_unique UNIQUE (document_id, tag)
);

CREATE INDEX idx_document_tags_document_id ON document_tags (document_id);
CREATE INDEX idx_document_tags_tag ON document_tags (tag);


-- ============================================================
-- Table: oauth_clients
-- Migration: 2025_11_13_134728_create_oauth_clients_table.php
-- Modified:  2025_11_16_044923_add_rfc_7591_fields_to_oauth_clients_table.php
--            (added client_secret_expires_at, software_id, software_version,
--             token_endpoint_auth_method)
-- Modified:  2025_11_16_195537_make_client_secret_nullable_in_oauth_clients_table.php
--            (client_secret made nullable for public clients)
-- Data-only: 2025_11_22_000001_rehash_oauth_client_secrets.php (no schema change)
-- Model:     App\Models\OAuthClient
-- ============================================================
CREATE TABLE oauth_clients (
    id                         BIGSERIAL     PRIMARY KEY,
    client_id                  VARCHAR(255)  NOT NULL,
    client_secret              VARCHAR(255)  NULL,
    client_secret_expires_at   TIMESTAMPTZ   NULL,
    client_name                VARCHAR(255)  NOT NULL,
    software_id                VARCHAR(255)  NULL,
    software_version           VARCHAR(100)  NULL,
    redirect_uris              TEXT          NOT NULL,
    grant_types                TEXT          NOT NULL,
    response_types             TEXT          NOT NULL,
    token_endpoint_auth_method VARCHAR(50)   NOT NULL DEFAULT 'none',
    scope                      VARCHAR(500)  NULL,
    user_id                    BIGINT        NULL,
    is_active                  BOOLEAN       NOT NULL DEFAULT TRUE,
    last_used_at               TIMESTAMPTZ   NULL,
    created_at                 TIMESTAMPTZ   NULL,
    updated_at                 TIMESTAMPTZ   NULL,

    CONSTRAINT oauth_clients_client_id_unique UNIQUE (client_id),
    CONSTRAINT oauth_clients_user_id_foreign
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

CREATE INDEX idx_oauth_clients_user_id ON oauth_clients (user_id);
CREATE INDEX idx_oauth_clients_is_active ON oauth_clients (is_active);


-- ============================================================
-- Table: oauth_authorization_codes
-- Migration: 2025_11_16_044741_create_oauth_authorization_codes_table.php
-- Modified:  2025_11_22_000003_add_foreign_keys_to_oauth_tables.php
--            (added FK constraints for client_id, user_id)
-- Data-only: 2025_11_22_000002_revoke_existing_oauth_tokens.php (no schema change)
-- Model:     App\Models\OAuthAuthorizationCode
-- ============================================================
CREATE TABLE oauth_authorization_codes (
    id                    BIGSERIAL     PRIMARY KEY,
    code                  VARCHAR(128)  NOT NULL,
    client_id             BIGINT        NOT NULL,
    user_id               BIGINT        NULL,
    redirect_uri          TEXT          NOT NULL,
    scope                 TEXT          NULL,
    code_challenge        VARCHAR(128)  NULL,
    code_challenge_method VARCHAR(10)   NULL,
    expires_at            TIMESTAMPTZ   NOT NULL,
    revoked               BOOLEAN       NOT NULL DEFAULT FALSE,
    created_at            TIMESTAMPTZ   NULL,
    updated_at            TIMESTAMPTZ   NULL,

    CONSTRAINT oauth_authorization_codes_code_unique UNIQUE (code),
    CONSTRAINT oauth_authorization_codes_client_id_foreign
        FOREIGN KEY (client_id) REFERENCES oauth_clients (id) ON DELETE CASCADE,
    CONSTRAINT oauth_authorization_codes_user_id_foreign
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

CREATE INDEX idx_oauth_authorization_codes_code ON oauth_authorization_codes (code);
CREATE INDEX idx_oauth_authorization_codes_client_id ON oauth_authorization_codes (client_id);
CREATE INDEX idx_oauth_authorization_codes_expires_at_revoked ON oauth_authorization_codes (expires_at, revoked);


-- ============================================================
-- Table: oauth_access_tokens
-- Migration: 2025_11_16_044812_create_oauth_access_tokens_table.php
-- Modified:  2025_11_22_000003_add_foreign_keys_to_oauth_tables.php
--            (added FK constraints for client_id, user_id)
-- Data-only: 2025_11_22_000002_revoke_existing_oauth_tokens.php (no schema change)
-- Model:     App\Models\OAuthAccessToken
-- ============================================================
CREATE TABLE oauth_access_tokens (
    id         BIGSERIAL     PRIMARY KEY,
    token      VARCHAR(255)  NOT NULL,
    client_id  BIGINT        NOT NULL,
    user_id    BIGINT        NULL,
    scope      TEXT          NULL,
    expires_at TIMESTAMPTZ   NOT NULL,
    revoked    BOOLEAN       NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ   NULL,
    updated_at TIMESTAMPTZ   NULL,

    CONSTRAINT oauth_access_tokens_token_unique UNIQUE (token),
    CONSTRAINT oauth_access_tokens_client_id_foreign
        FOREIGN KEY (client_id) REFERENCES oauth_clients (id) ON DELETE CASCADE,
    CONSTRAINT oauth_access_tokens_user_id_foreign
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

CREATE INDEX idx_oauth_access_tokens_token ON oauth_access_tokens (token);
CREATE INDEX idx_oauth_access_tokens_client_id ON oauth_access_tokens (client_id);
CREATE INDEX idx_oauth_access_tokens_user_id ON oauth_access_tokens (user_id);
CREATE INDEX idx_oauth_access_tokens_expires_at_revoked ON oauth_access_tokens (expires_at, revoked);


-- ============================================================
-- Table: oauth_refresh_tokens
-- Migration: 2025_11_16_044841_create_oauth_refresh_tokens_table.php
-- Modified:  2025_11_22_000003_add_foreign_keys_to_oauth_tables.php
--            (added FK constraint for access_token_id)
-- Data-only: 2025_11_22_000002_revoke_existing_oauth_tokens.php (no schema change)
-- Model:     App\Models\OAuthRefreshToken
-- ============================================================
CREATE TABLE oauth_refresh_tokens (
    id              BIGSERIAL     PRIMARY KEY,
    token           VARCHAR(255)  NOT NULL,
    access_token_id BIGINT        NOT NULL,
    expires_at      TIMESTAMPTZ   NOT NULL,
    revoked         BOOLEAN       NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ   NULL,
    updated_at      TIMESTAMPTZ   NULL,

    CONSTRAINT oauth_refresh_tokens_token_unique UNIQUE (token),
    CONSTRAINT oauth_refresh_tokens_access_token_id_foreign
        FOREIGN KEY (access_token_id) REFERENCES oauth_access_tokens (id) ON DELETE CASCADE
);

CREATE INDEX idx_oauth_refresh_tokens_token ON oauth_refresh_tokens (token);
CREATE INDEX idx_oauth_refresh_tokens_access_token_id ON oauth_refresh_tokens (access_token_id);
CREATE INDEX idx_oauth_refresh_tokens_expires_at_revoked ON oauth_refresh_tokens (expires_at, revoked);


-- ============================================================
-- Table: oauth_device_codes
-- Migration: 2026_01_17_000001_create_oauth_device_codes_table.php
-- Model:     App\Models\OAuthDeviceCode
-- ============================================================
CREATE TABLE oauth_device_codes (
    id                          BIGSERIAL     PRIMARY KEY,
    device_code                 VARCHAR(255)  NOT NULL,
    user_code                   VARCHAR(9)    NOT NULL,
    client_id                   BIGINT        NOT NULL,
    user_id                     BIGINT        NULL,
    scope                       TEXT          NULL,
    verification_uri            VARCHAR(512)  NOT NULL,
    verification_uri_complete   VARCHAR(512)  NULL,
    interval                    INTEGER       NOT NULL DEFAULT 5,
    last_polled_at              TIMESTAMPTZ   NULL,
    status                      VARCHAR(20)   NOT NULL DEFAULT 'pending',
    expires_at                  TIMESTAMPTZ   NOT NULL,
    created_at                  TIMESTAMPTZ   NULL,
    updated_at                  TIMESTAMPTZ   NULL,

    CONSTRAINT oauth_device_codes_user_code_unique UNIQUE (user_code),
    CONSTRAINT oauth_device_codes_client_id_foreign
        FOREIGN KEY (client_id) REFERENCES oauth_clients (id) ON DELETE CASCADE,
    CONSTRAINT oauth_device_codes_user_id_foreign
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

CREATE INDEX idx_oauth_device_codes_client_id ON oauth_device_codes (client_id);
CREATE INDEX idx_oauth_device_codes_user_id ON oauth_device_codes (user_id);
CREATE INDEX idx_oauth_device_codes_status_expires_at ON oauth_device_codes (status, expires_at);


-- ============================================================
-- Table: search_queries
-- Migration: 2025_11_13_134729_create_search_queries_table.php
-- Model:     App\Models\SearchQuery
-- ============================================================
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


-- ============================================================
-- Table: personal_access_tokens
-- Migration: 2025_11_22_164038_create_personal_access_tokens_table.php
-- Note:      Uses Sanctum's polymorphic morphs (tokenable_type, tokenable_id)
-- ============================================================
CREATE TABLE personal_access_tokens (
    id             BIGSERIAL     PRIMARY KEY,
    tokenable_type VARCHAR(255)  NOT NULL,
    tokenable_id   BIGINT        NOT NULL,
    name           TEXT          NOT NULL,
    token          VARCHAR(64)   NOT NULL,
    abilities      TEXT          NULL,
    last_used_at   TIMESTAMPTZ   NULL,
    expires_at     TIMESTAMPTZ   NULL,
    created_at     TIMESTAMPTZ   NULL,
    updated_at     TIMESTAMPTZ   NULL,

    CONSTRAINT personal_access_tokens_token_unique UNIQUE (token)
);

CREATE INDEX idx_personal_access_tokens_tokenable ON personal_access_tokens (tokenable_type, tokenable_id);
CREATE INDEX idx_personal_access_tokens_expires_at ON personal_access_tokens (expires_at);


-- ============================================================
-- Table: external_services
-- Migration: 2025_11_26_000001_create_external_services_table.php
-- Modified:  2026_01_18_000001_add_is_env_managed_to_external_services.php
-- Model:     App\Models\ExternalService
-- ============================================================
CREATE TABLE external_services (
    id                    BIGSERIAL     PRIMARY KEY,
    uuid                  UUID          NOT NULL,
    name                  VARCHAR(255)  NOT NULL,
    slug                  VARCHAR(255)  NOT NULL,
    type                  VARCHAR(255)  NOT NULL,
    base_url              VARCHAR(255)  NOT NULL,
    api_key               TEXT          NULL,
    config                TEXT          NULL,
    priority              INTEGER       NOT NULL DEFAULT 0,
    status                VARCHAR(255)  NOT NULL DEFAULT 'unknown',
    last_check_at         TIMESTAMPTZ   NULL,
    last_latency_ms       INTEGER       NULL,
    error_count           INTEGER       NOT NULL DEFAULT 0,
    consecutive_failures  INTEGER       NOT NULL DEFAULT 0,
    last_error            TEXT          NULL,
    last_error_at         TIMESTAMPTZ   NULL,
    is_enabled            BOOLEAN       NOT NULL DEFAULT TRUE,
    is_env_managed        BOOLEAN       NOT NULL DEFAULT FALSE,
    created_at            TIMESTAMPTZ   NULL,
    updated_at            TIMESTAMPTZ   NULL,

    CONSTRAINT external_services_uuid_unique UNIQUE (uuid),
    CONSTRAINT external_services_slug_unique UNIQUE (slug)
);

CREATE INDEX idx_external_services_type_is_enabled ON external_services (type, is_enabled);
CREATE INDEX idx_external_services_type_priority ON external_services (type, priority);
CREATE INDEX idx_external_services_status ON external_services (status);


-- ============================================================
-- Table: zim_archives
-- Migration: 2025_11_28_000001_create_zim_archives_table.php
-- Model:     App\Models\ZimArchive
-- ============================================================
CREATE TABLE zim_archives (
    id                     BIGSERIAL     PRIMARY KEY,
    uuid                   UUID          NOT NULL,
    name                   VARCHAR(255)  NOT NULL,
    slug                   VARCHAR(255)  NOT NULL,
    kiwix_id               VARCHAR(255)  NULL,
    title                  VARCHAR(255)  NOT NULL,
    description            TEXT          NULL,
    language               VARCHAR(10)   NOT NULL DEFAULT 'en',
    category               VARCHAR(255)  NULL,
    creator                VARCHAR(255)  NULL,
    publisher              VARCHAR(255)  NULL,
    favicon                TEXT          NULL,
    article_count          BIGINT        NOT NULL DEFAULT 0,
    media_count            BIGINT        NOT NULL DEFAULT 0,
    file_size              BIGINT        NOT NULL DEFAULT 0,
    tags                   TEXT          NULL,
    external_service_id    BIGINT        NULL,
    is_enabled             BOOLEAN       NOT NULL DEFAULT TRUE,
    is_searchable          BOOLEAN       NOT NULL DEFAULT TRUE,
    last_synced_at         TIMESTAMPTZ   NULL,
    meilisearch_indexed_at TIMESTAMPTZ   NULL,
    created_at             TIMESTAMPTZ   NULL,
    updated_at             TIMESTAMPTZ   NULL,

    CONSTRAINT zim_archives_uuid_unique UNIQUE (uuid),
    CONSTRAINT zim_archives_name_unique UNIQUE (name),
    CONSTRAINT zim_archives_slug_unique UNIQUE (slug),
    CONSTRAINT zim_archives_external_service_id_foreign
        FOREIGN KEY (external_service_id) REFERENCES external_services (id) ON DELETE SET NULL
);

CREATE INDEX idx_zim_archives_name ON zim_archives (name);
CREATE INDEX idx_zim_archives_category ON zim_archives (category);
CREATE INDEX idx_zim_archives_language ON zim_archives (language);
CREATE INDEX idx_zim_archives_is_enabled_is_searchable ON zim_archives (is_enabled, is_searchable);
CREATE INDEX idx_zim_archives_external_service_id ON zim_archives (external_service_id);


-- ============================================================
-- Table: git_templates
-- Migration: 2026_01_26_000001_create_git_templates_table.php
-- Model:     App\Models\GitTemplate
-- ============================================================
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


-- ============================================================
-- Table: git_template_files
-- Migration: 2026_01_26_000002_create_git_template_files_table.php
-- Model:     App\Models\GitTemplateFile
-- ============================================================
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


-- ============================================================
-- Table: pulse_values
-- Migration: 2025_11_21_014306_create_pulse_tables.php
-- Note:      Laravel Pulse framework table (PostgreSQL variant)
-- ============================================================
CREATE TABLE pulse_values (
    id        BIGSERIAL     PRIMARY KEY,
    timestamp INTEGER       NOT NULL,
    type      VARCHAR(255)  NOT NULL,
    key       TEXT          NOT NULL,
    key_hash  UUID          NOT NULL GENERATED ALWAYS AS (md5("key")::uuid) STORED,
    value     TEXT          NOT NULL,

    CONSTRAINT pulse_values_type_key_hash_unique UNIQUE (type, key_hash)
);

CREATE INDEX idx_pulse_values_timestamp ON pulse_values (timestamp);
CREATE INDEX idx_pulse_values_type ON pulse_values (type);


-- ============================================================
-- Table: pulse_entries
-- Migration: 2025_11_21_014306_create_pulse_tables.php
-- Note:      Laravel Pulse framework table (PostgreSQL variant)
-- ============================================================
CREATE TABLE pulse_entries (
    id        BIGSERIAL     PRIMARY KEY,
    timestamp INTEGER       NOT NULL,
    type      VARCHAR(255)  NOT NULL,
    key       TEXT          NOT NULL,
    key_hash  UUID          NOT NULL GENERATED ALWAYS AS (md5("key")::uuid) STORED,
    value     BIGINT        NULL
);

CREATE INDEX idx_pulse_entries_timestamp ON pulse_entries (timestamp);
CREATE INDEX idx_pulse_entries_type ON pulse_entries (type);
CREATE INDEX idx_pulse_entries_key_hash ON pulse_entries (key_hash);
CREATE INDEX idx_pulse_entries_timestamp_type_key_hash_value ON pulse_entries (timestamp, type, key_hash, value);


-- ============================================================
-- Table: pulse_aggregates
-- Migration: 2025_11_21_014306_create_pulse_tables.php
-- Note:      Laravel Pulse framework table (PostgreSQL variant)
-- ============================================================
CREATE TABLE pulse_aggregates (
    id        BIGSERIAL       PRIMARY KEY,
    bucket    INTEGER         NOT NULL,
    period    INTEGER         NOT NULL,
    type      VARCHAR(255)    NOT NULL,
    key       TEXT            NOT NULL,
    key_hash  UUID            NOT NULL GENERATED ALWAYS AS (md5("key")::uuid) STORED,
    aggregate VARCHAR(255)    NOT NULL,
    value     DECIMAL(20, 2)  NOT NULL,
    count     INTEGER         NULL,

    CONSTRAINT pulse_aggregates_bucket_period_type_aggregate_key_hash_unique
        UNIQUE (bucket, period, type, aggregate, key_hash)
);

CREATE INDEX idx_pulse_aggregates_period_bucket ON pulse_aggregates (period, bucket);
CREATE INDEX idx_pulse_aggregates_type ON pulse_aggregates (type);
CREATE INDEX idx_pulse_aggregates_period_type_aggregate_bucket ON pulse_aggregates (period, type, aggregate, bucket);


-- ============================================================
-- Laravel internal: migrations tracking table
-- Note:      Auto-created by Laravel framework, not in migrations/
-- ============================================================
CREATE TABLE migrations (
    id        SERIAL        PRIMARY KEY,
    migration VARCHAR(255)  NOT NULL,
    batch     INTEGER       NOT NULL
);
