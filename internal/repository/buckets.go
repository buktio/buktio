package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

const bucketCols = `id::text, org_id::text, project_id::text, storage_cluster_id::text,
       name, garage_bucket_id, garage_global_alias, visibility::text,
       website_enabled, website_index_doc, COALESCE(website_error_doc,''),
       quota_max_size, quota_max_objects, created_at`

func scanBucket(row pgx.Row) (*Bucket, error) {
	var b Bucket
	err := row.Scan(&b.ID, &b.OrgID, &b.ProjectID, &b.ClusterID, &b.Name,
		&b.GarageBucketID, &b.GarageGlobalAlias, &b.Visibility,
		&b.WebsiteEnabled, &b.WebsiteIndexDoc, &b.WebsiteErrorDoc,
		&b.QuotaMaxSize, &b.QuotaMaxObjects, &b.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// CreateBucket inserts a bucket row and returns its id.
func (s *Store) CreateBucket(ctx context.Context, b Bucket) (string, error) {
	const q = `
INSERT INTO buckets
  (org_id, project_id, storage_cluster_id, name, garage_bucket_id, garage_global_alias,
   visibility, website_enabled, website_index_doc, quota_max_size, quota_max_objects)
VALUES ($1::uuid,$2::uuid,$3::uuid,$4,$5,$6,$7::bucket_visibility,$8,$9,$10,$11)
RETURNING id::text`
	if b.Visibility == "" {
		b.Visibility = "private"
	}
	if b.WebsiteIndexDoc == "" {
		b.WebsiteIndexDoc = "index.html"
	}
	var id string
	err := s.q(ctx).QueryRow(ctx, q,
		b.OrgID, b.ProjectID, b.ClusterID, b.Name, b.GarageBucketID, b.GarageGlobalAlias,
		b.Visibility, b.WebsiteEnabled, b.WebsiteIndexDoc, b.QuotaMaxSize, b.QuotaMaxObjects,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("repository: create bucket: %w", err)
	}
	return id, nil
}

// ListBuckets returns live buckets for a project.
func (s *Store) ListBuckets(ctx context.Context, projectID string) ([]Bucket, error) {
	q := `SELECT ` + bucketCols + ` FROM buckets
WHERE project_id=$1::uuid AND deleted_at IS NULL ORDER BY created_at DESC`
	rows, err := s.q(ctx).Query(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("repository: list buckets: %w", err)
	}
	defer rows.Close()
	var out []Bucket
	for rows.Next() {
		b, err := scanBucket(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *b)
	}
	return out, rows.Err()
}

// GetBucket returns a live bucket by id.
func (s *Store) GetBucket(ctx context.Context, id string) (*Bucket, error) {
	q := `SELECT ` + bucketCols + ` FROM buckets WHERE id=$1::uuid AND deleted_at IS NULL`
	return scanBucket(s.q(ctx).QueryRow(ctx, q, id))
}

// UpdateBucketQuota sets a bucket's quota (nil = unlimited).
func (s *Store) UpdateBucketQuota(ctx context.Context, id string, maxSize, maxObjects *int64) error {
	_, err := s.q(ctx).Exec(ctx,
		`UPDATE buckets SET quota_max_size=$2, quota_max_objects=$3 WHERE id=$1::uuid AND deleted_at IS NULL`,
		id, maxSize, maxObjects)
	if err != nil {
		return fmt.Errorf("repository: update quota: %w", err)
	}
	return nil
}

// UpdateBucketVisibility sets a bucket's visibility + website config.
func (s *Store) UpdateBucketVisibility(ctx context.Context, id, visibility string, websiteEnabled bool, index, errorDoc string) error {
	if index == "" {
		index = "index.html"
	}
	_, err := s.q(ctx).Exec(ctx,
		`UPDATE buckets SET visibility=$2::bucket_visibility, website_enabled=$3,
		    website_index_doc=$4, website_error_doc=NULLIF($5,'')
		 WHERE id=$1::uuid AND deleted_at IS NULL`,
		id, visibility, websiteEnabled, index, errorDoc)
	if err != nil {
		return fmt.Errorf("repository: update visibility: %w", err)
	}
	return nil
}

// SoftDeleteBucket marks a bucket deleted.
func (s *Store) SoftDeleteBucket(ctx context.Context, id string) error {
	_, err := s.q(ctx).Exec(ctx,
		`UPDATE buckets SET deleted_at=now() WHERE id=$1::uuid AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("repository: soft delete bucket: %w", err)
	}
	return nil
}

// CountBuckets returns the number of live buckets for a project.
func (s *Store) CountBuckets(ctx context.Context, projectID string) (int, error) {
	var n int
	err := s.q(ctx).QueryRow(ctx,
		`SELECT count(*) FROM buckets WHERE project_id=$1::uuid AND deleted_at IS NULL`, projectID).Scan(&n)
	return n, err
}
