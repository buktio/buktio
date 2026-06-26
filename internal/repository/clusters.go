package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// GetActiveCluster returns the single live storage cluster, or ErrNotFound.
func (s *Store) GetActiveCluster(ctx context.Context) (*Cluster, error) {
	const q = `
SELECT id::text, name, provider::text, mode::text, s3_endpoint, admin_endpoint, s3_region,
       COALESCE(web_endpoint,''), COALESCE(garage_version,''),
       rpc_secret_enc, admin_token_enc, metrics_token_enc,
       COALESCE(system_s3_access_key_id,''), system_s3_secret_enc,
       db_engine, replication_factor, status::text
FROM storage_clusters
WHERE deleted_at IS NULL
ORDER BY created_at ASC
LIMIT 1`
	var c Cluster
	err := s.q(ctx).QueryRow(ctx, q).Scan(
		&c.ID, &c.Name, &c.Provider, &c.Mode, &c.S3Endpoint, &c.AdminEndpoint, &c.S3Region,
		&c.WebEndpoint, &c.GarageVersion,
		&c.RPCSecretEnc, &c.AdminTokenEnc, &c.MetricsTokenEnc,
		&c.SystemS3AccessKeyID, &c.SystemS3SecretEnc,
		&c.DBEngine, &c.ReplicationFactor, &c.Status,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("repository: get cluster: %w", err)
	}
	return &c, nil
}

// CreateCluster inserts a storage cluster and returns its id.
func (s *Store) CreateCluster(ctx context.Context, c Cluster) (string, error) {
	const q = `
INSERT INTO storage_clusters
  (name, provider, mode, s3_endpoint, admin_endpoint, s3_region, web_endpoint, garage_version,
   rpc_secret_enc, admin_token_enc, metrics_token_enc,
   system_s3_access_key_id, system_s3_secret_enc,
   db_engine, replication_factor, status)
VALUES ($1, $2::provider_type, $3::cluster_mode, $4, $5, $6, NULLIF($7,''), NULLIF($8,''),
        $9, $10, $11, NULLIF($12,''), $13, $14, $15, $16::cluster_status)
RETURNING id::text`
	if c.Provider == "" {
		c.Provider = "garage"
	}
	if c.Mode == "" {
		c.Mode = "managed"
	}
	if c.Status == "" {
		c.Status = "healthy"
	}
	var id string
	err := s.q(ctx).QueryRow(ctx, q,
		c.Name, c.Provider, c.Mode, c.S3Endpoint, c.AdminEndpoint, c.S3Region, c.WebEndpoint, c.GarageVersion,
		c.RPCSecretEnc, c.AdminTokenEnc, c.MetricsTokenEnc,
		c.SystemS3AccessKeyID, c.SystemS3SecretEnc,
		c.DBEngine, c.ReplicationFactor, c.Status,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("repository: create cluster: %w", err)
	}
	return id, nil
}

const clusterCols = `id::text, name, provider::text, mode::text, s3_endpoint, admin_endpoint, s3_region,
       COALESCE(web_endpoint,''), COALESCE(garage_version,''),
       rpc_secret_enc, admin_token_enc, metrics_token_enc,
       COALESCE(system_s3_access_key_id,''), system_s3_secret_enc,
       db_engine, replication_factor, status::text`

func scanCluster(row DBRow) (*Cluster, error) {
	var c Cluster
	err := row.Scan(
		&c.ID, &c.Name, &c.Provider, &c.Mode, &c.S3Endpoint, &c.AdminEndpoint, &c.S3Region,
		&c.WebEndpoint, &c.GarageVersion,
		&c.RPCSecretEnc, &c.AdminTokenEnc, &c.MetricsTokenEnc,
		&c.SystemS3AccessKeyID, &c.SystemS3SecretEnc,
		&c.DBEngine, &c.ReplicationFactor, &c.Status,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// GetClusterByID returns a live cluster by id.
func (s *Store) GetClusterByID(ctx context.Context, id string) (*Cluster, error) {
	q := `SELECT ` + clusterCols + ` FROM storage_clusters WHERE id=$1::uuid AND deleted_at IS NULL`
	c, err := scanCluster(s.q(ctx).QueryRow(ctx, q, id))
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, fmt.Errorf("repository: get cluster: %w", err)
	}
	return c, err
}

// ListClusters returns all live clusters (primary first by creation order).
func (s *Store) ListClusters(ctx context.Context) ([]Cluster, error) {
	q := `SELECT ` + clusterCols + ` FROM storage_clusters WHERE deleted_at IS NULL ORDER BY created_at ASC`
	rows, err := s.q(ctx).Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("repository: list clusters: %w", err)
	}
	defer rows.Close()
	var out []Cluster
	for rows.Next() {
		c, err := scanCluster(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

// UpdateClusterStatus updates a cluster's status + cached health detail.
func (s *Store) UpdateClusterStatus(ctx context.Context, id, status string, healthDetail []byte) error {
	_, err := s.q(ctx).Exec(ctx,
		`UPDATE storage_clusters SET status=$2::cluster_status, last_health_at=now(),
		    last_health_detail = COALESCE($3::jsonb, last_health_detail)
		 WHERE id=$1::uuid AND deleted_at IS NULL`,
		id, status, healthDetail)
	if err != nil {
		return fmt.Errorf("repository: update cluster status: %w", err)
	}
	return nil
}

// SoftDeleteCluster marks a cluster deleted (used by the remove-cluster flow).
func (s *Store) SoftDeleteCluster(ctx context.Context, id string) error {
	_, err := s.q(ctx).Exec(ctx,
		`UPDATE storage_clusters SET deleted_at=now() WHERE id=$1::uuid AND deleted_at IS NULL`, id)
	return err
}

// CountBucketsInCluster counts live buckets on a cluster (guards removal).
func (s *Store) CountBucketsInCluster(ctx context.Context, clusterID string) (int, error) {
	var n int
	err := s.q(ctx).QueryRow(ctx,
		`SELECT count(*) FROM buckets WHERE storage_cluster_id=$1::uuid AND deleted_at IS NULL`,
		clusterID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("repository: count buckets in cluster: %w", err)
	}
	return n, nil
}
