package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/buktio/buktio/internal/service"
)

func (h *apiHandlers) listNodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := h.svc.ListNodes(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listEnvelope{Data: nodes})
}

type addNodeReq struct {
	NodeID        string `json:"node_id"`
	Peer          string `json:"peer"`
	Zone          string `json:"zone"`
	CapacityBytes int64  `json:"capacity_bytes"`
}

func (h *apiHandlers) addNode(w http.ResponseWriter, r *http.Request) {
	var req addNodeReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.svc.AddNode(r.Context(), chi.URLParam(r, "id"), service.AddNodeInput{
		NodeID:        req.NodeID,
		Peer:          req.Peer,
		Zone:          req.Zone,
		CapacityBytes: req.CapacityBytes,
	}); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *apiHandlers) removeNode(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RemoveNode(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "nodeId")); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *apiHandlers) getLayout(w http.ResponseWriter, r *http.Request) {
	l, err := h.svc.GetLayout(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, l)
}

func (h *apiHandlers) previewLayout(w http.ResponseWriter, r *http.Request) {
	msgs, err := h.svc.PreviewLayout(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": msgs})
}

func (h *apiHandlers) revertLayout(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RevertLayout(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
