// Package authz is the access-control seam.
//
// Every sensitive handler calls Authorizer.Can(subject, action, resource). The
// roles and call sites exist from day one so RBAC enforcement can be switched on
// in paid editions with no handler rewrites. The OSS authorizer is PermitAll:
// the single admin is effectively a superuser, but identity is still threaded so
// the audit log and a future enforcing authorizer drop in cleanly.
package authz

import "time"

// Role is a coarse role assigned to a user within a tenant/project.
type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
	RoleViewer Role = "viewer"
)

// Action is a verb performed on a resource.
type Action string

const (
	ActionRead   Action = "read"
	ActionCreate Action = "create"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"
)

// ResourceKind identifies what is being acted upon, e.g. "bucket", "access_key".
type ResourceKind string

const (
	ResourceBucket    ResourceKind = "bucket"
	ResourceAccessKey ResourceKind = "access_key"
	ResourceProject   ResourceKind = "project"
	ResourceCluster   ResourceKind = "cluster"
	ResourceSystem    ResourceKind = "system"
	ResourceUser      ResourceKind = "user"
	ResourceObject    ResourceKind = "object"
	ResourceMember    ResourceKind = "member"
	ResourceAudit     ResourceKind = "audit"
	ResourceBackup    ResourceKind = "backup"
	ResourceSettings  ResourceKind = "settings"
	ResourceAPIToken  ResourceKind = "api_token"
)

// Subject is the actor a decision is made about.
type Subject struct {
	UserID   string
	TenantID string
	Role     Role
	// PlatformAdmin marks the instance operator (the SaaS/self-host superadmin),
	// distinct from an org role. Cross-org/platform operations (tenant suspend,
	// per-org cluster assignment) require it.
	PlatformAdmin bool
	// ABAC attributes (Enterprise). Zero values mean "unknown" and are ignored by
	// policy conditions, so OSS/Pro (which never set them) are unaffected.
	IP  string    // request client IP, for ip_allowlist policies
	Now time.Time // request time, for business_hours policies
}

// Target is the resource an action is performed on.
type Target struct {
	Kind      ResourceKind
	ID        string
	ProjectID string
	// Attributes carries resource attributes (e.g. bucket tags) for ABAC policies.
	Attributes map[string]string
}

// Policy is an attribute-based rule layered on top of the RBAC matrix (Enterprise).
// It binds a built-in Template (evaluated with Config) to one or more Roles; an
// enforcing authorizer denies a request that an applicable policy rejects, even
// when the role matrix would allow it. Owners are exempt so a policy can't lock the
// org out. This type lives in core so the repository can carry policies to the ee
// evaluator; OSS never constructs any.
type Policy struct {
	Name     string
	Template string
	Roles    []Role
	Config   map[string]string
}

// Decision is the outcome of an authorization check.
type Decision struct {
	Allowed bool
	Reason  string
}

// Authorizer makes access-control decisions.
type Authorizer interface {
	Can(subject Subject, action Action, target Target) Decision
	// Enforces reports whether this authorizer actually restricts access. The OSS
	// PermitAll returns false (it allows everything); the paid enforcing authorizers
	// return true. The /auth/me features map uses this so OSS doesn't advertise
	// "enforced RBAC" it isn't running.
	Enforces() bool
}
