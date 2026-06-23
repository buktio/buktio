// Package billing is the usage-based-billing seam (Hosted). The reporter aggregates
// metered usage per org and hands it to a Provider; webhooks from the payment
// processor drive the org lifecycle (suspend on non-payment, resume on success).
//
// The OSS/default build uses Disabled (no billing). The Hosted edition injects the
// Stripe-backed ee/billing.Provider. A Manual provider is available for local
// testing of the reporter + webhook flow without a real processor. The Provider
// never touches the database — the service writes billing_customers from the
// Subscription a Provider returns.
package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
)

// Subscription is the processor-side identity created for an org.
type Subscription struct {
	CustomerID     string
	SubscriptionID string
	StorageItemID  string // metered item: storage GB-month
	EgressItemID   string // metered item: egress GB
	RequestsItemID string // metered item: request count
}

// WebhookEvent is a normalized payment-processor event the core reacts to.
type WebhookEvent struct {
	// Type is normalized to: payment_failed | payment_succeeded | subscription_canceled.
	Type string `json:"type"`
	// CustomerID is the processor customer id (the service resolves the org from it).
	CustomerID string `json:"customer_id"`
	// OrgID, when set (e.g. the Manual provider), is used directly.
	OrgID string `json:"org_id"`
}

// Provider integrates with a payment processor (Stripe in production).
type Provider interface {
	Enabled() bool
	// CreateCustomerSubscription provisions a customer + metered subscription.
	CreateCustomerSubscription(ctx context.Context, orgID, email string) (Subscription, error)
	// ReportUsage posts a metered quantity to a subscription item. idempotencyKey
	// dedupes retries at the processor too.
	ReportUsage(ctx context.Context, subscriptionItemID string, quantity int64, idempotencyKey string) error
	// ParseWebhook verifies + normalizes an inbound webhook.
	ParseWebhook(payload []byte, sigHeader string) (WebhookEvent, error)
}

// Disabled is the OSS no-billing provider.
type Disabled struct{}

func (Disabled) Enabled() bool { return false }
func (Disabled) CreateCustomerSubscription(context.Context, string, string) (Subscription, error) {
	return Subscription{}, fmt.Errorf("billing: not enabled")
}
func (Disabled) ReportUsage(context.Context, string, int64, string) error { return nil }
func (Disabled) ParseWebhook([]byte, string) (WebhookEvent, error) {
	return WebhookEvent{}, fmt.Errorf("billing: not enabled")
}

// Manual is a processor-free provider for local testing. It logs usage reports and
// parses an UNSIGNED JSON webhook {"type":"...","org_id":"..."}. Never use it in
// production — no signature verification.
type Manual struct{ Logger *slog.Logger }

func (Manual) Enabled() bool { return true }

func (Manual) CreateCustomerSubscription(_ context.Context, orgID, _ string) (Subscription, error) {
	return Subscription{
		CustomerID:     "cus_manual_" + orgID[:8],
		SubscriptionID: "sub_manual_" + orgID[:8],
		StorageItemID:  "si_storage_" + orgID[:8],
		EgressItemID:   "si_egress_" + orgID[:8],
		RequestsItemID: "si_requests_" + orgID[:8],
	}, nil
}

func (m Manual) ReportUsage(_ context.Context, itemID string, qty int64, idem string) error {
	if m.Logger != nil {
		m.Logger.Info("billing(manual): report usage",
			slog.String("item", itemID), slog.Int64("qty", qty), slog.String("idem", idem))
	}
	return nil
}

func (Manual) ParseWebhook(payload []byte, _ string) (WebhookEvent, error) {
	var e WebhookEvent
	if err := json.Unmarshal(payload, &e); err != nil {
		return WebhookEvent{}, fmt.Errorf("billing(manual): bad payload: %w", err)
	}
	if e.Type == "" {
		return WebhookEvent{}, fmt.Errorf("billing(manual): missing type")
	}
	return e, nil
}
