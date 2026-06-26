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

// StoragePoint is one interval of the storage-growth series.
type StoragePoint struct {
	TS          time.Time
	BytesUsed   int64
	ObjectCount int64
}

// StorageSeries returns project storage totals bucketed into stepSeconds-wide
// intervals since `since`. Within each interval it takes the latest snapshot per
// bucket, then sums across buckets — so the series tracks total stored bytes/objects
// over time. Epoch bucketing differs by backend (Postgres extract(epoch)/to_timestamp
// vs SQLite strftime/datetime), so the query is selected by driver; the row_number()
// window and the rest are identical.
func (s *Store) StorageSeries(ctx context.Context, projectID string, since time.Time, stepSeconds int) ([]StoragePoint, error) {
	q := `
WITH bucketed AS (
  SELECT
    to_timestamp(floor(extract(epoch from captured_at) / $2) * $2) AS ts,
    bucket_id, bytes_used, object_count,
    row_number() OVER (
      PARTITION BY to_timestamp(floor(extract(epoch from captured_at) / $2) * $2), bucket_id
      ORDER BY captured_at DESC
    ) AS rn
  FROM usage_snapshots
  WHERE project_id=$1::uuid AND captured_at >= $3
)
SELECT ts, COALESCE(sum(bytes_used),0), COALESCE(sum(object_count),0)
FROM bucketed WHERE rn=1
GROUP BY ts ORDER BY ts`
	args := []any{projectID, stepSeconds, since}
	sqliteMode := s.Driver() == "sqlite"
	if sqliteMode {
		// captured_at is DATETIME TEXT (UTC); strftime('%s') yields the epoch. ts is
		// returned as the bucket epoch (INTEGER) — a computed column has no declared
		// type, so modernc cannot map it to time.Time; we convert in Go below. Bind
		// `since` as a matching UTC string so the comparison is correct.
		q = `
WITH bucketed AS (
  SELECT
    (CAST(strftime('%s', captured_at) AS INTEGER) / $2) * $2 AS ts,
    bucket_id, bytes_used, object_count,
    row_number() OVER (
      PARTITION BY (CAST(strftime('%s', captured_at) AS INTEGER) / $2) * $2, bucket_id
      ORDER BY captured_at DESC
    ) AS rn
  FROM usage_snapshots
  WHERE project_id=$1 AND captured_at >= $3
)
SELECT ts, COALESCE(sum(bytes_used),0), COALESCE(sum(object_count),0)
FROM bucketed WHERE rn=1
GROUP BY ts ORDER BY ts`
		args = []any{projectID, stepSeconds, since.UTC().Format("2006-01-02 15:04:05")}
	}
	rows, err := s.q(ctx).Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("repository: storage series: %w", err)
	}
	defer rows.Close()
	var out []StoragePoint
	for rows.Next() {
		var p StoragePoint
		if sqliteMode {
			var epoch int64
			if err := rows.Scan(&epoch, &p.BytesUsed, &p.ObjectCount); err != nil {
				return nil, fmt.Errorf("repository: storage series scan: %w", err)
			}
			p.TS = time.Unix(epoch, 0).UTC()
		} else if err := rows.Scan(&p.TS, &p.BytesUsed, &p.ObjectCount); err != nil {
			return nil, fmt.Errorf("repository: storage series scan: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// BucketUsageRow is the latest usage + quota for one bucket.
type BucketUsageRow struct {
	BucketID     string
	Name         string
	BytesUsed    int64
	ObjectCount  int64
	QuotaMaxSize *int64
}

// BucketUsageList returns the latest snapshot per live bucket in a project.
func (s *Store) BucketUsageList(ctx context.Context, projectID string) ([]BucketUsageRow, error) {
	// Latest snapshot per bucket via row_number() (portable; DISTINCT ON is PG-only).
	const q = `
SELECT s.bucket_id, b.name, s.bytes_used, s.object_count, s.quota_max_size
FROM (
  SELECT bucket_id, bytes_used, object_count, quota_max_size,
         row_number() OVER (PARTITION BY bucket_id ORDER BY captured_at DESC) AS rn
  FROM usage_snapshots WHERE project_id=$1::uuid
) s
JOIN buckets b ON b.id = s.bucket_id
WHERE s.rn=1 AND b.deleted_at IS NULL`
	rows, err := s.q(ctx).Query(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("repository: bucket usage list: %w", err)
	}
	defer rows.Close()
	var out []BucketUsageRow
	for rows.Next() {
		var r BucketUsageRow
		if err := rows.Scan(&r.BucketID, &r.Name, &r.BytesUsed, &r.ObjectCount, &r.QuotaMaxSize); err != nil {
			return nil, fmt.Errorf("repository: bucket usage scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ProjectUsageTotals sums the latest-per-bucket usage across a project.
func (s *Store) ProjectUsageTotals(ctx context.Context, projectID string) (bytesUsed, objectCount int64, err error) {
	const q = `
WITH latest AS (
  SELECT bytes_used, object_count,
         row_number() OVER (PARTITION BY bucket_id ORDER BY captured_at DESC) AS rn
  FROM usage_snapshots
  WHERE project_id=$1::uuid
)
SELECT COALESCE(sum(bytes_used),0), COALESCE(sum(object_count),0) FROM latest WHERE rn=1`
	err = s.q(ctx).QueryRow(ctx, q, projectID).Scan(&bytesUsed, &objectCount)
	if err != nil {
		return 0, 0, nil
	}
	return bytesUsed, objectCount, nil
}
