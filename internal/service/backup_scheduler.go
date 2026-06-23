package service

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/buktio/buktio/internal/authz"
	"github.com/buktio/buktio/internal/entitlements"
	"github.com/buktio/buktio/internal/repository"
	"github.com/buktio/buktio/internal/storage/s3core"
)

// BackupOffsiteConfig points scheduled backups at an off-box S3 target (optional).
type BackupOffsiteConfig struct {
	Endpoint, Region, Bucket, AccessKey, Secret string
}

func (c BackupOffsiteConfig) enabled() bool {
	return c.Bucket != "" && c.AccessKey != "" && c.Secret != ""
}

// ScheduleDTO is a backup schedule as the API returns it.
type ScheduleDTO struct {
	ID              string     `json:"id"`
	Enabled         bool       `json:"enabled"`
	IntervalMinutes int        `json:"interval_minutes"`
	RetentionCount  int        `json:"retention_count"`
	OffsiteEnabled  bool       `json:"offsite_enabled"`
	NextRunAt       time.Time  `json:"next_run_at"`
	LastRunAt       *time.Time `json:"last_run_at,omitempty"`
}

// CreateBackupSchedule creates a recurring backup policy (Pro: scheduled_backups).
func (s *Services) CreateBackupSchedule(ctx context.Context, intervalMin, retention int, offsite bool) (*ScheduleDTO, error) {
	if err := s.authorize(ctx, authz.ActionCreate, authz.Target{Kind: authz.ResourceBackup}); err != nil {
		return nil, err
	}
	t := s.tenant(ctx)
	if d := s.Ent.Allowed(entitlements.FeatureScheduledBackups, entitlements.TenantID(t.OrgID)); !d.Allowed {
		return nil, &Error{Code: "not_entitled", Message: "scheduled backups are not in your plan", HTTP: http.StatusPaymentRequired}
	}
	if intervalMin < 5 {
		intervalMin = 5
	}
	if retention < 1 {
		retention = 7
	}
	id, err := s.Store.CreateBackupSchedule(ctx, t.OrgID, intervalMin, retention, offsite)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	s.audit(ctx, "backup.schedule.create", "backup_schedule", id, nil)
	return s.scheduleByID(ctx, id)
}

// ListBackupSchedules returns the org's schedules.
func (s *Services) ListBackupSchedules(ctx context.Context) ([]ScheduleDTO, error) {
	if err := s.authorize(ctx, authz.ActionRead, authz.Target{Kind: authz.ResourceBackup}); err != nil {
		return nil, err
	}
	rows, err := s.Store.ListBackupSchedules(ctx, s.tenant(ctx).OrgID)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	out := make([]ScheduleDTO, 0, len(rows))
	for i := range rows {
		out = append(out, scheduleToDTO(&rows[i]))
	}
	return out, nil
}

// UpdateBackupSchedule updates a schedule's mutable fields.
func (s *Services) UpdateBackupSchedule(ctx context.Context, id string, enabled bool, intervalMin, retention int, offsite bool) error {
	if err := s.authorize(ctx, authz.ActionUpdate, authz.Target{Kind: authz.ResourceBackup, ID: id}); err != nil {
		return err
	}
	if intervalMin < 5 {
		intervalMin = 5
	}
	if err := s.Store.UpdateBackupSchedule(ctx, id, s.tenant(ctx).OrgID, enabled, intervalMin, retention, offsite); err != nil {
		return mapRepoErr(err)
	}
	s.audit(ctx, "backup.schedule.update", "backup_schedule", id, nil)
	return nil
}

// DeleteBackupSchedule removes a schedule.
func (s *Services) DeleteBackupSchedule(ctx context.Context, id string) error {
	if err := s.authorize(ctx, authz.ActionDelete, authz.Target{Kind: authz.ResourceBackup, ID: id}); err != nil {
		return err
	}
	if err := s.Store.DeleteBackupSchedule(ctx, id, s.tenant(ctx).OrgID); err != nil {
		return mapRepoErr(err)
	}
	s.audit(ctx, "backup.schedule.delete", "backup_schedule", id, nil)
	return nil
}

func (s *Services) scheduleByID(ctx context.Context, id string) (*ScheduleDTO, error) {
	rows, err := s.Store.ListBackupSchedules(ctx, s.tenant(ctx).OrgID)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	for i := range rows {
		if rows[i].ID == id {
			d := scheduleToDTO(&rows[i])
			return &d, nil
		}
	}
	return nil, notFoundErr()
}

func scheduleToDTO(sc *repository.BackupSchedule) ScheduleDTO {
	return ScheduleDTO{
		ID: sc.ID, Enabled: sc.Enabled, IntervalMinutes: sc.IntervalMinutes,
		RetentionCount: sc.RetentionCount, OffsiteEnabled: sc.OffsiteEnabled,
		NextRunAt: sc.NextRunAt, LastRunAt: sc.LastRunAt,
	}
}

// RunBackupScheduler runs due schedules on each tick until ctx is cancelled.
func (s *Services) RunBackupScheduler(ctx context.Context, tick time.Duration) {
	if tick <= 0 {
		tick = time.Minute
	}
	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runDueBackups(ctx)
		}
	}
}

func (s *Services) runDueBackups(ctx context.Context) {
	if s.DatabaseURL == "" || s.BackupDir == "" {
		return
	}
	due, err := s.Store.DueBackupSchedules(ctx)
	if err != nil {
		return
	}
	for _, sc := range due {
		id, err := s.Store.CreateBackupJobForSchedule(ctx, sc.OrgID, sc.ID)
		if err != nil {
			continue
		}
		s.runBackup(ctx, id) // synchronous pg_dump
		if sc.OffsiteEnabled {
			s.offsiteCopy(ctx, id)
		}
		_ = s.Store.MarkScheduleRan(ctx, sc.ID, sc.IntervalMinutes)
		s.pruneBackups(ctx, sc.OrgID, sc.RetentionCount)
	}
}

// offsiteCopy uploads a succeeded backup file to the configured off-box S3 target.
func (s *Services) offsiteCopy(ctx context.Context, jobID string) {
	if !s.BackupOffsite.enabled() {
		return
	}
	job, err := s.Store.GetBackupJob(ctx, jobID)
	if err != nil || job.Status != "succeeded" || job.Path == "" {
		return
	}
	f, err := os.Open(job.Path)
	if err != nil {
		return
	}
	defer f.Close()
	fi, _ := f.Stat()
	key := filepath.Base(job.Path)
	cli := s3core.New(s.BackupOffsite.Endpoint, s.BackupOffsite.Endpoint, s.BackupOffsite.Region, s.BackupOffsite.AccessKey, s.BackupOffsite.Secret)
	if err := cli.PutObject(ctx, s.BackupOffsite.Bucket, key, f, fi.Size(), "application/octet-stream"); err != nil {
		s.Logger.Warn("backup offsite copy failed", slog.Any("error", err))
		return
	}
	_ = s.Store.SetBackupOffsiteURI(ctx, jobID, "s3://"+s.BackupOffsite.Bucket+"/"+key)
}

// pruneBackups deletes an org's on-disk dump files beyond the retention count.
func (s *Services) pruneBackups(ctx context.Context, orgID string, keep int) {
	if keep < 1 {
		keep = 1
	}
	paths, err := s.Store.SucceededBackupPathsBeyond(ctx, orgID, keep)
	if err != nil {
		return
	}
	for _, p := range paths {
		_ = os.Remove(p)
	}
}
