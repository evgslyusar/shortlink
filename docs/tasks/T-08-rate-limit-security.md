# T-08: Rate Limiting & Security

**Status:** Done
**Branch:** `feature/t-08-rate-limit`
**Deps:** T-06

---

## Summary

Redis-based rate limiter (INCR + EXPIRE). Security headers middleware.
Request body size limit (1MB).

---

## Files to Create

| File | Description |
|------|-------------|
| `internal/transport/middleware/ratelimit.go` | Rate limit middleware with Redis backend |
| `internal/transport/middleware/ratelimit_test.go` | Tests for limit enforcement |
| `internal/transport/middleware/security.go` | Security headers middleware |

## Files to Modify

| File | Changes |
|------|---------|
| `cmd/slinkapi/main.go` | Wire rate limiter, apply security headers middleware |

---

## Interfaces

```go
// middleware/ratelimit.go
type RateLimiter interface {
    Allow(ctx context.Context, key string, limit int, window time.Duration) (allowed bool, remaining int, retryAfter time.Duration, err error)
}
```

## Rate Limits (BR-05)

| Scope | Endpoint | Limit | Window | Key |
|-------|----------|-------|--------|-----|
| Guest | `POST /v1/links` | 10 | 1 hour | IP address |
| Auth user | `POST /v1/links` | 100 | 1 hour | user_id |
| Redirects | `GET /{slug}` | **Not rate-limited** | — | — |

## Redis Implementation

```
key = "rl:{scope}:{identifier}"  // e.g. "rl:guest:192.168.1.1"
INCR key
if count == 1: EXPIRE key {window}
if count > limit: reject with 429
```

## 429 Response

```json
{
  "error": { "code": "RATE_LIMITED", "message": "too many requests" },
  "meta": { "request_id": "..." }
}
```
Headers: `Retry-After: {seconds}`

## Security Headers

Applied to all responses:

| Header | Value |
|--------|-------|
| `X-Content-Type-Options` | `nosniff` |
| `X-Frame-Options` | `DENY` |
| `Content-Security-Policy` | `default-src 'none'` |
| `Referrer-Policy` | `strict-origin-when-cross-origin` |

## Body Size Limit

Already implemented in `request.go` as `maxBodySize = 1 << 20` (1MB).
Add middleware-level enforcement for 413 response on oversized bodies.

---

## Acceptance Criteria

- [x] 11th guest creation in 1hr: 429 with Retry-After
- [x] 101st auth creation in 1hr: 429 with Retry-After
- [x] Redirects are not rate-limited
- [x] All responses include security headers
- [x] Body > 1MB: 413
