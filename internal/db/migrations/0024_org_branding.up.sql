-- 0024: white-label branding + custom domains (Enterprise). Each org may set a
-- display name, logo, primary color (a shadcn theme token), a from-address, and a
-- verified custom domain. The panel resolves branding by the request Host so a
-- custom domain renders the org's theme pre-login. OSS never reads this table.
CREATE TABLE org_branding (
    org_id        uuid        PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
    display_name  text,
    logo_url      text,
    primary_color text,                 -- e.g. "oklch(0.6 0.18 250)" or "#4f46e5"
    email_from    text,
    custom_domain text,                 -- e.g. "storage.acme.com"; NULL = none
    updated_at    timestamptz NOT NULL DEFAULT now()
);
-- A custom domain maps to at most one org (the Caddy on_demand_tls allow-list key).
CREATE UNIQUE INDEX uq_branding_domain ON org_branding (lower(custom_domain)) WHERE custom_domain IS NOT NULL;
