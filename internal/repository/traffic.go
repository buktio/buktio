package repository

import (
	"context"
	"fmt"
	"time"
)

// TrafficSample is one (access-key, bucket, method) counter row to persist.
type TrafficSample struct {
	ClusterID   string // optional; "" => NULL
	AccessKeyID string
	Bucket      string
	Method      string
	Requests    int64
	BytesIn     int64
	BytesOut    int64
}

// InsertTrafficSnapshots batch-inserts traffic counters (from buktio-s3proxy).
func (s *Store) InsertTrafficSnapshots(ctx context.Context, samples []TrafficSample) error {
	if len(samples) == 0 {
		return nil
	}
	// Sequential inserts through the driver-agnostic querier (portable to SQLite).
	// Traffic snapshots flush at a low cadence, so the batch round-trip isn't needed.
	q := s.q(ctx)
	for _, sm := range samples {
		if _, err := q.Exec(ctx, `INSERT INTO traffic_snapshots
		  (storage_cluster_id, access_key_id, bucket, method, requests, bytes_in, bytes_out)
		 VALUES (NULLIF($1,'')::uuid, $2, $3, $4, $5, $6, $7)`,
			sm.ClusterID, sm.AccessKeyID, sm.Bucket, sm.Method, sm.Requests, sm.BytesIn, sm.BytesOut); err != nil {
			return fmt.Errorf("repository: insert traffic: %w", err)
		}
	}
	return nil
}

// TrafficKeyRow is per-access-key traffic aggregated over a window.
type TrafficKeyRow struct {
	AccessKeyID string
	KeyName     string
	Requests    int64
	BytesIn     int64
	BytesOut    int64
}

// TrafficByKey aggregates traffic per access key since the given time, joining the
// key name where the key is buktio-managed.
func (s *Store) TrafficByKey(ctx context.Context, since time.Time) ([]TrafficKeyRow, error) {
	rows, err := s.q(ctx).Query(ctx, `
SELECT t.access_key_id, COALESCE(k.name, ''),
       sum(t.requests)::bigint, sum(t.bytes_in)::bigint, sum(t.bytes_out)::bigint
FROM traffic_snapshots t
LEFT JOIN access_keys k
       ON k.garage_access_key_id = t.access_key_id AND k.deleted_at IS NULL
WHERE t.captured_at >= $1
GROUP BY t.access_key_id, k.name
ORDER BY sum(t.bytes_out) DESC`, since)
	if err != nil {
		return nil, fmt.Errorf("repository: traffic by key: %w", err)
	}
	defer rows.Close()
	var out []TrafficKeyRow
	for rows.Next() {
		var r TrafficKeyRow
		if err := rows.Scan(&r.AccessKeyID, &r.KeyName, &r.Requests, &r.BytesIn, &r.BytesOut); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
