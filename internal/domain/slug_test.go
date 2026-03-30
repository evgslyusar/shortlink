package domain_test

import (
	"testing"

	"github.com/evgslyusar/shortlink/internal/domain"
)

func TestGenerateSlug(t *testing.T) {
	t.Run("produces 6-character base62 strings", func(t *testing.T) {
		const base62 = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
		allowed := make(map[byte]bool, len(base62))
		for i := range base62 {
			allowed[base62[i]] = true
		}

		for range 100 {
			slug, err := domain.GenerateSlug()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(slug) != 6 {
				t.Errorf("expected length 6, got %d: %q", len(slug), slug)
			}
			for _, c := range []byte(slug) {
				if !allowed[c] {
					t.Errorf("invalid character %q in slug %q", c, slug)
				}
			}
		}
	})

	t.Run("produces unique slugs across 1000 runs", func(t *testing.T) {
		seen := make(map[string]bool, 1000)
		for range 1000 {
			slug, err := domain.GenerateSlug()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if seen[slug] {
				t.Errorf("duplicate slug: %q", slug)
			}
			seen[slug] = true
		}
	})
}

func TestValidateCustomSlug(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		// Valid cases.
		{"min length 4", "abcd", false},
		{"max length 12", "abcdefghijkl", false},
		{"with digits", "abc123", false},
		{"with hyphens", "my-slug", false},
		{"with underscores", "my_slug", false},
		{"mixed chars", "A1-b_2C", false},

		// Invalid: too short.
		{"empty", "", true},
		{"1 char", "a", true},
		{"3 chars", "abc", true},

		// Invalid: too long.
		{"13 chars", "abcdefghijklm", true},
		{"20 chars", "abcdefghijklmnopqrst", true},

		// Invalid: bad characters.
		{"space", "ab cd", true},
		{"dot", "ab.cd", true},
		{"slash", "ab/cd", true},
		{"at sign", "ab@cd", true},
		{"unicode", "abcд", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.ValidateCustomSlug(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCustomSlug(%q): got err=%v, wantErr=%v", tt.input, err, tt.wantErr)
			}
		})
	}
}
