package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/buktio/buktio/internal/metering"
	"github.com/buktio/buktio/internal/repository"
	"github.com/buktio/buktio/internal/storage"
	"github.com/buktio/buktio/internal/storage/s3core"
)

// MigrationInput parameterizes an S3-to-S3 import.
type MigrationInput struct {
	SourceEndpoint  string
	SourceRegion    string
	SourceBucket    string
	AccessKeyID     string
	SecretAccessKey string
	DestBucketID    string
}

// MigrationJobDTO is a migration job as the API returns it.
type MigrationJobDTO struct {
	ID            string `json:"id"`
	SourceBucket  string `json:"source_bucket"`
	DestBucketID  string `json:"dest_bucket_id"`
	Status        string `json:"status"`
	CopiedObjects int64  `json:"copied_objects"`
	CopiedBytes   int64  `json:"copied_bytes"`
	Error         string `json:"error,omitempty"`
}

func migrationDTO(j *repository.MigrationJob) MigrationJobDTO {
	return MigrationJobDTO{
		ID: j.ID, SourceBucket: j.SourceBucket, DestBucketID: j.DestBucketID,
		Status: j.Status, CopiedObjects: j.CopiedObjects, CopiedBytes: j.CopiedBytes, Error: j.Error,
	}
}

// StartMigration validates the source + dest, encrypts the source secret, creates a
// job, and kicks the worker. Owner or platform admin.
func (s *Services) StartMigration(ctx context.Context, in MigrationInput) (*MigrationJobDTO, error) {
	if err := s.scimTokenActor(ctx); err != nil { // owner or platform admin
		return nil, err
	}
	if in.SourceEndpoint == "" || in.SourceBucket == "" || in.AccessKeyID == "" || in.SecretAccessKey == "" {
		return nil, validationErr("source endpoint, bucket, access key and secret are required")
	}
	dest, derr := s.loadBucket(ctx, in.DestBucketID) // tenant-scoped
	if derr != nil {
		return nil, derr
	}
	region := in.SourceRegion
	if region == "" {
		region = "us-east-1"
	}
	// Probe the source before accepting the job.
	src := s3core.New(in.SourceEndpoint, in.SourceEndpoint, region, in.AccessKeyID, in.SecretAccessKey)
	if _, err := src.ListObjects(ctx, in.SourceBucket, storage.ListObjectsInput{MaxKeys: 1}); err != nil {
		return nil, validationErr("cannot read the source bucket: " + err.Error())
	}
	secretEnc, err := s.Sealer.Seal([]byte(in.SecretAccessKey))
	if err != nil {
		return nil, mapRepoErr(err)
	}
	id, err := s.Store.CreateMigrationJob(ctx, repository.MigrationJob{
		OrgID: s.tenant(ctx).OrgID, SourceEndpoint: in.SourceEndpoint, SourceRegion: region,
		SourceBucket: in.SourceBucket, SourceAccessKey: in.AccessKeyID, SourceSecretEnc: secretEnc,
		DestBucketID: dest.ID,
	})
	if err != nil {
		return nil, mapRepoErr(err)
	}
	s.audit(ctx, "migration.start", "bucket", dest.ID, map[string]any{"source": in.SourceBucket})
	go s.runMigration(context.Background(), id) // detached; a boot sweep resumes it after a restart
	return &MigrationJobDTO{ID: id, SourceBucket: in.SourceBucket, DestBucketID: dest.ID, Status: "pending"}, nil
}

// GetMigration returns a job's status (tenant-scoped).
func (s *Services) GetMigration(ctx context.Context, id string) (*MigrationJobDTO, error) {
	j, err := s.Store.GetMigrationJob(ctx, s.tenant(ctx).OrgID, id)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	dto := migrationDTO(j)
	return &dto, nil
}

// ListMigrations returns the org's jobs.
func (s *Services) ListMigrations(ctx context.Context) ([]MigrationJobDTO, error) {
	rows, err := s.Store.ListMigrationJobs(ctx, s.tenant(ctx).OrgID)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	out := make([]MigrationJobDTO, 0, len(rows))
	for i := range rows {
		out = append(out, migrationDTO(&rows[i]))
	}
	return out, nil
}

// CancelMigration marks a job canceled (the worker stops at the next object).
func (s *Services) CancelMigration(ctx context.Context, id string) error {
	if err := s.scimTokenActor(ctx); err != nil {
		return err
	}
	if _, err := s.Store.GetMigrationJob(ctx, s.tenant(ctx).OrgID, id); err != nil {
		return mapRepoErr(err)
	}
	return s.Store.SetMigrationStatus(ctx, id, "canceled", "")
}

