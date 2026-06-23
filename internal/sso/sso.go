// Package sso is the single-sign-on seam. The OSS build uses Disabled (password
// login only); paid editions inject an OIDC (or SAML) IdentityProvider from ee/sso.
// Core never imports ee/ — the provider is wired via dependency injection.
package sso

import "context"

// ExternalIdentity is the verified identity returned by an IdP after a successful
// login. Groups carry IdP group memberships for role mapping.
type ExternalIdentity struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	Groups        []string
}

// IdentityProvider performs an OAuth2/OIDC authorization-code flow.
type IdentityProvider interface {
	// Enabled reports whether SSO is configured (false => only password login).
	Enabled() bool
	// AuthURL is where to redirect the browser to start login (with an anti-CSRF state).
	AuthURL(state string) string
	// Exchange swaps an authorization code for a verified identity.
	Exchange(ctx context.Context, code string) (*ExternalIdentity, error)
	// MapRole returns the buktio role for the given IdP groups (default "member").
	MapRole(groups []string) string
}

// Disabled is the OSS no-SSO provider.
type Disabled struct{}

func (Disabled) Enabled() bool         { return false }
func (Disabled) AuthURL(string) string { return "" }
func (Disabled) Exchange(context.Context, string) (*ExternalIdentity, error) {
	return nil, errNotEnabled
}
func (Disabled) MapRole([]string) string { return "member" }

type ssoError string

func (e ssoError) Error() string { return string(e) }

const errNotEnabled = ssoError("sso is not enabled")
