package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (h *apiHandlers) listWebhooks(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.ListWebhooks(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listEnvelope{Data: rows})
}

type createWebhookReq struct {
	URL    string   `json:"url"`
	Events []string `json:"events"`
	Secret string   `json:"secret"`
}

func (h *apiHandlers) createWebhook(w http.ResponseWriter, r *http.Request) {
	var req createWebhookReq
	if !decodeJSON(w, r, &req) {
		return
	}
	dto, err := h.svc.CreateWebhook(r.Context(), chi.URLParam(r, "id"), req.URL, req.Events, req.Secret)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, dto)
}

func (h *apiHandlers) deleteWebhook(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteWebhook(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "whId")); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
