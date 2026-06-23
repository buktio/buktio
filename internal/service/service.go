// Package service holds buktio's business logic, transport-agnostic so a gRPC
// transport could be added later. Services orchestrate the StorageProvider, the
// repository, and the audit log.
package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/buktio/buktio/internal/audit"
	"github.com/buktio/buktio/internal/authz"
	"github.com/buktio/buktio/internal/billing"
	clusterreg "github.com/buktio/buktio/internal/cluster"
	"github.com/buktio/buktio/internal/entitlements"
	"github.com/buktio/buktio/internal/metering"
	"github.com/buktio/buktio/internal/notify"
	"github.com/buktio/buktio/internal/repository"
	"github.com/buktio/buktio/internal/secret"
	"github.com/buktio/buktio/internal/sso"
	"github.com/buktio/buktio/internal/storage"
	"github.com/buktio/buktio/internal/tenancy"
)

// Services is the business-logic facade.
type Services struct {
	Store *repository.Store
	// Provider is the primary (default) cluster's provider. Multi-cluster routing
	// goes through Reg; Provider remains the fallback/default for primary-cluster ops.
	Provider storage.StorageProvider
	Reg      *clusterreg.Registry
	Sealer   secret.Sealer
	Logger   *slog.Logger

	// Open-core enforcers. OSS defaults (PermitAll / AlwaysAllow / NoOp) are set in
	// New; paid editions inject enforcing implementations via WithEnforcers. They are
	// never nil, so call sites can consult them unconditionally.
	Authz authz.Authorizer
	Ent   entitlements.Service
	Meter metering.Sink
	// AuditSink forwards audit events to a SIEM (Enterprise). OSS default is NoOp.
	AuditSink audit.Sink
	// Mailer sends invite emails; OSS default is log-only.
	Mailer notify.Mailer
	// IdP is the SSO identity provider; OSS default is Disabled (password only).
	IdP sso.IdentityProvider
	// Provisioner selects/provisions an org's storage cluster (Hosted). OSS default
	// is Pooled (every org shares the primary cluster).
	Provisioner tenancy.Provisioner
	// Billing integrates usage-based billing (Hosted). OSS default is Disabled.
	Billing billing.Provider
	// PublicBaseURL is the externally-reachable base (e.g. https://panel.example.com)
	// used to build invite/accept links. Empty => links use a relative path.
	PublicBaseURL string

	OrgID     string
	ProjectID string
	ClusterID string
	// SystemKeyID is the Garage access-key id buktio uses for S3 management ops
	// (object browser, presign). It must be granted on every managed bucket.
	SystemKeyID string

	// Presentation/context fields set by the wiring layer.
	Version          string
	S3PublicEndpoint string
	S3Region         string
	GarageVersion    string
	ClusterMode      string              // "managed" | "external"
	GarageAdminURL   string              // for the ops metrics proxy
	MetricsToken     string              // Garage metrics_token (server-side only)
	WebPublicDomain  string              // base domain for public bucket websites
	DatabaseURL      string              // for pg_dump backups (server-side only)
	BackupDir        string              // directory where metadata backups are written
	BackupOffsite    BackupOffsiteConfig // optional off-box S3 target for scheduled backups

	// RLSEnabled turns on per-request Postgres RLS connection scoping (Enterprise).
	// When true, ScopeRequest pins each authenticated request's connection to its
	// org. Off by default; OSS/Pro behavior is unchanged.
	RLSEnabled bool

	// SelfServeSignup activates the public /signup flow (Hosted). Off by default, so
	// on-prem deployments never expose org creation. Also gated by the
	// FeatureSelfServeSignup entitlement.
	SelfServeSignup bool
	// SignupDevReturnToken (dev only) returns the email-verification token in the
	// signup API response so flows can be tested without a real mailbox. Never set
	// in production — it would let a signer-upper bypass email verification.
	SignupDevReturnToken bool

	// SCIMEnabled reports whether the SCIM 2.0 protocol handler is mounted (set by
	// the wiring layer when a SCIM handler is injected). Drives the /auth/me map so
	// OSS — which never mounts SCIM — doesn't advertise it.
	SCIMEnabled bool
}

