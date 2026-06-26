package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ResolveUserTenant returns a user's active (org, project, role): their earliest
// organization_members org, that org's earliest live project, and the membership
// role. Falls back to the supplied defaults with an empty role when the user has no
// membership — the OSS single-admin case.
func (s *Store) ResolveUserTenant(ctx context.Context, userID, defaultOrg, defaultProject string) (orgID, projectID, role string, err error) {
	err = s.q(ctx).QueryRow(ctx,
		`SELECT org_id::text, role::text FROM organization_members WHERE user_id=$1::uuid ORDER BY created_at ASC LIMIT 1`,
		userID).Scan(&orgID, &role)
	if errors.Is(err, pgx.ErrNoRows) {
		return defaultOrg, defaultProject, "", nil
	}
	if err != nil {
		return defaultOrg, defaultProject, "", fmt.Errorf("repository: resolve user org: %w", err)
	}

	perr := s.q(ctx).QueryRow(ctx,
		`SELECT id::text FROM projects WHERE org_id=$1::uuid AND deleted_at IS NULL ORDER BY created_at ASC LIMIT 1`,
		orgID).Scan(&projectID)
	if perr != nil {
		projectID = defaultProject // org with no project yet — degrade gracefully
	}
	return orgID, projectID, role, nil
}

// OrgStatus holds the tenant-lifecycle fields used by suspend/resume + quota.
type OrgStatus struct {
	Status        string
	SuspendedAt   *time.Time
	SuspendReason string
	QuotaMaxBytes *int64
}

// GetOrgStatus returns a live org's lifecycle/quota state, or ErrNotFound.
func (s *Store) GetOrgStatus(ctx context.Context, orgID string) (*OrgStatus, error) {
	var o OrgStatus
	var reason *string
	err := s.q(ctx).QueryRow(ctx,
		`SELECT status::text, suspended_at, suspend_reason, quota_max_bytes
		   FROM organizations WHERE id=$1::uuid AND deleted_at IS NULL`,
		orgID).Scan(&o.Status, &o.SuspendedAt, &reason, &o.QuotaMaxBytes)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("repository: get org status: %w", err)
	}
	if reason != nil {
		o.SuspendReason = *reason
	}
	return &o, nil
}

// SuspendOrg flips an org to suspended with a reason and timestamp (idempotent).
func (s *Store) SuspendOrg(ctx context.Context, orgID, reason string) error {
	ct, err := s.q(ctx).Exec(ctx,
		`UPDATE organizations
		    SET status='suspended', suspended_at=now(), suspend_reason=NULLIF($2,'')
		  WHERE id=$1::uuid AND deleted_at IS NULL`, orgID, reason)
	if err != nil {
		return fmt.Errorf("repository: suspend org: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ResumeOrg clears suspension, returning the org to active (idempotent).
func (s *Store) ResumeOrg(ctx context.Context, orgID string) error {
	ct, err := s.q(ctx).Exec(ctx,
		`UPDATE organizations
		    SET status='active', suspended_at=NULL, suspend_reason=NULL
		  WHERE id=$1::uuid AND deleted_at IS NULL`, orgID)
	if err != nil {
		return fmt.Errorf("repository: resume org: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetOrgStatus sets an org's lifecycle status (active|suspended|pending_setup).
func (s *Store) SetOrgStatus(ctx context.Context, orgID, status string) error {
	ct, err := s.q(ctx).Exec(ctx,
		`UPDATE organizations SET status=$2::org_status WHERE id=$1::uuid AND deleted_at IS NULL`,
		orgID, status)
	if err != nil {
		return fmt.Errorf("repository: set org status: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListOrgIDsByStatus returns live org ids in a given status (control loops).
func (s *Store) ListOrgIDsByStatus(ctx context.Context, status string) ([]string, error) {
	rows, err := s.q(ctx).Query(ctx,
		`SELECT id::text FROM organizations WHERE status=$1::org_status AND deleted_at IS NULL`, status)
	if err != nil {
		return nil, fmt.Errorf("repository: list orgs by status: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// SetOrgQuota sets (or clears, when nil) the org-level storage ceiling in bytes.
func (s *Store) SetOrgQuota(ctx context.Context, orgID string, maxBytes *int64) error {
	ct, err := s.q(ctx).Exec(ctx,
		`UPDATE organizations SET quota_max_bytes=$2 WHERE id=$1::uuid AND deleted_at IS NULL`,
		orgID, maxBytes)
	if err != nil {
		return fmt.Errorf("repository: set org quota: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// OrgUsageTotals sums the latest-per-bucket usage across an entire org.
func (s *Store) OrgUsageTotals(ctx context.Context, orgID string) (bytesUsed, objectCount int64, err error) {
	// row_number() window keeps the latest snapshot per bucket — portable across
	// Postgres and SQLite (DISTINCT ON is Postgres-only).
	const q = `
WITH latest AS (
  SELECT bytes_used, object_count,
         row_number() OVER (PARTITION BY bucket_id ORDER BY captured_at DESC) AS rn
  FROM usage_snapshots
  WHERE org_id=$1::uuid
)
SELECT COALESCE(sum(bytes_used),0), COALESCE(sum(object_count),0) FROM latest WHERE rn=1`
	err = s.q(ctx).QueryRow(ctx, q, orgID).Scan(&bytesUsed, &objectCount)
	if err != nil {
		return 0, 0, nil
	}
	return bytesUsed, objectCount, nil
}

// ListOrgIDs returns all live organization ids (for org-aware background loops).
func (s *Store) ListOrgIDs(ctx context.Context) ([]string, error) {
	rows, err := s.q(ctx).Query(ctx, `SELECT id::text FROM organizations WHERE deleted_at IS NULL ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("repository: list org ids: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
