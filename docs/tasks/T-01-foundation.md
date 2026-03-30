# T-01: Foundation

**Status:** Done (PR #1)
**Branch:** `feature/t-01-foundation`
**Deps:** none

---

## Summary

Database migrations for all 5 tables. Domain types with zero external imports.
Sentinel errors and `ValidationError`. Slug generation (crypto/rand, 6-char base62) and validation.
Switch both binaries from slog to zap. JWT config fields. Makefile targets.
Request logging and correlation-id middleware.

---

## Files Created

| File | Description |
|------|-------------|
| `migrations/000001_create_users.{up,down}.sql` | users table with email unique constraint |
| `migrations/000002_create_links.{up,down}.sql` | links table with slug unique, partial indexes on user_id/expires_at |
| `migrations/000003_create_clicks.{up,down}.sql` | clicks table with composite index on (link_id, clicked_at) |
| `migrations/000004_create_refresh_tokens.{up,down}.sql` | refresh_tokens with unique token_hash |
| `migrations/000005_create_telegram_accounts.{up,down}.sql` | telegram_accounts with unique user_id and telegram_id |
| `internal/domain/user.go` | `User{ID, Email, Password, CreatedAt}` |
| `internal/domain/link.go` | `Link{ID, Slug, OriginalURL, UserID*, ExpiresAt*, CreatedAt}` + `IsExpired()`, `IsOwnedBy()` |
| `internal/domain/click.go` | `Click{ID, LinkID, ClickedAt, Country*, Referer*, UserAgent*}` |
| `internal/domain/errors.go` | `ErrNotFound`, `ErrAlreadyExists`, `ErrForbidden`, `ErrUnauthorized`, `ValidationError` |
| `internal/domain/slug.go` | `GenerateSlug()` (6-char base62, crypto/rand), `ValidateCustomSlug()` (4-12 chars, `[a-zA-Z0-9_-]`) |
| `internal/domain/slug_test.go` | Table-driven tests for slug generation and validation |
| `internal/transport/middleware/correlation.go` | X-Request-ID injection (reuse or generate UUID v4) |
| `internal/transport/middleware/logger.go` | Structured request logging (method, path, status, duration, request_id) |
| `internal/transport/middleware/logger.go` | `Recovery()` — panic → 500, log error |

## Files Modified

| File | Changes |
|------|---------|
| `go.mod` | Added golang-migrate, google/uuid, go.uber.org/zap, golang-jwt/jwt/v5 |
| `cmd/slinkapi/main.go` | slog → zap, extracted middleware |
| `cmd/slinkbot/main.go` | slog → zap |
| `internal/config/config.go` | Added JWT fields (key paths, TTLs), BaseURL, bot webhook URL |
| `internal/telegram/handler.go` | slog → zap |
| `Makefile` | migrate-{up,down,status,new}, gen-keys, dev-api, dev-bot, test-*, lint |
| `.gitignore` | Added `keys/` |

---

## Domain Types

```go
// Zero external imports in internal/domain/

type User struct {
    ID, Email, Password string
    CreatedAt           time.Time
}

type Link struct {
    ID, Slug, OriginalURL string
    UserID                *string
    ExpiresAt             *time.Time
    CreatedAt             time.Time
}

type Click struct {
    ID, LinkID            string
    ClickedAt             time.Time
    Country, Referer, UserAgent *string
}

type ValidationError struct {
    Field, Message string
}

var (
    ErrNotFound      = errors.New("not found")
    ErrAlreadyExists = errors.New("already exists")
    ErrForbidden     = errors.New("forbidden")
    ErrUnauthorized  = errors.New("unauthorized")
)
```

## Slug Rules

- **GenerateSlug():** 6 chars, base62 (A-Z a-z 0-9), `crypto/rand`
- **ValidateCustomSlug():** 4–12 chars, `^[a-zA-Z0-9_-]+$`, returns `*ValidationError`

## Config Struct

```go
type Config struct {
    ServerHost, ServerPort, BaseURL      // HTTP server
    DatabaseURL, RedisURL                // infra
    LogLevel                             // zap level
    RequestTimeout, IdleTimeout          // timeouts
    JWTPrivateKeyPath, JWTPublicKeyPath  // RS256 keys
    JWTAccessTTL, JWTRefreshTTL          // token lifetimes
    TelegramBotToken, TelegramWebhookURL // bot
    BotHost, BotPort                     // bot HTTP
}
```

## Middleware Chain (order)

1. `Correlation` — inject/propagate X-Request-ID
2. `Recovery` — catch panics → 500
3. `Logger` — log method, path, status, duration, request_id

---

## Acceptance Criteria

- [x] `make migrate-up` runs all 5 migration pairs cleanly
- [x] `make migrate-down` (x5) reverses all migrations
- [x] `make gen-keys` creates `keys/{private,public}.pem`; `keys/` is gitignored
- [x] `go build ./...` compiles after zap switch
- [x] `internal/domain` has zero non-stdlib imports
- [x] `GenerateSlug` produces 6-char base62 strings; table-driven tests pass
- [x] `ValidateCustomSlug` rejects invalid slugs; table-driven tests pass
