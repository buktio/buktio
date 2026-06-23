package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// OrgCluster is an org→cluster assignment joined with cluster presentation fields.
type OrgCluster struct {
	StorageClusterID string
	Name             string
	Provider         string
	IsDefault        bool
}

// AssignClusterToOrg maps a cluster to an org (idempotent). When makeDefault is
// true it becomes the org's default, atomically clearing any prior default.
func (s *Store) AssignClusterToOrg(ctx context.Context, orgID, clusterID string, makeDefault bool) error {
	if makeDefault {
		if _, err := s.q(ctx).Exec(ctx,
			`UPDATE org_storage_clusters SET is_default=false WHERE org_id=$1::uuid AND is_default`,
			orgID); err != nil {
			return fmt.Errorf("repository: clear default cluster: %w", err)
		}
	}
	_, err := s.q(ctx).Exec(ctx,
		`INSERT INTO org_storage_clusters (org_id, storage_cluster_id, is_default)
		 VALUES ($1::uuid,$2::uuid,$3)
		 ON CONFLICT (org_id, storage_cluster_id) DO UPDATE SET is_default = EXCLUDED.is_default`,
		orgID, clusterID, makeDefault)
	if err != nil {
		return fmt.Errorf("repository: assign cluster to org: %w", err)
	}
	return nil
}

// UnassignClusterFromOrg removes an org→cluster mapping.
func (s *Store) UnassignClusterFromOrg(ctx context.Context, orgID, clusterID string) error {
	ct, err := s.q(ctx).Exec(ctx,
		`DELETE FROM org_storage_clusters WHERE org_id=$1::uuid AND storage_cluster_id=$2::uuid`,
		orgID, clusterID)
	if err != nil {
		return fmt.Errorf("repository: unassign cluster: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListOrgClusters returns an org's assigned clusters (with their default flag).
func (s *Store) ListOrgClusters(ctx context.Context, orgID string) ([]OrgCluster, error) {
	rows, err := s.q(ctx).Query(ctx,
		`SELECT osc.storage_cluster_id::text, c.name, c.provider::text, osc.is_default
		   FROM org_storage_clusters osc
		   JOIN storage_clusters c ON c.id = osc.storage_cluster_id AND c.deleted_at IS NULL
		  WHERE osc.org_id=$1::uuid
		  ORDER BY osc.is_default DESC, osc.created_at ASC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("repository: list org clusters: %w", err)
	}
	defer rows.Close()
	var out []OrgCluster
	for rows.Next() {
		var oc OrgCluster
		if err := rows.Scan(&oc.StorageClusterID, &oc.Name, &oc.Provider, &oc.IsDefault); err != nil {
			return nil, err
		}
		out = append(out, oc)
	}
	return out, rows.Err()
}

// DefaultClusterForOrg returns the org's default cluster id, or ErrNotFound when
// the org has no assignment (caller falls back to the primary cluster).
func (s *Store) DefaultClusterForOrg(ctx context.Context, orgID string) (string, error) {
	var id string
	err := s.q(ctx).QueryRow(ctx,
		`SELECT osc.storage_cluster_id::text
		   FROM org_storage_clusters osc
		   JOIN storage_clusters c ON c.id = osc.storage_cluster_id AND c.deleted_at IS NULL
		  WHERE osc.org_id=$1::uuid AND osc.is_default
		  LIMIT 1`, orgID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("repository: default cluster for org: %w", err)
	}
	return id, nil
}
