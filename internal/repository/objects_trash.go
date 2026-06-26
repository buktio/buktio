package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// TrashItem is a soft-deleted object (physically moved to a .trash/ prefix).
type TrashItem struct {
	ID          string
	BucketID    string
	OrgID       string
	OriginalKey string
	TrashKey    string
	SizeBytes   int64
	DeletedAt   time.Time
	PurgeAfter  time.Time
}

const trashCols = `id::text, bucket_id::text, org_id::text, original_key, trash_key, size_bytes, deleted_at, purge_after`

func scanTrash(row DBRow) (*TrashItem, error) {
	var t TrashItem
	err := row.Scan(&t.ID, &t.BucketID, &t.OrgID, &t.OriginalKey, &t.TrashKey, &t.SizeBytes, &t.DeletedAt, &t.PurgeAfter)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// InsertTrash records a soft-deleted object.
func (s *Store) InsertTrash(ctx context.Context, t TrashItem) (string, error) {
	var id string
	err := s.q(ctx).QueryRow(ctx,
		`INSERT INTO object_trash (bucket_id, org_id, original_key, trash_key, size_bytes, purge_after)
		 VALUES ($1::uuid,$2::uuid,$3,$4,$5,$6) RETURNING id::text`,
		t.BucketID, t.OrgID, t.OriginalKey, t.TrashKey, t.SizeBytes, t.PurgeAfter).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("repository: insert trash: %w", err)
	}
	return id, nil
}

// ListTrash returns non-restored trash items for a bucket (newest first).
func (s *Store) ListTrash(ctx context.Context, bucketID string) ([]TrashItem, error) {
	q := `SELECT ` + trashCols + ` FROM object_trash
WHERE bucket_id=$1::uuid AND restored_at IS NULL ORDER BY deleted_at DESC`
	rows, err := s.q(ctx).Query(ctx, q, bucketID)
	if err != nil {
		return nil, fmt.Errorf("repository: list trash: %w", err)
	}
	defer rows.Close()
	var out []TrashItem
	for rows.Next() {
		t, err := scanTrash(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

// GetTrash returns one non-restored trash item.
func (s *Store) GetTrash(ctx context.Context, id string) (*TrashItem, error) {
	q := `SELECT ` + trashCols + ` FROM object_trash WHERE id=$1::uuid AND restored_at IS NULL`
	return scanTrash(s.q(ctx).QueryRow(ctx, q, id))
}

// MarkRestored marks a trash item restored.
func (s *Store) MarkRestored(ctx context.Context, id string) error {
	_, err := s.q(ctx).Exec(ctx, `UPDATE object_trash SET restored_at=now() WHERE id=$1::uuid`, id)
	return err
}

// DeleteTrash removes a trash row (after a hard purge).
func (s *Store) DeleteTrash(ctx context.Context, id string) error {
	_, err := s.q(ctx).Exec(ctx, `DELETE FROM object_trash WHERE id=$1::uuid`, id)
	return err
}

// DueForPurge returns trash items past their purge_after that are not restored.
func (s *Store) DueForPurge(ctx context.Context, limit int) ([]TrashItem, error) {
	if limit <= 0 {
		limit = 200
	}
	q := `SELECT ` + trashCols + ` FROM object_trash
WHERE restored_at IS NULL AND purge_after < now() LIMIT $1`
	rows, err := s.q(ctx).Query(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TrashItem
	for rows.Next() {
		t, err := scanTrash(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}
