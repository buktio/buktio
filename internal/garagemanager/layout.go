package garagemanager

import (
	"context"
	"fmt"
	"strings"

	"github.com/buktio/buktio/internal/storage/garage"
)

// NodeSpec describes a node's desired storage role for layout staging.
type NodeSpec struct {
	NodeID        string
	Zone          string
	CapacityBytes int64 // > 0 for a storage node; 0 stages a gateway (no capacity)
	Tags          []string
}

// ConnectNode tells the cluster to dial a peer ("<node_id>@<host>:<port>"). It is
// idempotent; an already-connected peer is reported as success by Garage.
func ConnectNode(ctx context.Context, admin *garage.AdminClient, peer string) error {
	if !strings.Contains(peer, "@") {
		return fmt.Errorf("garagemanager: peer must be \"<node_id>@<host>:<port>\", got %q", peer)
	}
	results, err := admin.ConnectClusterNodes(ctx, []string{peer})
	if err != nil {
		return fmt.Errorf("garagemanager: connect node: %w", err)
	}
	for _, r := range results {
		if !r.Success {
			return fmt.Errorf("garagemanager: connect node %q failed: %s", peer, r.Error)
		}
	}
	return nil
}

// StageAndApplyRoles stages the given role assignments/removals and applies them at
// version current+1 (re-reading the authoritative version first, never double-
// applying). This is the shared "edit the layout" primitive for add/drain/remove.
func StageAndApplyRoles(ctx context.Context, admin *garage.AdminClient, changes []garage.LayoutRoleChange) (int, error) {
	if len(changes) == 0 {
		return 0, fmt.Errorf("garagemanager: no role changes to apply")
	}
	if err := admin.UpdateClusterLayoutChanges(ctx, changes); err != nil {
		return 0, fmt.Errorf("garagemanager: stage layout: %w", err)
	}
	cur, err := admin.GetClusterLayout(ctx)
	if err != nil {
		return 0, fmt.Errorf("garagemanager: re-read layout: %w", err)
	}
	next := cur.Version + 1
	if err := admin.ApplyClusterLayout(ctx, next); err != nil {
		return 0, fmt.Errorf("garagemanager: apply layout v%d: %w", next, err)
	}
	return next, nil
}

// AddOrUpdateNode connects the peer (if a peer address is given), then stages and
// applies its storage role. Pass peer="" when the node is already connected.
func AddOrUpdateNode(ctx context.Context, admin *garage.AdminClient, peer string, spec NodeSpec) (int, error) {
	if spec.NodeID == "" {
		return 0, fmt.Errorf("garagemanager: node id is required")
	}
	if peer != "" {
		if err := ConnectNode(ctx, admin, peer); err != nil {
			return 0, err
		}
	}
	change := garage.LayoutRoleChange{ID: spec.NodeID, Zone: spec.Zone, Tags: spec.Tags}
	if spec.CapacityBytes > 0 {
		cap := spec.CapacityBytes
		change.Capacity = &cap
	}
	return StageAndApplyRoles(ctx, admin, []garage.LayoutRoleChange{change})
}

// RemoveNode stages removal of a node's role and applies it (the node stops holding
// data after re-balancing). For a graceful drain, callers can first set capacity to
// a gateway role; Garage re-balances on apply either way.
func RemoveNode(ctx context.Context, admin *garage.AdminClient, nodeID string) (int, error) {
	if nodeID == "" {
		return 0, fmt.Errorf("garagemanager: node id is required")
	}
	return StageAndApplyRoles(ctx, admin, []garage.LayoutRoleChange{{ID: nodeID, Remove: true}})
}

// PreviewStaged returns the human-readable preview of currently staged changes.
func PreviewStaged(ctx context.Context, admin *garage.AdminClient) (*garage.LayoutPreviewResponse, error) {
	return admin.PreviewClusterLayoutChanges(ctx)
}

// RevertStaged drops all staged (un-applied) layout changes.
func RevertStaged(ctx context.Context, admin *garage.AdminClient) error {
	return admin.RevertClusterLayout(ctx)
}
