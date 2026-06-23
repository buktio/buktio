package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

type createTokenReq struct {
	Name      string `json:"name"`
	ExpiresIn int    `json:"expires_in_days"` // 0 = never
}

func (h *apiHandlers) createAPIToken(w http.ResponseWriter, r *http.Request) {
	var req createTokenReq
	if !decodeJSON(w, r, &req) {
		return
	}
	var expires *time.Time
	if req.ExpiresIn > 0 {
		t := time.Now().UTC().Add(time.Duration(req.ExpiresIn) * 24 * time.Hour)
		expires = &t
	}
	res, err := h.svc.CreateAPIToken(r.Context(), req.Name, expires)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

func (h *apiHandlers) listAPITokens(w http.ResponseWriter, r *http.Request) {
	tokens, err := h.svc.ListAPITokens(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listEnvelope{Data: tokens})
}

func (h *apiHandlers) revokeAPIToken(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RevokeAPIToken(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
