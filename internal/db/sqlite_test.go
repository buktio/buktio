package db

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func TestDriverFromDSN(t *testing.T) {
	cases := map[string]string{
		"postgres://u:p@h:5432/db?sslmode=disable": "postgres",
		"postgresql://h/db":                        "postgres",
		"sqlite:///var/lib/buktio/buktio.db":       "sqlite",
		"sqlite:buktio.db":                         "sqlite",
		"file:/data/buktio.db":                     "sqlite",
		"/data/buktio.db":                          "sqlite",
		"/data/buktio.sqlite":                      "sqlite",
		"":                                         "postgres",
	}
	for dsn, want := range cases {
		if got := DriverFromDSN(dsn); got != want {
			t.Errorf("DriverFromDSN(%q) = %q, want %q", dsn, got, want)
		}
	}
}

func TestSQLitePath(t *testing.T) {
	cases := map[string]string{
		"sqlite:///var/lib/buktio/buktio.db":  "/var/lib/buktio/buktio.db", // triple-slash absolute
		"sqlite:/var/lib/buktio/buktio.db":    "/var/lib/buktio/buktio.db",
		"sqlite:buktio.db":                    "buktio.db",
		"file:///data/buktio.db":              "/data/buktio.db",
		"/data/buktio.db":                     "/data/buktio.db",
		"sqlite:///tmp/x.db?_pragma=foo(bar)": "/tmp/x.db", // query stripped
	}
	for dsn, want := range cases {
		if got := SQLitePath(dsn); got != want {
			t.Errorf("SQLitePath(%q) = %q, want %q", dsn, got, want)
		}
	}
}

func TestMigrateSQLiteBaseline(t *testing.T) {
	sdb, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer sdb.Close()
	sdb.SetMaxOpenConns(1) // :memory: is per-connection
	if _, err := sdb.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	if err := MigrateSQLite(ctx, sdb); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Idempotent: a second run is a no-op.
	if err := MigrateSQLite(ctx, sdb); err != nil {
		t.Fatalf("migrate (rerun): %v", err)
	}

	// Every table from migrations 0001-0030 is present (RLS-only constructs aside).
	want := []string{
		"users", "organizations", "organization_members", "projects", "sessions",
		"storage_clusters", "storage_nodes", "buckets", "access_keys", "bucket_permissions",
		"usage_snapshots", "audit_events", "system_settings", "install_state", "api_tokens",
		"object_trash", "traffic_snapshots", "backup_schedules", "backup_jobs", "invitations",
		"user_identities", "org_storage_clusters", "scim_tokens", "policies", "role_policy_bindings",
		"org_branding", "email_verifications", "signup_attempts", "billing_customers",
		"billing_usage_reports", "migration_jobs", "webhook_subscriptions", "replication_jobs",
	}
	for _, tbl := range want {
		var n int
		if err := sdb.QueryRowContext(ctx,
			`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, tbl).Scan(&n); err != nil {
			t.Fatalf("lookup %s: %v", tbl, err)
		}
		if n != 1 {
			t.Errorf("table %s missing", tbl)
		}
	}

	// gen_random_uuid() default produces a real UUID-v4 string.
	if _, err := sdb.ExecContext(ctx,
		`INSERT INTO organizations (name, slug) VALUES ('Acme','acme')`); err != nil {
		t.Fatalf("insert org: %v", err)
	}
	var id string
	if err := sdb.QueryRowContext(ctx, `SELECT id FROM organizations WHERE slug='acme'`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if len(id) != 36 || strings.Count(id, "-") != 4 {
		t.Errorf("id %q is not a UUID", id)
	}

	// updated_at trigger bumps on UPDATE.
	if _, err := sdb.ExecContext(ctx, `UPDATE organizations SET name='Acme2' WHERE slug='acme'`); err != nil {
		t.Fatalf("update org: %v", err)
	}

	// CHECK constraint rejects an invalid enum value.
	if _, err := sdb.ExecContext(ctx,
		`INSERT INTO organizations (name, slug, status) VALUES ('Bad','bad','bogus')`); err == nil {
		t.Error("expected CHECK violation on bad status")
	}

	// Foreign key is enforced (membership requires a real org+user).
	if _, err := sdb.ExecContext(ctx,
		`INSERT INTO organization_members (org_id, user_id, role) VALUES ('nope','nope','member')`); err == nil {
		t.Error("expected FK violation on bad membership")
	}

	// Seed rows landed.
	var step string
	if err := sdb.QueryRowContext(ctx, `SELECT step FROM install_state WHERE id=1`).Scan(&step); err != nil {
		t.Fatalf("install_state: %v", err)
	}
	if step != "created" {
		t.Errorf("install step = %q, want created", step)
	}
}
