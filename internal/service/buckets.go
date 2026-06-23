package service

import (
	"context"

	"github.com/buktio/buktio/internal/authz"
	"github.com/buktio/buktio/internal/metering"
	"github.com/buktio/buktio/internal/repository"
	"github.com/buktio/buktio/internal/storage"
)

// CreateBucket provisions a bucket on the chosen cluster (clusterID empty = the
// primary cluster) and records it. Key-grant + quota are applied only on backends
// that support them (Garage); on generic-S3 backends the quota is recorded as a
// soft display value and there is no buktio-managed key to grant.
func (s *Services) CreateBucket(ctx context.Context, name string, quota QuotaDTO, clusterID string) (*BucketDTO, error) {
	if err := s.authorize(ctx, authz.ActionCreate, authz.Target{Kind: authz.ResourceBucket}); err != nil {
		return nil, err
	}
	if err := ValidateBucketName(name); err != nil {
		return nil, validationErr(err.Error())
	}
	if err := s.quotaGuard(ctx, 0); err != nil {
		return nil, err
	}
	cid := clusterID
	if cid == "" {
		// No explicit cluster: land on the org's default cluster (Enterprise per-org
		// cluster add-on), falling back to the primary when the org has no mapping.
		if s.Reg != nil {
			if _, ocid, oerr := s.Reg.ProviderForOrg(ctx, s.tenant(ctx).OrgID); oerr == nil && ocid != "" {
				cid = ocid
			}
		}
		if cid == "" {
			cid = s.ClusterID
		}
	}
	// Validate an explicit (non-primary) cluster id BEFORE any backend mutation so a
	// bogus id can't create a bucket on the primary and then fail the DB insert.
	if cid != s.ClusterID {
		if _, cerr := s.Store.GetClusterByID(ctx, cid); cerr != nil {
			return nil, validationErr("unknown or invalid cluster_id")
		}
	}
	prov, err := s.providerFor(ctx, cid)
	if err != nil {
		return nil, storageUnavailableErr("cannot reach the selected cluster: " + err.Error())
	}
	caps := prov.Capabilities()
	alias := name // the S3 global alias is the (validated) bucket name

	gb, err := prov.CreateBucket(ctx, alias)
	if err != nil {
		return nil, mapStorageErr(err)
	}

	// Grant the buktio-system key owner so the object browser and presigned URLs
	// (signed with that key) can operate on this bucket. Only Garage manages keys.
	if caps.ManagesKeys && s.SystemKeyID != "" {
		if err := prov.GrantKey(ctx, gb.ID, s.SystemKeyID, storage.Permissions{Read: true, Write: true, Owner: true}); err != nil {
			return nil, mapStorageErr(err)
		}
	}

	if (quota.MaxBytes != nil || quota.MaxObjects != nil) && caps.ManagesQuota {
		if err := prov.SetQuota(ctx, gb.ID, storage.Quota{MaxSizeBytes: quota.MaxBytes, MaxObjects: quota.MaxObjects}); err != nil {
			return nil, mapStorageErr(err)
		}
	}

	row := repository.Bucket{
		OrgID:             s.tenant(ctx).OrgID,
		ProjectID:         s.tenant(ctx).ProjectID,
		ClusterID:         cid,
		Name:              name,
		GarageBucketID:    gb.ID,
		GarageGlobalAlias: alias,
		Visibility:        "private",
		QuotaMaxSize:      quota.MaxBytes,
		QuotaMaxObjects:   quota.MaxObjects,
	}
	id, err := s.Store.CreateBucket(ctx, row)
	if err != nil {
		// Avoid orphaning the just-created backend bucket if the DB insert fails.
		_ = prov.DeleteBucket(ctx, gb.ID)
		return nil, mapRepoErr(err)
	}

	s.audit(ctx, "bucket.create", "bucket", id, map[string]any{"name": name, "cluster_id": cid})
	s.emit(ctx, metering.EventBucketCreated, id, 0)
	return s.GetBucket(ctx, id)
}

// ListBuckets returns the project's buckets with their latest snapshot usage.
func (s *Services) ListBuckets(ctx context.Context) ([]BucketDTO, error) {
	rows, err := s.Store.ListBuckets(ctx, s.tenant(ctx).ProjectID)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	out := make([]BucketDTO, 0, len(rows))
	for i := range rows {
		bytesUsed, objects, _ := s.Store.LatestUsageForBucket(ctx, rows[i].ID)
		out = append(out, s.bucketToDTO(&rows[i], UsageDTO{Bytes: bytesUsed, Objects: objects}))
	}
	return out, nil
}

// GetBucket returns one bucket with live usage from Garage.
func (s *Services) GetBucket(ctx context.Context, id string) (*BucketDTO, error) {
	b, serr := s.loadBucket(ctx, id)
	if serr != nil {
		return nil, serr
	}
	usage := UsageDTO{Bytes: 0, Objects: 0}
	live := false
	if prov, perr := s.providerForBucket(ctx, b); perr == nil {
		if u, uerr := prov.GetBucketUsage(ctx, b.GarageBucketID); uerr == nil {
			usage = UsageDTO{Bytes: u.BytesUsed, Objects: u.ObjectCount}
			live = true
		}
	}
	if !live {
		bytesUsed, objects, _ := s.Store.LatestUsageForBucket(ctx, b.ID)
		usage = UsageDTO{Bytes: bytesUsed, Objects: objects}
	}
	dto := s.bucketToDTO(b, usage)
	return &dto, nil
}

