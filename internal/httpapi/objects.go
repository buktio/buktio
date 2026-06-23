package httpapi

import (
	"bytes"
	"io"
	"mime"
	"net/http"
	"path"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// maxUploadBytes caps API-proxied uploads (buffered to make the body seekable for
// SigV4). Larger uploads should use presigned URLs against a directly-reachable S3.
const maxUploadBytes = 200 << 20 // 200 MiB

func (h *apiHandlers) listObjects(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var maxKeys int32 = 100
	if v := q.Get("max_keys"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 {
			maxKeys = int32(n)
		}
	}
	res, err := h.svc.ListObjects(r.Context(), chi.URLParam(r, "id"),
		q.Get("prefix"), q.Get("delimiter"), q.Get("cursor"), maxKeys)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

type deleteObjectsReq struct {
	Keys []string `json:"keys"`
}

func (h *apiHandlers) deleteObjects(w http.ResponseWriter, r *http.Request) {
	var req deleteObjectsReq
	if !decodeJSON(w, r, &req) {
		return
	}
	permanent := r.URL.Query().Get("permanent") == "true"
	if err := h.svc.DeleteObjects(r.Context(), chi.URLParam(r, "id"), req.Keys, permanent); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *apiHandlers) listTrash(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListTrash(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listEnvelope{Data: items})
}

func (h *apiHandlers) restoreTrash(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RestoreObject(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "trashId")); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *apiHandlers) purgeTrash(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.PurgeObject(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "trashId")); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type presignReq struct {
	Operation   string `json:"operation"` // "get" | "put"
	Key         string `json:"key"`
	ExpiresIn   int    `json:"expires_in"`
	ContentType string `json:"content_type"`
}

type copyMoveReq struct {
	SrcKey string `json:"src_key"`
	DstKey string `json:"dst_key"`
}

func (h *apiHandlers) copyObject(w http.ResponseWriter, r *http.Request) {
	var req copyMoveReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.svc.CopyObject(r.Context(), chi.URLParam(r, "id"), req.SrcKey, req.DstKey); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *apiHandlers) moveObject(w http.ResponseWriter, r *http.Request) {
	var req copyMoveReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.svc.MoveObject(r.Context(), chi.URLParam(r, "id"), req.SrcKey, req.DstKey); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// uploadObject streams a raw body (PUT) into the bucket through the API.
func (h *apiHandlers) uploadObject(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	data, err := io.ReadAll(io.LimitReader(r.Body, maxUploadBytes))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errEnvelope{Error: errBody{
			Code: "validation_failed", Message: "could not read body: " + err.Error(),
			RequestID: middleware.GetReqID(r.Context()),
		}})
		return
	}
	ct := r.Header.Get("Content-Type")
	ssec := r.Header.Get("X-Buktio-SSEC-Key")
	if err := h.svc.PutObjectStream(r.Context(), chi.URLParam(r, "id"), key, bytes.NewReader(data), int64(len(data)), ct, ssec); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// downloadObject streams an object (GET) out through the API as an attachment.
func (h *apiHandlers) downloadObject(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	ssec := r.Header.Get("X-Buktio-SSEC-Key")
	rc, obj, err := h.svc.GetObjectStream(r.Context(), chi.URLParam(r, "id"), key, ssec)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	defer rc.Close()

	ct := mime.TypeByExtension(path.Ext(key))
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", `attachment; filename="`+path.Base(key)+`"`)
	if obj != nil && obj.Size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(obj.Size, 10))
	}
	_, _ = io.Copy(w, rc)
}

func (h *apiHandlers) presignObject(w http.ResponseWriter, r *http.Request) {
	var req presignReq
	if !decodeJSON(w, r, &req) {
		return
	}
	method := "GET"
	if req.Operation == "put" {
		method = "PUT"
	}
	url, err := h.svc.PresignObject(r.Context(), chi.URLParam(r, "id"), req.Key, method, req.ContentType, req.ExpiresIn)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"url": url, "method": method})
}
