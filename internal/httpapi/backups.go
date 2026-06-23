package httpapi

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func (h *apiHandlers) listBackups(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	jobs, err := h.svc.ListBackups(r.Context(), limit)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listEnvelope{Data: jobs})
}

func (h *apiHandlers) createBackup(w http.ResponseWriter, r *http.Request) {
	job, err := h.svc.CreateBackup(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (h *apiHandlers) getBackup(w http.ResponseWriter, r *http.Request) {
	job, err := h.svc.GetBackup(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (h *apiHandlers) reconcileReport(w http.ResponseWriter, r *http.Request) {
	d, err := h.svc.ReconcileReport(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}
