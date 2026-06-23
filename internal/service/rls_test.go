package service

import (
	"context"
	"testing"
)

// ScopeRequest must be a pure no-op unless RLS is explicitly enabled — this is the
// guarantee that OSS/Pro deployments are byte-for-byte unchanged and that no DB
// connection is touched on the request path when BUKTIO_RLS is off.
func TestScopeRequest_NoopWhenDisabled(t *testing.T) {
	s := &Services{RLSEnabled: false, OrgID: "org-123"}
	ctx := context.Background()
	got, release, err := s.ScopeRequest(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != ctx {
		t.Fatalf("expected the original context back when RLS is off")
	}
	release() // must be safe to call
}

// With RLS on but no resolvable org, ScopeRequest is still a no-op (it never
// reaches the Store), so it cannot panic on a nil pool.
func TestScopeRequest_NoopWithoutOrg(t *testing.T) {
	s := &Services{RLSEnabled: true} // OrgID empty => tenant(ctx).OrgID == ""
	ctx := context.Background()
	got, release, err := s.ScopeRequest(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != ctx {
		t.Fatalf("expected the original context back when no org is resolved")
	}
	release()
}
