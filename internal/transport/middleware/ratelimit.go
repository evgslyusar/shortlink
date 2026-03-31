package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RateLimiter decides whether a request identified by key should be allowed.
type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (allowed bool, remaining int, retryAfter time.Duration, err error)
}

// RedisRateLimiter implements RateLimiter using Redis INCR + EXPIRE via Lua
// script for atomicity. A single round-trip increments the counter and sets
// the TTL when the key is first created.
type RedisRateLimiter struct {
	rdb    *redis.Client
	script *redis.Script
}

// rateLimitScript atomically increments a counter and sets its expiry.
// Returns {count, ttl_ms}. The TTL is only set on the first increment to
// avoid resetting the window on every request.
var rateLimitScript = redis.NewScript(`
local count = redis.call('INCR', KEYS[1])
if count == 1 then
    redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
local ttl = redis.call('PTTL', KEYS[1])
return {count, ttl}
`)

// NewRedisRateLimiter creates a rate limiter backed by Redis.
func NewRedisRateLimiter(rdb *redis.Client) *RedisRateLimiter {
	return &RedisRateLimiter{rdb: rdb, script: rateLimitScript}
}

// Allow checks whether the given key has exceeded its limit within the window.
func (rl *RedisRateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, time.Duration, error) {
	result, err := rl.script.Run(ctx, rl.rdb, []string{key}, window.Milliseconds()).Int64Slice()
	if err != nil {
		return false, 0, 0, fmt.Errorf("rate limit script for key %q: %w", key, err)
	}

	count := result[0]
	ttlMs := result[1]

	if count > int64(limit) {
		retryAfter := time.Duration(ttlMs) * time.Millisecond
		if retryAfter <= 0 {
			retryAfter = window
		}
		return false, 0, retryAfter, nil
	}

	remaining := int64(limit) - count
	return true, int(remaining), 0, nil
}

// RateLimitConfig holds parameters for the rate limit middleware.
type RateLimitConfig struct {
	GuestLimit int
	AuthLimit  int
	Window     time.Duration
}

// DefaultRateLimitConfig returns the default rate limit configuration (BR-05).
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		GuestLimit: 10,
		AuthLimit:  100,
		Window:     time.Hour,
	}
}

// RateLimit returns middleware that enforces rate limits on requests.
// It uses the user ID from context (if authenticated) or the client IP as key.
func RateLimit(limiter RateLimiter, cfg RateLimitConfig, logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := UserIDFromContext(r.Context())

			var key string
			var limit int
			if userID != "" {
				key = "rl:auth:" + userID
				limit = cfg.AuthLimit
			} else {
				ip := clientIP(r)
				key = "rl:guest:" + ip
				limit = cfg.GuestLimit
			}

			allowed, remaining, retryAfter, err := limiter.Allow(r.Context(), key, limit, cfg.Window)
			if err != nil {
				// Fail open: if Redis is unavailable, allow the request
				// to preserve availability. This is intentional.
				logger.Warn("rate limiter error, allowing request", zap.Error(err))
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

			if !allowed {
				retrySeconds := int(math.Ceil(retryAfter.Seconds()))
				w.Header().Set("Retry-After", strconv.Itoa(retrySeconds))
				respondRateLimited(w, r)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// BodySizeLimit returns middleware that rejects requests with a body larger
// than maxBytes. It checks Content-Length for a fast reject and wraps the body
// with http.MaxBytesReader to enforce the limit on chunked transfers.
func BodySizeLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > maxBytes {
				respondPayloadTooLarge(w, r)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// clientIP extracts the client IP from RemoteAddr. We intentionally do NOT
// trust X-Forwarded-For or X-Real-IP because there is no trusted reverse proxy
// configured — those headers are trivially spoofable by clients.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func respondRateLimited(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
		"error": map[string]string{
			"code":    "RATE_LIMITED",
			"message": "too many requests",
		},
		"meta": map[string]string{
			"request_id": GetRequestID(r.Context()),
		},
	})
}

func respondPayloadTooLarge(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusRequestEntityTooLarge)
	json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
		"error": map[string]string{
			"code":    "PAYLOAD_TOO_LARGE",
			"message": "request body too large",
		},
		"meta": map[string]string{
			"request_id": GetRequestID(r.Context()),
		},
	})
}
