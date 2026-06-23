package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// APIToken is a personal access token row (no secret material stored).
type APIToken struct {
	ID             string
	UserID         string
	Name           string
	SecretLastFour string
	Scopes         []string
	ExpiresAt      *time.Time
	LastUsedAt     *time.Time
	CreatedAt      time.Time
}

// CreateAPIToken inserts a token (hash + last-four only) and returns its id.
func (s *Store) CreateAPIToken(ctx context.Context, userID, orgID, name string, tokenHash []byte, lastFour string, expiresAt *time.Time) (string, error) {
	var id string
	err := s.q(ctx).QueryRow(ctx,
		`INSERT INTO api_tokens (user_id, org_id, name, token_hash, secret_last_four, expires_at)
		 VALUES ($1::uuid, NULLIF($2,'')::uuid, $3, $4, NULLIF($5,''), $6) RETURNING id::text`,
		userID, orgID, name, tokenHash, lastFour, expiresAt).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("repository: create api token: %w", err)
	}
	return id, nil
}

// GetUserByAPIToken returns the user for a valid (unexpired, unrevoked) token and
// updates its last_used_at.
func (s *Store) GetUserByAPIToken(ctx context.Context, tokenHash []byte) (*User, error) {
	const q = `
SELECT u.id::text, u.email::text, COALESCE(u.full_name,''), u.password_hash, u.is_platform_admin, u.created_at, u.last_login_at
FROM api_tokens t
JOIN users u ON u.id = t.user_id AND u.deleted_at IS NULL
WHERE t.token_hash=$1 AND t.deleted_at IS NULL AND (t.expires_at IS NULL OR t.expires_at > now())`
	var u User
	err := s.q(ctx).QueryRow(ctx, q, tokenHash).Scan(
		&u.ID, &u.Email, &u.FullName, &u.PasswordHash, &u.IsPlatformAdmin, &u.CreatedAt, &u.LastLoginAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("repository: get api token: %w", err)
	}
	_, _ = s.q(ctx).Exec(ctx, `UPDATE api_tokens SET last_used_at=now() WHERE token_hash=$1`, tokenHash)
	return &u, nil
}

// ListAPITokens returns a user's live tokens (newest first).
func (s *Store) ListAPITokens(ctx context.Context, userID string) ([]APIToken, error) {
	const q = `
SELECT id::text, user_id::text, name, COALESCE(secret_last_four,''), scopes, expires_at, last_used_at, created_at
FROM api_tokens WHERE user_id=$1::uuid AND deleted_at IS NULL ORDER BY created_at DESC`
	rows, err := s.q(ctx).Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("repository: list api tokens: %w", err)
	}
	defer rows.Close()
	var out []APIToken
	for rows.Next() {
		var t APIToken
		if err := rows.Scan(&t.ID, &t.UserID, &t.Name, &t.SecretLastFour, &t.Scopes, &t.ExpiresAt, &t.LastUsedAt, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// SoftDeleteAPIToken revokes a token (scoped to its owner).
func (s *Store) SoftDeleteAPIToken(ctx context.Context, id, userID string) error {
	_, err := s.q(ctx).Exec(ctx,
		`UPDATE api_tokens SET deleted_at=now() WHERE id=$1::uuid AND user_id=$2::uuid AND deleted_at IS NULL`,
		id, userID)
	return err
}
