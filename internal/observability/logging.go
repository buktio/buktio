// Package observability wires structured logging, metrics, and (later) tracing.
//
// MVP principle: everything works with no mandatory external dependency.
// Logging is stdlib log/slog in JSON. A Prometheus /metrics endpoint and an
// authenticated Garage /metrics proxy are added in a later milestone.
package observability

import (
	"log/slog"
	"os"
	"strings"
)

// NewLogger returns a JSON slog.Logger at the given level (debug|info|warn|error).
func NewLogger(level string) *slog.Logger {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLevel(level),
	})
	return slog.New(h)
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
