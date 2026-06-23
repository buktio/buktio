package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/buktio/buktio/internal/service"
)

type createBucketReq struct {
	Name            string `json:"name"`
	QuotaMaxBytes   *int64 `json:"quota_max_bytes"`
	QuotaMaxObjects *int64 `json:"quota_max_objects"`
	ClusterID       string `json:"cluster_id"` // optional; empty = primary cluster
}

func (h *apiHandlers) createBucket(w http.ResponseWriter, r *http.Request) {
	var req createBucketReq
	if !decodeJSON(w, r, &req) {
		return
	}
	b, err := h.svc.CreateBucket(r.Context(), req.Name, service.QuotaDTO{
		MaxBytes: req.QuotaMaxBytes, MaxObjects: req.QuotaMaxObjects,
	}, req.ClusterID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, b)
}

func (h *apiHandlers) listBuckets(w http.ResponseWriter, r *http.Request) {
	buckets, err := h.svc.ListBuckets(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listEnvelope{Data: buckets})
}

func (h *apiHandlers) getBucket(w http.ResponseWriter, r *http.Request) {
	b, err := h.svc.GetBucket(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (h *apiHandlers) getBucketUsage(w http.ResponseWriter, r *http.Request) {
	b, err := h.svc.GetBucket(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, b.Usage)
}

type patchBucketReq struct {
	QuotaMaxBytes   *int64 `json:"quota_max_bytes"`
	QuotaMaxObjects *int64 `json:"quota_max_objects"`
}

func (h *apiHandlers) patchBucket(w http.ResponseWriter, r *http.Request) {
	var req patchBucketReq
	if !decodeJSON(w, r, &req) {
		return
	}
	b, err := h.svc.SetQuota(r.Context(), chi.URLParam(r, "id"), service.QuotaDTO{
		MaxBytes: req.QuotaMaxBytes, MaxObjects: req.QuotaMaxObjects,
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

type setAccessReq struct {
	Public        bool   `json:"public"`
	IndexDocument string `json:"index_document"`
	ErrorDocument string `json:"error_document"`
}

func (h *apiHandlers) setBucketAccess(w http.ResponseWriter, r *http.Request) {
	var req setAccessReq
	if !decodeJSON(w, r, &req) {
		return
	}
	b, err := h.svc.SetVisibility(r.Context(), chi.URLParam(r, "id"), req.Public, req.IndexDocument, req.ErrorDocument)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (h *apiHandlers) deleteBucket(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteBucket(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
