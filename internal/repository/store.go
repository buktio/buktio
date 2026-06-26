// Package repository is buktio's data-access layer over PostgreSQL (pgx). It owns
// the buktio-entity <-> Garage-UUID mapping, encrypted infra secrets, tenancy, and
// the audit/usage tables. Queries are hand-written SQL (no ORM).
package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a row does not exist.
var ErrNotFound = errors.New("repository: not found")

// connCtxKey carries a request-scoped, org-bound connection on the context.
type connCtxKey struct{}

// Store is the data-access layer. It runs over either PostgreSQL (pgx — the default,
// and the only backend the paid editions support) or, for OSS single-node installs,
// SQLite — selected by the dbHandle it is constructed with (see dbx.go / sqlite.go).
type Store struct {
	h dbHandle
}

// NewStore wraps a PostgreSQL pool.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{h: pgxHandle{pool}} }

// NewStoreSQLite wraps a SQLite (database/sql) handle.
func NewStoreSQLite(db *sql.DB) *Store { return &Store{h: sqliteHandle{db}} }

// Pool exposes the pgx pool for health checks; nil on the SQLite backend.
func (s *Store) Pool() *pgxpool.Pool {
	if p, ok := s.h.(pgxHandle); ok {
		return p.pool
	}
	return nil
}

// Ping verifies the database connection (any backend).
func (s *Store) Ping(ctx context.Context) error { return s.h.ping(ctx) }

// Driver reports the active backend: "postgres" or "sqlite".
func (s *Store) Driver() string { return s.h.driver() }

// Close releases the database handle.
func (s *Store) Close() { s.h.close() }

// q returns the statement runner for this context: a request-scoped org-bound
// connection if one was attached by WithOrgConn (Postgres RLS), otherwise the
// backend's default querier.
func (s *Store) q(ctx context.Context) Querier {
	if c, ok := ctx.Value(connCtxKey{}).(pgxNative); ok && c != nil {
		return pgxQuerier{c}
	}
	return s.h.querier()
}

// WithOrgConn pins the RLS org for the scope and routes statements through a
// dedicated connection (Postgres only — a no-op on SQLite). Call release exactly
// once (defer). See pgxHandle.withOrgConn for the Postgres RLS details.
func (s *Store) WithOrgConn(ctx context.Context, orgID string) (context.Context, func(), error) {
	return s.h.withOrgConn(ctx, orgID)
}

// begin starts a transaction on the active backend (Postgres or SQLite).
func (s *Store) begin(ctx context.Context) (Tx, error) { return s.h.begin(ctx) }

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
