-- 0016: scheduled metadata backups (Pro). A schedule runs a pg_dump every
-- interval_minutes, keeps retention_count newest succeeded dumps, and optionally
-- copies each off-box to an S3 target. backup_jobs records the off-box URI + the
-- originating schedule.
CREATE TABLE backup_schedules (
    id               uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id           uuid        REFERENCES organizations(id) ON DELETE CASCADE,
    enabled          boolean     NOT NULL DEFAULT true,
    interval_minutes integer     NOT NULL DEFAULT 1440,   -- daily
    retention_count  integer     NOT NULL DEFAULT 7,
    offsite_enabled  boolean     NOT NULL DEFAULT false,
    next_run_at      timestamptz NOT NULL DEFAULT now(),
    last_run_at      timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ix_backup_schedules_due ON backup_schedules (next_run_at) WHERE enabled;

ALTER TABLE backup_jobs ADD COLUMN offsite_uri text NOT NULL DEFAULT '';
ALTER TABLE backup_jobs ADD COLUMN schedule_id uuid REFERENCES backup_schedules(id);
