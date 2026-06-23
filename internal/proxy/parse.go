// Package proxy implements buktio-s3proxy: a thin reverse proxy in front of the
// Garage S3 plane that counts per-(access-key, bucket, method) requests + bytes so
// buktio can report per-key traffic/egress (Garage exposes none). It forwards every
// request unchanged — Garage still validates SigV4 — and never inspects payloads.
package proxy

import (
	"net/http"
	"net/url"
	"strings"
)

// Key identifies a traffic counter bucket.
type Key struct {
	AccessKeyID string
	Bucket      string
	Method      string
}

// AccessKeyID extracts the S3 access key id from a request without validating the
// signature: from the SigV4 Authorization header's Credential, or the presigned
// X-Amz-Credential query parameter. Returns "" when neither is present.
func AccessKeyID(r *http.Request) string {
	if cred := credentialFromAuth(r.Header.Get("Authorization")); cred != "" {
		return cred
	}
	if c := r.URL.Query().Get("X-Amz-Credential"); c != "" {
		// "<accessKeyId>/<date>/<region>/s3/aws4_request" (URL-decoded by Query()).
		return beforeSlash(c)
	}
	return ""
}

// credentialFromAuth pulls "<accessKeyId>" out of an Authorization header like:
// "AWS4-HMAC-SHA256 Credential=GK123/20240101/garage/s3/aws4_request, SignedHeaders=..., Signature=...".
func credentialFromAuth(auth string) string {
	if auth == "" {
		return ""
	}
	const marker = "Credential="
	i := strings.Index(auth, marker)
	if i < 0 {
		return ""
	}
	rest := auth[i+len(marker):]
	// up to the next comma or space
	if j := strings.IndexAny(rest, ", "); j >= 0 {
		rest = rest[:j]
	}
	return beforeSlash(rest)
}

func beforeSlash(s string) string {
	if i := strings.IndexByte(s, '/'); i >= 0 {
		return s[:i]
	}
	return s
}

// BucketFromPath returns the bucket name from a path-style S3 URL ("/bucket/key").
// buktio always uses path-style addressing. Returns "" for the service root.
func BucketFromPath(p string) string {
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		return ""
	}
	if i := strings.IndexByte(p, '/'); i >= 0 {
		return p[:i]
	}
	return p
}

// KeyFor builds the traffic Key for a request.
func KeyFor(r *http.Request) Key {
	return Key{
		AccessKeyID: AccessKeyID(r),
		Bucket:      BucketFromPath(r.URL.Path),
		Method:      r.Method,
	}
}

// ParseUpstream validates and parses the upstream base URL.
func ParseUpstream(raw string) (*url.URL, error) {
	return url.Parse(raw)
}
