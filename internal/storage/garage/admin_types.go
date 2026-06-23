package garage

// Garage Admin API v2 request/response shapes used by buktio. Field names follow
// the v2 docs; confirm against the live garage-admin-v2.json before production.

// --- Cluster status (GET /v2/GetClusterStatus) ---

type ClusterStatusResponse struct {
	LayoutVersion int        `json:"layoutVersion"`
	Nodes         []NodeResp `json:"nodes"`
}

type NodeResp struct {
	ID            string    `json:"id"`
	GarageVersion string    `json:"garageVersion"`
	Addr          string    `json:"addr"`
	Hostname      string    `json:"hostname"`
	IsUp          bool      `json:"isUp"`
	Draining      bool      `json:"draining"`
	Role          *NodeRole `json:"role"` // nil when the node has no layout role yet
	DataPartition *struct {
		Available int64 `json:"available"`
		Total     int64 `json:"total"`
	} `json:"dataPartition"`
}

// PrimaryNode returns the node to treat as "this node" for single-node bootstrap:
// it prefers an up node that already has a storage role, else the first up node.
func (r *ClusterStatusResponse) PrimaryNode() *NodeResp {
	var fallback *NodeResp
	for i := range r.Nodes {
		n := &r.Nodes[i]
		if n.Role != nil && n.Role.Capacity != nil {
			return n
		}
		if fallback == nil && n.IsUp {
			fallback = n
		}
	}
	if fallback != nil {
		return fallback
	}
	if len(r.Nodes) > 0 {
		return &r.Nodes[0]
	}
	return nil
}

type NodeRole struct {
	ID       string   `json:"id"`
	Zone     string   `json:"zone"`
	Capacity *int64   `json:"capacity"` // nil => gateway (not a storage node)
	Tags     []string `json:"tags"`
}

// --- Cluster health (GET /v2/GetClusterHealth) ---

type ClusterHealthResponse struct {
	Status           string `json:"status"`
	KnownNodes       int    `json:"knownNodes"`
	ConnectedNodes   int    `json:"connectedNodes"`
	StorageNodes     int    `json:"storageNodes"`
	StorageNodesOK   int    `json:"storageNodesUp"`
	Partitions       int    `json:"partitions"`
	PartitionsQuorum int    `json:"partitionsQuorum"`
	PartitionsAllOK  int    `json:"partitionsAllOk"`
}

// --- Cluster layout (GET /v2/GetClusterLayout) ---

type ClusterLayoutResponse struct {
	Version           int          `json:"version"`
	Roles             []LayoutRole `json:"roles"`
	StagedRoleChanges []LayoutRole `json:"stagedRoleChanges"`
}

type LayoutRole struct {
	ID       string   `json:"id"`
	Zone     string   `json:"zone"`
	Capacity *int64   `json:"capacity"`
	Tags     []string `json:"tags"`
}

// --- Update layout (POST /v2/UpdateClusterLayout) — stages role changes ---

type UpdateLayoutRequest struct {
	Roles []LayoutRole `json:"roles"`
}

// --- Apply layout (POST /v2/ApplyClusterLayout) — version must be current+1 ---

type ApplyLayoutRequest struct {
	Version int `json:"version"`
}

// --- Multi-node layout staging (v2) ---

// LayoutRoleChange is one staged role change: assign (id+zone+capacity+tags) or
// remove (id+remove:true). Used by UpdateClusterLayout for multi-node clusters.
type LayoutRoleChange struct {
	ID       string   `json:"id"`
	Zone     string   `json:"zone,omitempty"`
	Capacity *int64   `json:"capacity,omitempty"` // nil + Remove=false => gateway
	Tags     []string `json:"tags,omitempty"`
	Remove   bool     `json:"remove,omitempty"`
}

type UpdateLayoutChangesRequest struct {
	Roles []LayoutRoleChange `json:"roles"`
}

// --- Connect nodes (POST /v2/ConnectClusterNodes) ---
// Request body is a JSON array of peer addresses ("<node_id>@<host>:<port>").

type ConnectNodeResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

// --- Preview staged layout (POST /v2/PreviewClusterLayoutChanges) ---
// Best-effort shape; confirm against the live garage-admin-v2.json. On success the
// API returns a human-readable message (+ the computed new layout); on a rejected
// staging it returns an error string.

type LayoutPreviewResponse struct {
	Error     string                 `json:"error,omitempty"`
	Message   []string               `json:"message,omitempty"`
	NewLayout *ClusterLayoutResponse `json:"newLayout,omitempty"`
}

// --- Create key (POST /v2/CreateKey) — returns the secret ONCE ---

type CreateKeyRequest struct {
	Name  string    `json:"name"`
	Allow *KeyAllow `json:"allow,omitempty"`
}

type KeyAllow struct {
	CreateBucket bool `json:"createBucket"`
}

type CreateKeyResponse struct {
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
	Name            string `json:"name"`
}

// --- Bucket/key permission grant (POST /v2/AllowBucketKey) ---

type AllowBucketKeyRequest struct {
	BucketID    string         `json:"bucketId"`
	AccessKeyID string         `json:"accessKeyId"`
	Permissions KeyPermissions `json:"permissions"`
}

type KeyPermissions struct {
	Read  bool `json:"read"`
	Write bool `json:"write"`
	Owner bool `json:"owner"`
}
