package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/buktio/buktio/internal/service"
)

func (h *apiHandlers) listPolicies(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.ListPolicies(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policies": rows})
}

type createPolicyReq struct {
	Name     string            `json:"name"`
	Template string            `json:"template"`
	Config   map[string]string `json:"config"`
	Roles    []string          `json:"roles"`
}

func (h *apiHandlers) createPolicy(w http.ResponseWriter, r *http.Request) {
	var req createPolicyReq
	if !decodeJSON(w, r, &req) {
		return
	}
	id, err := h.svc.CreatePolicy(r.Context(), service.CreatePolicyInput{
		Name: req.Name, Template: req.Template, Config: req.Config, Roles: req.Roles,
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

type policyEnabledReq struct {
	Enabled bool `json:"enabled"`
}

func (h *apiHandlers) setPolicyEnabled(w http.ResponseWriter, r *http.Request) {
	var req policyEnabledReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.svc.SetPolicyEnabled(r.Context(), chi.URLParam(r, "id"), req.Enabled); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *apiHandlers) deletePolicy(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeletePolicy(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
