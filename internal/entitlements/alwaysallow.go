package entitlements

// AlwaysAllow is the OSS implementation: every feature is allowed and every limit
// is unlimited. It performs no network calls and emits no telemetry.
type AlwaysAllow struct{}

// NewAlwaysAllow returns the OSS entitlements service.
func NewAlwaysAllow() Service { return AlwaysAllow{} }

func (AlwaysAllow) Allowed(Feature, TenantID) Decision {
	return Decision{Allowed: true}
}

func (AlwaysAllow) Limit(Resource, TenantID) Limit {
	return Limit{Unlimited: true}
}
