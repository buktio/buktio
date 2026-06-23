package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/buktio/buktio/internal/metering"
	"github.com/buktio/buktio/internal/repository"
)

// CollectUsageOnce snapshots every bucket's usage (from GetBucketInfo) into the
// usage_snapshots table, and purges expired trash. Errors per bucket are logged.
func (s *Services) CollectUsageOnce(ctx context.Context) {
	s.PurgeDueTrash(ctx)

	buckets, err := s.Store.ListBuckets(ctx, s.ProjectID)
	if err != nil {
		s.Logger.Warn("usage collector: list buckets failed", slog.Any("error", err))
		return
	}
	for i := range buckets {
		b := &buckets[i]
		prov, perr := s.providerForBucket(ctx, b)
		if perr != nil {
			s.Logger.Debug("usage collector: resolve provider failed", slog.String("bucket", b.Name), slog.Any("error", perr))
			continue
		}
		u, err := prov.GetBucketUsage(ctx, b.GarageBucketID)
		if err != nil {
			s.Logger.Debug("usage collector: get usage failed", slog.String("bucket", b.Name), slog.Any("error", err))
			continue
		}
		if err := s.Store.InsertUsageSnapshot(ctx, repository.UsageSnapshot{
			BucketID:                   b.ID,
			ProjectID:                  b.ProjectID,
			OrgID:                      b.OrgID,
			ClusterID:                  b.ClusterID,
			BytesUsed:                  u.BytesUsed,
			ObjectCount:                u.ObjectCount,
			UnfinishedUploads:          u.UnfinishedUploads,
			UnfinishedMultipartUploads: u.UnfinishedMultipartUploads,
			QuotaMaxSize:               u.QuotaMaxSize,
			QuotaMaxObjects:            u.QuotaMaxObjects,
		}); err != nil {
			s.Logger.Warn("usage collector: insert snapshot failed", slog.String("bucket", b.Name), slog.Any("error", err))
			continue
		}
		// Primary storage signal for billing: current bytes_used per bucket. The
		// reporter aggregates these into GB-month. TenantID from the bucket's org so
		// background collection meters the right tenant (ctx has no request tenant).
		if s.Meter != nil {
			_ = s.Meter.Emit(ctx, metering.Event{
				Type: metering.EventUsageSnapshot, TenantID: b.OrgID, Subject: b.ID,
				Quantity: u.BytesUsed, At: time.Now().UTC(),
				Attributes: map[string]string{"cluster_id": b.ClusterID},
			})
		}
	}
}

// RunUsageCollector runs the snapshot loop until ctx is cancelled. It snapshots
// once on start, then every interval.
func (s *Services) RunUsageCollector(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	s.CollectUsageOnce(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.CollectUsageOnce(ctx)
		}
	}
}
