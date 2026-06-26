package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDeliverWebhook_SignsAndDelivers(t *testing.T) {
	type received struct {
		sig, event, contentType string
		body                    []byte
	}
	ch := make(chan received, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		ch <- received{
			sig:         r.Header.Get("X-Buktio-Signature"),
			event:       r.Header.Get("X-Buktio-Event"),
			contentType: r.Header.Get("Content-Type"),
			body:        b,
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := &Services{Logger: slog.Default()}
	body := []byte(`{"event":"object.created","key":"a.txt"}`)
	// deliverWebhook is synchronous (returns after the request succeeds).
	s.deliverWebhook(srv.URL, "topsecret", EventObjectCreated, body)

	select {
	case got := <-ch:
		mac := hmac.New(sha256.New, []byte("topsecret"))
		mac.Write(body)
		want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		if got.sig != want {
			t.Fatalf("signature = %q, want %q", got.sig, want)
		}
		if got.event != EventObjectCreated {
			t.Fatalf("event header = %q, want %q", got.event, EventObjectCreated)
		}
		if got.contentType != "application/json" {
			t.Fatalf("content-type = %q", got.contentType)
		}
		if string(got.body) != string(body) {
			t.Fatalf("body = %q, want %q", got.body, body)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("webhook was not delivered")
	}
}

func TestDeliverWebhook_NoSecretNoSignature(t *testing.T) {
	ch := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ch <- r.Header.Get("X-Buktio-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := &Services{Logger: slog.Default()}
	s.deliverWebhook(srv.URL, "", EventObjectDeleted, []byte(`{}`))
	select {
	case sig := <-ch:
		if sig != "" {
			t.Fatalf("expected no signature without a secret, got %q", sig)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("webhook was not delivered")
	}
}

func TestEventMatches(t *testing.T) {
	cases := []struct {
		csv, event string
		want       bool
	}{
		{"object.created,object.deleted", "object.created", true},
		{"object.deleted", "object.created", false},
		{"", "object.created", false},
		{"object.created", "object.created", true},
	}
	for _, c := range cases {
		if got := eventMatches(c.csv, c.event); got != c.want {
			t.Errorf("eventMatches(%q,%q) = %v, want %v", c.csv, c.event, got, c.want)
		}
	}
}
