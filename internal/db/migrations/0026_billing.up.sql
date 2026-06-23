-- 0026: usage-based billing (Hosted). billing_customers maps an org to its Stripe
-- customer + metered subscription items. billing_usage_reports is an idempotency
-- ledger: one row per (org, period, metric) so a re-run of the reporter never
-- double-bills. OSS never writes these.
CREATE TABLE billing_customers (
    org_id                 uuid        PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
    stripe_customer_id     text        NOT NULL,
    stripe_subscription_id text,
    -- Metered subscription item ids the reporter posts usage to.
    storage_item_id        text,
    egress_item_id         text,
    requests_item_id       text,
    status                 text        NOT NULL DEFAULT 'active', -- active|past_due|canceled
    created_at             timestamptz NOT NULL DEFAULT now(),
    updated_at             timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_billing_stripe_customer ON billing_customers (stripe_customer_id);

CREATE TABLE billing_usage_reports (
    id              bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    org_id          uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    metric          text        NOT NULL,           -- storage_gb_month|egress_gb|requests
    quantity        bigint      NOT NULL,
    period_start    timestamptz NOT NULL,
    period_end      timestamptz NOT NULL,
    idempotency_key text        NOT NULL,
    reported_at     timestamptz NOT NULL DEFAULT now()
);
-- The idempotency key (org+metric+period) makes re-reporting a no-op.
CREATE UNIQUE INDEX uq_billing_report_idem ON billing_usage_reports (idempotency_key);
