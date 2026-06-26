package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (h *apiHandlers) listReplications(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.ListReplications(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listEnvelope{Data: rows})
}

type startReplicationReq struct {
	DstBucketID string `json:"dst_bucket_id"`
}

func (h *apiHandlers) startReplication(w http.ResponseWriter, r *http.Request) {
	var req startReplicationReq
	if !decodeJSON(w, r, &req) {
		return
	}
	dto, err := h.svc.StartReplication(r.Context(), chi.URLParam(r, "id"), req.DstBucketID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, dto)
}
