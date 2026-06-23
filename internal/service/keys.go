package service

import (
	"context"
	"crypto/sha256"

	"github.com/buktio/buktio/internal/authz"
	"github.com/buktio/buktio/internal/metering"
	"github.com/buktio/buktio/internal/repository"
	"github.com/buktio/buktio/internal/storage"
)

// GrantInput is a requested permission grant at key creation / update.
type GrantInput struct {
	BucketID string
	Read     bool
	Write    bool
	Owner    bool
}

// CreateAccessKey creates a Garage key (secret shown once), records it (secret not
// stored), and applies any initial bucket grants.
func (s *Services) CreateAccessKey(ctx context.Context, name string, canCreateBucket bool, grants []GrantInput) (*KeyCreateResult, error) {
	if err := s.authorize(ctx, authz.ActionCreate, authz.Target{Kind: authz.ResourceAccessKey}); err != nil {
		return nil, err
	}
	if err := ValidateKeyName(name); err != nil {
		return nil, validationErr(err.Error())
	}

	cred, err := s.Provider.CreateAccessKey(ctx, name, canCreateBucket)
	if err != nil {
		return nil, mapStorageErr(err)
	}

	hash := sha256.Sum256([]byte(cred.SecretAccessKey))
	row := repository.AccessKey{
		OrgID:             s.tenant(ctx).OrgID,
		ProjectID:         s.tenant(ctx).ProjectID,
		ClusterID:         s.ClusterID,
		Name:              name,
		GarageAccessKeyID: cred.AccessKeyID,
		SecretLastFour:    lastFour(cred.SecretAccessKey),
		CanCreateBucket:   canCreateBucket,
	}
	id, err := s.Store.CreateAccessKey(ctx, row, hash[:])
	if err != nil {
		return nil, mapRepoErr(err)
	}
	row.ID = id

	for _, g := range grants {
		if serr := s.grant(ctx, id, cred.AccessKeyID, g); serr != nil {
			return nil, serr
		}
	}

	s.audit(ctx, "access_key.create", "access_key", id, map[string]any{"name": name})
	s.emit(ctx, metering.EventKeyCreated, id, 0)

	if k2, gerr := s.Store.GetAccessKey(ctx, id); gerr == nil {
		row.CreatedAt = k2.CreatedAt
	}
	gdtos, _ := s.keyGrants(ctx, id)
	return &KeyCreateResult{
		KeyDTO:          keyToDTO(&row, gdtos),
		SecretAccessKey: cred.SecretAccessKey,
		SecretShownOnce: true,
	}, nil
}

// ListAccessKeys returns the project's keys with their grants.
func (s *Services) ListAccessKeys(ctx context.Context) ([]KeyDTO, error) {
	rows, err := s.Store.ListAccessKeys(ctx, s.tenant(ctx).ProjectID)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	out := make([]KeyDTO, 0, len(rows))
	for i := range rows {
		gdtos, _ := s.keyGrants(ctx, rows[i].ID)
		out = append(out, keyToDTO(&rows[i], gdtos))
	}
	return out, nil
}

// GetAccessKey returns one key with its grants.
func (s *Services) GetAccessKey(ctx context.Context, id string) (*KeyDTO, error) {
	k, serr := s.loadKey(ctx, id)
	if serr != nil {
		return nil, serr
	}
	gdtos, _ := s.keyGrants(ctx, id)
	dto := keyToDTO(k, gdtos)
	return &dto, nil
}

// DeleteAccessKey fully revokes a key in Garage and soft-deletes the record.
func (s *Services) DeleteAccessKey(ctx context.Context, id string) error {
	if err := s.authorize(ctx, authz.ActionDelete, authz.Target{Kind: authz.ResourceAccessKey, ID: id}); err != nil {
		return err
	}
	k, serr := s.loadKey(ctx, id)
	if serr != nil {
		return serr
	}
	if err := s.Provider.DeleteAccessKey(ctx, k.GarageAccessKeyID); err != nil {
		return mapStorageErr(err)
	}
	if err := s.Store.SoftDeleteAccessKey(ctx, id); err != nil {
		return mapRepoErr(err)
	}
	s.audit(ctx, "access_key.delete", "access_key", id, map[string]any{"name": k.Name})
	s.emit(ctx, metering.EventKeyRevoked, id, 0)
	return nil
}

