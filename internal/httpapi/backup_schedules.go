package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (h *apiHandlers) listBackupSchedules(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.ListBackupSchedules(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listEnvelope{Data: rows})
}

type scheduleReq struct {
	Enabled         bool `json:"enabled"`
	IntervalMinutes int  `json:"interval_minutes"`
	RetentionCount  int  `json:"retention_count"`
	OffsiteEnabled  bool `json:"offsite_enabled"`
}

func (h *apiHandlers) createBackupSchedule(w http.ResponseWriter, r *http.Request) {
	var req scheduleReq
	if !decodeJSON(w, r, &req) {
		return
	}
	sc, err := h.svc.CreateBackupSchedule(r.Context(), req.IntervalMinutes, req.RetentionCount, req.OffsiteEnabled)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, sc)
}

func (h *apiHandlers) updateBackupSchedule(w http.ResponseWriter, r *http.Request) {
	var req scheduleReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.svc.UpdateBackupSchedule(r.Context(), chi.URLParam(r, "id"), req.Enabled, req.IntervalMinutes, req.RetentionCount, req.OffsiteEnabled); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *apiHandlers) deleteBackupSchedule(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteBackupSchedule(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
