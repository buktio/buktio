package httpapi

import "net/http"

func (h *apiHandlers) garageMetrics(w http.ResponseWriter, r *http.Request) {
	body, err := h.svc.GarageMetrics(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = w.Write(body)
}
