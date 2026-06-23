package garage

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateBucketSendsGlobalAlias(t *testing.T) {
	var gotReq CreateBucketRequest
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotReq)
		_ = json.NewEncoder(w).Encode(BucketInfoResponse{ID: "bkt-1", GlobalAliases: []string{gotReq.GlobalAlias}})
	}))
	defer srv.Close()

	c := NewAdminClient(srv.URL, "t")
	info, err := c.CreateBucket(context.Background(), "logs-prod")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v2/CreateBucket" || gotReq.GlobalAlias != "logs-prod" {
		t.Errorf("unexpected request: path=%q alias=%q", gotPath, gotReq.GlobalAlias)
	}
	if info.ID != "bkt-1" {
		t.Errorf("info.ID = %q", info.ID)
	}
}

func TestGetBucketInfoUsesIDQuery(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("id")
		maxSize := int64(1 << 30)
		_ = json.NewEncoder(w).Encode(BucketInfoResponse{
			ID: "bkt-1", Objects: 42, Bytes: 1000,
			Quotas: BucketQuotas{MaxSize: &maxSize},
		})
	}))
	defer srv.Close()

	c := NewAdminClient(srv.URL, "t")
	info, err := c.GetBucketInfo(context.Background(), "bkt-1")
	if err != nil {
		t.Fatal(err)
	}
	if gotQuery != "bkt-1" {
		t.Errorf("id query = %q, want bkt-1", gotQuery)
	}
	if info.Objects != 42 || info.Bytes != 1000 || info.Quotas.MaxSize == nil || *info.Quotas.MaxSize != 1<<30 {
		t.Errorf("unexpected info: %+v", info)
	}
}

func TestUpdateBucketQuotaBody(t *testing.T) {
	var gotReq UpdateBucketRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotReq)
		_ = json.NewEncoder(w).Encode(BucketInfoResponse{ID: "bkt-1"})
	}))
	defer srv.Close()

	c := NewAdminClient(srv.URL, "t")
	max := int64(500)
	if _, err := c.UpdateBucket(context.Background(), "bkt-1", UpdateBucketRequest{
		Quotas: &BucketQuotas{MaxObjects: &max},
	}); err != nil {
		t.Fatal(err)
	}
	if gotReq.Quotas == nil || gotReq.Quotas.MaxObjects == nil || *gotReq.Quotas.MaxObjects != 500 {
		t.Errorf("unexpected update body: %+v", gotReq)
	}
	if gotReq.WebsiteAccess != nil {
		t.Error("websiteAccess should be omitted when only quotas are set")
	}
}
