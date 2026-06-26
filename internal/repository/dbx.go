package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This file is the database-driver seam for the optional SQLite backend (ADR 0001).
// Repository code runs every statement through the minimal Querier/DBRow/DBRows/
// DBResult interfaces below — the exact subset of pgx the repository actually uses.
// pgx's own result types satisfy these structurally, so the Postgres path is
// unchanged; a database/sql + modernc.org/sqlite adapter (a later phase) implements
// the same interfaces, translating dialect at the driver boundary.

// DBRow is a single-row result (QueryRow).
type DBRow interface {
	Scan(dest ...any) error
}

// DBRows is a multi-row result (Query). Close returns nothing (pgx semantics); the
// SQLite adapter discards database/sql's Close error.
type DBRows interface {
	Next() bool
	Scan(dest ...any) error
	Close()
	Err() error
}

// DBResult is the outcome of a non-query statement (Exec).
type DBResult interface {
	RowsAffected() int64
}

// Querier runs SQL statements. A Postgres pool/conn and a SQLite connection both
// provide one (via the adapters in this file / the SQLite driver).
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (DBRows, error)
	QueryRow(ctx context.Context, sql string, args ...any) DBRow
	Exec(ctx context.Context, sql string, args ...any) (DBResult, error)
}

// pgxNative is exactly what *pgxpool.Pool and *pgxpool.Conn expose. Both satisfy it,
// so repository code can run on the shared pool or an org-scoped connection.
type pgxNative interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// pgxQuerier adapts a pgx pool/conn to Querier. pgx.Rows/pgx.Row/pgconn.CommandTag
// already satisfy DBRows/DBRow/DBResult, so these methods just re-wrap the tuples.
type pgxQuerier struct{ q pgxNative }

func (p pgxQuerier) Query(ctx context.Context, sql string, args ...any) (DBRows, error) {
	rows, err := p.q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (p pgxQuerier) QueryRow(ctx context.Context, sql string, args ...any) DBRow {
	return p.q.QueryRow(ctx, sql, args...)
}

func (p pgxQuerier) Exec(ctx context.Context, sql string, args ...any) (DBResult, error) {
	tag, err := p.q.Exec(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return tag, nil
}

// Tx is a database transaction: a Querier plus commit/rollback.
type Tx interface {
	Querier
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// pgxTx adapts a pgx.Tx (which satisfies pgxNative for the querier methods).
type pgxTx struct {
	pgxQuerier
	tx pgx.Tx
}

func (t pgxTx) Commit(ctx context.Context) error   { return t.tx.Commit(ctx) }
func (t pgxTx) Rollback(ctx context.Context) error { return t.tx.Rollback(ctx) }

// dbHandle abstracts the database connection behind the repository: the default
// querier, transactions, optional Postgres RLS org-scoping (a no-op on SQLite), a
// health ping, and lifecycle. Store holds one of these (pgxHandle or sqliteHandle).
type dbHandle interface {
	querier() Querier
	begin(ctx context.Context) (Tx, error)
	withOrgConn(ctx context.Context, orgID string) (context.Context, func(), error)
	ping(ctx context.Context) error
	close()
	driver() string
}

// pgxHandle is the PostgreSQL dbHandle.
type pgxHandle struct{ pool *pgxpool.Pool }

func (h pgxHandle) querier() Querier               { return pgxQuerier{h.pool} }
func (h pgxHandle) ping(ctx context.Context) error { return h.pool.Ping(ctx) }
func (h pgxHandle) close()                         { h.pool.Close() }
func (h pgxHandle) driver() string                 { return "postgres" }

func (h pgxHandle) begin(ctx context.Context) (Tx, error) {
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return pgxTx{pgxQuerier{tx}, tx}, nil
}

// withOrgConn checks out a dedicated connection and pins the RLS org on it via the
// `app.current_org` GUC (migration 0018). It is the Postgres-only integration point
// for Row-Level Security; SQLite relies on the app-layer org_id filters alone.
func (h pgxHandle) withOrgConn(ctx context.Context, orgID string) (context.Context, func(), error) {
	conn, err := h.pool.Acquire(ctx)
	if err != nil {
		return ctx, func() {}, fmt.Errorf("repository: acquire org conn: %w", err)
	}
	// Session-level (is_local=false) so it persists across this connection's
	// statements; cleared on release. set_config is parameterized (no injection).
	if _, err := conn.Exec(ctx, "SELECT set_config('app.current_org', $1, false)", orgID); err != nil {
		conn.Release()
		return ctx, func() {}, fmt.Errorf("repository: set org scope: %w", err)
	}
	release := func() {
		// Best-effort clear on a fresh context so a cancelled request still resets
		// the GUC before the connection is reused by another tenant.
		_, _ = conn.Exec(context.Background(), "SELECT set_config('app.current_org', '', false)")
		conn.Release()
	}
	return context.WithValue(ctx, connCtxKey{}, pgxNative(conn)), release, nil
}
