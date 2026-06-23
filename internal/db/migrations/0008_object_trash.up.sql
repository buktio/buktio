-- 0008: app-level trash for objects. Garage has no object versioning, so a
-- deleted object is physically MOVED to a reserved ".trash/<ts>/<key>" prefix in
-- the same bucket; this table records it for restore/purge. A scheduled purge
-- hard-deletes after purge_after.

CREATE TABLE object_trash (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    bucket_id    uuid        NOT NULL REFERENCES buckets(id) ON DELETE CASCADE,
    org_id       uuid        NOT NULL,
    original_key text        NOT NULL,
    trash_key    text        NOT NULL,                 -- where the object now lives
    size_bytes   bigint      NOT NULL DEFAULT 0,
    deleted_at   timestamptz NOT NULL DEFAULT now(),
    purge_after  timestamptz NOT NULL,
    restored_at  timestamptz
);
CREATE INDEX ix_object_trash_bucket ON object_trash (bucket_id, deleted_at DESC) WHERE restored_at IS NULL;
CREATE INDEX ix_object_trash_purge  ON object_trash (purge_after) WHERE restored_at IS NULL;
