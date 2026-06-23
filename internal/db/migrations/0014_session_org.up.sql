-- 0014: per-request tenancy. Sessions remember the active org (for future org
-- switching); PATs are org-scoped (api_tokens.org_id already exists — backfill it
-- to the default org so existing tokens resolve). The active tenant is otherwise
-- derived from the user's organization_members membership, falling back to the
-- default org (single-tenant OSS UX is unchanged).
ALTER TABLE sessions ADD COLUMN org_id uuid REFERENCES organizations(id);

UPDATE api_tokens
   SET org_id = (SELECT id FROM organizations WHERE slug = 'default' AND deleted_at IS NULL LIMIT 1)
 WHERE org_id IS NULL;
