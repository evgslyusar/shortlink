package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/evgslyusar/shortlink/internal/domain"
)

// TelegramAccountPostgres implements telegram account persistence against PostgreSQL.
type TelegramAccountPostgres struct {
	db *pgxpool.Pool
}

// NewTelegramAccountPostgres creates a new TelegramAccountPostgres repository.
func NewTelegramAccountPostgres(db *pgxpool.Pool) *TelegramAccountPostgres {
	return &TelegramAccountPostgres{db: db}
}

// LinkTelegram creates a new telegram_accounts row binding a user to a Telegram ID.
// Returns domain.ErrAlreadyExists if the user or telegram_id is already linked.
func (r *TelegramAccountPostgres) LinkTelegram(ctx context.Context, account *domain.TelegramAccount) error {
	const query = `
		INSERT INTO telegram_accounts (id, user_id, telegram_id, username, linked_at)
		VALUES ($1, $2, $3, $4, $5)`

	_, err := r.db.Exec(ctx, query,
		account.ID, account.UserID, account.TelegramID, account.Username, account.LinkedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return fmt.Errorf("telegram_account_postgres.LinkTelegram: %w", domain.ErrAlreadyExists)
		}
		return fmt.Errorf("telegram_account_postgres.LinkTelegram: %w", err)
	}
	return nil
}

// FindByTelegramID retrieves a telegram account by Telegram user ID.
// Returns domain.ErrNotFound if no account exists for the given Telegram ID.
func (r *TelegramAccountPostgres) FindByTelegramID(ctx context.Context, telegramID int64) (*domain.TelegramAccount, error) {
	const query = `
		SELECT id, user_id, telegram_id, username, linked_at
		FROM telegram_accounts
		WHERE telegram_id = $1`

	var a domain.TelegramAccount
	err := r.db.QueryRow(ctx, query, telegramID).Scan(
		&a.ID, &a.UserID, &a.TelegramID, &a.Username, &a.LinkedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("telegram_account_postgres.FindByTelegramID: %w", domain.ErrNotFound)
		}
		return nil, fmt.Errorf("telegram_account_postgres.FindByTelegramID: %w", err)
	}
	return &a, nil
}
