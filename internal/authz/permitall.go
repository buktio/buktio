package authz

// PermitAll is the OSS authorizer: every action is allowed. Identity is still
// carried on the Subject so audit logging works and a future enforcing
// authorizer can be swapped in without changing any call site.
type PermitAll struct{}

// NewPermitAll returns the OSS authorizer.
func NewPermitAll() Authorizer { return PermitAll{} }

func (PermitAll) Can(Subject, Action, Target) Decision {
	return Decision{Allowed: true}
}

// Enforces is false: PermitAll restricts nothing.
func (PermitAll) Enforces() bool { return false }
