package service

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/buktio/buktio/internal/authz"
	"github.com/buktio/buktio/internal/metering"
	"github.com/buktio/buktio/internal/storage"
)

// ObjectDTO is an entry in an object listing (an object or a folder prefix).
type ObjectDTO struct {
	Type         string    `json:"type"` // "object" | "prefix"
	Key          string    `json:"key"`
	SizeBytes    int64     `json:"size_bytes,omitempty"`
	ETag         string    `json:"etag,omitempty"`
	LastModified time.Time `json:"last_modified,omitempty"`
}

// ObjectListDTO is a page of an object listing.
type ObjectListDTO struct {
	Data           []ObjectDTO `json:"data"`
	CommonPrefixes []string    `json:"common_prefixes"`
	NextCursor     string      `json:"next_cursor,omitempty"`
	IsTruncated    bool        `json:"is_truncated"`
}

// ListObjects lists objects in a bucket (delimiter "/" gives folder-style nav).
func (s *Services) ListObjects(ctx context.Context, bucketID, prefix, delimiter, cursor string, maxKeys int32) (*ObjectListDTO, error) {
	b, prov, err := s.bucketProvider(ctx, bucketID)
	if err != nil {
		return nil, err
	}
	res, err := prov.ListObjects(ctx, b.GarageGlobalAlias, storage.ListObjectsInput{
		Prefix: prefix, Delimiter: delimiter, ContinuationToken: cursor, MaxKeys: maxKeys,
	})
	if err != nil {
		return nil, mapStorageErr(err)
	}
	out := &ObjectListDTO{
		Data:           make([]ObjectDTO, 0, len(res.Objects)),
		CommonPrefixes: res.CommonPrefixes,
		NextCursor:     res.NextContinuationToken,
		IsTruncated:    res.IsTruncated,
	}
	// Hide the reserved trash prefix from normal listings.
	cp := make([]string, 0, len(res.CommonPrefixes))
	for _, p := range res.CommonPrefixes {
		if !strings.HasPrefix(p, trashPrefix) {
			cp = append(cp, p)
		}
	}
	out.CommonPrefixes = cp
	for _, o := range res.Objects {
		if strings.HasPrefix(o.Key, trashPrefix) {
			continue
		}
		out.Data = append(out.Data, ObjectDTO{
			Type: "object", Key: o.Key, SizeBytes: o.Size, ETag: o.ETag, LastModified: o.LastModified,
		})
	}
	return out, nil
}

// DeleteObjects removes objects. By default they are moved to the bucket's trash
// (recoverable); permanent=true hard-deletes them.
func (s *Services) DeleteObjects(ctx context.Context, bucketID string, keys []string, permanent bool) error {
	if err := s.authorize(ctx, authz.ActionDelete, authz.Target{Kind: authz.ResourceObject}); err != nil {
		return err
	}
	if len(keys) == 0 {
		return validationErr("no object keys provided")
	}
	b, prov, err := s.bucketProvider(ctx, bucketID)
	if err != nil {
		return err
	}
	if permanent {
		if err := prov.DeleteObjects(ctx, b.GarageGlobalAlias, keys); err != nil {
			return mapStorageErr(err)
		}
		s.audit(ctx, "object.delete", "bucket", bucketID, map[string]any{"count": len(keys), "permanent": true})
		for _, k := range keys {
			s.fireWebhook(bucketID, EventObjectDeleted, k)
		}
		return nil
	}
	if err := s.trashObjects(ctx, b, keys); err != nil {
		return err
	}
	for _, k := range keys {
		s.fireWebhook(bucketID, EventObjectDeleted, k)
	}
	return nil
}

// HeadObject returns an object's metadata.
func (s *Services) HeadObject(ctx context.Context, bucketID, key string) (*ObjectDTO, error) {
	b, prov, err := s.bucketProvider(ctx, bucketID)
	if err != nil {
		return nil, err
	}
	o, gerr := prov.HeadObject(ctx, b.GarageGlobalAlias, key)
	if gerr != nil {
		return nil, mapStorageErr(gerr)
	}
	return &ObjectDTO{Type: "object", Key: o.Key, SizeBytes: o.Size, ETag: o.ETag, LastModified: o.LastModified}, nil
}

// CopyObject copies an object within a bucket.
func (s *Services) CopyObject(ctx context.Context, bucketID, srcKey, dstKey string) error {
	if err := s.authorize(ctx, authz.ActionCreate, authz.Target{Kind: authz.ResourceObject}); err != nil {
		return err
	}
	if srcKey == "" || dstKey == "" {
		return validationErr("src_key and dst_key are required")
	}
	b, prov, err := s.bucketProvider(ctx, bucketID)
	if err != nil {
		return err
	}
	if gerr := prov.CopyObject(ctx, b.GarageGlobalAlias, srcKey, dstKey); gerr != nil {
		return mapStorageErr(gerr)
	}
	s.audit(ctx, "object.copy", "bucket", bucketID, map[string]any{"src": srcKey, "dst": dstKey})
	s.fireWebhook(bucketID, EventObjectCreated, dstKey)
	return nil
}