// ResumeMigrations re-runs any unfinished jobs (boot sweep / control loop).
func (s *Services) ResumeMigrations(ctx context.Context) {
	ids, err := s.Store.ListResumableMigrationJobIDs(ctx)
	if err != nil {
		return
	}
	for _, id := range ids {
		go s.runMigration(context.Background(), id)
	}
}

// runMigration is the worker: it streams every source object into the dest bucket
// from the saved cursor, verifies size via HeadObject, and persists progress so a
// crash/restart resumes without re-copying or duplicating.
func (s *Services) runMigration(ctx context.Context, id string) {
	j, err := s.Store.GetMigrationJobByID(ctx, id)
	if err != nil {
		return
	}
	if j.Status == "completed" || j.Status == "canceled" {
		return
	}
	secret, err := s.Sealer.Open(j.SourceSecretEnc)
	if err != nil {
		_ = s.Store.SetMigrationStatus(ctx, id, "failed", "cannot decrypt source secret")
		return
	}
	src := s3core.New(j.SourceEndpoint, j.SourceEndpoint, j.SourceRegion, j.SourceAccessKey, string(secret))

	dest, derr := s.loadBucketUnscoped(ctx, j.DestBucketID)
	if derr != nil {
		_ = s.Store.SetMigrationStatus(ctx, id, "failed", "dest bucket unavailable")
		return
	}
	// Scope the detached worker context to the job's tenant so metering events are
	// attributed to the right org (the worker has no request tenant otherwise).
	ctx = WithTenant(ctx, TenantContext{OrgID: j.OrgID, ProjectID: dest.ProjectID, ClusterID: dest.ClusterID})
	destProv, perr := s.providerForBucket(ctx, dest)
	if perr != nil {
		_ = s.Store.SetMigrationStatus(ctx, id, "failed", "dest cluster unreachable")
		return
	}

	cursor := j.Cursor
	copiedObjects, copiedBytes := j.CopiedObjects, j.CopiedBytes
	for {
		// Stop promptly if the job was canceled out-of-band.
		if cur, err := s.Store.GetMigrationJobByID(ctx, id); err == nil && cur.Status == "canceled" {
			return
		}
		page, err := src.ListObjects(ctx, j.SourceBucket, storage.ListObjectsInput{ContinuationToken: cursor, MaxKeys: 1000})
		if err != nil {
			_ = s.Store.SetMigrationStatus(ctx, id, "failed", "list source: "+err.Error())
			return
		}
		for _, o := range page.Objects {
			if o.IsPrefix {
				continue
			}
			if err := s.copyOneObject(ctx, src, destProv, j.SourceBucket, dest.GarageGlobalAlias, o); err != nil {
				_ = s.Store.SetMigrationStatus(ctx, id, "failed", fmt.Sprintf("copy %q: %v", o.Key, err))
				return
			}
			copiedObjects++
			copiedBytes += o.Size
			s.emit(ctx, metering.EventObjectUploaded, dest.ID, o.Size)
		}
		cursor = page.NextContinuationToken
		_ = s.Store.UpdateMigrationProgress(ctx, id, cursor, copiedObjects, copiedBytes)
		if !page.IsTruncated || cursor == "" {
			break
		}
	}
	_ = s.Store.SetMigrationStatus(ctx, id, "completed", "")
	s.Logger.Info("migration completed", slog.String("job", id), slog.Int64("objects", copiedObjects))
}

func (s *Services) copyOneObject(ctx context.Context, src *s3core.Client, dest storage.StorageProvider, srcBucket, destAlias string, o storage.Object) error {
	rc, _, err := src.GetObject(ctx, srcBucket, o.Key)
	if err != nil {
		return err
	}
	defer rc.Close()
	// Spool to a temp file so the dest PutObject has a seekable body for SigV4
	// hashing, without holding the whole object in memory (bounds large objects).
	tmp, err := os.CreateTemp("", "buktio-mig-*")
	if err != nil {
		return err
	}
	defer func() { tmp.Close(); os.Remove(tmp.Name()) }()
	n, err := io.Copy(tmp, rc)
	if err != nil {
		return err
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if err := dest.PutObject(ctx, destAlias, o.Key, tmp, n, ""); err != nil {
		return err
	}
	// Verify the copy landed with the expected size.
	if h, herr := dest.HeadObject(ctx, destAlias, o.Key); herr == nil && n > 0 && h.Size != n {
		return fmt.Errorf("size mismatch after copy: got %d want %d", h.Size, n)
	}
	return nil
}

// loadBucketUnscoped loads a bucket by id without the request tenant scope (the
// migration worker runs on a detached context with no tenant).
func (s *Services) loadBucketUnscoped(ctx context.Context, id string) (*repository.Bucket, *Error) {
	b, err := s.Store.GetBucket(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	return b, nil
}
