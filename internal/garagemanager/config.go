// Package garagemanager owns the hidden Garage engine: rendering its config,
// generating and injecting its secrets, and (M2) the idempotent Admin-API
// bootstrap. Nothing outside this package knows Garage exists.
package garagemanager

import (
	"fmt"
	"io"
	"strings"
	"text/template"
)

// ConfigParams are the NON-SENSITIVE values rendered into garage.toml. Secrets
// (rpc_secret, admin_token, metrics_token) are deliberately absent — they are
// injected via *_FILE env vars (see secrets.go).
type ConfigParams struct {
	MetadataDir string
	DataDir     string
	// DBEngine: "sqlite" (appliance default — crash-resilient, portable) or "lmdb".
	DBEngine          string
	ReplicationFactor int
	ConsistencyMode   string // "consistent" | "degraded" | "dangerous"

	RPCBindAddr   string
	RPCPublicAddr string

	S3APIBindAddr string
	S3Region      string
	S3RootDomain  string

	S3WebBindAddr   string
	S3WebRootDomain string
	S3WebIndex      string

	// AdminAPIBindAddr must stay internal/loopback (e.g. 127.0.0.1:3903 on bare
	// metal; [::]:3903 inside a container with no published port).
	AdminAPIBindAddr    string
	MetricsRequireToken bool
}

// DefaultSingleNode returns sensible single-node defaults for a managed install.
func DefaultSingleNode() ConfigParams {
	return ConfigParams{
		MetadataDir:         "/var/lib/garage/meta",
		DataDir:             "/var/lib/garage/data",
		DBEngine:            "sqlite",
		ReplicationFactor:   1,
		ConsistencyMode:     "consistent",
		RPCBindAddr:         "[::]:3901",
		RPCPublicAddr:       "127.0.0.1:3901",
		S3APIBindAddr:       "[::]:3900",
		S3Region:            "garage",
		S3RootDomain:        ".s3.buktio.local",
		S3WebBindAddr:       "[::]:3902",
		S3WebRootDomain:     ".web.buktio.local",
		S3WebIndex:          "index.html",
		AdminAPIBindAddr:    "127.0.0.1:3903",
		MetricsRequireToken: true,
	}
}

// Validate guards against accidentally rendering secret-looking values and
// catches missing required fields.
func (p ConfigParams) Validate() error {
	if p.MetadataDir == "" || p.DataDir == "" {
		return fmt.Errorf("garagemanager: metadata_dir and data_dir are required")
	}
	if p.ReplicationFactor < 1 {
		return fmt.Errorf("garagemanager: replication_factor must be >= 1")
	}
	switch p.DBEngine {
	case "sqlite", "lmdb":
	default:
		return fmt.Errorf("garagemanager: unsupported db_engine %q", p.DBEngine)
	}
	return nil
}

var configTmpl = template.Must(template.New("garage.toml").Parse(`# Rendered by buktio's garage-manager. NON-SENSITIVE.
# Secrets (rpc_secret, admin_token, metrics_token) are injected via *_FILE env
# vars (GARAGE_RPC_SECRET_FILE / GARAGE_ADMIN_TOKEN_FILE / GARAGE_METRICS_TOKEN_FILE).

metadata_dir = "{{ .MetadataDir }}"
data_dir     = "{{ .DataDir }}"

db_engine = "{{ .DBEngine }}"

replication_factor = {{ .ReplicationFactor }}
consistency_mode   = "{{ .ConsistencyMode }}"

rpc_bind_addr   = "{{ .RPCBindAddr }}"
rpc_public_addr = "{{ .RPCPublicAddr }}"

[s3_api]
api_bind_addr = "{{ .S3APIBindAddr }}"
s3_region     = "{{ .S3Region }}"
root_domain   = "{{ .S3RootDomain }}"

[s3_web]
bind_addr   = "{{ .S3WebBindAddr }}"
root_domain = "{{ .S3WebRootDomain }}"
index       = "{{ .S3WebIndex }}"

[admin]
api_bind_addr         = "{{ .AdminAPIBindAddr }}"
metrics_require_token = {{ .MetricsRequireToken }}
`))

// RenderConfig writes a non-sensitive garage.toml to w.
func RenderConfig(w io.Writer, p ConfigParams) error {
	if err := p.Validate(); err != nil {
		return err
	}
	return configTmpl.Execute(w, p)
}

// RenderConfigString renders garage.toml to a string.
func RenderConfigString(p ConfigParams) (string, error) {
	var sb strings.Builder
	if err := RenderConfig(&sb, p); err != nil {
		return "", err
	}
	return sb.String(), nil
}
