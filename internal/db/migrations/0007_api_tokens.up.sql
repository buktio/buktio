-- 0007: API tokens (PATs) for CLI/CI/scripts. The raw token is shown once at
-- creation; only a hash + last-four are stored (same policy as access keys).

CREATE TABLE api_tokens (
    id               uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          uuid        NOT NULL REFERENCES users(id)         ON DELETE CASCADE,
    org_id           uuid        REFERENCES organizations(id)          ON DELETE CASCADE,
    name             text        NOT NULL,
    token_hash       bytea       NOT NULL,                 -- SHA-256(raw token)
    secret_last_four text,
    scopes           text[]      NOT NULL DEFAULT '{}',    -- real but unrestricted in OSS
    expires_at       timestamptz,
    last_used_at     timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    deleted_at       timestamptz
);
CREATE UNIQUE INDEX uq_api_tokens_hash ON api_tokens (token_hash);
CREATE INDEX ix_api_tokens_user ON api_tokens (user_id) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_api_tokens_updated_at BEFORE UPDATE ON api_tokens
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
