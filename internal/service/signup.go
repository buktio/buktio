package service

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/buktio/buktio/internal/auth"
	"github.com/buktio/buktio/internal/entitlements"
)

const (
	signupWindow    = time.Hour
	signupPerIP     = 5 // max signups per IP per window
	verifyTokenTTL  = 24 * time.Hour
	verifyTokenPref = "bk_verify_"
)

// SignupResult is returned from Register. No session is issued until the email is
// verified — this enforces "verify before any action" by construction.
type SignupResult struct {
	UserID           string `json:"user_id"`
	VerificationSent bool   `json:"verification_sent"`
	// VerificationToken is populated only when SignupDevReturnToken is on (dev).
	VerificationToken string `json:"verification_token,omitempty"`
}

var slugInvalid = regexp.MustCompile(`[^a-z0-9-]+`)

// signupEnabled reports whether self-serve signup is active (config flag + license).
func (s *Services) signupEnabled() bool {
	return s.SelfServeSignup && s.Ent.Allowed(entitlements.FeatureSelfServeSignup, "").Allowed
}

// Register creates a new org + unverified owner and sends a verification email.
// ip is used for rate limiting. It deliberately returns no session.
func (s *Services) Register(ctx context.Context, email, password, orgName, ip string) (*SignupResult, error) {
	if !s.signupEnabled() {
		return nil, &Error{Code: "signup_disabled", Message: "self-serve signup is not enabled", HTTP: http.StatusNotFound}
	}
	email = strings.TrimSpace(strings.ToLower(email))
	if !validEmail(email) {
		return nil, validationErr("invalid email address")
	}
	if len(password) < 8 {
		return nil, validationErr("password must be at least 8 characters")
	}
	if strings.TrimSpace(orgName) == "" {
		return nil, validationErr("organization name is required")
	}

	// IP rate limit (defense against automated abuse).
	if ip != "" {
		if n, _ := s.Store.CountRecentSignups(ctx, ip, time.Now().Add(-signupWindow)); n >= signupPerIP {
			return nil, &Error{Code: "rate_limited", Message: "too many signups from this address; try again later", HTTP: http.StatusTooManyRequests}
		}
	}
	s.Store.RecordSignupAttempt(ctx, ip, email)

	// Enumeration-safe: never reveal whether the email is already registered. For an
	// existing account, return the same "verification sent" shape without creating
	// anything (a real mailer would email the existing owner a heads-up).
	if _, err := s.Store.GetUserByEmail(ctx, email); err == nil {
		return &SignupResult{VerificationSent: true}, nil
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	slug := orgSlug(orgName)
	_, _, userID, err := s.Store.CreateOrgWithOwner(ctx, orgName, slug, email, "", hash)
	if err != nil {
		return nil, conflictErr("could not create the organization (the name may be taken)")
	}

	raw, _ := auth.NewToken()
	token := verifyTokenPref + raw
	if err := s.Store.CreateEmailVerification(ctx, userID, auth.HashToken(token), time.Now().Add(verifyTokenTTL)); err != nil {
		return nil, mapRepoErr(err)
	}
	link := s.verifyLink(token)
	_ = s.Mailer.SendVerification(ctx, email, link)
	s.audit(ctx, "auth.signup", "user", userID, map[string]any{"email": email, "org": orgName})

	res := &SignupResult{UserID: userID, VerificationSent: true}
	if s.SignupDevReturnToken {
		res.VerificationToken = token
	}
	return res, nil
}

// VerifyEmail consumes a verification token, marks the user verified, and returns
// a session (auto-login on success).
func (s *Services) VerifyEmail(ctx context.Context, token string) (*AuthResult, error) {
	userID, err := s.Store.ConsumeEmailVerification(ctx, auth.HashToken(token))
	if err != nil {
		return nil, &Error{Code: "invalid_token", Message: "the verification link is invalid or has expired", HTTP: http.StatusBadRequest}
	}
	u, err := s.Store.GetUserByID(ctx, userID)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	s.audit(ctx, "auth.verify_email", "user", userID, map[string]any{"email": u.Email})
	return s.newSession(ctx, u)
}

// ResendVerification issues a fresh token for an unverified account. Always
// reports success to avoid email enumeration.
func (s *Services) ResendVerification(ctx context.Context, email string) error {
	if !s.signupEnabled() {
		return &Error{Code: "signup_disabled", Message: "self-serve signup is not enabled", HTTP: http.StatusNotFound}
	}
	email = strings.TrimSpace(strings.ToLower(email))
	userID, err := s.Store.UnverifiedUserByEmail(ctx, email)
	if err != nil {
		return nil // do not reveal whether the account exists / is verified
	}
	raw, _ := auth.NewToken()
	token := verifyTokenPref + raw
	if err := s.Store.CreateEmailVerification(ctx, userID, auth.HashToken(token), time.Now().Add(verifyTokenTTL)); err != nil {
		return mapRepoErr(err)
	}
	_ = s.Mailer.SendVerification(ctx, email, s.verifyLink(token))
	return nil
}

func (s *Services) verifyLink(token string) string {
	base := strings.TrimRight(s.PublicBaseURL, "/")
	return fmt.Sprintf("%s/signup/verify?token=%s", base, token)
}

func orgSlug(name string) string {
	slug := slugInvalid.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "org"
	}
	if len(slug) > 50 {
		slug = slug[:50]
	}
	// Add a short random suffix so concurrent signups don't collide on the slug.
	suffix, _ := auth.NewToken()
	return slug + "-" + strings.ToLower(suffix[:6])
}
