package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// GetUserByExternalIdentity returns the user linked to an (provider, subject) pair.
func (s *Store) GetUserByExternalIdentity(ctx context.Context, provider, subject string) (*User, error) {
	var userID string
	err := s.q(ctx).QueryRow(ctx,
		`SELECT user_id::text FROM user_identities WHERE provider=$1 AND external_subject=$2`,
		provider, subject).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("repository: get user by identity: %w", err)
	}
	return s.GetUserByID(ctx, userID)
}

// LinkIdentity associates an external identity with a user (idempotent).
func (s *Store) LinkIdentity(ctx context.Context, userID, provider, subject string) error {
	_, err := s.q(ctx).Exec(ctx,
		`INSERT INTO user_identities (user_id, provider, external_subject) VALUES ($1::uuid,$2,$3)
		 ON CONFLICT (provider, external_subject) DO NOTHING`,
		userID, provider, subject)
	if err != nil {
		return fmt.Errorf("repository: link identity: %w", err)
	}
	return nil
}
