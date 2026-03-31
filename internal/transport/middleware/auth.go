package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

type userIDKey struct{}

// AccessTokenValidator validates a JWT access token and returns the user ID.
type AccessTokenValidator interface {
	ValidateAccessToken(tokenString string) (userID string, err error)
}

// RequireAuth returns middleware that rejects requests without a valid Bearer token.
func RequireAuth(validator AccessTokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := extractAndValidate(r, validator)
			if !ok {
				respondUnauthorized(w, r)
				return
			}
			ctx := context.WithValue(r.Context(), userIDKey{}, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalAuth returns middleware that sets the user ID if a valid Bearer token
// is present, but allows the request to proceed without one.
func OptionalAuth(validator AccessTokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if userID, ok := extractAndValidate(r, validator); ok {
				ctx := context.WithValue(r.Context(), userIDKey{}, userID)
				r = r.WithContext(ctx)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// UserIDFromContext extracts the user ID set by auth middleware.
// Returns empty string if not set.
func UserIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(userIDKey{}).(string)
	return id
}

// ContextWithUserID returns a new context with the given user ID.
// Intended for testing and internal middleware composition.
func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey{}, userID)
}

func respondUnauthorized(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
		"error": map[string]string{
			"code":    "UNAUTHORIZED",
			"message": "invalid credentials",
		},
		"meta": map[string]string{
			"request_id": GetRequestID(r.Context()),
		},
	})
}

func extractAndValidate(r *http.Request, validator AccessTokenValidator) (string, bool) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return "", false
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	userID, err := validator.ValidateAccessToken(parts[1])
	if err != nil {
		return "", false
	}
	return userID, true
}
