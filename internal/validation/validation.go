// Package validation centralizes input sanitization for any value that ends
// up in an AWS API call or a filesystem path. Keeping it in one file makes
// it easy to audit.
package validation

import (
	"errors"
	"regexp"
	"strings"
)

// bucketNameRe enforces a conservative subset of S3 bucket naming rules.
// AWS allows dots but we forbid them: dot-buckets disable TLS hostname
// verification on the virtual-hosted-style endpoint, and we have no use
// for them.
var bucketNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9\-]{1,61}[a-z0-9]$`)

// keyRe restricts object keys to a printable ASCII subset. S3 accepts a
// wider range but every other character (newlines, control bytes, NUL) has
// only ever caused trouble — none of our users will name files with them.
var keyRe = regexp.MustCompile(`^[a-zA-Z0-9._/\- ()\[\]+@#,!=]+$`)

// regionRe matches the canonical region format. We additionally check
// against a whitelist below.
var regionRe = regexp.MustCompile(`^[a-z]{2,4}-[a-z]+-[1-9][0-9]?$`)

var knownRegions = map[string]struct{}{
	"us-east-1": {}, "us-east-2": {}, "us-west-1": {}, "us-west-2": {},
	"eu-west-1": {}, "eu-west-2": {}, "eu-west-3": {}, "eu-central-1": {}, "eu-north-1": {}, "eu-south-1": {},
	"ap-northeast-1": {}, "ap-northeast-2": {}, "ap-northeast-3": {},
	"ap-southeast-1": {}, "ap-southeast-2": {}, "ap-south-1": {},
	"sa-east-1": {}, "ca-central-1": {}, "me-south-1": {}, "af-south-1": {},
}

var (
	ErrBucketInvalid = errors.New("nom de bucket invalide")
	ErrKeyInvalid    = errors.New("nom de fichier ou de chemin invalide")
	ErrRegionInvalid = errors.New("région AWS inconnue")
)

// BucketName returns the cleaned bucket name or an error.
func BucketName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if !bucketNameRe.MatchString(name) {
		return "", ErrBucketInvalid
	}
	return name, nil
}

// S3Key validates and cleans an object key.
//
// We reject any path traversal (`..`), leading slash, double slash, and
// anything outside the printable ASCII charset. Length is capped at 1024
// (S3's documented hard limit).
func S3Key(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" || len(key) > 1024 {
		return "", ErrKeyInvalid
	}
	if strings.HasPrefix(key, "/") || strings.Contains(key, "//") {
		return "", ErrKeyInvalid
	}
	for _, part := range strings.Split(key, "/") {
		if part == ".." || part == "." {
			return "", ErrKeyInvalid
		}
	}
	if !keyRe.MatchString(key) {
		return "", ErrKeyInvalid
	}
	return key, nil
}

// Region returns the cleaned region or an error if it doesn't match either
// the canonical pattern or our whitelist.
func Region(region string) (string, error) {
	region = strings.TrimSpace(region)
	if !regionRe.MatchString(region) {
		return "", ErrRegionInvalid
	}
	if _, ok := knownRegions[region]; !ok {
		return "", ErrRegionInvalid
	}
	return region, nil
}

// AWSAccessKey checks the shape of an Access Key ID. We don't require it
// to start with "AKIA" because STS short-term credentials use "ASIA" etc.
func AWSAccessKey(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 16 || len(s) > 128 {
		return false
	}
	for _, r := range s {
		if !((r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

// AWSSecretKey checks the shape of a Secret Access Key — base64-ish, 40 chars.
// We avoid leaking even the length in error messages elsewhere.
func AWSSecretKey(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 16 || len(s) > 256 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '/' || r == '+' || r == '=':
		default:
			return false
		}
	}
	return true
}

// Prefix validates the "folder" portion of a path used in list operations.
// Empty prefix is allowed (means root).
func Prefix(p string) (string, error) {
	if p == "" {
		return "", nil
	}
	if _, err := S3Key(p); err != nil {
		return "", err
	}
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return p, nil
}
