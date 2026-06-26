-- PostgreSQL cannot drop a value from an enum type, so this is a no-op. The added
-- provider_type values (wasabi/storj/hetzner/gcs/minio) are harmless if unused.
SELECT 1;
