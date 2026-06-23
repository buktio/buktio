package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// CountUsers returns the number of live users (used to gate the setup wizard).
func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.q(ctx).QueryRow(ctx, `SELECT count(*) FROM users WHERE deleted_at IS NULL`).Scan(&n)
	return n, err
}

// CreateUser inserts a user and returns its id.
func (s *Store) CreateUser(ctx context.Context, email, fullName, passwordHash string, platformAdmin bool) (string, error) {
	const q = `
INSERT INTO users (email, full_name, password_hash, is_platform_admin)
VALUES ($1, NULLIF($2,''), $3, $4) RETURNING id::text`
	var id string
	err := s.q(ctx).QueryRow(ctx, q, email, fullName, passwordHash, platformAdmin).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("repository: create user: %w", err)
	}
	return id, nil
}

// GetUserByEmail returns a live user by email (case-insensitive).
func (s *Store) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	const q = `
SELECT id::text, email::text, COALESCE(full_name,''), password_hash, is_platform_admin, email_verified, created_at, last_login_at
FROM users WHERE lower(email)=lower($1) AND deleted_at IS NULL`
	var u User
	err := s.q(ctx).QueryRow(ctx, q, email).Scan(
		&u.ID, &u.Email, &u.FullName, &u.PasswordHash, &u.IsPlatformAdmin, &u.EmailVerified, &u.CreatedAt, &u.LastLoginAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("repository: get user: %w", err)
	}
	return &u, nil
}

// GetUserByID fetches a user by id.
func (s *Store) GetUserByID(ctx context.Context, id string) (*User, error) {
	const q = `
SELECT id::text, email::text, COALESCE(full_name,''), password_hash, is_platform_admin, created_at, last_login_at
FROM users WHERE id=$1::uuid AND deleted_at IS NULL`
	var u User
	err := s.q(ctx).QueryRow(ctx, q, id).Scan(
		&u.ID, &u.Email, &u.FullName, &u.PasswordHash, &u.IsPlatformAdmin, &u.CreatedAt, &u.LastLoginAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("repository: get user by id: %w", err)
	}
	return &u, nil
}

// TouchUserLogin updates last_login_at.
func (s *Store) TouchUserLogin(ctx context.Context, id string) error {
	_, err := s.q(ctx).Exec(ctx, `UPDATE users SET last_login_at=now() WHERE id=$1::uuid`, id)
	return err
}

// --- sessions ---

// CreateSession inserts a session.
func (s *Store) CreateSession(ctx context.Context, userID string, tokenHash []byte, expiresAt time.Time) (string, error) {
	var id string
	err := s.q(ctx).QueryRow(ctx,
		`INSERT INTO sessions (user_id, token_hash, expires_at) VALUES ($1::uuid,$2,$3) RETURNING id::text`,
		userID, tokenHash, expiresAt).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("repository: create session: %w", err)
	}
	return id, nil
}

// GetUserBySessionToken returns the user for a valid (unexpired, unrevoked) session.
func (s *Store) GetUserBySessionToken(ctx context.Context, tokenHash []byte) (*User, error) {
	const q = `
SELECT u.id::text, u.email::text, COALESCE(u.full_name,''), u.password_hash, u.is_platform_admin, u.created_at, u.last_login_at
FROM sessions s
JOIN users u ON u.id = s.user_id AND u.deleted_at IS NULL
WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at > now()`
	var u User
	err := s.q(ctx).QueryRow(ctx, q, tokenHash).Scan(
		&u.ID, &u.Email, &u.FullName, &u.PasswordHash, &u.IsPlatformAdmin, &u.CreatedAt, &u.LastLoginAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("repository: get session: %w", err)
	}
	return &u, nil
}

// RevokeSessionByToken revokes a session given its token hash.
func (s *Store) RevokeSessionByToken(ctx context.Context, tokenHash []byte) error {
	_, err := s.q(ctx).Exec(ctx, `UPDATE sessions SET revoked_at=now() WHERE token_hash=$1`, tokenHash)
	return err
}

// RevokeAllUserSessions revokes every live session for a user (SCIM deactivation,
// password reset, etc.).
func (s *Store) RevokeAllUserSessions(ctx context.Context, userID string) error {
	_, err := s.q(ctx).Exec(ctx,
		`UPDATE sessions SET revoked_at=now() WHERE user_id=$1::uuid AND revoked_at IS NULL`, userID)
	return err
}

// SoftDeleteUser marks a user deleted (SCIM deprovision). Idempotent.
func (s *Store) SoftDeleteUser(ctx context.Context, userID string) error {
	_, err := s.q(ctx).Exec(ctx,
		`UPDATE users SET deleted_at=now() WHERE id=$1::uuid AND deleted_at IS NULL`, userID)
	return err
}
