package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (h *apiHandlers) getVersioning(w http.ResponseWriter, r *http.Request) {
	on, err := h.svc.GetBucketVersioning(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"enabled": on})
}

type setVersioningReq struct {
	Enabled bool `json:"enabled"`
}

func (h *apiHandlers) setVersioning(w http.ResponseWriter, r *http.Request) {
	var req setVersioningReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.svc.SetBucketVersioning(r.Context(), chi.URLParam(r, "id"), req.Enabled); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"enabled": req.Enabled})
}

func (h *apiHandlers) listVersions(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.ListObjectVersions(r.Context(), chi.URLParam(r, "id"), r.URL.Query().Get("prefix"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listEnvelope{Data: rows})
}

type versionRef struct {
	Key       string `json:"key"`
	VersionID string `json:"version_id"`
}

func (h *apiHandlers) restoreVersion(w http.ResponseWriter, r *http.Request) {
	var req versionRef
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.svc.RestoreObjectVersion(r.Context(), chi.URLParam(r, "id"), req.Key, req.VersionID); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *apiHandlers) deleteVersion(w http.ResponseWriter, r *http.Request) {
	var req versionRef
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.svc.DeleteObjectVersion(r.Context(), chi.URLParam(r, "id"), req.Key, req.VersionID); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
