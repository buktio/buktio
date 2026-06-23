package garagemanager

import (
	"path/filepath"

	"github.com/buktio/buktio/internal/secret"
)

// ClusterSecrets are the three Garage secrets buktio generates and owns. They are
// stored encrypted in PostgreSQL (source of truth) and materialized as 0600 files
// for Garage to read via *_FILE env vars.
type ClusterSecrets struct {
	RPCSecret    string // hex(32)
	AdminToken   string // base64(32)
	MetricsToken string // base64(32)
}

// GenerateClusterSecrets produces a fresh set of Garage secrets using crypto/rand.
func GenerateClusterSecrets() (ClusterSecrets, error) {
	rpc, err := secret.NewRPCSecret()
	if err != nil {
		return ClusterSecrets{}, err
	}
	admin, err := secret.NewToken()
	if err != nil {
		return ClusterSecrets{}, err
	}
	metrics, err := secret.NewToken()
	if err != nil {
		return ClusterSecrets{}, err
	}
	return ClusterSecrets{RPCSecret: rpc, AdminToken: admin, MetricsToken: metrics}, nil
}

// Standard file names for the materialized secret files.
const (
	rpcSecretFile    = "rpc_secret"
	adminTokenFile   = "admin_token"
	metricsTokenFile = "metrics_token"
)

// WriteSecretFiles materializes the secrets as 0600 files under dir and returns
// the GARAGE_*_FILE env map Garage uses to read them. The rendered garage.toml
// never contains these values.
func WriteSecretFiles(dir string, s ClusterSecrets) (map[string]string, error) {
	files := []struct {
		name string
		val  string
	}{
		{rpcSecretFile, s.RPCSecret},
		{adminTokenFile, s.AdminToken},
		{metricsTokenFile, s.MetricsToken},
	}
	for _, f := range files {
		if err := secret.WriteFile(filepath.Join(dir, f.name), []byte(f.val)); err != nil {
			return nil, err
		}
	}
	return map[string]string{
		"GARAGE_RPC_SECRET_FILE":    filepath.Join(dir, rpcSecretFile),
		"GARAGE_ADMIN_TOKEN_FILE":   filepath.Join(dir, adminTokenFile),
		"GARAGE_METRICS_TOKEN_FILE": filepath.Join(dir, metricsTokenFile),
	}, nil
}
