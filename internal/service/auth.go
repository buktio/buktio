package service

import (
	"context"
	"net/http"
	"time"

	"github.com/buktio/buktio/internal/auth"
	"github.com/buktio/buktio/internal/repository"
)

// SessionTTL is how long a session is valid.
const SessionTTL = 7 * 24 * time.Hour

// UserDTO is a user as the API returns it.
type UserDTO struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	FullName string `json:"full_name,omitempty"`
	Role     string `json:"role"`
}

// AuthResult carries the session + CSRF tokens the handler turns into cookies.
type AuthResult struct {
	User         *UserDTO `json:"user"`
	SessionToken string   `json:"-"`
	CSRFToken    string   `json:"-"`
}

func userDTO(u *repository.User) *UserDTO {
	role := "admin"
	if u.IsPlatformAdmin {
		role = "owner"
	}
	return &UserDTO{ID: u.ID, Email: u.Email, FullName: u.FullName, Role: role}
}

// SetupInitialized reports whether the first admin has been created.
func (s *Services) SetupInitialized(ctx context.Context) (bool, error) {
	n, err := s.Store.CountUsers(ctx)
	return n > 0, err
}

// CreateAdmin creates the first platform-admin user (only allowed once) and
// returns a fresh session.
func (s *Services) CreateAdmin(ctx context.Context, email, password, fullName string) (*AuthResult, error) {
	n, err := s.Store.CountUsers(ctx)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	if n > 0 {
		return nil, &Error{Code: "setup_already_complete", Message: "an admin account already exists", HTTP: 409}
	}
	if !validEmail(email) {
		return nil, validationErr("invalid email address")
	}
	if len(password) < 8 {
		return nil, validationErr("password must be at least 8 characters")
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	id, err := s.Store.CreateUser(ctx, email, fullName, hash, true)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	// The first admin is the owner of the default org (so RBAC resolves a real role).
	_ = s.Store.UpsertMember(ctx, s.OrgID, id, "owner")
	_ = s.Store.SetInstallStep(ctx, "completed")
	s.audit(ctx, "auth.create_admin", "user", id, map[string]any{"email": email})
	return s.newSession(ctx, &repository.User{ID: id, Email: email, FullName: fullName, IsPlatformAdmin: true})
}

// Login verifies credentials and returns a fresh session. Errors are generic to
// avoid user enumeration.
func (s *Services) Login(ctx context.Context, email, password string) (*AuthResult, error) {
	u, err := s.Store.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, unauthorizedErr()
	}
	ok, err := auth.VerifyPassword(password, u.PasswordHash)
	if err != nil || !ok {
		return nil, unauthorizedErr()
	}
	// Block password login until the email is verified (self-serve signup). Existing
	// users default to verified, so OSS/on-prem is unaffected.
	if !u.EmailVerified {
		return nil, &Error{Code: "email_not_verified", Message: "please verify your email before signing in", HTTP: http.StatusForbidden}
	}
	_ = s.Store.TouchUserLogin(ctx, u.ID)
	return s.newSession(ctx, u)
}

func (s *Services) newSession(ctx context.Context, u *repository.User) (*AuthResult, error) {
	token, err := auth.NewToken()
	if err != nil {
		return nil, mapRepoErr(err)
	}
	csrf, err := auth.NewToken()
	if err != nil {
		return nil, mapRepoErr(err)
	}
	if _, err := s.Store.CreateSession(ctx, u.ID, auth.HashToken(token), time.Now().Add(SessionTTL)); err != nil {
		return nil, mapRepoErr(err)
	}
	return &AuthResult{User: userDTO(u), SessionToken: token, CSRFToken: csrf}, nil
}

// Logout revokes a session by its token.
func (s *Services) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.Store.RevokeSessionByToken(ctx, auth.HashToken(token))
}

// SessionUser resolves the user for a session token (used by the auth middleware).
func (s *Services) SessionUser(ctx context.Context, token string) (*repository.User, error) {
	return s.Store.GetUserBySessionToken(ctx, auth.HashToken(token))
}