// MoveObject renames/moves an object (copy then delete the source). Used for
// rename in the object browser.
func (s *Services) MoveObject(ctx context.Context, bucketID, srcKey, dstKey string) error {
	if err := s.authorize(ctx, authz.ActionUpdate, authz.Target{Kind: authz.ResourceObject}); err != nil {
		return err
	}
	if srcKey == "" || dstKey == "" || srcKey == dstKey {
		return validationErr("distinct src_key and dst_key are required")
	}
	b, prov, err := s.bucketProvider(ctx, bucketID)
	if err != nil {
		return err
	}
	if gerr := prov.CopyObject(ctx, b.GarageGlobalAlias, srcKey, dstKey); gerr != nil {
		return mapStorageErr(gerr)
	}
	if gerr := prov.DeleteObject(ctx, b.GarageGlobalAlias, srcKey); gerr != nil {
		return mapStorageErr(gerr)
	}
	s.audit(ctx, "object.move", "bucket", bucketID, map[string]any{"src": srcKey, "dst": dstKey})
	s.fireWebhook(bucketID, EventObjectCreated, dstKey)
	s.fireWebhook(bucketID, EventObjectDeleted, srcKey)
	return nil
}

// PutObjectStream streams an object into a bucket through the API (using the
// system key). This works behind any reverse proxy, unlike presigned URLs.
// ssecKeyB64, if non-empty, enables SSE-C (client-supplied encryption key).
func (s *Services) PutObjectStream(ctx context.Context, bucketID, key string, body io.Reader, size int64, contentType, ssecKeyB64 string) error {
	if err := s.authorize(ctx, authz.ActionCreate, authz.Target{Kind: authz.ResourceObject}); err != nil {
		return err
	}
	if key == "" {
		return validationErr("key is required")
	}
	if size > 0 {
		if err := s.quotaGuard(ctx, size); err != nil {
			return err
		}
	}
	b, prov, err := s.bucketProvider(ctx, bucketID)
	if err != nil {
		return err
	}
	ctx = storage.WithSSEC(ctx, &storage.SSECustomerKey{KeyB64: ssecKeyB64})
	if err := prov.PutObject(ctx, b.GarageGlobalAlias, key, body, size, contentType); err != nil {
		return mapStorageErr(err)
	}
	s.audit(ctx, "object.upload", "bucket", bucketID, map[string]any{"key": key, "sse_c": ssecKeyB64 != ""})
	s.emit(ctx, metering.EventObjectUploaded, bucketID, size)
	s.fireWebhook(bucketID, EventObjectCreated, key)
	return nil
}

// GetObjectStream returns a reader for an object (streamed through the API).
// ssecKeyB64, if non-empty, supplies the SSE-C key needed to decrypt.
func (s *Services) GetObjectStream(ctx context.Context, bucketID, key, ssecKeyB64 string) (io.ReadCloser, *storage.Object, error) {
	if key == "" {
		return nil, nil, validationErr("key is required")
	}
	b, prov, err := s.bucketProvider(ctx, bucketID)
	if err != nil {
		return nil, nil, err
	}
	ctx = storage.WithSSEC(ctx, &storage.SSECustomerKey{KeyB64: ssecKeyB64})
	rc, obj, err := prov.GetObject(ctx, b.GarageGlobalAlias, key)
	if err != nil {
		return nil, nil, mapStorageErr(err)
	}
	return rc, obj, nil
}

// PresignObject returns a presigned URL for a direct browser GET/PUT.
func (s *Services) PresignObject(ctx context.Context, bucketID, key, method, contentType string, expiresSeconds int) (string, error) {
	b, prov, err := s.bucketProvider(ctx, bucketID)
	if err != nil {
		return "", err
	}
	method = strings.ToUpper(method)
	if method != "GET" && method != "PUT" {
		return "", validationErr("method must be GET or PUT")
	}
	// A presigned PUT grants write capability — gate it as object-create; GET as read.
	act := authz.ActionRead
	if method == "PUT" {
		act = authz.ActionCreate
	}
	if aerr := s.authorize(ctx, act, authz.Target{Kind: authz.ResourceObject}); aerr != nil {
		return "", aerr
	}
	if key == "" {
		return "", validationErr("key is required")
	}
	expires := time.Duration(expiresSeconds) * time.Second
	if expires <= 0 {
		expires = 15 * time.Minute
	}
	url, err := prov.Presign(ctx, storage.PresignInput{
		BucketID: b.GarageGlobalAlias, Key: key, Method: method, ContentType: contentType, Expires: expires,
	})
	if err != nil {
		return "", mapStorageErr(err)
	}
	return url, nil
}
