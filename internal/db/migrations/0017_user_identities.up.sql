-- 0017: external (SSO) identities linked to a user. A login via OIDC/SAML matches
-- on (provider, external_subject), auto-provisioning the user on first sign-in.
CREATE TABLE user_identities (
    id               uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider         text        NOT NULL,
    external_subject text        NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_user_identity ON user_identities (provider, external_subject);
