-- 0001: extensions, enum types, and the shared updated_at trigger function.

CREATE EXTENSION IF NOT EXISTS pgcrypto;   -- gen_random_uuid()
CREATE EXTENSION IF NOT EXISTS citext;     -- case-insensitive email

-- Closed value sets. (Tables attach the set_updated_at trigger themselves.)
CREATE TYPE org_status        AS ENUM ('active', 'suspended', 'pending_setup');
CREATE TYPE org_member_role   AS ENUM ('owner', 'admin', 'member', 'viewer');
CREATE TYPE provider_type     AS ENUM ('garage', 'seaweedfs', 'ceph_rgw', 'aws_s3', 'r2', 'b2');
CREATE TYPE cluster_status    AS ENUM ('provisioning', 'healthy', 'degraded', 'unavailable', 'disabled');
CREATE TYPE bucket_visibility AS ENUM ('private', 'public_website');
CREATE TYPE actor_type        AS ENUM ('user', 'system', 'api');
CREATE TYPE install_step      AS ENUM (
    'created',
    'admin_created',
    'cluster_provisioned',
    'first_org_created',
    'completed'
);

-- Maintains updated_at on UPDATE. Attached per mutable table in later migrations.
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
