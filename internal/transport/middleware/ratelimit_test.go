package middleware_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	mw "github.com/evgslyusar/shortlink/internal/transport/middleware"
)

// fakeLimiter implements mw.RateLimiter for testing.
type fakeLimiter struct {
	count    int
	limit    int
	lastKey  string
	lastWin  time.Duration
}

func (f *fakeLimiter) Allow(_ context.Context, key string, limit int, window time.Duration) (bool, int, time.Duration, error) {
	f.count++
	f.limit = limit
	f.lastKey = key
	f.lastWin = window
	if f.count > limit {
		return false, 0, 30 * time.Second, nil
	}
	return true, limit - f.count, 0, nil
}

func TestRateLimit_GuestBlocked(t *testing.T) {
	limiter := &fakeLimiter{}
	logger := zaptest.NewLogger(t)
	cfg := mw.RateLimitConfig{GuestLimit: 2, AuthLimit: 100, Window: time.Hour}
	handler := mw.RateLimit(limiter, cfg, logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First two requests succeed.
	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/links", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, "request %d should succeed", i+1)
	}

	// Third request is rate-limited.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/links", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Retry-After"))
	assert.Equal(t, 2, limiter.limit, "should use guest limit")
}

func TestRateLimit_GuestKeyFormat(t *testing.T) {
	limiter := &fakeLimiter{}
	logger := zaptest.NewLogger(t)
	cfg := mw.DefaultRateLimitConfig()
	handler := mw.RateLimit(limiter, cfg, logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/links", nil)
	req.RemoteAddr = "10.0.0.5:9999"
	handler.ServeHTTP(rec, req)

	assert.Equal(t, "rl:guest:10.0.0.5", limiter.lastKey)
	assert.Equal(t, time.Hour, limiter.lastWin)
}

func TestRateLimit_AuthUserKeyFormat(t *testing.T) {
	limiter := &fakeLimiter{}
	logger := zaptest.NewLogger(t)
	cfg := mw.RateLimitConfig{GuestLimit: 2, AuthLimit: 5, Window: time.Hour}

	handler := setUserID("user-abc")(
		mw.RateLimit(limiter, cfg, logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})),
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/links", nil)
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "rl:auth:user-abc", limiter.lastKey)
	assert.Equal(t, 5, limiter.limit, "should use auth limit")
}

func TestRateLimit_ResponseHeaders(t *testing.T) {
	limiter := &fakeLimiter{}
	logger := zaptest.NewLogger(t)
	cfg := mw.DefaultRateLimitConfig()
	handler := mw.RateLimit(limiter, cfg, logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/links", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "10", rec.Header().Get("X-RateLimit-Limit"))
	assert.Equal(t, "9", rec.Header().Get("X-RateLimit-Remaining"))
}

func TestRateLimit_429ResponseBody(t *testing.T) {
	limiter := &fakeLimiter{count: 100} // already over any reasonable limit
	logger := zaptest.NewLogger(t)
	cfg := mw.RateLimitConfig{GuestLimit: 1, AuthLimit: 1, Window: time.Hour}
	handler := mw.RateLimit(limiter, cfg, logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/links", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)

	var body map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	errObj, ok := body["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "RATE_LIMITED", errObj["code"])
	assert.Equal(t, "too many requests", errObj["message"])
}

func TestRateLimit_FailOpen(t *testing.T) {
	limiter := &errorLimiter{}
	logger := zaptest.NewLogger(t)
	cfg := mw.DefaultRateLimitConfig()

	var called bool
	handler := mw.RateLimit(limiter, cfg, logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/links", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	handler.ServeHTTP(rec, req)

	assert.True(t, called, "should call next handler on limiter error")
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestSecurity_Headers(t *testing.T) {
	handler := mw.Security(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
	assert.Equal(t, "strict-origin-when-cross-origin", rec.Header().Get("Referrer-Policy"))
	// CSP is not set by Security — use APISecurityHeaders for API routes.
	assert.Empty(t, rec.Header().Get("Content-Security-Policy"))
}

func TestAPISecurityHeaders(t *testing.T) {
	handler := mw.APISecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	assert.Equal(t, "default-src 'none'", rec.Header().Get("Content-Security-Policy"))
}

func TestBodySizeLimit_TooLarge(t *testing.T) {
	handler := mw.BodySizeLimit(1024)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.ContentLength = 2048
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
}

func TestBodySizeLimit_Allowed(t *testing.T) {
	handler := mw.BodySizeLimit(1024)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.ContentLength = 512
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestBodySizeLimit_ChunkedTransfer(t *testing.T) {
	handler := mw.BodySizeLimit(10)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1024)
		_, err := r.Body.Read(buf)
		if err != nil {
			// MaxBytesReader returns an error when body exceeds the limit.
			http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	// No Content-Length set, but body is larger than limit.
	rec := httptest.NewRecorder()
	body := strings.NewReader("this body is definitely longer than ten bytes")
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.ContentLength = -1 // simulate chunked / unknown length
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
}

// setUserID is a test helper that simulates authenticated context.
func setUserID(userID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := mw.ContextWithUserID(r.Context(), userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// errorLimiter always returns an error.
type errorLimiter struct{}

func (e *errorLimiter) Allow(context.Context, string, int, time.Duration) (bool, int, time.Duration, error) {
	return false, 0, 0, context.DeadlineExceeded
}
