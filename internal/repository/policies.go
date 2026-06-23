package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/buktio/buktio/internal/authz"
)

// Policy is a stored ABAC policy row (with its role bindings).
type Policy struct {
	ID        string
	Name      string
	Template  string
	Config    map[string]string
	Enabled   bool
	Roles     []string
	CreatedAt time.Time
}

// CreatePolicy inserts a policy + its role bindings and returns the id.
func (s *Store) CreatePolicy(ctx context.Context, orgID, name, template string, config map[string]string, roles []string) (string, error) {
	cfg, _ := json.Marshal(config)
	var id string
	if err := s.q(ctx).QueryRow(ctx,
		`INSERT INTO policies (org_id, name, template, config) VALUES ($1::uuid,$2,$3,$4::jsonb) RETURNING id::text`,
		orgID, name, template, string(cfg)).Scan(&id); err != nil {
		return "", fmt.Errorf("repository: create policy: %w", err)
	}
	for _, role := range roles {
		if _, err := s.q(ctx).Exec(ctx,
			`INSERT INTO role_policy_bindings (policy_id, role) VALUES ($1::uuid,$2::org_member_role)
			 ON CONFLICT DO NOTHING`, id, role); err != nil {
			return "", fmt.Errorf("repository: bind policy role: %w", err)
		}
	}
	return id, nil
}

// SetPolicyEnabled toggles a policy (org-scoped).
func (s *Store) SetPolicyEnabled(ctx context.Context, orgID, id string, enabled bool) error {
	ct, err := s.q(ctx).Exec(ctx,
		`UPDATE policies SET enabled=$3 WHERE id=$1::uuid AND org_id=$2::uuid`, id, orgID, enabled)
	if err != nil {
		return fmt.Errorf("repository: set policy enabled: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeletePolicy removes a policy (org-scoped); bindings cascade.
func (s *Store) DeletePolicy(ctx context.Context, orgID, id string) error {
	ct, err := s.q(ctx).Exec(ctx, `DELETE FROM policies WHERE id=$1::uuid AND org_id=$2::uuid`, id, orgID)
	if err != nil {
		return fmt.Errorf("repository: delete policy: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListPolicies returns an org's policies with their role bindings.
func (s *Store) ListPolicies(ctx context.Context, orgID string) ([]Policy, error) {
	rows, err := s.q(ctx).Query(ctx,
		`SELECT p.id::text, p.name, p.template, p.config, p.enabled, p.created_at,
		        COALESCE(array_agg(rpb.role::text) FILTER (WHERE rpb.role IS NOT NULL), '{}')
		   FROM policies p
		   LEFT JOIN role_policy_bindings rpb ON rpb.policy_id = p.id
		  WHERE p.org_id=$1::uuid
		  GROUP BY p.id
		  ORDER BY p.created_at DESC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("repository: list policies: %w", err)
	}
	defer rows.Close()
	var out []Policy
	for rows.Next() {
		var p Policy
		var cfg []byte
		if err := rows.Scan(&p.ID, &p.Name, &p.Template, &cfg, &p.Enabled, &p.CreatedAt, &p.Roles); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(cfg, &p.Config)
		out = append(out, p)
	}
	return out, rows.Err()
}

// EnabledPoliciesForAuthz returns an org's ENABLED policies as authz.Policy values
// for the ee ABAC evaluator.
func (s *Store) EnabledPoliciesForAuthz(ctx context.Context, orgID string) ([]authz.Policy, error) {
	rows, err := s.ListPolicies(ctx, orgID)
	if err != nil {
		return nil, err
	}
	var out []authz.Policy
	for _, p := range rows {
		if !p.Enabled {
			continue
		}
		roles := make([]authz.Role, 0, len(p.Roles))
		for _, r := range p.Roles {
			roles = append(roles, authz.Role(r))
		}
		out = append(out, authz.Policy{Name: p.Name, Template: p.Template, Roles: roles, Config: p.Config})
	}
	return out, nil
}