// GrantBucket grants a key permissions on a bucket.
func (s *Services) GrantBucket(ctx context.Context, keyID string, g GrantInput) error {
	if err := s.authorize(ctx, authz.ActionUpdate, authz.Target{Kind: authz.ResourceAccessKey, ID: keyID}); err != nil {
		return err
	}
	k, serr := s.loadKey(ctx, keyID)
	if serr != nil {
		return serr
	}
	if serr := s.grant(ctx, keyID, k.GarageAccessKeyID, g); serr != nil {
		return serr
	}
	s.audit(ctx, "access_key.grant", "access_key", keyID, map[string]any{"bucket_id": g.BucketID})
	return nil
}

// RevokeBucket removes a key's grant on a bucket.
func (s *Services) RevokeBucket(ctx context.Context, keyID, bucketID string) error {
	if err := s.authorize(ctx, authz.ActionUpdate, authz.Target{Kind: authz.ResourceAccessKey, ID: keyID}); err != nil {
		return err
	}
	k, serr := s.loadKey(ctx, keyID)
	if serr != nil {
		return serr
	}
	b, prov, err := s.bucketProvider(ctx, bucketID)
	if err != nil {
		return err
	}
	if !prov.Capabilities().ManagesKeys {
		return unsupportedErr()
	}
	if err := prov.RevokeKey(ctx, b.GarageBucketID, k.GarageAccessKeyID); err != nil {
		return mapStorageErr(err)
	}
	if err := s.Store.DeleteGrant(ctx, bucketID, keyID); err != nil {
		return mapRepoErr(err)
	}
	s.audit(ctx, "access_key.revoke", "access_key", keyID, map[string]any{"bucket_id": bucketID})
	return nil
}

// grant applies a single grant on the bucket's own cluster + the DB. buktio-managed
// keys exist only on key-managing backends (Garage); granting on a generic-S3
// bucket is unsupported (those backends use operator-supplied credentials).
func (s *Services) grant(ctx context.Context, keyID, garageKeyID string, g GrantInput) *Error {
	b, prov, err := s.bucketProvider(ctx, g.BucketID)
	if err != nil {
		if se, ok := err.(*Error); ok {
			return se
		}
		return mapRepoErr(err)
	}
	if !prov.Capabilities().ManagesKeys {
		return unsupportedErr()
	}
	if err := prov.GrantKey(ctx, b.GarageBucketID, garageKeyID, storage.Permissions{
		Read: g.Read, Write: g.Write, Owner: g.Owner,
	}); err != nil {
		return mapStorageErr(err)
	}
	if err := s.Store.UpsertGrant(ctx, repository.Grant{
		BucketID: g.BucketID, AccessKeyID: keyID,
		CanRead: g.Read, CanWrite: g.Write, IsOwner: g.Owner,
	}); err != nil {
		return mapRepoErr(err)
	}
	return nil
}

func (s *Services) keyGrants(ctx context.Context, keyID string) ([]GrantDTO, error) {
	grants, err := s.Store.ListGrantsForKey(ctx, keyID)
	if err != nil {
		return nil, err
	}
	out := make([]GrantDTO, 0, len(grants))
	for _, g := range grants {
		out = append(out, GrantDTO{
			BucketID: g.BucketID, BucketName: g.BucketName,
			Read: g.CanRead, Write: g.CanWrite, Owner: g.IsOwner,
		})
	}
	return out, nil
}

func keyToDTO(k *repository.AccessKey, grants []GrantDTO) KeyDTO {
	if grants == nil {
		grants = []GrantDTO{}
	}
	return KeyDTO{
		ID:              k.ID,
		Name:            k.Name,
		AccessKeyID:     k.GarageAccessKeyID,
		CanCreateBucket: k.CanCreateBucket,
		SecretLastFour:  k.SecretLastFour,
		Grants:          grants,
		CreatedAt:       k.CreatedAt,
	}
}

func lastFour(s string) string {
	if len(s) <= 4 {
		return s
	}
	return s[len(s)-4:]
}
