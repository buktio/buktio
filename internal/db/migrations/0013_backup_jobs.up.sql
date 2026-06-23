-- 0013: backup job records. buktio backs up its OWN state (PostgreSQL metadata +
-- non-secret config) — NEVER the KEK and NEVER Garage object data (operator's
-- responsibility, documented). The api runs pg_dump into the backups volume.
CREATE TYPE backup_kind   AS ENUM ('metadata', 'config');
CREATE TYPE backup_status AS ENUM ('pending', 'running', 'succeeded', 'failed');

CREATE TABLE backup_jobs (
    id          uuid          PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      uuid          REFERENCES organizations(id) ON DELETE SET NULL,
    kind        backup_kind   NOT NULL DEFAULT 'metadata',
    status      backup_status NOT NULL DEFAULT 'pending',
    path        text          NOT NULL DEFAULT '',
    size_bytes  bigint        NOT NULL DEFAULT 0,
    error       text          NOT NULL DEFAULT '',
    started_at  timestamptz,
    finished_at timestamptz,
    created_at  timestamptz   NOT NULL DEFAULT now()
);
CREATE INDEX idx_backup_jobs_created ON backup_jobs (created_at DESC);
