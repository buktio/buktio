// Package db holds the PostgreSQL access layer. M0 ships the embedded migration
// set; the pgx pool, sqlc-generated queries, and an automatic on-boot migrate
// (golang-migrate over the embedded FS, run before serving traffic) land in M1.
package db

import "embed"

// MigrationsFS embeds the SQL migrations so they ship inside the binary and can
// be applied with golang-migrate's iofs source on startup.
//
//go:embed migrations/*.sql
var MigrationsFS embed.FS
