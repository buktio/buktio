-- 0001 down

DROP FUNCTION IF EXISTS set_updated_at();

DROP TYPE IF EXISTS install_step;
DROP TYPE IF EXISTS actor_type;
DROP TYPE IF EXISTS bucket_visibility;
DROP TYPE IF EXISTS cluster_status;
DROP TYPE IF EXISTS provider_type;
DROP TYPE IF EXISTS org_member_role;
DROP TYPE IF EXISTS org_status;

-- Extensions are left in place (other schemas may rely on them).
