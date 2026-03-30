package domain

import "time"

// Link represents a shortened URL with an optional owner and expiration.
type Link struct {
	ID          string
	Slug        string
	OriginalURL string
	UserID      *string
	ExpiresAt   *time.Time
	CreatedAt   time.Time
}

// IsExpired reports whether the link has passed its expiration time.
func (l *Link) IsExpired() bool {
	if l.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*l.ExpiresAt)
}

// IsOwnedBy reports whether the link belongs to the given user.
func (l *Link) IsOwnedBy(userID string) bool {
	return l.UserID != nil && *l.UserID == userID
}
