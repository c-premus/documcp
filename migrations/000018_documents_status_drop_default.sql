-- +goose Up

-- Drop the documents.status DEFAULT 'processing' inherited from migration
-- 000002. Migration 000014 added CHECK (status IN ('pending', 'uploaded',
-- 'indexed', 'failed')), so the default value would silently violate the
-- constraint if any future code path ever omitted the column. Every
-- production writer already sets status explicitly (see
-- service/document_service.go and service/document_pipeline.go), so the
-- default is dead today.
--
-- Dropping the default — rather than changing it to a valid value like
-- 'pending' — ensures that any future caller which forgets to set status
-- fails loudly at INSERT with a NOT NULL violation, instead of quietly
-- falling back to a fallback value and hiding the omission.

ALTER TABLE documents ALTER COLUMN status DROP DEFAULT;

-- +goose Down

ALTER TABLE documents ALTER COLUMN status SET DEFAULT 'processing';
