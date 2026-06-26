// Package httpapi contains the chi router, HTTP handlers, and middleware.
//
// Handlers are deliberately THIN: they translate HTTP <-> service calls and own
// no business logic (which lives in internal/service, transport-agnostic so a
// gRPC transport can be added later). M0 wires only health/readiness; the
// /api/v1 surface is filled in over subsequent milestones.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/buktio/buktio/internal/observability"
	"github.com/buktio/buktio/internal/service"
	"github.com/buktio/buktio/internal/webui"
)

// ReadinessProbe reports the health of buktio's dependencies. The map is
// component -> status ("ok" | "down" | ...). It is implemented in main (M0) and
// by the storage/db layers in later milestones.
type ReadinessProbe interface {
	Check(ctx context.Context) (ready bool, components map[string]string)
}

// Deps are the dependencies the router needs.
type Deps struct {
	Logger    *slog.Logger
	Version   string
	Readiness ReadinessProbe
	// Services is the business-logic facade. When nil (e.g. before provisioning),
	// only the health endpoints are served.
	Services *service.Services
	// MetricsToken, when set, guards the /metrics endpoint with a bearer token.
	MetricsToken string
	// AuthMiddleware guards the product endpoints (set in M8). When nil, endpoints
	// are unauthenticated.
	AuthMiddleware func(http.Handler) http.Handler
	// SCIMHandler, when non-nil (paid editions with SCIM licensed), serves the SCIM
	// 2.0 protocol at /scim/v2 with its own per-org bearer-token auth. Nil in OSS.
	SCIMHandler http.Handler
}

// New builds the API router.
func New(d Deps) http.Handler {
	r := chi.NewRouter()

	// Base middleware chain (see middleware.go for the buktio-specific ones).
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(securityHeaders)
	r.Use(requestLogger(d.Logger))
	r.Use(middleware.Recoverer)

	h := &healthHandler{version: d.Version, probe: d.Readiness}

	// Unversioned probes that must survive API version bumps.
	r.Get("/healthz", h.live)
	r.Get("/readyz", h.ready)

	// SCIM 2.0 (paid; nil in OSS). Its own bearer-token auth, mounted as a sibling
	// of /api/v1 so IdPs hit a stable, versionless base URL.
	if d.SCIMHandler != nil {
		r.Mount("/scim/v2", d.SCIMHandler)
	}

	// Prometheus metrics (buktio's own). Guarded by a bearer token when
	// BUKTIO_METRICS_TOKEN is set (open otherwise — api is not host-published).
	r.Handle("/metrics", metricsGuard(d.MetricsToken, observability.MetricsHandler()))

	// Versioned application surface.
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{
				"service": "buktio-api",
				"version": d.Version,
				"status":  "ok",
			})
		})

		if d.Services == nil {
			return
		}
		h := &apiHandlers{svc: d.Services, logger: d.Logger}

		// Public: setup wizard + login/logout (no session required).
		r.Route("/setup", func(r chi.Router) {
			r.Get("/status", h.setupStatus)
			r.Post("/create-admin", h.createAdmin)
		})
		r.Post("/auth/login", h.login)
		r.Post("/auth/logout", h.logout)
		r.Post("/auth/accept-invite", h.acceptInvite)     // public: accept an org invite
		r.Get("/auth/methods", h.authMethods)             // public: available login methods
		r.Get("/auth/sso/login", h.ssoLogin)              // public: start SSO
		r.Get("/auth/sso/callback", h.ssoCallback)        // public: SSO callback
		r.Get("/branding/resolve", h.resolveBranding)     // public: theme the login page by Host
		r.Get("/branding/domains/verify", h.verifyDomain) // public: Caddy on_demand_tls allow-list
		r.Post("/signup", h.signup)                       // public: self-serve org signup (Hosted)
		r.Post("/signup/verify", h.signupVerify)          // public: consume email verification
		r.Get("/signup/verify", h.signupVerify)           // public: verification link (GET)
		r.Post("/signup/resend", h.signupResend)          // public: resend verification
		r.Post("/billing/webhook", h.billingWebhook)      // public: processor webhook (signature-verified)

		// Guarded: everything else requires a valid session (+ CSRF on mutations).
		authMw := d.AuthMiddleware
		if authMw == nil {
			authMw = newAuthMiddleware(d.Services)
		}
		r.Group(func(r chi.Router) {
			r.Use(authMw)
			r.Get("/auth/me", h.me)
			h.register(r)
		})
	})

	// Embedded web panel (single-binary delivery). Any request not matched by the
	// API, probes, metrics or SCIM falls through here: the UI handler serves the
	// static export and SPA-falls-back to index.html. Unknown /api/ paths keep a
	// JSON 404 instead of getting the HTML shell.
	if ui, err := webui.Handler(); err != nil {
		d.Logger.Error("web panel disabled: failed to init embedded UI", slog.Any("error", err))
	} else {
		r.NotFound(func(w http.ResponseWriter, req *http.Request) {
			if strings.HasPrefix(req.URL.Path, "/api/") {
				writeJSON(w, http.StatusNotFound, errEnvelope{Error: errBody{
					Code: "not_found", Message: "no such endpoint",
				}})
				return
			}
			ui.ServeHTTP(w, req)
		})
	}

	return r
}
