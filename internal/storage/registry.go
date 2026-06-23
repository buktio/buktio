package storage

import "fmt"

// ProviderConfig is the backend-agnostic configuration a Factory consumes.
type ProviderConfig struct {
	// Kind selects the backend: "garage" | "seaweedfs" | "ceph-rgw" |
	// "aws-s3" | "r2" | "b2".
	Kind string

	// S3Endpoint is the S3 API base (Garage: http://garage:3900).
	S3Endpoint string
	// S3Region is the SigV4 region (Garage: "garage").
	S3Region string
	// AdminURL is the admin/control-plane base (Garage: http://garage:3903).
	// Empty for backends without a separate admin plane.
	AdminURL string
	// AdminToken authenticates the admin plane (server-side only).
	AdminToken string

	// SystemKey is the owner-scoped S3 key buktio uses for S3 management ops
	// (object browser, presign, emptying buckets before delete).
	SystemKey Credential

	// Extra carries backend-specific knobs.
	Extra map[string]string
}

// Factory builds a StorageProvider from configuration.
type Factory func(cfg ProviderConfig) (StorageProvider, error)

var registry = map[string]Factory{}

// Register associates a backend kind with its factory. It is called from each
// backend package's init (e.g. internal/storage/garage).
func Register(kind string, f Factory) {
	if f == nil {
		panic("storage: Register called with nil factory for " + kind)
	}
	registry[kind] = f
}

// New resolves and constructs the provider for cfg.Kind.
func New(cfg ProviderConfig) (StorageProvider, error) {
	f, ok := registry[cfg.Kind]
	if !ok {
		return nil, fmt.Errorf("storage: unknown provider %q", cfg.Kind)
	}
	return f(cfg)
}

// Registered returns the set of registered backend kinds (for diagnostics).
func Registered() []string {
	kinds := make([]string, 0, len(registry))
	for k := range registry {
		kinds = append(kinds, k)
	}
	return kinds
}
