//go:build integration

package s3generic_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/buktio/buktio/internal/storage"
	"github.com/buktio/buktio/internal/storage/s3generic"
)

func minioImage() string {
	if v := os.Getenv("MINIO_IMAGE"); v != "" {
		return v
	}
	return "minio/minio:latest"
}

type liveMinio struct {
	s3URL  string
	access string
	secret string
}

func startMinio(t *testing.T) *liveMinio {
	t.Helper()
	ctx := context.Background()
	const access, secret = "minioadmin", "minioadmin123"

	req := testcontainers.ContainerRequest{
		Image:        minioImage(),
		ExposedPorts: []string{"9000/tcp"},
		Env: map[string]string{
			"MINIO_ROOT_USER":     access,
			"MINIO_ROOT_PASSWORD": secret,
		},
		Cmd:        []string{"server", "/data"},
		WaitingFor: wait.ForHTTP("/minio/health/live").WithPort("9000/tcp").WithStartupTimeout(60 * time.Second),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: req, Started: true})
	if err != nil {
		t.Fatalf("start minio: %v", err)
	}
	host, _ := c.Host(ctx)
	port, _ := c.MappedPort(ctx, "9000/tcp")
	t.Cleanup(func() { _ = c.Terminate(ctx) })
	return &liveMinio{
		s3URL:  fmt.Sprintf("http://%s:%s", host, port.Port()),
		access: access, secret: secret,
	}
}

// TestIntegrationGenericS3Conformance exercises the generic-S3 adapter end-to-end
// against a real MinIO container, reusing the same object-plane conformance as the
// Garage suite and asserting that control-plane operations report ErrUnsupported.
func TestIntegrationGenericS3Conformance(t *testing.T) {
	m := startMinio(t)
	ctx := context.Background()

	provider, err := storage.New(storage.ProviderConfig{
		Kind:       s3generic.KindAWSS3, // MinIO speaks the AWS S3 API
		S3Endpoint: m.s3URL,
		S3Region:   "us-east-1",
		SystemKey:  storage.Credential{AccessKeyID: m.access, SecretAccessKey: m.secret},
		Extra:      map[string]string{"s3_public_endpoint": m.s3URL},
	})
	if err != nil {
		t.Fatalf("storage.New: %v", err)
	}

	// Ping (ListBuckets) works with operator credentials.
	if err := provider.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}

	const alias = "conf-bucket"
	b, err := provider.CreateBucket(ctx, alias)
	if err != nil {
		t.Fatalf("create bucket: %v", err)
	}
	if b.ID != alias {
		t.Fatalf("generic bucket id should equal name, got %q", b.ID)
	}

	// Put / Get byte-equality.
	payload := bytes.Repeat([]byte("buktio-generic-"), 1024) // ~15 KiB
	if err := provider.PutObject(ctx, alias, "a/b/c.txt", bytes.NewReader(payload), int64(len(payload)), "text/plain"); err != nil {
		t.Fatalf("put: %v", err)
	}
	rc, _, err := provider.GetObject(ctx, alias, "a/b/c.txt")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	got, _ := io.ReadAll(rc)
	rc.Close()
	if !bytes.Equal(got, payload) {
		t.Fatalf("get mismatch: %d vs %d bytes", len(got), len(payload))
	}

	// Folder navigation via delimiter.
	res, err := provider.ListObjects(ctx, alias, storage.ListObjectsInput{Delimiter: "/"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(res.CommonPrefixes) != 1 || res.CommonPrefixes[0] != "a/" {
		t.Fatalf("expected common prefix a/, got %v", res.CommonPrefixes)
	}

	// Lifecycle round-trip (MinIO supports PutBucketLifecycleConfiguration).
	if err := provider.SetLifecycle(ctx, alias, []storage.LifecycleRule{{Prefix: "tmp/", Enabled: true, ExpireDays: 5}}); err != nil {
		t.Fatalf("set lifecycle: %v", err)
	}
	lc, err := provider.GetLifecycle(ctx, alias)
	if err != nil || len(lc) != 1 || lc[0].ExpireDays != 5 {
		t.Fatalf("get lifecycle: %v %+v", err, lc)
	}

	// Usage via full-scan (no GetBucketInfo on generic backends).
	usage, err := provider.GetBucketUsage(ctx, alias)
	if err != nil || usage.ObjectCount < 1 || usage.BytesUsed < int64(len(payload)) {
		t.Fatalf("usage: %v %+v", err, usage)
	}

	// Control-plane operations must be cleanly unsupported.
	if err := provider.SetVisibility(ctx, alias, storage.VisibilityPublic, storage.WebsiteConfig{}); !errors.Is(err, storage.ErrUnsupported) {
		t.Errorf("SetVisibility: want ErrUnsupported, got %v", err)
	}
	if _, err := provider.CreateAccessKey(ctx, "x", false); !errors.Is(err, storage.ErrUnsupported) {
		t.Errorf("CreateAccessKey: want ErrUnsupported, got %v", err)
	}
	if _, err := provider.GetClusterHealth(ctx); !errors.Is(err, storage.ErrUnsupported) {
		t.Errorf("GetClusterHealth: want ErrUnsupported, got %v", err)
	}
	if err := provider.SetQuota(ctx, alias, storage.Quota{}); !errors.Is(err, storage.ErrUnsupported) {
		t.Errorf("SetQuota: want ErrUnsupported, got %v", err)
	}

	// Empty + delete.
	if err := provider.DeleteObjects(ctx, alias, []string{"a/b/c.txt"}); err != nil {
		t.Fatalf("delete objects: %v", err)
	}
	if err := provider.DeleteBucket(ctx, alias); err != nil {
		t.Fatalf("delete bucket: %v", err)
	}
}
