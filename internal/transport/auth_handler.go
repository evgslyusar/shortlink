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
	RefreshTTLSeconds() int
}

// TokenRefresher rotates a refresh token into a new token pair.
type TokenRefresher interface {
	RefreshTokens(ctx context.Context, rawRefreshToken string) (accessToken, refreshToken string, err error)
	AccessTTLSeconds() int
	RefreshTTLSeconds() int
}

// TokenRevoker revokes a refresh token (logout).
type TokenRevoker interface {
	RevokeRefreshToken(ctx context.Context, rawRefreshToken string) error
}

// AuthHandler handles HTTP requests for authentication endpoints.
type AuthHandler struct {
	reg          Registerer
	auth         Authenticator
	issuer       TokenIssuer
	refresher    TokenRefresher
	revoker      TokenRevoker
	secureCookie bool
	logger       *zap.Logger
}

// NewAuthHandler creates a new AuthHandler.
// secureCookie controls the Secure flag on auth cookies (true for HTTPS, false for local HTTP dev).
func NewAuthHandler(
	reg Registerer,
	auth Authenticator,
	issuer TokenIssuer,
	refresher TokenRefresher,
	revoker TokenRevoker,
	secureCookie bool,
	logger *zap.Logger,
) *AuthHandler {
	return &AuthHandler{
		reg:          reg,
		auth:         auth,
		issuer:       issuer,
		refresher:    refresher,
		revoker:      revoker,
		secureCookie: secureCookie,
		logger:       logger,
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

type loginResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	ExpiresIn    int          `json:"expires_in"`
	User         userResponse `json:"user"`
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

	setAuthCookies(w, accessToken, refreshToken, h.issuer.AccessTTLSeconds(), h.issuer.RefreshTTLSeconds(), h.secureCookie)

	respondData(w, r, http.StatusOK, loginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    h.issuer.AccessTTLSeconds(),
		User: userResponse{
			UserID: user.ID,
			Email:  user.Email,
		},
	})
}

// Refresh handles POST /v1/auth/refresh.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	// Decode body; ignore errors so cookie-only clients can send an empty body.
	req, _ := decodeJSON[refreshRequest](w, r)

	// Fallback: read refresh token from cookie if not in JSON body.
	rawRefresh := req.RefreshToken
	if rawRefresh == "" {
		if c, cookieErr := r.Cookie("refresh_token"); cookieErr == nil {
			rawRefresh = c.Value
		}
	}

	if rawRefresh == "" {
		respondError(w, r, http.StatusBadRequest, ErrCodeValidation, "refresh_token is required")
		return
	}

	accessToken, refreshToken, err := h.refresher.RefreshTokens(r.Context(), rawRefresh)
	if err != nil {
		status, code, msg := mapError(err)
		h.logError(err, status)
		respondError(w, r, status, code, msg)
		return
	}

	setAuthCookies(w, accessToken, refreshToken, h.refresher.AccessTTLSeconds(), h.refresher.RefreshTTLSeconds(), h.secureCookie)

	respondData(w, r, http.StatusOK, tokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    h.refresher.AccessTTLSeconds(),
	})
}

// Logout handles POST /v1/auth/logout.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// Decode body; ignore errors so cookie-only clients can send an empty body.
	req, _ := decodeJSON[logoutRequest](w, r)

	// Fallback: read refresh token from cookie if not in JSON body.
	rawRefresh := req.RefreshToken
	if rawRefresh == "" {
		if c, cookieErr := r.Cookie("refresh_token"); cookieErr == nil {
			rawRefresh = c.Value
		}
	}

	if rawRefresh == "" {
		respondError(w, r, http.StatusBadRequest, ErrCodeValidation, "refresh_token is required")
		return
	}

	if err := h.revoker.RevokeRefreshToken(r.Context(), rawRefresh); err != nil {
		h.logError(err, http.StatusInternalServerError)
		respondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "internal error")
		return
	}

	clearAuthCookies(w, h.secureCookie)
	w.WriteHeader(http.StatusNoContent)
}

func setAuthCookies(w http.ResponseWriter, accessToken, refreshToken string, accessTTL, refreshTTL int, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		Path:     "/",
		MaxAge:   accessTTL,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/v1/auth",
		MaxAge:   refreshTTL,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearAuthCookies(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/v1/auth",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *AuthHandler) logError(err error, status int) {
	if status >= http.StatusInternalServerError {
		h.logger.Error("internal error", zap.Error(err))
	}
}
