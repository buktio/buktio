package garagemanager

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/buktio/buktio/internal/storage/garage"
)

// fakeGarage is a minimal stateful Admin API v2 fake for bootstrap tests.
type fakeGarage struct {
	mu            sync.Mutex
	nodeID        string
	version       string
	layoutVersion int
	hasRole       bool
	appliedVers   []int
	createdKeys   int
}

func (f *fakeGarage) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
		case "/v2/GetClusterStatus":
			_ = json.NewEncoder(w).Encode(garage.ClusterStatusResponse{
				LayoutVersion: f.layoutVersion,
				Nodes:         []garage.NodeResp{{ID: f.nodeID, GarageVersion: f.version, IsUp: true}},
			})
		case "/v2/GetClusterLayout":
			resp := garage.ClusterLayoutResponse{Version: f.layoutVersion}
			if f.hasRole {
				capacity := int64(defaultCapacityBytes)
				resp.Roles = []garage.LayoutRole{{ID: f.nodeID, Zone: DefaultZone, Capacity: &capacity}}
			}
			_ = json.NewEncoder(w).Encode(resp)
		case "/v2/UpdateClusterLayout":
			// Staged; applied on ApplyClusterLayout.
			w.WriteHeader(http.StatusOK)
		case "/v2/ApplyClusterLayout":
			var req garage.ApplyLayoutRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			f.appliedVers = append(f.appliedVers, req.Version)
			f.layoutVersion = req.Version
			f.hasRole = true
			w.WriteHeader(http.StatusOK)
		case "/v2/CreateKey":
			f.createdKeys++
			_ = json.NewEncoder(w).Encode(garage.CreateKeyResponse{
				AccessKeyID:     "GKsystem",
				SecretAccessKey: "system-secret",
				Name:            "buktio-system",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func fastParams() BootstrapParams {
	return BootstrapParams{HealthTimeout: 2 * time.Second, PollInterval: 10 * time.Millisecond}
}

func TestBootstrapFreshNode(t *testing.T) {
	fg := &fakeGarage{nodeID: "node-1", version: "v2.3.0", layoutVersion: 0, hasRole: false}
	srv := httptest.NewServer(fg.handler())
	defer srv.Close()

	admin := garage.NewAdminClient(srv.URL, "tok")
	res, err := Bootstrap(context.Background(), admin, fastParams())
	if err != nil {
		t.Fatal(err)
	}
	if !res.LayoutApplied {
		t.Error("fresh node should have layout applied")
	}
	if len(fg.appliedVers) != 1 || fg.appliedVers[0] != 1 {
		t.Errorf("expected apply of version 1, got %v", fg.appliedVers)
	}
	if !res.SystemKeyCreated || res.SystemSecretAccessKey != "system-secret" {
		t.Errorf("expected a system key with captured secret, got %+v", res)
	}
	if res.NodeID != "node-1" {
		t.Errorf("node id = %q", res.NodeID)
	}
}

func TestBootstrapIdempotent(t *testing.T) {
	// Node already has a role and buktio already owns a system key.
	fg := &fakeGarage{nodeID: "node-1", version: "v2.3.0", layoutVersion: 1, hasRole: true}
	srv := httptest.NewServer(fg.handler())
	defer srv.Close()

	p := fastParams()
	p.ExistingSystemKeyID = "GKexisting"

	admin := garage.NewAdminClient(srv.URL, "tok")
	res, err := Bootstrap(context.Background(), admin, p)
	if err != nil {
		t.Fatal(err)
	}
	if res.LayoutApplied {
		t.Error("idempotent re-run must NOT re-apply layout")
	}
	if len(fg.appliedVers) != 0 {
		t.Errorf("expected no apply calls, got %v", fg.appliedVers)
	}
	if fg.createdKeys != 0 {
		t.Errorf("expected no key creation, got %d", fg.createdKeys)
	}
	if res.SystemAccessKeyID != "GKexisting" {
		t.Errorf("system key id = %q, want GKexisting", res.SystemAccessKeyID)
	}
}

func TestBootstrapRejectsOldVersion(t *testing.T) {
	fg := &fakeGarage{nodeID: "node-1", version: "v1.0.4", layoutVersion: 0}
	srv := httptest.NewServer(fg.handler())
	defer srv.Close()

	admin := garage.NewAdminClient(srv.URL, "tok")
	if _, err := Bootstrap(context.Background(), admin, fastParams()); err == nil {
		t.Fatal("expected version guard to reject Garage 1.x")
	}
}
