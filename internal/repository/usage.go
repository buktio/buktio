package repository

import (
	"context"
	"fmt"
	"time"
)

// UsageSnapshot is a point-in-time per-bucket usage record (from GetBucketInfo).
type UsageSnapshot struct {
	BucketID                   string
	ProjectID                  string
	OrgID                      string
	ClusterID                  string
	BytesUsed                  int64
	ObjectCount                int64
	UnfinishedUploads          int64
	UnfinishedMultipartUploads int64
	QuotaMaxSize               *int64
	QuotaMaxObjects            *int64
	CapturedAt                 time.Time
}

// InsertUsageSnapshot appends a usage snapshot.
func (s *Store) InsertUsageSnapshot(ctx context.Context, u UsageSnapshot) error {
	const q = `
INSERT INTO usage_snapshots
  (bucket_id, project_id, org_id, storage_cluster_id, bytes_used, object_count,
   unfinished_uploads, unfinished_multipart_uploads, quota_max_size, quota_max_objects)
VALUES ($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5,$6,$7,$8,$9,$10)`
	_, err := s.q(ctx).Exec(ctx, q,
		u.BucketID, u.ProjectID, u.OrgID, u.ClusterID, u.BytesUsed, u.ObjectCount,
		u.UnfinishedUploads, u.UnfinishedMultipartUploads, u.QuotaMaxSize, u.QuotaMaxObjects)
	if err != nil {
		return fmt.Errorf("repository: insert usage snapshot: %w", err)
	}
	return nil
}

// LatestUsageForBucket returns the most recent snapshot for a bucket (zero values
// if none yet).
func (s *Store) LatestUsageForBucket(ctx context.Context, bucketID string) (bytesUsed, objectCount int64, err error) {
	err = s.q(ctx).QueryRow(ctx,
		`SELECT bytes_used, object_count FROM usage_snapshots
		 WHERE bucket_id=$1::uuid ORDER BY captured_at DESC LIMIT 1`, bucketID).
		Scan(&bytesUsed, &objectCount)
	if err != nil {
		// No snapshot yet is not an error for the dashboard.
		return 0, 0, nil
	}
	return bytesUsed, objectCount, nil
}

// ProjectUsageTotals sums the latest-per-bucket usage across a project.
func (s *Store) ProjectUsageTotals(ctx context.Context, projectID string) (bytesUsed, objectCount int64, err error) {
	const q = `
WITH latest AS (
  SELECT DISTINCT ON (bucket_id) bucket_id, bytes_used, object_count
  FROM usage_snapshots
  WHERE project_id=$1::uuid
  ORDER BY bucket_id, captured_at DESC
)
SELECT COALESCE(sum(bytes_used),0), COALESCE(sum(object_count),0) FROM latest`
	err = s.q(ctx).QueryRow(ctx, q, projectID).Scan(&bytesUsed, &objectCount)
	if err != nil {
		return 0, 0, nil
	}
	return bytesUsed, objectCount, nil
}
