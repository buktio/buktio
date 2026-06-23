package service

import (
	"context"

	"github.com/buktio/buktio/internal/authz"
)

// PolicyDTO is an ABAC policy as the API returns it.
type PolicyDTO struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Template  string            `json:"template"`
	Config    map[string]string `json:"config"`
	Enabled   bool              `json:"enabled"`
	Roles     []string          `json:"roles"`
	CreatedAt string            `json:"created_at"`
}

// policyActor gates policy management: an org owner or a platform admin.
func (s *Services) policyActor(ctx context.Context) *Error {
	subj, _ := authz.SubjectFrom(ctx)
	if subj.PlatformAdmin || subj.Role == authz.RoleOwner {
		return nil
	}
	return &Error{Code: "forbidden", Message: "owner or platform administrator privilege required", HTTP: 403}
}

// ListPolicies returns the active org's ABAC policies.
func (s *Services) ListPolicies(ctx context.Context) ([]PolicyDTO, error) {
	if err := s.policyActor(ctx); err != nil {
		return nil, err
	}
	rows, err := s.Store.ListPolicies(ctx, s.tenant(ctx).OrgID)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	out := make([]PolicyDTO, 0, len(rows))
	for _, p := range rows {
		out = append(out, PolicyDTO{
			ID: p.ID, Name: p.Name, Template: p.Template, Config: p.Config,
			Enabled: p.Enabled, Roles: p.Roles,
			CreatedAt: p.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	return out, nil
}

// CreatePolicyInput is the create payload.
type CreatePolicyInput struct {
	Name     string
	Template string
	Config   map[string]string
	Roles    []string
}

// validPolicyTemplates mirrors ee/authz built-ins (kept here so OSS validates too).
var validPolicyTemplates = map[string]bool{"ip_allowlist": true, "business_hours": true, "read_only": true}

// CreatePolicy creates an ABAC policy bound to the given roles.
func (s *Services) CreatePolicy(ctx context.Context, in CreatePolicyInput) (string, error) {
	if err := s.policyActor(ctx); err != nil {
		return "", err
	}
	if in.Name == "" {
		return "", validationErr("name is required")
	}
	if !validPolicyTemplates[in.Template] {
		return "", validationErr("unknown policy template")
	}
	for _, r := range in.Roles {
		switch authz.Role(r) {
		case authz.RoleOwner:
			return "", validationErr("owners cannot be bound to a policy (no lock-out)")
		case authz.RoleAdmin, authz.RoleMember, authz.RoleViewer:
		default:
			return "", validationErr("invalid role: " + r)
		}
	}
	id, err := s.Store.CreatePolicy(ctx, s.tenant(ctx).OrgID, in.Name, in.Template, in.Config, in.Roles)
	if err != nil {
		return "", mapRepoErr(err)
	}
	s.audit(ctx, "policy.create", "policy", id, map[string]any{"template": in.Template, "roles": in.Roles})
	return id, nil
}

// SetPolicyEnabled enables/disables a policy.
func (s *Services) SetPolicyEnabled(ctx context.Context, id string, enabled bool) error {
	if err := s.policyActor(ctx); err != nil {
		return err
	}
	if err := s.Store.SetPolicyEnabled(ctx, s.tenant(ctx).OrgID, id, enabled); err != nil {
		return mapRepoErr(err)
	}
	s.audit(ctx, "policy.set_enabled", "policy", id, map[string]any{"enabled": enabled})
	return nil
}

// DeletePolicy removes a policy.
func (s *Services) DeletePolicy(ctx context.Context, id string) error {
	if err := s.policyActor(ctx); err != nil {
		return err
	}
	if err := s.Store.DeletePolicy(ctx, s.tenant(ctx).OrgID, id); err != nil {
		return mapRepoErr(err)
	}
	s.audit(ctx, "policy.delete", "policy", id, nil)
	return nil
}
