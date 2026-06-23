package storage

import "context"

// SSECustomerKey is a client-supplied SSE-C key (base64 of a 32-byte AES-256 key).
// buktio never stores it — it is supplied per request and forgotten.
type SSECustomerKey struct {
	KeyB64 string
}

type ssecCtxKey struct{}

// WithSSEC attaches an SSE-C customer key to the context for the next object op.
func WithSSEC(ctx context.Context, k *SSECustomerKey) context.Context {
	if k == nil || k.KeyB64 == "" {
		return ctx
	}
	return context.WithValue(ctx, ssecCtxKey{}, k)
}

// SSECFrom returns the SSE-C key on the context, if any.
func SSECFrom(ctx context.Context) *SSECustomerKey {
	k, _ := ctx.Value(ssecCtxKey{}).(*SSECustomerKey)
	return k
}
