# T-02: User Registration & Login

**Status:** Done (PR #2)
**Branch:** `feature/t-02-auth`
**Deps:** T-01

---

## Summary

User persistence (pgx), auth service (register + login, bcrypt cost 12),
HTTP handlers for `POST /v1/auth/register` (201) and `POST /v1/auth/login` (200).
JSON envelope helpers. Decode+validate helper. Shared test utilities.

Login returns user data only — JWT issuance is added in T-06.

---

## Files Created

| File | Description |
|------|-------------|
| `internal/repository/user_postgres.go` | `CreateUser`, `FindByEmail` — pgx, maps PG 23505 → ErrAlreadyExists |
| `internal/service/auth.go` | `AuthService` — Register (validate + bcrypt + persist), Login (find + compare) |
| `internal/service/auth_test.go` | Table-driven tests with fakes for both Register and Login |
| `internal/transport/auth_handler.go` | `AuthHandler` — Register (201), Login (200) handlers |
| `internal/transport/response.go` | `respondData`, `respondDataWithMeta`, `respondError`, `mapError` |
| `internal/transport/request.go` | `decodeJSON[T]` (generic, 1MB limit), `getRequestID` |
| `internal/transport/context.go` | `getUserID(ctx)` — placeholder for T-06 auth middleware |
| `internal/testutil/testutil.go` | `NewTestLogger`, `AssertErrorIs`, `AssertErrorAs[T]` |

---

## Interfaces

```go
// service/auth.go — consumer of persistence
type UserCreator interface {
    CreateUser(ctx context.Context, user *domain.User) error
}
type UserByEmailFinder interface {
    FindByEmail(ctx context.Context, email string) (*domain.User, error)
}

// transport/auth_handler.go — consumer of service
type Registerer interface {
    Register(ctx context.Context, email, password string) (*domain.User, error)
}
type Authenticator interface {
    Login(ctx context.Context, email, password string) (*domain.User, error)
}
```

## Business Logic

- **Register:** validate email (net/mail.ParseAddress) + password (≥8 chars) → bcrypt(cost=12) → persist
- **Login:** find by email → bcrypt compare → return user. Same `ErrUnauthorized` for missing user and wrong password (prevent enumeration)

## Response Envelope

```json
// Success
{ "data": { ... }, "meta": { "request_id": "..." } }

// Error
{ "error": { "code": "...", "message": "..." }, "meta": { "request_id": "..." } }
```

## Error Mapping (transport layer)

| Domain Error | HTTP Status | Code |
|---|---|---|
| `ErrAlreadyExists` | 409 | `CONFLICT` |
| `ErrNotFound` | 404 | `NOT_FOUND` |
| `ErrForbidden` | 403 | `FORBIDDEN` |
| `ErrUnauthorized` | 401 | `UNAUTHORIZED` |
| `*ValidationError` | 422 | `VALIDATION_ERROR` |
| default | 500 | `INTERNAL_ERROR` |

## Tests

**TestRegister:** success, duplicate email, invalid email (3 cases), short/empty password
**TestLogin:** success, wrong password, user not found

- Fakes: `fakeUserCreator`, `fakeUserByEmailFinder` (in-memory maps)
- Logger: `zap.NewNop()`
- Assertions: ID non-empty, password hashed, bcrypt round-trip, `errors.Is`/`errors.As`

---

## Acceptance Criteria

- [x] Register: 201 on success, 409 duplicate email, 422 validation errors
- [x] Login: 200 on success, 401 on bad credentials (same message for missing user)
- [x] Password stored as bcrypt hash (cost ≥ 12)
- [x] Unit tests for auth service with mock store
- [x] Integration test for user repository with testcontainers
