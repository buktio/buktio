ALTER TABLE backup_jobs DROP COLUMN IF EXISTS schedule_id;
ALTER TABLE backup_jobs DROP COLUMN IF EXISTS offsite_uri;
DROP TABLE IF EXISTS backup_schedules;
