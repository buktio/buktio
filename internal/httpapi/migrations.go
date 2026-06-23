package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/buktio/buktio/internal/service"
)

type startMigrationReq struct {
	SourceEndpoint  string `json:"source_endpoint"`
	SourceRegion    string `json:"source_region"`
	SourceBucket    string `json:"source_bucket"`
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	DestBucketID    string `json:"dest_bucket_id"`
}

func (h *apiHandlers) startMigration(w http.ResponseWriter, r *http.Request) {
	var req startMigrationReq
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := h.svc.StartMigration(r.Context(), service.MigrationInput{
		SourceEndpoint: req.SourceEndpoint, SourceRegion: req.SourceRegion, SourceBucket: req.SourceBucket,
		AccessKeyID: req.AccessKeyID, SecretAccessKey: req.SecretAccessKey, DestBucketID: req.DestBucketID,
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

func (h *apiHandlers) listMigrations(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.ListMigrations(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"migrations": rows})
}

func (h *apiHandlers) getMigration(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.GetMigration(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *apiHandlers) cancelMigration(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.CancelMigration(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
