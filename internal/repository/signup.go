package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// CreateOrgWithOwner creates a fresh org + its default project + an (unverified)
// owner user with an owner membership, all in one transaction (self-serve signup).
// Returns the new ids.
func (s *Store) CreateOrgWithOwner(ctx context.Context, orgName, orgSlug, email, fullName, passwordHash string) (orgID, projectID, userID string, err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", "", "", fmt.Errorf("repository: signup begin: %w", err)
	}
	defer tx.Rollback(ctx)

	if err = tx.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ($1,$2) RETURNING id::text`, orgName, orgSlug,
	).Scan(&orgID); err != nil {
		return "", "", "", fmt.Errorf("repository: create org: %w", err)
	}
	if err = tx.QueryRow(ctx,
		`INSERT INTO projects (org_id, name, slug) VALUES ($1::uuid,'Default','default') RETURNING id::text`, orgID,
	).Scan(&projectID); err != nil {
		return "", "", "", fmt.Errorf("repository: create project: %w", err)
	}
	if err = tx.QueryRow(ctx,
		`INSERT INTO users (email, full_name, password_hash, is_platform_admin, email_verified)
		 VALUES ($1,$2,$3,false,false) RETURNING id::text`, email, fullName, passwordHash,
	).Scan(&userID); err != nil {
		return "", "", "", fmt.Errorf("repository: create user: %w", err)
	}
	if _, err = tx.Exec(ctx,
		`INSERT INTO organization_members (org_id, user_id, role) VALUES ($1::uuid,$2::uuid,'owner')`,
		orgID, userID); err != nil {
		return "", "", "", fmt.Errorf("repository: create membership: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return "", "", "", fmt.Errorf("repository: signup commit: %w", err)
	}
	return orgID, projectID, userID, nil
}

// CreateEmailVerification stores a verification token (hash only) for a user.
func (s *Store) CreateEmailVerification(ctx context.Context, userID string, tokenHash []byte, expiresAt time.Time) error {
	_, err := s.q(ctx).Exec(ctx,
		`INSERT INTO email_verifications (user_id, token_hash, expires_at) VALUES ($1::uuid,$2,$3)`,
		userID, tokenHash, expiresAt)
	if err != nil {
		return fmt.Errorf("repository: create email verification: %w", err)
	}
	return nil
}

// ConsumeEmailVerification validates + consumes a token, marks the user verified,
// and returns the user id. ErrNotFound on an unknown/expired/used token.
func (s *Store) ConsumeEmailVerification(ctx context.Context, tokenHash []byte) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("repository: verify begin: %w", err)
	}
	defer tx.Rollback(ctx)

	var userID string
	err = tx.QueryRow(ctx,
		`UPDATE email_verifications SET consumed_at=now()
		  WHERE token_hash=$1 AND consumed_at IS NULL AND expires_at > now()
		  RETURNING user_id::text`, tokenHash).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("repository: consume verification: %w", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE users SET email_verified=true WHERE id=$1::uuid`, userID); err != nil {
		return "", fmt.Errorf("repository: mark verified: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("repository: verify commit: %w", err)
	}
	return userID, nil
}

// CountRecentSignups counts signup attempts from an IP since a time (rate limit).
func (s *Store) CountRecentSignups(ctx context.Context, ip string, since time.Time) (int, error) {
	var n int
	err := s.q(ctx).QueryRow(ctx,
		`SELECT count(*) FROM signup_attempts WHERE ip=$1::inet AND created_at >= $2`, ip, since).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("repository: count signups: %w", err)
	}
	return n, nil
}

// RecordSignupAttempt logs a signup attempt for rate-limiting/abuse analysis.
func (s *Store) RecordSignupAttempt(ctx context.Context, ip, email string) {
	_, _ = s.q(ctx).Exec(ctx,
		`INSERT INTO signup_attempts (ip, email) VALUES (NULLIF($1,'')::inet, $2)`, ip, email)
}

// UnverifiedUserByEmail returns an unverified user's id by email (for resend).
func (s *Store) UnverifiedUserByEmail(ctx context.Context, email string) (string, error) {
	var id string
	err := s.q(ctx).QueryRow(ctx,
		`SELECT id::text FROM users WHERE email=$1 AND email_verified=false AND deleted_at IS NULL`, email).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("repository: unverified user: %w", err)
	}
	return id, nil
}
