package httpapi

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
)

const ssoStateCookie = "buktio_sso_state"

func randState() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// ssoLogin redirects the browser to the SSO provider with an anti-CSRF state cookie.
func (h *apiHandlers) ssoLogin(w http.ResponseWriter, r *http.Request) {
	if !h.svc.SSOEnabled() {
		writeAuthErr(w, r, "sso_disabled", "SSO is not enabled", http.StatusNotFound)
		return
	}
	state := randState()
	http.SetCookie(w, &http.Cookie{
		Name: ssoStateCookie, Value: state, Path: "/",
		HttpOnly: true, Secure: isSecure(r), SameSite: http.SameSiteLaxMode, MaxAge: 600,
	})
	http.Redirect(w, r, h.svc.SSOAuthURL(state), http.StatusFound)
}

// ssoCallback validates the state, exchanges the code, logs the user in, and
// redirects to the panel.
func (h *apiHandlers) ssoCallback(w http.ResponseWriter, r *http.Request) {
	if !h.svc.SSOEnabled() {
		writeAuthErr(w, r, "sso_disabled", "SSO is not enabled", http.StatusNotFound)
		return
	}
	q := r.URL.Query()
	c, err := r.Cookie(ssoStateCookie)
	if err != nil || c.Value == "" || c.Value != q.Get("state") {
		writeAuthErr(w, r, "sso_state_mismatch", "invalid SSO state", http.StatusForbidden)
		return
	}
	ext, err := h.svc.SSOExchange(r.Context(), q.Get("code"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	res, err := h.svc.LoginWithExternalIdentity(r.Context(), ext)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	setAuthCookies(w, r, res)
	http.SetCookie(w, &http.Cookie{Name: ssoStateCookie, Value: "", Path: "/", MaxAge: -1, Secure: isSecure(r)})
	http.Redirect(w, r, "/", http.StatusFound)
}

// authMethods reports which login methods are available (for the login page).
func (h *apiHandlers) authMethods(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"password": true, "sso": h.svc.SSOEnabled()})
}