// ScopeRequest binds the request to its tenant's org at the database connection
// level when RLS is enabled, returning a (possibly) child context and a release
// func that must always be called (defer). When RLS is off — or no org is
// resolved — it is a no-op returning the original context. This is the single
// hook the HTTP auth middleware uses to activate migration 0018's policies.
func (s *Services) ScopeRequest(ctx context.Context) (context.Context, func(), error) {
	if !s.RLSEnabled {
		return ctx, func() {}, nil
	}
	orgID := s.tenant(ctx).OrgID
	if orgID == "" {
		return ctx, func() {}, nil
	}
	return s.Store.WithOrgConn(ctx, orgID)
}

// New builds the service facade with OSS-default enforcers (all-enabled).
func New(store *repository.Store, provider storage.StorageProvider, sealer secret.Sealer, logger *slog.Logger, orgID, projectID, clusterID string) *Services {
	return &Services{
		Store: store, Provider: provider, Sealer: sealer, Logger: logger,
		OrgID: orgID, ProjectID: projectID, ClusterID: clusterID,
		Authz:       authz.NewPermitAll(),
		Ent:         entitlements.NewAlwaysAllow(),
		Meter:       metering.NewNoOp(),
		AuditSink:   audit.NoOp{},
		Mailer:      notify.LogMailer{Logger: logger},
		IdP:         sso.Disabled{},
		Provisioner: tenancy.Pooled{},
		Billing:     billing.Disabled{},
	}
}

// WithEnforcers injects paid-edition enforcers (any nil arg keeps the OSS default).
func (s *Services) WithEnforcers(a authz.Authorizer, e entitlements.Service, m metering.Sink) *Services {
	if a != nil {
		s.Authz = a
	}
	if e != nil {
		s.Ent = e
	}
	if m != nil {
		s.Meter = m
	}
	return s
}

// providerFor resolves the StorageProvider for a cluster id. An empty id (or the
// primary cluster) resolves to the default Provider. For an explicit secondary
// cluster it builds/caches the per-cluster provider and returns an error if it
// cannot be resolved — it never silently falls back to the primary backend (that
// would operate on the wrong storage and mask failures).
func (s *Services) providerFor(ctx context.Context, clusterID string) (storage.StorageProvider, error) {
	if s.Reg == nil || clusterID == "" || clusterID == s.ClusterID {
		return s.Provider, nil
	}
	return s.Reg.Provider(ctx, clusterID)
}

// providerForBucket resolves the provider for the cluster a bucket lives on.
func (s *Services) providerForBucket(ctx context.Context, b *repository.Bucket) (storage.StorageProvider, error) {
	return s.providerFor(ctx, b.ClusterID)
}

// bucketProvider loads a bucket and the provider for its cluster in one step. The
// returned error is already a *service.Error (mapped repo/storage error).
func (s *Services) bucketProvider(ctx context.Context, bucketID string) (*repository.Bucket, storage.StorageProvider, error) {
	b, serr := s.loadBucket(ctx, bucketID) // tenant-scoped (cross-org -> not found)
	if serr != nil {
		return nil, nil, serr
	}
	prov, perr := s.providerForBucket(ctx, b)
	if perr != nil {
		return nil, nil, storageUnavailableErr("cannot reach the bucket's cluster: " + perr.Error())
	}
	return b, prov, nil
}

