//go:build integration

package garage_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/buktio/buktio/internal/storage"
	"github.com/buktio/buktio/internal/storage/garage"
)

func garageImage() string {
	if v := os.Getenv("GARAGE_IMAGE"); v != "" {
		return v
	}
	return "dxflrs/garage:v2.3.0"
}

const garageConfig = `
metadata_dir = "/tmp/meta"
data_dir     = "/tmp/data"
db_engine    = "sqlite"
replication_factor = 1
rpc_bind_addr = "[::]:3901"
rpc_public_addr = "127.0.0.1:3901"

[s3_api]
api_bind_addr = "0.0.0.0:3900"
s3_region     = "garage"

[admin]
api_bind_addr         = "0.0.0.0:3903"
metrics_require_token = false
`

type liveGarage struct {
	container  testcontainers.Container
	adminURL   string
	s3URL      string
	adminToken string
}

func startGarage(t *testing.T) *liveGarage {
	t.Helper()
	ctx := context.Background()

	rpcBytes := make([]byte, 32)
	_, _ = rand.Read(rpcBytes)
	rpcSecret := hex.EncodeToString(rpcBytes)
	adminToken := "integration-admin-token"

	req := testcontainers.ContainerRequest{
		Image:        garageImage(),
		ExposedPorts: []string{"3900/tcp", "3903/tcp"},
		Env: map[string]string{
			"GARAGE_RPC_SECRET":  rpcSecret,
			"GARAGE_ADMIN_TOKEN": adminToken,
		},
		Files: []testcontainers.ContainerFile{{
			Reader:            strings.NewReader(garageConfig),
			ContainerFilePath: "/etc/garage.toml",
			FileMode:          0o644,
		}},
		Cmd:        []string{"/garage", "-c", "/etc/garage.toml", "server", "--single-node"},
		WaitingFor: wait.ForHTTP("/health").WithPort("3903/tcp").WithStatusCodeMatcher(func(s int) bool { return s == 200 }).WithStartupTimeout(90 * time.Second),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: req, Started: true})
	if err != nil {
		t.Fatalf("start garage: %v", err)
	}

	host, _ := c.Host(ctx)
	adminPort, _ := c.MappedPort(ctx, "3903/tcp")
	s3Port, _ := c.MappedPort(ctx, "3900/tcp")
	lg := &liveGarage{
		container:  c,
		adminURL:   fmt.Sprintf("http://%s:%s", host, adminPort.Port()),
		s3URL:      fmt.Sprintf("http://%s:%s", host, s3Port.Port()),
		adminToken: adminToken,
	}
	t.Cleanup(func() { _ = c.Terminate(ctx) })
	return lg
}

// TestIntegrationProviderConformance exercises the v1.1 Garage adapter end-to-end
// against a real pinned Garage container.
func TestIntegrationProviderConformance(t *testing.T) {
	lg := startGarage(t)
	ctx := context.Background()

	admin := garage.NewAdminClient(lg.adminURL, lg.adminToken)

	// System key buktio uses for S3 management ops.
	sysKey, err := admin.CreateKey(ctx, "buktio-system", true)
	if err != nil {
		t.Fatalf("create system key: %v", err)
	}

	provider, err := storage.New(storage.ProviderConfig{
		Kind:       garage.Kind,
		S3Endpoint: lg.s3URL,
		S3Region:   "garage",
		AdminURL:   lg.adminURL,
		AdminToken: lg.adminToken,
		SystemKey:  storage.Credential{AccessKeyID: sysKey.AccessKeyID, SecretAccessKey: sysKey.SecretAccessKey},
		Extra:      map[string]string{"s3_public_endpoint": lg.s3URL},
	})
	if err != nil {
		t.Fatalf("storage.New: %v", err)
	}

	const alias = "conf-bucket"
	b, err := provider.CreateBucket(ctx, alias)
	if err != nil {
		t.Fatalf("create bucket: %v", err)
	}
	// Grant the system key on the bucket (needed for S3 ops).
	if err := provider.GrantKey(ctx, b.ID, sysKey.AccessKeyID, storage.Permissions{Read: true, Write: true, Owner: true}); err != nil {
		t.Fatalf("grant system key: %v", err)
	}

	// Put / Get byte-equality.
	payload := bytes.Repeat([]byte("buktio-"), 1024) // ~7 KiB
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

	// CORS round-trip.
	if err := provider.SetCORS(ctx, alias, []storage.CORSRule{{AllowedOrigins: []string{"https://x.test"}, AllowedMethods: []string{"GET", "PUT"}, MaxAgeSeconds: 60}}); err != nil {
		t.Fatalf("set cors: %v", err)
	}
	cors, err := provider.GetCORS(ctx, alias)
	if err != nil || len(cors) != 1 || cors[0].MaxAgeSeconds != 60 {
		t.Fatalf("get cors: %v %+v", err, cors)
	}

	// Lifecycle round-trip.
	if err := provider.SetLifecycle(ctx, alias, []storage.LifecycleRule{{Prefix: "tmp/", Enabled: true, ExpireDays: 5}}); err != nil {
		t.Fatalf("set lifecycle: %v", err)
	}
	lc, err := provider.GetLifecycle(ctx, alias)
	if err != nil || len(lc) != 1 || lc[0].ExpireDays != 5 {
		t.Fatalf("get lifecycle: %v %+v", err, lc)
	}

	// Usage from a single GetBucketInfo.
	usage, err := provider.GetBucketUsage(ctx, b.ID)
	if err != nil || usage.ObjectCount < 1 {
		t.Fatalf("usage: %v %+v", err, usage)
	}

	// Cluster health.
	if h, err := provider.GetClusterHealth(ctx); err != nil || h.Status == "" {
		t.Fatalf("cluster health: %v %+v", err, h)
	}

	// Empty + delete.
	if err := provider.DeleteObjects(ctx, alias, []string{"a/b/c.txt"}); err != nil {
		t.Fatalf("delete objects: %v", err)
	}
	if err := provider.DeleteBucket(ctx, b.ID); err != nil {
		t.Fatalf("delete bucket: %v", err)
	}
}
