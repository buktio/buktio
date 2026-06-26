-- buktio SQLite baseline schema (ADR 0001 phase 3).
--
-- This is the consolidated SQLite-dialect schema equivalent to PostgreSQL
-- migrations 0001-0030 applied in order. SQLite is a brand-new, OSS-only,
-- single-node backend, so there are no existing SQLite databases to migrate
-- incrementally — one baseline at version 30 is cleaner and avoids un-portable
-- intermediate steps (e.g. 0010/0011 ALTER COLUMN DROP NOT NULL, which SQLite
-- cannot express). Future migrations add 0031+.sql in SQLite dialect.
--
-- Dialect mapping from the Postgres source:
--   uuid/citext/jsonb/inet/<enum>             -> TEXT      bytea -> BLOB
--   timestamptz                               -> DATETIME  (modernc round-trips time.Time)
--   integer/bigint/boolean                    -> INTEGER   (bool stored 0/1)
--   gen_random_uuid()  -> (gen_random_uuid())  [registered Go scalar fn, sqlite.go]
--   now()              -> CURRENT_TIMESTAMP
--   GENERATED ALWAYS AS IDENTITY PRIMARY KEY   -> INTEGER PRIMARY KEY AUTOINCREMENT
--   ENUM types         -> TEXT + CHECK (col IN (...))  (preserves the value set)
--   set_updated_at() BEFORE-UPDATE trigger     -> per-table AFTER-UPDATE trigger
-- Row-Level Security (0018) is omitted: it is Postgres-only defence-in-depth for
-- hard multi-tenancy (Enterprise); single-node SQLite relies on the app-layer
-- org_id filters, which are identical on both backends.

-- 0002: tenancy backbone -----------------------------------------------------
CREATE TABLE users (
    id                TEXT    PRIMARY KEY DEFAULT (gen_random_uuid()),
    email             TEXT    NOT NULL,
    email_verified_at DATETIME,
    password_hash     TEXT    NOT NULL,
    full_name         TEXT,
    is_platform_admin INTEGER NOT NULL DEFAULT 0,
    last_login_at     DATETIME,
    email_verified    INTEGER NOT NULL DEFAULT 1,                  -- 0025
    created_at        DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at        DATETIME
);
CREATE UNIQUE INDEX uq_users_email_live ON users (lower(email)) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_users_updated_at AFTER UPDATE ON users FOR EACH ROW
BEGIN UPDATE users SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id; END;

CREATE TABLE organizations (
    id                TEXT    PRIMARY KEY DEFAULT (gen_random_uuid()),
    name              TEXT    NOT NULL,
    slug              TEXT    NOT NULL,
    status            TEXT    NOT NULL DEFAULT 'active'
                              CHECK (status IN ('active','suspended','pending_setup')),
    max_projects      INTEGER,
    max_storage_bytes INTEGER,
    suspended_at      DATETIME,                                        -- 0019
    suspend_reason    TEXT,                                        -- 0019
    quota_max_bytes   INTEGER,                                     -- 0019
    created_at        DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at        DATETIME
);
CREATE UNIQUE INDEX uq_org_slug_live ON organizations (slug) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_org_updated_at AFTER UPDATE ON organizations FOR EACH ROW
BEGIN UPDATE organizations SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id; END;

CREATE TABLE organization_members (
    org_id     TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id    TEXT NOT NULL REFERENCES users(id)         ON DELETE CASCADE,
    role       TEXT NOT NULL DEFAULT 'member'
                    CHECK (role IN ('owner','admin','member','viewer')),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (org_id, user_id)
);
CREATE INDEX ix_org_members_user ON organization_members (user_id);

CREATE TABLE projects (
    id                      TEXT    PRIMARY KEY DEFAULT (gen_random_uuid()),
    org_id                  TEXT    NOT NULL REFERENCES organizations(id) ON DELETE RESTRICT,
    name                    TEXT    NOT NULL,
    slug                    TEXT    NOT NULL,
    description             TEXT,
    quota_max_storage_bytes INTEGER,
    quota_max_objects       INTEGER,
    created_at              DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at              DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at              DATETIME
);
CREATE UNIQUE INDEX uq_project_slug_per_org_live ON projects (org_id, slug) WHERE deleted_at IS NULL;
CREATE INDEX ix_projects_org ON projects (org_id) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_projects_updated_at AFTER UPDATE ON projects FOR EACH ROW
BEGIN UPDATE projects SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id; END;

