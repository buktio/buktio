package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// DefaultTenant holds the ids of the auto-created default org + project.
type DefaultTenant struct {
	OrgID     string
	ProjectID string
}

// EnsureDefaultTenant returns the default org + project, creating them on first
// run. MVP ships single-tenant UX over the multi-tenant schema.
func (s *Store) EnsureDefaultTenant(ctx context.Context) (*DefaultTenant, error) {
	tx, err := s.begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("repository: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var orgID string
	err = tx.QueryRow(ctx, `SELECT id::text FROM organizations WHERE slug='default' AND deleted_at IS NULL`).Scan(&orgID)
	if errors.Is(err, pgx.ErrNoRows) {
		if err = tx.QueryRow(ctx,
			`INSERT INTO organizations (name, slug) VALUES ('Default','default') RETURNING id::text`,
		).Scan(&orgID); err != nil {
			return nil, fmt.Errorf("repository: create default org: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("repository: lookup default org: %w", err)
	}

	var projectID string
	err = tx.QueryRow(ctx,
		`SELECT id::text FROM projects WHERE org_id=$1::uuid AND slug='default' AND deleted_at IS NULL`, orgID,
	).Scan(&projectID)
	if errors.Is(err, pgx.ErrNoRows) {
		if err = tx.QueryRow(ctx,
			`INSERT INTO projects (org_id, name, slug) VALUES ($1::uuid,'Default','default') RETURNING id::text`, orgID,
		).Scan(&projectID); err != nil {
			return nil, fmt.Errorf("repository: create default project: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("repository: lookup default project: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("repository: commit: %w", err)
	}
	return &DefaultTenant{OrgID: orgID, ProjectID: projectID}, nil
}
