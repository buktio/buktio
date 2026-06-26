package repository

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
)

// SQLite driver layer (ADR 0001 phase 2). The repository writes Postgres SQL; this
// file translates it to the SQLite dialect at the driver boundary and adapts
// database/sql result types to the DBRows/DBRow/DBResult interfaces, so the 146
// repository statements run unchanged on SQLite. PostgreSQL stays the default and
// is required for the paid editions.

var (
	sqlitePlaceholderRe = regexp.MustCompile(`\$(\d+)`)                      // $1 -> ?1
	sqliteCastRe        = regexp.MustCompile(`::[a-zA-Z_][\w]*`)             // id::uuid -> id
	sqliteNowRe         = regexp.MustCompile(`(?i)\bnow\(\)`)                // now() -> CURRENT_TIMESTAMP
	sqliteILikeRe       = regexp.MustCompile(`(?i)\sILIKE\s`)                // ILIKE -> LIKE
	sqliteNotDistinctRe = regexp.MustCompile(`(?i)\bIS NOT DISTINCT FROM\b`) // null-safe = -> IS
	sqliteDistinctRe    = regexp.MustCompile(`(?i)\bIS DISTINCT FROM\b`)     // null-safe <> -> IS NOT
)

// translateToSQLite rewrites a Postgres statement into SQLite's dialect:
//   - $N numbered placeholders -> ?N (SQLite numbered params, preserving reuse)
//   - drops ::type casts (SQLite has no cast-colon syntax)
//   - now() -> CURRENT_TIMESTAMP
//   - ILIKE -> LIKE (SQLite LIKE is ASCII-case-insensitive)
//
// The few genuinely Postgres-only queries (DISTINCT ON, to_timestamp/extract(epoch),
// jsonb operators) are rewritten portably at the call site in a later phase.
func translateToSQLite(q string) string {
	q = sqlitePlaceholderRe.ReplaceAllString(q, "?$1")
	q = sqliteCastRe.ReplaceAllString(q, "")
	q = sqliteNowRe.ReplaceAllString(q, "CURRENT_TIMESTAMP")
	q = sqliteILikeRe.ReplaceAllString(q, " LIKE ")
	q = sqliteNotDistinctRe.ReplaceAllString(q, " IS ") // before the DISTINCT-FROM rule
	q = sqliteDistinctRe.ReplaceAllString(q, " IS NOT ")
	return q
}

// mapSQLiteErr normalises database/sql's no-rows sentinel to pgx.ErrNoRows, so the
// repository's existing errors.Is(err, pgx.ErrNoRows) checks work on both drivers.
func mapSQLiteErr(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return pgx.ErrNoRows
	}
	return err
}

// sqlRunner is the subset of *sql.DB / *sql.Tx the querier needs.
type sqlRunner interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// sqliteQuerier implements Querier over a database/sql handle with translation.
type sqliteQuerier struct{ db sqlRunner }

func (q sqliteQuerier) Query(ctx context.Context, query string, args ...any) (DBRows, error) {
	rows, err := q.db.QueryContext(ctx, translateToSQLite(query), args...)
	if err != nil {
		return nil, mapSQLiteErr(err)
	}
	return sqliteRows{rows}, nil
}

func (q sqliteQuerier) QueryRow(ctx context.Context, query string, args ...any) DBRow {
	return sqliteRow{q.db.QueryRowContext(ctx, translateToSQLite(query), args...)}
}

func (q sqliteQuerier) Exec(ctx context.Context, query string, args ...any) (DBResult, error) {
	res, err := q.db.ExecContext(ctx, translateToSQLite(query), args...)
	if err != nil {
		return nil, mapSQLiteErr(err)
	}
	return sqliteResult{res}, nil
}

// --- adapters: database/sql result types -> DBRows/DBRow/DBResult ---

type sqliteRows struct{ *sql.Rows }

// Close drops sql.Rows.Close's error (DBRows.Close returns nothing, pgx semantics).
func (r sqliteRows) Close() { _ = r.Rows.Close() }

type sqliteRow struct{ *sql.Row }

func (r sqliteRow) Scan(dest ...any) error { return mapSQLiteErr(r.Row.Scan(dest...)) }

type sqliteResult struct{ sql.Result }

func (r sqliteResult) RowsAffected() int64 { n, _ := r.Result.RowsAffected(); return n }

// sqliteHandle is the SQLite dbHandle. RLS / org-connection scoping is Postgres-only
// (the app-layer org_id filters still apply), so withOrgConn is a no-op.
type sqliteHandle struct{ db *sql.DB }

func (h sqliteHandle) querier() Querier { return sqliteQuerier{h.db} }
func (h sqliteHandle) withOrgConn(ctx context.Context, _ string) (context.Context, func(), error) {
	return ctx, func() {}, nil
}
func (h sqliteHandle) ping(ctx context.Context) error { return h.db.PingContext(ctx) }
func (h sqliteHandle) close()                         { _ = h.db.Close() }
func (h sqliteHandle) driver() string                 { return "sqlite" }

func (h sqliteHandle) begin(ctx context.Context) (Tx, error) {
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return sqliteTx{sqliteQuerier{tx}, tx}, nil
}

// sqliteTx adapts a *sql.Tx (which satisfies sqlRunner for the querier methods).
// sql.Tx's Commit/Rollback take no context; the ctx argument is accepted for
// interface parity and ignored.
type sqliteTx struct {
	sqliteQuerier
	tx *sql.Tx
}

func (t sqliteTx) Commit(context.Context) error   { return t.tx.Commit() }
func (t sqliteTx) Rollback(context.Context) error { return t.tx.Rollback() }

// BackupSQLite writes a consistent online snapshot of the SQLite database to
// destPath via VACUUM INTO (the SQLite analogue of pg_dump). It returns an error
// if the active backend is not SQLite. destPath is server-generated (no user
// input); single quotes are escaped defensively since VACUUM INTO takes a literal.
func (s *Store) BackupSQLite(ctx context.Context, destPath string) error {
	h, ok := s.h.(sqliteHandle)
	if !ok {
		return errors.New("repository: BackupSQLite requires the sqlite backend")
	}
	lit := "'" + strings.ReplaceAll(destPath, "'", "''") + "'"
	_, err := h.db.ExecContext(ctx, "VACUUM INTO "+lit)
	return err
}
