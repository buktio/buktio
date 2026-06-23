// Package s3generic implements storage.StorageProvider for S3-compatible backends
// that buktio reaches with operator-supplied credentials and NO admin plane:
// Cloudflare R2, AWS S3, Backblaze B2, SeaweedFS, and Ceph RGW.
//
// It is the "not just a Garage UI" move: the same object browser, presign, CORS,
// and lifecycle work against any S3 backend. Operations that need a control plane
// buktio doesn't own — managing access keys, quotas, cluster health, website
// hosting — return storage.ErrUnsupported, and Capabilities() advertises the gap so
// the UI hides those controls.
package s3generic

import (
	"context"
	"io"

	"github.com/buktio/buktio/internal/storage"
	"github.com/buktio/buktio/internal/storage/s3core"
)

// Registry kinds (must match the DB provider_type enum).
const (
	KindAWSS3     = "aws_s3"
	KindR2        = "r2"
	KindB2        = "b2"
	KindSeaweedFS = "seaweedfs"
	KindCephRGW   = "ceph_rgw"
)

func init() {
	for _, k := range []string{KindAWSS3, KindR2, KindB2, KindSeaweedFS, KindCephRGW} {
		storage.Register(k, newProvider)
	}
}

// Provider implements storage.StorageProvider against a generic S3 backend.
type Provider struct {
	kind string
	s3   *s3core.Client
}

var _ storage.StorageProvider = (*Provider)(nil)

func newProvider(cfg storage.ProviderConfig) (storage.StorageProvider, error) {
	// Presign against the public endpoint if set, else the S3 endpoint itself
	// (generic backends are usually directly reachable by clients).
	public := cfg.Extra["s3_public_endpoint"]
	if public == "" {
		public = cfg.S3Endpoint
	}
	return &Provider{
		kind: cfg.Kind,
		s3: s3core.New(
			cfg.S3Endpoint,
			public,
			cfg.S3Region,
			cfg.SystemKey.AccessKeyID,
			cfg.SystemKey.SecretAccessKey,
		),
	}, nil
}

func (p *Provider) Name() string { return p.kind }

// Capabilities reports what each generic-S3 backend supports. Control-plane
// features (keys/quota/cluster-health/website) are uniformly unavailable; the
// object-plane subset (CORS, lifecycle) is on. AWS S3 additionally exposes
// versioning + per-object ACLs.
func (p *Provider) Capabilities() storage.Capabilities {
	c := storage.Capabilities{
		BucketCORS:      true,
		LifecycleExpiry: true,
		// control-plane: all false for operator-credentialed generic backends
		ManagesKeys:      false,
		ManagesQuota:     false,
		HasClusterHealth: false,
		PublicWebsite:    false,
	}
	if p.kind == KindAWSS3 {
		c.ObjectVersioning = true
		c.PerObjectPublicACL = true
	}
	return c
}

// Ping verifies credentials + reachability cheaply (ListBuckets).
func (p *Provider) Ping(ctx context.Context) error {
	_, err := p.s3.ListBuckets(ctx)
	return err
}

// --- bucket lifecycle (plain S3; bucket id == name == alias) ---

func (p *Provider) CreateBucket(ctx context.Context, alias string) (*storage.Bucket, error) {
	if err := p.s3.CreateBucket(ctx, alias); err != nil {
		return nil, err
	}
	return &storage.Bucket{ID: alias, GlobalAliases: []string{alias}, Visibility: storage.VisibilityPrivate}, nil
}

func (p *Provider) DeleteBucket(ctx context.Context, bucketID string) error {
	return p.s3.DeleteBucket(ctx, bucketID)
}

func (p *Provider) ListBuckets(ctx context.Context) ([]storage.Bucket, error) {
	names, err := p.s3.ListBuckets(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]storage.Bucket, 0, len(names))
	for _, n := range names {
		out = append(out, storage.Bucket{ID: n, GlobalAliases: []string{n}})
	}
	return out, nil
}

func (p *Provider) GetBucket(ctx context.Context, bucketID string) (*storage.Bucket, error) {
	return &storage.Bucket{ID: bucketID, GlobalAliases: []string{bucketID}, Visibility: storage.VisibilityPrivate}, nil
}

// --- control plane buktio does not own on generic backends ---

func (p *Provider) SetVisibility(ctx context.Context, bucketID string, v storage.Visibility, w storage.WebsiteConfig) error {
	return storage.ErrUnsupported
}
func (p *Provider) GrantKey(ctx context.Context, bucketID, accessKeyID string, perm storage.Permissions) error {
	return storage.ErrUnsupported
}
func (p *Provider) RevokeKey(ctx context.Context, bucketID, accessKeyID string) error {
	return storage.ErrUnsupported
}
func (p *Provider) SetQuota(ctx context.Context, bucketID string, q storage.Quota) error {
	return storage.ErrUnsupported
}
func (p *Provider) CreateAccessKey(ctx context.Context, name string, canCreateBucket bool) (*storage.Credential, error) {
	return nil, storage.ErrUnsupported
}
func (p *Provider) ImportAccessKey(ctx context.Context, c storage.Credential) error {
	return storage.ErrUnsupported
}
func (p *Provider) DeleteAccessKey(ctx context.Context, accessKeyID string) error {
	return storage.ErrUnsupported
}
func (p *Provider) ListAccessKeys(ctx context.Context) ([]storage.Credential, error) {
	return nil, storage.ErrUnsupported
}
func (p *Provider) GetClusterHealth(ctx context.Context) (*storage.ClusterHealth, error) {
	return nil, storage.ErrUnsupported
}
func (p *Provider) GetClusterStatus(ctx context.Context) (*storage.ClusterStatus, error) {
	return nil, storage.ErrUnsupported
}

// --- usage (no GetBucketInfo; full ListObjectsV2 scan) ---

func (p *Provider) GetBucketUsage(ctx context.Context, bucketID string) (*storage.BucketUsage, error) {
	return p.s3.UsageByScan(ctx, bucketID)
}

// --- object plane + presign + CORS + lifecycle (s3core) ---

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
func (p *Provider) Presign(ctx context.Context, in storage.PresignInput) (string, error) {
	return p.s3.PresignURL(ctx, in)
}
func (p *Provider) SetCORS(ctx context.Context, bucket string, rules []storage.CORSRule) error {
	return p.s3.SetCORS(ctx, bucket, rules)
}
func (p *Provider) GetCORS(ctx context.Context, bucket string) ([]storage.CORSRule, error) {
	return p.s3.GetCORS(ctx, bucket)
}
func (p *Provider) DeleteCORS(ctx context.Context, bucket string) error {
	return p.s3.DeleteCORS(ctx, bucket)
}
func (p *Provider) SetLifecycle(ctx context.Context, bucket string, rules []storage.LifecycleRule) error {
	return p.s3.SetLifecycle(ctx, bucket, rules)
}
func (p *Provider) GetLifecycle(ctx context.Context, bucket string) ([]storage.LifecycleRule, error) {
	return p.s3.GetLifecycle(ctx, bucket)
}
func (p *Provider) DeleteLifecycle(ctx context.Context, bucket string) error {
	return p.s3.DeleteLifecycle(ctx, bucket)
}
