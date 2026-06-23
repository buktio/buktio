// Package edition resolves the build/license edition.
//
// The OSS build is always Edition "oss": it runs no license check, enables every
// feature, and phones nothing home. Paid builds resolve an offline-verifiable
// license token into a richer edition + entitlements. This package only *names*
// the edition; capability decisions live in package entitlements, and access
// decisions live in package authz — so the rest of the codebase never branches
// on `edition == "pro"`.
package edition

import "strings"

// Edition identifies the product edition.
type Edition string

const (
	OSS        Edition = "oss"
	Pro        Edition = "pro"
	Enterprise Edition = "enterprise"
	Hosted     Edition = "hosted"
)

// Parse normalizes a raw edition string, defaulting to OSS.
func Parse(s string) Edition {
	switch Edition(strings.ToLower(strings.TrimSpace(s))) {
	case Pro:
		return Pro
	case Enterprise:
		return Enterprise
	case Hosted:
		return Hosted
	default:
		return OSS
	}
}

// IsOSS reports whether this is the free, fully-enabled OSS build.
func (e Edition) IsOSS() bool { return e == OSS }

func (e Edition) String() string { return string(e) }
