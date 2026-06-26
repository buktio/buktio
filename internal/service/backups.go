package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/buktio/buktio/internal/authz"
	"github.com/buktio/buktio/internal/repository"
)

// BackupDTO is a backup job as the API returns it.
type BackupDTO struct {
	ID         string     `json:"id"`
	Kind       string     `json:"kind"`
	Status     string     `json:"status"`
	Path       string     `json:"path"`
	SizeBytes  int64      `json:"size_bytes"`
	Error      string     `json:"error,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

func backupToDTO(b *repository.BackupJob) BackupDTO {
	return BackupDTO{
		ID: b.ID, Kind: b.Kind, Status: b.Status, Path: b.Path, SizeBytes: b.SizeBytes,
		Error: b.Error, CreatedAt: b.CreatedAt, FinishedAt: b.FinishedAt,
	}
}

// CreateBackup starts an asynchronous pg_dump of buktio's metadata DB and returns
// the pending job. It backs up PostgreSQL state only — NEVER the KEK and NEVER
// Garage object data (the operator's responsibility, documented).
func (s *Services) CreateBackup(ctx context.Context) (*BackupDTO, error) {
	if err := s.authorize(ctx, authz.ActionCreate, authz.Target{Kind: authz.ResourceBackup}); err != nil {
		return nil, err
	}
	if s.DatabaseURL == "" || s.BackupDir == "" {
		return nil, &Error{Code: "backup_unavailable", Message: "backups are not configured on this deployment", HTTP: http.StatusServiceUnavailable}
	}
	id, err := s.Store.CreateBackupJob(ctx, s.tenant(ctx).OrgID, "metadata")
	if err != nil {
		return nil, mapRepoErr(err)
	}
	s.audit(ctx, "backup.start", "backup", id, nil)
	go s.runBackup(context.Background(), id)
	return s.GetBackup(ctx, id)
}

func (s *Services) runBackup(ctx context.Context, id string) {
	_ = s.Store.MarkBackupRunning(ctx, id)
	if err := os.MkdirAll(s.BackupDir, 0o750); err != nil {
		_ = s.Store.FinishBackupJob(ctx, id, "failed", "", 0, "create backup dir: "+err.Error())
		return
	}
	ts := time.Now().UTC().Format("20060102-150405")

	// SQLite (OSS single-node): an online VACUUM INTO produces a consistent copy
	// of the database file — the SQLite analogue of pg_dump, no external tool.
	if s.Store.Driver() == "sqlite" {
		path := filepath.Join(s.BackupDir, fmt.Sprintf("buktio-metadata-%s.sqlite", ts))
		if err := s.Store.BackupSQLite(ctx, path); err != nil {
			_ = s.Store.FinishBackupJob(ctx, id, "failed", "", 0, "vacuum into: "+err.Error())
			return
		}
		var size int64
		if fi, err := os.Stat(path); err == nil {
			size = fi.Size()
		}
		_ = s.Store.FinishBackupJob(ctx, id, "succeeded", path, size, "")
		s.Logger.Info("backup completed", slog.String("id", id), slog.String("path", path), slog.Int64("bytes", size))
		return
	}

	path := filepath.Join(s.BackupDir, fmt.Sprintf("buktio-metadata-%s.dump", ts))

	// Pass connection params via PG* env vars (NOT argv) so the password never
	// appears in the process list. pg_dump custom format (-Fc) is compressed and
	// restorable via pg_restore.
	cfg, perr := pgconn.ParseConfig(s.DatabaseURL)
	if perr != nil {
		_ = s.Store.FinishBackupJob(ctx, id, "failed", "", 0, "parse database url: "+perr.Error())
		return
	}
	cmd := exec.CommandContext(ctx, "pg_dump", "-Fc", "--no-owner", "--no-privileges", "-f", path)
	cmd.Env = append(os.Environ(),
		"PGHOST="+cfg.Host,
		fmt.Sprintf("PGPORT=%d", cfg.Port),
		"PGUSER="+cfg.User,
		"PGPASSWORD="+cfg.Password,
		"PGDATABASE="+cfg.Database,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = s.Store.FinishBackupJob(ctx, id, "failed", "", 0, fmt.Sprintf("pg_dump: %v: %s", err, string(out)))
		return
	}
	var size int64
	if fi, err := os.Stat(path); err == nil {
		size = fi.Size()
	}
	_ = s.Store.FinishBackupJob(ctx, id, "succeeded", path, size, "")
	s.Logger.Info("backup completed", slog.String("id", id), slog.String("path", path), slog.Int64("bytes", size))
}

// GetBackup returns one backup job (tenant-scoped).
func (s *Services) GetBackup(ctx context.Context, id string) (*BackupDTO, error) {
	b, err := s.Store.GetBackupJob(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	if b.OrgID != s.tenant(ctx).OrgID {
		return nil, notFoundErr() // another tenant's backup
	}
	dto := backupToDTO(b)
	return &dto, nil
}

// ListBackups returns recent backup jobs for the active tenant.
func (s *Services) ListBackups(ctx context.Context, limit int) ([]BackupDTO, error) {
	rows, err := s.Store.ListBackupJobs(ctx, s.tenant(ctx).OrgID, limit)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	out := make([]BackupDTO, 0, len(rows))
	for i := range rows {
		out = append(out, backupToDTO(&rows[i]))
	}
	return out, nil
}

// DriftDTO reports differences between the DB's record of buckets and what the
// primary backend actually has — used after a restore to spot drift.
type DriftDTO struct {
	BucketsOnlyInDB      []string `json:"buckets_only_in_db"`
	BucketsOnlyInBackend []string `json:"buckets_only_in_backend"`
	InSync               bool     `json:"in_sync"`
}

// ReconcileReport compares the primary backend's buckets to the DB record. Only
// buckets on the primary cluster are considered (s.Provider is the primary backend),
// so secondary-cluster buckets do not show up as false drift.
func (s *Services) ReconcileReport(ctx context.Context) (*DriftDTO, error) {
	if err := s.authorize(ctx, authz.ActionRead, authz.Target{Kind: authz.ResourceSystem}); err != nil {
		return nil, err
	}
	dbRows, err := s.Store.ListBuckets(ctx, s.tenant(ctx).ProjectID)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	dbSet := map[string]bool{}
	for i := range dbRows {
		if dbRows[i].ClusterID != s.ClusterID {
			continue // only the primary cluster is compared against s.Provider
		}
		dbSet[dbRows[i].GarageGlobalAlias] = true
	}
	backendSet := map[string]bool{}
	if bks, berr := s.Provider.ListBuckets(ctx); berr == nil {
		for _, b := range bks {
			for _, a := range b.GlobalAliases {
				backendSet[a] = true
			}
		}
	}
	d := &DriftDTO{}
	for name := range dbSet {
		if !backendSet[name] {
			d.BucketsOnlyInDB = append(d.BucketsOnlyInDB, name)
		}
	}
	for name := range backendSet {
		if !dbSet[name] {
			d.BucketsOnlyInBackend = append(d.BucketsOnlyInBackend, name)
		}
	}
	d.InSync = len(d.BucketsOnlyInDB) == 0 && len(d.BucketsOnlyInBackend) == 0
	return d, nil
}
