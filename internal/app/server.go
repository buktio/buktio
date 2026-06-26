package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/buktio/buktio/internal/audit"
	"github.com/buktio/buktio/internal/authz"
	"github.com/buktio/buktio/internal/billing"
	"github.com/buktio/buktio/internal/config"
	"github.com/buktio/buktio/internal/db"
	"github.com/buktio/buktio/internal/edition"
	"github.com/buktio/buktio/internal/entitlements"
	"github.com/buktio/buktio/internal/httpapi"
	"github.com/buktio/buktio/internal/metering"
	"github.com/buktio/buktio/internal/observability"
	"github.com/buktio/buktio/internal/repository"
	"github.com/buktio/buktio/internal/secret"
	"github.com/buktio/buktio/internal/service"
	"github.com/buktio/buktio/internal/sso"
	"github.com/buktio/buktio/internal/tenancy"
)

// Enforcers are the open-core implementations injected at startup. The zero value
// is OSS (fully enabled): RunServer fills any nil field with its OSS default, so
// the free build and a paid build share one boot path. Core never imports ee/ —
// the paid main (cmd/buktio-api-ee) constructs ee/ impls and passes them here.
type Enforcers struct {
	Authz authz.Authorizer
	Ent   entitlements.Service
	Meter metering.Sink
	IdP   sso.IdentityProvider
	// AuditSink forwards audit events to a SIEM (Enterprise). Nil => OSS NoOp.
	AuditSink audit.Sink
	// Provisioner provisions per-tenant clusters (Hosted dedicated mode). Nil =>
	// the core Pooled provisioner (every org on the shared primary cluster).
	Provisioner tenancy.Provisioner
	// Billing integrates usage-based billing (Hosted). Nil => OSS Disabled.
	Billing billing.Provider
	// SCIM, when non-nil (SCIM licensed), builds the SCIM 2.0 protocol handler from
	// the wired Services. Called after the store/services exist. Nil in OSS.
	SCIM func(*service.Services) http.Handler
	// AuthzFactory, when non-nil, builds the authorizer once the store exists (for
	// ABAC policies that need DB access) and overrides Authz. Nil in OSS.
	AuthzFactory func(*service.Services) authz.Authorizer
	// Edition overrides the configured edition for logging/UX (paid mains set it
	// from the verified license). Empty => use cfg.Edition.
	Edition edition.Edition
}

