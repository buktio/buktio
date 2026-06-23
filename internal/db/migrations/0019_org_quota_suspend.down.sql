ALTER TABLE organizations DROP COLUMN IF EXISTS quota_max_bytes;
ALTER TABLE organizations DROP COLUMN IF EXISTS suspend_reason;
ALTER TABLE organizations DROP COLUMN IF EXISTS suspended_at;
