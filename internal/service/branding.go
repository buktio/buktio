package service

import (
	"context"
	"errors"
	"strings"

	"github.com/buktio/buktio/internal/authz"
	"github.com/buktio/buktio/internal/repository"
)

// BrandingDTO is an org's white-label settings as the API returns them.
type BrandingDTO struct {
	DisplayName  string `json:"display_name,omitempty"`
	LogoURL      string `json:"logo_url,omitempty"`
	PrimaryColor string `json:"primary_color,omitempty"`
	EmailFrom    string `json:"email_from,omitempty"`
	CustomDomain string `json:"custom_domain,omitempty"`
}

func brandingDTO(b *repository.Branding) BrandingDTO {
	return BrandingDTO{
		DisplayName: b.DisplayName, LogoURL: b.LogoURL, PrimaryColor: b.PrimaryColor,
		EmailFrom: b.EmailFrom, CustomDomain: b.CustomDomain,
	}
}

// GetBranding returns the active org's branding (empty DTO when unset). Any
// authenticated member may read it (the UI themes itself from it).
func (s *Services) GetBranding(ctx context.Context) (BrandingDTO, error) {
	b, err := s.Store.GetBranding(ctx, s.tenant(ctx).OrgID)
	if errors.Is(err, repository.ErrNotFound) {
		return BrandingDTO{}, nil
	}
	if err != nil {
		return BrandingDTO{}, mapRepoErr(err)
	}
	return brandingDTO(b), nil
}

// BrandingForHost resolves branding for a request Host (a custom domain). Public,
// pre-login: returns an empty DTO when the host isn't a registered custom domain.
func (s *Services) BrandingForHost(ctx context.Context, host string) (BrandingDTO, error) {
	host = normalizeHost(host)
	if host == "" {
		return BrandingDTO{}, nil
	}
	b, err := s.Store.GetBrandingByDomain(ctx, host)
	if errors.Is(err, repository.ErrNotFound) {
		return BrandingDTO{}, nil
	}
	if err != nil {
		return BrandingDTO{}, mapRepoErr(err)
	}
	return brandingDTO(b), nil
}

// SetBrandingInput is the update payload.
type SetBrandingInput struct {
	DisplayName  string
	LogoURL      string
	PrimaryColor string
	EmailFrom    string
	CustomDomain string
}

// SetBranding replaces the active org's branding (owner or platform admin).
func (s *Services) SetBranding(ctx context.Context, in SetBrandingInput) error {
	subj, _ := authz.SubjectFrom(ctx)
	if !subj.PlatformAdmin && subj.Role != authz.RoleOwner {
		return &Error{Code: "forbidden", Message: "owner or platform administrator privilege required", HTTP: 403}
	}
	orgID := s.tenant(ctx).OrgID
	domain := normalizeHost(in.CustomDomain)
	if domain != "" {
		if !validDomain(domain) {
			return validationErr("custom_domain is not a valid hostname")
		}
		// Guard against claiming another org's domain (the unique index also enforces).
		if existing, err := s.Store.GetBrandingByDomain(ctx, domain); err == nil && existing.OrgID != orgID {
			return conflictErr("that custom domain is already claimed by another organization")
		}
	}
	err := s.Store.UpsertBranding(ctx, repository.Branding{
		OrgID: orgID, DisplayName: in.DisplayName, LogoURL: in.LogoURL,
		PrimaryColor: in.PrimaryColor, EmailFrom: in.EmailFrom, CustomDomain: domain,
	})
	if err != nil {
		return mapRepoErr(err)
	}
	s.audit(ctx, "branding.update", "organization", orgID, map[string]any{"custom_domain": domain})
	return nil
}

// DomainAllowed reports whether a hostname is a registered custom domain. This is
// the Caddy on_demand_tls allow-list check (mandatory — without it any host could
// trigger unbounded certificate issuance).
func (s *Services) DomainAllowed(ctx context.Context, domain string) bool {
	domain = normalizeHost(domain)
	if domain == "" {
		return false
	}
	ok, err := s.Store.DomainRegistered(ctx, domain)
	return err == nil && ok
}

func normalizeHost(h string) string {
	h = strings.TrimSpace(strings.ToLower(h))
	if i := strings.IndexByte(h, ':'); i >= 0 { // strip :port
		h = h[:i]
	}
	return h
}

// validDomain does a light hostname sanity check (labels, dots, length).
func validDomain(h string) bool {
	if len(h) < 3 || len(h) > 253 || !strings.Contains(h, ".") {
		return false
	}
	for _, label := range strings.Split(h, ".") {
		if label == "" || len(label) > 63 {
			return false
		}
		for _, r := range label {
			if !(r == '-' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z')) {
				return false
			}
		}
	}
	return true
}
