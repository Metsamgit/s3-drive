// Package validation: valide les inputs qui finissent en appel AWS ou en
// clé S3. Tout est centralisé pour pouvoir relire ça rapidement.
package validation

import (
	"errors"
	"regexp"
	"strings"
)

// bucketNameRe: sous-ensemble des règles S3. On exclut les points (les
// bucket "x.y.z" cassent la vérification TLS du host virtual-hosted).
var bucketNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9\-]{1,61}[a-z0-9]$`)

// keyRe: ASCII imprimable seulement. Suffisant pour des noms de fichiers.
var keyRe = regexp.MustCompile(`^[a-zA-Z0-9._/\- ()\[\]+@#,!=]+$`)

// regionRe: format canonique des régions AWS. Doublé d'une whitelist.
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

// BucketName valide un nom de bucket.
func BucketName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if !bucketNameRe.MatchString(name) {
		return "", ErrBucketInvalid
	}
	return name, nil
}

// S3Key valide une clé S3. Rejette les `..`, les slashs en tête ou
// doublés, tout ce qui sort de l'ASCII imprimable. Max 1024 caractères.
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

// Region valide une région AWS (forme + whitelist).
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

// AWSAccessKey vérifie la forme d'un Access Key ID. Pas de check sur
// "AKIA" car les credentials STS commencent par "ASIA".
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

// AWSSecretKey vérifie la forme d'un Secret Access Key (base64-ish).
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

// Prefix valide la partie "dossier" d'une clé. Vide = racine.
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
