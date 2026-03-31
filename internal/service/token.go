package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/evgslyusar/shortlink/internal/domain"
)

const refreshTokenBytes = 32

// RefreshTokenCreator persists a new refresh token.
type RefreshTokenCreator interface {
	Create(ctx context.Context, token *domain.RefreshToken) error
}

// RefreshTokenByHashFinder looks up a refresh token by its SHA-256 hash.
type RefreshTokenByHashFinder interface {
	FindByHash(ctx context.Context, tokenHash string) (*domain.RefreshToken, error)
}

// RefreshTokenDeleter removes a refresh token by its hash.
type RefreshTokenDeleter interface {
	DeleteByHash(ctx context.Context, tokenHash string) error
}

// TokenService handles JWT access tokens and opaque refresh tokens.
type TokenService struct {
	creator    RefreshTokenCreator
	finder     RefreshTokenByHashFinder
	deleter    RefreshTokenDeleter
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	accessTTL  time.Duration
	refreshTTL time.Duration
	logger     *zap.Logger
}

// NewTokenService creates a new TokenService with pre-parsed RSA keys.
func NewTokenService(
	creator RefreshTokenCreator,
	finder RefreshTokenByHashFinder,
	deleter RefreshTokenDeleter,
	privateKey *rsa.PrivateKey,
	publicKey *rsa.PublicKey,
	accessTTL, refreshTTL time.Duration,
	logger *zap.Logger,
) *TokenService {
	return &TokenService{
		creator:    creator,
		finder:     finder,
		deleter:    deleter,
		privateKey: privateKey,
		publicKey:  publicKey,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		logger:     logger,
	}
}

// IssueTokens generates a new access/refresh token pair for the given user.
func (s *TokenService) IssueTokens(ctx context.Context, userID string) (accessToken, refreshToken string, err error) {
	accessToken, err = s.signAccessToken(userID)
	if err != nil {
		return "", "", fmt.Errorf("token.IssueTokens: %w", err)
	}

	refreshToken, err = s.createRefreshToken(ctx, userID)
	if err != nil {
		return "", "", fmt.Errorf("token.IssueTokens: %w", err)
	}

	s.logger.Info("tokens issued", zap.String("user_id", userID))
	return accessToken, refreshToken, nil
}

// RefreshTokens performs token rotation: validates the old refresh token,
// deletes it, and issues a new pair. Returns ErrUnauthorized on replay.
func (s *TokenService) RefreshTokens(ctx context.Context, rawRefreshToken string) (accessToken, refreshToken string, err error) {
	hash := hashToken(rawRefreshToken)

	stored, err := s.finder.FindByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return "", "", domain.ErrUnauthorized
		}
		return "", "", fmt.Errorf("token.RefreshTokens: %w", err)
	}

	// Delete old token regardless of expiry (cleanup).
	if delErr := s.deleter.DeleteByHash(ctx, hash); delErr != nil {
		s.logger.Error("failed to delete old refresh token", zap.Error(delErr))
	}

	if time.Now().After(stored.ExpiresAt) {
		return "", "", domain.ErrUnauthorized
	}

	return s.IssueTokens(ctx, stored.UserID)
}

// RevokeRefreshToken deletes the refresh token (for logout).
func (s *TokenService) RevokeRefreshToken(ctx context.Context, rawRefreshToken string) error {
	hash := hashToken(rawRefreshToken)
	if err := s.deleter.DeleteByHash(ctx, hash); err != nil {
		return fmt.Errorf("token.RevokeRefreshToken: %w", err)
	}
	return nil
}

// ValidateAccessToken parses and validates an RS256 JWT, returning the user ID.
func (s *TokenService) ValidateAccessToken(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.publicKey, nil
	})
	if err != nil {
		return "", domain.ErrUnauthorized
	}

	sub, err := token.Claims.GetSubject()
	if err != nil || sub == "" {
		return "", domain.ErrUnauthorized
	}

	return sub, nil
}

// AccessTTLSeconds returns the access token TTL in seconds (for expires_in response field).
func (s *TokenService) AccessTTLSeconds() int {
	return int(s.accessTTL.Seconds())
}

func (s *TokenService) signAccessToken(userID string) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTTL)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(s.privateKey)
}

func (s *TokenService) createRefreshToken(ctx context.Context, userID string) (string, error) {
	raw := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generating refresh token: %w", err)
	}
	rawHex := hex.EncodeToString(raw)

	now := time.Now().UTC()
	rt := &domain.RefreshToken{
		ID:        uuid.NewString(),
		UserID:    userID,
		TokenHash: hashToken(rawHex),
		ExpiresAt: now.Add(s.refreshTTL),
		CreatedAt: now,
	}
	if err := s.creator.Create(ctx, rt); err != nil {
		return "", fmt.Errorf("persisting refresh token: %w", err)
	}

	return rawHex, nil
}

func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}
