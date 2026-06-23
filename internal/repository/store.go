// Package repository is buktio's data-access layer over PostgreSQL (pgx). It owns
// the buktio-entity <-> Garage-UUID mapping, encrypted infra secrets, tenancy, and
// the audit/usage tables. Queries are hand-written SQL (no ORM).
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a row does not exist.
var ErrNotFound = errors.New("repository: not found")

// querier is the subset of pgx used for ordinary statements. Both *pgxpool.Pool
// and a request-scoped *pgxpool.Conn satisfy it, so repository methods run on the
// pool by default and on an org-scoped connection when one is on the context.
type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// connCtxKey carries a request-scoped, org-bound connection on the context.
type connCtxKey struct{}

// Store is the data-access layer.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore wraps a pgx pool.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Pool exposes the underlying pool (for health checks).
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// q returns the statement runner for this context: a request-scoped org-bound
// connection if one was attached by WithOrgConn, otherwise the shared pool.
func (s *Store) q(ctx context.Context) querier {
	if c, ok := ctx.Value(connCtxKey{}).(querier); ok && c != nil {
		return c
	}
	return s.pool
}

// WithOrgConn checks out a dedicated connection, pins the RLS org on it via the
// `app.current_org` GUC, and returns a child context that routes all repository
// statements through that connection. Call release exactly once (defer) to clear
// the GUC and return the connection to the pool. This is the integration point
// for Postgres RLS (migration 0018): when the app connects as the non-superuser
// `buktio_app` role, the org_isolation policy restricts every row to orgID.
//
// It holds a pooled connection (not a transaction) for the scope's lifetime, so
// there is no idle-in-transaction risk across slow upstream (S3/Garage) calls.
func (s *Store) WithOrgConn(ctx context.Context, orgID string) (context.Context, func(), error) {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return ctx, func() {}, fmt.Errorf("repository: acquire org conn: %w", err)
	}
	// Session-level (is_local=false) so it persists across this connection's
	// statements; cleared on release. set_config is parameterized (no injection).
	if _, err := conn.Exec(ctx, "SELECT set_config('app.current_org', $1, false)", orgID); err != nil {
		conn.Release()
		return ctx, func() {}, fmt.Errorf("repository: set org scope: %w", err)
	}
	release := func() {
		// Best-effort clear on a fresh context so a cancelled request still resets
		// the GUC before the connection is reused by another tenant.
		_, _ = conn.Exec(context.Background(), "SELECT set_config('app.current_org', '', false)")
		conn.Release()
	}
	return context.WithValue(ctx, connCtxKey{}, querier(conn)), release, nil
}

// --- Domain row types ---

// Cluster is a storage backend deployment (a Garage cluster). Secret fields are
// envelope-encrypted blobs.
type Cluster struct {
	ID                  string
	Name                string
	Provider            string
	Mode                string // "managed" | "external"
	S3Endpoint          string
	AdminEndpoint       string
	S3Region            string
	WebEndpoint         string
	GarageVersion       string
	RPCSecretEnc        []byte
	AdminTokenEnc       []byte
	MetricsTokenEnc     []byte
	SystemS3AccessKeyID string
	SystemS3SecretEnc   []byte
	DBEngine            string
	ReplicationFactor   int
	Status              string
}

// Bucket maps a buktio bucket to its Garage bucket.
type Bucket struct {
	ID                string
	OrgID             string
	ProjectID         string
	ClusterID         string
	Name              string
	GarageBucketID    string
	GarageGlobalAlias string
	Visibility        string
	WebsiteEnabled    bool
	WebsiteIndexDoc   string
	WebsiteErrorDoc   string
	QuotaMaxSize      *int64
	QuotaMaxObjects   *int64
	CreatedAt         time.Time
}

// AccessKey maps a buktio key to its Garage key. The S3 secret is never stored.
type AccessKey struct {
	ID                string
	OrgID             string
	ProjectID         string
	ClusterID         string
	Name              string
	GarageAccessKeyID string
	SecretLastFour    string
	CanCreateBucket   bool
	CreatedAt         time.Time
}

// Grant mirrors a Garage AllowBucketKey grant (read/write/owner).
type Grant struct {
	BucketID     string
	AccessKeyID  string
	BucketName   string
	GarageBucket string
	CanRead      bool
	CanWrite     bool
	IsOwner      bool
}

// User is a panel user.
type User struct {
	ID              string
	Email           string
	FullName        string
	PasswordHash    string
	IsPlatformAdmin bool
	EmailVerified   bool
	CreatedAt       time.Time
	LastLoginAt     *time.Time
}

// Session is a server-side session.
type Session struct {
	ID        string
	UserID    string
	TokenHash []byte
	ExpiresAt time.Time
}
