package webui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// TestHandlerServes verifies the embedded SPA handler works regardless of whether
// a real export or only the committed placeholder is present (unit CI has no web
// build, so dist holds just the placeholder).
func TestHandlerServes(t *testing.T) {
	h, err := Handler()
	if err != nil {
		t.Fatalf("Handler(): %v", err)
	}

	// Root serves an HTML document.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /: status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("GET /: content-type = %q, want text/html", ct)
	}

	// An unknown, non-asset path falls back to the app shell (SPA routing).
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/buckets/anything", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("SPA fallback: status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("SPA fallback: content-type = %q, want text/html", ct)
	}
}

// TestRealExportEmbedded is the release guard: when BUKTIO_REQUIRE_EMBED=1 (set by
// the image build / release verification), a real UI export MUST be embedded —
// not just the placeholder. Skipped in ordinary unit CI where the web isn't built.
func TestRealExportEmbedded(t *testing.T) {
	if os.Getenv("BUKTIO_REQUIRE_EMBED") != "1" {
		t.Skip("set BUKTIO_REQUIRE_EMBED=1 to enforce a real embedded UI (release builds)")
	}
	if !Embedded() {
		t.Fatal("no real UI embedded (only the placeholder): run `make web-embed` before building")
	}

	// A real export serves hashed assets under /_next with an immutable cache.
	h, err := Handler()
	if err != nil {
		t.Fatalf("Handler(): %v", err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(rec.Body.String(), "/_next/") {
		t.Fatal("root document does not reference /_next/ assets — export looks incomplete")
	}
}
