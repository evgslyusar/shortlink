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

// AuthHandler handles HTTP requests for authentication endpoints.
type AuthHandler struct {
	reg    Registerer
	auth   Authenticator
	logger *zap.Logger
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(reg Registerer, auth Authenticator, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{
		reg:    reg,
		auth:   auth,
		logger: logger,
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

// Register handles POST /v1/auth/register.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	req, err := decodeJSON[authRequest](w, r)
	if err != nil {
		respondError(w, r, http.StatusUnprocessableEntity, ErrCodeValidation, err.Error())
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
		respondError(w, r, http.StatusUnprocessableEntity, ErrCodeValidation, err.Error())
		return
	}

	user, err := h.auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		status, code, msg := mapError(err)
		h.logError(err, status)
		respondError(w, r, status, code, msg)
		return
	}

	respondData(w, r, http.StatusOK, userResponse{
		UserID: user.ID,
		Email:  user.Email,
	})
}

func (h *AuthHandler) logError(err error, status int) {
	if status >= http.StatusInternalServerError {
		h.logger.Error("internal error", zap.Error(err))
	}
}
