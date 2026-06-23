package service

import (
	"context"

	"github.com/buktio/buktio/internal/entitlements"
)

// gatedFeatures are the paid features the UI gates on. Core capabilities here
// (multi-user, audit, backups, white-label) genuinely run in OSS; the
// enforcement/identity/billing ones are additionally ANDed with their actual
// wiring in Features() so OSS doesn't advertise capabilities it never wires.
var gatedFeatures = []entitlements.Feature{
	entitlements.FeatureRBACEnforced,
	entitlements.FeatureMultiUser,
	entitlements.FeatureSSO,
	entitlements.FeatureScheduledBackups,
	entitlements.FeatureAdvancedAudit,
	entitlements.FeatureWhiteLabel,
	entitlements.FeatureSCIM,
	entitlements.FeatureSelfServeSignup,
	entitlements.FeatureBilling,
}

// Features reports which gated features are actually AVAILABLE for the active
// tenant — the licence entitlement AND the capability being wired in this build.
// This matters because the OSS entitlements impl (AlwaysAllow) returns true for
// every feature, but OSS does not wire the enforcement/identity/billing
// capabilities (PermitAll authorizer, Disabled SSO/billing, no SCIM mount). Using
// the raw entitlement here would make OSS advertise SSO/SCIM/billing/enforced-RBAC
// it isn't running, so the UI would show controls that 404/400. Core capabilities
// (multi-user, audit, backups, white-label) genuinely run in OSS and stay true.
func (s *Services) Features(ctx context.Context) map[string]bool {
	org := s.tenant(ctx).OrgID
	out := make(map[string]bool, len(gatedFeatures)+1)
	for _, f := range gatedFeatures {
		allowed := s.Ent.Allowed(f, entitlements.TenantID(org)).Allowed
		switch f {
		case entitlements.FeatureRBACEnforced:
			allowed = allowed && s.Authz.Enforces() // PermitAll (OSS) => not enforced
		case entitlements.FeatureSSO:
			allowed = allowed && s.SSOEnabled() // only when an IdP is actually wired
		case entitlements.FeatureSCIM:
			allowed = allowed && s.SCIMEnabled // only when /scim/v2 is mounted
		case entitlements.FeatureBilling:
			allowed = allowed && s.Billing.Enabled() // Disabled (OSS) => off
		case entitlements.FeatureSelfServeSignup:
			allowed = allowed && s.signupEnabled() // config flag + entitlement
		}
		out[string(f)] = allowed
	}
	out["sso_configured"] = s.SSOEnabled()
	return out
}
