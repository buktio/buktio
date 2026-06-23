-- 0012: per-(access-key, bucket, method) traffic counters, flushed periodically by
-- the buktio-s3proxy that fronts the S3 plane. Garage exposes no per-key/per-bucket
-- request or egress metrics, so the proxy is the only source of this billing-grade
-- traffic data. Append-only time series (like usage_snapshots).
CREATE TABLE traffic_snapshots (
    id                 uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    storage_cluster_id uuid        REFERENCES storage_clusters(id) ON DELETE CASCADE,
    access_key_id      text        NOT NULL,            -- S3 access key id (e.g. GK...)
    bucket             text        NOT NULL DEFAULT '', -- bucket name (path-style)
    method             text        NOT NULL,            -- GET/PUT/POST/DELETE/HEAD
    requests           bigint      NOT NULL DEFAULT 0,
    bytes_in           bigint      NOT NULL DEFAULT 0,  -- request body bytes
    bytes_out          bigint      NOT NULL DEFAULT 0,  -- response body bytes (egress)
    captured_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_traffic_key_time    ON traffic_snapshots (access_key_id, captured_at);
CREATE INDEX idx_traffic_bucket_time ON traffic_snapshots (bucket, captured_at);
