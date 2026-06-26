package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/buktio/buktio/internal/authz"
	"github.com/buktio/buktio/internal/metering"
	"github.com/buktio/buktio/internal/repository"
	"github.com/buktio/buktio/internal/storage"
)

// ReplicationJobDTO is a replication job as the API returns it.
type ReplicationJobDTO struct {
	ID             string    `json:"id"`
	SrcBucketID    string    `json:"src_bucket_id"`
	DstBucketID    string    `json:"dst_bucket_id"`
	Status         string    `json:"status"`
	CopiedObjects  int64     `json:"copied_objects"`
	SkippedObjects int64     `json:"skipped_objects"`
	CopiedBytes    int64     `json:"copied_bytes"`
	Error          string    `json:"error,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func replicationDTO(j *repository.ReplicationJob) ReplicationJobDTO {
	return ReplicationJobDTO{
		ID: j.ID, SrcBucketID: j.SrcBucketID, DstBucketID: j.DstBucketID, Status: j.Status,
		CopiedObjects: j.CopiedObjects, SkippedObjects: j.SkippedObjects, CopiedBytes: j.CopiedBytes,
		Error: j.Error, CreatedAt: j.CreatedAt, UpdatedAt: j.UpdatedAt,
	}
}

// StartReplication kicks off a one-shot replication of src into dst (which may live
// on a different backend). Re-running skips objects already present with the same
// size, so it doubles as an incremental sync.
func (s *Services) StartReplication(ctx context.Context, srcBucketID, dstBucketID string) (*ReplicationJobDTO, error) {
	if err := s.authorize(ctx, authz.ActionUpdate, authz.Target{Kind: authz.ResourceBucket}); err != nil {
		return nil, err
	}
	if srcBucketID == "" || dstBucketID == "" || srcBucketID == dstBucketID {
		return nil, validationErr("distinct source and destination buckets are required")
	}
	// Validate both buckets exist and are reachable in the tenant scope.
	if _, _, err := s.bucketProvider(ctx, srcBucketID); err != nil {
		return nil, err
	}
	if _, _, err := s.bucketProvider(ctx, dstBucketID); err != nil {
		return nil, err
	}
	id, err := s.Store.CreateReplicationJob(ctx, s.tenant(ctx).OrgID, srcBucketID, dstBucketID)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	s.audit(ctx, "replication.start", "bucket", srcBucketID, map[string]any{"dst": dstBucketID, "job": id})
	go s.runReplication(context.Background(), id)
	j, err := s.Store.GetReplicationJobByID(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	dto := replicationDTO(j)
	return &dto, nil
}

// ListReplications returns a bucket's replication jobs (newest first).
func (s *Services) ListReplications(ctx context.Context, srcBucketID string) ([]ReplicationJobDTO, error) {
	if _, _, err := s.bucketProvider(ctx, srcBucketID); err != nil {
		return nil, err
	}
	rows, err := s.Store.ListReplicationJobsBySrc(ctx, srcBucketID)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	out := make([]ReplicationJobDTO, 0, len(rows))
	for i := range rows {
		out = append(out, replicationDTO(&rows[i]))
	}
	return out, nil
}

// ResumeReplications restarts jobs interrupted by a restart (called on boot).
func (s *Services) ResumeReplications(ctx context.Context) {
	ids, err := s.Store.ListResumableReplicationJobIDs(ctx)
	if err != nil {
		return
	}
	for _, id := range ids {
		go s.runReplication(context.Background(), id)
	}
}

// runReplication is the worker: it streams every source object into the dest bucket
// from the saved cursor, skipping objects already present with the same size, and
// persists progress so a crash/restart resumes without re-copying.
func (s *Services) runReplication(ctx context.Context, id string) {
	j, err := s.Store.GetReplicationJobByID(ctx, id)
	if err != nil || j.Status == "completed" || j.Status == "canceled" {
		return
	}
	srcB, derr := s.loadBucketUnscoped(ctx, j.SrcBucketID)
	if derr != nil {
		_ = s.Store.SetReplicationStatus(ctx, id, "failed", "source bucket unavailable")
		return
	}
	dstB, derr := s.loadBucketUnscoped(ctx, j.DstBucketID)
	if derr != nil {
		_ = s.Store.SetReplicationStatus(ctx, id, "failed", "destination bucket unavailable")
		return
	}
	ctx = WithTenant(ctx, TenantContext{OrgID: j.OrgID, ProjectID: dstB.ProjectID, ClusterID: dstB.ClusterID})
	srcProv, perr := s.providerForBucket(ctx, srcB)
	if perr != nil {
		_ = s.Store.SetReplicationStatus(ctx, id, "failed", "source cluster unreachable")
		return
	}
	dstProv, perr := s.providerForBucket(ctx, dstB)
	if perr != nil {
		_ = s.Store.SetReplicationStatus(ctx, id, "failed", "destination cluster unreachable")
		return
	}

	cursor := j.Cursor
	copied, skipped, copiedBytes := j.CopiedObjects, j.SkippedObjects, j.CopiedBytes
	for {
		if cur, err := s.Store.GetReplicationJobByID(ctx, id); err == nil && cur.Status == "canceled" {
			return
		}
		page, err := srcProv.ListObjects(ctx, srcB.GarageGlobalAlias, storage.ListObjectsInput{ContinuationToken: cursor, MaxKeys: 1000})
		if err != nil {
			_ = s.Store.SetReplicationStatus(ctx, id, "failed", "list source: "+err.Error())
			return
		}
		for _, o := range page.Objects {
			if o.IsPrefix {
				continue
			}
			// Incremental: skip when the destination already has the same-sized object.
			if h, herr := dstProv.HeadObject(ctx, dstB.GarageGlobalAlias, o.Key); herr == nil && h.Size == o.Size {
				skipped++
				continue
			}
			if err := s.copyOneObjectProv(ctx, srcProv, dstProv, srcB.GarageGlobalAlias, dstB.GarageGlobalAlias, o); err != nil {
				_ = s.Store.SetReplicationStatus(ctx, id, "failed", fmt.Sprintf("copy %q: %v", o.Key, err))
				return
			}
			copied++
			copiedBytes += o.Size
			s.emit(ctx, metering.EventObjectUploaded, dstB.ID, o.Size)
		}
		cursor = page.NextContinuationToken
		_ = s.Store.UpdateReplicationProgress(ctx, id, cursor, copied, skipped, copiedBytes)
		if !page.IsTruncated || cursor == "" {
			break
		}
	}
	_ = s.Store.SetReplicationStatus(ctx, id, "completed", "")
	s.Logger.Info("replication completed",
		slog.String("job", id), slog.Int64("copied", copied), slog.Int64("skipped", skipped))
}

// copyOneObjectProv streams one object from a source provider to a dest provider,
// spooling through a temp file so the dest PutObject has a seekable body for SigV4.
func (s *Services) copyOneObjectProv(ctx context.Context, srcProv, dstProv storage.StorageProvider, srcAlias, dstAlias string, o storage.Object) error {
	rc, _, err := srcProv.GetObject(ctx, srcAlias, o.Key)
	if err != nil {
		return err
	}
	defer rc.Close()
	tmp, err := os.CreateTemp("", "buktio-repl-*")
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
	if err := dstProv.PutObject(ctx, dstAlias, o.Key, tmp, n, ""); err != nil {
		return err
	}
	if h, herr := dstProv.HeadObject(ctx, dstAlias, o.Key); herr == nil && n > 0 && h.Size != n {
		return fmt.Errorf("size mismatch after copy: got %d want %d", h.Size, n)
	}
	return nil
}
