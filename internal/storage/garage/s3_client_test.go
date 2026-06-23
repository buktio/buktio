package garage

import (
	"testing"

	"github.com/buktio/buktio/internal/storage"
)

func TestProviderBuildsWithS3Config(t *testing.T) {
	p, err := storage.New(storage.ProviderConfig{
		Kind:       Kind,
		AdminURL:   "http://garage:3903",
		S3Endpoint: "http://garage:3900",
		S3Region:   "garage",
		SystemKey:  storage.Credential{AccessKeyID: "GK", SecretAccessKey: "s"},
		Extra:      map[string]string{"s3_public_endpoint": "https://s3.example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Name() != Kind {
		t.Errorf("name = %q", p.Name())
	}
}
