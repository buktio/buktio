-- 0009: cache the last-applied S3 lifecycle config per bucket (like cors_config).
ALTER TABLE buckets ADD COLUMN lifecycle_config jsonb;
