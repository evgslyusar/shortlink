# T-06: JWT Auth Middleware & Refresh Tokens

**Status:** Done (PR #10)
**Branch:** `feature/t-06-jwt-refresh`
**Deps:** T-02

---

## Summary

RS256 JWT via `golang-jwt/jwt/v5`. Auth middleware extracts user ID into context.
Extends login to return tokens. Adds `POST /v1/auth/refresh` (200) and `POST /v1/auth/logout` (204).
Refresh token rotation: old token deleted on use, replay returns 401.

---

## Files to Create

| File | Description |
|------|-------------|
| `internal/transport/middleware/auth.go` | JWT validation middleware, extracts user_id into context |
| `internal/transport/middleware/auth_test.go` | Tests for valid/expired/missing token scenarios |
| `internal/service/token.go` | Token service — issue access+refresh, validate, rotate |
| `internal/service/token_test.go` | Tests for token lifecycle |
| `internal/repository/refresh_token_postgres.go` | Create, FindByHash, DeleteByHash |

## Files to Modify

| File | Changes |
|------|---------|
| `internal/transport/auth_handler.go` | Login returns tokens, add Refresh and Logout handlers |
| `internal/transport/context.go` | Auth middleware sets user_id in context |
| `cmd/slinkapi/main.go` | Load RSA keys, wire token service, apply auth middleware to routes |

---

## Interfaces

```go
// middleware/auth.go — consumer
type AccessTokenValidator interface {
    ValidateAccessToken(tokenString string) (userID string, err error)
}

// service/token.go — consumer of persistence
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

## Token Flow

**Login (updated):**
1. Authenticate user (existing logic)
2. Generate RS256 access token (15m TTL, claims: sub=userID, exp, iat)
3. Generate opaque refresh token (crypto/rand, 32 bytes, hex encoded)
4. Store SHA-256 hash of refresh token in DB (7d TTL)
5. Return `{ access_token, refresh_token, expires_in: 900 }`

**Refresh:**
1. Receive refresh token in body
2. SHA-256 hash → look up in DB
3. Not found → 401 (replay detection)
4. Found but expired → delete, return 401
5. Delete old token, create new pair
6. Return new `{ access_token, refresh_token, expires_in }`

**Logout:**
1. Receive refresh token in body
2. SHA-256 hash → delete from DB
3. Return 204

## Auth Middleware

- Extract `Authorization: Bearer <token>` header
- Validate JWT signature (RS256 public key), check `exp` claim
- On success: inject `user_id` into context
- On failure: 401 with `UNAUTHORIZED` error

## Route Protection

| Route | Auth |
|-------|------|
| `POST /v1/auth/register` | None |
| `POST /v1/auth/login` | None |
| `POST /v1/auth/refresh` | None |
| `POST /v1/auth/logout` | Required |
| `POST /v1/links` | Optional (guest ok) |
| `GET /v1/links` | Required |
| `DELETE /v1/links/{slug}` | Required |
| `GET /v1/links/{slug}/stats` | Required |
| `GET /{slug}` | None |

---

## Acceptance Criteria

- [x] Login returns `{access_token, refresh_token, expires_in}`
- [x] Protected routes: 401 without/expired token
- [x] Refresh: new token pair issued, old refresh token revoked
- [x] Reused refresh token: 401 (replay detection)
- [x] Logout: revokes refresh token, 204
- [x] `POST /v1/links` works with and without auth
- [x] `make gen-keys` required before API starts; clear error if keys missing
