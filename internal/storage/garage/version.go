package garage

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Minimum / feature-gate Garage versions (see RELEASE-MANIFEST.md).
const (
	minMajor        = 2 // Admin API v2 requires Garage >= 2.0
	minMinor        = 0
	singleNodeMajor = 2 // `server --single-node` requires Garage >= 2.3
	singleNodeMinor = 3
)

// ErrUnsupportedVersion indicates the connected Garage is too old for the v2 client.
var ErrUnsupportedVersion = errors.New("garage: unsupported version (requires >= 2.0)")

// Version is a parsed semantic version (suffixes are tolerated and ignored).
type Version struct {
	Major int
	Minor int
	Patch int
}

func (v Version) String() string { return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch) }

// ParseVersion parses strings like "v2.3.0", "2.3.0", "2.3.0-rc1", "2.3".
func ParseVersion(s string) (Version, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	if s == "" {
		return Version{}, fmt.Errorf("garage: empty version string")
	}
	// Drop any build/pre-release suffix after '-' or '+'.
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	var v Version
	var err error
	if v.Major, err = strconv.Atoi(parts[0]); err != nil {
		return Version{}, fmt.Errorf("garage: bad major in %q: %w", s, err)
	}
	if len(parts) > 1 {
		if v.Minor, err = strconv.Atoi(parts[1]); err != nil {
			return Version{}, fmt.Errorf("garage: bad minor in %q: %w", s, err)
		}
	}
	if len(parts) > 2 {
		if v.Patch, err = strconv.Atoi(parts[2]); err != nil {
			return Version{}, fmt.Errorf("garage: bad patch in %q: %w", s, err)
		}
	}
	return v, nil
}

// atLeast reports whether v >= (major.minor).
func (v Version) atLeast(major, minor int) bool {
	if v.Major != major {
		return v.Major > major
	}
	return v.Minor >= minor
}

// CheckSupported returns ErrUnsupportedVersion if v is below the minimum the v2
// client requires (< 2.0). buktio fails fast on startup when this fires.
func CheckSupported(v Version) error {
	if !v.atLeast(minMajor, minMinor) {
		return fmt.Errorf("%w: connected Garage is %s", ErrUnsupportedVersion, v)
	}
	return nil
}

// SupportsSingleNode reports whether `server --single-node` is available (>= 2.3).
func SupportsSingleNode(v Version) bool {
	return v.atLeast(singleNodeMajor, singleNodeMinor)
}
