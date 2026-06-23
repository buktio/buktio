package secret

import (
	"fmt"
	"os"
	"path/filepath"
)

// secretFileMode is the only acceptable mode for a secret file.
const secretFileMode os.FileMode = 0o600

// WriteFile writes secret data to path with 0600 permissions, creating parent
// directories (0700) as needed. It is used to materialize the Garage secret files
// referenced by the *_FILE env vars, so the rendered garage.toml stays non-secret.
func WriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("secret: mkdir %s: %w", dir, err)
	}
	if err := os.WriteFile(path, data, secretFileMode); err != nil {
		return fmt.Errorf("secret: write %s: %w", path, err)
	}
	// WriteFile honors umask; force the exact mode.
	if err := os.Chmod(path, secretFileMode); err != nil {
		return fmt.Errorf("secret: chmod %s: %w", path, err)
	}
	return nil
}

// CheckPerms returns an error if the file at path is readable by group or other.
// buktio fails fast at startup if any secret file is group/world-readable.
func CheckPerms(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("secret: stat %s: %w", path, err)
	}
	if fi.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("secret: %s has insecure permissions %#o (must be group/world-unreadable, e.g. 0600)",
			path, fi.Mode().Perm())
	}
	return nil
}
