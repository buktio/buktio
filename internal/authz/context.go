package authz

import "context"

type ctxKey int

const subjectKey ctxKey = iota

// WithSubject attaches the authenticated subject to the context (set by the auth
// middleware).
func WithSubject(ctx context.Context, s Subject) context.Context {
	return context.WithValue(ctx, subjectKey, s)
}

// SubjectFrom returns the authenticated subject, if any.
func SubjectFrom(ctx context.Context) (Subject, bool) {
	s, ok := ctx.Value(subjectKey).(Subject)
	return s, ok
}
