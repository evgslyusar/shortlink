package service_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/evgslyusar/shortlink/internal/domain"
	"github.com/evgslyusar/shortlink/internal/service"
)

// --- fakes ---

type fakeRefreshTokenStore struct {
	tokens map[string]*domain.RefreshToken // keyed by token_hash
	err    error
}

func newFakeRefreshTokenStore() *fakeRefreshTokenStore {
	return &fakeRefreshTokenStore{tokens: make(map[string]*domain.RefreshToken)}
}

func (f *fakeRefreshTokenStore) Create(_ context.Context, token *domain.RefreshToken) error {
	if f.err != nil {
		return f.err
	}
	f.tokens[token.TokenHash] = token
	return nil
}

func (f *fakeRefreshTokenStore) FindByHash(_ context.Context, tokenHash string) (*domain.RefreshToken, error) {
	if f.err != nil {
		return nil, f.err
	}
	t, ok := f.tokens[tokenHash]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return t, nil
}

func (f *fakeRefreshTokenStore) DeleteByHash(_ context.Context, tokenHash string) error {
	if f.err != nil {
		return f.err
	}
	delete(f.tokens, tokenHash)
	return nil
}

// --- helpers ---

func newTestTokenService(t *testing.T, store *fakeRefreshTokenStore) *service.TokenService {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}
	return service.NewTokenService(
		store, store, store,
		key, &key.PublicKey,
		15*time.Minute, 7*24*time.Hour,
		zap.NewNop(),
	)
}

// --- tests ---

func TestTokenService_IssueAndValidate(t *testing.T) {
	store := newFakeRefreshTokenStore()
	svc := newTestTokenService(t, store)

	accessToken, refreshToken, err := svc.IssueTokens(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if accessToken == "" {
		t.Error("expected non-empty access token")
	}
	if refreshToken == "" {
		t.Error("expected non-empty refresh token")
	}

	// Validate access token.
	userID, err := svc.ValidateAccessToken(accessToken)
	if err != nil {
		t.Fatalf("unexpected error validating token: %v", err)
	}
	if userID != "user-1" {
		t.Errorf("expected user-1, got %q", userID)
	}

	// Refresh token should be stored.
	if len(store.tokens) != 1 {
		t.Errorf("expected 1 stored token, got %d", len(store.tokens))
	}
}

func TestTokenService_ValidateAccessToken_Invalid(t *testing.T) {
	store := newFakeRefreshTokenStore()
	svc := newTestTokenService(t, store)

	_, err := svc.ValidateAccessToken("invalid-jwt")
	if err != domain.ErrUnauthorized {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestTokenService_RefreshTokens(t *testing.T) {
	t.Run("valid refresh rotates tokens", func(t *testing.T) {
		store := newFakeRefreshTokenStore()
		svc := newTestTokenService(t, store)

		_, refreshToken, err := svc.IssueTokens(context.Background(), "user-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		newAccess, newRefresh, err := svc.RefreshTokens(context.Background(), refreshToken)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if newAccess == "" || newRefresh == "" {
			t.Error("expected non-empty new tokens")
		}
		if newRefresh == refreshToken {
			t.Error("new refresh token should differ from old one")
		}

		// Old token should be deleted, new one stored.
		if len(store.tokens) != 1 {
			t.Errorf("expected 1 stored token after rotation, got %d", len(store.tokens))
		}
	})

	t.Run("reused refresh token returns unauthorized", func(t *testing.T) {
		store := newFakeRefreshTokenStore()
		svc := newTestTokenService(t, store)

		_, refreshToken, err := svc.IssueTokens(context.Background(), "user-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// First refresh: succeeds.
		_, _, err = svc.RefreshTokens(context.Background(), refreshToken)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Second refresh with same token: replay detected.
		_, _, err = svc.RefreshTokens(context.Background(), refreshToken)
		if err != domain.ErrUnauthorized {
			t.Fatalf("expected ErrUnauthorized on replay, got %v", err)
		}
	})

	t.Run("expired refresh token returns unauthorized", func(t *testing.T) {
		store := newFakeRefreshTokenStore()
		svc := newTestTokenService(t, store)

		_, refreshToken, err := svc.IssueTokens(context.Background(), "user-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Manually expire the stored token.
		for _, tok := range store.tokens {
			tok.ExpiresAt = time.Now().Add(-time.Hour)
		}

		_, _, err = svc.RefreshTokens(context.Background(), refreshToken)
		if err != domain.ErrUnauthorized {
			t.Fatalf("expected ErrUnauthorized on expired token, got %v", err)
		}
	})
}

func TestTokenService_RevokeRefreshToken(t *testing.T) {
	store := newFakeRefreshTokenStore()
	svc := newTestTokenService(t, store)

	_, refreshToken, err := svc.IssueTokens(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := svc.RevokeRefreshToken(context.Background(), refreshToken); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(store.tokens) != 0 {
		t.Error("expected token to be deleted after revoke")
	}
}

func TestTokenService_AccessTTLSeconds(t *testing.T) {
	store := newFakeRefreshTokenStore()
	svc := newTestTokenService(t, store)

	if svc.AccessTTLSeconds() != 900 {
		t.Errorf("expected 900 seconds, got %d", svc.AccessTTLSeconds())
	}
}
