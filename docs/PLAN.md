# PLAN.md — Shortlink Implementation Plan

## Context

Shortlink is a greenfield URL shortening service with REST API and Telegram bot (@SlinkBot).
Infrastructure is scaffolded (Postgres, Redis, Chi router, config, Telegram webhook stubs),
but domain, service, repository, and transport layers are empty. This plan covers the full
backend implementation from database schema through production hardening.

**Key decisions (from CLAUDE.md Architecture Decisions):**
- Logging: `go.uber.org/zap` (replace current slog)
- Migrations: `golang-migrate/migrate`
- Click tracking: async via buffered channel (1000) + background worker
- JWT: RS256 via `golang-jwt/jwt/v5`, keys generated with `make gen-keys`
- Telegram: `go-telegram-bot-api/telegram-bot-api/v5`
- TelegramAccount: separate table (not a column on users)
- Refresh tokens: opaque, SHA-256 hashed in DB, rotation on use

---

## 1. Database Schema

### `users`
```sql
CREATE TABLE users (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email      TEXT        NOT NULL,
    password   TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_users_email UNIQUE (email)
);
```

### `links`
```sql
CREATE TABLE links (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    slug         TEXT        NOT NULL,
    original_url TEXT        NOT NULL,
    user_id      UUID        REFERENCES users(id) ON DELETE SET NULL,
    expires_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_links_slug UNIQUE (slug)
);
CREATE INDEX idx_links_user_id    ON links(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX idx_links_expires_at ON links(expires_at) WHERE expires_at IS NOT NULL;
```

### `clicks`
```sql
CREATE TABLE clicks (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    link_id    UUID        NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    clicked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    country    CHAR(2),
    referer    TEXT,
    user_agent TEXT
);
CREATE INDEX idx_clicks_link_id    ON clicks(link_id);
CREATE INDEX idx_clicks_clicked_at ON clicks(link_id, clicked_at);
```

### `refresh_tokens`
```sql
CREATE TABLE refresh_tokens (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT        NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_refresh_tokens_hash UNIQUE (token_hash)
);
CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);
```

### `telegram_accounts`
```sql
CREATE TABLE telegram_accounts (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    telegram_id BIGINT      NOT NULL,
    username    TEXT,
    linked_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_telegram_accounts_user_id     UNIQUE (user_id),
    CONSTRAINT uq_telegram_accounts_telegram_id UNIQUE (telegram_id)
);
```

---

## 2. Task List

### T-01: Foundation
**Deps:** none

Database migrations for all 5 tables. Domain types (`User`, `Link`, `Click`, `RefreshToken`,
`TelegramAccount`, `Stats`) with zero external imports (use `string` for IDs). Sentinel errors.
Slug generation (`crypto/rand`, 6 base62 chars) and validation. Switch both binaries + telegram
handler from `slog` to `zap`. Add JWT config fields. Makefile: `migrate-*`, `gen-keys`, `dev-api`,
`dev-bot`. Extract request logging and correlation-id into `internal/transport/middleware/`.

**Files created:**
- `migrations/000001_create_users.{up,down}.sql`
- `migrations/000002_create_links.{up,down}.sql`
- `migrations/000003_create_clicks.{up,down}.sql`
- `migrations/000004_create_refresh_tokens.{up,down}.sql`
- `migrations/000005_create_telegram_accounts.{up,down}.sql`
- `internal/domain/user.go`, `link.go`, `click.go`, `errors.go`, `slug.go`, `slug_test.go`
- `internal/transport/middleware/logger.go`, `correlation.go`

**Files modified:**
- `go.mod` — add `golang-migrate/migrate/v4`, `google/uuid`, `go.uber.org/zap`, `golang-jwt/jwt/v5`
- `cmd/slinkapi/main.go` — slog -> zap, extract middleware
- `cmd/slinkbot/main.go` — slog -> zap
- `internal/config/config.go` — JWT fields, bot webhook URL
- `internal/telegram/handler.go` — slog -> zap
- `Makefile` — migrate-*, gen-keys, dev-api, dev-bot
- `.gitignore` — add `keys/`

**Acceptance criteria:**
- [ ] `make migrate-up` runs all 5 migration pairs cleanly
- [ ] `make migrate-down` (x5) reverses all migrations
- [ ] `make gen-keys` creates `keys/{private,public}.pem`; `keys/` is gitignored
- [ ] `go build ./...` compiles after zap switch
- [ ] `internal/domain` has zero non-stdlib imports
- [ ] `GenerateSlug` produces 6-char base62 strings; table-driven tests pass
- [ ] `ValidateCustomSlug` rejects invalid slugs; table-driven tests pass

