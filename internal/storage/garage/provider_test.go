package garage

import (
	"testing"

	"github.com/buktio/buktio/internal/storage"
)

func TestProviderRegistered(t *testing.T) {
	p, err := storage.New(storage.ProviderConfig{
		Kind:     Kind,
		AdminURL: "http://garage:3903",
		S3Region: "garage",
	})
	if err != nil {
		t.Fatalf("storage.New(%q): %v", Kind, err)
	}
	if p.Name() != Kind {
		t.Fatalf("Name() = %q, want %q", p.Name(), Kind)
	}
}

func TestCapabilitiesReflectGarage(t *testing.T) {
	p := &Provider{}
	caps := p.Capabilities()
	if caps.ObjectVersioning {
		t.Error("Garage has no object versioning; ObjectVersioning should be false")
	}
	if caps.PerObjectPublicACL {
		t.Error("Garage has no per-object ACL; PerObjectPublicACL should be false")
	}
	if !caps.BucketCORS {
		t.Error("Garage supports bucket CORS; BucketCORS should be true")
	}
	if !caps.LifecycleExpiry {
		t.Error("Garage supports expiry lifecycle; LifecycleExpiry should be true")
	}
}
