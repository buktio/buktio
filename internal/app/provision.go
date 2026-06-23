// Package app wires buktio together: provisioning the storage backend on boot,
// constructing services, and assembling the HTTP server.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	clusterreg "github.com/buktio/buktio/internal/cluster"
	"github.com/buktio/buktio/internal/config"
	"github.com/buktio/buktio/internal/garagemanager"
	"github.com/buktio/buktio/internal/repository"
	"github.com/buktio/buktio/internal/secret"
	"github.com/buktio/buktio/internal/storage"
	"github.com/buktio/buktio/internal/storage/garage"
)

// Provisioned is the result of boot-time provisioning.
type Provisioned struct {
	Registry *clusterreg.Registry    // multi-cluster provider registry
	Provider storage.StorageProvider // primary (default) cluster provider
	Cluster  *repository.Cluster     // primary cluster row
	Tenant   *repository.DefaultTenant
}

// Provision ensures the storage backend is ready: it creates the default tenant,
// and on first run bootstraps the hidden Garage engine over the Admin API and
// persists the cluster (with encrypted secrets). On subsequent runs it loads the
// existing cluster. It returns a ready StorageProvider.
func Provision(ctx context.Context, cfg *config.Config, store *repository.Store, sealer secret.Sealer, logger *slog.Logger) (*Provisioned, error) {
	tenant, err := store.EnsureDefaultTenant(ctx)
	if err != nil {
		return nil, err
	}

	cluster, err := store.GetActiveCluster(ctx)
	if errors.Is(err, repository.ErrNotFound) {
		if cfg.Garage.Mode == "external" {
			cluster, err = connectExistingCluster(ctx, cfg, store, sealer, logger)
		} else {
			cluster, err = bootstrapAndPersist(ctx, cfg, store, sealer, logger)
		}
	}
	if err != nil {
		return nil, err
	}

	// The registry lazily builds + caches a provider per cluster (decrypting each
	// cluster's secrets). The primary (bootstrapped/connected) cluster is the default.
	reg := clusterreg.NewRegistry(store, sealer, cfg.Garage.S3PublicEndpoint)
	provider, err := reg.Provider(ctx, cluster.ID)
	if err != nil {
		return nil, fmt.Errorf("app: build primary provider: %w", err)
	}

	logger.Info("storage provisioned",
		slog.String("cluster_id", cluster.ID),
		slog.String("garage_version", cluster.GarageVersion),
		slog.String("system_key", cluster.SystemS3AccessKeyID))

	return &Provisioned{Registry: reg, Provider: provider, Cluster: cluster, Tenant: tenant}, nil
}

// bootstrapAndPersist runs the first-boot Garage bootstrap and stores the cluster.
func bootstrapAndPersist(ctx context.Context, cfg *config.Config, store *repository.Store, sealer secret.Sealer, logger *slog.Logger) (*repository.Cluster, error) {
	adminToken := os.Getenv("GARAGE_ADMIN_TOKEN")
	rpcSecret := os.Getenv("GARAGE_RPC_SECRET")
	if adminToken == "" {
		return nil, fmt.Errorf("app: GARAGE_ADMIN_TOKEN is required for first-run provisioning")
	}

	logger.Info("first run: bootstrapping storage engine")
	admin := garage.NewAdminClient(cfg.Garage.AdminURL, adminToken)
	res, err := garagemanager.Bootstrap(ctx, admin, garagemanager.BootstrapParams{})
	if err != nil {
		return nil, fmt.Errorf("app: bootstrap: %w", err)
	}

	adminEnc, err := sealer.Seal([]byte(adminToken))
	if err != nil {
		return nil, err
	}
	rpcEnc, err := sealer.Seal([]byte(rpcSecret))
	if err != nil {
		return nil, err
	}
	sysEnc, err := sealer.Seal([]byte(res.SystemSecretAccessKey))
	if err != nil {
		return nil, err
	}

	c := repository.Cluster{
		Name:                "default",
		Provider:            garage.Kind,
		Mode:                "managed",
		S3Endpoint:          cfg.Garage.S3URL,
		AdminEndpoint:       cfg.Garage.AdminURL,
		S3Region:            cfg.Garage.S3Region,
		GarageVersion:       res.GarageVersion,
		RPCSecretEnc:        rpcEnc,
		AdminTokenEnc:       adminEnc,
		SystemS3AccessKeyID: res.SystemAccessKeyID,
		SystemS3SecretEnc:   sysEnc,
		DBEngine:            "sqlite",
		ReplicationFactor:   1,
		Status:              "healthy",
	}
	if _, err := store.CreateCluster(ctx, c); err != nil {
		return nil, err
	}
	return store.GetActiveCluster(ctx)
}

// connectExistingCluster attaches to an operator-owned Garage WITHOUT bootstrapping
// (no layout/config changes — preserves the separate-works posture). It verifies
// connectivity + version, provisions or imports the buktio-system S3 key, and
// persists the cluster as "external".
func connectExistingCluster(ctx context.Context, cfg *config.Config, store *repository.Store, sealer secret.Sealer, logger *slog.Logger) (*repository.Cluster, error) {
	adminToken := os.Getenv("GARAGE_ADMIN_TOKEN")
	if adminToken == "" {
		return nil, fmt.Errorf("app: external mode requires GARAGE_ADMIN_TOKEN")
	}
	logger.Info("connecting to existing storage cluster", slog.String("admin", cfg.Garage.AdminURL))

	admin := garage.NewAdminClient(cfg.Garage.AdminURL, adminToken)
	status, err := admin.GetClusterStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("app: cannot reach existing cluster admin API: %w", err)
	}
	node := status.PrimaryNode()
	version := ""
	if node != nil {
		version = node.GarageVersion
		if v, perr := garage.ParseVersion(version); perr == nil {
			if err := garage.CheckSupported(v); err != nil {
				return nil, err
			}
		}
	}

	// Use an operator-supplied system key if given, else create one via the admin API.
	sysAccess := os.Getenv("BUKTIO_SYSTEM_S3_ACCESS_KEY")
	sysSecret := os.Getenv("BUKTIO_SYSTEM_S3_SECRET")
	if sysAccess == "" || sysSecret == "" {
		key, kerr := admin.CreateKey(ctx, "buktio-system", true)
		if kerr != nil {
			return nil, fmt.Errorf("app: create system key on existing cluster: %w", kerr)
		}
		sysAccess, sysSecret = key.AccessKeyID, key.SecretAccessKey
	}

	adminEnc, err := sealer.Seal([]byte(adminToken))
	if err != nil {
		return nil, err
	}
	sysEnc, err := sealer.Seal([]byte(sysSecret))
	if err != nil {
		return nil, err
	}

	c := repository.Cluster{
		Name:                "external",
		Provider:            garage.Kind,
		Mode:                "external",
		S3Endpoint:          cfg.Garage.S3URL,
		AdminEndpoint:       cfg.Garage.AdminURL,
		S3Region:            cfg.Garage.S3Region,
		GarageVersion:       version,
		AdminTokenEnc:       adminEnc, // rpc_secret_enc left nil (external = operator-owned)
		SystemS3AccessKeyID: sysAccess,
		SystemS3SecretEnc:   sysEnc,
		ReplicationFactor:   1,
		Status:              "healthy",
	}
	if _, err := store.CreateCluster(ctx, c); err != nil {
		return nil, err
	}
	return store.GetActiveCluster(ctx)
}
