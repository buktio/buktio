package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (h *apiHandlers) listMembers(w http.ResponseWriter, r *http.Request) {
	members, invites, err := h.svc.ListMembers(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": members, "invitations": invites})
}

type inviteReq struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

func (h *apiHandlers) inviteMember(w http.ResponseWriter, r *http.Request) {
	var req inviteReq
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := h.svc.InviteMember(r.Context(), req.Email, req.Role)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

type roleReq struct {
	Role string `json:"role"`
}

func (h *apiHandlers) changeMemberRole(w http.ResponseWriter, r *http.Request) {
	var req roleReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.svc.ChangeMemberRole(r.Context(), chi.URLParam(r, "userId"), req.Role); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *apiHandlers) removeMember(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RemoveMember(r.Context(), chi.URLParam(r, "userId")); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type acceptInviteReq struct {
	Token    string `json:"token"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
}

func (h *apiHandlers) acceptInvite(w http.ResponseWriter, r *http.Request) {
	var req acceptInviteReq
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := h.svc.AcceptInvite(r.Context(), req.Token, req.Password, req.FullName)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	setAuthCookies(w, r, res)
	writeJSON(w, http.StatusOK, map[string]any{"user": res.User})
}
