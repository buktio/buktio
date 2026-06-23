-- 0015: org member invitations (Pro multi-user). An invite carries a one-time
-- token (hashed); accepting it creates/links a user and an organization_members row.
CREATE TABLE invitations (
    id          uuid            PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      uuid            NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email       citext          NOT NULL,
    role        org_member_role NOT NULL DEFAULT 'member',
    token_hash  bytea           NOT NULL,
    invited_by  uuid            REFERENCES users(id),
    expires_at  timestamptz     NOT NULL,
    accepted_at timestamptz,
    created_at  timestamptz     NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_invitation_token ON invitations (token_hash);
CREATE INDEX ix_invitation_org_email ON invitations (org_id, lower(email::text)) WHERE accepted_at IS NULL;
