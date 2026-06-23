package service

import (
	"context"
	"fmt"
	"time"

	"github.com/buktio/buktio/internal/authz"
	"github.com/buktio/buktio/internal/repository"
)

// trashRetention is how long trashed objects are kept before auto-purge.
const trashRetention = 7 * 24 * time.Hour

// trashPrefix is the reserved key prefix where trashed objects live.
const trashPrefix = ".trash/"

// TrashDTO is a trashed object as the API returns it.
type TrashDTO struct {
	ID         string    `json:"id"`
	Key        string    `json:"key"`
	SizeBytes  int64     `json:"size_bytes"`
	DeletedAt  time.Time `json:"deleted_at"`
	PurgeAfter time.Time `json:"purge_after"`
}

// trashObjects moves objects to the bucket's trash prefix (Garage has no
// versioning, so trash = physical move) and records them for restore/purge.
func (s *Services) trashObjects(ctx context.Context, b *repository.Bucket, keys []string) error {
	prov, perr := s.providerForBucket(ctx, b)
	if perr != nil {
		return storageUnavailableErr("cannot reach the bucket's cluster: " + perr.Error())
	}
	now := time.Now().UTC()
	for _, key := range keys {
		var size int64
		if o, err := prov.HeadObject(ctx, b.GarageGlobalAlias, key); err == nil {
			size = o.Size
		}
		trashKey := fmt.Sprintf("%s%d/%s", trashPrefix, now.UnixNano(), key)
		if err := prov.CopyObject(ctx, b.GarageGlobalAlias, key, trashKey); err != nil {
			return mapStorageErr(err)
		}
		if err := prov.DeleteObject(ctx, b.GarageGlobalAlias, key); err != nil {
			return mapStorageErr(err)
		}
		if _, err := s.Store.InsertTrash(ctx, repository.TrashItem{
			BucketID: b.ID, OrgID: b.OrgID, OriginalKey: key, TrashKey: trashKey,
			SizeBytes: size, PurgeAfter: now.Add(trashRetention),
		}); err != nil {
			return mapRepoErr(err)
		}
	}
	s.audit(ctx, "object.trash", "bucket", b.ID, map[string]any{"count": len(keys)})
	return nil
}

// ListTrash returns a bucket's trashed (recoverable) objects.
func (s *Services) ListTrash(ctx context.Context, bucketID string) ([]TrashDTO, error) {
	items, err := s.Store.ListTrash(ctx, bucketID)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	out := make([]TrashDTO, 0, len(items))
	for _, t := range items {
		out = append(out, TrashDTO{ID: t.ID, Key: t.OriginalKey, SizeBytes: t.SizeBytes, DeletedAt: t.DeletedAt, PurgeAfter: t.PurgeAfter})
	}
	return out, nil
}

// RestoreObject moves a trashed object back to its original key.
func (s *Services) RestoreObject(ctx context.Context, bucketID, trashID string) error {
	if err := s.authorize(ctx, authz.ActionCreate, authz.Target{Kind: authz.ResourceObject}); err != nil {
		return err
	}
	t, err := s.Store.GetTrash(ctx, trashID)
	if err != nil {
		return mapRepoErr(err)
	}
	b, prov, err := s.bucketProvider(ctx, bucketID)
	if err != nil {
		return err
	}
	if err := prov.CopyObject(ctx, b.GarageGlobalAlias, t.TrashKey, t.OriginalKey); err != nil {
		return mapStorageErr(err)
	}
	if err := prov.DeleteObject(ctx, b.GarageGlobalAlias, t.TrashKey); err != nil {
		return mapStorageErr(err)
	}
	if err := s.Store.MarkRestored(ctx, trashID); err != nil {
		return mapRepoErr(err)
	}
	s.audit(ctx, "object.restore", "bucket", bucketID, map[string]any{"key": t.OriginalKey})
	return nil
}

// PurgeObject permanently deletes a trashed object.
func (s *Services) PurgeObject(ctx context.Context, bucketID, trashID string) error {
	if err := s.authorize(ctx, authz.ActionDelete, authz.Target{Kind: authz.ResourceObject}); err != nil {
		return err
	}
	t, err := s.Store.GetTrash(ctx, trashID)
	if err != nil {
		return mapRepoErr(err)
	}
	b, prov, err := s.bucketProvider(ctx, bucketID)
	if err != nil {
		return err
	}
	if err := prov.DeleteObject(ctx, b.GarageGlobalAlias, t.TrashKey); err != nil {
		return mapStorageErr(err)
	}
	if err := s.Store.DeleteTrash(ctx, trashID); err != nil {
		return mapRepoErr(err)
	}
	s.audit(ctx, "object.purge", "bucket", bucketID, map[string]any{"key": t.OriginalKey})
	return nil
}

// PurgeDueTrash hard-deletes trashed objects past their retention (run by the
// background collector).
func (s *Services) PurgeDueTrash(ctx context.Context) {
	due, err := s.Store.DueForPurge(ctx, 200)
	if err != nil {
		return
	}
	for _, t := range due {
		if b, berr := s.Store.GetBucket(ctx, t.BucketID); berr == nil {
			if prov, perr := s.providerForBucket(ctx, b); perr == nil {
				_ = prov.DeleteObject(ctx, b.GarageGlobalAlias, t.TrashKey)
			}
		}
		_ = s.Store.DeleteTrash(ctx, t.ID)
	}
}
