package repository

import (
	"context"
	"fmt"
)

// StorageNode is a projection of one Garage node (reconciled from GetClusterStatus).
type StorageNode struct {
	ID             string
	ClusterID      string
	GarageNodeID   string
	Hostname       string
	Addr           string
	Zone           string
	CapacityBytes  *int64
	Role           string
	IsUp           bool
	Draining       bool
	DiskTotalBytes *int64
	DiskAvailBytes *int64
}

// UpsertNode inserts or updates a node by (cluster, garage_node_id).
func (s *Store) UpsertNode(ctx context.Context, n StorageNode) error {
	_, err := s.q(ctx).Exec(ctx, `
INSERT INTO storage_nodes
  (storage_cluster_id, garage_node_id, hostname, addr, zone, capacity_bytes, role,
   is_up, draining, disk_total_bytes, disk_avail_bytes, last_seen_at)
VALUES ($1::uuid, $2, NULLIF($3,''), NULLIF($4,''), NULLIF($5,''), $6, NULLIF($7,''),
        $8, $9, $10, $11, now())
ON CONFLICT (storage_cluster_id, garage_node_id) DO UPDATE SET
  hostname=EXCLUDED.hostname, addr=EXCLUDED.addr, zone=EXCLUDED.zone,
  capacity_bytes=EXCLUDED.capacity_bytes, role=EXCLUDED.role, is_up=EXCLUDED.is_up,
  draining=EXCLUDED.draining, disk_total_bytes=EXCLUDED.disk_total_bytes,
  disk_avail_bytes=EXCLUDED.disk_avail_bytes, last_seen_at=now(), updated_at=now()`,
		n.ClusterID, n.GarageNodeID, n.Hostname, n.Addr, n.Zone, n.CapacityBytes, n.Role,
		n.IsUp, n.Draining, n.DiskTotalBytes, n.DiskAvailBytes)
	if err != nil {
		return fmt.Errorf("repository: upsert node: %w", err)
	}
	return nil
}

// ListNodesByCluster lists a cluster's nodes.
func (s *Store) ListNodesByCluster(ctx context.Context, clusterID string) ([]StorageNode, error) {
	rows, err := s.q(ctx).Query(ctx, `
SELECT id::text, storage_cluster_id::text, garage_node_id,
       COALESCE(hostname,''), COALESCE(addr,''), COALESCE(zone,''),
       capacity_bytes, COALESCE(role,''), is_up, draining,
       disk_total_bytes, disk_avail_bytes
FROM storage_nodes WHERE storage_cluster_id=$1::uuid
ORDER BY zone NULLS LAST, hostname NULLS LAST, garage_node_id`, clusterID)
	if err != nil {
		return nil, fmt.Errorf("repository: list nodes: %w", err)
	}
	defer rows.Close()
	var out []StorageNode
	for rows.Next() {
		var n StorageNode
		if err := rows.Scan(&n.ID, &n.ClusterID, &n.GarageNodeID, &n.Hostname, &n.Addr, &n.Zone,
			&n.CapacityBytes, &n.Role, &n.IsUp, &n.Draining, &n.DiskTotalBytes, &n.DiskAvailBytes); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// DeleteStaleNodes removes a cluster's nodes whose garage_node_id is not in keep
// (reconciliation: drop nodes that left the cluster). An empty keep deletes none.
func (s *Store) DeleteStaleNodes(ctx context.Context, clusterID string, keep []string) error {
	if len(keep) == 0 {
		return nil
	}
	_, err := s.q(ctx).Exec(ctx,
		`DELETE FROM storage_nodes WHERE storage_cluster_id=$1::uuid AND garage_node_id <> ALL($2)`,
		clusterID, keep)
	if err != nil {
		return fmt.Errorf("repository: delete stale nodes: %w", err)
	}
	return nil
}
