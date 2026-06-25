package service

import (
	"context"
	"strings"
	"time"

	"github.com/buktio/buktio/internal/auth"
	"github.com/buktio/buktio/internal/authz"
	"github.com/buktio/buktio/internal/repository"
)

// APITokenPrefix marks buktio personal access tokens.
const APITokenPrefix = "bk_pat_"

// APITokenDTO is a token as the API returns it (never includes the secret).
type APITokenDTO struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	SecretLastFour string     `json:"secret_last_four,omitempty"`
	Scopes         []string   `json:"scopes"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	LastUsedAt     *time.Time `json:"last_used_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// APITokenCreateResult is returned once at creation and includes the raw token.
type APITokenCreateResult struct {
	APITokenDTO
	Token           string `json:"token"`
	SecretShownOnce bool   `json:"secret_shown_once"`
}

func currentUserID(ctx context.Context) string {
	if subj, ok := authz.SubjectFrom(ctx); ok {
		return subj.UserID
	}
	return ""
}

// CreateAPIToken issues a PAT (shown once) for the current user.
func (s *Services) CreateAPIToken(ctx context.Context, name string, expiresAt *time.Time) (*APITokenCreateResult, error) {
	uid := currentUserID(ctx)
	if uid == "" {
		return nil, unauthorizedErr()
	}
	if err := s.authorize(ctx, authz.ActionCreate, authz.Target{Kind: authz.ResourceAPIToken}); err != nil {
		return nil, err
	}
	if strings.TrimSpace(name) == "" {
		return nil, validationErr("token name is required")
	}
	raw, err := auth.NewToken()
	if err != nil {
		return nil, mapRepoErr(err)
	}
	token := APITokenPrefix + raw
	lastFour := token[len(token)-4:]
	id, err := s.Store.CreateAPIToken(ctx, uid, s.tenant(ctx).OrgID, name, auth.HashToken(token), lastFour, expiresAt)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	s.audit(ctx, "api_token.create", "api_token", id, map[string]any{"name": name})
	return &APITokenCreateResult{
		APITokenDTO:     APITokenDTO{ID: id, Name: name, SecretLastFour: lastFour, Scopes: []string{}, ExpiresAt: expiresAt, CreatedAt: time.Now().UTC()},
		Token:           token,
		SecretShownOnce: true,
	}, nil
}

// ListAPITokens returns the current user's tokens.
func (s *Services) ListAPITokens(ctx context.Context) ([]APITokenDTO, error) {
	uid := currentUserID(ctx)
	if uid == "" {
		return nil, unauthorizedErr()
	}
	rows, err := s.Store.ListAPITokens(ctx, uid)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	out := make([]APITokenDTO, 0, len(rows))
	for _, t := range rows {
		scopes := t.Scopes
		if scopes == nil {
			scopes = []string{}
		}
		out = append(out, APITokenDTO{
			ID: t.ID, Name: t.Name, SecretLastFour: t.SecretLastFour, Scopes: scopes,
			ExpiresAt: t.ExpiresAt, LastUsedAt: t.LastUsedAt, CreatedAt: t.CreatedAt,
		})
	}
	return out, nil
}

// RevokeAPIToken deletes one of the current user's tokens.
func (s *Services) RevokeAPIToken(ctx context.Context, id string) error {
	uid := currentUserID(ctx)
	if uid == "" {
		return unauthorizedErr()
	}
	if err := s.authorize(ctx, authz.ActionDelete, authz.Target{Kind: authz.ResourceAPIToken, ID: id}); err != nil {
		return err
	}
	if err := s.Store.SoftDeleteAPIToken(ctx, id, uid); err != nil {
		return mapRepoErr(err)
	}
	s.audit(ctx, "api_token.revoke", "api_token", id, nil)
	return nil
}

// UserByAPIToken resolves the user for a bearer PAT (used by the auth middleware).
func (s *Services) UserByAPIToken(ctx context.Context, token string) (*repository.User, error) {
	if !strings.HasPrefix(token, APITokenPrefix) {
		return nil, repository.ErrNotFound
	}
	return s.Store.GetUserByAPIToken(ctx, auth.HashToken(token))
}
