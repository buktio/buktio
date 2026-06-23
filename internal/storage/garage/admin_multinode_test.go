package garage

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConnectClusterNodesSendsPeerArray(t *testing.T) {
	var gotPath string
	var gotPeers []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotPeers)
		_ = json.NewEncoder(w).Encode([]ConnectNodeResult{{Success: true}})
	}))
	defer srv.Close()

	c := NewAdminClient(srv.URL, "t")
	res, err := c.ConnectClusterNodes(context.Background(), []string{"node2@10.0.0.2:3901"})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v2/ConnectClusterNodes" {
		t.Errorf("path = %q", gotPath)
	}
	if len(gotPeers) != 1 || gotPeers[0] != "node2@10.0.0.2:3901" {
		t.Errorf("peers = %v", gotPeers)
	}
	if len(res) != 1 || !res[0].Success {
		t.Errorf("result = %+v", res)
	}
}

func TestUpdateClusterLayoutChangesStagesAssignAndRemove(t *testing.T) {
	var gotBody UpdateLayoutChangesRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cap := int64(1 << 30)
	c := NewAdminClient(srv.URL, "t")
	err := c.UpdateClusterLayoutChanges(context.Background(), []LayoutRoleChange{
		{ID: "n1", Zone: "dc1", Capacity: &cap},
		{ID: "n2", Remove: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(gotBody.Roles) != 2 {
		t.Fatalf("roles = %d, want 2", len(gotBody.Roles))
	}
	if gotBody.Roles[0].ID != "n1" || gotBody.Roles[0].Capacity == nil || *gotBody.Roles[0].Capacity != cap {
		t.Errorf("assign role wrong: %+v", gotBody.Roles[0])
	}
	if gotBody.Roles[1].ID != "n2" || !gotBody.Roles[1].Remove {
		t.Errorf("remove role wrong: %+v", gotBody.Roles[1])
	}
}

func TestRevertClusterLayoutHitsEndpoint(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewAdminClient(srv.URL, "t")
	if err := c.RevertClusterLayout(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v2/RevertClusterLayout" {
		t.Errorf("path = %q", gotPath)
	}
}
