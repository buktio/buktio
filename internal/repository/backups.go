package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// BackupJob is a record of a buktio metadata/config backup.
type BackupJob struct {
	ID         string
	OrgID      string
	Kind       string
	Status     string
	Path       string
	SizeBytes  int64
	Error      string
	StartedAt  *time.Time
	FinishedAt *time.Time
	CreatedAt  time.Time
}

const backupCols = `id::text, COALESCE(org_id::text,''), kind::text, status::text,
       path, size_bytes, error, started_at, finished_at, created_at`

func scanBackup(row pgx.Row) (*BackupJob, error) {
	var b BackupJob
	err := row.Scan(&b.ID, &b.OrgID, &b.Kind, &b.Status, &b.Path, &b.SizeBytes, &b.Error,
		&b.StartedAt, &b.FinishedAt, &b.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// CreateBackupJob inserts a pending backup job and returns its id.
func (s *Store) CreateBackupJob(ctx context.Context, orgID, kind string) (string, error) {
	var id string
	err := s.q(ctx).QueryRow(ctx,
		`INSERT INTO backup_jobs (org_id, kind, status) VALUES (NULLIF($1,'')::uuid, $2::backup_kind, 'pending')
		 RETURNING id::text`, orgID, kind).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("repository: create backup job: %w", err)
	}
	return id, nil
}

// MarkBackupRunning flips a job to running.
func (s *Store) MarkBackupRunning(ctx context.Context, id string) error {
	_, err := s.q(ctx).Exec(ctx,
		`UPDATE backup_jobs SET status='running', started_at=now() WHERE id=$1::uuid`, id)
	return err
}

// FinishBackupJob records the terminal state of a backup job.
func (s *Store) FinishBackupJob(ctx context.Context, id, status, path string, size int64, errMsg string) error {
	_, err := s.q(ctx).Exec(ctx,
		`UPDATE backup_jobs SET status=$2::backup_status, path=$3, size_bytes=$4, error=$5, finished_at=now()
		 WHERE id=$1::uuid`, id, status, path, size, errMsg)
	if err != nil {
		return fmt.Errorf("repository: finish backup job: %w", err)
	}
	return nil
}

// GetBackupJob returns one job.
func (s *Store) GetBackupJob(ctx context.Context, id string) (*BackupJob, error) {
	b, err := scanBackup(s.q(ctx).QueryRow(ctx,
		`SELECT `+backupCols+` FROM backup_jobs WHERE id=$1::uuid`, id))
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, fmt.Errorf("repository: get backup job: %w", err)
	}
	return b, err
}

// ListBackupJobs returns recent backup jobs for an org.
func (s *Store) ListBackupJobs(ctx context.Context, orgID string, limit int) ([]BackupJob, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.q(ctx).Query(ctx,
		`SELECT `+backupCols+` FROM backup_jobs WHERE org_id=$1::uuid ORDER BY created_at DESC LIMIT $2`, orgID, limit)
	if err != nil {
		return nil, fmt.Errorf("repository: list backup jobs: %w", err)
	}
	defer rows.Close()
	var out []BackupJob
	for rows.Next() {
		b, err := scanBackup(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *b)
	}
	return out, rows.Err()
}
