package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/buktio/buktio/internal/tenancy"
)

// TenantClusterResult reports an org's cluster assignment after provisioning.
type TenantClusterResult struct {
	OrgID     string `json:"org_id"`
	ClusterID string `json:"cluster_id"`
	Mode      string `json:"mode"`
	Status    string `json:"status"` // active | pending_setup
}

// AssignTenantCluster provisions/selects an org's storage cluster (platform admin).
// pooled maps the org to the shared primary cluster; dedicated delegates to the
// injected Provisioner (the core Pooled one falls back to shared). When the cluster
// is not immediately ready the org is left pending_setup for the control loop.
func (s *Services) AssignTenantCluster(ctx context.Context, orgID string, mode string) (*TenantClusterResult, error) {
	if err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	if _, err := s.Store.GetOrgStatus(ctx, orgID); err != nil {
		return nil, mapRepoErr(err)
	}
	m := tenancy.Mode(mode)
	if m != tenancy.ModePooled && m != tenancy.ModeDedicated {
		return nil, validationErr("mode must be 'pooled' or 'dedicated'")
	}

	res, err := s.Provisioner.Provision(ctx, orgID, m, s.ClusterID)
	if err != nil {
		return nil, &Error{Code: "provision_failed", Message: err.Error(), HTTP: 502}
	}
	clusterID := res.ClusterID
	if clusterID == "" {
		clusterID = s.ClusterID
	}
	if err := s.Store.AssignClusterToOrg(ctx, orgID, clusterID, true); err != nil {
		return nil, mapRepoErr(err)
	}
	status := "pending_setup"
	if res.Ready {
		status = "active"
	}
	if err := s.Store.SetOrgStatus(ctx, orgID, status); err != nil {
		return nil, mapRepoErr(err)
	}
	s.audit(ctx, "org.assign_tenant_cluster", "organization", orgID, map[string]any{"mode": mode, "cluster_id": clusterID, "status": status})
	return &TenantClusterResult{OrgID: orgID, ClusterID: clusterID, Mode: mode, Status: status}, nil
}

// RunTenantControlLoop flips pending_setup orgs to active once their default
// cluster reports healthy. Dedicated provisioning (cloud) leaves orgs in
// pending_setup until the new cluster comes up; this loop completes the handoff.
func (s *Services) RunTenantControlLoop(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.reconcilePendingOrgs(ctx)
		}
	}
}

func (s *Services) reconcilePendingOrgs(ctx context.Context) {
	orgs, err := s.Store.ListOrgIDsByStatus(ctx, "pending_setup")
	if err != nil {
		return
	}
	for _, orgID := range orgs {
		if s.Reg == nil {
			continue
		}
		prov, clusterID, perr := s.Reg.ProviderForOrg(ctx, orgID)
		if perr != nil || clusterID == "" {
			continue
		}
		// Healthy => promote. Generic-S3 backends report no cluster health; treat a
		// successful provider build as ready for those.
		ready := true
		if h, herr := prov.GetClusterHealth(ctx); herr == nil {
			ready = h.Status == "healthy" || h.StorageNodesOK > 0
		}
		if ready {
			if err := s.Store.SetOrgStatus(ctx, orgID, "active"); err == nil {
				s.Logger.Info("tenant org activated", slog.String("org", orgID), slog.String("cluster", clusterID))
			}
		}
	}
}
