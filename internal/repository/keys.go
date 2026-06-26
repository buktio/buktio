package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

const keyCols = `id::text, org_id::text, COALESCE(project_id::text,''), storage_cluster_id::text,
       name, garage_access_key_id, COALESCE(secret_last_four,''), can_create_bucket, created_at`

func scanKey(row DBRow) (*AccessKey, error) {
	var k AccessKey
	err := row.Scan(&k.ID, &k.OrgID, &k.ProjectID, &k.ClusterID, &k.Name,
		&k.GarageAccessKeyID, &k.SecretLastFour, &k.CanCreateBucket, &k.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &k, nil
}

// CreateAccessKey inserts a key row (secret material is NOT stored — only a hash
// + last four). Returns the new id.
func (s *Store) CreateAccessKey(ctx context.Context, k AccessKey, secretHash []byte) (string, error) {
	const q = `
INSERT INTO access_keys
  (org_id, project_id, storage_cluster_id, name, garage_access_key_id,
   secret_hash, secret_last_four, can_create_bucket, secret_revealed_at)
VALUES ($1::uuid, NULLIF($2,'')::uuid, $3::uuid, $4, $5, $6, NULLIF($7,''), $8, now())
RETURNING id::text`
	var id string
	err := s.q(ctx).QueryRow(ctx, q,
		k.OrgID, k.ProjectID, k.ClusterID, k.Name, k.GarageAccessKeyID,
		secretHash, k.SecretLastFour, k.CanCreateBucket,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("repository: create access key: %w", err)
	}
	return id, nil
}

// ListAccessKeys returns live keys for a project.
func (s *Store) ListAccessKeys(ctx context.Context, projectID string) ([]AccessKey, error) {
	q := `SELECT ` + keyCols + ` FROM access_keys
WHERE project_id=$1::uuid AND deleted_at IS NULL ORDER BY created_at DESC`
	rows, err := s.q(ctx).Query(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("repository: list keys: %w", err)
	}
	defer rows.Close()
	var out []AccessKey
	for rows.Next() {
		k, err := scanKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *k)
	}
	return out, rows.Err()
}

// GetAccessKey returns a live key by id.
func (s *Store) GetAccessKey(ctx context.Context, id string) (*AccessKey, error) {
	q := `SELECT ` + keyCols + ` FROM access_keys WHERE id=$1::uuid AND deleted_at IS NULL`
	return scanKey(s.q(ctx).QueryRow(ctx, q, id))
}

// SoftDeleteAccessKey marks a key revoked/deleted.
func (s *Store) SoftDeleteAccessKey(ctx context.Context, id string) error {
	_, err := s.q(ctx).Exec(ctx,
		`UPDATE access_keys SET deleted_at=now() WHERE id=$1::uuid AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("repository: soft delete key: %w", err)
	}
	return nil
}

// CountAccessKeys returns the number of live keys for a project.
func (s *Store) CountAccessKeys(ctx context.Context, projectID string) (int, error) {
	var n int
	err := s.q(ctx).QueryRow(ctx,
		`SELECT count(*) FROM access_keys WHERE project_id=$1::uuid AND deleted_at IS NULL`, projectID).Scan(&n)
	return n, err
}

// --- bucket_permissions (grants) ---

// UpsertGrant records a key's permissions on a bucket (mirrors AllowBucketKey).
func (s *Store) UpsertGrant(ctx context.Context, g Grant) error {
	const q = `
INSERT INTO bucket_permissions (bucket_id, access_key_id, can_read, can_write, is_owner)
VALUES ($1::uuid, $2::uuid, $3, $4, $5)
ON CONFLICT (bucket_id, access_key_id) WHERE deleted_at IS NULL
DO UPDATE SET can_read=EXCLUDED.can_read, can_write=EXCLUDED.can_write, is_owner=EXCLUDED.is_owner`
	_, err := s.q(ctx).Exec(ctx, q, g.BucketID, g.AccessKeyID, g.CanRead, g.CanWrite, g.IsOwner)
	if err != nil {
		return fmt.Errorf("repository: upsert grant: %w", err)
	}
	return nil
}

// DeleteGrant removes a key's grant on a bucket.
func (s *Store) DeleteGrant(ctx context.Context, bucketID, accessKeyID string) error {
	_, err := s.q(ctx).Exec(ctx,
		`UPDATE bucket_permissions SET deleted_at=now()
		 WHERE bucket_id=$1::uuid AND access_key_id=$2::uuid AND deleted_at IS NULL`,
		bucketID, accessKeyID)
	if err != nil {
		return fmt.Errorf("repository: delete grant: %w", err)
	}
	return nil
}

// ListGrantsForKey returns the buckets a key is granted on, with bucket names.
func (s *Store) ListGrantsForKey(ctx context.Context, accessKeyID string) ([]Grant, error) {
	const q = `
SELECT bp.bucket_id::text, bp.access_key_id::text, b.name, b.garage_bucket_id,
       bp.can_read, bp.can_write, bp.is_owner
FROM bucket_permissions bp
JOIN buckets b ON b.id = bp.bucket_id AND b.deleted_at IS NULL
WHERE bp.access_key_id=$1::uuid AND bp.deleted_at IS NULL`
	rows, err := s.q(ctx).Query(ctx, q, accessKeyID)
	if err != nil {
		return nil, fmt.Errorf("repository: list grants: %w", err)
	}
	defer rows.Close()
	var out []Grant
	for rows.Next() {
		var g Grant
		if err := rows.Scan(&g.BucketID, &g.AccessKeyID, &g.BucketName, &g.GarageBucket,
			&g.CanRead, &g.CanWrite, &g.IsOwner); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}