---

### T-02: User Registration & Login
**Deps:** T-01

User persistence (pgx), auth service (register + login, bcrypt cost >= 12), HTTP handlers
for `POST /v1/auth/register` (201) and `POST /v1/auth/login` (200). JSON envelope helpers.
Decode+validate helper. Shared test utilities.

Login returns user data only — JWT issuance is added in T-06.

**Files created:**
- `internal/repository/user_postgres.go`
- `internal/service/auth.go`, `auth_test.go`
- `internal/transport/auth_handler.go`, `response.go`, `request.go`
- `internal/testutil/testutil.go`

**Key interfaces:**
```go
// service/auth.go (consumer of persistence)
type UserCreator interface {
    CreateUser(ctx context.Context, user *domain.User) error
}
type UserByEmailFinder interface {
    FindByEmail(ctx context.Context, email string) (*domain.User, error)
}

// transport/auth_handler.go (consumer of service)
type Registerer interface {
    Register(ctx context.Context, email, password string) (*domain.User, error)
}
type Authenticator interface {
    Login(ctx context.Context, email, password string) (*domain.User, error)
}
```

**Acceptance criteria:**
- [ ] Register: 201 on success, 409 duplicate email, 422 validation errors (all at once)
- [ ] Login: 200 on success, 401 on bad credentials (same message for missing user)
- [ ] Password stored as bcrypt hash (cost >= 12)
- [ ] Unit tests for auth service with mock store
- [ ] Integration test for user repository with testcontainers

---

### T-03: Link CRUD
**Deps:** T-01, T-02

Link persistence, link service (create with auto/custom slug, list by user, delete), HTTP
handlers for `POST /v1/links` (201), `GET /v1/links` (200 paginated), `DELETE /v1/links/:slug` (204).

**Files created:**
- `internal/repository/link_postgres.go`
- `internal/service/link.go`, `link_test.go`
- `internal/transport/link_handler.go`

**Key interfaces:**
```go
// service/link.go (consumer)
type LinkCreator interface {
    CreateLink(ctx context.Context, link *domain.Link) error
}
type LinkBySlugFinder interface {
    FindBySlug(ctx context.Context, slug string) (*domain.Link, error)
}
type LinksByUserLister interface {
    ListByUser(ctx context.Context, userID string, page, perPage int) ([]domain.Link, int, error)
}
type LinkDeleter interface {
    DeleteBySlug(ctx context.Context, slug string) error
}
```

**Business logic:**
- No custom slug -> `GenerateSlug()`, retry up to 3x on collision
- Custom slug -> `ValidateCustomSlug()`
- No userID -> `expires_at = now + 7 days` (BR-03)
- Delete checks ownership -> `ErrForbidden` if mismatch

**Acceptance criteria:**
- [ ] Auto slug: 201 with 6-char slug; custom slug: 201 when valid, 422 when invalid
- [ ] Duplicate slug: 409
- [ ] Guest link: expires_at set to now+7d; user link: no expiry
- [ ] `GET /v1/links`: paginated with `page`, `per_page`, `total` in meta
- [ ] Delete by owner: 204; by non-owner: 403
- [ ] Unit tests for collision retry and guest expiry logic

---

### T-04: Redirect with Redis Cache
**Deps:** T-03

Redis cache for slug->URL. Redirect handler at `GET /:slug` (top-level, no `/v1/`).
Flow: check Redis -> miss -> check Postgres -> populate Redis -> 302. Expired links: 404.

**Files created:**
- `internal/repository/link_cache.go`
- `internal/transport/redirect_handler.go`

**Key interface:**
```go
// service/link.go or redirect service (consumer)
type LinkCache interface {
    GetOriginalURL(ctx context.Context, slug string) (string, error)
    SetOriginalURL(ctx context.Context, slug string, url string, ttl time.Duration) error
}
```

**Acceptance criteria:**
- [ ] Valid slug: 302 with `Location` header
- [ ] Missing/expired slug: 404
- [ ] Second request hits Redis only (Postgres not called again)
- [ ] `/healthz` and `/v1/*` routes unaffected (no routing conflict)
- [ ] Redis unavailable: falls back to Postgres, logs warning

---

### T-05: Click Tracking & Stats
**Deps:** T-04

