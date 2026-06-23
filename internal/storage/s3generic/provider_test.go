package s3generic

import (
	"context"
	"errors"
	"testing"

	"github.com/buktio/buktio/internal/storage"
)

func buildGeneric(t *testing.T, kind string) storage.StorageProvider {
	t.Helper()
	p, err := storage.New(storage.ProviderConfig{
		Kind:       kind,
		S3Endpoint: "https://s3.example.com",
		S3Region:   "us-east-1",
		SystemKey:  storage.Credential{AccessKeyID: "AK", SecretAccessKey: "sk"},
	})
	if err != nil {
		t.Fatalf("build %s: %v", kind, err)
	}
	return p
}

func TestGenericKindsRegistered(t *testing.T) {
	for _, k := range []string{KindAWSS3, KindR2, KindB2, KindSeaweedFS, KindCephRGW} {
		p := buildGeneric(t, k)
		if p.Name() != k {
			t.Errorf("Name() = %q, want %q", p.Name(), k)
		}
	}
}

func TestGenericCapabilitiesGateControlPlane(t *testing.T) {
	// All generic backends: no key/quota/health/website management.
	for _, k := range []string{KindAWSS3, KindR2, KindB2, KindSeaweedFS, KindCephRGW} {
		c := buildGeneric(t, k).Capabilities()
		if c.ManagesKeys || c.ManagesQuota || c.HasClusterHealth || c.PublicWebsite {
			t.Errorf("%s: expected control-plane caps all false, got %+v", k, c)
		}
		if !c.BucketCORS || !c.LifecycleExpiry {
			t.Errorf("%s: expected CORS+lifecycle true, got %+v", k, c)
		}
	}
	// AWS S3 additionally advertises versioning + per-object ACL.
	aws := buildGeneric(t, KindAWSS3).Capabilities()
	if !aws.ObjectVersioning || !aws.PerObjectPublicACL {
		t.Errorf("aws_s3 should advertise versioning + per-object ACL, got %+v", aws)
	}
}

func TestGenericUnsupportedReturnsErrUnsupported(t *testing.T) {
	p := buildGeneric(t, KindR2)
	ctx := context.Background()
	checks := map[string]error{
		"SetVisibility": p.SetVisibility(ctx, "b", storage.VisibilityPublic, storage.WebsiteConfig{}),
		"GrantKey":      p.GrantKey(ctx, "b", "k", storage.Permissions{Read: true}),
		"RevokeKey":     p.RevokeKey(ctx, "b", "k"),
		"SetQuota":      p.SetQuota(ctx, "b", storage.Quota{}),
		"ImportKey":     p.ImportAccessKey(ctx, storage.Credential{}),
		"DeleteKey":     p.DeleteAccessKey(ctx, "k"),
	}
	for name, err := range checks {
		if !errors.Is(err, storage.ErrUnsupported) {
			t.Errorf("%s: got %v, want ErrUnsupported", name, err)
		}
	}
	if _, err := p.CreateAccessKey(ctx, "n", false); !errors.Is(err, storage.ErrUnsupported) {
		t.Errorf("CreateAccessKey: got %v, want ErrUnsupported", err)
	}
	if _, err := p.ListAccessKeys(ctx); !errors.Is(err, storage.ErrUnsupported) {
		t.Errorf("ListAccessKeys: got %v, want ErrUnsupported", err)
	}
	if _, err := p.GetClusterHealth(ctx); !errors.Is(err, storage.ErrUnsupported) {
		t.Errorf("GetClusterHealth: got %v, want ErrUnsupported", err)
	}
	if _, err := p.GetClusterStatus(ctx); !errors.Is(err, storage.ErrUnsupported) {
		t.Errorf("GetClusterStatus: got %v, want ErrUnsupported", err)
	}
}
