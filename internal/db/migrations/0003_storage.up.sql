-- 0003: the storage backend (a Garage deployment) and its nodes. The provider
-- column makes the schema multi-backend ready. Master secrets are ENCRYPTED at
-- rest (envelope/AES-256-GCM), never hashed, because buktio must replay them.

CREATE TABLE storage_clusters (
    id                      uuid           PRIMARY KEY DEFAULT gen_random_uuid(),
    name                    text           NOT NULL,
    provider                provider_type  NOT NULL DEFAULT 'garage',

    -- Endpoints (internal addresses). S3 plane :3900, admin plane :3903.
    s3_endpoint             text           NOT NULL,
    admin_endpoint          text           NOT NULL,
    s3_region               text           NOT NULL DEFAULT 'garage',
    web_endpoint            text,                                  -- :3902 static-site host

    -- Version guard: pin v2; warn < 2.0; --single-node needs >= 2.3.
    garage_version          text,
    admin_api_version       text           NOT NULL DEFAULT 'v2',

    -- INFRA SECRETS — encrypted at rest. Never sent to the frontend.
    rpc_secret_enc          bytea          NOT NULL,
    admin_token_enc         bytea          NOT NULL,
    metrics_token_enc       bytea,

    -- buktio's own S3 system key (owner perms) for object-plane operations.
    system_s3_access_key_id text,
    system_s3_secret_enc    bytea,

    db_engine               text           NOT NULL DEFAULT 'sqlite',  -- appliance durability default
    replication_factor      integer        NOT NULL DEFAULT 1,
    status                  cluster_status NOT NULL DEFAULT 'provisioning',
    last_health_at          timestamptz,
    last_health_detail      jsonb,                                 -- cached GetClusterHealth payload

    created_at              timestamptz    NOT NULL DEFAULT now(),
    updated_at              timestamptz    NOT NULL DEFAULT now(),
    deleted_at              timestamptz
);
CREATE TRIGGER trg_storage_clusters_updated_at BEFORE UPDATE ON storage_clusters
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Projection of Garage GetClusterStatus; reconciled by the health collector.
CREATE TABLE storage_nodes (
    id                 uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    storage_cluster_id uuid        NOT NULL REFERENCES storage_clusters(id) ON DELETE CASCADE,
    garage_node_id     text        NOT NULL,
    hostname           text,
    addr               text,
    zone               text,
    capacity_bytes     bigint,                          -- NULL => gateway, not storage node
    role               text,
    is_up              boolean     NOT NULL DEFAULT true,
    draining           boolean     NOT NULL DEFAULT false,
    disk_total_bytes   bigint,
    disk_avail_bytes   bigint,
    last_seen_at       timestamptz NOT NULL DEFAULT now(),
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_storage_node_garage_id ON storage_nodes (storage_cluster_id, garage_node_id);
CREATE TRIGGER trg_storage_nodes_updated_at BEFORE UPDATE ON storage_nodes
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
