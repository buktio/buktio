-- 0022: ABAC policy templates (Enterprise). A policy binds a built-in template
-- (ip_allowlist, business_hours, …) + JSON config to one or more roles; the
-- enforcing authorizer denies a request an applicable, enabled policy rejects even
-- when the RBAC matrix allows it. Owners are exempt (no lock-out). OSS/Pro never
-- read these tables.
CREATE TABLE policies (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name       text        NOT NULL,
    template   text        NOT NULL,
    config     jsonb       NOT NULL DEFAULT '{}'::jsonb,
    enabled    boolean     NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ix_policies_org ON policies (org_id);

CREATE TABLE role_policy_bindings (
    policy_id uuid            NOT NULL REFERENCES policies(id) ON DELETE CASCADE,
    role      org_member_role NOT NULL,
    PRIMARY KEY (policy_id, role)
);
