package auth

import "testing"

func TestHashVerifyRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "correct horse battery staple" {
		t.Fatal("hash must not equal the password")
	}
	ok, err := VerifyPassword("correct horse battery staple", hash)
	if err != nil || !ok {
		t.Fatalf("correct password should verify: ok=%v err=%v", ok, err)
	}
	bad, err := VerifyPassword("wrong password", hash)
	if err != nil {
		t.Fatal(err)
	}
	if bad {
		t.Fatal("wrong password must not verify")
	}
}

func TestHashesAreSalted(t *testing.T) {
	h1, _ := HashPassword("same")
	h2, _ := HashPassword("same")
	if h1 == h2 {
		t.Fatal("identical passwords must produce different hashes (random salt)")
	}
}

func TestVerifyRejectsGarbage(t *testing.T) {
	if _, err := VerifyPassword("x", "not-a-phc-string"); err == nil {
		t.Fatal("expected error for malformed hash")
	}
}

func TestTokenAndHash(t *testing.T) {
	tok, err := NewToken()
	if err != nil || tok == "" {
		t.Fatalf("token: %v", err)
	}
	if len(HashToken(tok)) != 32 {
		t.Fatal("token hash should be 32 bytes (sha256)")
	}
	if string(HashToken("a")) == string(HashToken("b")) {
		t.Fatal("different tokens must hash differently")
	}
}
