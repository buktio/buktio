-- 0004: buckets, access keys, and the per-key permission grants. These hold the
-- buktio-entity <-> Garage-UUID mapping. User-facing S3 secrets are NOT stored:
-- only a fingerprint (secret_hash) + last-four for display.

CREATE TABLE buckets (
    id                  uuid              PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id              uuid              NOT NULL REFERENCES organizations(id)    ON DELETE RESTRICT,
    project_id          uuid              NOT NULL REFERENCES projects(id)         ON DELETE RESTRICT,
    storage_cluster_id  uuid              NOT NULL REFERENCES storage_clusters(id) ON DELETE RESTRICT,

    name                text              NOT NULL,                 -- buktio display name

    -- Garage mapping.
    garage_bucket_id    text              NOT NULL,                 -- Garage internal UUID (hex)
    garage_global_alias text              NOT NULL,                 -- cluster-wide alias buktio assigns

    -- Access model: Garage has no S3 policy/ACL; only website + per-key bits.
    visibility          bucket_visibility NOT NULL DEFAULT 'private',
    website_enabled     boolean           NOT NULL DEFAULT false,
    website_index_doc   text              NOT NULL DEFAULT 'index.html',
    website_error_doc   text,

    -- Quotas: nullable, mirror Garage UpdateBucket quotas; NOT billing in MVP.
    quota_max_size      bigint,
    quota_max_objects   bigint,

    cors_config         jsonb,                                      -- last-applied S3 CORS rules (cache)

    created_at          timestamptz       NOT NULL DEFAULT now(),
    updated_at          timestamptz       NOT NULL DEFAULT now(),
    deleted_at          timestamptz
);
CREATE UNIQUE INDEX uq_bucket_name_per_project_live
    ON buckets (project_id, name) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX uq_bucket_garage_id
    ON buckets (storage_cluster_id, garage_bucket_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX uq_bucket_global_alias
    ON buckets (storage_cluster_id, garage_global_alias) WHERE deleted_at IS NULL;
CREATE INDEX ix_buckets_project ON buckets (project_id) WHERE deleted_at IS NULL;
CREATE INDEX ix_buckets_org     ON buckets (org_id)     WHERE deleted_at IS NULL;
CREATE TRIGGER trg_buckets_updated_at BEFORE UPDATE ON buckets
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE access_keys (
    id                   uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id               uuid        NOT NULL REFERENCES organizations(id)    ON DELETE RESTRICT,
    project_id           uuid        REFERENCES projects(id)                  ON DELETE RESTRICT,
    storage_cluster_id   uuid        NOT NULL REFERENCES storage_clusters(id) ON DELETE RESTRICT,

    name                 text        NOT NULL,

    -- Garage mapping. The 'GK...' id is an identifier (not secret) -> plaintext.
    garage_access_key_id text        NOT NULL,

    -- Secret lifecycle: shown once at CreateKey, then DISCARDED. No material stored.
    secret_hash          bytea       NOT NULL,                       -- HMAC/SHA-256(secret), verify-only
    secret_last_four     text,
    secret_revealed_at   timestamptz,

    can_create_bucket    boolean     NOT NULL DEFAULT false,
    expires_at           timestamptz,
    last_used_at         timestamptz,

    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),
    deleted_at           timestamptz
);
CREATE UNIQUE INDEX uq_key_garage_id
    ON access_keys (storage_cluster_id, garage_access_key_id) WHERE deleted_at IS NULL;
CREATE INDEX ix_keys_project ON access_keys (project_id) WHERE deleted_at IS NULL;
CREATE INDEX ix_keys_org     ON access_keys (org_id)     WHERE deleted_at IS NULL;
CREATE TRIGGER trg_access_keys_updated_at BEFORE UPDATE ON access_keys
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Mirror of Garage AllowBucketKey grants (the read/write/owner bitmask) — the
-- common-denominator the StorageProvider policy abstraction maps to.
CREATE TABLE bucket_permissions (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    bucket_id     uuid        NOT NULL REFERENCES buckets(id)     ON DELETE CASCADE,
    access_key_id uuid        NOT NULL REFERENCES access_keys(id) ON DELETE CASCADE,
    can_read      boolean     NOT NULL DEFAULT false,
    can_write     boolean     NOT NULL DEFAULT false,
    is_owner      boolean     NOT NULL DEFAULT false,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    deleted_at    timestamptz
);
CREATE UNIQUE INDEX uq_bucketkey_live
    ON bucket_permissions (bucket_id, access_key_id) WHERE deleted_at IS NULL;
CREATE INDEX ix_bucketperm_bucket ON bucket_permissions (bucket_id);
CREATE INDEX ix_bucketperm_key    ON bucket_permissions (access_key_id);
CREATE TRIGGER trg_bucket_permissions_updated_at BEFORE UPDATE ON bucket_permissions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
