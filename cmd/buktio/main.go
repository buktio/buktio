// Command buktio is the product CLI — the only operational surface for an
// operator. It NEVER exposes a Garage/Rust/Cargo command; Garage is only ever
// referred to as the "storage engine". It orchestrates the docker compose stack on
// the host (logs/restart/upgrade), drives metadata backups/restore, and talks to
// the buktio API for status and cluster management.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var version = "0.0.0-dev"

func main() {
	root := &cobra.Command{
		Use:           "buktio",
		Short:         "Control plane for self-hosted object storage",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: false,
	}
	root.AddCommand(
		statusCmd(),
		logsCmd(),
		restartCmd(),
		backupCmd(),
		restoreCmd(),
		upgradeCmd(),
		doctorCmd(),
		clusterCmd(),
		dbCmd(),
	)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show panel, database, and storage-engine health",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return status()
		},
	}
}

// status queries the API readiness endpoint and prints a friendly summary.
// Component names are relabeled so the word "Garage" never appears.
func status() error {
	base := os.Getenv("BUKTIO_API_URL")
	if base == "" {
		base = "http://localhost:8080"
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(base + "/readyz")
	if err != nil {
		return fmt.Errorf("cannot reach buktio API at %s: %w", base, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var out struct {
		Status     string            `json:"status"`
		Version    string            `json:"version"`
		Components map[string]string `json:"components"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return fmt.Errorf("unexpected response: %s", string(body))
	}

	fmt.Printf("buktio %s — %s\n", out.Version, friendlyOverall(out.Status))
	for comp, st := range out.Components {
		fmt.Printf("  %-16s %s\n", friendlyComponent(comp)+":", st)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("not ready")
	}
	return nil
}

func friendlyOverall(s string) string {
	if s == "ok" {
		return "healthy"
	}
	return "not ready"
}

// friendlyComponent relabels internal component names; "garage_*" becomes
// "storage engine" so the engine stays hidden from the operator.
func friendlyComponent(c string) string {
	switch c {
	case "db":
		return "database"
	case "garage_admin", "garage_s3":
		return "storage engine"
	default:
		return c
	}
}
