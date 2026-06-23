package secret

import (
	"encoding/base64"
	"encoding/hex"
	"testing"
)

func TestNewRPCSecretIsHex32(t *testing.T) {
	s, err := NewRPCSecret()
	if err != nil {
		t.Fatal(err)
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("rpc secret not hex: %v", err)
	}
	if len(b) != 32 {
		t.Fatalf("rpc secret = %d bytes, want 32", len(b))
	}
}

func TestNewTokenIsBase64_32(t *testing.T) {
	s, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		t.Fatalf("token not base64: %v", err)
	}
	if len(b) != 32 {
		t.Fatalf("token = %d bytes, want 32", len(b))
	}
}

func TestNewPasswordRejectsShort(t *testing.T) {
	if _, err := NewPassword(8); err == nil {
		t.Fatal("expected error for short password length")
	}
	if _, err := NewPassword(24); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSecretsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		s, err := NewToken()
		if err != nil {
			t.Fatal(err)
		}
		if seen[s] {
			t.Fatal("duplicate token generated")
		}
		seen[s] = true
	}
}
