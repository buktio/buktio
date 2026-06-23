package db

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	pgxmig "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
)

// Migrate applies all pending migrations from the embedded FS. It runs on
// startup, before the API serves traffic. It is a no-op if already up to date.
func Migrate(url string) error {
	src, err := iofs.New(MigrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("db: migrate source: %w", err)
	}
	defer src.Close()

	sqldb, err := sql.Open("pgx", url)
	if err != nil {
		return fmt.Errorf("db: migrate open: %w", err)
	}
	defer sqldb.Close()

	drv, err := pgxmig.WithInstance(sqldb, &pgxmig.Config{})
	if err != nil {
		return fmt.Errorf("db: migrate driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "pgx5", drv)
	if err != nil {
		return fmt.Errorf("db: migrate init: %w", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("db: migrate up: %w", err)
	}
	return nil
}
