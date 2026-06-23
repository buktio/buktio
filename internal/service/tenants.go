package service

import (
	"context"
	"net/http"

	"github.com/buktio/buktio/internal/auth"
	"github.com/buktio/buktio/internal/authz"
)

// OrgStatusDTO is the tenant lifecycle/quota view returned to platform admins.
type OrgStatusDTO struct {
	OrgID         string `json:"org_id"`
	Status        string `json:"status"`
	SuspendedAt   string `json:"suspended_at,omitempty"`
	SuspendReason string `json:"suspend_reason,omitempty"`
	QuotaMaxBytes *int64 `json:"quota_max_bytes"`
	BytesUsed     int64  `json:"bytes_used"`
}

// requirePlatformAdmin gates cross-org/platform operations on the operator flag.
func (s *Services) requirePlatformAdmin(ctx context.Context) *Error {
	subj, _ := authz.SubjectFrom(ctx)
	if !subj.PlatformAdmin {
		return &Error{Code: "forbidden", Message: "platform administrator privilege required", HTTP: http.StatusForbidden}
	}
	return nil
}

// GetOrgStatus returns an org's lifecycle + quota + current usage (platform admin).
func (s *Services) GetOrgStatus(ctx context.Context, orgID string) (*OrgStatusDTO, error) {
	if err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	st, err := s.Store.GetOrgStatus(ctx, orgID)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	used, _, _ := s.Store.OrgUsageTotals(ctx, orgID)
	dto := &OrgStatusDTO{
		OrgID: orgID, Status: st.Status, SuspendReason: st.SuspendReason,
		QuotaMaxBytes: st.QuotaMaxBytes, BytesUsed: used,
	}
	if st.SuspendedAt != nil {
		dto.SuspendedAt = st.SuspendedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	return dto, nil
}

// SuspendOrg suspends a tenant: its members' sessions are rejected until resumed.
func (s *Services) SuspendOrg(ctx context.Context, orgID, reason string) error {
	if err := s.requirePlatformAdmin(ctx); err != nil {
		return err
	}
	if err := s.Store.SuspendOrg(ctx, orgID, reason); err != nil {
		return mapRepoErr(err)
	}
	s.audit(ctx, "org.suspend", "organization", orgID, map[string]any{"reason": reason})
	return nil
}

// ResumeOrg returns a suspended tenant to active.
func (s *Services) ResumeOrg(ctx context.Context, orgID string) error {
	if err := s.requirePlatformAdmin(ctx); err != nil {
		return err
	}
	if err := s.Store.ResumeOrg(ctx, orgID); err != nil {
		return mapRepoErr(err)
	}
	s.audit(ctx, "org.resume", "organization", orgID, nil)
	return nil
}

// SetOrgQuota sets (maxBytes != nil) or clears (nil) an org's storage ceiling.
func (s *Services) SetOrgQuota(ctx context.Context, orgID string, maxBytes *int64) error {
	if err := s.requirePlatformAdmin(ctx); err != nil {
		return err
	}
	if maxBytes != nil && *maxBytes < 0 {
		return validationErr("quota_max_bytes must be >= 0")
	}
	if err := s.Store.SetOrgQuota(ctx, orgID, maxBytes); err != nil {
		return mapRepoErr(err)
	}
	s.audit(ctx, "org.set_quota", "organization", orgID, map[string]any{"quota_max_bytes": maxBytes})
	return nil
}

// OrgClusterDTO is an org→cluster assignment as the API returns it.
type OrgClusterDTO struct {
	ClusterID string `json:"cluster_id"`
	Name      string `json:"name"`
	Provider  string `json:"provider"`
	IsDefault bool   `json:"is_default"`
}

// ListOrgClusters returns the clusters assigned to an org (platform admin).
func (s *Services) ListOrgClusters(ctx context.Context, orgID string) ([]OrgClusterDTO, error) {
	if err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	rows, err := s.Store.ListOrgClusters(ctx, orgID)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	out := make([]OrgClusterDTO, 0, len(rows))
	for _, oc := range rows {
		out = append(out, OrgClusterDTO{ClusterID: oc.StorageClusterID, Name: oc.Name, Provider: oc.Provider, IsDefault: oc.IsDefault})
	}
	return out, nil
}

// AssignClusterToOrg pins a (registered) cluster to an org, optionally as its
// default so new buckets without an explicit cluster land there (platform admin).
func (s *Services) AssignClusterToOrg(ctx context.Context, orgID, clusterID string, makeDefault bool) error {
	if err := s.requirePlatformAdmin(ctx); err != nil {
		return err
	}
	if _, err := s.Store.GetOrgStatus(ctx, orgID); err != nil {
		return mapRepoErr(err)
	}
	if _, err := s.Store.GetClusterByID(ctx, clusterID); err != nil {
		return validationErr("unknown or invalid cluster_id")
	}
	if err := s.Store.AssignClusterToOrg(ctx, orgID, clusterID, makeDefault); err != nil {
		return mapRepoErr(err)
	}
	s.audit(ctx, "org.assign_cluster", "organization", orgID, map[string]any{"cluster_id": clusterID, "default": makeDefault})
	return nil
}

// UnassignClusterFromOrg removes an org→cluster mapping (platform admin).
func (s *Services) UnassignClusterFromOrg(ctx context.Context, orgID, clusterID string) error {
	if err := s.requirePlatformAdmin(ctx); err != nil {
		return err
	}
	if err := s.Store.UnassignClusterFromOrg(ctx, orgID, clusterID); err != nil {
		return mapRepoErr(err)
	}
	s.audit(ctx, "org.unassign_cluster", "organization", orgID, map[string]any{"cluster_id": clusterID})
	return nil
}

// SCIMTokenDTO is a provisioning-token row (no secret).
type SCIMTokenDTO struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	LastFour   string `json:"last_four,omitempty"`
	CreatedAt  string `json:"created_at"`
	LastUsedAt string `json:"last_used_at,omitempty"`
}

