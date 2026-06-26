package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// convertTableOrder lists every table parent-before-child so foreign keys resolve
// during the copy. It mirrors the SQLite baseline / Postgres migrations 0001-0030.
var convertTableOrder = []string{
	"users", "organizations", "organization_members", "projects", "sessions",
	"storage_clusters", "storage_nodes", "buckets", "access_keys", "bucket_permissions",
	"usage_snapshots", "audit_events", "system_settings", "install_state", "api_tokens",
	"object_trash", "traffic_snapshots", "backup_schedules", "backup_jobs", "invitations",
	"user_identities", "org_storage_clusters", "scim_tokens", "policies", "role_policy_bindings",
	"org_branding", "email_verifications", "signup_attempts", "billing_customers",
	"billing_usage_reports", "migration_jobs", "webhook_subscriptions", "replication_jobs",
}

type pgCol struct{ name, dataType, udtName string }

// ConvertSQLiteToPostgres migrates a SQLite buktio database into a PostgreSQL one:
// it runs the Postgres migrations on the destination, clears the rows those
// migrations seed, then copies every table in foreign-key order. This is the
// one-time free->paid upgrade path (the paid editions require PostgreSQL). The
// destination must be reachable and hold no buktio data yet (a fresh database).
func ConvertSQLiteToPostgres(ctx context.Context, sqlitePath, pgURL string, log *slog.Logger) error {
	sdb, err := OpenSQLite(ctx, sqlitePath)
	if err != nil {
		return err
	}
	defer func() { _ = sdb.Close() }()

	if err := Migrate(pgURL); err != nil {
		return fmt.Errorf("convert: migrate destination: %w", err)
	}
	pool, err := OpenPool(ctx, pgURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Clear the rows the migrations seed (install_state + default system_settings)
	// so the copy from SQLite — which carries the same rows — does not collide on PK.
	// Reverse order respects foreign keys.
	for i := len(convertTableOrder) - 1; i >= 0; i-- {
		if _, err := pool.Exec(ctx, "DELETE FROM "+convertTableOrder[i]); err != nil {
			return fmt.Errorf("convert: clear %s: %w", convertTableOrder[i], err)
		}
	}

	total := 0
	for _, table := range convertTableOrder {
		cols, err := pgColumns(ctx, pool, table)
		if err != nil {
			return fmt.Errorf("convert: columns of %s: %w", table, err)
		}
		n, err := copyTable(ctx, sdb, pool, table, cols)
		if err != nil {
			return fmt.Errorf("convert: copy %s: %w", table, err)
		}
		total += n
		if log != nil {
			log.Info("converted table", slog.String("table", table), slog.Int("rows", n))
		}
	}
	if log != nil {
		log.Info("conversion complete", slog.Int("tables", len(convertTableOrder)), slog.Int("rows", total))
	}
	return nil
}

// pgColumns returns the destination columns in ordinal order, skipping identity
// columns (Postgres GENERATED ALWAYS — the destination regenerates them; none are
// foreign-key referenced).
func pgColumns(ctx context.Context, pool *pgxpool.Pool, table string) ([]pgCol, error) {
	rows, err := pool.Query(ctx, `
		SELECT column_name, data_type, udt_name
		FROM information_schema.columns
		WHERE table_schema='public' AND table_name=$1 AND is_identity='NO'
		ORDER BY ordinal_position`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []pgCol
	for rows.Next() {
		var c pgCol
		if err := rows.Scan(&c.name, &c.dataType, &c.udtName); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// copyTable streams every row of one SQLite table into the Postgres table, casting
// each placeholder to the destination column's type so text-stored uuid/enum/jsonb/
// inet/array values land correctly.
func copyTable(ctx context.Context, sdb *sql.DB, pool *pgxpool.Pool, table string, cols []pgCol) (int, error) {
	if len(cols) == 0 {
		return 0, nil
	}
	names := make([]string, len(cols))
	placeholders := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.name
		ph := fmt.Sprintf("$%d", i+1)
		switch {
		case c.dataType == "ARRAY":
			ph += "::" + strings.TrimPrefix(c.udtName, "_") + "[]"
		case c.dataType == "USER-DEFINED": // enum
			ph += "::" + c.udtName
		case c.dataType == "uuid", c.dataType == "jsonb", c.dataType == "inet":
			ph += "::" + c.dataType
		}
		placeholders[i] = ph
	}
	selSQL := "SELECT " + strings.Join(names, ", ") + " FROM " + table
	insSQL := "INSERT INTO " + table + " (" + strings.Join(names, ", ") + ") VALUES (" + strings.Join(placeholders, ", ") + ")"

	srows, err := sdb.QueryContext(ctx, selSQL)
	if err != nil {
		return 0, err
	}
	defer srows.Close()
	n := 0
	for srows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := srows.Scan(ptrs...); err != nil {
			return n, err
		}
		for i, c := range cols {
			vals[i] = convertValue(c, vals[i])
		}
		if _, err := pool.Exec(ctx, insSQL, vals...); err != nil {
			return n, err
		}
		n++
	}
	return n, srows.Err()
}

// convertValue adapts a SQLite-scanned value to its Postgres column type: SQLite's
// 0/1 integers become booleans, and a CSV TEXT array becomes a Postgres array
// literal. Everything else (uuid/jsonb/inet text, bytea []byte, DATETIME time.Time)
// is passed through and handled by the placeholder cast / pgx.
func convertValue(c pgCol, v any) any {
	if v == nil {
		return nil
	}
	switch c.dataType {
	case "boolean":
		switch x := v.(type) {
		case int64:
			return x != 0
		case bool:
			return x
		}
	case "ARRAY":
		if s, ok := v.(string); ok {
			if s == "" {
				return "{}"
			}
			return "{" + s + "}"
		}
	}
	return v
}
