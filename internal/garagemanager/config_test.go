package garagemanager

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderConfigHasNoSecrets(t *testing.T) {
	out, err := RenderConfigString(DefaultSingleNode())
	if err != nil {
		t.Fatal(err)
	}
	// No secret may be ASSIGNED inline (a comment naming them is fine — they are
	// injected via *_FILE env vars).
	for _, forbidden := range []string{"rpc_secret =", "admin_token =", "metrics_token ="} {
		if strings.Contains(out, forbidden) {
			t.Errorf("rendered garage.toml must not assign %q (secrets are injected via *_FILE)", forbidden)
		}
	}
	// Sanity: key non-sensitive settings are present.
	for _, want := range []string{
		`db_engine = "sqlite"`,
		`replication_factor = 1`,
		`s3_region     = "garage"`,
		`metrics_require_token = true`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered garage.toml missing %q\n---\n%s", want, out)
		}
	}
}

func TestValidateRejectsBadEngine(t *testing.T) {
	p := DefaultSingleNode()
	p.DBEngine = "rocksdb"
	if err := p.Validate(); err == nil {
		t.Fatal("expected error for unsupported db_engine")
	}
}

func TestWriteSecretFiles(t *testing.T) {
	dir := t.TempDir()
	secrets, err := GenerateClusterSecrets()
	if err != nil {
		t.Fatal(err)
	}
	env, err := WriteSecretFiles(dir, secrets)
	if err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{"GARAGE_RPC_SECRET_FILE", "GARAGE_ADMIN_TOKEN_FILE", "GARAGE_METRICS_TOKEN_FILE"} {
		path, ok := env[key]
		if !ok {
			t.Fatalf("missing env entry %s", key)
		}
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if fi.Mode().Perm() != 0o600 {
			t.Errorf("%s mode = %#o, want 0600", path, fi.Mode().Perm())
		}
	}

	// The RPC secret file should hold exactly the generated value.
	got, err := os.ReadFile(filepath.Join(dir, rpcSecretFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != secrets.RPCSecret {
		t.Errorf("rpc_secret file content mismatch")
	}
}
