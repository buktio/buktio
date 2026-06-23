package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// MigrationJob is an S3-to-S3 import job.
type MigrationJob struct {
	ID              string
	OrgID           string
	SourceEndpoint  string
	SourceRegion    string
	SourceBucket    string
	SourceAccessKey string
	SourceSecretEnc []byte
	DestBucketID    string
	Status          string
	CopiedObjects   int64
	CopiedBytes     int64
	Cursor          string
	Error           string
	CreatedAt       time.Time
}

// CreateMigrationJob inserts a pending job and returns its id.
func (s *Store) CreateMigrationJob(ctx context.Context, j MigrationJob) (string, error) {
	var id string
	err := s.q(ctx).QueryRow(ctx,
		`INSERT INTO migration_jobs
		   (org_id, source_endpoint, source_region, source_bucket, source_access_key, source_secret_enc, dest_bucket_id)
		 VALUES ($1::uuid,$2,$3,$4,$5,$6,$7::uuid) RETURNING id::text`,
		j.OrgID, j.SourceEndpoint, j.SourceRegion, j.SourceBucket, j.SourceAccessKey, j.SourceSecretEnc, j.DestBucketID).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("repository: create migration job: %w", err)
	}
	return id, nil
}

const migrationCols = `id::text, org_id::text, source_endpoint, source_region, source_bucket,
	source_access_key, source_secret_enc, dest_bucket_id::text, status, copied_objects, copied_bytes,
	cursor, COALESCE(error,''), created_at`

func scanMigrationJob(row pgx.Row) (*MigrationJob, error) {
	var j MigrationJob
	err := row.Scan(&j.ID, &j.OrgID, &j.SourceEndpoint, &j.SourceRegion, &j.SourceBucket,
		&j.SourceAccessKey, &j.SourceSecretEnc, &j.DestBucketID, &j.Status, &j.CopiedObjects, &j.CopiedBytes,
		&j.Cursor, &j.Error, &j.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("repository: get migration job: %w", err)
	}
	return &j, nil
}

// GetMigrationJob fetches a job, scoped to an org.
func (s *Store) GetMigrationJob(ctx context.Context, orgID, id string) (*MigrationJob, error) {
	return scanMigrationJob(s.q(ctx).QueryRow(ctx,
		`SELECT `+migrationCols+` FROM migration_jobs WHERE id=$1::uuid AND org_id=$2::uuid`, id, orgID))
}

// GetMigrationJobByID fetches a job without org scoping (background worker).
func (s *Store) GetMigrationJobByID(ctx context.Context, id string) (*MigrationJob, error) {
	return scanMigrationJob(s.q(ctx).QueryRow(ctx,
		`SELECT `+migrationCols+` FROM migration_jobs WHERE id=$1::uuid`, id))
}

// ListMigrationJobs returns an org's jobs (newest first).
func (s *Store) ListMigrationJobs(ctx context.Context, orgID string) ([]MigrationJob, error) {
	rows, err := s.q(ctx).Query(ctx,
		`SELECT `+migrationCols+` FROM migration_jobs WHERE org_id=$1::uuid ORDER BY created_at DESC LIMIT 100`, orgID)
	if err != nil {
		return nil, fmt.Errorf("repository: list migration jobs: %w", err)
	}
	defer rows.Close()
	var out []MigrationJob
	for rows.Next() {
		j, err := scanMigrationJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *j)
	}
	return out, rows.Err()
}

// ListResumableMigrationJobIDs returns ids of jobs to (re)start (boot sweep).
func (s *Store) ListResumableMigrationJobIDs(ctx context.Context) ([]string, error) {
	rows, err := s.q(ctx).Query(ctx, `SELECT id::text FROM migration_jobs WHERE status IN ('pending','running')`)
	if err != nil {
		return nil, fmt.Errorf("repository: list resumable migrations: %w", err)
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

// UpdateMigrationProgress saves the cursor + cumulative counters.
func (s *Store) UpdateMigrationProgress(ctx context.Context, id, cursor string, copiedObjects, copiedBytes int64) error {
	_, err := s.q(ctx).Exec(ctx,
		`UPDATE migration_jobs SET cursor=$2, copied_objects=$3, copied_bytes=$4, status='running', updated_at=now() WHERE id=$1::uuid`,
		id, cursor, copiedObjects, copiedBytes)
	return err
}

// SetMigrationStatus sets the terminal/intermediate status (+ optional error).
func (s *Store) SetMigrationStatus(ctx context.Context, id, status, errMsg string) error {
	_, err := s.q(ctx).Exec(ctx,
		`UPDATE migration_jobs SET status=$2, error=NULLIF($3,''), updated_at=now() WHERE id=$1::uuid`, id, status, errMsg)
	return err
}
