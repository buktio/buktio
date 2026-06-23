package httpapi

import (
	"net/http"

	"github.com/buktio/buktio/internal/service"
)

// getBranding returns the active org's branding (authenticated).
func (h *apiHandlers) getBranding(w http.ResponseWriter, r *http.Request) {
	b, err := h.svc.GetBranding(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

type setBrandingReq struct {
	DisplayName  string `json:"display_name"`
	LogoURL      string `json:"logo_url"`
	PrimaryColor string `json:"primary_color"`
	EmailFrom    string `json:"email_from"`
	CustomDomain string `json:"custom_domain"`
}

func (h *apiHandlers) setBranding(w http.ResponseWriter, r *http.Request) {
	var req setBrandingReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.svc.SetBranding(r.Context(), service.SetBrandingInput{
		DisplayName: req.DisplayName, LogoURL: req.LogoURL, PrimaryColor: req.PrimaryColor,
		EmailFrom: req.EmailFrom, CustomDomain: req.CustomDomain,
	}); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// resolveBranding themes the pre-login UI: it returns branding for the request
// Host (a custom domain). Public; empty when the host isn't a custom domain.
func (h *apiHandlers) resolveBranding(w http.ResponseWriter, r *http.Request) {
	host := r.URL.Query().Get("host")
	if host == "" {
		host = r.Host
	}
	b, err := h.svc.BrandingForHost(r.Context(), host)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

// verifyDomain is Caddy's on_demand_tls "ask" endpoint: 200 => issue a cert for
// this (registered) custom domain; anything else => refuse. This is mandatory to
// stop arbitrary hosts from triggering unbounded certificate issuance.
func (h *apiHandlers) verifyDomain(w http.ResponseWriter, r *http.Request) {
	domain := r.URL.Query().Get("domain")
	if h.svc.DomainAllowed(r.Context(), domain) {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusForbidden)
}
