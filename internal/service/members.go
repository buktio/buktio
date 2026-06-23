package service

import (
	"context"
	"net/http"
	"time"

	"github.com/buktio/buktio/internal/auth"
	"github.com/buktio/buktio/internal/authz"
	"github.com/buktio/buktio/internal/entitlements"
	"github.com/buktio/buktio/internal/repository"
)

// inviteTTL is how long an invitation link is valid.
const inviteTTL = 7 * 24 * time.Hour

// MemberDTO is an org member as the API returns it.
type MemberDTO struct {
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	FullName  string    `json:"full_name,omitempty"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// InvitationDTO is a pending invite.
type InvitationDTO struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	ExpiresAt time.Time `json:"expires_at"`
}

// InviteResult is returned once at invite creation; Link is shown to the inviter.
type InviteResult struct {
	InvitationDTO
	Link string `json:"link"`
}

var validRoles = map[string]bool{"owner": true, "admin": true, "member": true, "viewer": true}

func roleRank(r string) int {
	switch r {
	case "owner":
		return 3
	case "admin":
		return 2
	case "member":
		return 1
	default:
		return 0
	}
}

// guardRoleCeiling blocks granting a role higher than the acting user's own — an
// admin may not mint owners/admins above themselves (privilege escalation).
func (s *Services) guardRoleCeiling(ctx context.Context, requested string) *Error {
	subj, _ := authz.SubjectFrom(ctx)
	if roleRank(requested) > roleRank(string(subj.Role)) {
		return &Error{Code: "forbidden", Message: "you cannot grant a role higher than your own", HTTP: http.StatusForbidden}
	}
	return nil
}

// ListMembers returns the active org's members + pending invitations.
func (s *Services) ListMembers(ctx context.Context) ([]MemberDTO, []InvitationDTO, error) {
	if err := s.authorize(ctx, authz.ActionRead, authz.Target{Kind: authz.ResourceMember}); err != nil {
		return nil, nil, err
	}
	org := s.tenant(ctx).OrgID
	rows, err := s.Store.ListMembers(ctx, org)
	if err != nil {
		return nil, nil, mapRepoErr(err)
	}
	members := make([]MemberDTO, 0, len(rows))
	for _, m := range rows {
		members = append(members, MemberDTO{UserID: m.UserID, Email: m.Email, FullName: m.FullName, Role: m.Role, CreatedAt: m.CreatedAt})
	}
	invRows, _ := s.Store.ListInvitations(ctx, org)
	invites := make([]InvitationDTO, 0, len(invRows))
	for _, iv := range invRows {
		invites = append(invites, InvitationDTO{ID: iv.ID, Email: iv.Email, Role: iv.Role, ExpiresAt: iv.ExpiresAt})
	}
	return members, invites, nil
}

// InviteMember creates a pending invite (enforcing the seat limit) and returns a
// one-time accept link. The link is also emailed via the (OSS log-only) mailer.
func (s *Services) InviteMember(ctx context.Context, email, role string) (*InviteResult, error) {
	if err := s.authorize(ctx, authz.ActionUpdate, authz.Target{Kind: authz.ResourceMember}); err != nil {
		return nil, err
	}
	if !validEmail(email) {
		return nil, validationErr("invalid email address")
	}
	if role == "" {
		role = "member"
	}
	if !validRoles[role] {
		return nil, validationErr("role must be owner, admin, member, or viewer")
	}
	if err := s.guardRoleCeiling(ctx, role); err != nil {
		return nil, err
	}
	org := s.tenant(ctx).OrgID

	// Seat limit: members + pending invites must stay within the entitlement.
	if lim := s.Ent.Limit(entitlements.ResourceSeats, entitlements.TenantID(org)); !lim.Unlimited {
		members, _ := s.Store.CountMembers(ctx, org)
		invites, _ := s.Store.ListInvitations(ctx, org)
		if int64(members+len(invites)) >= lim.Max {
			return nil, &Error{Code: "seat_limit_reached", Message: "your plan's seat limit has been reached", HTTP: http.StatusPaymentRequired}
		}
	}

	token, err := auth.NewToken()
	if err != nil {
		return nil, mapRepoErr(err)
	}
	id, err := s.Store.CreateInvitation(ctx, org, email, role, auth.HashToken(token), currentUserID(ctx), time.Now().Add(inviteTTL))
	if err != nil {
		return nil, mapRepoErr(err)
	}
	link := s.PublicBaseURL + "/accept-invite?token=" + token
	_ = s.Mailer.SendInvite(ctx, email, link)
	s.audit(ctx, "member.invite", "invitation", id, map[string]any{"email": email, "role": role})
	return &InviteResult{
		InvitationDTO: InvitationDTO{ID: id, Email: email, Role: role, ExpiresAt: time.Now().Add(inviteTTL)},
		Link:          link,
	}, nil
}

// ChangeMemberRole updates a member's role (blocking demotion of the last owner).
func (s *Services) ChangeMemberRole(ctx context.Context, userID, role string) error {
	if err := s.authorize(ctx, authz.ActionUpdate, authz.Target{Kind: authz.ResourceMember, ID: userID}); err != nil {
		return err
	}
	if !validRoles[role] {
		return validationErr("invalid role")
	}
	if err := s.guardRoleCeiling(ctx, role); err != nil {
		return err
	}
	org := s.tenant(ctx).OrgID
	if role != "owner" {
		if err := s.guardLastOwner(ctx, org, userID); err != nil {
			return err
		}
	}
	if err := s.Store.UpsertMember(ctx, org, userID, role); err != nil {
		return mapRepoErr(err)
	}
	s.audit(ctx, "member.role_change", "user", userID, map[string]any{"role": role})
	return nil
}

// RemoveMember removes a member (blocking removal of the last owner).
func (s *Services) RemoveMember(ctx context.Context, userID string) error {
	if err := s.authorize(ctx, authz.ActionDelete, authz.Target{Kind: authz.ResourceMember, ID: userID}); err != nil {
		return err
	}
	org := s.tenant(ctx).OrgID
	if err := s.guardLastOwner(ctx, org, userID); err != nil {
		return err
	}
	if err := s.Store.RemoveMember(ctx, org, userID); err != nil {
		return mapRepoErr(err)
	}
	s.audit(ctx, "member.remove", "user", userID, nil)
	return nil
}

// guardLastOwner blocks demoting/removing the org's last owner.
func (s *Services) guardLastOwner(ctx context.Context, orgID, userID string) *Error {
	members, _ := s.Store.ListMembers(ctx, orgID)
	owners, target := 0, ""
	for _, m := range members {
		if m.Role == "owner" {
			owners++
			if m.UserID == userID {
				target = m.Role
			}
		}
	}
	if owners <= 1 && target == "owner" {
		return conflictErr("cannot remove or demote the last owner")
	}
	return nil
}

// AcceptInvite consumes an invite: it creates or links the user, adds the
// membership, and returns a fresh session. Public (no auth).
func (s *Services) AcceptInvite(ctx context.Context, token, password, fullName string) (*AuthResult, error) {
	iv, err := s.Store.GetInvitationByToken(ctx, auth.HashToken(token))
	if err != nil {
		return nil, &Error{Code: "invalid_invite", Message: "invitation is invalid or expired", HTTP: http.StatusBadRequest}
	}

	var userID string
	if existing, gerr := s.Store.GetUserByEmail(ctx, iv.Email); gerr == nil {
		userID = existing.ID // already a user — just add the membership
	} else {
		if len(password) < 8 {
			return nil, validationErr("password must be at least 8 characters")
		}
		hash, herr := auth.HashPassword(password)
		if herr != nil {
			return nil, mapRepoErr(herr)
		}
		userID, err = s.Store.CreateUser(ctx, iv.Email, fullName, hash, false)
		if err != nil {
			return nil, mapRepoErr(err)
		}
	}

	if err := s.Store.UpsertMember(ctx, iv.OrgID, userID, iv.Role); err != nil {
		return nil, mapRepoErr(err)
	}
	_ = s.Store.MarkInvitationAccepted(ctx, iv.ID)
	s.audit(ctx, "member.accept_invite", "user", userID, map[string]any{"org_id": iv.OrgID, "role": iv.Role})
	return s.newSession(ctx, &repository.User{ID: userID, Email: iv.Email, FullName: fullName})
}
