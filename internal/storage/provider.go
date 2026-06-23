// Package storage defines the backend-agnostic StorageProvider interface that
// buktio's services depend on. Garage is the first implementation
// (internal/storage/garage); SeaweedFS, Ceph RGW, AWS S3, Cloudflare R2, and
// Backblaze B2 can be added later behind the same interface.
//
// Access control is abstracted to `private` vs `public-website` + per-key
// read/write/owner flags — never raw S3 ACL/policy JSON, which Garage does not
// have. Per-bucket usage comes from a single GetBucketUsage call; object
// enumeration is a separate concern. Backends richer than Garage report extra
// support via Capabilities so the UI can light up additional features.
package storage

import (
	"context"
	"io"
	"time"
)

// Permissions is Garage's exact 3-flag authorization model. Richer backends map
// their native ACL/policy down to / up from these flags.
type Permissions struct {
	Read  bool
	Write bool
	Owner bool // administrative control: aliases, website toggle
}

// Visibility abstracts bucket-level public access. Garage supports only private
// vs website hosting (no per-object public ACL).
type Visibility string

const (
	VisibilityPrivate Visibility = "private"
	VisibilityPublic  Visibility = "public-website"
)

// WebsiteConfig configures static-website (public) hosting for a bucket.
type WebsiteConfig struct {
	Enabled       bool
	IndexDocument string // default "index.html"
	ErrorDocument string
}

// Quota is a per-bucket cap. A nil field means unlimited.
type Quota struct {
	MaxSizeBytes *int64
	MaxObjects   *int64
}

// Bucket is a backend bucket as buktio sees it.
type Bucket struct {
	ID            string
	GlobalAliases []string
	LocalAliases  []string
	Visibility    Visibility
	Website       WebsiteConfig
	Quota         Quota
}

// BucketUsage is filled from a single GetBucketUsage call (no S3 full-scan).
type BucketUsage struct {
	ObjectCount                int64
	BytesUsed                  int64
	UnfinishedUploads          int64
	UnfinishedMultipartUploads int64
	UnfinishedMultipartParts   int64
	UnfinishedMultipartBytes   int64
	QuotaMaxSize               *int64
	QuotaMaxObjects            *int64
	// CapturedAt marks the (eventually-consistent) snapshot time.
	CapturedAt time.Time
}

// Credential is a full S3 access key. SecretAccessKey is populated ONLY at
// creation and is never re-retrievable.
type Credential struct {
	AccessKeyID     string
	SecretAccessKey string
	Name            string
	CanCreateBucket bool
	CreatedAt       time.Time
	ExpiresAt       *time.Time
}

// Object is an item in a bucket listing.
type Object struct {
	Key          string
	Size         int64
	LastModified time.Time
	ETag         string
	IsPrefix     bool // a CommonPrefix when delimiter="/" folder browsing
}

// ListObjectsInput parameterizes an object listing.
type ListObjectsInput struct {
	Prefix            string
	Delimiter         string // "/" for folder-style navigation
	ContinuationToken string
	MaxKeys           int32
}

// ListObjectsResult is a page of objects/prefixes.
type ListObjectsResult struct {
	Objects               []Object
	CommonPrefixes        []string
	NextContinuationToken string
	IsTruncated           bool
}

// PresignInput parameterizes a presigned URL for direct browser transfers.
type PresignInput struct {
	BucketID    string
	Key         string
	Method      string // "GET" or "PUT"
	ContentType string
	Expires     time.Duration
}

// CORSRule is one cross-origin rule (needed for browser presigned uploads).
type CORSRule struct {
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
	ExposeHeaders  []string
	MaxAgeSeconds  int
}

// LifecycleRule is an object lifecycle rule. Garage supports only Expiration and
// AbortIncompleteMultipartUpload (no storage-class transitions).
type LifecycleRule struct {
	ID                     string
	Prefix                 string
	Enabled                bool
	ExpireDays             int // 0 = unset
	AbortIncompleteMPUDays int // 0 = unset
}

// ClusterHealth summarizes backend cluster health.
type ClusterHealth struct {
	Status           string // healthy | degraded | unavailable
	KnownNodes       int
	ConnectedNodes   int
	StorageNodes     int
	StorageNodesOK   int
	Partitions       int
	PartitionsQuorum int
	PartitionsAllOK  bool
}

// NodeStatus is one node's status.
type NodeStatus struct {
	ID            string
	Hostname      string
	Addr          string
	Zone          string // layout zone (e.g. "dc1")
	Role          string // "storage" | "gateway" | "" (no role yet)
	IsUp          bool
	CapacityBytes *int64
	DiskUsed      int64
	DiskAvail     int64
	Draining      bool
}

