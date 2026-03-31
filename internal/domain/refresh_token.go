package domain

import "time"

// RefreshToken represents an opaque refresh token stored as a SHA-256 hash.
type RefreshToken struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
}
