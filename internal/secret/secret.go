// Package secret generates and protects buktio's secrets.
//
// All secrets are generated with crypto/rand (no openssl/CLI). Infrastructure
// secrets buktio must replay (Garage rpc_secret / admin_token / metrics_token,
// the buktio-system S3 secret) are stored ENCRYPTED at rest via envelope
// encryption (see envelope.go); user-facing S3 secrets are shown once and never
// stored. The rendered garage.toml never contains secrets — they are injected
// into Garage via *_FILE env vars (see files.go + internal/garagemanager).
package secret

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// RandomBytes returns n cryptographically-random bytes.
func RandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("secret: read random: %w", err)
	}
	return b, nil
}

// NewRPCSecret returns Garage's rpc_secret as 32 random bytes, hex-encoded
// (matches `openssl rand -hex 32`).
func NewRPCSecret() (string, error) {
	b, err := RandomBytes(32)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// NewToken returns a 32-byte random token, standard-base64 encoded (matches
// `openssl rand -base64 32`). Used for Garage admin_token / metrics_token and for
// buktio's session/signing secrets.
func NewToken() (string, error) {
	b, err := RandomBytes(32)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// NewPassword returns a URL-safe random password of n random bytes (e.g. the
// generated PostgreSQL password). n must be >= 16.
func NewPassword(n int) (string, error) {
	if n < 16 {
		return "", fmt.Errorf("secret: password length %d too short (min 16)", n)
	}
	b, err := RandomBytes(n)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// NewKEK returns a 32-byte key-encryption key (AES-256). The KEK is never stored
// in PostgreSQL; it comes from a secret manager, env, or a 0600 file (see kek.go).
func NewKEK() ([]byte, error) {
	return RandomBytes(KEKSize)
}
