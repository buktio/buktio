package service

import (
	"context"
	"time"

	"github.com/buktio/buktio/internal/authz"
	"github.com/buktio/buktio/internal/repository"
	"github.com/buktio/buktio/internal/storage"
)

// CapabilitiesDTO mirrors storage.Capabilities for the API/UI capability matrix.
type CapabilitiesDTO struct {
	ObjectVersioning   bool `json:"object_versioning"`
	PerObjectPublicACL bool `json:"per_object_public_acl"`
	BucketCORS         bool `json:"bucket_cors"`
	LifecycleExpiry    bool `json:"lifecycle_expiry"`
	ManagesKeys        bool `json:"manages_keys"`
	ManagesQuota       bool `json:"manages_quota"`
	HasClusterHealth   bool `json:"has_cluster_health"`
	PublicWebsite      bool `json:"public_website"`
}

func capsToDTO(c storage.Capabilities) CapabilitiesDTO {
	return CapabilitiesDTO{
		ObjectVersioning:   c.ObjectVersioning,
		PerObjectPublicACL: c.PerObjectPublicACL,
		BucketCORS:         c.BucketCORS,
		LifecycleExpiry:    c.LifecycleExpiry,
		ManagesKeys:        c.ManagesKeys,
		ManagesQuota:       c.ManagesQuota,
		HasClusterHealth:   c.HasClusterHealth,
		PublicWebsite:      c.PublicWebsite,
	}
}

// ClusterDTO is a storage backend as the API returns it.
type ClusterDTO struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Provider     string          `json:"provider"` // kind: garage | aws_s3 | r2 | b2 | seaweedfs | ceph_rgw
	Mode         string          `json:"mode"`     // managed | external
	S3Endpoint   string          `json:"s3_endpoint"`
	S3Region     string          `json:"s3_region"`
	Status       string          `json:"status"`
	IsPrimary    bool            `json:"is_primary"`
	BucketCount  int             `json:"bucket_count"`
	Capabilities CapabilitiesDTO `json:"capabilities"`
}

// AddClusterInput connects an external generic-S3 backend.
type AddClusterInput struct {
	Name            string
	Provider        string // kind (generic-S3 only)
	S3Endpoint      string
	S3Region        string
	AccessKeyID     string
	SecretAccessKey string
	PublicEndpoint  string // optional; defaults to S3Endpoint
}

var genericKinds = map[string]bool{
	"aws_s3": true, "r2": true, "b2": true, "seaweedfs": true, "ceph_rgw": true,
}

func (s *Services) clusterToDTO(ctx context.Context, c *repository.Cluster) ClusterDTO {
	dto := ClusterDTO{
		ID:         c.ID,
		Name:       c.Name,
		Provider:   c.Provider,
		Mode:       c.Mode,
		S3Endpoint: c.S3Endpoint,
		S3Region:   c.S3Region,
		Status:     c.Status,
		IsPrimary:  c.ID == s.ClusterID,
	}
	if n, err := s.Store.CountBucketsInCluster(ctx, c.ID); err == nil {
		dto.BucketCount = n
	}
	if s.Reg != nil {
		if p, err := s.Reg.Provider(ctx, c.ID); err == nil {
			dto.Capabilities = capsToDTO(p.Capabilities())
		}
	}
	return dto
}

// ListClusters returns all storage backends with their capabilities.
func (s *Services) ListClusters(ctx context.Context) ([]ClusterDTO, error) {
	rows, err := s.Store.ListClusters(ctx)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	out := make([]ClusterDTO, 0, len(rows))
	for i := range rows {
		out = append(out, s.clusterToDTO(ctx, &rows[i]))
	}
	return out, nil
}

