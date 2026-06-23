package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// BillingCustomer maps an org to its payment-processor customer + metered items.
type BillingCustomer struct {
	OrgID                string
	StripeCustomerID     string
	StripeSubscriptionID string
	StorageItemID        string
	EgressItemID         string
	RequestsItemID       string
	Status               string
}

// UpsertBillingCustomer creates or updates an org's billing customer record.
func (s *Store) UpsertBillingCustomer(ctx context.Context, c BillingCustomer) error {
	_, err := s.q(ctx).Exec(ctx,
		`INSERT INTO billing_customers
		   (org_id, stripe_customer_id, stripe_subscription_id, storage_item_id, egress_item_id, requests_item_id, status, updated_at)
		 VALUES ($1::uuid,$2,NULLIF($3,''),NULLIF($4,''),NULLIF($5,''),NULLIF($6,''),$7,now())
		 ON CONFLICT (org_id) DO UPDATE SET
		   stripe_customer_id=EXCLUDED.stripe_customer_id,
		   stripe_subscription_id=EXCLUDED.stripe_subscription_id,
		   storage_item_id=EXCLUDED.storage_item_id,
		   egress_item_id=EXCLUDED.egress_item_id,
		   requests_item_id=EXCLUDED.requests_item_id,
		   status=EXCLUDED.status, updated_at=now()`,
		c.OrgID, c.StripeCustomerID, c.StripeSubscriptionID, c.StorageItemID, c.EgressItemID, c.RequestsItemID, c.Status)
	if err != nil {
		return fmt.Errorf("repository: upsert billing customer: %w", err)
	}
	return nil
}

func scanBillingCustomer(row pgx.Row) (*BillingCustomer, error) {
	var c BillingCustomer
	var sub, storage, egress, requests *string
	err := row.Scan(&c.OrgID, &c.StripeCustomerID, &sub, &storage, &egress, &requests, &c.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("repository: get billing customer: %w", err)
	}
	c.StripeSubscriptionID = deref(sub)
	c.StorageItemID, c.EgressItemID, c.RequestsItemID = deref(storage), deref(egress), deref(requests)
	return &c, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

const billingCols = `org_id::text, stripe_customer_id, stripe_subscription_id, storage_item_id, egress_item_id, requests_item_id, status`

// GetBillingCustomer returns an org's billing record, or ErrNotFound.
func (s *Store) GetBillingCustomer(ctx context.Context, orgID string) (*BillingCustomer, error) {
	return scanBillingCustomer(s.q(ctx).QueryRow(ctx,
		`SELECT `+billingCols+` FROM billing_customers WHERE org_id=$1::uuid`, orgID))
}

// OrgByStripeCustomer resolves an org id from a Stripe customer id (webhook path).
func (s *Store) OrgByStripeCustomer(ctx context.Context, customerID string) (string, error) {
	var orgID string
	err := s.q(ctx).QueryRow(ctx,
		`SELECT org_id::text FROM billing_customers WHERE stripe_customer_id=$1`, customerID).Scan(&orgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("repository: org by stripe customer: %w", err)
	}
	return orgID, nil
}

// SetBillingStatus updates the billing status (active|past_due|canceled).
func (s *Store) SetBillingStatus(ctx context.Context, orgID, status string) error {
	_, err := s.q(ctx).Exec(ctx,
		`UPDATE billing_customers SET status=$2, updated_at=now() WHERE org_id=$1::uuid`, orgID, status)
	return err
}

// ListBillingOrgIDs returns org ids with an active billing customer (the reporter).
func (s *Store) ListBillingOrgIDs(ctx context.Context) ([]string, error) {
	rows, err := s.q(ctx).Query(ctx, `SELECT org_id::text FROM billing_customers WHERE status <> 'canceled'`)
	if err != nil {
		return nil, fmt.Errorf("repository: list billing orgs: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// RecordUsageReport stores a usage report idempotently. Returns true when the row
// was newly inserted (caller should then post to the processor); false means an
// identical report already exists (skip — prevents double billing).
func (s *Store) RecordUsageReport(ctx context.Context, orgID, metric string, quantity int64, periodStart, periodEnd time.Time, idempotencyKey string) (bool, error) {
	ct, err := s.q(ctx).Exec(ctx,
		`INSERT INTO billing_usage_reports (org_id, metric, quantity, period_start, period_end, idempotency_key)
		 VALUES ($1::uuid,$2,$3,$4,$5,$6) ON CONFLICT (idempotency_key) DO NOTHING`,
		orgID, metric, quantity, periodStart, periodEnd, idempotencyKey)
	if err != nil {
		return false, fmt.Errorf("repository: record usage report: %w", err)
	}
	return ct.RowsAffected() == 1, nil
}

// TrafficByOrg sums egress bytes + request counts for an org since a time, joining
// traffic_snapshots to buckets by the Garage global alias.
func (s *Store) TrafficByOrg(ctx context.Context, orgID string, since time.Time) (bytesOut, requests int64, err error) {
	err = s.q(ctx).QueryRow(ctx, `
SELECT COALESCE(sum(t.bytes_out),0), COALESCE(sum(t.requests),0)
  FROM traffic_snapshots t
  JOIN buckets b ON b.garage_global_alias = t.bucket AND b.deleted_at IS NULL
 WHERE b.org_id=$1::uuid AND t.captured_at >= $2`, orgID, since).Scan(&bytesOut, &requests)
	if err != nil {
		return 0, 0, fmt.Errorf("repository: traffic by org: %w", err)
	}
	return bytesOut, requests, nil
}
