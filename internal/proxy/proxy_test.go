package proxy

import (
	"net/http"
	"testing"
)

func TestAccessKeyIDFromAuthHeader(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://s3.local/bucket/key", nil)
	r.Header.Set("Authorization",
		"AWS4-HMAC-SHA256 Credential=GKabc123/20240101/garage/s3/aws4_request, SignedHeaders=host, Signature=deadbeef")
	if got := AccessKeyID(r); got != "GKabc123" {
		t.Errorf("AccessKeyID = %q, want GKabc123", got)
	}
}

func TestAccessKeyIDFromPresignQuery(t *testing.T) {
	r, _ := http.NewRequest("PUT",
		"http://s3.local/bucket/key?X-Amz-Credential=GKpresign%2F20240101%2Fgarage%2Fs3%2Faws4_request&X-Amz-Signature=x", nil)
	if got := AccessKeyID(r); got != "GKpresign" {
		t.Errorf("AccessKeyID = %q, want GKpresign", got)
	}
}

func TestBucketFromPath(t *testing.T) {
	cases := map[string]string{
		"/my-bucket/path/to/obj.txt": "my-bucket",
		"/just-bucket":               "just-bucket",
		"/":                          "",
		"":                           "",
	}
	for in, want := range cases {
		if got := BucketFromPath(in); got != want {
			t.Errorf("BucketFromPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAggregatorAccumulatesAndDrains(t *testing.T) {
	a := NewAggregator()
	k := Key{AccessKeyID: "GK1", Bucket: "b", Method: "PUT"}
	a.Add(k, 100, 0)
	a.Add(k, 50, 0)
	a.Add(Key{AccessKeyID: "GK1", Bucket: "b", Method: "GET"}, 0, 2048)

	samples := a.Drain()
	if len(samples) != 2 {
		t.Fatalf("drained %d keys, want 2", len(samples))
	}
	// Draining must reset.
	if again := a.Drain(); len(again) != 0 {
		t.Errorf("second drain returned %d, want 0", len(again))
	}
	var putReqs, putIn, getOut int64
	for _, s := range samples {
		if s.Method == "PUT" {
			putReqs, putIn = s.Requests, s.BytesIn
		}
		if s.Method == "GET" {
			getOut = s.BytesOut
		}
	}
	if putReqs != 2 || putIn != 150 {
		t.Errorf("PUT: requests=%d bytesIn=%d, want 2/150", putReqs, putIn)
	}
	if getOut != 2048 {
		t.Errorf("GET bytesOut=%d, want 2048", getOut)
	}
}
