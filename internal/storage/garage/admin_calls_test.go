package garage

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateKeyParsesSecretAndSetsAuth(t *testing.T) {
	var gotAuth, gotPath string
	var gotBody CreateKeyRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(CreateKeyResponse{
			AccessKeyID:     "GKtest123",
			SecretAccessKey: "supersecret",
			Name:            gotBody.Name,
		})
	}))
	defer srv.Close()

	c := NewAdminClient(srv.URL, "tok-abc")
	key, err := c.CreateKey(context.Background(), "buktio-system", true)
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v2/CreateKey" {
		t.Errorf("path = %q, want /v2/CreateKey", gotPath)
	}
	if gotAuth != "Bearer tok-abc" {
		t.Errorf("auth = %q, want Bearer tok-abc", gotAuth)
	}
	if gotBody.Name != "buktio-system" || gotBody.Allow == nil || !gotBody.Allow.CreateBucket {
		t.Errorf("unexpected request body: %+v", gotBody)
	}
	if key.AccessKeyID != "GKtest123" || key.SecretAccessKey != "supersecret" {
		t.Errorf("unexpected key: %+v", key)
	}
}

func TestApplyClusterLayoutSendsVersion(t *testing.T) {
	var gotBody ApplyLayoutRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewAdminClient(srv.URL, "t")
	if err := c.ApplyClusterLayout(context.Background(), 3); err != nil {
		t.Fatal(err)
	}
	if gotBody.Version != 3 {
		t.Errorf("applied version = %d, want 3", gotBody.Version)
	}
}

func TestGetClusterStatusParsesNode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(ClusterStatusResponse{
			LayoutVersion: 1,
			Nodes:         []NodeResp{{ID: "node-xyz", GarageVersion: "v2.3.0", IsUp: true}},
		})
	}))
	defer srv.Close()

	c := NewAdminClient(srv.URL, "t")
	st, err := c.GetClusterStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	n := st.PrimaryNode()
	if n == nil || n.ID != "node-xyz" || n.GarageVersion != "v2.3.0" {
		t.Errorf("unexpected primary node: %+v", n)
	}
}

func TestNon2xxBecomesAdminError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"bad token"}`))
	}))
	defer srv.Close()

	c := NewAdminClient(srv.URL, "wrong")
	_, err := c.GetClusterStatus(context.Background())
	if err == nil {
		t.Fatal("expected error on 401")
	}
	var ae *AdminError
	if !errors.As(err, &ae) || ae.Status != http.StatusUnauthorized {
		t.Errorf("expected AdminError with 401, got %v", err)
	}
}