CREATE TABLE sessions (
    id         TEXT PRIMARY KEY DEFAULT (gen_random_uuid()),
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash BLOB NOT NULL,
    ip_address TEXT,
    user_agent TEXT,
    org_id     TEXT REFERENCES organizations(id),                 -- 0014
    expires_at DATETIME NOT NULL,
    revoked_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX uq_sessions_token  ON sessions (token_hash);
CREATE INDEX        ix_sessions_user   ON sessions (user_id);
CREATE INDEX        ix_sessions_expiry ON sessions (expires_at);

-- 0003: storage clusters + nodes ---------------------------------------------
CREATE TABLE storage_clusters (
    id                      TEXT    PRIMARY KEY DEFAULT (gen_random_uuid()),
    name                    TEXT    NOT NULL,
    provider                TEXT    NOT NULL DEFAULT 'garage'
                                    CHECK (provider IN ('garage','seaweedfs','ceph_rgw','aws_s3',
                                                        'r2','b2','wasabi','storj','hetzner','gcs','minio')),
    s3_endpoint             TEXT    NOT NULL,
    admin_endpoint          TEXT    NOT NULL,
    s3_region               TEXT    NOT NULL DEFAULT 'garage',
    web_endpoint            TEXT,
    garage_version          TEXT,
    admin_api_version       TEXT    NOT NULL DEFAULT 'v2',
    rpc_secret_enc          BLOB,                                  -- nullable since 0010
    admin_token_enc         BLOB,                                  -- nullable since 0011
    metrics_token_enc       BLOB,
    system_s3_access_key_id TEXT,
    system_s3_secret_enc    BLOB,
    db_engine               TEXT    NOT NULL DEFAULT 'sqlite',
    replication_factor      INTEGER NOT NULL DEFAULT 1,
    status                  TEXT    NOT NULL DEFAULT 'provisioning'
                                    CHECK (status IN ('provisioning','healthy','degraded','unavailable','disabled')),
    mode                    TEXT    NOT NULL DEFAULT 'managed'     -- 0010
                                    CHECK (mode IN ('managed','external')),
    last_health_at          DATETIME,
    last_health_detail      TEXT,
    created_at              DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at              DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at              DATETIME
);
CREATE TRIGGER trg_storage_clusters_updated_at AFTER UPDATE ON storage_clusters FOR EACH ROW
BEGIN UPDATE storage_clusters SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id; END;

CREATE TABLE storage_nodes (
    id                 TEXT    PRIMARY KEY DEFAULT (gen_random_uuid()),
    storage_cluster_id TEXT    NOT NULL REFERENCES storage_clusters(id) ON DELETE CASCADE,
    garage_node_id     TEXT    NOT NULL,
    hostname           TEXT,
    addr               TEXT,
    zone               TEXT,
    capacity_bytes     INTEGER,
    role               TEXT,
    is_up              INTEGER NOT NULL DEFAULT 1,
    draining           INTEGER NOT NULL DEFAULT 0,
    disk_total_bytes   INTEGER,
    disk_avail_bytes   INTEGER,
    last_seen_at       DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at         DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX uq_storage_node_garage_id ON storage_nodes (storage_cluster_id, garage_node_id);
CREATE TRIGGER trg_storage_nodes_updated_at AFTER UPDATE ON storage_nodes FOR EACH ROW
BEGIN UPDATE storage_nodes SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id; END;

-- 0004: buckets, access keys, permissions ------------------------------------
CREATE TABLE buckets (
    id                  TEXT    PRIMARY KEY DEFAULT (gen_random_uuid()),
    org_id              TEXT    NOT NULL REFERENCES organizations(id)    ON DELETE RESTRICT,
    project_id          TEXT    NOT NULL REFERENCES projects(id)         ON DELETE RESTRICT,
    storage_cluster_id  TEXT    NOT NULL REFERENCES storage_clusters(id) ON DELETE RESTRICT,
    name                TEXT    NOT NULL,
    garage_bucket_id    TEXT    NOT NULL,
    garage_global_alias TEXT    NOT NULL,
    visibility          TEXT    NOT NULL DEFAULT 'private'
                                CHECK (visibility IN ('private','public_website')),
    website_enabled     INTEGER NOT NULL DEFAULT 0,
    website_index_doc   TEXT    NOT NULL DEFAULT 'index.html',
    website_error_doc   TEXT,
    quota_max_size      INTEGER,
    quota_max_objects   INTEGER,
    cors_config         TEXT,
    lifecycle_config    TEXT,                                       -- 0009
    created_at          DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at          DATETIME
);
CREATE UNIQUE INDEX uq_bucket_name_per_project_live ON buckets (project_id, name) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX uq_bucket_garage_id ON buckets (storage_cluster_id, garage_bucket_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX uq_bucket_global_alias ON buckets (storage_cluster_id, garage_global_alias) WHERE deleted_at IS NULL;
CREATE INDEX ix_buckets_project ON buckets (project_id) WHERE deleted_at IS NULL;
CREATE INDEX ix_buckets_org     ON buckets (org_id)     WHERE deleted_at IS NULL;
CREATE TRIGGER trg_buckets_updated_at AFTER UPDATE ON buckets FOR EACH ROW
BEGIN UPDATE buckets SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id; END;

CREATE TABLE access_keys (
    id                   TEXT    PRIMARY KEY DEFAULT (gen_random_uuid()),
    org_id               TEXT    NOT NULL REFERENCES organizations(id)    ON DELETE RESTRICT,
    project_id           TEXT    REFERENCES projects(id)                  ON DELETE RESTRICT,
    storage_cluster_id   TEXT    NOT NULL REFERENCES storage_clusters(id) ON DELETE RESTRICT,
    name                 TEXT    NOT NULL,
    garage_access_key_id TEXT    NOT NULL,
    secret_hash          BLOB    NOT NULL,
    secret_last_four     TEXT,
    secret_revealed_at   DATETIME,
    can_create_bucket    INTEGER NOT NULL DEFAULT 0,
    expires_at           DATETIME,
    last_used_at         DATETIME,
    created_at           DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at           DATETIME
);
CREATE UNIQUE INDEX uq_key_garage_id ON access_keys (storage_cluster_id, garage_access_key_id) WHERE deleted_at IS NULL;
CREATE INDEX ix_keys_project ON access_keys (project_id) WHERE deleted_at IS NULL;
CREATE INDEX ix_keys_org     ON access_keys (org_id)     WHERE deleted_at IS NULL;
CREATE TRIGGER trg_access_keys_updated_at AFTER UPDATE ON access_keys FOR EACH ROW
BEGIN UPDATE access_keys SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id; END;

CREATE TABLE bucket_permissions (
    id            TEXT    PRIMARY KEY DEFAULT (gen_random_uuid()),
    bucket_id     TEXT    NOT NULL REFERENCES buckets(id)     ON DELETE CASCADE,
    access_key_id TEXT    NOT NULL REFERENCES access_keys(id) ON DELETE CASCADE,
    can_read      INTEGER NOT NULL DEFAULT 0,
    can_write     INTEGER NOT NULL DEFAULT 0,
    is_owner      INTEGER NOT NULL DEFAULT 0,
    created_at    DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at    DATETIME
);
CREATE UNIQUE INDEX uq_bucketkey_live ON bucket_permissions (bucket_id, access_key_id) WHERE deleted_at IS NULL;
CREATE INDEX ix_bucketperm_bucket ON bucket_permissions (bucket_id);
CREATE INDEX ix_bucketperm_key    ON bucket_permissions (access_key_id);
CREATE TRIGGER trg_bucket_permissions_updated_at AFTER UPDATE ON bucket_permissions FOR EACH ROW
BEGIN UPDATE bucket_permissions SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id; END;

-- 0005: metering + audit -----------------------------------------------------
CREATE TABLE usage_snapshots (
    id                           INTEGER PRIMARY KEY AUTOINCREMENT,
    bucket_id                    TEXT    NOT NULL REFERENCES buckets(id) ON DELETE CASCADE,
    project_id                   TEXT    NOT NULL,
    org_id                       TEXT    NOT NULL,
    storage_cluster_id           TEXT    NOT NULL,
    provider                     TEXT    NOT NULL DEFAULT 'garage',
    captured_at                  DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    bytes_used                   INTEGER NOT NULL,
    object_count                 INTEGER NOT NULL,
    unfinished_uploads           INTEGER NOT NULL DEFAULT 0,
    unfinished_multipart_uploads INTEGER NOT NULL DEFAULT 0,
    unfinished_multipart_parts   INTEGER NOT NULL DEFAULT 0,
    unfinished_multipart_bytes   INTEGER NOT NULL DEFAULT 0,
    quota_max_size               INTEGER,
    quota_max_objects            INTEGER
);
CREATE INDEX ix_usage_bucket_time  ON usage_snapshots (bucket_id, captured_at DESC);
CREATE INDEX ix_usage_project_time ON usage_snapshots (project_id, captured_at DESC);
CREATE INDEX ix_usage_org_time     ON usage_snapshots (org_id, captured_at DESC);

CREATE TABLE audit_events (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    org_id        TEXT    REFERENCES organizations(id) ON DELETE SET NULL,
    actor_user_id TEXT    REFERENCES users(id)         ON DELETE SET NULL,
    actor_type    TEXT    NOT NULL DEFAULT 'user' CHECK (actor_type IN ('user','system','api')),
    action        TEXT    NOT NULL,
    target_type   TEXT,
    target_id     TEXT,
    metadata      TEXT    NOT NULL DEFAULT '{}',
    ip_address    TEXT,
    prev_hash     BLOB,                                            -- 0023
    row_hash      BLOB,                                            -- 0023
    created_at    DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX ix_audit_org_time    ON audit_events (org_id, created_at DESC);
CREATE INDEX ix_audit_actor_time  ON audit_events (actor_user_id, created_at DESC);
CREATE INDEX ix_audit_action_time ON audit_events (action, created_at DESC);

-- 0006: settings + install FSM -----------------------------------------------
CREATE TABLE system_settings (
    key         TEXT    PRIMARY KEY,
    value       TEXT    NOT NULL,
    is_secret   INTEGER NOT NULL DEFAULT 0,
    description TEXT,
    updated_by  TEXT    REFERENCES users(id) ON DELETE SET NULL,
    created_at  DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TRIGGER trg_system_settings_updated_at AFTER UPDATE ON system_settings FOR EACH ROW
BEGIN UPDATE system_settings SET updated_at = CURRENT_TIMESTAMP WHERE key = NEW.key; END;

CREATE TABLE install_state (
    id             INTEGER PRIMARY KEY CHECK (id = 1),
    step           TEXT    NOT NULL DEFAULT 'created'
                           CHECK (step IN ('created','admin_created','cluster_provisioned','first_org_created','completed')),
    schema_version TEXT,
    instance_id    TEXT    NOT NULL DEFAULT (gen_random_uuid()),
    completed_at   DATETIME,
    details        TEXT    NOT NULL DEFAULT '{}',
    created_at     DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TRIGGER trg_install_state_updated_at AFTER UPDATE ON install_state FOR EACH ROW
BEGIN UPDATE install_state SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id; END;

-- 0007: API tokens -----------------------------------------------------------
CREATE TABLE api_tokens (
    id               TEXT    PRIMARY KEY DEFAULT (gen_random_uuid()),
    user_id          TEXT    NOT NULL REFERENCES users(id)         ON DELETE CASCADE,
    org_id           TEXT    REFERENCES organizations(id)          ON DELETE CASCADE,
    name             TEXT    NOT NULL,
    token_hash       BLOB    NOT NULL,
    secret_last_four TEXT,
    scopes           TEXT    NOT NULL DEFAULT '',                  -- pg text[] -> CSV TEXT on sqlite
    expires_at       DATETIME,
    last_used_at     DATETIME,
    created_at       DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at       DATETIME
);
CREATE UNIQUE INDEX uq_api_tokens_hash ON api_tokens (token_hash);
CREATE INDEX ix_api_tokens_user ON api_tokens (user_id) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_api_tokens_updated_at AFTER UPDATE ON api_tokens FOR EACH ROW
BEGIN UPDATE api_tokens SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id; END;

-- 0008: object trash ---------------------------------------------------------
CREATE TABLE object_trash (
    id           TEXT    PRIMARY KEY DEFAULT (gen_random_uuid()),
    bucket_id    TEXT    NOT NULL REFERENCES buckets(id) ON DELETE CASCADE,
    org_id       TEXT    NOT NULL,
    original_key TEXT    NOT NULL,
    trash_key    TEXT    NOT NULL,
    size_bytes   INTEGER NOT NULL DEFAULT 0,
    deleted_at   DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    purge_after  DATETIME    NOT NULL,
    restored_at  DATETIME
);
CREATE INDEX ix_object_trash_bucket ON object_trash (bucket_id, deleted_at DESC) WHERE restored_at IS NULL;
CREATE INDEX ix_object_trash_purge  ON object_trash (purge_after) WHERE restored_at IS NULL;

-- 0012: traffic snapshots ----------------------------------------------------
CREATE TABLE traffic_snapshots (
    id                 TEXT    PRIMARY KEY DEFAULT (gen_random_uuid()),
    storage_cluster_id TEXT    REFERENCES storage_clusters(id) ON DELETE CASCADE,
    access_key_id      TEXT    NOT NULL,
    bucket             TEXT    NOT NULL DEFAULT '',
    method             TEXT    NOT NULL,
    requests           INTEGER NOT NULL DEFAULT 0,
    bytes_in           INTEGER NOT NULL DEFAULT 0,
    bytes_out          INTEGER NOT NULL DEFAULT 0,
    captured_at        DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_traffic_key_time    ON traffic_snapshots (access_key_id, captured_at);
CREATE INDEX idx_traffic_bucket_time ON traffic_snapshots (bucket, captured_at);

-- 0013 + 0016: backup jobs and schedules -------------------------------------
CREATE TABLE backup_schedules (
    id               TEXT    PRIMARY KEY DEFAULT (gen_random_uuid()),
    org_id           TEXT    REFERENCES organizations(id) ON DELETE CASCADE,
    enabled          INTEGER NOT NULL DEFAULT 1,
    interval_minutes INTEGER NOT NULL DEFAULT 1440,
    retention_count  INTEGER NOT NULL DEFAULT 7,
    offsite_enabled  INTEGER NOT NULL DEFAULT 0,
    next_run_at      DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_run_at      DATETIME,
    created_at       DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX ix_backup_schedules_due ON backup_schedules (next_run_at) WHERE enabled = 1;
CREATE TRIGGER trg_backup_schedules_updated_at AFTER UPDATE ON backup_schedules FOR EACH ROW
BEGIN UPDATE backup_schedules SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id; END;

CREATE TABLE backup_jobs (
    id          TEXT    PRIMARY KEY DEFAULT (gen_random_uuid()),
    org_id      TEXT    REFERENCES organizations(id) ON DELETE SET NULL,
    kind        TEXT    NOT NULL DEFAULT 'metadata' CHECK (kind IN ('metadata','config')),
    status      TEXT    NOT NULL DEFAULT 'pending'  CHECK (status IN ('pending','running','succeeded','failed')),
    path        TEXT    NOT NULL DEFAULT '',
    size_bytes  INTEGER NOT NULL DEFAULT 0,
    error       TEXT    NOT NULL DEFAULT '',
    offsite_uri TEXT    NOT NULL DEFAULT '',                       -- 0016
    schedule_id TEXT    REFERENCES backup_schedules(id),          -- 0016
    started_at  DATETIME,
    finished_at DATETIME,
    created_at  DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_backup_jobs_created ON backup_jobs (created_at DESC);

-- 0015: invitations ----------------------------------------------------------
CREATE TABLE invitations (
    id          TEXT PRIMARY KEY DEFAULT (gen_random_uuid()),
    org_id      TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email       TEXT NOT NULL,
    role        TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('owner','admin','member','viewer')),
    token_hash  BLOB NOT NULL,
    invited_by  TEXT REFERENCES users(id),
    expires_at  DATETIME NOT NULL,
    accepted_at DATETIME,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX uq_invitation_token ON invitations (token_hash);
CREATE INDEX ix_invitation_org_email ON invitations (org_id, lower(email)) WHERE accepted_at IS NULL;

-- 0017: external (SSO) identities --------------------------------------------
CREATE TABLE user_identities (
    id               TEXT PRIMARY KEY DEFAULT (gen_random_uuid()),
    user_id          TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider         TEXT NOT NULL,
    external_subject TEXT NOT NULL,
    created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX uq_user_identity ON user_identities (provider, external_subject);

-- 0020: per-org cluster assignment -------------------------------------------
CREATE TABLE org_storage_clusters (
    id                 TEXT    PRIMARY KEY DEFAULT (gen_random_uuid()),
    org_id             TEXT    NOT NULL REFERENCES organizations(id)    ON DELETE CASCADE,
    storage_cluster_id TEXT    NOT NULL REFERENCES storage_clusters(id) ON DELETE CASCADE,
    is_default         INTEGER NOT NULL DEFAULT 0,
    created_at         DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX uq_org_cluster ON org_storage_clusters (org_id, storage_cluster_id);
CREATE UNIQUE INDEX uq_org_default_cluster ON org_storage_clusters (org_id) WHERE is_default = 1;

-- 0021: SCIM tokens ----------------------------------------------------------
CREATE TABLE scim_tokens (
    id           TEXT PRIMARY KEY DEFAULT (gen_random_uuid()),
    org_id       TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    token_hash   BLOB NOT NULL,
    last_four    TEXT,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at DATETIME,
    deleted_at   DATETIME
);
CREATE UNIQUE INDEX uq_scim_token_hash ON scim_tokens (token_hash) WHERE deleted_at IS NULL;
CREATE INDEX        ix_scim_tokens_org ON scim_tokens (org_id)     WHERE deleted_at IS NULL;

-- 0022: ABAC policy templates ------------------------------------------------
CREATE TABLE policies (
    id         TEXT    PRIMARY KEY DEFAULT (gen_random_uuid()),
    org_id     TEXT    NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name       TEXT    NOT NULL,
    template   TEXT    NOT NULL,
    config     TEXT    NOT NULL DEFAULT '{}',
    enabled    INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX ix_policies_org ON policies (org_id);

CREATE TABLE role_policy_bindings (
    policy_id TEXT NOT NULL REFERENCES policies(id) ON DELETE CASCADE,
    role      TEXT NOT NULL CHECK (role IN ('owner','admin','member','viewer')),
    PRIMARY KEY (policy_id, role)
);

-- 0024: white-label branding -------------------------------------------------
CREATE TABLE org_branding (
    org_id        TEXT PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
    display_name  TEXT,
    logo_url      TEXT,
    primary_color TEXT,
    email_from    TEXT,
    custom_domain TEXT,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX uq_branding_domain ON org_branding (lower(custom_domain)) WHERE custom_domain IS NOT NULL;
CREATE TRIGGER trg_org_branding_updated_at AFTER UPDATE ON org_branding FOR EACH ROW
BEGIN UPDATE org_branding SET updated_at = CURRENT_TIMESTAMP WHERE org_id = NEW.org_id; END;

-- 0025: self-serve signup ----------------------------------------------------
CREATE TABLE email_verifications (
    id          TEXT PRIMARY KEY DEFAULT (gen_random_uuid()),
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  BLOB NOT NULL,
    expires_at  DATETIME NOT NULL,
    consumed_at DATETIME,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX uq_email_verif_token ON email_verifications (token_hash);
CREATE INDEX ix_email_verif_user ON email_verifications (user_id) WHERE consumed_at IS NULL;

CREATE TABLE signup_attempts (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    ip         TEXT,
    email      TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX ix_signup_attempts_ip_time ON signup_attempts (ip, created_at DESC);

-- 0026: usage-based billing --------------------------------------------------
CREATE TABLE billing_customers (
    org_id                 TEXT PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
    stripe_customer_id     TEXT NOT NULL,
    stripe_subscription_id TEXT,
    storage_item_id        TEXT,
    egress_item_id         TEXT,
    requests_item_id       TEXT,
    status                 TEXT NOT NULL DEFAULT 'active',
    created_at             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX uq_billing_stripe_customer ON billing_customers (stripe_customer_id);
CREATE TRIGGER trg_billing_customers_updated_at AFTER UPDATE ON billing_customers FOR EACH ROW
BEGIN UPDATE billing_customers SET updated_at = CURRENT_TIMESTAMP WHERE org_id = NEW.org_id; END;

CREATE TABLE billing_usage_reports (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    org_id          TEXT    NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    metric          TEXT    NOT NULL,
    quantity        INTEGER NOT NULL,
    period_start    DATETIME    NOT NULL,
    period_end      DATETIME    NOT NULL,
    idempotency_key TEXT    NOT NULL,
    reported_at     DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX uq_billing_report_idem ON billing_usage_reports (idempotency_key);

-- 0027: S3-to-S3 migration import --------------------------------------------
CREATE TABLE migration_jobs (
    id                TEXT    PRIMARY KEY DEFAULT (gen_random_uuid()),
    org_id            TEXT    NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    source_endpoint   TEXT    NOT NULL,
    source_region     TEXT    NOT NULL DEFAULT 'us-east-1',
    source_bucket     TEXT    NOT NULL,
    source_access_key TEXT    NOT NULL,
    source_secret_enc BLOB    NOT NULL,
    dest_bucket_id    TEXT    NOT NULL REFERENCES buckets(id) ON DELETE CASCADE,
    status            TEXT    NOT NULL DEFAULT 'pending',
    copied_objects    INTEGER NOT NULL DEFAULT 0,
    copied_bytes      INTEGER NOT NULL DEFAULT 0,
    cursor            TEXT    NOT NULL DEFAULT '',
    error             TEXT,
    created_at        DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX ix_migration_jobs_org    ON migration_jobs (org_id, created_at DESC);
CREATE INDEX ix_migration_jobs_active ON migration_jobs (status) WHERE status IN ('pending','running');
CREATE TRIGGER trg_migration_jobs_updated_at AFTER UPDATE ON migration_jobs FOR EACH ROW
BEGIN UPDATE migration_jobs SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id; END;

-- 0029: app-level event webhooks (OSS) ---------------------------------------
CREATE TABLE webhook_subscriptions (
    id         TEXT    PRIMARY KEY DEFAULT (gen_random_uuid()),
    org_id     TEXT    NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    bucket_id  TEXT    NOT NULL REFERENCES buckets(id)        ON DELETE CASCADE,
    url        TEXT    NOT NULL,
    secret     TEXT    NOT NULL DEFAULT '',
    events     TEXT    NOT NULL DEFAULT '',
    enabled    INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_webhook_subs_bucket ON webhook_subscriptions (bucket_id) WHERE enabled = 1;

-- 0030: cross-backend replication (OSS) --------------------------------------
CREATE TABLE replication_jobs (
    id              TEXT    PRIMARY KEY DEFAULT (gen_random_uuid()),
    org_id          TEXT    NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    src_bucket_id   TEXT    NOT NULL REFERENCES buckets(id)        ON DELETE CASCADE,
    dst_bucket_id   TEXT    NOT NULL REFERENCES buckets(id)        ON DELETE CASCADE,
    status          TEXT    NOT NULL DEFAULT 'pending',
    copied_objects  INTEGER NOT NULL DEFAULT 0,
    skipped_objects INTEGER NOT NULL DEFAULT 0,
    copied_bytes    INTEGER NOT NULL DEFAULT 0,
    cursor          TEXT    NOT NULL DEFAULT '',
    error           TEXT    NOT NULL DEFAULT '',
    created_at      DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_replication_jobs_src ON replication_jobs (src_bucket_id, created_at DESC);
CREATE TRIGGER trg_replication_jobs_updated_at AFTER UPDATE ON replication_jobs FOR EACH ROW
BEGIN UPDATE replication_jobs SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id; END;

-- Structural seed (mirror of 0006) -------------------------------------------
INSERT INTO install_state (id, step) VALUES (1, 'created');
INSERT INTO system_settings (key, value, description) VALUES
    ('garage.pinned_version',    '"v2.3.0"', 'Pinned Garage image/binary version'),
    ('garage.default_db_engine', '"sqlite"', 'Default Garage storage engine for new clusters'),
    ('branding.product_name',    '"buktio"', 'Display name shown in the panel');
