package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/buktio/buktio/internal/repository"
)

const gib = 1024 * 1024 * 1024

// BillingStatusDTO is an org's billing state + current-period usage.
type BillingStatusDTO struct {
	Enabled       bool   `json:"enabled"`
	Status        string `json:"status,omitempty"`
	StorageBytes  int64  `json:"storage_bytes"`
	EgressBytes   int64  `json:"egress_bytes"`
	Requests      int64  `json:"requests"`
	CustomerSet   bool   `json:"customer_set"`
	PeriodStartIS string `json:"period_start,omitempty"`
}

// SetupBilling provisions a processor customer + metered subscription for the active
// org and stores the mapping. Owner or platform-admin; idempotent.
func (s *Services) SetupBilling(ctx context.Context, email string) error {
	if err := s.scimTokenActor(ctx); err != nil { // owner or platform admin
		return err
	}
	if !s.Billing.Enabled() {
		return &Error{Code: "billing_disabled", Message: "billing is not enabled", HTTP: 404}
	}
	orgID := s.tenant(ctx).OrgID
	if existing, err := s.Store.GetBillingCustomer(ctx, orgID); err == nil {
		_ = existing
		return nil // already set up
	}
	sub, err := s.Billing.CreateCustomerSubscription(ctx, orgID, email)
	if err != nil {
		return &Error{Code: "billing_setup_failed", Message: err.Error(), HTTP: 502}
	}
	if err := s.Store.UpsertBillingCustomer(ctx, repository.BillingCustomer{
		OrgID: orgID, StripeCustomerID: sub.CustomerID, StripeSubscriptionID: sub.SubscriptionID,
		StorageItemID: sub.StorageItemID, EgressItemID: sub.EgressItemID, RequestsItemID: sub.RequestsItemID,
		Status: "active",
	}); err != nil {
		return mapRepoErr(err)
	}
	s.audit(ctx, "billing.setup", "organization", orgID, map[string]any{"customer": sub.CustomerID})
	return nil
}

// BillingStatus returns the active org's billing state + current-day usage.
func (s *Services) BillingStatus(ctx context.Context) (*BillingStatusDTO, error) {
	orgID := s.tenant(ctx).OrgID
	dto := &BillingStatusDTO{Enabled: s.Billing.Enabled()}
	if c, err := s.Store.GetBillingCustomer(ctx, orgID); err == nil {
		dto.CustomerSet, dto.Status = true, c.Status
	}
	start := dayStart(time.Now().UTC())
	storage, _, _ := s.Store.OrgUsageTotals(ctx, orgID)
	egress, reqs, _ := s.Store.TrafficByOrg(ctx, orgID, start)
	dto.StorageBytes, dto.EgressBytes, dto.Requests = storage, egress, reqs
	dto.PeriodStartIS = start.Format(time.RFC3339)
	return dto, nil
}

// BillingReportOnce aggregates the current day's usage per billed org and posts it
// to the processor, once per (org, metric, day) via the idempotency ledger.
func (s *Services) BillingReportOnce(ctx context.Context) {
	if !s.Billing.Enabled() {
		return
	}
	orgs, err := s.Store.ListBillingOrgIDs(ctx)
	if err != nil {
		return
	}
	now := time.Now().UTC()
	start, end := dayStart(now), dayStart(now).Add(24*time.Hour)
	day := start.Format("2006-01-02")
	for _, orgID := range orgs {
		c, err := s.Store.GetBillingCustomer(ctx, orgID)
		if err != nil {
			continue
		}
		storageBytes, _, _ := s.Store.OrgUsageTotals(ctx, orgID)
		egressBytes, reqs, _ := s.Store.TrafficByOrg(ctx, orgID, start)
		s.reportMetric(ctx, orgID, "storage_gb_month", storageBytes/gib, c.StorageItemID, start, end, day)
		s.reportMetric(ctx, orgID, "egress_gb", egressBytes/gib, c.EgressItemID, start, end, day)
		s.reportMetric(ctx, orgID, "requests", reqs, c.RequestsItemID, start, end, day)
	}
}

// reportMetric records the usage idempotently, then posts to the processor only if
// the ledger row was new (so a re-run within the same day never double-bills).
func (s *Services) reportMetric(ctx context.Context, orgID, metric string, qty int64, itemID string, start, end time.Time, day string) {
	if itemID == "" {
		return
	}
	idem := fmt.Sprintf("%s:%s:%s", orgID, metric, day)
	fresh, err := s.Store.RecordUsageReport(ctx, orgID, metric, qty, start, end, idem)
	if err != nil || !fresh {
		return
	}
	if err := s.Billing.ReportUsage(ctx, itemID, qty, idem); err != nil {
		s.Logger.Warn("billing report failed", slog.String("metric", metric), slog.Any("error", err))
	}
}

// HandleBillingWebhook verifies + applies a processor webhook: non-payment suspends
// the org, recovery resumes it. Public endpoint (signature verified by the Provider).
func (s *Services) HandleBillingWebhook(ctx context.Context, payload []byte, sig string) error {
	ev, err := s.Billing.ParseWebhook(payload, sig)
	if err != nil {
		return &Error{Code: "invalid_webhook", Message: "could not verify webhook", HTTP: 400}
	}
	orgID := ev.OrgID
	if orgID == "" && ev.CustomerID != "" {
		if id, rerr := s.Store.OrgByStripeCustomer(ctx, ev.CustomerID); rerr == nil {
			orgID = id
		}
	}
	if orgID == "" {
		return nil // unknown customer; nothing to do (ack so the processor stops retrying)
	}
	switch ev.Type {
	case "payment_failed":
		_ = s.Store.SetBillingStatus(ctx, orgID, "past_due")
		_ = s.Store.SuspendOrg(ctx, orgID, "payment failed")
		// No request subject on the webhook path → audit() records a system actor.
		s.audit(ctx, "billing.suspend", "organization", orgID, map[string]any{"reason": "payment_failed"})
	case "payment_succeeded":
		_ = s.Store.SetBillingStatus(ctx, orgID, "active")
		_ = s.Store.ResumeOrg(ctx, orgID)
		s.audit(ctx, "billing.resume", "organization", orgID, nil)
	case "subscription_canceled":
		_ = s.Store.SetBillingStatus(ctx, orgID, "canceled")
	}
	return nil
}

// TriggerBillingReport runs one reporting pass now (platform admin / ops).
func (s *Services) TriggerBillingReport(ctx context.Context) error {
	if err := s.requirePlatformAdmin(ctx); err != nil {
		return err
	}
	if !s.Billing.Enabled() {
		return &Error{Code: "billing_disabled", Message: "billing is not enabled", HTTP: 404}
	}
	s.BillingReportOnce(ctx)
	return nil
}

// RunBillingReporter runs the usage reporter loop until ctx is cancelled.
func (s *Services) RunBillingReporter(ctx context.Context, interval time.Duration) {
	if !s.Billing.Enabled() {
		return
	}
	if interval <= 0 {
		interval = time.Hour
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	s.BillingReportOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.BillingReportOnce(ctx)
		}
	}
}

func dayStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
