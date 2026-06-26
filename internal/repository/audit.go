package repository

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strings"
	"time"
)

// auditCanonical builds the deterministic byte string hashed into the chain. It
// must be reproducible from the stored row (see VerifyAuditChain).
func auditCanonical(prev []byte, id int64, orgID, actorUserID, actorType, action, targetType, targetID, metadata string, createdAt time.Time) []byte {
	h := sha256.New()
	h.Write(prev)
	fmt.Fprintf(h, "%d|%s|%s|%s|%s|%s|%s|%s|%s",
		id, orgID, actorUserID, actorType, action, targetType, targetID, metadata,
		createdAt.UTC().Format(time.RFC3339Nano))
	return h.Sum(nil)
}

// orgLockKey derives a stable advisory-lock key from an org id so hash-chain
// writes serialize per org (concurrent writers can't fork the chain).
func orgLockKey(orgID string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte("audit:" + orgID))
	return int64(h.Sum64())
}

// AuditEntry is one audit-log record.
type AuditEntry struct {
	ID          int64
	ActorUserID string
	ActorType   string
	Action      string
	TargetType  string
	TargetID    string
	Metadata    map[string]any
	CreatedAt   time.Time
}

// AuditFilter parameterizes a filtered audit query. Zero-value fields are ignored.
type AuditFilter struct {
	OrgID      string
	Actor      string
	Action     string
	TargetType string
	From       time.Time
	To         time.Time
	Limit      int
	Offset     int
}

