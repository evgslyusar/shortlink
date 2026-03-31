package domain

import "time"

// TelegramAccount represents a binding between a User and a Telegram user.
type TelegramAccount struct {
	ID         string
	UserID     string
	TelegramID int64
	Username   string
	LinkedAt   time.Time
}
