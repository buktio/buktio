package httpapi

import (
	"io"
	"net/http"
)

func (h *apiHandlers) billingStatus(w http.ResponseWriter, r *http.Request) {
	st, err := h.svc.BillingStatus(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

type setupBillingReq struct {
	Email string `json:"email"`
}

func (h *apiHandlers) setupBilling(w http.ResponseWriter, r *http.Request) {
	var req setupBillingReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.svc.SetupBilling(r.Context(), req.Email); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *apiHandlers) triggerBillingReport(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.TriggerBillingReport(r.Context()); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// billingWebhook receives processor webhooks. Public (the Provider verifies the
// signature). The raw body must be read for signature verification.
func (h *apiHandlers) billingWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	sig := r.Header.Get("Stripe-Signature")
	if err := h.svc.HandleBillingWebhook(r.Context(), body, sig); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}
