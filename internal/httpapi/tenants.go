package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (h *apiHandlers) getOrgStatus(w http.ResponseWriter, r *http.Request) {
	st, err := h.svc.GetOrgStatus(r.Context(), chi.URLParam(r, "orgId"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

type suspendReq struct {
	Reason string `json:"reason"`
}

func (h *apiHandlers) suspendOrg(w http.ResponseWriter, r *http.Request) {
	var req suspendReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.svc.SuspendOrg(r.Context(), chi.URLParam(r, "orgId"), req.Reason); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *apiHandlers) resumeOrg(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.ResumeOrg(r.Context(), chi.URLParam(r, "orgId")); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type quotaReq struct {
	// MaxBytes nil clears the org-level ceiling.
	MaxBytes *int64 `json:"max_bytes"`
}

func (h *apiHandlers) setOrgQuota(w http.ResponseWriter, r *http.Request) {
	var req quotaReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.svc.SetOrgQuota(r.Context(), chi.URLParam(r, "orgId"), req.MaxBytes); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *apiHandlers) listOrgClusters(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.ListOrgClusters(r.Context(), chi.URLParam(r, "orgId"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"clusters": rows})
}

type assignClusterReq struct {
	ClusterID string `json:"cluster_id"`
	Default   bool   `json:"default"`
}

func (h *apiHandlers) assignOrgCluster(w http.ResponseWriter, r *http.Request) {
	var req assignClusterReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.svc.AssignClusterToOrg(r.Context(), chi.URLParam(r, "orgId"), req.ClusterID, req.Default); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *apiHandlers) unassignOrgCluster(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.UnassignClusterFromOrg(r.Context(), chi.URLParam(r, "orgId"), chi.URLParam(r, "clusterId")); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type tenantClusterReq struct {
	Mode string `json:"mode"` // pooled | dedicated
}

func (h *apiHandlers) assignTenantCluster(w http.ResponseWriter, r *http.Request) {
	var req tenantClusterReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Mode == "" {
		req.Mode = "pooled"
	}
	res, err := h.svc.AssignTenantCluster(r.Context(), chi.URLParam(r, "orgId"), req.Mode)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *apiHandlers) listSCIMTokens(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.ListSCIMTokens(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tokens": rows})
}

type scimTokenReq struct {
	Name string `json:"name"`
}

func (h *apiHandlers) createSCIMToken(w http.ResponseWriter, r *http.Request) {
	var req scimTokenReq
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := h.svc.CreateSCIMToken(r.Context(), req.Name)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

func (h *apiHandlers) revokeSCIMToken(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RevokeSCIMToken(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
