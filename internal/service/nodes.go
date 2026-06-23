package service

import (
	"context"
	"time"

	"github.com/buktio/buktio/internal/authz"
	"github.com/buktio/buktio/internal/garagemanager"
	"github.com/buktio/buktio/internal/repository"
	"github.com/buktio/buktio/internal/storage"
	"github.com/buktio/buktio/internal/storage/garage"
)

// NodeDTO is one cluster node as the API returns it.
type NodeDTO struct {
	ID             string `json:"id"` // garage node id
	Hostname       string `json:"hostname"`
	Addr           string `json:"addr"`
	Zone           string `json:"zone"`
	Role           string `json:"role"` // storage | gateway | ""
	IsUp           bool   `json:"is_up"`
	Draining       bool   `json:"draining"`
	CapacityBytes  *int64 `json:"capacity_bytes"`
	DiskTotalBytes int64  `json:"disk_total_bytes"`
	DiskAvailBytes int64  `json:"disk_avail_bytes"`
}

// LayoutRoleDTO is one role in the cluster layout.
type LayoutRoleDTO struct {
	ID       string   `json:"id"`
	Zone     string   `json:"zone"`
	Capacity *int64   `json:"capacity"`
	Tags     []string `json:"tags"`
}

// LayoutDTO is the applied layout plus any staged (un-applied) changes.
type LayoutDTO struct {
	Version int             `json:"version"`
	Roles   []LayoutRoleDTO `json:"roles"`
	Staged  []LayoutRoleDTO `json:"staged_role_changes"`
}

// AddNodeInput stages a node's storage role (and optionally connects the peer).
type AddNodeInput struct {
	NodeID        string // garage node id (required)
	Peer          string // optional "<node_id>@<host>:<port>" to connect first
	Zone          string // layout zone (default "dc1")
	CapacityBytes int64  // > 0 storage node; 0 => gateway
}

func nodeStatusToDTO(n storage.NodeStatus) NodeDTO {
	return NodeDTO{
		ID: n.ID, Hostname: n.Hostname, Addr: n.Addr, Zone: n.Zone, Role: n.Role,
		IsUp: n.IsUp, Draining: n.Draining, CapacityBytes: n.CapacityBytes,
		DiskTotalBytes: n.DiskUsed + n.DiskAvail, DiskAvailBytes: n.DiskAvail,
	}
}

// adminForGarageCluster builds an Admin API client for a Garage cluster, decrypting
// its admin token. Returns unsupportedErr for non-Garage backends.
func (s *Services) adminForGarageCluster(ctx context.Context, clusterID string) (*garage.AdminClient, *repository.Cluster, error) {
	c, err := s.Store.GetClusterByID(ctx, clusterID)
	if err != nil {
		return nil, nil, mapRepoErr(err)
	}
	if c.Provider != garage.Kind || len(c.AdminTokenEnc) == 0 {
		return nil, nil, unsupportedErr()
	}
	tok, err := s.Sealer.Open(c.AdminTokenEnc)
	if err != nil {
		return nil, nil, mapRepoErr(err)
	}
	return garage.NewAdminClient(c.AdminEndpoint, string(tok)), c, nil
}

// ListNodes returns a cluster's live node topology (and refreshes the snapshot).
func (s *Services) ListNodes(ctx context.Context, clusterID string) ([]NodeDTO, error) {
	if _, err := s.Store.GetClusterByID(ctx, clusterID); err != nil {
		return nil, mapRepoErr(err)
	}
	prov, perr := s.providerFor(ctx, clusterID)
	if perr != nil {
		return nil, storageUnavailableErr("cannot reach the cluster: " + perr.Error())
	}
	if !prov.Capabilities().HasClusterHealth {
		return nil, unsupportedErr() // generic-S3 backends have no node topology
	}
	st, err := prov.GetClusterStatus(ctx)
	if err != nil {
		return s.nodesFromDB(ctx, clusterID) // fall back to last reconciled snapshot
	}
	s.upsertNodes(ctx, clusterID, st)
	out := make([]NodeDTO, 0, len(st.Nodes))
	for _, n := range st.Nodes {
		out = append(out, nodeStatusToDTO(n))
	}
	return out, nil
}

func (s *Services) nodesFromDB(ctx context.Context, clusterID string) ([]NodeDTO, error) {
	rows, err := s.Store.ListNodesByCluster(ctx, clusterID)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	out := make([]NodeDTO, 0, len(rows))
	for _, n := range rows {
		dto := NodeDTO{
			ID: n.GarageNodeID, Hostname: n.Hostname, Addr: n.Addr, Zone: n.Zone,
			Role: n.Role, IsUp: n.IsUp, Draining: n.Draining, CapacityBytes: n.CapacityBytes,
		}
		if n.DiskTotalBytes != nil {
			dto.DiskTotalBytes = *n.DiskTotalBytes
		}
		if n.DiskAvailBytes != nil {
			dto.DiskAvailBytes = *n.DiskAvailBytes
		}
		out = append(out, dto)
	}
	return out, nil
}

