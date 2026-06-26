# ADR 0001 — Optional SQLite backend

**Status:** Implemented · **Date:** 2026-06-26 · **Scope:** OSS core (v2.3b)

## Context

PostgreSQL is buktio's only database. For a single-node / homelab install that is a heavy
dependency (a separate server, password, volume, backup). The roadmap's "zero-dependency" goal
calls for an **optional** SQLite backend: one database *file*, no server. PostgreSQL stays the
default and is **required** for the Pro/Enterprise editions, which depend on Postgres-only
features (RLS, `pg_dump`, plpgsql triggers, GUC-scoped tenancy). SQLite is therefore an
OSS-single-node option, never a replacement.

### Constraints discovered in the code

- Data layer is hand-written SQL over pgx (no ORM): **146 statements across 25 files**.
- The `querier` used by `Store` returns pgx types (`pgx.Rows`, `pgx.Row`, `pgconn.CommandTag`).
- Repository code uses only `Next/Scan/Close/Err` on rows, `Scan` on a row, and `RowsAffected`
  on the tag — **no** `Values/RawValues/FieldDescriptions/Conn`.
- pgx coupling inventory: `pgx.ErrNoRows` ×26 (no-row detection), explicit `pgx.Row`/`pgx.Rows`
  annotations ×13, `pgx.Batch` ×1, `pgconn.CommandTag` ×1.
- Postgres-only SQL needing portable rewrites is small and localized: `DISTINCT ON`
  (`orgs.go`, `usage.go`), `to_timestamp/extract(epoch …)` (`usage.go`), a `jsonb` operator.
- Postgres-specific DDL: 10 ENUM types, `citext`, `gen_random_uuid()`, plpgsql `set_updated_at`
  triggers, `timestamptz`, RLS (`0018`, Enterprise only).

## Decision

**Abstract the statement runner behind buktio's own minimal interfaces, with two drivers.**

Because repository code touches only a tiny slice of pgx, define thin interfaces
(`Querier`, `Rows`, `Row`, `Result`) that pgx satisfies structurally, plus a SQLite
implementation over `database/sql` + `modernc.org/sqlite` (pure Go, **no CGO** — keeps the
static cross-arch single binary). The 25 repository files keep their hand-written SQL; the
SQLite driver wrapper translates dialect at runtime, and a handful of PG-only queries are
rewritten portably.

Driver selected from the DSN scheme: `postgres://…` → pgx (default); `sqlite:///path` or
`BUKTIO_DB=sqlite` → SQLite.

### SQLite driver wrapper responsibilities

1. **Placeholders**: `$1,$2,…` → `?` (positional, tokenizer-aware to skip string literals).
2. **Casts**: strip `::type` (e.g. `id::text`, `$1::uuid`) — SQLite has no `::` cast syntax.
3. **Functions**: `now()` → `CURRENT_TIMESTAMP`; `gen_random_uuid()` only appears in DDL defaults.
4. **No-row normalization**: map `sql.ErrNoRows` → `pgx.ErrNoRows` so the 26 `errors.Is` sites are unchanged.
5. **Result/Rows adapters**: `sql.Result.RowsAffected()` drops its error; `sql.Rows.Close()` drops its error.

### Dual migrations

A parallel `migrations-sqlite/` set translates the **OSS** migrations `0001–0013` to the SQLite
dialect (ENUM→`TEXT` + `CHECK`; `citext`→`COLLATE NOCASE`; `gen_random_uuid()`→app-generated UUID
or `lower(hex(randomblob(16)))`; plpgsql trigger→SQLite `AFTER UPDATE` trigger; `timestamptz`→
`TEXT` UTC; `jsonb`→`TEXT`). Paid migrations `0014+` are **never applied** on SQLite; entitlements
fail closed for PG-only paid features with a clear "requires PostgreSQL" message.

### Upgrade path (preserves the open-core promise)

A SQLite OSS install that upgrades to Pro/Enterprise runs a one-shot `buktio db convert --to
postgres` (CLI): create the PG schema (migrations), stream every table SQLite→PG, then switch the
DSN. Documented as the required step before applying a paid license.

## Alternatives considered

- **A — `database/sql` everywhere (rewrite all 146 sites).** Cleanest long-term but maximal
  blast radius; rejected for churn/risk.
- **B — full pgx-compatible shim over `database/sql`.** `pgx.Rows` is a large interface
  (`Values`, `FieldDescriptions`, …) we'd only partially implement → fragile. Rejected.
- **Chosen (C) — minimal buktio interfaces pgx already satisfies + SQLite adapter.** Lowest churn
  to the repository, no fragile shim, Postgres path byte-for-byte unchanged.

## Phased implementation — all phases delivered

1. ✅ **Foundation:** `Querier/DBRows/DBRow/DBResult` interfaces + pgx adapter; `Store` routed
   through them; 9 scan helpers + the batch made driver-agnostic. Postgres behaviour unchanged.
   (`internal/repository/dbx.go`)
2. ✅ **SQLite driver:** `modernc.org/sqlite` (CGO-free) behind `database/sql`; `translateToSQLite`
   (`$N`→`?N`, strip `::casts`, `now()`→`CURRENT_TIMESTAMP`, `ILIKE`→`LIKE`, `IS [NOT] DISTINCT
   FROM`→`IS [NOT]`), `sql.ErrNoRows`→`pgx.ErrNoRows`, `Tx` abstraction, `DriverFromDSN`.
   (`internal/repository/sqlite.go`, `internal/db/sqlite.go`)
3. ✅ **Migrations:** one consolidated SQLite-dialect baseline = Postgres `0001–0030` (a fresh
   backend needs no incremental history); enum→`CHECK`, plpgsql `set_updated_at`→per-table
   triggers, `gen_random_uuid()` registered as a Go scalar, timestamptz→`DATETIME` (so modernc
   round-trips `time.Time`); RLS (0018) omitted (no-op on single-node). A tiny version-tracked
   runner applies it. (`internal/db/migrations-sqlite/`, `MigrateSQLite`)
4. ✅ **Portable rewrites:** `DISTINCT ON`→`row_number()` window (both backends); `StorageSeries`
   epoch bucketing branched (PG `extract`/`to_timestamp` vs SQLite `strftime`); `scopes text[]`
   via `array_to_string`/CSV; advisory lock skipped on SQLite (single writer serializes); backup
   via `VACUUM INTO`. (`usage.go`, `orgs.go`, `audit.go`, `api_tokens.go`, `service/backups.go`)
5. ✅ **Boot + upgrade + e2e:** `wireServices` selects the backend from the DSN; the paid editions
   **fail closed** on a SQLite DSN; `buktio db convert --to postgres` copies every table in FK
   order. Verified end to end on SQLite (setup→login→bucket→object→usage→backup) **and** the
   convert path (SQLite→Postgres→login as the migrated admin). Postgres has zero regression.

## Consequences

- New OSS dependency `modernc.org/sqlite` (pure Go, CGO-free).
- A small dialect-translation surface to maintain; every **new** OSS migration/query must stay
  portable or carry a SQLite variant (review rule).
- Paid editions are unaffected and remain Postgres-only by design.
