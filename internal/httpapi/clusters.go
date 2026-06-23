package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/buktio/buktio/internal/service"
)

func (h *apiHandlers) listClusters(w http.ResponseWriter, r *http.Request) {
	clusters, err := h.svc.ListClusters(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listEnvelope{Data: clusters})
}

func (h *apiHandlers) getCluster(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.GetCluster(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

type addClusterReq struct {
	Name            string `json:"name"`
	Provider        string `json:"provider"`
	S3Endpoint      string `json:"s3_endpoint"`
	S3Region        string `json:"s3_region"`
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	PublicEndpoint  string `json:"public_endpoint"`
}

func (h *apiHandlers) addCluster(w http.ResponseWriter, r *http.Request) {
	var req addClusterReq
	if !decodeJSON(w, r, &req) {
		return
	}
	c, err := h.svc.AddCluster(r.Context(), service.AddClusterInput{
		Name:            req.Name,
		Provider:        req.Provider,
		S3Endpoint:      req.S3Endpoint,
		S3Region:        req.S3Region,
		AccessKeyID:     req.AccessKeyID,
		SecretAccessKey: req.SecretAccessKey,
		PublicEndpoint:  req.PublicEndpoint,
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (h *apiHandlers) removeCluster(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RemoveCluster(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