// AddNode connects + stages + applies a storage role for a node (Garage only).
func (s *Services) AddNode(ctx context.Context, clusterID string, in AddNodeInput) error {
	if err := s.authorize(ctx, authz.ActionUpdate, authz.Target{Kind: authz.ResourceCluster, ID: clusterID}); err != nil {
		return err
	}
	if in.NodeID == "" {
		return validationErr("node id is required")
	}
	admin, _, err := s.adminForGarageCluster(ctx, clusterID)
	if err != nil {
		return err
	}
	if in.Zone == "" {
		in.Zone = "dc1"
	}
	if _, gerr := garagemanager.AddOrUpdateNode(ctx, admin, in.Peer, garagemanager.NodeSpec{
		NodeID: in.NodeID, Zone: in.Zone, CapacityBytes: in.CapacityBytes,
	}); gerr != nil {
		return mapStorageErr(gerr)
	}
	s.audit(ctx, "cluster.node.add", "cluster", clusterID, map[string]any{"node_id": in.NodeID, "zone": in.Zone})
	s.reconcileCluster(ctx, clusterID)
	return nil
}

// RemoveNode stages removal of a node's role and applies it (Garage only).
func (s *Services) RemoveNode(ctx context.Context, clusterID, nodeID string) error {
	if err := s.authorize(ctx, authz.ActionUpdate, authz.Target{Kind: authz.ResourceCluster, ID: clusterID}); err != nil {
		return err
	}
	admin, _, err := s.adminForGarageCluster(ctx, clusterID)
	if err != nil {
		return err
	}
	if _, gerr := garagemanager.RemoveNode(ctx, admin, nodeID); gerr != nil {
		return mapStorageErr(gerr)
	}
	s.audit(ctx, "cluster.node.remove", "cluster", clusterID, map[string]any{"node_id": nodeID})
	s.reconcileCluster(ctx, clusterID)
	return nil
}

// GetLayout returns a Garage cluster's applied + staged layout.
func (s *Services) GetLayout(ctx context.Context, clusterID string) (*LayoutDTO, error) {
	admin, _, err := s.adminForGarageCluster(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	l, gerr := admin.GetClusterLayout(ctx)
	if gerr != nil {
		return nil, mapStorageErr(gerr)
	}
	return &LayoutDTO{
		Version: l.Version,
		Roles:   layoutRolesToDTO(l.Roles),
		Staged:  layoutRolesToDTO(l.StagedRoleChanges),
	}, nil
}

// PreviewLayout returns a human-readable preview of staged changes.
func (s *Services) PreviewLayout(ctx context.Context, clusterID string) ([]string, error) {
	admin, _, err := s.adminForGarageCluster(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	p, gerr := garagemanager.PreviewStaged(ctx, admin)
	if gerr != nil {
		return nil, mapStorageErr(gerr)
	}
	if p.Error != "" {
		return nil, validationErr(p.Error)
	}
	return p.Message, nil
}

// RevertLayout drops staged (un-applied) layout changes.
func (s *Services) RevertLayout(ctx context.Context, clusterID string) error {
	if err := s.authorize(ctx, authz.ActionUpdate, authz.Target{Kind: authz.ResourceCluster, ID: clusterID}); err != nil {
		return err
	}
	admin, _, err := s.adminForGarageCluster(ctx, clusterID)
	if err != nil {
		return err
	}
	if gerr := garagemanager.RevertStaged(ctx, admin); gerr != nil {
		return mapStorageErr(gerr)
	}
	s.audit(ctx, "cluster.layout.revert", "cluster", clusterID, nil)
	return nil
}

func layoutRolesToDTO(roles []garage.LayoutRole) []LayoutRoleDTO {
	out := make([]LayoutRoleDTO, 0, len(roles))
	for _, r := range roles {
		out = append(out, LayoutRoleDTO{ID: r.ID, Zone: r.Zone, Capacity: r.Capacity, Tags: r.Tags})
	}
	return out
}

// --- reconciler ---

func (s *Services) upsertNodes(ctx context.Context, clusterID string, st *storage.ClusterStatus) {
	keep := make([]string, 0, len(st.Nodes))
	for _, n := range st.Nodes {
		var totalPtr, availPtr *int64
		if total := n.DiskUsed + n.DiskAvail; total > 0 {
			t := total
			totalPtr = &t
		}
		if n.DiskAvail > 0 {
			a := n.DiskAvail
			availPtr = &a
		}
		_ = s.Store.UpsertNode(ctx, repository.StorageNode{
			ClusterID: clusterID, GarageNodeID: n.ID, Hostname: n.Hostname, Addr: n.Addr,
			Zone: n.Zone, CapacityBytes: n.CapacityBytes, Role: n.Role, IsUp: n.IsUp,
			Draining: n.Draining, DiskTotalBytes: totalPtr, DiskAvailBytes: availPtr,
		})
		keep = append(keep, n.ID)
	}
	_ = s.Store.DeleteStaleNodes(ctx, clusterID, keep)
}

func (s *Services) reconcileCluster(ctx context.Context, clusterID string) {
	prov, perr := s.providerFor(ctx, clusterID)
	if perr != nil {
		return
	}
	if !prov.Capabilities().HasClusterHealth {
		return
	}
	if st, err := prov.GetClusterStatus(ctx); err == nil {
		s.upsertNodes(ctx, clusterID, st)
	}
}

// ReconcileNodesOnce refreshes storage_nodes for every cluster with node topology.
func (s *Services) ReconcileNodesOnce(ctx context.Context) {
	clusters, err := s.Store.ListClusters(ctx)
	if err != nil {
		return
	}
	for i := range clusters {
		s.reconcileCluster(ctx, clusters[i].ID)
	}
}

// RunNodeReconciler runs the node reconciliation loop until ctx is cancelled.
func (s *Services) RunNodeReconciler(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 2 * time.Minute
	}
	s.ReconcileNodesOnce(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.ReconcileNodesOnce(ctx)
		}
	}
}
