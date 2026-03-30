package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/evgslyusar/shortlink/internal/domain"
	"github.com/google/uuid"
)

const bcryptCost = 12

// UserCreator persists a new user.
type UserCreator interface {
	CreateUser(ctx context.Context, user *domain.User) error
}

// UserByEmailFinder looks up a user by email address.
type UserByEmailFinder interface {
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
}

// AuthService handles user registration and login.
type AuthService struct {
	creator UserCreator
	finder  UserByEmailFinder
	logger  *zap.Logger
}

// NewAuthService creates a new AuthService.
func NewAuthService(creator UserCreator, finder UserByEmailFinder, logger *zap.Logger) *AuthService {
	return &AuthService{
		creator: creator,
		finder:  finder,
		logger:  logger,
	}
}

// Register creates a new user with the given email and password.
func (s *AuthService) Register(ctx context.Context, email, password string) (*domain.User, error) {
	if err := validateEmail(email); err != nil {
		return nil, err
	}
	if err := validatePassword(password); err != nil {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("auth.Register: hashing password: %w", err)
	}

	user := &domain.User{
		ID:        uuid.NewString(),
		Email:     email,
		Password:  string(hash),
		CreatedAt: time.Now().UTC(),
	}

	if err := s.creator.CreateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("auth.Register: %w", err)
	}

	s.logger.Info("user registered",
		zap.String("user_id", user.ID),
	)

	return user, nil
}

// Login authenticates a user by email and password.
// Returns domain.ErrUnauthorized for both missing users and wrong passwords
// to prevent user enumeration.
func (s *AuthService) Login(ctx context.Context, email, password string) (*domain.User, error) {
	user, err := s.finder.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrUnauthorized
		}
		return nil, fmt.Errorf("auth.Login: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, domain.ErrUnauthorized
	}

	s.logger.Info("user logged in",
		zap.String("user_id", user.ID),
	)

	return user, nil
}

func validateEmail(email string) error {
	_, err := mail.ParseAddress(email)
	if err != nil {
		return &domain.ValidationError{
			Field:   "email",
			Message: "must be a valid email address",
		}
	}
	return nil
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return &domain.ValidationError{
			Field:   "password",
			Message: "must be at least 8 characters",
		}
	}
	return nil
}
