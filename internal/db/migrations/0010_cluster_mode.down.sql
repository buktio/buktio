-- 0010 down
ALTER TABLE storage_clusters ALTER COLUMN rpc_secret_enc SET NOT NULL;
ALTER TABLE storage_clusters DROP COLUMN IF EXISTS mode;
DROP TYPE IF EXISTS cluster_mode;
