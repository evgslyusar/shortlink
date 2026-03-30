package domain

import "time"

// User represents a registered user with email/password authentication.
type User struct {
	ID        string
	Email     string
	Password  string // bcrypt hash
	CreatedAt time.Time
}