// RunServer boots the buktio API end-to-end: load config, migrate, provision the
// storage backend, start the background loops, and serve HTTP — injecting the given
// enforcers (OSS defaults when nil). Both the OSS and ee mains call this.
func RunServer(version string, enf Enforcers) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := observability.NewLogger(cfg.LogLevel)

	ed := enf.Edition
	if ed == "" {
		ed = edition.Parse(cfg.Edition)
	}

	logger.Info("starting buktio-api",
		slog.String("version", version),
		slog.String("edition", ed.String()),
		slog.String("addr", cfg.HTTP.Addr))
	if cfg.AllowInsecure {
		logger.Warn("BUKTIO_ALLOW_INSECURE is set — serving without TLS enforcement; dev only")
	}

	// Fail closed: the paid editions require PostgreSQL (RLS, pg_dump, plpgsql, the
	// app.current_org GUC are not portable to SQLite). SQLite is OSS-only; the
	// upgrade path is `buktio db convert --to postgres` then point DATABASE_URL at it.
	if ed != edition.OSS && cfg.DB.URL != "" && db.DriverFromDSN(cfg.DB.URL) == "sqlite" {
		return fmt.Errorf("the %s edition requires PostgreSQL; SQLite is OSS-only — migrate with `buktio db convert --to postgres` and set DATABASE_URL to the Postgres DSN", ed)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var (
		store *repository.Store
		svc   *service.Services
	)

	if cfg.DB.URL == "" {
		logger.Warn("DATABASE_URL is not set — serving health endpoints only (no product API)")
	} else {
		store, svc, err = wireServices(ctx, cfg, logger, version, enf)
		if err != nil {
			return err
		}
		// Background loops on the cancellable ctx; cleanup cancels, waits, then closes.
		var bg sync.WaitGroup
		bg.Add(5)
		go func() { defer bg.Done(); svc.RunUsageCollector(ctx, 5*time.Minute) }()    // M9
		go func() { defer bg.Done(); svc.RunNodeReconciler(ctx, 2*time.Minute) }()    // v2-M4
		go func() { defer bg.Done(); svc.RunBackupScheduler(ctx, 1*time.Minute) }()   // Pro-M8
		go func() { defer bg.Done(); svc.RunTenantControlLoop(ctx, 1*time.Minute) }() // Hosted-M3
		go func() { defer bg.Done(); svc.RunBillingReporter(ctx, 1*time.Hour) }()     // Hosted-M4
		// Resume any migration / replication jobs interrupted by a restart (from cursor).
		svc.ResumeMigrations(ctx)   // Hosted-M5
		svc.ResumeReplications(ctx) // v2.6 cross-backend replication
		defer func() { stop(); bg.Wait(); store.Close() }()
	}

	probe := &readinessProbe{
		client:         &http.Client{Timeout: 3 * time.Second},
		garageAdminURL: cfg.Garage.AdminURL,
		store:          store,
	}

	var scimHandler http.Handler
	if svc != nil && enf.SCIM != nil {
		scimHandler = enf.SCIM(svc)
		svc.SCIMEnabled = true
		logger.Info("SCIM 2.0 provisioning enabled at /scim/v2")
	}

	handler := httpapi.New(httpapi.Deps{
		Logger:       logger,
		Version:      version,
		Readiness:    probe,
		Services:     svc,
		MetricsToken: os.Getenv("BUKTIO_METRICS_TOKEN"),
		SCIMHandler:  scimHandler,
	})

	srv := &http.Server{
		Addr:         cfg.HTTP.Addr,
		Handler:      handler,
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("listening",
			slog.String("addr", cfg.HTTP.Addr),
			slog.String("tls", cfg.HTTP.TLS.Mode))
		if err := serveWithTLS(srv, cfg.HTTP.TLS, logger); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// wireServices opens the DB, migrates, provisions the storage backend, builds the
// service facade, and injects the enforcers.
func wireServices(ctx context.Context, cfg *config.Config, logger *slog.Logger, version string, enf Enforcers) (*repository.Store, *service.Services, error) {
	// Select the backend from the DSN: a sqlite:/file: scheme or *.db path uses the
	// optional single-node SQLite backend (OSS); anything else is PostgreSQL (the
	// default, and the only backend the paid editions support).
	var store *repository.Store
	if db.DriverFromDSN(cfg.DB.URL) == "sqlite" {
		sdb, err := db.OpenSQLite(ctx, db.SQLitePath(cfg.DB.URL))
		if err != nil {
			return nil, nil, err
		}
		if err := db.MigrateSQLite(ctx, sdb); err != nil {
			_ = sdb.Close()
			return nil, nil, err
		}
		store = repository.NewStoreSQLite(sdb)
	} else {
		pool, err := db.OpenPool(ctx, cfg.DB.URL)
		if err != nil {
			return nil, nil, err
		}
		if err := db.Migrate(cfg.DB.URL); err != nil {
			pool.Close()
			return nil, nil, err
		}
		store = repository.NewStore(pool)
	}
	logger.Info("database migrated", slog.String("driver", store.Driver()))

	kek, err := secret.DefaultProvider().KEK()
	if err != nil {
		store.Close()
		return nil, nil, err
	}
	sealer, err := secret.NewEnvelopeSealer(kek)
	if err != nil {
		store.Close()
		return nil, nil, err
	}

	prov, err := Provision(ctx, cfg, store, sealer, logger)
	if err != nil {
		store.Close()
		return nil, nil, err
	}

	svc := service.New(store, prov.Provider, sealer, logger, prov.Tenant.OrgID, prov.Tenant.ProjectID, prov.Cluster.ID)
	svc.WithEnforcers(enf.Authz, enf.Ent, enf.Meter)
	// Store-backed authorizer (ABAC policies) overrides the stateless RBAC one once
	// the store exists. No-op in OSS (factory nil).
	if enf.AuthzFactory != nil {
		svc.Authz = enf.AuthzFactory(svc)
	}
	if enf.AuditSink != nil {
		svc.AuditSink = enf.AuditSink
	}
	if enf.Provisioner != nil {
		svc.Provisioner = enf.Provisioner
	}
	if enf.Billing != nil {
		svc.Billing = enf.Billing
	} else if os.Getenv("BUKTIO_BILLING_DEV") != "" {
		// Processor-free billing for local testing of the reporter + webhook flow.
		svc.Billing = billing.Manual{Logger: logger}
	}
	if enf.IdP != nil {
		svc.IdP = enf.IdP
	}
	svc.Reg = prov.Registry
	svc.SystemKeyID = prov.Cluster.SystemS3AccessKeyID
	svc.Version = version
	svc.S3PublicEndpoint = cfg.Garage.S3PublicEndpoint
	svc.S3Region = cfg.Garage.S3Region
	svc.GarageVersion = prov.Cluster.GarageVersion
	svc.ClusterMode = prov.Cluster.Mode
	svc.GarageAdminURL = cfg.Garage.AdminURL
	svc.MetricsToken = os.Getenv("GARAGE_METRICS_TOKEN")
	svc.WebPublicDomain = cfg.Garage.WebPublicDomain
	svc.DatabaseURL = cfg.DB.URL
	svc.RLSEnabled = cfg.RLSEnabled
	svc.SelfServeSignup = cfg.SelfServeSignup
	svc.SignupDevReturnToken = cfg.SignupDevReturnToken
	svc.PublicBaseURL = os.Getenv("BUKTIO_PUBLIC_URL")
	svc.BackupDir = os.Getenv("BUKTIO_BACKUP_DIR")
	if svc.BackupDir == "" {
		svc.BackupDir = "/var/lib/buktio/backups"
	}
	svc.BackupOffsite = service.BackupOffsiteConfig{
		Endpoint:  os.Getenv("BUKTIO_BACKUP_S3_ENDPOINT"),
		Region:    os.Getenv("BUKTIO_BACKUP_S3_REGION"),
		Bucket:    os.Getenv("BUKTIO_BACKUP_S3_BUCKET"),
		AccessKey: os.Getenv("BUKTIO_BACKUP_S3_ACCESS_KEY"),
		Secret:    os.Getenv("BUKTIO_BACKUP_S3_SECRET"),
	}
	return store, svc, nil
}

// readinessProbe reports dependency health for /readyz.
type readinessProbe struct {
	client         *http.Client
	garageAdminURL string
	store          *repository.Store
}

func (p *readinessProbe) Check(ctx context.Context) (bool, map[string]string) {
	components := map[string]string{}
	ready := true

	if p.store == nil {
		components["db"] = "unconfigured"
	} else if err := p.store.Ping(ctx); err != nil {
		components["db"] = "down"
		ready = false
	} else {
		components["db"] = "ok"
	}

	if p.garageAdminURL == "" {
		components["garage_admin"] = "unconfigured"
	} else if p.pingGarage(ctx) {
		components["garage_admin"] = "ok"
	} else {
		components["garage_admin"] = "down"
		ready = false
	}

	return ready, components
}

func (p *readinessProbe) pingGarage(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.garageAdminURL+"/health", nil)
	if err != nil {
		return false
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
