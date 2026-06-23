package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Branding is an org's white-label settings.
type Branding struct {
	OrgID        string
	DisplayName  string
	LogoURL      string
	PrimaryColor string
	EmailFrom    string
	CustomDomain string
}

// GetBranding returns an org's branding, or ErrNotFound when unset.
func (s *Store) GetBranding(ctx context.Context, orgID string) (*Branding, error) {
	return scanBranding(s.q(ctx).QueryRow(ctx,
		`SELECT org_id::text, COALESCE(display_name,''), COALESCE(logo_url,''),
		        COALESCE(primary_color,''), COALESCE(email_from,''), COALESCE(custom_domain,'')
		   FROM org_branding WHERE org_id=$1::uuid`, orgID))
}

// GetBrandingByDomain resolves branding by a custom domain (case-insensitive).
func (s *Store) GetBrandingByDomain(ctx context.Context, domain string) (*Branding, error) {
	return scanBranding(s.q(ctx).QueryRow(ctx,
		`SELECT org_id::text, COALESCE(display_name,''), COALESCE(logo_url,''),
		        COALESCE(primary_color,''), COALESCE(email_from,''), COALESCE(custom_domain,'')
		   FROM org_branding WHERE lower(custom_domain)=lower($1)`, domain))
}

func scanBranding(row pgx.Row) (*Branding, error) {
	var b Branding
	err := row.Scan(&b.OrgID, &b.DisplayName, &b.LogoURL, &b.PrimaryColor, &b.EmailFrom, &b.CustomDomain)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("repository: get branding: %w", err)
	}
	return &b, nil
}

// UpsertBranding sets an org's branding (full replace).
func (s *Store) UpsertBranding(ctx context.Context, b Branding) error {
	_, err := s.q(ctx).Exec(ctx,
		`INSERT INTO org_branding (org_id, display_name, logo_url, primary_color, email_from, custom_domain, updated_at)
		 VALUES ($1::uuid, NULLIF($2,''), NULLIF($3,''), NULLIF($4,''), NULLIF($5,''), NULLIF($6,''), now())
		 ON CONFLICT (org_id) DO UPDATE SET
		   display_name=EXCLUDED.display_name, logo_url=EXCLUDED.logo_url,
		   primary_color=EXCLUDED.primary_color, email_from=EXCLUDED.email_from,
		   custom_domain=EXCLUDED.custom_domain, updated_at=now()`,
		b.OrgID, b.DisplayName, b.LogoURL, b.PrimaryColor, b.EmailFrom, b.CustomDomain)
	if err != nil {
		return fmt.Errorf("repository: upsert branding: %w", err)
	}
	return nil
}

// DomainRegistered reports whether a custom domain is claimed by some org (the
// Caddy on_demand_tls allow-list check).
func (s *Store) DomainRegistered(ctx context.Context, domain string) (bool, error) {
	var n int
	err := s.q(ctx).QueryRow(ctx,
		`SELECT count(*) FROM org_branding WHERE lower(custom_domain)=lower($1)`, domain).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("repository: domain registered: %w", err)
	}
	return n > 0, nil
}
