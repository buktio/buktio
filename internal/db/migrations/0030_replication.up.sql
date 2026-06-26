-- Cross-backend replication (OSS, free): a one-shot, resumable copy of one buktio
-- bucket's objects into another buktio bucket — possibly on a different storage
-- backend (e.g. mirror a local Garage bucket to an off-site S3 provider). Re-running
-- skips objects already present with the same size, so it acts as an incremental
-- sync. Scheduled/continuous orchestration for teams stays a paid concern.
CREATE TABLE replication_jobs (
    id             uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id         uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    src_bucket_id  uuid        NOT NULL REFERENCES buckets(id)        ON DELETE CASCADE,
    dst_bucket_id  uuid        NOT NULL REFERENCES buckets(id)        ON DELETE CASCADE,
    status         text        NOT NULL DEFAULT 'pending',  -- pending|running|completed|failed|canceled
    copied_objects bigint      NOT NULL DEFAULT 0,
    skipped_objects bigint     NOT NULL DEFAULT 0,
    copied_bytes   bigint      NOT NULL DEFAULT 0,
    cursor         text        NOT NULL DEFAULT '',
    error          text        NOT NULL DEFAULT '',
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_replication_jobs_src ON replication_jobs (src_bucket_id, created_at DESC);
