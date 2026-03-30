package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/evgslyusar/shortlink/internal/domain"
)

// RefreshTokenPostgres implements refresh token persistence against PostgreSQL.
type RefreshTokenPostgres struct {
	db *pgxpool.Pool
}

// NewRefreshTokenPostgres creates a new RefreshTokenPostgres repository.
func NewRefreshTokenPostgres(db *pgxpool.Pool) *RefreshTokenPostgres {
	return &RefreshTokenPostgres{db: db}
}

// Create inserts a new refresh token record.
func (r *RefreshTokenPostgres) Create(ctx context.Context, token *domain.RefreshToken) error {
	const query = `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)`

	_, err := r.db.Exec(ctx, query,
		token.ID, token.UserID, token.TokenHash, token.ExpiresAt, token.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("refresh_token_postgres.Create: %w", err)
	}
	return nil
}

// FindByHash retrieves a refresh token by its SHA-256 hash.
// Returns domain.ErrNotFound if no token exists with the given hash.
func (r *RefreshTokenPostgres) FindByHash(ctx context.Context, tokenHash string) (*domain.RefreshToken, error) {
	const query = `
		SELECT id, user_id, token_hash, expires_at, created_at
		FROM refresh_tokens
		WHERE token_hash = $1`

	var t domain.RefreshToken
	err := r.db.QueryRow(ctx, query, tokenHash).Scan(
		&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("refresh_token_postgres.FindByHash: %w", domain.ErrNotFound)
		}
		return nil, fmt.Errorf("refresh_token_postgres.FindByHash: %w", err)
	}
	return &t, nil
}

// DeleteByHash removes a refresh token by its SHA-256 hash.
func (r *RefreshTokenPostgres) DeleteByHash(ctx context.Context, tokenHash string) error {
	const query = `DELETE FROM refresh_tokens WHERE token_hash = $1`

	_, err := r.db.Exec(ctx, query, tokenHash)
	if err != nil {
		return fmt.Errorf("refresh_token_postgres.DeleteByHash: %w", err)
	}
	return nil
}
