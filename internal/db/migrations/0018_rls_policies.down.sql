-- Reverse 0018: drop policies, disable RLS, revoke grants, drop the role.
DO $$
DECLARE
    t text;
BEGIN
    FOREACH t IN ARRAY ARRAY[
        'organizations','organization_members','projects','buckets','access_keys',
        'usage_snapshots','audit_events','api_tokens','object_trash','invitations',
        'sessions','backup_schedules','backup_jobs'
    ]
    LOOP
        EXECUTE format('DROP POLICY IF EXISTS org_isolation ON %I', t);
        EXECUTE format('ALTER TABLE %I NO FORCE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE %I DISABLE ROW LEVEL SECURITY', t);
    END LOOP;
END
$$;

ALTER DEFAULT PRIVILEGES IN SCHEMA public
    REVOKE SELECT, INSERT, UPDATE, DELETE ON TABLES FROM buktio_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    REVOKE USAGE, SELECT ON SEQUENCES FROM buktio_app;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'buktio_app') THEN
        REVOKE ALL ON ALL TABLES IN SCHEMA public FROM buktio_app;
        REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM buktio_app;
        REVOKE USAGE ON SCHEMA public FROM buktio_app;
        DROP ROLE buktio_app;
    END IF;
END
$$;