Buffered channel (1000) + background worker. Flush to Postgres every 5s or at 100 items.
Channel full -> drop + log warning. Stats endpoint `GET /v1/links/:slug/stats`.

**Files created:**
- `internal/repository/click_postgres.go`
- `internal/service/click.go`, `click_test.go`

**Key interfaces:**
```go
// service/click.go (consumer)
type ClickBatchInserter interface {
    BatchInsert(ctx context.Context, clicks []domain.Click) error
}
type ClickStatsQuerier interface {
    CountByLink(ctx context.Context, linkID string) (int64, error)
    CountByDay(ctx context.Context, linkID string, days int) ([]DayStat, error)
    CountByCountry(ctx context.Context, linkID string) ([]CountryStat, error)
}

// Used by redirect handler
type ClickRecorder interface {
    Record(click domain.Click) // non-blocking send to channel
}
```

**Acceptance criteria:**
- [ ] Click appears in DB within flush interval after redirect
- [ ] Redirect response time unaffected by click recording
- [ ] `GET /v1/links/:slug/stats` returns `{total_clicks, by_day, by_country}`
- [ ] Stats for non-owned slug: 403; nonexistent: 404
- [ ] Graceful shutdown drains remaining channel items
- [ ] Channel full: event dropped, warning logged (not blocked)

---

### T-06: JWT Auth Middleware & Refresh Tokens
**Deps:** T-02

RS256 JWT via `golang-jwt/jwt/v5`. Auth middleware extracts user ID into context. Extends
login to return tokens. Adds `POST /v1/auth/refresh` (200) and `POST /v1/auth/logout` (204).
Refresh token rotation: old token deleted on use, replay returns 401.

**Files created:**
- `internal/transport/middleware/auth.go`, `auth_test.go`
- `internal/service/token.go`, `token_test.go`
- `internal/repository/refresh_token_postgres.go`

**Key interfaces:**
```go
// middleware/auth.go (consumer)
type AccessTokenValidator interface {
    ValidateAccessToken(tokenString string) (userID string, err error)
}

// service/token.go (consumer)
type RefreshTokenCreator interface {
    Create(ctx context.Context, userID string, tokenHash string, expiresAt time.Time) error
}
type RefreshTokenByHashFinder interface {
    FindByHash(ctx context.Context, tokenHash string) (*domain.RefreshToken, error)
}
type RefreshTokenDeleter interface {
    DeleteByHash(ctx context.Context, tokenHash string) error
}
```

**Route protection:**
- Required auth: `GET /v1/links`, `DELETE /v1/links/:slug`, `GET /v1/links/:slug/stats`, `POST /v1/auth/logout`
- Optional auth: `POST /v1/links` (guest creation still allowed)
- No auth: `POST /v1/auth/register`, `/login`, `/refresh`

**Acceptance criteria:**
- [ ] Login returns `{access_token, refresh_token, expires_in}`
- [ ] Protected routes: 401 without/expired token
- [ ] Refresh: new token pair issued, old refresh token revoked
- [ ] Reused refresh token: 401 (replay detection)
- [ ] Logout: revokes refresh token, 204
- [ ] `POST /v1/links` works with and without auth
- [ ] `make gen-keys` required before API starts; clear error if keys missing

---

### T-07: Telegram Bot Wiring
**Deps:** T-02, T-03, T-05

Switch from raw types to `go-telegram-bot-api/v5`. Replace stub handlers with real service
calls. Add `telegram_accounts` persistence and account linking flow (BR-07). Wire DB + Redis
in slinkbot `main.go`.

**Files created:**
- `internal/repository/telegram_account_postgres.go`
- `internal/service/telegram.go`

**Files modified:**
- `internal/telegram/handler.go` — inject services, use real bot API
- `internal/telegram/types.go` — may be replaced by bot-api types
- `cmd/slinkbot/main.go` — create DB pool, wire services

**Key interfaces:**
```go
// service/telegram.go (consumer)
type TelegramAccountLinker interface {
    LinkTelegram(ctx context.Context, userID string, telegramID int64, username string) error
}
type TelegramAccountByTelegramIDFinder interface {
    FindByTelegramID(ctx context.Context, telegramID int64) (*domain.TelegramAccount, error)
}
```

**Telegram `/account` flow (BR-07):**
1. `/account` -> bot replies with instructions
2. `/account connect email password` -> verify credentials, create `telegram_accounts` row
3. Success/error response

