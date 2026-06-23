-- 0010: support "connect existing cluster" mode. External clusters are operator-
-- owned: buktio holds no rpc_secret (it never edits layout/config), so make it
-- nullable, and tag the cluster mode.

CREATE TYPE cluster_mode AS ENUM ('managed', 'external');

ALTER TABLE storage_clusters ADD COLUMN mode cluster_mode NOT NULL DEFAULT 'managed';
ALTER TABLE storage_clusters ALTER COLUMN rpc_secret_enc DROP NOT NULL;
