package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/buktio/buktio/internal/service"
)

const maxBodyBytes = 2 << 20 // 2 MiB JSON cap

type errEnvelope struct {
	Error errBody `json:"error"`
}

type errBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

type listEnvelope struct {
	Data any `json:"data"`
}

// writeErr maps a service.Error (or any error) to the API error envelope.
func writeErr(w http.ResponseWriter, r *http.Request, err error) {
	reqID := middleware.GetReqID(r.Context())
	var se *service.Error
	if errors.As(err, &se) {
		writeJSON(w, se.HTTP, errEnvelope{Error: errBody{Code: se.Code, Message: se.Message, RequestID: reqID}})
		return
	}
	writeJSON(w, http.StatusInternalServerError, errEnvelope{
		Error: errBody{Code: "internal_error", Message: "internal error", RequestID: reqID},
	})
}

// decodeJSON reads and unmarshals a JSON request body (size-limited).
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil && !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, errEnvelope{Error: errBody{
			Code: "validation_failed", Message: "invalid JSON body: " + err.Error(),
			RequestID: middleware.GetReqID(r.Context()),
		}})
		return false
	}
	return true
}
