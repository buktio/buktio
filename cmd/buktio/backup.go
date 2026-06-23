package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

// doBackup runs pg_dump inside the postgres container and writes a custom-format
// dump to a host file. It backs up metadata + config ONLY — never object data.
func doBackup(out string) (string, error) {
	env := dotenv()
	user, dbn := env["POSTGRES_USER"], env["POSTGRES_DB"]
	if user == "" || dbn == "" {
		return "", fmt.Errorf("POSTGRES_USER/POSTGRES_DB not found in %s/.env", composeDir())
	}
	if out == "" {
		out = fmt.Sprintf("buktio-metadata-%s.dump", time.Now().UTC().Format("20060102-150405"))
	}
	f, err := os.Create(out)
	if err != nil {
		return "", err
	}
	defer f.Close()

	c := compose("exec", "-T", "postgres", "pg_dump", "-Fc", "--no-owner", "--no-privileges", "-U", user, dbn)
	c.Stdout = f // capture the dump to the host file
	fmt.Printf("Backing up metadata to %s ...\n", out)
	if err := c.Run(); err != nil {
		return "", fmt.Errorf("pg_dump: %w", err)
	}
	return out, nil
}

func backupCmd() *cobra.Command {
	var out string
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Back up buktio metadata (PostgreSQL) to a file",
		Long:  "Backs up buktio's PostgreSQL metadata + config. Does NOT back up object data (operator responsibility) and never includes the master key.",
		RunE: func(_ *cobra.Command, _ []string) error {
			path, err := doBackup(out)
			if err != nil {
				return err
			}
			if fi, err := os.Stat(path); err == nil {
				fmt.Printf("Done (%d bytes). Metadata + config only — NOT object data.\n", fi.Size())
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&out, "out", "o", "", "output file (default buktio-metadata-<ts>.dump)")
	return cmd
}

func restoreCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "restore <file>",
		Short: "Restore buktio metadata from a backup (DESTRUCTIVE)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if !yes {
				return fmt.Errorf("restore OVERWRITES the current metadata database; re-run with --yes to confirm")
			}
			env := dotenv()
			user, dbn := env["POSTGRES_USER"], env["POSTGRES_DB"]
			if user == "" || dbn == "" {
				return fmt.Errorf("POSTGRES_USER/POSTGRES_DB not found in %s/.env", composeDir())
			}
			f, err := os.Open(args[0])
			if err != nil {
				return err
			}
			defer f.Close()

			c := compose("exec", "-T", "postgres", "pg_restore", "--clean", "--if-exists", "--no-owner", "-U", user, "-d", dbn)
			c.Stdin = f
			fmt.Printf("Restoring metadata from %s ...\n", args[0])
			if err := c.Run(); err != nil {
				return fmt.Errorf("pg_restore: %w", err)
			}
			fmt.Println("Restore complete. Check `buktio status` and the dashboard; if buckets/keys drift from the backend, reconcile via the API (GET /api/v1/system/reconcile) or web UI.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm the destructive restore")
	return cmd
}

func upgradeCmd() *cobra.Command {
	var to string
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade buktio (backup-first, then pull/build + restart, health-gated)",
		RunE: func(_ *cobra.Command, _ []string) error {
			fmt.Println("Step 1/4: backing up metadata...")
			if _, err := doBackup(""); err != nil {
				return fmt.Errorf("pre-upgrade backup failed (aborting): %w", err)
			}
			if to != "" {
				fmt.Printf("Step 2/4: pinning BUKTIO_VERSION=%s ...\n", to)
				if err := setDotenvKey("BUKTIO_VERSION", to); err != nil {
					return err
				}
			} else {
				fmt.Println("Step 2/4: no --to given; keeping current image tags")
			}
			fmt.Println("Step 3/4: pulling/building and restarting...")
			_ = run(compose("pull")) // best-effort: locally built images may not be in a registry
			if err := run(compose("up", "-d", "--build")); err != nil {
				return err
			}
			fmt.Println("Step 4/4: waiting for health...")
			for i := 0; i < 30; i++ {
				if status() == nil {
					fmt.Println("Upgrade complete; services healthy.")
					return nil
				}
				time.Sleep(2 * time.Second)
			}
			return fmt.Errorf("services did not become healthy after upgrade; check `buktio logs`")
		},
	}
	cmd.Flags().StringVar(&to, "to", "", "image tag to pin (BUKTIO_VERSION)")
	return cmd
}
