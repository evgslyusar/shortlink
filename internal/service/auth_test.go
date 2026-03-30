package service_test

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/evgslyusar/shortlink/internal/domain"
	"github.com/evgslyusar/shortlink/internal/service"
)

// --- fakes ---

type fakeUserCreator struct {
	users map[string]*domain.User // keyed by email
	err   error                   // forced error
}

func newFakeUserCreator() *fakeUserCreator {
	return &fakeUserCreator{users: make(map[string]*domain.User)}
}

func (f *fakeUserCreator) CreateUser(_ context.Context, user *domain.User) error {
	if f.err != nil {
		return f.err
	}
	if _, exists := f.users[user.Email]; exists {
		return domain.ErrAlreadyExists
	}
	f.users[user.Email] = user
	return nil
}

type fakeUserByEmailFinder struct {
	users map[string]*domain.User
}

func newFakeUserByEmailFinder() *fakeUserByEmailFinder {
	return &fakeUserByEmailFinder{users: make(map[string]*domain.User)}
}

func (f *fakeUserByEmailFinder) addUser(user *domain.User) {
	f.users[user.Email] = user
}

func (f *fakeUserByEmailFinder) FindByEmail(_ context.Context, email string) (*domain.User, error) {
	user, ok := f.users[email]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return user, nil
}

// --- tests ---

func TestRegister(t *testing.T) {
	tests := []struct {
		name      string
		email     string
		password  string
		setupErr  error
		wantErr   error
		wantField string
	}{
		{
			name:     "success",
			email:    "user@example.com",
			password: "secret1234",
		},
		{
			name:     "duplicate email",
			email:    "dup@example.com",
			password: "secret1234",
			setupErr: domain.ErrAlreadyExists,
			wantErr:  domain.ErrAlreadyExists,
		},
		{
			name:      "invalid email - no at sign",
			email:     "bademail",
			password:  "secret1234",
			wantField: "email",
		},
		{
			name:      "invalid email - too short",
			email:     "a@",
			password:  "secret1234",
			wantField: "email",
		},
		{
			name:      "invalid email - empty",
			email:     "",
			password:  "secret1234",
			wantField: "email",
		},
		{
			name:      "short password",
			email:     "user@example.com",
			password:  "short",
			wantField: "password",
		},
		{
			name:      "empty password",
			email:     "user@example.com",
			password:  "",
			wantField: "password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creator := newFakeUserCreator()
			if tt.setupErr != nil {
				creator.err = tt.setupErr
			}
			finder := newFakeUserByEmailFinder()
			svc := service.NewAuthService(creator, finder, zap.NewNop())

			user, err := svc.Register(context.Background(), tt.email, tt.password)

			if tt.wantField != "" {
				var ve *domain.ValidationError
				if !errors.As(err, &ve) {
					t.Fatalf("expected ValidationError, got %v", err)
				}
				if ve.Field != tt.wantField {
					t.Errorf("expected field %q, got %q", tt.wantField, ve.Field)
				}
				return
			}

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if user.Email != tt.email {
				t.Errorf("expected email %q, got %q", tt.email, user.Email)
			}
			if user.ID == "" {
				t.Error("expected non-empty user ID")
			}
			// Verify password is hashed, not stored in plain text.
			if user.Password == tt.password {
				t.Error("password stored in plain text")
			}
			if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(tt.password)); err != nil {
				t.Error("password hash does not match")
			}
		})
	}
}

func TestLogin(t *testing.T) {
	// Pre-hash a password for the fake user store.
	hash, err := bcrypt.GenerateFromPassword([]byte("secret1234"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	existingUser := &domain.User{
		ID:       "user-123",
		Email:    "user@example.com",
		Password: string(hash),
	}

	tests := []struct {
		name     string
		email    string
		password string
		wantErr  error
	}{
		{
			name:     "success",
			email:    "user@example.com",
			password: "secret1234",
		},
		{
			name:     "wrong password",
			email:    "user@example.com",
			password: "wrongpassword",
			wantErr:  domain.ErrUnauthorized,
		},
		{
			name:     "user not found",
			email:    "nobody@example.com",
			password: "secret1234",
			wantErr:  domain.ErrUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creator := newFakeUserCreator()
			finder := newFakeUserByEmailFinder()
			finder.addUser(existingUser)
			svc := service.NewAuthService(creator, finder, zap.NewNop())

			user, err := svc.Login(context.Background(), tt.email, tt.password)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected %v, got %v", tt.wantErr, err)
				}
				if user != nil {
					t.Error("expected nil user on error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if user.ID != existingUser.ID {
				t.Errorf("expected user ID %q, got %q", existingUser.ID, user.ID)
			}
		})
	}
}
