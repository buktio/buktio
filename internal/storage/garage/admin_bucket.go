package garage

import (
	"context"
	"net/http"
	"net/url"
)

// Garage Admin API v2 bucket/key shapes and calls (control plane). Field names
// follow the v2 docs and must be confirmed against the live garage-admin-v2.json.
// DeleteBucket/UpdateBucket/DeleteKey use path params (/v2/<Call>/{id}); some
// third-party clients use a query form — confirm before production.

// --- Bucket info (GET /v2/GetBucketInfo, returned by CreateBucket/UpdateBucket) ---

type BucketInfoResponse struct {
	ID            string         `json:"id"`
	GlobalAliases []string       `json:"globalAliases"`
	WebsiteAccess bool           `json:"websiteAccess"`
	WebsiteConfig *WebsiteAccess `json:"websiteConfig"`

	// Usage statistics (authoritative per-bucket storage signal).
	Objects                        int64 `json:"objects"`
	Bytes                          int64 `json:"bytes"`
	UnfinishedUploads              int64 `json:"unfinishedUploads"`
	UnfinishedMultipartUploads     int64 `json:"unfinishedMultipartUploads"`
	UnfinishedMultipartUploadParts int64 `json:"unfinishedMultipartUploadParts"`
	UnfinishedMultipartUploadBytes int64 `json:"unfinishedMultipartUploadBytes"`

	Quotas BucketQuotas `json:"quotas"`
}

// WebsiteAccess models bucket website (public) hosting.
type WebsiteAccess struct {
	Enabled       bool   `json:"enabled"`
	IndexDocument string `json:"indexDocument,omitempty"`
	ErrorDocument string `json:"errorDocument,omitempty"`
}

// BucketQuotas are per-bucket caps; nil fields mean unlimited.
type BucketQuotas struct {
	MaxSize    *int64 `json:"maxSize"`
	MaxObjects *int64 `json:"maxObjects"`
}

// --- Create bucket (POST /v2/CreateBucket) ---

type CreateBucketRequest struct {
	GlobalAlias string `json:"globalAlias,omitempty"`
}

// --- Update bucket (POST /v2/UpdateBucket/{id}) — quotas and/or website ---

type UpdateBucketRequest struct {
	WebsiteAccess *WebsiteAccess `json:"websiteAccess,omitempty"`
	Quotas        *BucketQuotas  `json:"quotas,omitempty"`
}

// --- List buckets (GET /v2/ListBuckets) ---

type ListBucketsItem struct {
	ID            string   `json:"id"`
	GlobalAliases []string `json:"globalAliases"`
	LocalAliases  []struct {
		AccessKeyID string `json:"accessKeyId"`
		Alias       string `json:"alias"`
	} `json:"localAliases"`
}

// --- List keys (GET /v2/ListKeys) ---

type KeyListItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// --- Import key (POST /v2/ImportKey) — buktio-owned secret ---

type ImportKeyRequest struct {
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
	Name            string `json:"name,omitempty"`
}

// CreateBucket creates a bucket with the given global alias.
func (c *AdminClient) CreateBucket(ctx context.Context, globalAlias string) (*BucketInfoResponse, error) {
	var out BucketInfoResponse
	if err := c.doJSON(ctx, http.MethodPost, "/v2/CreateBucket",
		CreateBucketRequest{GlobalAlias: globalAlias}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetBucketInfo returns bucket details + usage by bucket id.
func (c *AdminClient) GetBucketInfo(ctx context.Context, bucketID string) (*BucketInfoResponse, error) {
	var out BucketInfoResponse
	path := "/v2/GetBucketInfo?id=" + url.QueryEscape(bucketID)
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListBuckets returns all buckets with their aliases.
func (c *AdminClient) ListBuckets(ctx context.Context) ([]ListBucketsItem, error) {
	var out []ListBucketsItem
	if err := c.doJSON(ctx, http.MethodGet, "/v2/ListBuckets", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateBucket sets website access and/or quotas.
func (c *AdminClient) UpdateBucket(ctx context.Context, bucketID string, req UpdateBucketRequest) (*BucketInfoResponse, error) {
	var out BucketInfoResponse
	path := "/v2/UpdateBucket?id=" + url.QueryEscape(bucketID)
	if err := c.doJSON(ctx, http.MethodPost, path, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteBucket deletes a bucket (Garage requires it to be empty first).
func (c *AdminClient) DeleteBucket(ctx context.Context, bucketID string) error {
	return c.doJSON(ctx, http.MethodPost, "/v2/DeleteBucket?id="+url.QueryEscape(bucketID), nil, nil)
}

// ListKeys returns all access keys (id + name; no secrets).
func (c *AdminClient) ListKeys(ctx context.Context) ([]KeyListItem, error) {
	var out []KeyListItem
	if err := c.doJSON(ctx, http.MethodGet, "/v2/ListKeys", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteKey deletes (fully revokes) an access key.
func (c *AdminClient) DeleteKey(ctx context.Context, accessKeyID string) error {
	return c.doJSON(ctx, http.MethodPost, "/v2/DeleteKey?id="+url.QueryEscape(accessKeyID), nil, nil)
}

// DenyBucketKey revokes a key's permissions on a bucket.
func (c *AdminClient) DenyBucketKey(ctx context.Context, bucketID, accessKeyID string, p KeyPermissions) error {
	return c.doJSON(ctx, http.MethodPost, "/v2/DenyBucketKey",
		AllowBucketKeyRequest{BucketID: bucketID, AccessKeyID: accessKeyID, Permissions: p}, nil)
}

// ImportKey imports a buktio-generated key so buktio owns the secret.
func (c *AdminClient) ImportKey(ctx context.Context, accessKeyID, secretAccessKey, name string) error {
	return c.doJSON(ctx, http.MethodPost, "/v2/ImportKey",
		ImportKeyRequest{AccessKeyID: accessKeyID, SecretAccessKey: secretAccessKey, Name: name}, nil)
}
