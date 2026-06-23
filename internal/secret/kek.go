package secret

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

// Provider sources the key-encryption key (KEK). The default reads env/file; a
// secret-manager-backed provider (Vault/AWS SM/GCP SM) can be added later behind
// this interface without touching call sites.
type Provider interface {
	KEK() ([]byte, error)
}

// EnvFileProvider resolves the KEK in precedence order:
//  1. BUKTIO_MASTER_KEY      (base64-encoded 32 bytes)
//  2. BUKTIO_MASTER_KEY_FILE (path to a 0600 file containing base64 or raw bytes)
type EnvFileProvider struct {
	EnvVar  string // default "BUKTIO_MASTER_KEY"
	FileVar string // default "BUKTIO_MASTER_KEY_FILE"
}

// DefaultProvider returns the standard env/file KEK provider.
func DefaultProvider() EnvFileProvider {
	return EnvFileProvider{EnvVar: "BUKTIO_MASTER_KEY", FileVar: "BUKTIO_MASTER_KEY_FILE"}
}

// KEK implements Provider.
func (p EnvFileProvider) KEK() ([]byte, error) {
	envVar := p.EnvVar
	if envVar == "" {
		envVar = "BUKTIO_MASTER_KEY"
	}
	fileVar := p.FileVar
	if fileVar == "" {
		fileVar = "BUKTIO_MASTER_KEY_FILE"
	}

	if v := os.Getenv(envVar); v != "" {
		return decodeKEK([]byte(v))
	}
	if path := os.Getenv(fileVar); path != "" {
		if err := CheckPerms(path); err != nil {
			return nil, err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("secret: read KEK file: %w", err)
		}
		return decodeKEK(data)
	}
	return nil, fmt.Errorf("secret: no KEK configured (set %s or %s)", envVar, fileVar)
}

// decodeKEK accepts base64 (standard or raw) or exactly KEKSize raw bytes.
func decodeKEK(v []byte) ([]byte, error) {
	s := strings.TrimSpace(string(v))
	if raw, err := base64.StdEncoding.DecodeString(s); err == nil && len(raw) == KEKSize {
		return raw, nil
	}
	if raw, err := base64.RawStdEncoding.DecodeString(s); err == nil && len(raw) == KEKSize {
		return raw, nil
	}
	if len(v) == KEKSize {
		return append([]byte(nil), v...), nil
	}
	return nil, fmt.Errorf("secret: KEK must be %d bytes (base64 or raw)", KEKSize)
}
