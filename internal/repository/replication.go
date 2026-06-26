package repository

import (
	"context"
	"fmt"
	"time"
)

// ReplicationJob is a one-shot bucket→bucket replication (possibly cross-backend).
type ReplicationJob struct {
	ID             string
	OrgID          string
	SrcBucketID    string
	DstBucketID    string
	Status         string
	CopiedObjects  int64
	SkippedObjects int64
	CopiedBytes    int64
	Cursor         string
	Error          string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

const replicationCols = `id::text, org_id::text, src_bucket_id::text, dst_bucket_id::text, status,
  copied_objects, skipped_objects, copied_bytes, cursor, error, created_at, updated_at`

// rowScanner is satisfied by both pgx.Row and pgx.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanReplication(row rowScanner) (ReplicationJob, error) {
	var j ReplicationJob
	err := row.Scan(&j.ID, &j.OrgID, &j.SrcBucketID, &j.DstBucketID, &j.Status,
		&j.CopiedObjects, &j.SkippedObjects, &j.CopiedBytes, &j.Cursor, &j.Error, &j.CreatedAt, &j.UpdatedAt)
	return j, err
}

// CreateReplicationJob inserts a pending job and returns its id.
func (s *Store) CreateReplicationJob(ctx context.Context, orgID, srcBucketID, dstBucketID string) (string, error) {
	var id string
	err := s.q(ctx).QueryRow(ctx,
		`INSERT INTO replication_jobs (org_id, src_bucket_id, dst_bucket_id, status)
		 VALUES ($1::uuid,$2::uuid,$3::uuid,'pending') RETURNING id::text`,
		orgID, srcBucketID, dstBucketID).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("repository: create replication job: %w", err)
	}
	return id, nil
}

// GetReplicationJobByID loads a job without org scope (for the detached worker).
func (s *Store) GetReplicationJobByID(ctx context.Context, id string) (*ReplicationJob, error) {
	j, err := scanReplication(s.q(ctx).QueryRow(ctx,
		`SELECT `+replicationCols+` FROM replication_jobs WHERE id=$1::uuid`, id))
	if err != nil {
		return nil, ErrNotFound
	}
	return &j, nil
}

// ListReplicationJobsBySrc returns a bucket's replication jobs, newest first.
func (s *Store) ListReplicationJobsBySrc(ctx context.Context, srcBucketID string) ([]ReplicationJob, error) {
	rows, err := s.q(ctx).Query(ctx,
		`SELECT `+replicationCols+` FROM replication_jobs WHERE src_bucket_id=$1::uuid ORDER BY created_at DESC LIMIT 50`, srcBucketID)
	if err != nil {
		return nil, fmt.Errorf("repository: list replication jobs: %w", err)
	}
	defer rows.Close()
	var out []ReplicationJob
	for rows.Next() {
		j, err := scanReplication(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// ListResumableReplicationJobIDs returns jobs interrupted by a restart.
func (s *Store) ListResumableReplicationJobIDs(ctx context.Context) ([]string, error) {
	rows, err := s.q(ctx).Query(ctx, `SELECT id::text FROM replication_jobs WHERE status IN ('pending','running')`)
	if err != nil {
		return nil, fmt.Errorf("repository: list resumable replications: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// UpdateReplicationProgress persists the cursor and counters mid-run.
func (s *Store) UpdateReplicationProgress(ctx context.Context, id, cursor string, copied, skipped, copiedBytes int64) error {
	_, err := s.q(ctx).Exec(ctx,
		`UPDATE replication_jobs SET cursor=$2, copied_objects=$3, skipped_objects=$4, copied_bytes=$5,
		 status='running', updated_at=now() WHERE id=$1::uuid`,
		id, cursor, copied, skipped, copiedBytes)
	return err
}

// SetReplicationStatus updates the terminal/transition status.
func (s *Store) SetReplicationStatus(ctx context.Context, id, status, errMsg string) error {
	_, err := s.q(ctx).Exec(ctx,
		`UPDATE replication_jobs SET status=$2, error=$3, updated_at=now() WHERE id=$1::uuid`, id, status, errMsg)
	return err
}
