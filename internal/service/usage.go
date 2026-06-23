package service

import (
	"context"
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
