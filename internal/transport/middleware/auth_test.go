package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/evgslyusar/shortlink/internal/domain"
	mw "github.com/evgslyusar/shortlink/internal/transport/middleware"
)

// --- fake validator ---

type fakeValidator struct {
	userID string
	err    error
}

func (f *fakeValidator) ValidateAccessToken(token string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	if token == "valid-token" {
		return f.userID, nil
	}
	return "", domain.ErrUnauthorized
}

// --- RequireAuth tests ---

func TestRequireAuth(t *testing.T) {
	validator := &fakeValidator{userID: "user-1"}

	handler := mw.RequireAuth(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid := mw.UserIDFromContext(r.Context())
		w.Write([]byte(uid)) //nolint:errcheck
	}))

	t.Run("valid token passes and sets user_id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
		if rec.Body.String() != "user-1" {
			t.Errorf("expected user-1, got %q", rec.Body.String())
		}
	})

	t.Run("missing token returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("invalid token returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer bad-token")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("malformed header returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Token valid-token")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("valid cookie passes and sets user_id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: "access_token", Value: "valid-token"})
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
		if rec.Body.String() != "user-1" {
			t.Errorf("expected user-1, got %q", rec.Body.String())
		}
	})

	t.Run("header takes priority over cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		req.AddCookie(&http.Cookie{Name: "access_token", Value: "bad-token"})
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
		if rec.Body.String() != "user-1" {
			t.Errorf("expected user-1, got %q", rec.Body.String())
		}
	})
}

// --- OptionalAuth tests ---

func TestOptionalAuth(t *testing.T) {
	validator := &fakeValidator{userID: "user-1"}

	handler := mw.OptionalAuth(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid := mw.UserIDFromContext(r.Context())
		if uid == "" {
			uid = "guest"
		}
		w.Write([]byte(uid)) //nolint:errcheck
	}))

	t.Run("valid token sets user_id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
		if rec.Body.String() != "user-1" {
			t.Errorf("expected user-1, got %q", rec.Body.String())
		}
	})

	t.Run("no token passes as guest", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
		if rec.Body.String() != "guest" {
			t.Errorf("expected guest, got %q", rec.Body.String())
		}
	})

	t.Run("invalid token passes as guest", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer bad-token")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
		if rec.Body.String() != "guest" {
			t.Errorf("expected guest, got %q", rec.Body.String())
		}
	})

	t.Run("valid cookie sets user_id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: "access_token", Value: "valid-token"})
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
		if rec.Body.String() != "user-1" {
			t.Errorf("expected user-1, got %q", rec.Body.String())
		}
	})
}
