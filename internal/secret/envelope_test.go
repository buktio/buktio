package secret

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func newTestSealer(t *testing.T) *EnvelopeSealer {
	t.Helper()
	kek, err := NewKEK()
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewEnvelopeSealer(kek)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestEnvelopeRoundTrip(t *testing.T) {
	s := newTestSealer(t)
	plaintext := []byte("garage admin_token: super-secret-value")

	blob, err := s.Seal(plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(blob, plaintext) {
		t.Fatal("ciphertext leaks plaintext")
	}

	got, err := s.Open(blob)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("round trip mismatch: got %q", got)
	}
}

func TestEnvelopeDistinctCiphertexts(t *testing.T) {
	s := newTestSealer(t)
	a, _ := s.Seal([]byte("same"))
	b, _ := s.Seal([]byte("same"))
	if bytes.Equal(a, b) {
		t.Fatal("identical plaintext produced identical blobs (nonce/DEK reuse)")
	}
}

func TestEnvelopeTamperDetected(t *testing.T) {
	s := newTestSealer(t)
	blob, _ := s.Seal([]byte("immutable"))
	blob[len(blob)-1] ^= 0xff // flip a ciphertext byte
	if _, err := s.Open(blob); err == nil {
		t.Fatal("expected tamper to be detected")
	}
}

func TestEnvelopeWrongKEKFails(t *testing.T) {
	a := newTestSealer(t)
	b := newTestSealer(t) // different KEK
	blob, _ := a.Seal([]byte("secret"))
	if _, err := b.Open(blob); err == nil {
		t.Fatal("expected Open with wrong KEK to fail")
	}
}

func TestNewEnvelopeSealerRejectsBadKEK(t *testing.T) {
	if _, err := NewEnvelopeSealer([]byte("too-short")); err == nil {
		t.Fatal("expected error for short KEK")
	}
}

func TestCheckPerms(t *testing.T) {
	dir := t.TempDir()
	secure := filepath.Join(dir, "secure.key")
	if err := WriteFile(secure, []byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := CheckPerms(secure); err != nil {
		t.Fatalf("0600 file should pass: %v", err)
	}

	insecure := filepath.Join(dir, "insecure.key")
	if err := os.WriteFile(insecure, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CheckPerms(insecure); err == nil {
		t.Fatal("0644 file should be rejected")
	}
}
