package s3core

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/buktio/buktio/internal/storage"
)

func TestCORSRoundTripMapping(t *testing.T) {
	in := []storage.CORSRule{{
		AllowedOrigins: []string{"https://app.example.com"},
		AllowedMethods: []string{"GET", "PUT"},
		AllowedHeaders: []string{"*"},
		ExposeHeaders:  []string{"ETag"},
		MaxAgeSeconds:  3600,
	}}
	got := CORSRulesFromAWS(CORSRulesToAWS(in))
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	r := got[0]
	if r.MaxAgeSeconds != 3600 || r.AllowedOrigins[0] != "https://app.example.com" ||
		len(r.AllowedMethods) != 2 || r.ExposeHeaders[0] != "ETag" {
		t.Errorf("round-trip mismatch: %+v", r)
	}
}

func TestPresignGeneratesSignedURL(t *testing.T) {
	// Presigning is offline (pure SigV4 computation) — no network needed.
	c := New("http://garage:3900", "https://s3.example.com", "garage", "GKtest", "secretkey")

	url, err := c.PresignURL(context.Background(), storage.PresignInput{
		BucketID: "logs-prod",
		Key:      "uploads/video.mp4",
		Method:   "PUT",
		Expires:  10 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(url, "https://s3.example.com/") {
		t.Errorf("presigned URL should use the public endpoint, got %q", url)
	}
	for _, want := range []string{"logs-prod/uploads/video.mp4", "X-Amz-Signature=", "X-Amz-Credential="} {
		if !strings.Contains(url, want) {
			t.Errorf("presigned URL missing %q\n%s", want, url)
		}
	}
}
