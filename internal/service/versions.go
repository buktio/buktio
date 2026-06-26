package service

import (
	"context"
	"time"

	"github.com/buktio/buktio/internal/authz"
)

// ObjectVersionDTO is one version (or delete marker) of an object.
type ObjectVersionDTO struct {
	Key            string    `json:"key"`
	VersionID      string    `json:"version_id"`
	IsLatest       bool      `json:"is_latest"`
	IsDeleteMarker bool      `json:"is_delete_marker"`
	SizeBytes      int64     `json:"size_bytes"`
	LastModified   time.Time `json:"last_modified"`
	ETag           string    `json:"etag"`
}

// GetBucketVersioning reports whether versioning is enabled on the bucket's backend.
func (s *Services) GetBucketVersioning(ctx context.Context, bucketID string) (bool, error) {
	b, prov, err := s.bucketProvider(ctx, bucketID)
	if err != nil {
		return false, err
	}
	on, gerr := prov.GetBucketVersioning(ctx, b.GarageGlobalAlias)
	if gerr != nil {
		return false, mapStorageErr(gerr)
	}
	return on, nil
}

// SetBucketVersioning enables or suspends versioning on the bucket's backend.
func (s *Services) SetBucketVersioning(ctx context.Context, bucketID string, enabled bool) error {
	if err := s.authorize(ctx, authz.ActionUpdate, authz.Target{Kind: authz.ResourceBucket}); err != nil {
		return err
	}
	b, prov, err := s.bucketProvider(ctx, bucketID)
	if err != nil {
		return err
	}
	if gerr := prov.SetBucketVersioning(ctx, b.GarageGlobalAlias, enabled); gerr != nil {
		return mapStorageErr(gerr)
	}
	s.audit(ctx, "bucket.versioning", "bucket", bucketID, map[string]any{"enabled": enabled})
	return nil
}

// ListObjectVersions lists object versions under an optional prefix.
func (s *Services) ListObjectVersions(ctx context.Context, bucketID, prefix string) ([]ObjectVersionDTO, error) {
	b, prov, err := s.bucketProvider(ctx, bucketID)
	if err != nil {
		return nil, err
	}
	vs, gerr := prov.ListObjectVersions(ctx, b.GarageGlobalAlias, prefix)
	if gerr != nil {
		return nil, mapStorageErr(gerr)
	}
	out := make([]ObjectVersionDTO, 0, len(vs))
	for _, v := range vs {
		out = append(out, ObjectVersionDTO{
			Key: v.Key, VersionID: v.VersionID, IsLatest: v.IsLatest,
			IsDeleteMarker: v.IsDeleteMarker, SizeBytes: v.Size,
			LastModified: v.LastModified, ETag: v.ETag,
		})
	}
	return out, nil
}

// DeleteObjectVersion permanently removes a specific version.
func (s *Services) DeleteObjectVersion(ctx context.Context, bucketID, key, versionID string) error {
	if err := s.authorize(ctx, authz.ActionDelete, authz.Target{Kind: authz.ResourceObject}); err != nil {
		return err
	}
	if key == "" || versionID == "" {
		return validationErr("key and version_id are required")
	}
	b, prov, err := s.bucketProvider(ctx, bucketID)
	if err != nil {
		return err
	}
	if gerr := prov.DeleteObjectVersion(ctx, b.GarageGlobalAlias, key, versionID); gerr != nil {
		return mapStorageErr(gerr)
	}
	s.audit(ctx, "object.version.delete", "bucket", bucketID, map[string]any{"key": key, "version": versionID})
	return nil
}

// RestoreObjectVersion makes a prior version the current one.
func (s *Services) RestoreObjectVersion(ctx context.Context, bucketID, key, versionID string) error {
	if err := s.authorize(ctx, authz.ActionUpdate, authz.Target{Kind: authz.ResourceObject}); err != nil {
		return err
	}
	if key == "" || versionID == "" {
		return validationErr("key and version_id are required")
	}
	b, prov, err := s.bucketProvider(ctx, bucketID)
	if err != nil {
		return err
	}
	if gerr := prov.RestoreObjectVersion(ctx, b.GarageGlobalAlias, key, versionID); gerr != nil {
		return mapStorageErr(gerr)
	}
	s.audit(ctx, "object.version.restore", "bucket", bucketID, map[string]any{"key": key, "version": versionID})
	s.fireWebhook(bucketID, EventObjectCreated, key)
	return nil
}
