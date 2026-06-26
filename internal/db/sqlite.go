package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
	sqlite "modernc.org/sqlite" // pure-Go SQLite driver, registered as "sqlite"
)

//go:embed migrations-sqlite/*.sql
var sqliteMigrationsFS embed.FS

// Register gen_random_uuid() as a SQLite scalar so the baseline schema's
// `DEFAULT (gen_random_uuid())` columns generate the same UUID-v4 strings the
// Postgres pgcrypto extension does. Registered once at package load, before any
// connection is opened (modernc exposes the function to connections opened after
// registration). Non-deterministic: a fresh UUID per row.
func init() {
	sqlite.MustRegisterScalarFunction("gen_random_uuid", 0,
		func(_ *sqlite.FunctionContext, _ []driver.Value) (driver.Value, error) {
			return uuid.NewString(), nil
		})
}

// DriverFromDSN reports which backend a DSN selects: "sqlite" for sqlite:/file:
// schemes or a bare *.db/*.sqlite path; "postgres" otherwise (the default).
func DriverFromDSN(dsn string) string {
	s := strings.TrimSpace(dsn)
	switch {
	case strings.HasPrefix(s, "sqlite:"), strings.HasPrefix(s, "file:"),
		strings.HasSuffix(s, ".db"), strings.HasSuffix(s, ".sqlite"):
		return "sqlite"
	default:
		return "postgres"
	}
}

// SQLitePath extracts the filesystem path from a sqlite DSN. It handles the
// scheme forms sqlite:///abs (empty authority + absolute path), sqlite:/abs,
// sqlite:rel, the file: equivalents, and a bare path; any ?query suffix is
// dropped (OpenSQLite appends its own PRAGMAs). Examples:
//
//	sqlite:///var/lib/buktio/buktio.db -> /var/lib/buktio/buktio.db
//	sqlite:buktio.db                   -> buktio.db
//	/data/buktio.db                    -> /data/buktio.db
func SQLitePath(dsn string) string {
	s := strings.TrimSpace(dsn)
	s = strings.TrimPrefix(s, "sqlite://") // sqlite:///abs -> /abs ; sqlite://rel -> rel
	s = strings.TrimPrefix(s, "file://")
	s = strings.TrimPrefix(s, "sqlite:") // sqlite:/abs -> /abs ; sqlite:rel -> rel
	s = strings.TrimPrefix(s, "file:")
	if i := strings.IndexByte(s, '?'); i >= 0 {
		s = s[:i]
	}
	return s
}

// OpenSQLite opens a SQLite database file with server-sane PRAGMAs: WAL (concurrent
// readers), foreign keys on, and a busy timeout to ride out the single-writer lock.
// MaxOpenConns is 1 — SQLite is single-writer, so serialising avoids SQLITE_BUSY;
// fine for a single-node OSS install (the only place SQLite is supported).
func OpenSQLite(ctx context.Context, path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)", path)
	sdb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("db: open sqlite: %w", err)
	}
	sdb.SetMaxOpenConns(1)
	if err := sdb.PingContext(ctx); err != nil {
		_ = sdb.Close()
		return nil, fmt.Errorf("db: ping sqlite: %w", err)
	}
	return sdb, nil
}

// MigrateSQLite applies the embedded SQLite-dialect migrations in version order,
// tracking applied versions in schema_migrations_sqlite. It is idempotent and a
// no-op once the database is up to date. SQLite ships a single consolidated
// baseline (version 30 = Postgres migrations 0001-0030); future schema changes
// add 0031+.up.sql here in SQLite dialect (the dual-migration discipline).
func MigrateSQLite(ctx context.Context, sdb *sql.DB) error {
	if _, err := sdb.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS schema_migrations_sqlite (
			version    INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`); err != nil {
		return fmt.Errorf("db: sqlite migrate ledger: %w", err)
	}

	applied := map[int]bool{}
	rows, err := sdb.QueryContext(ctx, `SELECT version FROM schema_migrations_sqlite`)
	if err != nil {
		return fmt.Errorf("db: sqlite migrate read ledger: %w", err)
	}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			_ = rows.Close()
			return fmt.Errorf("db: sqlite migrate scan ledger: %w", err)
		}
		applied[v] = true
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("db: sqlite migrate ledger rows: %w", err)
	}

	entries, err := fs.ReadDir(sqliteMigrationsFS, "migrations-sqlite")
	if err != nil {
		return fmt.Errorf("db: sqlite migrate source: %w", err)
	}
	type mig struct {
		version int
		name    string
	}
	var migs []mig
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		idx := strings.IndexByte(name, '_')
		if idx <= 0 {
			return fmt.Errorf("db: sqlite migrate: bad filename %q", name)
		}
		v, err := strconv.Atoi(name[:idx])
		if err != nil {
			return fmt.Errorf("db: sqlite migrate: bad version in %q: %w", name, err)
		}
		migs = append(migs, mig{v, name})
	}
	sort.Slice(migs, func(i, j int) bool { return migs[i].version < migs[j].version })

	for _, m := range migs {
		if applied[m.version] {
			continue
		}
		body, err := sqliteMigrationsFS.ReadFile("migrations-sqlite/" + m.name)
		if err != nil {
			return fmt.Errorf("db: sqlite migrate read %q: %w", m.name, err)
		}
		// modernc runs all statements in a single Exec (including BEGIN..END
		// trigger bodies), so the whole file applies atomically per migration.
		if _, err := sdb.ExecContext(ctx, string(body)); err != nil {
			return fmt.Errorf("db: sqlite migrate apply %q: %w", m.name, err)
		}
		if _, err := sdb.ExecContext(ctx,
			`INSERT INTO schema_migrations_sqlite (version) VALUES (?)`, m.version); err != nil {
			return fmt.Errorf("db: sqlite migrate record %q: %w", m.name, err)
		}
	}
	return nil
}
