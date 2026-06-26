package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/buktio/buktio/internal/db"
	"github.com/spf13/cobra"
)

// dbCmd groups database maintenance subcommands.
func dbCmd() *cobra.Command {
	c := &cobra.Command{Use: "db", Short: "Database maintenance"}
	c.AddCommand(dbConvertCmd())
	return c
}

// dbConvertCmd implements `buktio db convert --to postgres`: the one-time upgrade
// path from the OSS SQLite backend to PostgreSQL (required by the paid editions).
func dbConvertCmd() *cobra.Command {
	var to, sqlitePath, pgURL string
	cmd := &cobra.Command{
		Use:   "convert",
		Short: "Convert a SQLite database to PostgreSQL (free → paid upgrade)",
		Long: `Convert copies a SQLite buktio database into a fresh PostgreSQL database.

The paid editions (Pro/Enterprise) require PostgreSQL, so this is the supported
upgrade path from the OSS SQLite backend. The destination must be an empty,
migrated-on-demand PostgreSQL database; this command runs the migrations on it,
then copies every table.

  buktio db convert --to postgres \
    --sqlite /var/lib/buktio/buktio.db \
    --postgres-url postgres://user:pass@host:5432/buktio?sslmode=disable

--sqlite defaults to $DATABASE_URL when it is a sqlite DSN; --postgres-url
defaults to $BUKTIO_TARGET_DATABASE_URL.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if to != "postgres" {
				return fmt.Errorf("only --to postgres is supported (got %q)", to)
			}
			if sqlitePath == "" {
				if src := os.Getenv("DATABASE_URL"); db.DriverFromDSN(src) == "sqlite" {
					sqlitePath = db.SQLitePath(src)
				}
			} else {
				sqlitePath = db.SQLitePath(sqlitePath)
			}
			if sqlitePath == "" {
				return fmt.Errorf("no source: pass --sqlite or set DATABASE_URL to a sqlite DSN")
			}
			if pgURL == "" {
				return fmt.Errorf("no destination: pass --postgres-url or set BUKTIO_TARGET_DATABASE_URL")
			}
			logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
			fmt.Printf("Converting %s → PostgreSQL …\n", sqlitePath)
			if err := db.ConvertSQLiteToPostgres(context.Background(), sqlitePath, pgURL, logger); err != nil {
				return err
			}
			fmt.Println("Done. Point DATABASE_URL at the PostgreSQL database and start buktio-api-ee.")
			return nil
		},
	}
	cmd.Flags().StringVar(&to, "to", "postgres", "target backend (only postgres)")
	cmd.Flags().StringVar(&sqlitePath, "sqlite", "", "source SQLite file or DSN (default: $DATABASE_URL if sqlite)")
	cmd.Flags().StringVar(&pgURL, "postgres-url", os.Getenv("BUKTIO_TARGET_DATABASE_URL"), "destination PostgreSQL DSN")
	return cmd
}
