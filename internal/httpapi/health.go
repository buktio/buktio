package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
)

type healthHandler struct {
	version string
	probe   ReadinessProbe
}

// live is the cheap liveness probe — always 200 if the process is up. Suitable
// for load-balancer / k8s liveness checks.
func (h *healthHandler) live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": h.version,
	})
}

// ready reports readiness of buktio's dependencies (DB, Garage S3/Admin). Returns
// 503 if any required dependency is down.
func (h *healthHandler) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), readyTimeout)
	defer cancel()

	ready := true
	components := map[string]string{}
	if h.probe != nil {
		ready, components = h.probe.Check(ctx)
	}

	status := http.StatusOK
	overall := "ok"
	if !ready {
		status = http.StatusServiceUnavailable
		overall = "not_ready"
	}
	writeJSON(w, status, map[string]any{
		"status":     overall,
		"version":    h.version,
		"components": components,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
