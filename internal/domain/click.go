package domain

import "time"

// Click represents a single redirect event for a link.
type Click struct {
	ID        string
	LinkID    string
	ClickedAt time.Time
	Country   *string
	Referer   *string
	UserAgent *string
}
