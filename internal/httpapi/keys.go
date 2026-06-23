package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/buktio/buktio/internal/service"
)

type grantReq struct {
	BucketID string `json:"bucket_id"`
	Read     bool   `json:"read"`
	Write    bool   `json:"write"`
	Owner    bool   `json:"owner"`
}

type createKeyReq struct {
	Name              string     `json:"name"`
	AllowCreateBucket bool       `json:"allow_create_bucket"`
	Grants            []grantReq `json:"grants"`
}

func (h *apiHandlers) createKey(w http.ResponseWriter, r *http.Request) {
	var req createKeyReq
	if !decodeJSON(w, r, &req) {
		return
	}
	grants := make([]service.GrantInput, 0, len(req.Grants))
	for _, g := range req.Grants {
		grants = append(grants, service.GrantInput{BucketID: g.BucketID, Read: g.Read, Write: g.Write, Owner: g.Owner})
	}
	res, err := h.svc.CreateAccessKey(r.Context(), req.Name, req.AllowCreateBucket, grants)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

func (h *apiHandlers) listKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := h.svc.ListAccessKeys(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listEnvelope{Data: keys})
}

func (h *apiHandlers) getKey(w http.ResponseWriter, r *http.Request) {
	k, err := h.svc.GetAccessKey(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, k)
}

func (h *apiHandlers) deleteKey(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteAccessKey(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *apiHandlers) grantKey(w http.ResponseWriter, r *http.Request) {
	var req grantReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.svc.GrantBucket(r.Context(), chi.URLParam(r, "id"), service.GrantInput{
		BucketID: req.BucketID, Read: req.Read, Write: req.Write, Owner: req.Owner,
	}); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *apiHandlers) revokeKey(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RevokeBucket(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "bucketId")); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
