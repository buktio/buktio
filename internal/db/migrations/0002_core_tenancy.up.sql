-- 0002: tenancy backbone. The organization is the tenant boundary (org_id is the
-- tenant scope carried, denormalized, on tenant-scoped tables). MVP ships a single
-- default org/project UX, but the model is multi-tenant from day one.

-- Users are platform-global; org membership is a join table.
CREATE TABLE users (
    id                uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    email             citext      NOT NULL,
    email_verified_at timestamptz,
    password_hash     text        NOT NULL,            -- argon2id PHC string
    full_name         text,
    is_platform_admin boolean     NOT NULL DEFAULT false,
    last_login_at     timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    deleted_at        timestamptz
);
CREATE UNIQUE INDEX uq_users_email_live ON users (lower(email)) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE organizations (
    id                uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    name              text        NOT NULL,
    slug              text        NOT NULL,
    status            org_status  NOT NULL DEFAULT 'active',
    -- Reserved for paid tiers; NULL = unlimited; NOT enforced in MVP.
    max_projects      integer,
    max_storage_bytes bigint,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    deleted_at        timestamptz
);
CREATE UNIQUE INDEX uq_org_slug_live ON organizations (slug) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_org_updated_at BEFORE UPDATE ON organizations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE organization_members (
    org_id     uuid            NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id    uuid            NOT NULL REFERENCES users(id)         ON DELETE CASCADE,
    role       org_member_role NOT NULL DEFAULT 'member',
    created_at timestamptz     NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, user_id)
);
CREATE INDEX ix_org_members_user ON organization_members (user_id);

CREATE TABLE projects (
    id                      uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id                  uuid        NOT NULL REFERENCES organizations(id) ON DELETE RESTRICT,
    name                    text        NOT NULL,
    slug                    text        NOT NULL,
    description             text,
    -- Informational/aggregation only in MVP (NOT enforced as billing).
    quota_max_storage_bytes bigint,
    quota_max_objects       bigint,
    created_at              timestamptz NOT NULL DEFAULT now(),
    updated_at              timestamptz NOT NULL DEFAULT now(),
    deleted_at              timestamptz
);
CREATE UNIQUE INDEX uq_project_slug_per_org_live
    ON projects (org_id, slug) WHERE deleted_at IS NULL;
CREATE INDEX ix_projects_org ON projects (org_id) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_projects_updated_at BEFORE UPDATE ON projects
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Server-side sessions. Raw token lives only in the cookie; we store a hash.
CREATE TABLE sessions (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash bytea       NOT NULL,
    ip_address inet,
    user_agent text,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_sessions_token  ON sessions (token_hash);
CREATE INDEX        ix_sessions_user   ON sessions (user_id);
CREATE INDEX        ix_sessions_expiry ON sessions (expires_at);
