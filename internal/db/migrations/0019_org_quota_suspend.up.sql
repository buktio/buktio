-- 0019: Enterprise hard-multi-tenancy enforcement metadata.
--
-- Tenant lifecycle: suspended_at/suspend_reason record when and why an org was
-- suspended (status already has the 'suspended' enum value from 0001). A suspended
-- org's sessions are rejected by the auth middleware.
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS suspended_at   timestamptz;
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS suspend_reason text;

-- Org-level quota: a per-org storage ceiling (bytes). NULL = no org-level cap
-- (the license-derived entitlements.Limit still applies). This is the soft,
-- buktio-enforced ceiling checked at bucket-create and API-proxied upload time;
-- Garage's per-bucket maxSize remains the hard, write-time enforcer.
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS quota_max_bytes bigint;
