package transport

import (
	"context"
	"net/http"

	"go.uber.org/zap"

	"github.com/evgslyusar/shortlink/internal/domain"
)

// Registerer creates a new user account.
type Registerer interface {
	Register(ctx context.Context, email, password string) (*domain.User, error)
}

// Authenticator verifies user credentials.
type Authenticator interface {
	Login(ctx context.Context, email, password string) (*domain.User, error)
}

// TokenIssuer issues access and refresh token pairs.
type TokenIssuer interface {
	IssueTokens(ctx context.Context, userID string) (accessToken, refreshToken string, err error)
	AccessTTLSeconds() int
}

// TokenRefresher rotates a refresh token into a new token pair.
type TokenRefresher interface {
	RefreshTokens(ctx context.Context, rawRefreshToken string) (accessToken, refreshToken string, err error)
	AccessTTLSeconds() int
}

// TokenRevoker revokes a refresh token (logout).
type TokenRevoker interface {
	RevokeRefreshToken(ctx context.Context, rawRefreshToken string) error
}

// AuthHandler handles HTTP requests for authentication endpoints.
type AuthHandler struct {
	reg       Registerer
	auth      Authenticator
	issuer    TokenIssuer
	refresher TokenRefresher
	revoker   TokenRevoker
	logger    *zap.Logger
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(
	reg Registerer,
	auth Authenticator,
	issuer TokenIssuer,
	refresher TokenRefresher,
	revoker TokenRevoker,
	logger *zap.Logger,
) *AuthHandler {
	return &AuthHandler{
		reg:       reg,
		auth:      auth,
		issuer:    issuer,
		refresher: refresher,
		revoker:   revoker,
		logger:    logger,
	}
}

type authRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userResponse struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// Register handles POST /v1/auth/register.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	req, err := decodeJSON[authRequest](w, r)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, ErrCodeValidation, "invalid request body")
		return
	}

	user, err := h.reg.Register(r.Context(), req.Email, req.Password)
	if err != nil {
		status, code, msg := mapError(err)
		h.logError(err, status)
		respondError(w, r, status, code, msg)
		return
	}

	respondData(w, r, http.StatusCreated, userResponse{
		UserID: user.ID,
		Email:  user.Email,
	})
}

// Login handles POST /v1/auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	req, err := decodeJSON[authRequest](w, r)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, ErrCodeValidation, "invalid request body")
		return
	}

	user, err := h.auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		status, code, msg := mapError(err)
		h.logError(err, status)
		respondError(w, r, status, code, msg)
		return
	}

	accessToken, refreshToken, err := h.issuer.IssueTokens(r.Context(), user.ID)
	if err != nil {
		h.logError(err, http.StatusInternalServerError)
		respondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "internal error")
		return
	}

	respondData(w, r, http.StatusOK, tokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    h.issuer.AccessTTLSeconds(),
	})
}

// Refresh handles POST /v1/auth/refresh.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	req, err := decodeJSON[refreshRequest](w, r)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, ErrCodeValidation, "invalid request body")
		return
	}

	if req.RefreshToken == "" {
		respondError(w, r, http.StatusBadRequest, ErrCodeValidation, "refresh_token is required")
		return
	}

	accessToken, refreshToken, err := h.refresher.RefreshTokens(r.Context(), req.RefreshToken)
	if err != nil {
		status, code, msg := mapError(err)
		h.logError(err, status)
		respondError(w, r, status, code, msg)
		return
	}

	respondData(w, r, http.StatusOK, tokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    h.refresher.AccessTTLSeconds(),
	})
}

// Logout handles POST /v1/auth/logout.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	req, err := decodeJSON[logoutRequest](w, r)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, ErrCodeValidation, "invalid request body")
		return
	}

	if req.RefreshToken == "" {
		respondError(w, r, http.StatusBadRequest, ErrCodeValidation, "refresh_token is required")
		return
	}

	if err := h.revoker.RevokeRefreshToken(r.Context(), req.RefreshToken); err != nil {
		h.logError(err, http.StatusInternalServerError)
		respondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) logError(err error, status int) {
	if status >= http.StatusInternalServerError {
		h.logger.Error("internal error", zap.Error(err))
	}
}
