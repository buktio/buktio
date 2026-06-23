// Package tenancy is the per-tenant cluster-provisioning seam (Hosted). An org can
// run in one of two modes:
//
//   - pooled: the org shares a multi-tenant cluster (the primary). Isolation comes
//     from RLS + Garage's per-bucket access control. This is the default and the
//     only mode the core provides — it needs no new infrastructure.
//   - dedicated: the org gets its own cluster. Provisioning real infrastructure
//     (spinning up a Garage StatefulSet, a cloud account, etc.) lives in a closed
//     ee/cloud Provisioner injected at startup; the core falls back to pooled.
package tenancy

import "context"

// Mode is the tenancy mode for an org's storage.
type Mode string

const (
	ModePooled    Mode = "pooled"
	ModeDedicated Mode = "dedicated"
)

// Result is the outcome of provisioning an org's cluster.
type Result struct {
	// ClusterID is the cluster the org should use as its default.
	ClusterID string
	// Ready is true when the cluster is immediately usable; false means the org is
	// left in pending_setup and a control loop flips it to active once healthy.
	Ready bool
}

// Provisioner provisions (or selects) the storage cluster for an org.
type Provisioner interface {
	// Provision returns the cluster the org should use for the requested mode. It
	// must be idempotent — re-running for an already-provisioned org returns the
	// same cluster.
	Provision(ctx context.Context, orgID string, mode Mode, primaryClusterID string) (Result, error)
}

// Pooled is the core (OSS) Provisioner: every org maps to the shared primary
// cluster, ready immediately. It ignores ModeDedicated (the core can't stand up new
// infrastructure) and pools instead — callers that require true dedication inject a
// cloud Provisioner.
type Pooled struct{}

func (Pooled) Provision(_ context.Context, _ string, _ Mode, primaryClusterID string) (Result, error) {
	return Result{ClusterID: primaryClusterID, Ready: true}, nil
}