// CreateSCIMTokenResult carries the one-time raw token plus the stored row id.
type CreateSCIMTokenResult struct {
	ID    string `json:"id"`
	Token string `json:"token"` // shown once
}

// scimTokenActor gates SCIM-token management: an org owner or a platform admin.
func (s *Services) scimTokenActor(ctx context.Context) *Error {
	subj, _ := authz.SubjectFrom(ctx)
	if subj.PlatformAdmin || subj.Role == authz.RoleOwner {
		return nil
	}
	return &Error{Code: "forbidden", Message: "owner or platform administrator privilege required", HTTP: http.StatusForbidden}
}

// CreateSCIMToken mints a provisioning bearer token for the active org, returning
// the raw value exactly once.
func (s *Services) CreateSCIMToken(ctx context.Context, name string) (*CreateSCIMTokenResult, error) {
	if err := s.scimTokenActor(ctx); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, validationErr("name is required")
	}
	orgID := s.tenant(ctx).OrgID
	raw, err := auth.NewToken()
	if err != nil {
		return nil, mapRepoErr(err)
	}
	token := "bk_scim_" + raw
	lastFour := token[len(token)-4:]
	id, cerr := s.Store.CreateSCIMToken(ctx, orgID, name, auth.HashToken(token), lastFour)
	if cerr != nil {
		return nil, mapRepoErr(cerr)
	}
	s.audit(ctx, "scim.token_create", "scim_token", id, map[string]any{"name": name})
	return &CreateSCIMTokenResult{ID: id, Token: token}, nil
}

// ListSCIMTokens lists the active org's provisioning tokens (no secrets).
func (s *Services) ListSCIMTokens(ctx context.Context) ([]SCIMTokenDTO, error) {
	if err := s.scimTokenActor(ctx); err != nil {
		return nil, err
	}
	rows, err := s.Store.ListSCIMTokens(ctx, s.tenant(ctx).OrgID)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	out := make([]SCIMTokenDTO, 0, len(rows))
	for _, t := range rows {
		dto := SCIMTokenDTO{ID: t.ID, Name: t.Name, LastFour: t.LastFour, CreatedAt: t.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")}
		if t.LastUsedAt != nil {
			dto.LastUsedAt = t.LastUsedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
		}
		out = append(out, dto)
	}
	return out, nil
}

// RevokeSCIMToken revokes a provisioning token by id (active org).
func (s *Services) RevokeSCIMToken(ctx context.Context, id string) error {
	if err := s.scimTokenActor(ctx); err != nil {
		return err
	}
	if err := s.Store.RevokeSCIMToken(ctx, s.tenant(ctx).OrgID, id); err != nil {
		return mapRepoErr(err)
	}
	s.audit(ctx, "scim.token_revoke", "scim_token", id, nil)
	return nil
}

// OrgIsSuspended reports whether an org is currently suspended. Used by the auth
// middleware to reject a suspended tenant's sessions. Fails open (false) on a
// lookup error so a transient DB hiccup never locks every tenant out — RLS/app
// scoping still bound access, and the next request re-checks.
func (s *Services) OrgIsSuspended(ctx context.Context, orgID string) bool {
	if orgID == "" {
		return false
	}
	st, err := s.Store.GetOrgStatus(ctx, orgID)
	if err != nil {
		return false
	}
	return st.Status == "suspended"
}