// ClusterStatus is the cluster topology + layout version.
type ClusterStatus struct {
	LayoutVersion int
	Nodes         []NodeStatus
}

// Capabilities lets services degrade gracefully on backends that support more
// than Garage (versioning, per-object ACL, etc.) and LESS than Garage (generic-S3
// backends buktio reaches with operator-supplied credentials cannot manage keys,
// quotas, cluster health, or website hosting). Each adapter encodes its own values
// and the service + UI gate controls accordingly.
type Capabilities struct {
	ObjectVersioning     bool // Garage: false
	PerObjectPublicACL   bool // Garage: false (bucket-level website only)
	BucketCORS           bool // Garage: true
	ServerSideEncDefault bool // Garage: false (SSE-C per-request only)
	EventNotifications   bool // Garage: false
	LifecycleExpiry      bool // Garage: true (Expiration + AbortIncompleteMPU only)

	// Control-plane capabilities (v2). Garage: all true; generic-S3: all false.
	ManagesKeys      bool // buktio can create/list/revoke access keys on this backend
	ManagesQuota     bool // backend enforces per-bucket quotas buktio can set
	HasClusterHealth bool // backend exposes cluster/node health + status
	PublicWebsite    bool // backend supports buktio-toggled public-website hosting
}

// StorageProvider is the backend-agnostic control + data plane buktio depends on.
//
// The user-facing method names from the design (CreateBucket, DeleteBucket,
// ListBuckets, GetBucket, CreateAccessKey, RevokeAccessKey, SetBucketPolicy,
// GetUsage, GetHealth, GetClusterStatus, ApplyQuota, ListObjects, DeleteObject,
// GenerateConnectionInstructions) map onto the typed methods below;
// SetBucketPolicy decomposes into SetVisibility + GrantKey/RevokeKey, and
// connection instructions are produced by the API/docs layer from a Credential +
// Bucket + endpoint.
type StorageProvider interface {
	Name() string
	Capabilities() Capabilities

	// Bucket lifecycle.
	CreateBucket(ctx context.Context, alias string) (*Bucket, error)
	DeleteBucket(ctx context.Context, bucketID string) error // must be empty first
	ListBuckets(ctx context.Context) ([]Bucket, error)
	GetBucket(ctx context.Context, bucketID string) (*Bucket, error)

	// Access control abstracted to visibility + per-key caps.
	SetVisibility(ctx context.Context, bucketID string, v Visibility, w WebsiteConfig) error
	GrantKey(ctx context.Context, bucketID, accessKeyID string, p Permissions) error
	RevokeKey(ctx context.Context, bucketID, accessKeyID string) error

	// Quotas.
	SetQuota(ctx context.Context, bucketID string, q Quota) error

	// Credentials (secret captured once).
	CreateAccessKey(ctx context.Context, name string, canCreateBucket bool) (*Credential, error)
	ImportAccessKey(ctx context.Context, c Credential) error // buktio-owned secret
	DeleteAccessKey(ctx context.Context, accessKeyID string) error
	ListAccessKeys(ctx context.Context) ([]Credential, error)

	// Usage / metering (single backend call).
	GetBucketUsage(ctx context.Context, bucketID string) (*BucketUsage, error)

	// Health & cluster.
	GetClusterHealth(ctx context.Context) (*ClusterHealth, error)
	GetClusterStatus(ctx context.Context) (*ClusterStatus, error)
	Ping(ctx context.Context) error // cheap liveness probe

	// Object browser (data plane).
	ListObjects(ctx context.Context, bucketID string, in ListObjectsInput) (*ListObjectsResult, error)
	HeadObject(ctx context.Context, bucketID, key string) (*Object, error)
	DeleteObject(ctx context.Context, bucketID, key string) error
	DeleteObjects(ctx context.Context, bucketID string, keys []string) error
	CopyObject(ctx context.Context, bucketID, srcKey, dstKey string) error
	PutObject(ctx context.Context, bucketID, key string, body io.Reader, size int64, contentType string) error
	GetObject(ctx context.Context, bucketID, key string) (io.ReadCloser, *Object, error)

	// Direct browser transfers.
	Presign(ctx context.Context, in PresignInput) (url string, err error)

	// CORS (needed for browser presigned uploads).
	SetCORS(ctx context.Context, bucketID string, rules []CORSRule) error
	GetCORS(ctx context.Context, bucketID string) ([]CORSRule, error)
	DeleteCORS(ctx context.Context, bucketID string) error

	// Lifecycle (Expiration + AbortIncompleteMultipartUpload only).
	SetLifecycle(ctx context.Context, bucketID string, rules []LifecycleRule) error
	GetLifecycle(ctx context.Context, bucketID string) ([]LifecycleRule, error)
	DeleteLifecycle(ctx context.Context, bucketID string) error
}
