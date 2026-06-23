package garage

import (
	"context"
	"io"
	"time"

	"github.com/buktio/buktio/internal/storage"
	"github.com/buktio/buktio/internal/storage/s3core"
)

// Kind is the registry key for the Garage backend.
const Kind = "garage"

func init() {
	storage.Register(Kind, newProvider)
}

// Provider implements storage.StorageProvider against Garage.
type Provider struct {
	admin *AdminClient
	s3    *s3core.Client
}

// newProvider is the registry factory.
func newProvider(cfg storage.ProviderConfig) (storage.StorageProvider, error) {
	return &Provider{
		admin: NewAdminClient(cfg.AdminURL, cfg.AdminToken),
		s3: s3core.New(
			cfg.S3Endpoint,
			cfg.Extra["s3_public_endpoint"],
			cfg.S3Region,
			cfg.SystemKey.AccessKeyID,
			cfg.SystemKey.SecretAccessKey,
		),
	}, nil
}

// Compile-time assertion that *Provider satisfies the interface.
var _ storage.StorageProvider = (*Provider)(nil)

func (p *Provider) Name() string { return Kind }

// Capabilities reports Garage's real S3 subset. Garage is buktio's fullest-featured
// backend: it manages keys, quotas, cluster health, and public-website hosting.
func (p *Provider) Capabilities() storage.Capabilities {
	return storage.Capabilities{
		ObjectVersioning:     false,
		PerObjectPublicACL:   false,
		BucketCORS:           true,
		ServerSideEncDefault: false,
		EventNotifications:   false,
		LifecycleExpiry:      true,
		ManagesKeys:          true,
		ManagesQuota:         true,
		HasClusterHealth:     true,
		PublicWebsite:        true,
	}
}

// Ping is the cheap liveness probe (admin GET /health).
func (p *Provider) Ping(ctx context.Context) error { return p.admin.Health(ctx) }

// --- Bucket lifecycle (Admin API v2) — wired in M3 ---

func (p *Provider) CreateBucket(ctx context.Context, alias string) (*storage.Bucket, error) {
	info, err := p.admin.CreateBucket(ctx, alias)
	if err != nil {
		return nil, err
	}
	return bucketFromInfo(info), nil
}

// DeleteBucket deletes the bucket. Garage requires it to be empty; the service
// layer empties it via the S3 plane first (DeleteObjects) before calling this.
func (p *Provider) DeleteBucket(ctx context.Context, bucketID string) error {
	return p.admin.DeleteBucket(ctx, bucketID)
}

func (p *Provider) ListBuckets(ctx context.Context) ([]storage.Bucket, error) {
	items, err := p.admin.ListBuckets(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]storage.Bucket, 0, len(items))
	for _, it := range items {
		out = append(out, storage.Bucket{ID: it.ID, GlobalAliases: it.GlobalAliases})
	}
	return out, nil
}

func (p *Provider) GetBucket(ctx context.Context, bucketID string) (*storage.Bucket, error) {
	info, err := p.admin.GetBucketInfo(ctx, bucketID)
	if err != nil {
		return nil, err
	}
	return bucketFromInfo(info), nil
}

// --- Access control (Admin API v2) — wired in M3 ---

func (p *Provider) SetVisibility(ctx context.Context, bucketID string, v storage.Visibility, w storage.WebsiteConfig) error {
	wa := &WebsiteAccess{Enabled: v == storage.VisibilityPublic}
	if wa.Enabled {
		wa.IndexDocument = w.IndexDocument
		if wa.IndexDocument == "" {
			wa.IndexDocument = "index.html"
		}
		wa.ErrorDocument = w.ErrorDocument
	}
	_, err := p.admin.UpdateBucket(ctx, bucketID, UpdateBucketRequest{WebsiteAccess: wa})
	return err
}

func (p *Provider) GrantKey(ctx context.Context, bucketID, accessKeyID string, perm storage.Permissions) error {
	return p.admin.AllowBucketKey(ctx, bucketID, accessKeyID, keyPermsFromStorage(perm))
}

func (p *Provider) RevokeKey(ctx context.Context, bucketID, accessKeyID string) error {
	return p.admin.DenyBucketKey(ctx, bucketID, accessKeyID, KeyPermissions{Read: true, Write: true, Owner: true})
}

// --- Quotas (Admin API v2) — wired in M3 ---

func (p *Provider) SetQuota(ctx context.Context, bucketID string, q storage.Quota) error {
	_, err := p.admin.UpdateBucket(ctx, bucketID, UpdateBucketRequest{
		Quotas: &BucketQuotas{MaxSize: q.MaxSizeBytes, MaxObjects: q.MaxObjects},
	})
	return err
}

// --- Credentials (Admin API v2) — wired in M3 ---

func (p *Provider) CreateAccessKey(ctx context.Context, name string, canCreateBucket bool) (*storage.Credential, error) {
	k, err := p.admin.CreateKey(ctx, name, canCreateBucket)
	if err != nil {
		return nil, err
	}
	return &storage.Credential{
		AccessKeyID:     k.AccessKeyID,
		SecretAccessKey: k.SecretAccessKey, // captured ONCE
		Name:            k.Name,
		CanCreateBucket: canCreateBucket,
	}, nil
}

func (p *Provider) ImportAccessKey(ctx context.Context, c storage.Credential) error {
	return p.admin.ImportKey(ctx, c.AccessKeyID, c.SecretAccessKey, c.Name)
}

