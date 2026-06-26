-- App-level event webhooks (OSS, free): fire an HTTP callback when objects are
-- created or deleted through buktio. Infrastructure automation, not team management.
-- `events` is a comma-separated list of event names (kept as TEXT rather than a
-- Postgres array so the schema ports cleanly to the optional SQLite backend).
CREATE TABLE webhook_subscriptions (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    bucket_id  uuid        NOT NULL REFERENCES buckets(id)        ON DELETE CASCADE,
    url        text        NOT NULL,
    secret     text        NOT NULL DEFAULT '',
    events     text        NOT NULL DEFAULT '',
    enabled    boolean     NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_webhook_subs_bucket ON webhook_subscriptions (bucket_id) WHERE enabled;
