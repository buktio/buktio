-- 0025: self-serve signup (Hosted). New users start unverified; they only get a
-- session after consuming an email-verification token. signup_attempts backs an
-- IP rate limit. OSS/on-prem keep email_verified defaulting to true (existing
-- admins + invite/SCIM-provisioned users are already trusted).
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified boolean NOT NULL DEFAULT true;

CREATE TABLE email_verifications (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash bytea       NOT NULL,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_email_verif_token ON email_verifications (token_hash);
CREATE INDEX ix_email_verif_user ON email_verifications (user_id) WHERE consumed_at IS NULL;

CREATE TABLE signup_attempts (
    id         bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ip         inet,
    email      text,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ix_signup_attempts_ip_time ON signup_attempts (ip, created_at DESC);
