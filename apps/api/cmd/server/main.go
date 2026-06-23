// Command buktio-api is the OSS buktio REST API server. It runs the shared server
// boot path (internal/app.RunServer) with zero-value Enforcers, which resolve to
// the fully-enabled OSS implementations (PermitAll / AlwaysAllow / NoOp). The paid
// build lives in cmd/buktio-api-ee and injects enforcing implementations.
package main

import (
	"log/slog"
	"os"

	"github.com/buktio/buktio/internal/app"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "0.0.0-dev"

func main() {
	if err := app.RunServer(version, app.Enforcers{}); err != nil {
		slog.Error("fatal", slog.Any("error", err))
		os.Exit(1)
	}
}
