package domain

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"regexp"
)

const (
	slugLength  = 6
	base62Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

	customSlugMinLen = 4
	customSlugMaxLen = 12
)

var customSlugPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// GenerateSlug produces a random 6-character base62 slug using crypto/rand.
func GenerateSlug() (string, error) {
	max := big.NewInt(int64(len(base62Chars)))
	buf := make([]byte, slugLength)

	for i := range buf {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("generating slug: %w", err)
		}
		buf[i] = base62Chars[n.Int64()]
	}

	return string(buf), nil
}

// ValidateCustomSlug checks that a user-provided slug meets the format rules:
// 4-12 characters, only [a-zA-Z0-9_-].
func ValidateCustomSlug(s string) error {
	if len(s) < customSlugMinLen {
		return &ValidationError{
			Field:   "slug",
			Message: fmt.Sprintf("must be at least %d characters", customSlugMinLen),
		}
	}
	if len(s) > customSlugMaxLen {
		return &ValidationError{
			Field:   "slug",
			Message: fmt.Sprintf("must be at most %d characters", customSlugMaxLen),
		}
	}
	if !customSlugPattern.MatchString(s) {
		return &ValidationError{
			Field:   "slug",
			Message: "must contain only letters, digits, hyphens, and underscores",
		}
	}
	return nil
}
