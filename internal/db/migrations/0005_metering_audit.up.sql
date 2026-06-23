-- 0005: metering (append-only storage snapshots from GetBucketInfo) and the
-- immutable audit log. Garage exposes no per-key/per-bucket TRAFFIC, so egress/
-- request columns are deliberately absent (a future traffic_snapshots table,
-- keyed by access_key, holds those once a metering proxy exists).

CREATE TABLE usage_snapshots (
    id                           bigint        GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    bucket_id                    uuid          NOT NULL REFERENCES buckets(id) ON DELETE CASCADE,
    project_id                   uuid          NOT NULL,                 -- denormalized for rollups
    org_id                       uuid          NOT NULL,                 -- denormalized
    storage_cluster_id           uuid          NOT NULL,
    provider                     provider_type NOT NULL DEFAULT 'garage',

    captured_at                  timestamptz   NOT NULL DEFAULT now(),

    -- From GET /v2/GetBucketInfo (authoritative per-bucket storage signal).
    bytes_used                   bigint        NOT NULL,
    object_count                 bigint        NOT NULL,
    unfinished_uploads           bigint        NOT NULL DEFAULT 0,
    unfinished_multipart_uploads bigint        NOT NULL DEFAULT 0,
    unfinished_multipart_parts   bigint        NOT NULL DEFAULT 0,
    unfinished_multipart_bytes   bigint        NOT NULL DEFAULT 0,

    -- Quota copied at snapshot time (point-in-time record).
    quota_max_size               bigint,
    quota_max_objects            bigint
);
CREATE INDEX ix_usage_bucket_time  ON usage_snapshots (bucket_id, captured_at DESC);
CREATE INDEX ix_usage_project_time ON usage_snapshots (project_id, captured_at DESC);
CREATE INDEX ix_usage_org_time     ON usage_snapshots (org_id, captured_at DESC);

CREATE TABLE audit_events (
    id            bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    org_id        uuid        REFERENCES organizations(id) ON DELETE SET NULL,
    actor_user_id uuid        REFERENCES users(id)         ON DELETE SET NULL,
    actor_type    actor_type  NOT NULL DEFAULT 'user',
    action        text        NOT NULL,                    -- e.g. bucket.create, key.revoke
    target_type   text,
    target_id     text,
    metadata      jsonb       NOT NULL DEFAULT '{}',       -- before/after, request_id; NEVER secrets
    ip_address    inet,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ix_audit_org_time    ON audit_events (org_id, created_at DESC);
CREATE INDEX ix_audit_actor_time  ON audit_events (actor_user_id, created_at DESC);
CREATE INDEX ix_audit_action_time ON audit_events (action, created_at DESC);
