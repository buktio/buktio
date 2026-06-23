// Package cluster holds the multi-cluster provider registry: it lazily builds and
// caches one storage.StorageProvider per storage cluster, decrypting the cluster's
// secrets from PostgreSQL. This lifts v1's single-provider assumption so buktio can
// manage several backends (Garage clusters + generic S3 backends like R2/S3/B2).
package cluster

import (
	"context"
	"fmt"
	"sync"

	"github.com/buktio/buktio/internal/repository"
	"github.com/buktio/buktio/internal/secret"
	"github.com/buktio/buktio/internal/storage"
	"github.com/buktio/buktio/internal/storage/garage"
	_ "github.com/buktio/buktio/internal/storage/s3generic" // register generic-S3 backends
)

// Registry resolves a StorageProvider for a cluster id, building it on first use.
type Registry struct {
	store                 *repository.Store
	sealer                secret.Sealer
	primaryPublicEndpoint string // presign endpoint for the primary Garage cluster

	mu    sync.Mutex
	cache map[string]storage.StorageProvider
}

// NewRegistry builds the registry. primaryPublicEndpoint is the S3 public endpoint
// used when signing presigned URLs for the primary Garage cluster.
func NewRegistry(store *repository.Store, sealer secret.Sealer, primaryPublicEndpoint string) *Registry {
	return &Registry{
		store:                 store,
		sealer:                sealer,
		primaryPublicEndpoint: primaryPublicEndpoint,
		cache:                 map[string]storage.StorageProvider{},
	}
}

// Provider returns (building + caching if needed) the provider for a cluster id.
func (r *Registry) Provider(ctx context.Context, clusterID string) (storage.StorageProvider, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.cache[clusterID]; ok {
		return p, nil
	}
	c, err := r.store.GetClusterByID(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	p, err := r.build(c)
	if err != nil {
		return nil, err
	}
	r.cache[clusterID] = p
	return p, nil
}

// Primary returns the provider + row for the primary (first/Garage) cluster.
func (r *Registry) Primary(ctx context.Context) (storage.StorageProvider, *repository.Cluster, error) {
	c, err := r.store.GetActiveCluster(ctx)
	if err != nil {
		return nil, nil, err
	}
	p, err := r.Provider(ctx, c.ID)
	if err != nil {
		return nil, nil, err
	}
	return p, c, nil
}

// ProviderForOrg resolves the provider + cluster id for an org's default cluster
// (Enterprise per-org-cluster add-on). When the org has no assignment it falls back
// to the primary cluster, so single-cluster/OSS deployments are unaffected.
func (r *Registry) ProviderForOrg(ctx context.Context, orgID string) (storage.StorageProvider, string, error) {
	if orgID != "" {
		if cid, err := r.store.DefaultClusterForOrg(ctx, orgID); err == nil && cid != "" {
			p, perr := r.Provider(ctx, cid)
			if perr != nil {
				return nil, "", perr
			}
			return p, cid, nil
		}
	}
	p, c, err := r.Primary(ctx)
	if err != nil {
		return nil, "", err
	}
	return p, c.ID, nil
}

// Invalidate drops a cached provider (after a cluster is updated/removed).
func (r *Registry) Invalidate(clusterID string) {
	r.mu.Lock()
	delete(r.cache, clusterID)
	r.mu.Unlock()
}

// build constructs a StorageProvider from a (decrypted) cluster row.
func (r *Registry) build(c *repository.Cluster) (storage.StorageProvider, error) {
	adminToken := ""
	if len(c.AdminTokenEnc) > 0 {
		b, err := r.sealer.Open(c.AdminTokenEnc)
		if err != nil {
			return nil, fmt.Errorf("cluster: decrypt admin token: %w", err)
		}
		adminToken = string(b)
	}
	systemSecret := ""
	if len(c.SystemS3SecretEnc) > 0 {
		b, err := r.sealer.Open(c.SystemS3SecretEnc)
		if err != nil {
			return nil, fmt.Errorf("cluster: decrypt system key secret: %w", err)
		}
		systemSecret = string(b)
	}

	// Garage presigns against the configured public endpoint; generic-S3 backends
	// presign against their stored public/web endpoint (the client-reachable host
	// supplied when the backend was added), falling back to the S3 endpoint.
	publicEndpoint := r.primaryPublicEndpoint
	if c.Provider != garage.Kind {
		publicEndpoint = c.WebEndpoint
		if publicEndpoint == "" {
			publicEndpoint = c.S3Endpoint
		}
	}

	return storage.New(storage.ProviderConfig{
		Kind:       c.Provider, // DB provider enum == registered provider kind
		S3Endpoint: c.S3Endpoint,
		S3Region:   c.S3Region,
		AdminURL:   c.AdminEndpoint,
		AdminToken: adminToken,
		SystemKey:  storage.Credential{AccessKeyID: c.SystemS3AccessKeyID, SecretAccessKey: systemSecret},
		Extra:      map[string]string{"s3_public_endpoint": publicEndpoint},
	})
}