// ListAuditFiltered returns audit events matching the filter (newest first). Always
// pass OrgID to scope to the active tenant.
func (s *Store) ListAuditFiltered(ctx context.Context, f AuditFilter) ([]AuditEntry, error) {
	conds := []string{}
	args := []any{}
	add := func(expr string, val any) {
		args = append(args, val)
		conds = append(conds, fmt.Sprintf(expr, len(args)))
	}
	if f.OrgID != "" {
		add("org_id = $%d::uuid", f.OrgID)
	}
	if f.Actor != "" {
		add("actor_user_id = $%d::uuid", f.Actor)
	}
	if f.Action != "" {
		add("action = $%d", f.Action)
	}
	if f.TargetType != "" {
		add("target_type = $%d", f.TargetType)
	}
	if !f.From.IsZero() {
		add("created_at >= $%d", f.From)
	}
	if !f.To.IsZero() {
		add("created_at <= $%d", f.To)
	}
	limit := f.Limit
	if limit <= 0 || limit > 5000 {
		limit = 100
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	q := `SELECT id, COALESCE(actor_user_id::text,''), actor_type::text, action,
	       COALESCE(target_type,''), COALESCE(target_id,''), metadata, created_at
	FROM audit_events`
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	q += fmt.Sprintf(" ORDER BY created_at DESC LIMIT %d OFFSET %d", limit, offset)

	rows, err := s.q(ctx).Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		var meta []byte
		if err := rows.Scan(&e.ID, &e.ActorUserID, &e.ActorType, &e.Action,
			&e.TargetType, &e.TargetID, &meta, &e.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(meta, &e.Metadata)
		out = append(out, e)
	}
	return out, rows.Err()
}

// WriteAudit appends an audit event. Secrets must never be placed in metadata.
func (s *Store) WriteAudit(ctx context.Context, orgID, actorUserID, actorType, action, targetType, targetID string, metadata map[string]any, ip string) error {
	meta, err := json.Marshal(metadata)
	if err != nil || metadata == nil {
		meta = []byte("{}")
	}
	// Hash-chain the row inside a per-org-serialized transaction so the chain can
	// never fork. The tx runs on the pool (a system write with an explicit org_id),
	// independent of any request-scoped RLS connection.
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("repository: audit begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, orgLockKey(orgID)); err != nil {
		return fmt.Errorf("repository: audit lock: %w", err)
	}
	var prev []byte
	_ = tx.QueryRow(ctx,
		`SELECT row_hash FROM audit_events
		  WHERE org_id IS NOT DISTINCT FROM NULLIF($1,'')::uuid AND row_hash IS NOT NULL
		  ORDER BY id DESC LIMIT 1`, orgID).Scan(&prev)

	var id int64
	var createdAt time.Time
	var metaText string // the jsonb-normalized form, so the hash matches verify time
	if err := tx.QueryRow(ctx, `
INSERT INTO audit_events (org_id, actor_user_id, actor_type, action, target_type, target_id, metadata, ip_address, prev_hash)
VALUES (NULLIF($1,'')::uuid, NULLIF($2,'')::uuid, $3::actor_type, $4, NULLIF($5,''), NULLIF($6,''), $7::jsonb, NULLIF($8,'')::inet, $9)
RETURNING id, created_at, metadata::text`,
		orgID, actorUserID, actorType, action, targetType, targetID, string(meta), ip, prev).Scan(&id, &createdAt, &metaText); err != nil {
		return fmt.Errorf("repository: audit insert: %w", err)
	}

	rowHash := auditCanonical(prev, id, orgID, actorUserID, actorType, action, targetType, targetID, metaText, createdAt)
	if _, err := tx.Exec(ctx, `UPDATE audit_events SET row_hash=$2 WHERE id=$1`, id, rowHash); err != nil {
		return fmt.Errorf("repository: audit hash: %w", err)
	}
	return tx.Commit(ctx)
}

// AuditVerifyResult reports the outcome of a hash-chain integrity check.
type AuditVerifyResult struct {
	Checked   int
	OK        bool
	BrokenID  int64  // first row whose hash doesn't match (0 if none)
	BrokenWhy string // "row_hash mismatch" | "prev_hash mismatch"
}

// VerifyAuditChain recomputes the hash chain for an org over its chained rows
// (row_hash NOT NULL), in id order, and reports the first break. Tampering with
// any historical row, or deleting a middle row, surfaces as a mismatch.
func (s *Store) VerifyAuditChain(ctx context.Context, orgID string) (*AuditVerifyResult, error) {
	rows, err := s.q(ctx).Query(ctx, `
SELECT id, COALESCE(actor_user_id::text,''), actor_type::text, action,
       COALESCE(target_type,''), COALESCE(target_id,''), metadata::text, created_at,
       prev_hash, row_hash
  FROM audit_events
 WHERE org_id IS NOT DISTINCT FROM NULLIF($1,'')::uuid AND row_hash IS NOT NULL
 ORDER BY id ASC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("repository: audit verify: %w", err)
	}
	defer rows.Close()

	res := &AuditVerifyResult{OK: true}
	var expectedPrev []byte
	first := true
	for rows.Next() {
		var (
			id                                               int64
			actorUserID, actorType, action, tType, tID, meta string
			createdAt                                        time.Time
			prev, stored                                     []byte
		)
		if err := rows.Scan(&id, &actorUserID, &actorType, &action, &tType, &tID, &meta, &createdAt, &prev, &stored); err != nil {
			return nil, err
		}
		// Chain linkage: each row's prev_hash must equal the prior row's row_hash.
		if !first && !bytes.Equal(prev, expectedPrev) {
			res.OK, res.BrokenID, res.BrokenWhy = false, id, "prev_hash mismatch"
			return res, nil
		}
		want := auditCanonical(prev, id, orgID, actorUserID, actorType, action, tType, tID, meta, createdAt)
		if !bytes.Equal(want, stored) {
			res.OK, res.BrokenID, res.BrokenWhy = false, id, "row_hash mismatch"
			return res, nil
		}
		expectedPrev = stored
		first = false
		res.Checked++
	}
	return res, rows.Err()
}

// ListAudit returns recent audit events (newest first), capped by limit.
func (s *Store) ListAudit(ctx context.Context, limit int) ([]AuditEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	const q = `
SELECT id, COALESCE(actor_user_id::text,''), actor_type::text, action,
       COALESCE(target_type,''), COALESCE(target_id,''), metadata, created_at
FROM audit_events ORDER BY created_at DESC LIMIT $1`
	rows, err := s.q(ctx).Query(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		var meta []byte
		if err := rows.Scan(&e.ID, &e.ActorUserID, &e.ActorType, &e.Action,
			&e.TargetType, &e.TargetID, &meta, &e.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(meta, &e.Metadata)
		out = append(out, e)
	}
	return out, rows.Err()
}
