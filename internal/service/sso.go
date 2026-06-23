package service

import (
	"context"

	"github.com/buktio/buktio/internal/auth"
	"github.com/buktio/buktio/internal/entitlements"
	"github.com/buktio/buktio/internal/repository"
	"github.com/buktio/buktio/internal/sso"
)

const ssoProvider = "oidc"

// SSOEnabled reports whether an SSO provider is configured.
func (s *Services) SSOEnabled() bool { return s.IdP != nil && s.IdP.Enabled() }

// SSOAuthURL returns the IdP redirect URL for the given state.
func (s *Services) SSOAuthURL(state string) string {
	if !s.SSOEnabled() {
		return ""
	}
	return s.IdP.AuthURL(state)
}

// SSOExchange verifies the authorization code into an external identity.
func (s *Services) SSOExchange(ctx context.Context, code string) (*sso.ExternalIdentity, error) {
	if !s.SSOEnabled() {
		return nil, unauthorizedErr()
	}
	ext, err := s.IdP.Exchange(ctx, code)
	if err != nil {
		return nil, unauthorizedErr()
	}
	return ext, nil
}

// LoginWithExternalIdentity find-or-creates a user for the verified identity, links
// it, ensures org membership (role from the IdP group mapping), and returns a
// session. First sign-in auto-provisions the user.
func (s *Services) LoginWithExternalIdentity(ctx context.Context, ext *sso.ExternalIdentity) (*AuthResult, error) {
	if ext == nil || ext.Subject == "" || ext.Email == "" {
		return nil, unauthorizedErr()
	}

	var u *repository.User
	if linked, err := s.Store.GetUserByExternalIdentity(ctx, ssoProvider, ext.Subject); err == nil {
		u = linked
	} else if byEmail, eerr := s.Store.GetUserByEmail(ctx, ext.Email); eerr == nil {
		// Only link to a pre-existing local account when the IdP asserts the email is
		// verified — otherwise an IdP with a spoofable email claim could hijack it.
		if !ext.EmailVerified {
			return nil, unauthorizedErr()
		}
		u = byEmail
		_ = s.Store.LinkIdentity(ctx, u.ID, ssoProvider, ext.Subject)
	} else {
		// Auto-provision: random (unusable) password so password login is disabled.
		rnd, _ := auth.NewToken()
		hash, herr := auth.HashPassword(rnd)
		if herr != nil {
			return nil, mapRepoErr(herr)
		}
		id, cerr := s.Store.CreateUser(ctx, ext.Email, ext.Name, hash, false)
		if cerr != nil {
			return nil, mapRepoErr(cerr)
		}
		_ = s.Store.LinkIdentity(ctx, id, ssoProvider, ext.Subject)
		u = &repository.User{ID: id, Email: ext.Email, FullName: ext.Name}
	}

	// Membership: only set the role on FIRST provisioning (do not overwrite — and
	// thus never silently demote — an existing member on every login). Enforce the
	// seat limit before adding a new member.
	if _, merr := s.Store.GetMembershipRole(ctx, s.OrgID, u.ID); merr != nil {
		if lim := s.Ent.Limit(entitlements.ResourceSeats, entitlements.TenantID(s.OrgID)); !lim.Unlimited {
			if n, _ := s.Store.CountMembers(ctx, s.OrgID); int64(n) >= lim.Max {
				return nil, &Error{Code: "seat_limit_reached", Message: "seat limit reached; ask an admin to add a seat", HTTP: 402}
			}
		}
		role := "member"
		if s.IdP != nil {
			role = s.IdP.MapRole(ext.Groups)
		}
		_ = s.Store.UpsertMember(ctx, s.OrgID, u.ID, role)
	}

	s.audit(ctx, "auth.sso_login", "user", u.ID, map[string]any{"email": ext.Email})
	return s.newSession(ctx, u)
}
