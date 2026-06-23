package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Member is an org member joined with the user record.
type Member struct {
	UserID    string
	Email     string
	FullName  string
	Role      string
	CreatedAt time.Time
}

// ListMembers returns an org's members.
func (s *Store) ListMembers(ctx context.Context, orgID string) ([]Member, error) {
	rows, err := s.q(ctx).Query(ctx, `
SELECT u.id::text, u.email::text, COALESCE(u.full_name,''), om.role::text, om.created_at
FROM organization_members om
JOIN users u ON u.id = om.user_id AND u.deleted_at IS NULL
WHERE om.org_id = $1::uuid
ORDER BY om.created_at ASC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("repository: list members: %w", err)
	}
	defer rows.Close()
	var out []Member
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.UserID, &m.Email, &m.FullName, &m.Role, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// GetMembershipRole returns a user's role in an org, or ErrNotFound if not a member.
func (s *Store) GetMembershipRole(ctx context.Context, orgID, userID string) (string, error) {
	var role string
	err := s.q(ctx).QueryRow(ctx,
		`SELECT role::text FROM organization_members WHERE org_id=$1::uuid AND user_id=$2::uuid`,
		orgID, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("repository: get membership: %w", err)
	}
	return role, nil
}

// CountUserMemberships returns how many orgs a user belongs to (used to decide
// whether deprovisioning from one org should also disable the global account).
func (s *Store) CountUserMemberships(ctx context.Context, userID string) (int, error) {
	var n int
	err := s.q(ctx).QueryRow(ctx,
		`SELECT count(*) FROM organization_members WHERE user_id=$1::uuid`, userID).Scan(&n)
	return n, err
}

// UpsertMember inserts or updates a membership role.
func (s *Store) UpsertMember(ctx context.Context, orgID, userID, role string) error {
	_, err := s.q(ctx).Exec(ctx,
		`INSERT INTO organization_members (org_id, user_id, role) VALUES ($1::uuid,$2::uuid,$3::org_member_role)
		 ON CONFLICT (org_id, user_id) DO UPDATE SET role = EXCLUDED.role`,
		orgID, userID, role)
	if err != nil {
		return fmt.Errorf("repository: upsert member: %w", err)
	}
	return nil
}

// RemoveMember deletes a membership.
func (s *Store) RemoveMember(ctx context.Context, orgID, userID string) error {
	_, err := s.q(ctx).Exec(ctx,
		`DELETE FROM organization_members WHERE org_id=$1::uuid AND user_id=$2::uuid`, orgID, userID)
	return err
}

// CountMembers / CountOwners gate seat limits and last-owner protection.
func (s *Store) CountMembers(ctx context.Context, orgID string) (int, error) {
	var n int
	err := s.q(ctx).QueryRow(ctx, `SELECT count(*) FROM organization_members WHERE org_id=$1::uuid`, orgID).Scan(&n)
	return n, err
}

func (s *Store) CountOwners(ctx context.Context, orgID string) (int, error) {
	var n int
	err := s.q(ctx).QueryRow(ctx, `SELECT count(*) FROM organization_members WHERE org_id=$1::uuid AND role='owner'`, orgID).Scan(&n)
	return n, err
}

// --- invitations ---

// Invitation is a pending org invite.
type Invitation struct {
	ID        string
	OrgID     string
	Email     string
	Role      string
	ExpiresAt time.Time
}

// CreateInvitation records a pending invite and returns its id.
func (s *Store) CreateInvitation(ctx context.Context, orgID, email, role string, tokenHash []byte, invitedBy string, expiresAt time.Time) (string, error) {
	var id string
	err := s.q(ctx).QueryRow(ctx,
		`INSERT INTO invitations (org_id, email, role, token_hash, invited_by, expires_at)
		 VALUES ($1::uuid,$2,$3::org_member_role,$4,NULLIF($5,'')::uuid,$6) RETURNING id::text`,
		orgID, email, role, tokenHash, invitedBy, expiresAt).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("repository: create invitation: %w", err)
	}
	return id, nil
}

// GetInvitationByToken returns a live (unaccepted, unexpired) invitation by token hash.
func (s *Store) GetInvitationByToken(ctx context.Context, tokenHash []byte) (*Invitation, error) {
	var iv Invitation
	err := s.q(ctx).QueryRow(ctx,
		`SELECT id::text, org_id::text, email::text, role::text, expires_at
		 FROM invitations WHERE token_hash=$1 AND accepted_at IS NULL AND expires_at > now()`,
		tokenHash).Scan(&iv.ID, &iv.OrgID, &iv.Email, &iv.Role, &iv.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("repository: get invitation: %w", err)
	}
	return &iv, nil
}

// MarkInvitationAccepted consumes an invitation.
func (s *Store) MarkInvitationAccepted(ctx context.Context, id string) error {
	_, err := s.q(ctx).Exec(ctx, `UPDATE invitations SET accepted_at=now() WHERE id=$1::uuid`, id)
	return err
}

// ListInvitations returns an org's pending invitations.
func (s *Store) ListInvitations(ctx context.Context, orgID string) ([]Invitation, error) {
	rows, err := s.q(ctx).Query(ctx,
		`SELECT id::text, org_id::text, email::text, role::text, expires_at
		 FROM invitations WHERE org_id=$1::uuid AND accepted_at IS NULL AND expires_at > now()
		 ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("repository: list invitations: %w", err)
	}
	defer rows.Close()
	var out []Invitation
	for rows.Next() {
		var iv Invitation
		if err := rows.Scan(&iv.ID, &iv.OrgID, &iv.Email, &iv.Role, &iv.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, iv)
	}
	return out, rows.Err()
}
