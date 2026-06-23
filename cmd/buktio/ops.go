package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

// serviceAlias maps operator-facing names to compose services. "storage" hides the
// Garage engine name.
func serviceAlias(name string) string {
	switch name {
	case "storage", "engine":
		return "garage"
	default:
		return name
	}
}

func logsCmd() *cobra.Command {
	var follow bool
	cmd := &cobra.Command{
		Use:   "logs [service]",
		Short: "Show service logs (api, web, caddy, storage, postgres, s3proxy)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cargs := []string{"logs", "--tail", "200"}
			if follow {
				cargs = append(cargs, "-f")
			}
			if len(args) == 1 {
				cargs = append(cargs, serviceAlias(args[0]))
			}
			return run(compose(cargs...))
		},
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "follow log output")
	return cmd
}

func restartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart [service]",
		Short: "Restart the stack (or a single service)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cargs := []string{"restart"}
			if len(args) == 1 {
				cargs = append(cargs, serviceAlias(args[0]))
			}
			return run(compose(cargs...))
		},
	}
}

func doctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose the deployment (docker, compose, env, health)",
		RunE: func(_ *cobra.Command, _ []string) error {
			ok := true
			check := func(label string, pass bool, detail string) {
				mark := "✓"
				if !pass {
					mark = "✗"
					ok = false
				}
				fmt.Printf("  %s %-22s %s\n", mark, label, detail)
			}

			_, dockerErr := exec.LookPath("docker")
			check("docker installed", dockerErr == nil, "")

			dir := composeDir()
			_, composeErr := os.Stat(filepath.Join(dir, "docker-compose.yml"))
			check("compose file", composeErr == nil, dir)

			_, envErr := os.Stat(filepath.Join(dir, ".env"))
			check(".env present", envErr == nil, "run `make gen-env` if missing")

			statusErr := status()
			check("services healthy", statusErr == nil, "")

			if !ok {
				return fmt.Errorf("doctor found problems")
			}
			fmt.Println("\nAll checks passed.")
			return nil
		},
	}
}
