-- 0021: SCIM 2.0 provisioning tokens (Enterprise). Each org issues one or more
-- bearer tokens its IdP (Okta/Azure AD/etc.) uses to provision users/groups via
-- /scim/v2. Only the hash is stored (like sessions/PATs); the raw token is shown
-- once at creation.
CREATE TABLE scim_tokens (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id       uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name         text        NOT NULL,
    token_hash   bytea       NOT NULL,
    last_four    text,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz,
    deleted_at   timestamptz
);
CREATE UNIQUE INDEX uq_scim_token_hash ON scim_tokens (token_hash) WHERE deleted_at IS NULL;
CREATE INDEX        ix_scim_tokens_org ON scim_tokens (org_id)     WHERE deleted_at IS NULL;
