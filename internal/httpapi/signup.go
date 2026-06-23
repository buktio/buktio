package httpapi

import (
	"net/http"
)

type signupReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	OrgName  string `json:"org_name"`
}

// signup creates an org + unverified owner and emails a verification link. Public.
func (h *apiHandlers) signup(w http.ResponseWriter, r *http.Request) {
	var req signupReq
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := h.svc.Register(r.Context(), req.Email, req.Password, req.OrgName, clientIP(r))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

type verifyReq struct {
	Token string `json:"token"`
}

// signupVerify consumes a verification token and logs the user in. Public.
func (h *apiHandlers) signupVerify(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		var req verifyReq
		if !decodeJSON(w, r, &req) {
			return
		}
		token = req.Token
	}
	res, err := h.svc.VerifyEmail(r.Context(), token)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	setAuthCookies(w, r, res)
	writeJSON(w, http.StatusOK, res)
}

type resendReq struct {
	Email string `json:"email"`
}

// signupResend re-sends a verification link. Public; always reports success.
func (h *apiHandlers) signupResend(w http.ResponseWriter, r *http.Request) {
	var req resendReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.svc.ResendVerification(r.Context(), req.Email); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
