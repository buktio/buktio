-- 0011: support generic-S3 backends (R2, AWS S3, B2, SeaweedFS, Ceph RGW) added
-- at runtime with operator-supplied credentials. They have no admin plane, so the
-- admin token is optional. (rpc_secret_enc is already nullable since 0010; the
-- admin_endpoint can hold '' for these clusters.)
ALTER TABLE storage_clusters ALTER COLUMN admin_token_enc DROP NOT NULL;