**Acceptance criteria:**
- [ ] URL message -> returns short link
- [ ] `/list` -> 5 most recent links (requires linked account)
- [ ] `/stats <slug>` -> click count
- [ ] `/account connect email password` -> links Telegram to user
- [ ] Unlinked user gets prompt to link account
- [ ] Wrong credentials -> error message

---

### T-08: Rate Limiting & Security
**Deps:** T-06

Redis-based rate limiter (`INCR`+`EXPIRE`). Security headers middleware. Request body size
limit (1MB).

**Files created:**
- `internal/transport/middleware/ratelimit.go`, `ratelimit_test.go`
- `internal/transport/middleware/security.go`

**Key interface:**
```go
// middleware/ratelimit.go
type RateLimiter interface {
    Allow(ctx context.Context, key string, limit int, window time.Duration) (allowed bool, remaining int, retryAfter time.Duration, err error)
}
```

**Rate limits (BR-05):**
- Guests: 10 `POST /v1/links` per hour per IP
- Auth users: 100 `POST /v1/links` per hour per user ID
- Redirects (`GET /:slug`): NOT rate-limited

**Security headers:**
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `Content-Security-Policy: default-src 'none'`
- `Referrer-Policy: strict-origin-when-cross-origin`

**Acceptance criteria:**
- [ ] 11th guest creation in 1hr: 429 with `Retry-After`
- [ ] 101st auth creation in 1hr: 429 with `Retry-After`
- [ ] Redirects are not rate-limited
- [ ] All responses include security headers
- [ ] Body > 1MB: 413

---

## 3. Dependency Graph

```
T-01 Foundation
 ├── T-02 User/Auth
 │    ├── T-06 JWT + Refresh ─── T-08 Rate Limit + Security
 │    └── T-07 Telegram Wiring (also needs T-03, T-05)
 └── T-03 Link CRUD
      └── T-04 Redirect + Cache
           └── T-05 Click Tracking + Stats
```

**Linear order:** T-01 → T-02 → T-03 → T-04 → T-05 → T-06 → T-07 → T-08

---

## 4. File Structure (after all tasks)

```
cmd/
  slinkapi/main.go                       # modified T-01..T-06, T-08
  slinkbot/main.go                       # modified T-01, T-07
internal/
  config/config.go                       # modified T-01
  domain/
    user.go                              # T-01
    link.go                              # T-01
    click.go                             # T-01
    errors.go                            # T-01
    slug.go                              # T-01
    slug_test.go                         # T-01
  service/
    auth.go, auth_test.go                # T-02, modified T-06
    link.go, link_test.go                # T-03, modified T-04
    click.go, click_test.go              # T-05
    token.go, token_test.go              # T-06
    telegram.go                          # T-07
  repository/
    user_postgres.go                     # T-02
    link_postgres.go                     # T-03
    link_cache.go                        # T-04
    click_postgres.go                    # T-05
    refresh_token_postgres.go            # T-06
    telegram_account_postgres.go         # T-07
  transport/
    auth_handler.go                      # T-02, modified T-06
    link_handler.go                      # T-03
    redirect_handler.go                  # T-04, modified T-05
    response.go                          # T-02
    request.go                           # T-02, modified T-08
    middleware/
      logger.go                          # T-01
      correlation.go                     # T-01
      auth.go, auth_test.go              # T-06
      ratelimit.go, ratelimit_test.go    # T-08
      security.go                        # T-08
  telegram/
    handler.go                           # existing, modified T-01, T-07
    types.go                             # existing, may be replaced T-07
  testutil/
    testutil.go                          # T-02
migrations/
  000001_create_users.{up,down}.sql      # T-01
  000002_create_links.{up,down}.sql      # T-01
  000003_create_clicks.{up,down}.sql     # T-01
  000004_create_refresh_tokens.{up,down}.sql    # T-01
  000005_create_telegram_accounts.{up,down}.sql # T-01
keys/                                    # gitignored, generated by make gen-keys
  private.pem
  public.pem
```

---

## 5. Verification

After each task:
- `go build ./...` compiles
- `go test ./... -short -race` passes
- `golangci-lint run ./...` passes

After all tasks:
- `make env-up && make migrate-up` sets up local DB
- `make gen-keys` generates JWT keys
- `make dev-api` starts the API; `make dev-bot` starts the bot
- Full manual flow: register → login → create link → follow short URL → check stats
- `go test -tags=integration -race ./...` passes with testcontainers
