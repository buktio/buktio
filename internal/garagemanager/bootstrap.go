package garagemanager

import (
	"context"
	"fmt"
	"time"

	"github.com/buktio/buktio/internal/storage/garage"
)

// Bootstrap defaults.
const (
	DefaultZone          = "dc1"
	DefaultSystemKeyName = "buktio-system"
	defaultCapacityBytes = 100 << 30 // 100 GiB advisory single-node capacity
)

// BootstrapParams configure the single-node bootstrap. The bootstrap is
// idempotent and restart-safe: pass ExistingSystemKeyID to skip re-creating the
// buktio-system key once it has been provisioned (buktio persists it in Postgres).
type BootstrapParams struct {
	Zone          string
	CapacityBytes int64
	SystemKeyName string
	// ExistingSystemKeyID, when set, means buktio already owns a system key — skip
	// creation. Empty means create one and capture its secret (shown once).
	ExistingSystemKeyID string

	HealthTimeout time.Duration
	PollInterval  time.Duration
}

func (p *BootstrapParams) setDefaults() {
	if p.Zone == "" {
		p.Zone = DefaultZone
	}
	if p.SystemKeyName == "" {
		p.SystemKeyName = DefaultSystemKeyName
	}
	if p.CapacityBytes <= 0 {
		p.CapacityBytes = defaultCapacityBytes
	}
	if p.HealthTimeout <= 0 {
		p.HealthTimeout = 2 * time.Minute
	}
	if p.PollInterval <= 0 {
		p.PollInterval = 2 * time.Second
	}
}

// BootstrapResult reports what the bootstrap observed/did. SystemSecretAccessKey
// is populated ONLY when a new system key was created (it is shown once).
type BootstrapResult struct {
	NodeID                string
	GarageVersion         string
	LayoutVersion         int
	LayoutApplied         bool
	SystemAccessKeyID     string
	SystemSecretAccessKey string
	SystemKeyCreated      bool
}

// Bootstrap brings a fresh single-node Garage to a usable state entirely over the
// Admin HTTP API (no Garage CLI):
//
//  1. wait for GET /health to report quorum,
//  2. read this node's id + version (GetClusterStatus) and version-guard,
//  3. if the node has no storage role: stage (UpdateClusterLayout) then apply
//     (ApplyClusterLayout, version = current+1) — skipped if a role already exists,
//  4. provision the buktio-system owner key, capturing its secret once.
//
// Every step is safe to re-run.
func Bootstrap(ctx context.Context, admin *garage.AdminClient, p BootstrapParams) (*BootstrapResult, error) {
	p.setDefaults()

	if err := waitHealthy(ctx, admin, p.HealthTimeout, p.PollInterval); err != nil {
		return nil, err
	}

	status, err := admin.GetClusterStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("bootstrap: get cluster status: %w", err)
	}
	node := status.PrimaryNode()
	if node == nil || node.ID == "" {
		return nil, fmt.Errorf("bootstrap: cluster status returned no usable node")
	}

	// Version guard: refuse Garage < 2.0 (Admin API v1 incompatible).
	if node.GarageVersion != "" {
		if v, perr := garage.ParseVersion(node.GarageVersion); perr == nil {
			if err := garage.CheckSupported(v); err != nil {
				return nil, err
			}
		}
	}

	res := &BootstrapResult{NodeID: node.ID, GarageVersion: node.GarageVersion}

	layout, err := admin.GetClusterLayout(ctx)
	if err != nil {
		return nil, fmt.Errorf("bootstrap: get cluster layout: %w", err)
	}
	res.LayoutVersion = layout.Version

	if !nodeHasStorageRole(layout, node.ID) {
		capacity := p.CapacityBytes
		role := garage.LayoutRole{ID: node.ID, Zone: p.Zone, Capacity: &capacity, Tags: []string{}}
		if err := admin.UpdateClusterLayout(ctx, []garage.LayoutRole{role}); err != nil {
			return nil, fmt.Errorf("bootstrap: stage layout: %w", err)
		}
		// Re-read for the authoritative current version, then apply current+1
		// (never blindly increment; never apply the same version twice).
		cur, err := admin.GetClusterLayout(ctx)
		if err != nil {
			return nil, fmt.Errorf("bootstrap: re-read layout: %w", err)
		}
		next := cur.Version + 1
		if err := admin.ApplyClusterLayout(ctx, next); err != nil {
			return nil, fmt.Errorf("bootstrap: apply layout v%d: %w", next, err)
		}
		res.LayoutVersion = next
		res.LayoutApplied = true
	}

	if p.ExistingSystemKeyID != "" {
		res.SystemAccessKeyID = p.ExistingSystemKeyID
	} else {
		key, err := admin.CreateKey(ctx, p.SystemKeyName, true)
		if err != nil {
			return nil, fmt.Errorf("bootstrap: create system key: %w", err)
		}
		res.SystemAccessKeyID = key.AccessKeyID
		res.SystemSecretAccessKey = key.SecretAccessKey // captured ONCE
		res.SystemKeyCreated = true
	}

	return res, nil
}

// waitHealthy polls GET /health until the engine reports quorum or the timeout
// elapses.
func waitHealthy(ctx context.Context, admin *garage.AdminClient, timeout, interval time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := admin.Health(ctx); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("bootstrap: storage engine not healthy within %s: %w", timeout, ctx.Err())
		case <-ticker.C:
		}
	}
}

// nodeHasStorageRole reports whether the node already has an applied storage role
// (a non-nil capacity; a nil capacity would be a gateway, not a storage node).
func nodeHasStorageRole(layout *garage.ClusterLayoutResponse, nodeID string) bool {
	for _, r := range layout.Roles {
		if r.ID == nodeID && r.Capacity != nil {
			return true
		}
	}
	return false
}