// audit writes an audit event using the actor from context (or "system").
func (s *Services) audit(ctx context.Context, action, targetType, targetID string, meta map[string]any) {
	actorUserID, actorType := "", "system"
	if subj, ok := authz.SubjectFrom(ctx); ok && subj.UserID != "" {
		actorUserID, actorType = subj.UserID, "user"
	}
	orgID := s.tenant(ctx).OrgID
	if err := s.Store.WriteAudit(ctx, orgID, actorUserID, actorType, action, targetType, targetID, meta, ""); err != nil {
		s.Logger.Warn("audit write failed", slog.String("action", action), slog.Any("error", err))
	}
	// Fan out to the SIEM sink (NoOp in OSS). Best-effort; never blocks the request.
	if s.AuditSink != nil {
		s.AuditSink.Emit(audit.Event{
			OrgID: orgID, ActorUserID: actorUserID, ActorType: actorType,
			Action: action, TargetType: targetType, TargetID: targetID,
			Metadata: meta, At: time.Now().UTC(),
		})
	}
}

// emit sends a metering event to the billing sink (NoOp in OSS/Pro/Enterprise;
// the Hosted edition forwards to Stripe). Best-effort; errors are swallowed so the
// request path never depends on the billing pipeline.
func (s *Services) emit(ctx context.Context, t metering.EventType, subject string, qty int64) {
	if s.Meter == nil {
		return
	}
	_ = s.Meter.Emit(ctx, metering.Event{
		Type: t, TenantID: s.tenant(ctx).OrgID, Subject: subject, Quantity: qty, At: time.Now().UTC(),
	})
}

// --- DTOs ---

// QuotaDTO is a bucket quota (nil = unlimited).
type QuotaDTO struct {
	MaxBytes   *int64 `json:"max_bytes"`
	MaxObjects *int64 `json:"max_objects"`
}

// UsageDTO is per-bucket usage.
type UsageDTO struct {
	Bytes   int64 `json:"bytes_used"`
	Objects int64 `json:"objects"`
}

// BucketDTO is a bucket as the API returns it.
type BucketDTO struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	GarageID       string    `json:"garage_id"`
	ClusterID      string    `json:"cluster_id"`
	Visibility     string    `json:"visibility"`
	WebsiteEnabled bool      `json:"website_enabled"`
	PublicURL      string    `json:"public_url,omitempty"`
	Quota          QuotaDTO  `json:"quota"`
	Usage          UsageDTO  `json:"usage"`
	CreatedAt      time.Time `json:"created_at"`
}

// GrantDTO is a key's permission on a bucket.
type GrantDTO struct {
	BucketID   string `json:"bucket_id"`
	BucketName string `json:"bucket_name"`
	Read       bool   `json:"read"`
	Write      bool   `json:"write"`
	Owner      bool   `json:"owner"`
}

// KeyDTO is an access key as the API returns it (never includes the secret).
type KeyDTO struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	AccessKeyID     string     `json:"access_key_id"`
	CanCreateBucket bool       `json:"can_create_bucket"`
	SecretLastFour  string     `json:"secret_last_four,omitempty"`
	Grants          []GrantDTO `json:"grants"`
	CreatedAt       time.Time  `json:"created_at"`
}

// KeyCreateResult is returned once at creation and includes the secret.
type KeyCreateResult struct {
	KeyDTO
	SecretAccessKey string `json:"secret_access_key"`
	SecretShownOnce bool   `json:"secret_shown_once"`
}

func (s *Services) bucketToDTO(b *repository.Bucket, usage UsageDTO) BucketDTO {
	dto := BucketDTO{
		ID:             b.ID,
		Name:           b.Name,
		GarageID:       b.GarageBucketID,
		ClusterID:      b.ClusterID,
		Visibility:     b.Visibility,
		WebsiteEnabled: b.WebsiteEnabled,
		Quota:          QuotaDTO{MaxBytes: b.QuotaMaxSize, MaxObjects: b.QuotaMaxObjects},
		Usage:          usage,
		CreatedAt:      b.CreatedAt,
	}
	if b.WebsiteEnabled && s.WebPublicDomain != "" {
		dto.PublicURL = fmt.Sprintf("https://%s.%s/", b.GarageGlobalAlias, s.WebPublicDomain)
	}
	return dto
}