// GetCluster returns one storage backend.
func (s *Services) GetCluster(ctx context.Context, id string) (*ClusterDTO, error) {
	c, err := s.Store.GetClusterByID(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	dto := s.clusterToDTO(ctx, c)
	return &dto, nil
}

// AddCluster connects an external generic-S3 backend: it validates the input,
// verifies connectivity with the supplied credentials, encrypts the secret, and
// persists the cluster. buktio manages buckets/objects/CORS/lifecycle on it via the
// S3 API only — no keys/quota/health/website (see Capabilities).
func (s *Services) AddCluster(ctx context.Context, in AddClusterInput) (*ClusterDTO, error) {
	if err := s.authorize(ctx, authz.ActionCreate, authz.Target{Kind: authz.ResourceCluster}); err != nil {
		return nil, err
	}
	if in.Name == "" {
		return nil, validationErr("name is required")
	}
	if !genericKinds[in.Provider] {
		return nil, validationErr("provider must be one of: aws_s3, r2, b2, seaweedfs, ceph_rgw")
	}
	if in.AccessKeyID == "" || in.SecretAccessKey == "" {
		return nil, validationErr("access key id and secret are required")
	}
	if in.Provider != "aws_s3" && in.S3Endpoint == "" {
		return nil, validationErr("s3_endpoint is required for this provider")
	}
	region := in.S3Region
	if region == "" {
		if in.Provider == "r2" {
			region = "auto"
		} else {
			region = "us-east-1"
		}
	}
	public := in.PublicEndpoint
	if public == "" {
		public = in.S3Endpoint
	}

	// Verify connectivity with the supplied credentials before persisting.
	probe, perr := storage.New(storage.ProviderConfig{
		Kind:       in.Provider,
		S3Endpoint: in.S3Endpoint,
		S3Region:   region,
		SystemKey:  storage.Credential{AccessKeyID: in.AccessKeyID, SecretAccessKey: in.SecretAccessKey},
		Extra:      map[string]string{"s3_public_endpoint": public},
	})
	if perr != nil {
		return nil, validationErr("unsupported provider: " + perr.Error())
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := probe.Ping(pingCtx); err != nil {
		return nil, storageUnavailableErr("could not connect to backend with supplied credentials: " + err.Error())
	}

	sysEnc, err := s.Sealer.Seal([]byte(in.SecretAccessKey))
	if err != nil {
		return nil, mapRepoErr(err)
	}

	id, err := s.Store.CreateCluster(ctx, repository.Cluster{
		Name:                in.Name,
		Provider:            in.Provider,
		Mode:                "external",
		S3Endpoint:          in.S3Endpoint,
		AdminEndpoint:       "", // generic-S3 has no admin plane
		S3Region:            region,
		WebEndpoint:         public, // presign endpoint (client-reachable host); empty => S3Endpoint
		SystemS3AccessKeyID: in.AccessKeyID,
		SystemS3SecretEnc:   sysEnc,
		DBEngine:            "s3",
		ReplicationFactor:   1,
		Status:              "healthy",
	})
	if err != nil {
		return nil, mapRepoErr(err)
	}
	if s.Reg != nil {
		s.Reg.Invalidate(id)
	}
	s.audit(ctx, "cluster.add", "cluster", id, map[string]any{"name": in.Name, "provider": in.Provider})
	return s.GetCluster(ctx, id)
}

// RemoveCluster soft-deletes a non-primary, empty backend.
func (s *Services) RemoveCluster(ctx context.Context, id string) error {
	if err := s.authorize(ctx, authz.ActionDelete, authz.Target{Kind: authz.ResourceCluster, ID: id}); err != nil {
		return err
	}
	c, err := s.Store.GetClusterByID(ctx, id)
	if err != nil {
		return mapRepoErr(err)
	}
	if c.ID == s.ClusterID {
		return conflictErr("cannot remove the primary cluster")
	}
	n, err := s.Store.CountBucketsInCluster(ctx, id)
	if err != nil {
		return mapRepoErr(err)
	}
	if n > 0 {
		return conflictErr("remove the cluster's buckets first")
	}
	if err := s.Store.SoftDeleteCluster(ctx, id); err != nil {
		return mapRepoErr(err)
	}
	if s.Reg != nil {
		s.Reg.Invalidate(id)
	}
	s.audit(ctx, "cluster.remove", "cluster", id, map[string]any{"name": c.Name})
	return nil
}
