package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
)

// KEKSize is the key-encryption-key length (AES-256).
const KEKSize = 32

const (
	dekSize     = 32 // AES-256 data-encryption key
	nonceSize   = 12 // AES-GCM standard nonce
	gcmOverhead = 16 // AES-GCM tag
	envVersion  = 1  // blob format version
)

// wrappedDEKSize is the size of a DEK encrypted under the KEK (DEK + GCM tag).
const wrappedDEKSize = dekSize + gcmOverhead

// Sealer encrypts/decrypts secret material at rest.
type Sealer interface {
	// Seal encrypts plaintext, returning an opaque, self-describing blob safe to
	// store in PostgreSQL.
	Seal(plaintext []byte) ([]byte, error)
	// Open decrypts a blob produced by Seal. It fails if the blob was tampered
	// with or the wrong KEK is used.
	Open(blob []byte) ([]byte, error)
}

// EnvelopeSealer implements envelope encryption: each Seal generates a fresh
// random data key (DEK), encrypts the plaintext with the DEK, and wraps the DEK
// with the KEK. Both layers are AES-256-GCM (authenticated).
//
// Blob layout:
//
//	[version:1][kekNonce:12][wrappedDEK:48][dataNonce:12][ciphertext:N]
type EnvelopeSealer struct {
	kekGCM cipher.AEAD
}

// NewEnvelopeSealer builds a Sealer from a 32-byte KEK.
func NewEnvelopeSealer(kek []byte) (*EnvelopeSealer, error) {
	if len(kek) != KEKSize {
		return nil, fmt.Errorf("secret: KEK must be %d bytes, got %d", KEKSize, len(kek))
	}
	block, err := aes.NewCipher(kek)
	if err != nil {
		return nil, fmt.Errorf("secret: new KEK cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secret: new KEK gcm: %w", err)
	}
	return &EnvelopeSealer{kekGCM: gcm}, nil
}

// Seal implements Sealer.
func (s *EnvelopeSealer) Seal(plaintext []byte) ([]byte, error) {
	// Fresh per-record data key.
	dek := make([]byte, dekSize)
	if _, err := rand.Read(dek); err != nil {
		return nil, fmt.Errorf("secret: gen DEK: %w", err)
	}
	dataGCM, err := newGCM(dek)
	if err != nil {
		return nil, err
	}

	dataNonce := make([]byte, nonceSize)
	if _, err := rand.Read(dataNonce); err != nil {
		return nil, fmt.Errorf("secret: gen data nonce: %w", err)
	}
	ciphertext := dataGCM.Seal(nil, dataNonce, plaintext, nil)

	// Wrap the DEK under the KEK.
	kekNonce := make([]byte, nonceSize)
	if _, err := rand.Read(kekNonce); err != nil {
		return nil, fmt.Errorf("secret: gen KEK nonce: %w", err)
	}
	wrappedDEK := s.kekGCM.Seal(nil, kekNonce, dek, nil)

	blob := make([]byte, 0, 1+nonceSize+wrappedDEKSize+nonceSize+len(ciphertext))
	blob = append(blob, byte(envVersion))
	blob = append(blob, kekNonce...)
	blob = append(blob, wrappedDEK...)
	blob = append(blob, dataNonce...)
	blob = append(blob, ciphertext...)
	return blob, nil
}

// Open implements Sealer.
func (s *EnvelopeSealer) Open(blob []byte) ([]byte, error) {
	const headerLen = 1 + nonceSize + wrappedDEKSize + nonceSize
	if len(blob) < headerLen {
		return nil, errors.New("secret: blob too short")
	}
	if blob[0] != byte(envVersion) {
		return nil, fmt.Errorf("secret: unsupported blob version %d", blob[0])
	}
	off := 1
	kekNonce := blob[off : off+nonceSize]
	off += nonceSize
	wrappedDEK := blob[off : off+wrappedDEKSize]
	off += wrappedDEKSize
	dataNonce := blob[off : off+nonceSize]
	off += nonceSize
	ciphertext := blob[off:]

	dek, err := s.kekGCM.Open(nil, kekNonce, wrappedDEK, nil)
	if err != nil {
		return nil, fmt.Errorf("secret: unwrap DEK (wrong KEK or tampered): %w", err)
	}
	dataGCM, err := newGCM(dek)
	if err != nil {
		return nil, err
	}
	plaintext, err := dataGCM.Open(nil, dataNonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("secret: decrypt (tampered): %w", err)
	}
	return plaintext, nil
}

// SealString is a convenience wrapper over Seal for string secrets.
func (s *EnvelopeSealer) SealString(plaintext string) ([]byte, error) {
	return s.Seal([]byte(plaintext))
}

// OpenString is a convenience wrapper over Open returning a string.
func (s *EnvelopeSealer) OpenString(blob []byte) (string, error) {
	b, err := s.Open(blob)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("secret: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secret: new gcm: %w", err)
	}
	return gcm, nil
}
