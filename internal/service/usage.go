package service

import (
	"context"
	"sort"
	"time"
)

// TrafficDTO is per-access-key traffic over a window (from the metering proxy).
type TrafficDTO struct {
	AccessKeyID string `json:"access_key_id"`
	KeyName     string `json:"key_name"`
	Requests    int64  `json:"requests"`
	BytesIn     int64  `json:"bytes_in"`
	BytesOut    int64  `json:"bytes_out"`
}

// TrafficUsage returns per-key traffic over the last `hours` hours (default 24).
// Garage exposes no per-key traffic; this data comes from buktio-s3proxy.
func (s *Services) TrafficUsage(ctx context.Context, hours int) ([]TrafficDTO, error) {
	if hours <= 0 {
		hours = 24
	}
	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	rows, err := s.Store.TrafficByKey(ctx, since)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	out := make([]TrafficDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, TrafficDTO{
			AccessKeyID: r.AccessKeyID, KeyName: r.KeyName,
			Requests: r.Requests, BytesIn: r.BytesIn, BytesOut: r.BytesOut,
		})
	}
	return out, nil
}

// StoragePointDTO is one interval of the storage-growth series.
type StoragePointDTO struct {
	TS          time.Time `json:"ts"`
	BytesUsed   int64     `json:"bytes_used"`
	ObjectCount int64     `json:"object_count"`
}

// StorageSeries returns the project's storage totals over the last `hours` hours,
// bucketed into ~48 intervals (step >= the 5-min collector cadence). Drives the
// dashboard storage-growth chart.
func (s *Services) StorageSeries(ctx context.Context, hours int) ([]StoragePointDTO, error) {
	if hours <= 0 {
		hours = 24 * 7
	}
	step := hours * 3600 / 48
	if step < 300 {
		step = 300
	}
	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	pts, err := s.Store.StorageSeries(ctx, s.tenant(ctx).ProjectID, since, step)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	out := make([]StoragePointDTO, 0, len(pts))
	for _, p := range pts {
		out = append(out, StoragePointDTO{TS: p.TS, BytesUsed: p.BytesUsed, ObjectCount: p.ObjectCount})
	}
	return out, nil
}

// BucketUsageDTO is per-bucket usage with quota utilisation for the dashboard.
type BucketUsageDTO struct {
	BucketID     string   `json:"bucket_id"`
	Name         string   `json:"name"`
	BytesUsed    int64    `json:"bytes_used"`
	ObjectCount  int64    `json:"object_count"`
	QuotaMaxSize *int64   `json:"quota_max_size"`
	QuotaPct     *float64 `json:"quota_pct"`
}

// BucketUsage returns the latest per-bucket usage in the project, largest first,
// with quota utilisation where a quota is set.
func (s *Services) BucketUsage(ctx context.Context) ([]BucketUsageDTO, error) {
	rows, err := s.Store.BucketUsageList(ctx, s.tenant(ctx).ProjectID)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	out := make([]BucketUsageDTO, 0, len(rows))
	for _, r := range rows {
		d := BucketUsageDTO{
			BucketID: r.BucketID, Name: r.Name,
			BytesUsed: r.BytesUsed, ObjectCount: r.ObjectCount, QuotaMaxSize: r.QuotaMaxSize,
		}
		if r.QuotaMaxSize != nil && *r.QuotaMaxSize > 0 {
			pct := float64(r.BytesUsed) / float64(*r.QuotaMaxSize) * 100
			d.QuotaPct = &pct
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].BytesUsed > out[j].BytesUsed })
	return out, nil
}
