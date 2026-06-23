-- 0018: Row-Level Security as defense-in-depth for hard multi-tenancy (Enterprise).
--
-- The PRIMARY tenant-isolation control is the application layer (every org-scoped
-- query already filters by org_id, and by-id loads verify ownership). RLS is a
-- second, database-enforced net that bites ONLY when:
--   (a) the app connects as the non-superuser `buktio_app` role, AND
--   (b) a request-scoped connection has set `app.current_org` (via Store.WithOrg).
--
-- The policies are PERMISSIVE-WHEN-UNSET: with no `app.current_org` set (the OSS
-- path, background loops, the bootstrap owner, or any superuser) they impose no
-- restriction — so default deployments are byte-for-byte unchanged. Enterprise
-- operators opt in by pointing DATABASE_URL at `buktio_app` and setting
-- BUKTIO_RLS=on, after which every request runs scoped to its org.

-- A non-login, non-superuser role for the application to connect as under RLS.
-- Operators grant it LOGIN + a password out-of-band, or GRANT buktio_app to their
-- own login role. Created idempotently.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'buktio_app') THEN
        CREATE ROLE buktio_app NOLOGIN;
    END IF;
END
$$;

GRANT USAGE ON SCHEMA public TO buktio_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO buktio_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO buktio_app;
-- Future tables/sequences created by later migrations (run as the owner) are
-- granted automatically.
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO buktio_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO buktio_app;

-- Apply ENABLE+FORCE RLS and a permissive-when-unset org_isolation policy to every
-- tenant table. `org_col` is the column to compare against app.current_org.
DO $$
DECLARE
    t record;
BEGIN
    FOR t IN
        SELECT * FROM (VALUES
            ('organizations',        'id'),
            ('organization_members', 'org_id'),
            ('projects',             'org_id'),
            ('buckets',              'org_id'),
            ('access_keys',          'org_id'),
            ('usage_snapshots',      'org_id'),
            ('audit_events',         'org_id'),
            ('api_tokens',           'org_id'),
            ('object_trash',         'org_id'),
            ('invitations',          'org_id'),
            ('sessions',             'org_id'),
            ('backup_schedules',     'org_id'),
            ('backup_jobs',          'org_id')
        ) AS v(tbl, org_col)
    LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t.tbl);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t.tbl);
        EXECUTE format('DROP POLICY IF EXISTS org_isolation ON %I', t.tbl);
        -- NULLIF maps an unset/empty GUC to NULL *before* the ::uuid cast, so the
        -- permissive-when-unset branch never has to evaluate ''::uuid (Postgres
        -- does not guarantee OR short-circuiting, so a raw ''::uuid would error).
        EXECUTE format($f$
            CREATE POLICY org_isolation ON %I
            USING (
                NULLIF(current_setting('app.current_org', true), '') IS NULL
                OR %I = NULLIF(current_setting('app.current_org', true), '')::uuid
            )
            WITH CHECK (
                NULLIF(current_setting('app.current_org', true), '') IS NULL
                OR %I = NULLIF(current_setting('app.current_org', true), '')::uuid
            )
        $f$, t.tbl, t.org_col, t.org_col);
    END LOOP;
END
$$;
