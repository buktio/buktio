// Package config loads and validates buktio's runtime configuration.
//
// Precedence is env > defaults (a file layer via Viper is added in a later
// milestone). Secrets are NEVER written into the rendered garage.toml; they are
// generated with crypto/rand and injected into Garage via *_FILE env vars.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the fully-resolved application configuration.
type Config struct {
	// Edition is the build/license edition. OSS by default; all features enabled.
	Edition string

	HTTP   HTTPConfig
	DB     DBConfig
	Garage GarageConfig

	// LogLevel: debug | info | warn | error.
	LogLevel string

	// AllowInsecure permits serving the panel over plaintext HTTP on a
	// non-loopback bind (dev only). Production must terminate TLS at the edge.
	AllowInsecure bool

	// RLSEnabled turns on per-request Postgres Row-Level Security scoping
	// (Enterprise defense-in-depth). When on, each authenticated request runs on
	// a connection pinned to its org via the `app.current_org` GUC, so the
	// migration 0018 policies enforce tenant isolation at the database. Requires
	// connecting as the non-superuser `buktio_app` role to actually bite; app-layer
	// scoping remains the primary control. Off by default (OSS/Pro unchanged).
	RLSEnabled bool

	// SelfServeSignup activates the public /signup flow (Hosted). Off by default so
	// on-prem installs never expose org creation.
	SelfServeSignup bool
	// SignupDevReturnToken (dev only) returns the verification token in the signup
	// response for testing. Never enable in production.
	SignupDevReturnToken bool
}

// HTTPConfig configures the API HTTP server.
type HTTPConfig struct {
	// Addr is the listen address, e.g. ":8080".
	Addr string
	// ReadTimeout / WriteTimeout bound slow clients.
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	// PanelOrigins is the allow-list of browser origins for CORS (never "*").
	PanelOrigins []string
	// TLS configures the optional built-in HTTPS listener (single-binary, edge-less
	// deployments). Default "off" — terminate TLS at a reverse proxy.
	TLS TLSConfig
}

// TLSConfig controls the built-in HTTPS listener. It exists so the single-binary
// "lite" deployment (no Caddy/edge proxy) can serve HTTPS directly:
//   - "off"  (default): plaintext HTTP; terminate TLS at an edge proxy.
//   - "self": an in-memory self-signed certificate (browser warning expected) —
//     handy for a quick private/homelab setup.
//   - "auto": Let's Encrypt via ACME for the configured public Domains.
type TLSConfig struct {
	Mode     string   // off | self | auto
	Domains  []string // required for "auto"; also added as SANs for "self"
	CacheDir string   // ACME certificate cache directory ("auto")
	Email    string   // optional ACME account email ("auto")
}

// DBConfig configures the PostgreSQL connection.
type DBConfig struct {
	// URL is the libpq/pgx connection string.
	URL string
}

// GarageConfig points at the hidden Garage engine. Secrets are loaded from
// files/secret-manager at runtime, not from these fields.
type GarageConfig struct {
	// Mode is the provisioning mode: "managed" (buktio bootstraps a single-node
	// Garage) or "external" (connect to an operator-owned Garage; no bootstrap).
	Mode string
	// AdminURL is the Admin API v2 base, e.g. http://garage:3903 (internal only).
	AdminURL string
	// S3URL is the S3 API base, e.g. http://garage:3900 (internal only).
	S3URL string
	// S3Region is Garage's configured region (default "garage").
	S3Region string
	// S3PublicEndpoint is the externally-reachable S3 host used when signing
	// presigned URLs the browser must reach (e.g. https://s3.host).
	S3PublicEndpoint string
	// WebPublicDomain is the base domain for public bucket websites; a public
	// bucket is served at https://<bucket>.<WebPublicDomain>/ (e.g. web.host).
	WebPublicDomain string
}

// Default values used when an env var is unset.
const (
	defaultHTTPAddr     = ":8080"
	defaultLogLevel     = "info"
	defaultS3Region     = "garage"
	defaultReadTimeout  = 15 * time.Second
	defaultWriteTimeout = 30 * time.Second
)

// Load reads configuration from the environment and validates it.
func Load() (*Config, error) {
	cfg := &Config{
		Edition:              env("BUKTIO_EDITION", "oss"),
		LogLevel:             env("BUKTIO_LOG_LEVEL", defaultLogLevel),
		AllowInsecure:        envBool("BUKTIO_ALLOW_INSECURE", false),
		RLSEnabled:           envBool("BUKTIO_RLS", false),
		SelfServeSignup:      envBool("BUKTIO_SELF_SERVE_SIGNUP", false),
		SignupDevReturnToken: envBool("BUKTIO_SIGNUP_DEV_TOKEN", false),
		HTTP: HTTPConfig{
			Addr:         env("BUKTIO_HTTP_ADDR", defaultHTTPAddr),
			ReadTimeout:  envDuration("BUKTIO_HTTP_READ_TIMEOUT", defaultReadTimeout),
			WriteTimeout: envDuration("BUKTIO_HTTP_WRITE_TIMEOUT", defaultWriteTimeout),
			PanelOrigins: envList("BUKTIO_PANEL_ORIGINS"),
			TLS: TLSConfig{
				Mode:     env("BUKTIO_TLS", "off"),
				Domains:  envList("BUKTIO_TLS_DOMAIN"),
				CacheDir: env("BUKTIO_TLS_CACHE_DIR", "/var/lib/buktio/tls"),
				Email:    env("BUKTIO_TLS_EMAIL", ""),
			},
		},
		DB: DBConfig{
			URL: os.Getenv("DATABASE_URL"),
		},
		Garage: GarageConfig{
			Mode:             env("BUKTIO_STORAGE_MODE", "managed"),
			AdminURL:         env("GARAGE_ADMIN_URL", "http://garage:3903"),
			S3URL:            env("GARAGE_S3_URL", "http://garage:3900"),
			S3Region:         env("GARAGE_S3_REGION", defaultS3Region),
			S3PublicEndpoint: os.Getenv("BUKTIO_S3_PUBLIC_ENDPOINT"),
			WebPublicDomain:  os.Getenv("BUKTIO_WEB_PUBLIC_DOMAIN"),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.HTTP.Addr == "" {
		return fmt.Errorf("config: BUKTIO_HTTP_ADDR must not be empty")
	}
	// DATABASE_URL is required once the persistence layer is wired (M1+). It is
	// allowed to be empty in the M0 skeleton so the server can boot for /healthz.
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("config: invalid BUKTIO_LOG_LEVEL %q", c.LogLevel)
	}
	switch c.HTTP.TLS.Mode {
	case "", "off", "self":
	case "auto":
		if len(c.HTTP.TLS.Domains) == 0 {
			return fmt.Errorf("config: BUKTIO_TLS=auto requires BUKTIO_TLS_DOMAIN")
		}
	default:
		return fmt.Errorf("config: invalid BUKTIO_TLS %q (want off|self|auto)", c.HTTP.TLS.Mode)
	}
	return nil
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		// Accept ParseBool values plus the operator-friendly on/off/yes/no that the
		// docs, .env.example, and Helm chart use (ParseBool rejects "on").
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "on", "yes", "enabled":
			return true
		case "off", "no", "disabled":
			return false
		}
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func envList(key string) []string {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
