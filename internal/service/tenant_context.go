package service

import (
	"context"

	"github.com/buktio/buktio/internal/authz"
)

// TenantContext is the active org/project/cluster for a request. It is set by the
// auth middleware (from the authenticated user's membership) and read by service
// methods via s.tenant(ctx). When absent, s.tenant falls back to the boot-time
// default tenant — so single-tenant OSS behavior is unchanged.
type TenantContext struct {
	OrgID     string
	ProjectID string
	ClusterID string
}

type tenantCtxKey struct{}

// WithTenant attaches the active tenant to the context.
func WithTenant(ctx context.Context, t TenantContext) context.Context {
	return context.WithValue(ctx, tenantCtxKey{}, t)
}

// TenantFrom returns the active tenant, if set.
func TenantFrom(ctx context.Context) (TenantContext, bool) {
	t, ok := ctx.Value(tenantCtxKey{}).(TenantContext)
	return t, ok
}

// tenant resolves the active tenant for a request, falling back to the boot-time
// default (OSS single tenant) when no per-request tenant is set.
func (s *Services) tenant(ctx context.Context) TenantContext {
	if t, ok := TenantFrom(ctx); ok && t.OrgID != "" {
		if t.ClusterID == "" {
			t.ClusterID = s.ClusterID
		}
		return t
	}
	return TenantContext{OrgID: s.OrgID, ProjectID: s.ProjectID, ClusterID: s.ClusterID}
}

// ResolveSubject resolves the authz Subject (user, active org, role) and the active
// TenantContext for a request. The role comes from the user's org membership; with
// no membership it falls back to the platform-admin heuristic (owner) or admin — so
// the OSS single admin stays a superuser under PermitAll.
func (s *Services) ResolveSubject(ctx context.Context, userID string, isPlatformAdmin bool) (authz.Subject, TenantContext) {
	orgID, projectID, role, err := s.Store.ResolveUserTenant(ctx, userID, s.OrgID, s.ProjectID)
	if err != nil || orgID == "" {
		orgID, projectID, role = s.OrgID, s.ProjectID, ""
	}
	r := authz.Role(role)
	if r == "" {
		r = authz.RoleAdmin
		if isPlatformAdmin {
			r = authz.RoleOwner
		}
	}
	t := TenantContext{OrgID: orgID, ProjectID: projectID, ClusterID: s.ClusterID}
	return authz.Subject{UserID: userID, TenantID: orgID, Role: r, PlatformAdmin: isPlatformAdmin}, t
}
