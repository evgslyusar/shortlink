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

// DayStat holds an aggregated click count for a single date.
type DayStat struct {
	Date  string
	Count int64
}

// CountryStat holds an aggregated click count for a single country.
type CountryStat struct {
	Country string
	Count   int64
}
