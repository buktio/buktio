package garage

import (
	"context"
	"net/http"
)

// GetClusterStatus returns node list, this node's id, the Garage version, and the
// current layout version.
func (c *AdminClient) GetClusterStatus(ctx context.Context) (*ClusterStatusResponse, error) {
	var out ClusterStatusResponse
	if err := c.doJSON(ctx, http.MethodGet, "/v2/GetClusterStatus", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetClusterHealth returns cluster health (status, nodes, partition quorum).
func (c *AdminClient) GetClusterHealth(ctx context.Context) (*ClusterHealthResponse, error) {
	var out ClusterHealthResponse
	if err := c.doJSON(ctx, http.MethodGet, "/v2/GetClusterHealth", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetClusterLayout returns the current (applied) layout plus any staged changes.
func (c *AdminClient) GetClusterLayout(ctx context.Context) (*ClusterLayoutResponse, error) {
	var out ClusterLayoutResponse
	if err := c.doJSON(ctx, http.MethodGet, "/v2/GetClusterLayout", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateClusterLayout stages role changes (does not apply them).
func (c *AdminClient) UpdateClusterLayout(ctx context.Context, roles []LayoutRole) error {
	return c.doJSON(ctx, http.MethodPost, "/v2/UpdateClusterLayout",
		UpdateLayoutRequest{Roles: roles}, nil)
}

// ApplyClusterLayout applies staged changes. version MUST equal current+1.
func (c *AdminClient) ApplyClusterLayout(ctx context.Context, version int) error {
	return c.doJSON(ctx, http.MethodPost, "/v2/ApplyClusterLayout",
		ApplyLayoutRequest{Version: version}, nil)
}

// UpdateClusterLayoutChanges stages role assignments AND removals (multi-node).
func (c *AdminClient) UpdateClusterLayoutChanges(ctx context.Context, changes []LayoutRoleChange) error {
	return c.doJSON(ctx, http.MethodPost, "/v2/UpdateClusterLayout",
		UpdateLayoutChangesRequest{Roles: changes}, nil)
}

// ConnectClusterNodes asks this node to connect to the given peers
// ("<node_id>@<host>:<port>"). Returns per-peer results.
func (c *AdminClient) ConnectClusterNodes(ctx context.Context, peers []string) ([]ConnectNodeResult, error) {
	var out []ConnectNodeResult
	if err := c.doJSON(ctx, http.MethodPost, "/v2/ConnectClusterNodes", peers, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// PreviewClusterLayoutChanges computes the effect of the currently staged changes
// without applying them (a dry run for the UI's preview → apply flow).
func (c *AdminClient) PreviewClusterLayoutChanges(ctx context.Context) (*LayoutPreviewResponse, error) {
	var out LayoutPreviewResponse
	if err := c.doJSON(ctx, http.MethodPost, "/v2/PreviewClusterLayoutChanges", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RevertClusterLayout drops all currently staged (un-applied) role changes.
func (c *AdminClient) RevertClusterLayout(ctx context.Context) error {
	return c.doJSON(ctx, http.MethodPost, "/v2/RevertClusterLayout", nil, nil)
}

// CreateKey creates an S3 access key and returns its secret ONCE.
func (c *AdminClient) CreateKey(ctx context.Context, name string, canCreateBucket bool) (*CreateKeyResponse, error) {
	req := CreateKeyRequest{Name: name}
	if canCreateBucket {
		req.Allow = &KeyAllow{CreateBucket: true}
	}
	var out CreateKeyResponse
	if err := c.doJSON(ctx, http.MethodPost, "/v2/CreateKey", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AllowBucketKey grants read/write/owner permissions to a key on a bucket.
func (c *AdminClient) AllowBucketKey(ctx context.Context, bucketID, accessKeyID string, p KeyPermissions) error {
	return c.doJSON(ctx, http.MethodPost, "/v2/AllowBucketKey",
		AllowBucketKeyRequest{BucketID: bucketID, AccessKeyID: accessKeyID, Permissions: p}, nil)
}
