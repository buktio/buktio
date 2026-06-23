// Package notify is the outbound-notification seam (e.g. invite emails). The OSS
// build uses a log-only mailer (no SMTP dependency); the invite link is also
// returned to the inviter in the API response. Paid editions can inject an SMTP
// mailer with no call-site changes.
package notify

import (
	"context"
	"log/slog"
)

// Mailer sends transactional messages.
type Mailer interface {
	SendInvite(ctx context.Context, email, link string) error
	// SendVerification sends a self-serve-signup email-verification link.
	SendVerification(ctx context.Context, email, link string) error
}

// LogMailer logs instead of sending — the OSS default.
type LogMailer struct{ Logger *slog.Logger }

func (m LogMailer) SendInvite(_ context.Context, email, _ string) error {
	if m.Logger != nil {
		// Never log the link/token (it grants account access); the inviter receives
		// it via the authenticated API response.
		m.Logger.Info("invite created (log-only mailer; share the link from the UI)",
			slog.String("email", email))
	}
	return nil
}

func (m LogMailer) SendVerification(_ context.Context, email, _ string) error {
	if m.Logger != nil {
		// Never log the verification link/token; a real Mailer emails it.
		m.Logger.Info("email verification created (log-only mailer)", slog.String("email", email))
	}
	return nil
}
