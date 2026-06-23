package garage

import (
	"testing"
	"time"

	"github.com/buktio/buktio/internal/storage"
)

func TestBucketFromInfoPrivate(t *testing.T) {
	b := bucketFromInfo(&BucketInfoResponse{ID: "x", WebsiteAccess: false})
	if b.Visibility != storage.VisibilityPrivate {
		t.Errorf("visibility = %q, want private", b.Visibility)
	}
	if b.Website.Enabled {
		t.Error("website should be disabled")
	}
}

func TestBucketFromInfoPublicWebsite(t *testing.T) {
	b := bucketFromInfo(&BucketInfoResponse{
		ID:            "x",
		WebsiteAccess: true,
		WebsiteConfig: &WebsiteAccess{Enabled: true, IndexDocument: "index.html", ErrorDocument: "404.html"},
	})
	if b.Visibility != storage.VisibilityPublic {
		t.Errorf("visibility = %q, want public-website", b.Visibility)
	}
	if !b.Website.Enabled || b.Website.IndexDocument != "index.html" || b.Website.ErrorDocument != "404.html" {
		t.Errorf("unexpected website config: %+v", b.Website)
	}
}

func TestUsageFromInfo(t *testing.T) {
	now := time.Unix(1000, 0).UTC()
	max := int64(99)
	u := usageFromInfo(&BucketInfoResponse{
		Objects: 7, Bytes: 2048, UnfinishedMultipartUploads: 2,
		Quotas: BucketQuotas{MaxObjects: &max},
	}, now)
	if u.ObjectCount != 7 || u.BytesUsed != 2048 || u.UnfinishedMultipartUploads != 2 {
		t.Errorf("unexpected usage: %+v", u)
	}
	if u.QuotaMaxObjects == nil || *u.QuotaMaxObjects != 99 {
		t.Errorf("quota not mapped: %+v", u.QuotaMaxObjects)
	}
	if !u.CapturedAt.Equal(now) {
		t.Errorf("capturedAt = %v, want %v", u.CapturedAt, now)
	}
}

func TestKeyPermsFromStorage(t *testing.T) {
	got := keyPermsFromStorage(storage.Permissions{Read: true, Write: true, Owner: false})
	if !got.Read || !got.Write || got.Owner {
		t.Errorf("unexpected perms: %+v", got)
	}
}
