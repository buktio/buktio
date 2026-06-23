package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// BackupSchedule is a recurring metadata-backup policy.
type BackupSchedule struct {
	ID              string
	OrgID           string
	Enabled         bool
	IntervalMinutes int
	RetentionCount  int
	OffsiteEnabled  bool
	NextRunAt       time.Time
	LastRunAt       *time.Time
}

const scheduleCols = `id::text, COALESCE(org_id::text,''), enabled, interval_minutes,
       retention_count, offsite_enabled, next_run_at, last_run_at`

func scanSchedule(row pgx.Row) (*BackupSchedule, error) {
	var s BackupSchedule
	err := row.Scan(&s.ID, &s.OrgID, &s.Enabled, &s.IntervalMinutes,
		&s.RetentionCount, &s.OffsiteEnabled, &s.NextRunAt, &s.LastRunAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// CreateBackupSchedule inserts a schedule and returns its id.
func (s *Store) CreateBackupSchedule(ctx context.Context, orgID string, intervalMin, retention int, offsite bool) (string, error) {
	var id string
	err := s.q(ctx).QueryRow(ctx,
		`INSERT INTO backup_schedules (org_id, interval_minutes, retention_count, offsite_enabled, next_run_at)
		 VALUES (NULLIF($1,'')::uuid, $2, $3, $4, now() + make_interval(mins => $2))
		 RETURNING id::text`, orgID, intervalMin, retention, offsite).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("repository: create backup schedule: %w", err)
	}
	return id, nil
}

// ListBackupSchedules returns an org's schedules.
func (s *Store) ListBackupSchedules(ctx context.Context, orgID string) ([]BackupSchedule, error) {
	rows, err := s.q(ctx).Query(ctx, `SELECT `+scheduleCols+` FROM backup_schedules WHERE org_id=$1::uuid ORDER BY created_at`, orgID)
	if err != nil {
		return nil, fmt.Errorf("repository: list backup schedules: %w", err)
	}
	defer rows.Close()
	var out []BackupSchedule
	for rows.Next() {
		sc, err := scanSchedule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *sc)
	}
	return out, rows.Err()
}

// UpdateBackupSchedule updates the mutable fields (org-scoped). Returns ErrNotFound
// when the schedule is not in the org.
func (s *Store) UpdateBackupSchedule(ctx context.Context, id, orgID string, enabled bool, intervalMin, retention int, offsite bool) error {
	tag, err := s.q(ctx).Exec(ctx,
		`UPDATE backup_schedules SET enabled=$3, interval_minutes=$4, retention_count=$5,
		    offsite_enabled=$6, updated_at=now() WHERE id=$1::uuid AND org_id=$2::uuid`,
		id, orgID, enabled, intervalMin, retention, offsite)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteBackupSchedule removes a schedule (org-scoped).
func (s *Store) DeleteBackupSchedule(ctx context.Context, id, orgID string) error {
	tag, err := s.q(ctx).Exec(ctx, `DELETE FROM backup_schedules WHERE id=$1::uuid AND org_id=$2::uuid`, id, orgID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DueBackupSchedules returns enabled schedules whose next run is in the past.
func (s *Store) DueBackupSchedules(ctx context.Context) ([]BackupSchedule, error) {
	rows, err := s.q(ctx).Query(ctx, `SELECT `+scheduleCols+` FROM backup_schedules WHERE enabled AND next_run_at <= now()`)
	if err != nil {
		return nil, fmt.Errorf("repository: due backup schedules: %w", err)
	}
	defer rows.Close()
	var out []BackupSchedule
	for rows.Next() {
		sc, err := scanSchedule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *sc)
	}
	return out, rows.Err()
}

// MarkScheduleRan advances next_run_at by the interval.
func (s *Store) MarkScheduleRan(ctx context.Context, id string, intervalMin int) error {
	_, err := s.q(ctx).Exec(ctx,
		`UPDATE backup_schedules SET last_run_at=now(), next_run_at=now() + make_interval(mins => $2) WHERE id=$1::uuid`,
		id, intervalMin)
	return err
}

// SucceededBackupPathsBeyond returns the on-disk paths of an org's succeeded jobs
// older than the newest `keep` (for org-scoped retention pruning).
func (s *Store) SucceededBackupPathsBeyond(ctx context.Context, orgID string, keep int) ([]string, error) {
	rows, err := s.q(ctx).Query(ctx,
		`SELECT path FROM backup_jobs WHERE org_id=$1::uuid AND status='succeeded' AND path <> ''
		 ORDER BY created_at DESC OFFSET $2`, orgID, keep)
	if err != nil {
		return nil, fmt.Errorf("repository: prune list: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// CreateBackupJobForSchedule inserts a pending job tied to a schedule.
func (s *Store) CreateBackupJobForSchedule(ctx context.Context, orgID, scheduleID string) (string, error) {
	var id string
	err := s.q(ctx).QueryRow(ctx,
		`INSERT INTO backup_jobs (org_id, kind, status, schedule_id)
		 VALUES (NULLIF($1,'')::uuid, 'metadata', 'pending', NULLIF($2,'')::uuid) RETURNING id::text`,
		orgID, scheduleID).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("repository: create scheduled backup job: %w", err)
	}
	return id, nil
}

// SetBackupOffsiteURI records the off-box location of a backup.
func (s *Store) SetBackupOffsiteURI(ctx context.Context, id, uri string) error {
	_, err := s.q(ctx).Exec(ctx, `UPDATE backup_jobs SET offsite_uri=$2 WHERE id=$1::uuid`, id, uri)
	return err
}
