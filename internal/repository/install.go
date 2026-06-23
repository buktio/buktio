package repository

import "context"

// GetInstallStep returns the current setup-wizard step.
func (s *Store) GetInstallStep(ctx context.Context) (string, error) {
	var step string
	err := s.q(ctx).QueryRow(ctx, `SELECT step::text FROM install_state WHERE id=1`).Scan(&step)
	return step, err
}

// SetInstallStep advances the setup-wizard step (and stamps completion).
func (s *Store) SetInstallStep(ctx context.Context, step string) error {
	_, err := s.q(ctx).Exec(ctx,
		`UPDATE install_state SET step=$1::install_step,
		    completed_at = CASE WHEN $1='completed' THEN now() ELSE completed_at END
		 WHERE id=1`, step)
	return err
}
