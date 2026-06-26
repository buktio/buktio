package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	_ "modernc.org/sqlite"
)

func TestTranslateToSQLite(t *testing.T) {
	cases := []struct{ in, want string }{
		{`SELECT $1`, `SELECT ?1`},
		{`WHERE a=$1 AND b=$2`, `WHERE a=?1 AND b=?2`},
		{`WHERE a=$1 OR c=$1`, `WHERE a=?1 OR c=?1`}, // reuse preserved
		{`id::uuid`, `id`},
		{`$1::text`, `?1`},
		{`NULLIF($1,'')::uuid`, `NULLIF(?1,'')`},
		{`status::actor_type`, `status`},
		{`x = now()`, `x = CURRENT_TIMESTAMP`},
		{`x ILIKE $1`, `x LIKE ?1`},
	}
	for _, c := range cases {
		if got := translateToSQLite(c.in); got != c.want {
			t.Errorf("translateToSQLite(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSQLiteQuerierRoundTrip(t *testing.T) {
	sdb, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer sdb.Close()
	sdb.SetMaxOpenConns(1) // :memory: is per-connection — pin to one so all queries share it
	if _, err := sdb.Exec(`CREATE TABLE t (id TEXT PRIMARY KEY, n INTEGER, created TEXT)`); err != nil {
		t.Fatal(err)
	}
	s := NewStoreSQLite(sdb)
	ctx := context.Background()

	if s.Driver() != "sqlite" {
		t.Fatalf("driver = %q, want sqlite", s.Driver())
	}

	// Exec: $N placeholders + ::cast + now() all translate.
	if _, err := s.q(ctx).Exec(ctx, `INSERT INTO t (id, n, created) VALUES ($1::text, $2, now())`, "a", 42); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// QueryRow + ::cast.
	var n int
	if err := s.q(ctx).QueryRow(ctx, `SELECT n FROM t WHERE id=$1::text`, "a").Scan(&n); err != nil {
		t.Fatalf("queryrow: %v", err)
	}
	if n != 42 {
		t.Fatalf("n = %d, want 42", n)
	}

	// No-rows is normalised to pgx.ErrNoRows (so the 26 errors.Is checks work).
	if err := s.q(ctx).QueryRow(ctx, `SELECT n FROM t WHERE id=$1`, "missing").Scan(&n); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("no-row scan err = %v, want pgx.ErrNoRows", err)
	}

	// Query (multi-row): Next / Scan / Err / Close via the DBRows adapter.
	rows, err := s.q(ctx).Query(ctx, `SELECT id, n FROM t WHERE n > $1`, 0)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for rows.Next() {
		var id string
		var v int
		if err := rows.Scan(&id, &v); err != nil {
			t.Fatal(err)
		}
		count++
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("rows = %d, want 1", count)
	}

	// Transaction through the abstraction (Begin/Exec/Commit).
	tx, err := s.begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO t (id, n, created) VALUES ($1, $2, now())`, "b", 7); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("tx insert: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	// DBResult.RowsAffected.
	res, err := s.q(ctx).Exec(ctx, `DELETE FROM t WHERE id=$1`, "b")
	if err != nil {
		t.Fatal(err)
	}
	if res.RowsAffected() != 1 {
		t.Fatalf("RowsAffected = %d, want 1", res.RowsAffected())
	}
}