func (p *Provider) DeleteAccessKey(ctx context.Context, accessKeyID string) error {
	return p.admin.DeleteKey(ctx, accessKeyID)
}

func (p *Provider) ListAccessKeys(ctx context.Context) ([]storage.Credential, error) {
	keys, err := p.admin.ListKeys(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]storage.Credential, 0, len(keys))
	for _, k := range keys {
		out = append(out, storage.Credential{AccessKeyID: k.ID, Name: k.Name})
	}
	return out, nil
}

// --- Usage (Admin API v2: GetBucketInfo) — wired in M3 ---

func (p *Provider) GetBucketUsage(ctx context.Context, bucketID string) (*storage.BucketUsage, error) {
	info, err := p.admin.GetBucketInfo(ctx, bucketID)
	if err != nil {
		return nil, err
	}
	return usageFromInfo(info, time.Now().UTC()), nil
}

// --- Health & cluster (Admin API v2) — wired in M2 ---

func (p *Provider) GetClusterHealth(ctx context.Context) (*storage.ClusterHealth, error) {
	h, err := p.admin.GetClusterHealth(ctx)
	if err != nil {
		return nil, err
	}
	return &storage.ClusterHealth{
		Status:           h.Status,
		KnownNodes:       h.KnownNodes,
		ConnectedNodes:   h.ConnectedNodes,
		StorageNodes:     h.StorageNodes,
		StorageNodesOK:   h.StorageNodesOK,
		Partitions:       h.Partitions,
		PartitionsQuorum: h.PartitionsQuorum,
		PartitionsAllOK:  h.PartitionsAllOK == h.Partitions && h.Partitions > 0,
	}, nil
}

func (p *Provider) GetClusterStatus(ctx context.Context) (*storage.ClusterStatus, error) {
	s, err := p.admin.GetClusterStatus(ctx)
	if err != nil {
		return nil, err
	}
	out := &storage.ClusterStatus{LayoutVersion: s.LayoutVersion}
	for _, n := range s.Nodes {
		ns := storage.NodeStatus{
			ID:       n.ID,
			Hostname: n.Hostname,
			Addr:     n.Addr,
			IsUp:     n.IsUp,
			Draining: n.Draining,
		}
		if n.Role != nil {
			ns.Zone = n.Role.Zone
			ns.CapacityBytes = n.Role.Capacity
			if n.Role.Capacity != nil {
				ns.Role = "storage"
			} else {
				ns.Role = "gateway"
			}
		}
		if n.DataPartition != nil {
			ns.DiskAvail = n.DataPartition.Available
			ns.DiskUsed = n.DataPartition.Total - n.DataPartition.Available
		}
		out.Nodes = append(out.Nodes, ns)
	}
	return out, nil
}

// --- Object browser (S3 API) — wired in M4 ---
//
// For these methods the bucket identifier is the S3-addressable bucket NAME
// (Garage global alias), which the service layer resolves from the bucket id.

func (p *Provider) ListObjects(ctx context.Context, bucket string, in storage.ListObjectsInput) (*storage.ListObjectsResult, error) {
	return p.s3.ListObjects(ctx, bucket, in)
}
func (p *Provider) HeadObject(ctx context.Context, bucket, key string) (*storage.Object, error) {
	return p.s3.HeadObject(ctx, bucket, key)
}
func (p *Provider) DeleteObject(ctx context.Context, bucket, key string) error {
	return p.s3.DeleteObject(ctx, bucket, key)
}
func (p *Provider) DeleteObjects(ctx context.Context, bucket string, keys []string) error {
	return p.s3.DeleteObjects(ctx, bucket, keys)
}
func (p *Provider) CopyObject(ctx context.Context, bucket, srcKey, dstKey string) error {
	return p.s3.CopyObject(ctx, bucket, srcKey, dstKey)
}
func (p *Provider) PutObject(ctx context.Context, bucket, key string, body io.Reader, size int64, contentType string) error {
	return p.s3.PutObject(ctx, bucket, key, body, size, contentType)
}
func (p *Provider) GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, *storage.Object, error) {
	return p.s3.GetObject(ctx, bucket, key)
}

// --- Presign (S3 API) — wired in M4 ---

func (p *Provider) Presign(ctx context.Context, in storage.PresignInput) (string, error) {
	return p.s3.PresignURL(ctx, in)
}

// --- CORS (S3 API) — wired in M4 ---

func (p *Provider) SetCORS(ctx context.Context, bucket string, rules []storage.CORSRule) error {
	return p.s3.SetCORS(ctx, bucket, rules)
}
func (p *Provider) GetCORS(ctx context.Context, bucket string) ([]storage.CORSRule, error) {
	return p.s3.GetCORS(ctx, bucket)
}
func (p *Provider) DeleteCORS(ctx context.Context, bucket string) error {
	return p.s3.DeleteCORS(ctx, bucket)
}

// --- Lifecycle (S3 API) — v1.1 ---

func (p *Provider) SetLifecycle(ctx context.Context, bucket string, rules []storage.LifecycleRule) error {
	return p.s3.SetLifecycle(ctx, bucket, rules)
}
func (p *Provider) GetLifecycle(ctx context.Context, bucket string) ([]storage.LifecycleRule, error) {
	return p.s3.GetLifecycle(ctx, bucket)
}
func (p *Provider) DeleteLifecycle(ctx context.Context, bucket string) error {
	return p.s3.DeleteLifecycle(ctx, bucket)
}
