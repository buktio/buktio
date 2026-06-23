package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/buktio/buktio/internal/service"
)

func (h *apiHandlers) getBucketCORS(w http.ResponseWriter, r *http.Request) {
	rules, err := h.svc.GetBucketCORS(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listEnvelope{Data: rules})
}

type setCORSReq struct {
	Rules []service.CORSRuleDTO `json:"rules"`
}

func (h *apiHandlers) setBucketCORS(w http.ResponseWriter, r *http.Request) {
	var req setCORSReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.svc.SetBucketCORS(r.Context(), chi.URLParam(r, "id"), req.Rules); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *apiHandlers) deleteBucketCORS(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteBucketCORS(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *apiHandlers) getBucketLifecycle(w http.ResponseWriter, r *http.Request) {
	rules, err := h.svc.GetBucketLifecycle(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listEnvelope{Data: rules})
}

type setLifecycleReq struct {
	Rules []service.LifecycleRuleDTO `json:"rules"`
}

func (h *apiHandlers) setBucketLifecycle(w http.ResponseWriter, r *http.Request) {
	var req setLifecycleReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.svc.SetBucketLifecycle(r.Context(), chi.URLParam(r, "id"), req.Rules); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *apiHandlers) deleteBucketLifecycle(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteBucketLifecycle(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
