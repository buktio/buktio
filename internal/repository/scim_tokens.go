package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// SCIMToken is a provisioning token row (no raw secret stored).
type SCIMToken struct {
	ID         string
	Name       string
	LastFour   string
	CreatedAt  time.Time
	LastUsedAt *time.Time
}

// CreateSCIMToken inserts a token (hash + last-four only) and returns its id.
func (s *Store) CreateSCIMToken(ctx context.Context, orgID, name string, tokenHash []byte, lastFour string) (string, error) {
	var id string
	err := s.q(ctx).QueryRow(ctx,
		`INSERT INTO scim_tokens (org_id, name, token_hash, last_four)
		 VALUES ($1::uuid,$2,$3,NULLIF($4,'')) RETURNING id::text`,
		orgID, name, tokenHash, lastFour).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("repository: create scim token: %w", err)
	}
	return id, nil
}

// ListSCIMTokens returns an org's live tokens (newest first).
func (s *Store) ListSCIMTokens(ctx context.Context, orgID string) ([]SCIMToken, error) {
	rows, err := s.q(ctx).Query(ctx,
		`SELECT id::text, name, COALESCE(last_four,''), created_at, last_used_at
		   FROM scim_tokens WHERE org_id=$1::uuid AND deleted_at IS NULL
		  ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("repository: list scim tokens: %w", err)
	}
	defer rows.Close()
	var out []SCIMToken
	for rows.Next() {
		var t SCIMToken
		if err := rows.Scan(&t.ID, &t.Name, &t.LastFour, &t.CreatedAt, &t.LastUsedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// RevokeSCIMToken soft-deletes an org's token by id.
func (s *Store) RevokeSCIMToken(ctx context.Context, orgID, id string) error {
	ct, err := s.q(ctx).Exec(ctx,
		`UPDATE scim_tokens SET deleted_at=now() WHERE id=$1::uuid AND org_id=$2::uuid AND deleted_at IS NULL`,
		id, orgID)
	if err != nil {
		return fmt.Errorf("repository: revoke scim token: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ResolveSCIMOrg returns the org id for a valid SCIM token hash and bumps
// last_used_at. ErrNotFound when the token is unknown/revoked.
func (s *Store) ResolveSCIMOrg(ctx context.Context, tokenHash []byte) (string, error) {
	var orgID string
	err := s.q(ctx).QueryRow(ctx,
		`UPDATE scim_tokens SET last_used_at=now()
		  WHERE token_hash=$1 AND deleted_at IS NULL
		  RETURNING org_id::text`, tokenHash).Scan(&orgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("repository: resolve scim org: %w", err)
	}
	return orgID, nil
}