// DeleteBucket empties the bucket (via S3) then deletes it in Garage and records
// the soft-delete.
func (s *Services) DeleteBucket(ctx context.Context, id string) error {
	if err := s.authorize(ctx, authz.ActionDelete, authz.Target{Kind: authz.ResourceBucket, ID: id}); err != nil {
		return err
	}
	b, serr := s.loadBucket(ctx, id)
	if serr != nil {
		return serr
	}
	prov, err := s.providerForBucket(ctx, b)
	if err != nil {
		return storageUnavailableErr("cannot reach the bucket's cluster: " + err.Error())
	}
	if err := s.emptyBucket(ctx, prov, b.GarageGlobalAlias); err != nil {
		return mapStorageErr(err)
	}
	if err := prov.DeleteBucket(ctx, b.GarageBucketID); err != nil {
		return mapStorageErr(err)
	}
	if err := s.Store.SoftDeleteBucket(ctx, id); err != nil {
		return mapRepoErr(err)
	}
	s.audit(ctx, "bucket.delete", "bucket", id, map[string]any{"name": b.Name})
	s.emit(ctx, metering.EventBucketDeleted, id, 0)
	return nil
}

// emptyBucket deletes all objects in a bucket (required before DeleteBucket).
func (s *Services) emptyBucket(ctx context.Context, prov storage.StorageProvider, alias string) error {
	for i := 0; i < 1000; i++ {
		res, err := prov.ListObjects(ctx, alias, storage.ListObjectsInput{MaxKeys: 1000})
		if err != nil {
			return err
		}
		keys := make([]string, 0, len(res.Objects))
		for _, o := range res.Objects {
			if !o.IsPrefix {
				keys = append(keys, o.Key)
			}
		}
		if len(keys) == 0 {
			return nil
		}
		if err := prov.DeleteObjects(ctx, alias, keys); err != nil {
			return err
		}
		if !res.IsTruncated && len(keys) < 1000 {
			return nil
		}
	}
	return nil
}

// SetQuota updates a bucket's quota.
func (s *Services) SetQuota(ctx context.Context, id string, quota QuotaDTO) (*BucketDTO, error) {
	if err := s.authorize(ctx, authz.ActionUpdate, authz.Target{Kind: authz.ResourceBucket, ID: id}); err != nil {
		return nil, err
	}
	b, serr := s.loadBucket(ctx, id)
	if serr != nil {
		return nil, serr
	}
	// On backends that enforce quotas (Garage) apply at the backend; elsewhere the
	// quota is a soft display value stored in PG only.
	prov, err := s.providerForBucket(ctx, b)
	if err != nil {
		return nil, storageUnavailableErr("cannot reach the bucket's cluster: " + err.Error())
	}
	if prov.Capabilities().ManagesQuota {
		if err := prov.SetQuota(ctx, b.GarageBucketID, storage.Quota{MaxSizeBytes: quota.MaxBytes, MaxObjects: quota.MaxObjects}); err != nil {
			return nil, mapStorageErr(err)
		}
	}
	if err := s.Store.UpdateBucketQuota(ctx, id, quota.MaxBytes, quota.MaxObjects); err != nil {
		return nil, mapRepoErr(err)
	}
	s.audit(ctx, "bucket.set_quota", "bucket", id, nil)
	return s.GetBucket(ctx, id)
}

// SetVisibility toggles a bucket between private and public-website.
func (s *Services) SetVisibility(ctx context.Context, id string, public bool, index, errorDoc string) (*BucketDTO, error) {
	if err := s.authorize(ctx, authz.ActionUpdate, authz.Target{Kind: authz.ResourceBucket, ID: id}); err != nil {
		return nil, err
	}
	b, serr := s.loadBucket(ctx, id)
	if serr != nil {
		return nil, serr
	}
	prov, err := s.providerForBucket(ctx, b)
	if err != nil {
		return nil, storageUnavailableErr("cannot reach the bucket's cluster: " + err.Error())
	}
	if !prov.Capabilities().PublicWebsite {
		return nil, unsupportedErr()
	}
	vis := storage.VisibilityPrivate
	dbVis := "private" // matches the bucket_visibility enum
	if public {
		vis = storage.VisibilityPublic
		dbVis = "public_website"
	}
	if err := prov.SetVisibility(ctx, b.GarageBucketID, vis, storage.WebsiteConfig{
		Enabled: public, IndexDocument: index, ErrorDocument: errorDoc,
	}); err != nil {
		return nil, mapStorageErr(err)
	}
	if err := s.Store.UpdateBucketVisibility(ctx, id, dbVis, public, index, errorDoc); err != nil {
		return nil, mapRepoErr(err)
	}
	s.audit(ctx, "bucket.set_visibility", "bucket", id, map[string]any{"public": public})
	return s.GetBucket(ctx, id)
}
