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

// userCreator persists a new user.
type userCreator interface {
	CreateUser(ctx context.Context, user *domain.User) error
}

// userByEmailFinder looks up a user by email address.
type userByEmailFinder interface {
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
}

// compile-time interface checks.
var (
	_ userCreator      = (*UserPostgres)(nil)
	_ userByEmailFinder = (*UserPostgres)(nil)
)

// UserPostgres implements user persistence against PostgreSQL.
type UserPostgres struct {
	db *pgxpool.Pool
}

// NewUserPostgres creates a new UserPostgres repository.
func NewUserPostgres(db *pgxpool.Pool) *UserPostgres {
	return &UserPostgres{db: db}
}

// CreateUser inserts a new user into the database.
// Returns domain.ErrAlreadyExists if the email is already taken.
func (r *UserPostgres) CreateUser(ctx context.Context, user *domain.User) error {
	const query = `
		INSERT INTO users (id, email, password, created_at)
		VALUES ($1, $2, $3, $4)`

	_, err := r.db.Exec(ctx, query, user.ID, user.Email, user.Password, user.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("user_postgres.CreateUser: %w", domain.ErrAlreadyExists)
		}
		return fmt.Errorf("user_postgres.CreateUser: %w", err)
	}

	return nil
}

// FindByEmail retrieves a user by email address.
// Returns domain.ErrNotFound if no user exists with the given email.
func (r *UserPostgres) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	const query = `
		SELECT id, email, password, created_at
		FROM users
		WHERE email = $1`

	var u domain.User
	err := r.db.QueryRow(ctx, query, email).Scan(&u.ID, &u.Email, &u.Password, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user_postgres.FindByEmail: %w", domain.ErrNotFound)
		}
		return nil, fmt.Errorf("user_postgres.FindByEmail: %w", err)
	}

	return &u, nil
}
