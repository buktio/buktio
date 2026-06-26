-- Additional S3-compatible backends buktio can manage as external clusters.
-- All are reached through the generic-S3 data plane (operator credentials, no admin
-- plane), like r2/b2/seaweedfs/ceph_rgw. ADD VALUE IF NOT EXISTS is idempotent and
-- safe to run on PostgreSQL 12+.
ALTER TYPE provider_type ADD VALUE IF NOT EXISTS 'wasabi';
ALTER TYPE provider_type ADD VALUE IF NOT EXISTS 'storj';
ALTER TYPE provider_type ADD VALUE IF NOT EXISTS 'hetzner';
ALTER TYPE provider_type ADD VALUE IF NOT EXISTS 'gcs';
ALTER TYPE provider_type ADD VALUE IF NOT EXISTS 'minio';
