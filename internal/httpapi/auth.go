package httpapi

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/buktio/buktio/internal/authz"
	"github.com/buktio/buktio/internal/repository"
	"github.com/buktio/buktio/internal/service"
)

// clientIP returns the request's client IP (host without port). chi's RealIP
// middleware normalizes RemoteAddr from X-Forwarded-For / X-Real-IP upstream.
func clientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// bearerToken extracts a "Authorization: Bearer <token>" value, if present.
func bearerToken(r *http.Request) string {
	const p = "Bearer "
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, p) {
		return strings.TrimSpace(h[len(p):])
	}
	return ""
}

// withUser threads the authenticated subject (with role + active org) + tenant +
// user into the request context, resolved from the user's org membership.
func withUser(ctx context.Context, svc *service.Services, u *repository.User) context.Context {
	subj, tenant := svc.ResolveSubject(ctx, u.ID, u.IsPlatformAdmin)
	ctx = authz.WithSubject(ctx, subj)
	ctx = service.WithTenant(ctx, tenant)
	return setCurrentUser(ctx, u)
}

// serveAuthed finalizes an authenticated request: it threads the subject/tenant,
// activates per-request RLS connection scoping (a no-op unless BUKTIO_RLS=on), and
// serves. The RLS connection (if any) is released when the handler returns.
func serveAuthed(w http.ResponseWriter, r *http.Request, next http.Handler, svc *service.Services, u *repository.User) {
	ctx := withUser(r.Context(), svc, u)
	// Attach ABAC request attributes (client IP + request time) to the subject so
	// ip_allowlist / business_hours policies can evaluate. Harmless when no policies
	// exist (OSS/Pro never read them).
	if subj, ok := authz.SubjectFrom(ctx); ok {
		subj.IP = clientIP(r)
		subj.Now = time.Now()
		ctx = authz.WithSubject(ctx, subj)
	}
	// Reject a suspended tenant's members outright (Enterprise). Platform admins are
	// exempt so they can always act (e.g. resume the org).
	if subj, _ := authz.SubjectFrom(ctx); !subj.PlatformAdmin && svc.OrgIsSuspended(ctx, subj.TenantID) {
		writeAuthErr(w, r, "org_suspended", "this organization is suspended; contact your administrator", http.StatusForbidden)
		return
	}
	ctx, release, err := svc.ScopeRequest(ctx)
	if err != nil {
		writeAuthErr(w, r, "internal", "could not establish tenant scope", http.StatusInternalServerError)
		return
	}
	defer release()
	next.ServeHTTP(w, r.WithContext(ctx))
}

const (
	sessionCookie = "buktio_session"
	csrfCookie    = "buktio_csrf"
)

type currentUserKey struct{}

func setCurrentUser(ctx context.Context, u *repository.User) context.Context {
	return context.WithValue(ctx, currentUserKey{}, u)
}

func currentUser(ctx context.Context) *repository.User {
	u, _ := ctx.Value(currentUserKey{}).(*repository.User)
	return u
}

func isSecure(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

func setAuthCookies(w http.ResponseWriter, r *http.Request, res *service.AuthResult) {
	secure := isSecure(r)
	maxAge := int(service.SessionTTL.Seconds())
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: res.SessionToken, Path: "/",
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: maxAge,
	})
	http.SetCookie(w, &http.Cookie{
		Name: csrfCookie, Value: res.CSRFToken, Path: "/",
		HttpOnly: false, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: maxAge,
	})
}

func clearAuthCookies(w http.ResponseWriter, r *http.Request) {
	secure := isSecure(r)
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode})
	http.SetCookie(w, &http.Cookie{Name: csrfCookie, Value: "", Path: "/", MaxAge: -1, Secure: secure, SameSite: http.SameSiteLaxMode})
}

func writeAuthErr(w http.ResponseWriter, r *http.Request, code, msg string, status int) {
	writeJSON(w, status, errEnvelope{Error: errBody{Code: code, Message: msg, RequestID: middleware.GetReqID(r.Context())}})
}

// --- handlers ---

func (h *apiHandlers) setupStatus(w http.ResponseWriter, r *http.Request) {
	initialized, err := h.svc.SetupInitialized(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"initialized": initialized})
}

type createAdminReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
}

func (h *apiHandlers) createAdmin(w http.ResponseWriter, r *http.Request) {
	var req createAdminReq
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := h.svc.CreateAdmin(r.Context(), req.Email, req.Password, req.FullName)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	setAuthCookies(w, r, res)
	writeJSON(w, http.StatusCreated, map[string]any{"user": res.User})
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *apiHandlers) login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	setAuthCookies(w, r, res)
	writeJSON(w, http.StatusOK, map[string]any{"user": res.User})
}

func (h *apiHandlers) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		_ = h.svc.Logout(r.Context(), c.Value)
	}
	clearAuthCookies(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func (h *apiHandlers) me(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r.Context())
	if u == nil {
		writeAuthErr(w, r, "unauthenticated", "authentication required", http.StatusUnauthorized)
		return
	}
	role := "admin"
	if u.IsPlatformAdmin {
		role = "owner"
	}
	if subj, ok := authz.SubjectFrom(r.Context()); ok && subj.Role != "" {
		role = string(subj.Role)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{
			"id": u.ID, "email": u.Email, "full_name": u.FullName, "role": role,
		},
		"features": h.svc.Features(r.Context()),
	})
}

// newAuthMiddleware guards product endpoints: it requires a valid session and, for
// unsafe methods, a matching CSRF token (double-submit). It threads the subject +
// user into the request context.
func newAuthMiddleware(svc *service.Services) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 1) Bearer PAT (machine clients) — CSRF-exempt (no ambient cookie).
			if bearer := bearerToken(r); bearer != "" {
				u, err := svc.UserByAPIToken(r.Context(), bearer)
				if err != nil {
					writeAuthErr(w, r, "unauthenticated", "invalid or expired token", http.StatusUnauthorized)
					return
				}
				serveAuthed(w, r, next, svc, u)
				return
			}

			// 2) Session cookie (browser) — CSRF required on mutations.
			c, err := r.Cookie(sessionCookie)
			if err != nil || c.Value == "" {
				writeAuthErr(w, r, "unauthenticated", "authentication required", http.StatusUnauthorized)
				return
			}
			u, err := svc.SessionUser(r.Context(), c.Value)
			if err != nil {
				writeAuthErr(w, r, "unauthenticated", "invalid or expired session", http.StatusUnauthorized)
				return
			}
			switch r.Method {
			case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
				cc, cerr := r.Cookie(csrfCookie)
				hdr := r.Header.Get("X-CSRF-Token")
				if cerr != nil || hdr == "" || hdr != cc.Value {
					writeAuthErr(w, r, "csrf_failed", "CSRF token missing or invalid", http.StatusForbidden)
					return
				}
			}
			serveAuthed(w, r, next, svc, u)
		})
	}
}
