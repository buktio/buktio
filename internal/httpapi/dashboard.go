package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/buktio/buktio/internal/service"
)

func (h *apiHandlers) dashboard(w http.ResponseWriter, r *http.Request) {
	d, err := h.svc.Dashboard(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (h *apiHandlers) trafficUsage(w http.ResponseWriter, r *http.Request) {
	hours := 24
	if v := r.URL.Query().Get("hours"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			hours = n
		}
	}
	rows, err := h.svc.TrafficUsage(r.Context(), hours)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listEnvelope{Data: rows})
}

func auditFilterFromQuery(r *http.Request) service.AuditFilterInput {
	q := r.URL.Query()
	f := service.AuditFilterInput{
		Actor: q.Get("actor"), Action: q.Get("action"), TargetType: q.Get("target"),
	}
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.Limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.Offset = n
		}
	}
	if v := q.Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.From = t
		}
	}
	if v := q.Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.To = t
		}
	}
	return f
}

func (h *apiHandlers) listAudit(w http.ResponseWriter, r *http.Request) {
	entries, err := h.svc.ListAudit(r.Context(), auditFilterFromQuery(r))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listEnvelope{Data: entries})
}

func (h *apiHandlers) verifyAudit(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.VerifyAuditChain(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *apiHandlers) exportAudit(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format != "json" {
		format = "csv"
	}
	data, contentType, err := h.svc.ExportAudit(r.Context(), auditFilterFromQuery(r), format)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "attachment; filename=buktio-audit."+format)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *apiHandlers) docsSnippets(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	snip, err := h.svc.Snippets(r.Context(), q.Get("bucket_id"), q.Get("access_key_id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, snip)
}
