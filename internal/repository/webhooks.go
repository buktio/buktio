package repository

import (
	"context"
	"fmt"
	"time"
)

// WebhookSub is an object-event webhook subscription (bucket-scoped).
type WebhookSub struct {
	ID        string
	OrgID     string
	BucketID  string
	URL       string
	Secret    string
	Events    string // comma-separated event names
	Enabled   bool
	CreatedAt time.Time
}

// CreateWebhook inserts a subscription and returns it.
func (s *Store) CreateWebhook(ctx context.Context, orgID, bucketID, url, secret, events string) (WebhookSub, error) {
	const q = `
INSERT INTO webhook_subscriptions (org_id, bucket_id, url, secret, events)
VALUES ($1::uuid,$2::uuid,$3,$4,$5)
RETURNING id::text, org_id::text, bucket_id::text, url, secret, events, enabled, created_at`
	var w WebhookSub
	err := s.q(ctx).QueryRow(ctx, q, orgID, bucketID, url, secret, events).
		Scan(&w.ID, &w.OrgID, &w.BucketID, &w.URL, &w.Secret, &w.Events, &w.Enabled, &w.CreatedAt)
	if err != nil {
		return WebhookSub{}, fmt.Errorf("repository: create webhook: %w", err)
	}
	return w, nil
}

// ListWebhooksByBucket returns all subscriptions on a bucket.
func (s *Store) ListWebhooksByBucket(ctx context.Context, bucketID string) ([]WebhookSub, error) {
	return s.queryWebhooks(ctx,
		`SELECT id::text, org_id::text, bucket_id::text, url, secret, events, enabled, created_at
		 FROM webhook_subscriptions WHERE bucket_id=$1::uuid ORDER BY created_at`, bucketID)
}

// ListEnabledWebhooksByBucket returns enabled subscriptions for event dispatch.
func (s *Store) ListEnabledWebhooksByBucket(ctx context.Context, bucketID string) ([]WebhookSub, error) {
	return s.queryWebhooks(ctx,
		`SELECT id::text, org_id::text, bucket_id::text, url, secret, events, enabled, created_at
		 FROM webhook_subscriptions WHERE bucket_id=$1::uuid AND enabled`, bucketID)
}

func (s *Store) queryWebhooks(ctx context.Context, q, bucketID string) ([]WebhookSub, error) {
	rows, err := s.q(ctx).Query(ctx, q, bucketID)
	if err != nil {
		return nil, fmt.Errorf("repository: list webhooks: %w", err)
	}
	defer rows.Close()
	var out []WebhookSub
	for rows.Next() {
		var w WebhookSub
		if err := rows.Scan(&w.ID, &w.OrgID, &w.BucketID, &w.URL, &w.Secret, &w.Events, &w.Enabled, &w.CreatedAt); err != nil {
			return nil, fmt.Errorf("repository: scan webhook: %w", err)
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// DeleteWebhook removes a subscription scoped to its org.
func (s *Store) DeleteWebhook(ctx context.Context, id, orgID string) error {
	ct, err := s.q(ctx).Exec(ctx,
		`DELETE FROM webhook_subscriptions WHERE id=$1::uuid AND org_id=$2::uuid`, id, orgID)
	if err != nil {
		return fmt.Errorf("repository: delete webhook: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
