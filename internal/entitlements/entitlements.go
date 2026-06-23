// Package entitlements is the single place feature availability is decided.
//
// Call sites use Service.Allowed(feature, tenant) rather than branching on the
// edition, so adding a paid feature later is a matter of wrapping a new code path
// in a gate. In the OSS build the gate always returns "allowed" and limits are
// always "unlimited" — see AlwaysAllow.
package entitlements

// Feature is a stable identifier for a gateable capability.
type Feature string

// MVP features. Everything here is enabled in OSS. Paid-only features are added
// over time; the OSS entitlements impl still returns Allowed=true for them,
// because OSS ships every capability that has ever been free.
const (
	FeatureBuckets       Feature = "buckets"
	FeatureAccessKeys    Feature = "access_keys"
	FeatureObjectBrowser Feature = "object_browser"
	FeatureUsage         Feature = "usage"
	FeatureAudit         Feature = "audit"
	FeatureAPITokens     Feature = "api_tokens"
	// Reserved for future paid tiers (still allowed in OSS):
	FeatureMultiNode    Feature = "multi_node"
	FeatureSSO          Feature = "sso"
	FeatureRBACEnforced Feature = "rbac_enforced"
	// Paid features (gated by the license in paid builds; allowed in OSS):
	FeatureMultiUser        Feature = "multi_user"
	FeatureScheduledBackups Feature = "scheduled_backups"
	FeatureAdvancedAudit    Feature = "advanced_audit"
	FeatureSCIM             Feature = "scim"
	FeatureWhiteLabel       Feature = "white_label"
	FeatureSelfServeSignup  Feature = "self_serve_signup"
	FeatureBilling          Feature = "billing"
)

// Resource is a quantity that may carry an edition-dependent limit.
type Resource string

const (
	ResourceTenants    Resource = "tenants"
	ResourceProjects   Resource = "projects"
	ResourceBuckets    Resource = "buckets"
	ResourceAccessKeys Resource = "access_keys"
	ResourceSeats      Resource = "seats"      // org members (paid seat cap)
	ResourceStorageGB  Resource = "storage_gb" // per-tenant storage cap
)

// Decision is the outcome of an entitlement check.
type Decision struct {
	Allowed bool
	// Reason is a human/machine-readable explanation when Allowed is false.
	Reason string
}

// Limit describes a numeric allowance. Unlimited == true means no cap.
type Limit struct {
	Unlimited bool
	Max       int64
}

// TenantID scopes entitlement decisions. In OSS there is a single default tenant.
type TenantID string

// Service decides feature availability and resource limits.
type Service interface {
	Allowed(feature Feature, tenant TenantID) Decision
	Limit(resource Resource, tenant TenantID) Limit
}
