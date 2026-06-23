package garagemanager

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/buktio/buktio/internal/storage/garage"
)

// fakeCluster routes the layout admin calls and records what was staged/applied.
type fakeCluster struct {
	version       int
	stagedRoles   []garage.LayoutRoleChange
	appliedAt     int
	connectedPeer []string
}

func (f *fakeCluster) server() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/ConnectClusterNodes":
			var peers []string
			_ = json.NewDecoder(r.Body).Decode(&peers)
			f.connectedPeer = append(f.connectedPeer, peers...)
			_ = json.NewEncoder(w).Encode([]garage.ConnectNodeResult{{Success: true}})
		case "/v2/UpdateClusterLayout":
			var body garage.UpdateLayoutChangesRequest
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.stagedRoles = body.Roles
			w.WriteHeader(http.StatusOK)
		case "/v2/GetClusterLayout":
			_ = json.NewEncoder(w).Encode(garage.ClusterLayoutResponse{Version: f.version})
		case "/v2/ApplyClusterLayout":
			var body garage.ApplyLayoutRequest
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.appliedAt = body.Version
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestAddOrUpdateNodeConnectsStagesAppliesNextVersion(t *testing.T) {
	f := &fakeCluster{version: 5}
	srv := f.server()
	defer srv.Close()

	admin := garage.NewAdminClient(srv.URL, "t")
	applied, err := AddOrUpdateNode(context.Background(), admin, "n2@10.0.0.2:3901",
		NodeSpec{NodeID: "n2", Zone: "dc1", CapacityBytes: 1 << 30})
	if err != nil {
		t.Fatal(err)
	}
	if applied != 6 {
		t.Errorf("applied version = %d, want 6 (current+1)", applied)
	}
	if f.appliedAt != 6 {
		t.Errorf("server applied at %d, want 6", f.appliedAt)
	}
	if len(f.connectedPeer) != 1 || f.connectedPeer[0] != "n2@10.0.0.2:3901" {
		t.Errorf("connected peers = %v", f.connectedPeer)
	}
	if len(f.stagedRoles) != 1 || f.stagedRoles[0].ID != "n2" || f.stagedRoles[0].Capacity == nil {
		t.Errorf("staged roles = %+v", f.stagedRoles)
	}
}

func TestRemoveNodeStagesRemoval(t *testing.T) {
	f := &fakeCluster{version: 9}
	srv := f.server()
	defer srv.Close()

	admin := garage.NewAdminClient(srv.URL, "t")
	applied, err := RemoveNode(context.Background(), admin, "n3")
	if err != nil {
		t.Fatal(err)
	}
	if applied != 10 {
		t.Errorf("applied = %d, want 10", applied)
	}
	if len(f.stagedRoles) != 1 || !f.stagedRoles[0].Remove || f.stagedRoles[0].ID != "n3" {
		t.Errorf("staged removal wrong: %+v", f.stagedRoles)
	}
}

func TestConnectNodeRejectsBadPeer(t *testing.T) {
	admin := garage.NewAdminClient("http://unused", "t")
	if err := ConnectNode(context.Background(), admin, "no-at-sign"); err == nil {
		t.Error("expected error for peer without @")
	}
}
