-- 0027: S3-to-S3 migration import (Hosted onboarding). A job copies every object
-- from a source S3 bucket into a buktio bucket, resumably (cursor) with progress.
-- The source secret is envelope-encrypted like every other stored credential.
CREATE TABLE migration_jobs (
    id                uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id            uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    source_endpoint   text        NOT NULL,
    source_region     text        NOT NULL DEFAULT 'us-east-1',
    source_bucket     text        NOT NULL,
    source_access_key text        NOT NULL,
    source_secret_enc bytea       NOT NULL,            -- encrypted at rest
    dest_bucket_id    uuid        NOT NULL REFERENCES buckets(id) ON DELETE CASCADE,
    status            text        NOT NULL DEFAULT 'pending', -- pending|running|completed|failed|canceled
    copied_objects    bigint      NOT NULL DEFAULT 0,
    copied_bytes      bigint      NOT NULL DEFAULT 0,
    cursor            text        NOT NULL DEFAULT '',  -- S3 continuation token for resume
    error             text,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ix_migration_jobs_org    ON migration_jobs (org_id, created_at DESC);
CREATE INDEX ix_migration_jobs_active ON migration_jobs (status) WHERE status IN ('pending','running');
