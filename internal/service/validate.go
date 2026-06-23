package service

import (
	"fmt"
	"regexp"
)

// bucketNameRe enforces S3/Garage-safe bucket names (also used as the Garage
// global alias): 3-63 chars, lowercase letters/digits/hyphens, no leading/trailing
// hyphen.
var bucketNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$`)

// ValidateBucketName checks a bucket name against S3 naming rules.
func ValidateBucketName(name string) error {
	if len(name) < 3 || len(name) > 63 {
		return fmt.Errorf("bucket name must be 3-63 characters")
	}
	if !bucketNameRe.MatchString(name) {
		return fmt.Errorf("bucket name may contain only lowercase letters, digits and hyphens, and must start/end with a letter or digit")
	}
	return nil
}

// keyNameRe allows a friendly label for access keys.
var keyNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 _.-]{0,63}$`)

// ValidateKeyName checks an access-key label.
func ValidateKeyName(name string) error {
	if !keyNameRe.MatchString(name) {
		return fmt.Errorf("key name must be 1-64 characters (letters, digits, space, _.-)")
	}
	return nil
}

var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

func validEmail(email string) bool { return emailRe.MatchString(email) }
