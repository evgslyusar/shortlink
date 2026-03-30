# CLAUDE.md — Go Project Constitution

This file defines the rules, principles, and workflows for AI-assisted development on this project.
It is the root CLAUDE.md for **Claude Code**. Subdirectory CLAUDE.md files may extend but never override these rules.
Read this file completely before starting any task.

---

## 🎯 Project Context

> **Fill this section in for each new project.** Claude Code uses it to make informed decisions.

- **What**: A URL shortening service that lets authenticated users create, manage, and track short links via REST API and Telegram bot
- **Tech stack**: Go 1.22+, PostgreSQL 16, Redis 7, optionally Kafka
- **Status**: greenfield
- **Team**: 1 developer working with Claude Code
- **Domain language**: User, ShortLink, Slug, Click, GuestLink, TelegramAccount, Stats

---
 
## 🧭 Priority Hierarchy
 
When rules in this file conflict, resolve in this order:
 
1. **Security** constraints are non-negotiable
2. **Correctness** > Performance > Readability > Brevity
3. **Working code with tests** > Perfect architecture without tests
4. **Simplicity** — the burden of proof is on complexity, never on simplicity
 
---
 
## 🤝 Collaboration Protocol
 
### Autonomy Level
 
Not every action requires confirmation. Use this guide:
 
| Action | Rule |
|---|---|
| Formatting, linting fixes, adding test cases | Proceed without asking |
| Implementing an agreed-upon design | Proceed without asking |
| Creating new files within existing packages | Proceed, mention in summary |
| New packages, new dependencies, interface changes | Propose and wait for approval |
| Deleting code, modifying migrations, changing public APIs | Always ask first |
| Anything in ❌ Forbidden Patterns | Never do it — raise for discussion |
 
### Before Starting Any Non-Trivial Task
 
1. **Ask clarifying questions** about ambiguous requirements before writing code.
   - What is the expected input/output?
   - What are the failure modes and expected behavior on error?
   - Are there performance or scalability constraints?
   - Who are the consumers of this API/function?
 
2. **Design before coding.** Propose and get approval for:
   - API structure (endpoints, request/response shapes)
   - Data flow (how data moves through the system)
   - Failure modes (what happens when dependencies are unavailable)
   - Key data structures and interfaces
 
3. **Multiple approaches rule.** If more than two valid approaches exist:
   - List each approach with pros and cons
   - State which you recommend and why
   - Wait for confirmation before proceeding
 
4. **Testing strategy first.** Before implementation, define:
   - Which layers need unit tests vs integration tests
   - What external dependencies need mocking
   - Acceptance criteria that determine "done"
 
### During Implementation
- Implement one logical unit at a time; confirm before moving to the next
- Surface blockers and assumptions immediately — do not silently work around them
- If you discover the agreed approach has a flaw, stop and re-discuss
 
---
 
## 📁 Project Structure
 
Follow standard Go project layout:
 
```
.
├── cmd/
│   ├── slinkapi/       # HTTP server
│   │   └── main.go
│   └── bot/            # Slink bot in one binary
│       └── main.go
├── internal/           # private application code
│   ├── domain/         # core business logic, no external deps
│   ├── service/        # use cases, orchestration
│   ├── repository/     # data access layer
│   ├── transport/      # HTTP/gRPC handlers
│   │   └── middleware/  # HTTP middleware (auth, logging, recovery, request ID)
│   ├── config/         # configuration loading
│   └── testutil/       # shared test helpers
├── pkg/                # public, reusable packages
├── migrations/         # SQL migrations (never edit applied migrations)
├── api/                # OpenAPI/proto specs
├── web/                # React frontend (see web/CLAUDE.md for frontend rules)
│   ├── CLAUDE.md       # frontend-specific rules for Claude Code
│   ├── src/
│   ├── package.json
│   └── tsconfig.json
├── scripts/            # build, lint, migration scripts
├── .github/workflows/  # CI/CD pipelines
├── docker-compose.yml  # local dev infrastructure (DB, Redis, Kafka, etc.)
├── .env.example        # template — committed to git, no real secrets
├── .env.local          # actual local values — never committed (.gitignore)
├── go.mod
├── go.sum
└── Makefile            # single entry point for all dev commands
```
 
- `internal/` is the default home for all new Go code
- `web/` is the home for all frontend code — see `web/CLAUDE.md`
- `pkg/` only for code genuinely intended for external reuse
- Each `cmd/` entry point should be thin — delegate to `internal/`
- Never put business logic in `main.go`
 
---
 
## 🏗️ Architecture & Design Principles
 
### Dependency Direction
```
transport → service → domain ← repository
```
- `domain` has **zero** external imports — no DB drivers, no HTTP libs, no infrastructure concerns.
- `service` depends on `domain` interfaces only, never on concrete implementations.
- `repository` implements `domain` interfaces; it knows about the DB, nothing else.
- `transport` calls `service` only — never `repository` directly, never `domain` directly.
- Violating this direction requires explicit discussion and approval.
 
### Responsibility
- Each package and type does exactly one thing. If its description requires the word "and" — it should be two packages or types.
- HTTP handlers do not contain business logic. Business logic does not contain SQL. SQL does not contain HTTP concepts.
- If adding a feature requires modifying more than two packages, the boundaries are likely wrong — stop and redesign.
 
### Simplicity
- Choose the simplest solution that correctly solves the problem.
- Complexity requires explicit justification. "We might need it later" is not justification.
- If you are considering a pattern (saga, event sourcing, CQRS, etc.), justify it with a concrete, current requirement — not a forecast. Default to the simplest approach until complexity is proven necessary.
- Prefer flat over nested, explicit over clever, readable over terse.
 
### Duplication vs Abstraction
- Duplication is cheaper than the wrong abstraction.
- Extract shared logic only after it appears in **three or more** places **with the same semantic meaning**.
- Two things that look alike but represent different concepts must stay separate — do not unify them prematurely.
- When in doubt: write it twice, abstract on the third occurrence (WET → DRY).
 
### Explicit over Implicit
- Avoid magic: configuration, dependency wiring, and control flow must be readable without IDE tooling or framework knowledge.
- Prefer `NewService(db, logger, cfg)` over service locators, global registries, or reflection-based injection.
- Side effects must be visible at the call site.
 
### Composition
- Build behaviour by composing small interfaces and structs, not by extending base types.
- Use interface embedding only when the composed interface genuinely represents a superset relationship.
 
### Interface Design
- Define interfaces in the **consumer** package, not in the package that implements them.
- Keep interfaces small — 1 to 3 methods is the target; more than 5 is a warning sign.
- Accept interfaces, return concrete structs (except at package boundaries where the concrete type should stay hidden).
- Do not define an interface until you have at least two implementations or a clear testing need.
 
```go
// Good — small, consumer-defined, single-purpose
type UserFinder interface {
    FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}
 
// Bad — god interface, defined in the provider package
type UserRepository interface {
    FindByID(...)
    FindByEmail(...)
    List(...)
    Create(...)
    Update(...)
    Delete(...)
    // ...
}
```
 
---
 
## 🚨 Error Handling
 
### Strategy
- Errors are values — treat them as first-class citizens, not afterthoughts.
- **Wrap at every layer boundary** with context: `fmt.Errorf("finding user %s: %w", id, err)`
- Error strings: lowercase, no trailing punctuation (`"user not found"`, not `"User not found."`)
- Never swallow errors with `_` unless explicitly justified with a comment.
 
### Sentinel Errors vs Custom Types
 
Use **sentinel errors** (`var ErrNotFound = errors.New(...)`) for well-known conditions that callers check with `errors.Is`:
 
```go
// internal/domain/errors.go
var (
    ErrNotFound      = errors.New("not found")
    ErrAlreadyExists = errors.New("already exists")
    ErrForbidden     = errors.New("forbidden")
)
```
 
Use **custom error types** when the caller needs structured data (e.g., which field failed validation):
 
```go
type ValidationError struct {
    Field   string
    Message string
}
 
func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation: %s — %s", e.Field, e.Message)
}
```
 
### Error Checking Rules
- Use `errors.Is(err, domain.ErrNotFound)` — never compare error strings.
- Use `errors.As(err, &target)` to extract custom error types.
- Check errors at the **service** level for business logic decisions.
- Map domain errors to HTTP status codes **only** in the `transport` layer — never in `service` or `domain`.
 
### Domain Error → HTTP Mapping (transport layer only)
 
```go
func mapError(err error) int {
    switch {
    case errors.Is(err, domain.ErrNotFound):
        return http.StatusNotFound
    case errors.Is(err, domain.ErrAlreadyExists):
        return http.StatusConflict
    case errors.Is(err, domain.ErrForbidden):
        return http.StatusForbidden
    default:
        var ve *domain.ValidationError
        if errors.As(err, &ve) {
            return http.StatusBadRequest
        }
        return http.StatusInternalServerError
    }
}
```
 
### Panic Policy
- `panic()` is forbidden in production code (only allowed in `main()` for unrecoverable startup errors).
- Always add a recovery middleware in the HTTP stack to catch unexpected panics and return 500.
 
---
 
## 🌐 API Design
 
### REST Conventions
- Resource names: plural nouns (`/users`, `/orders`), lowercase, hyphen-separated for multi-word (`/line-items`)
- Use HTTP methods semantically: `GET` = read, `POST` = create, `PUT` = full replace, `PATCH` = partial update, `DELETE` = remove
- Versioning: URL path prefix (`/api/v1/`) for breaking changes; avoid versioning until needed
 
### Standard Response Envelope
 
All JSON responses use a consistent envelope:
 
```go
// Success
{
    "data": { ... },
    "meta": { "request_id": "...", "page": 1, "per_page": 20, "total": 100 }
}
 
// Error
{
    "error": {
        "code": "VALIDATION_ERROR",
        "message": "email is required",
        "details": [ ... ]  // optional
    },
    "meta": { "request_id": "..." }
}
```
 
### Input Validation
- Validate at the **transport** layer boundary — before data reaches the service.
- Use a dedicated validation step, not scattered `if` checks in handlers.
- Return all validation errors at once, not one at a time.
 
### Pagination
- Default: `page` + `per_page` query params (max `per_page` = 100).
- Always return `total` count in `meta` (or use cursor-based pagination for large datasets).
 
### Middleware Chain (recommended order)
1. Recovery (panic → 500)
2. Request ID injection
3. Structured logging (request start/end, duration)
4. Authentication
5. Rate limiting
6. Route handler
 
---
 
## 🔄 Resilience & Retries
 
### Timeouts
- Set explicit timeouts on **all** outbound calls (HTTP clients, DB queries, Redis):
```go
ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()
```
- HTTP server must have `ReadTimeout`, `WriteTimeout`, `IdleTimeout` configured.
- Connection pools: set `MaxOpenConns`, `MaxIdleConns`, `ConnMaxLifetime` on every DB pool.
- Hardcoded timeout values are forbidden — use named constants or config.
 
### Retry Policy
- Retry only on transient errors (network timeouts, 503, DB connection lost) — never on 4xx or validation errors.
- Use exponential backoff with jitter: `baseDelay * 2^attempt + random(0, baseDelay)`.
- Max 3 retries for most operations; configurable via `Config`.
- Always respect `context.Context` cancellation during retries.
 
### Graceful Degradation
- If Redis is unavailable, the service should still function (skip cache, log warning) unless Redis is a hard dependency for the feature.
- If Kafka is unavailable, consider a fallback (write to DB outbox table, log warning).
- Document which dependencies are hard vs soft for each service.
 
---
 
## 🐳 Docker & Local Environment
 
### Overview
All infrastructure (PostgreSQL, Redis, Kafka, and other services) runs via `docker-compose.yml`.
The application itself runs on the host (not in Docker) during development.
Always use `make` commands — never raw `docker compose` calls directly.
 
Default images are recommendations for new projects — adjust per project needs:
- PostgreSQL: `postgres:16-alpine`
- Redis: `redis:7-alpine`
- Kafka (if needed): `confluentinc/cp-kafka:7.6.0`
 
### Canonical Makefile Commands
These are the only commands to interact with the environment:
 
```makefile
make env-up        # start all infrastructure containers
make env-down      # stop all containers (preserve volumes)
make env-reset     # stop, remove volumes, start fresh — full clean slate
make env-status    # show running containers and their health
make env-logs      # tail logs from all containers
make env-logs s=postgres  # tail logs from a specific service
 
make migrate-up    # apply all pending migrations
make migrate-down  # roll back last migration
make migrate-new name=add_users_table  # create a new migration file
make migrate-status # check current migration version
 
make test-unit     # run unit tests only (no Docker required)
make test-int      # ensure env is up, then run integration tests
make test-all      # unit + integration + race detector
make lint          # run golangci-lint
make build         # compile binary
make run           # build and run the application locally
```
 
> If a required make target does not exist, create it — do not run raw docker/go commands.
 
### Environment Variables
- `.env.example` — committed to git; contains all variable names with placeholder values and comments
- `.env.local` — developer's actual values; **never committed**; listed in `.gitignore`
- Application reads from environment; never from files directly
 
Required variables template (`.env.example`):
```bash
# PostgreSQL
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_DB=appdb
POSTGRES_USER=appuser
POSTGRES_PASSWORD=changeme
 
# Redis
REDIS_URL=redis://localhost:6379
 
# Kafka (optional — remove if not used)
KAFKA_BROKERS=localhost:9092
 
# App
HTTP_PORT=8080
LOG_LEVEL=debug
REQUEST_TIMEOUT=30s
```
 
Rules:
- Never hardcode connection strings — always read from environment variables
- If a required env variable is missing, fail fast with a clear error message at startup
- Do not read `.env` files inside the application — load them externally (`make run` handles this)
 
### Service Readiness
Before running any command that requires infrastructure, verify services are healthy:
 
```bash
make env-status   # check before running migrations or tests
```
 
docker-compose.yml must define `healthcheck` for every service.
Never assume a container is ready just because it is running — wait for healthy status.
 
Example healthcheck pattern:
```yaml
services:
  postgres:
    image: postgres:16-alpine
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U $$POSTGRES_USER -d $$POSTGRES_DB"]
      interval: 5s
      timeout: 5s
      retries: 10
      start_period: 10s
 
  redis:
    image: redis:7-alpine
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 10
 
  kafka:
    image: confluentinc/cp-kafka:7.6.0
    healthcheck:
      test: ["CMD-SHELL", "kafka-broker-api-versions --bootstrap-server localhost:9092"]
      interval: 10s
      timeout: 10s
      retries: 15
      start_period: 30s
```
 
### Integration Tests: Two Modes
 
**Mode 1 — Against local docker-compose (fast iteration):**
```bash
make env-up       # infrastructure already running
make migrate-up   # schema is up to date
make test-int     # tests connect to localhost ports
```
Use this during active development. Tests share the same DB across runs — use transactions
and roll back in teardown, or use per-test schemas to avoid state pollution.
 
**Mode 2 — Testcontainers (isolated, CI-safe):**
Tests spin up their own containers, run, and tear down automatically.
Use build tag `//go:build integration` to separate from unit tests.
 
```go
//go:build integration
 
func TestUserRepository(t *testing.T) {
    ctx := context.Background()
 
    pgContainer, err := postgres.RunContainer(ctx,
        testcontainers.WithImage("postgres:16-alpine"),
        postgres.WithDatabase("testdb"),
        postgres.WithUsername("test"),
        postgres.WithPassword("test"),
        testcontainers.WithWaitStrategy(
            wait.ForLog("database system is ready to accept connections").
                WithOccurrence(2).WithStartupTimeout(30*time.Second)),
    )
    require.NoError(t, err)
    t.Cleanup(func() { pgContainer.Terminate(ctx) })
 
    connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
    require.NoError(t, err)
 
    // run migrations against this container
    runMigrations(t, connStr)
 
    // run your test
    repo := repository.NewUserRepository(openDB(t, connStr))
    // ...
}
```
 
Rules for choosing mode:
- Unit tests: never touch Docker — pure Go, no I/O
- Integration tests (local dev): docker-compose mode — fast feedback
- Integration tests (CI): testcontainers mode — isolated, no shared state, no manual setup
- Both modes must use the same migration files from `/migrations`
 
### Migrations
- Migration files live in `/migrations`, named `{timestamp}_{description}.{up|down}.sql`
- Never edit a migration that has already been applied to any environment
- Every `up` migration must have a corresponding `down` migration
- Run `make migrate-up` after creating new migration files
- Migrations run as a separate step before app start — never auto-migrate inside the app
- If `migrate` reports `dirty database`, **stop and report** — do not attempt to fix automatically. The human must decide whether to force the version or fix the failed migration manually.
 
```bash
# Create new migration
make migrate-new name=create_orders_table
 
# Apply
make migrate-up
 
# Roll back one step
make migrate-down
 
# Check current version
make migrate-status
```
 
---
 
## 📦 Modules & Dependencies
 
### Dependency Policy
- **Prefer the standard library.** Before adding a dependency, ask: can `net/http`, `encoding/json`, `database/sql`, `sync`, `context` solve this?
- Introduce a dependency only when it provides **obvious, measurable** benefit.
- Before adding any new package:
  - Check license (MIT/Apache-2.0/BSD preferred; GPL requires explicit approval)
  - Check transitive dependency count (`go mod graph | wc -l`)
  - Check last commit date and open issues
- Never add dependencies for trivial utilities (e.g., `leftpad`-style helpers).
 
### Approved Core Dependencies
```
github.com/jackc/pgx/v5              — PostgreSQL driver (v5.5+)
github.com/redis/go-redis/v9         — Redis client (v9.x)
github.com/go-chi/chi/v5             — HTTP router, stdlib-compatible (v5.x)
github.com/golang-migrate/migrate/v4 — DB migrations (v4.x)
github.com/google/uuid               — UUID generation
go.uber.org/zap                      — structured logging (v1.26+)
github.com/stretchr/testify          — test assertions (v1.9+)
github.com/testcontainers/testcontainers-go — integration test containers (v0.30+)
```
 
> Introducing a dependency not on this list requires discussion and justification.
 
---
 
## ✍️ Code Style & Naming
 
### Formatting
- All code must pass `gofmt` and `goimports` — no exceptions
- Line length: soft limit 100 chars, hard limit 120
- Group imports: stdlib / external / internal (blank line between groups)
 
### Naming Conventions
| Element | Convention | Example |
|---|---|---|
| Packages | lowercase, single word | `repository`, `transport` |
| Interfaces | noun or noun+`er` | `UserStore`, `Reader` |
| Structs | PascalCase noun | `UserService`, `HTTPServer` |
| Constructors | `New` + type name | `NewUserService(...)` |
| Errors (sentinel) | `Err` + description | `ErrUserNotFound` |
| Error types | description + `Error` | `ValidationError` |
| Context arg | always `ctx` | `func (s *Service) Get(ctx context.Context, ...)` |
| Boolean vars | `is`/`has`/`can` prefix | `isValid`, `hasPermission` |
| Unexported vars | camelCase | `maxRetries`, `defaultTimeout` |
 
### Code Rules
- No naked returns
- No `init()` functions except for test setup
- No global mutable state
- Error strings lowercase, no trailing punctuation (`"user not found"`, not `"User not found."`)
- Always wrap errors with context: `fmt.Errorf("finding user %s: %w", id, err)`
- Never swallow errors with `_` unless explicitly justified with a comment
 
---
 
## 🔀 Concurrency
 
- Prefer `sync.WaitGroup`, channels, and `errgroup` over raw goroutines
- Every goroutine must have a clear owner responsible for its lifecycle
- Always pass `context.Context` to goroutines that do I/O or can be cancelled
- Use `context.WithCancel` / `context.WithTimeout` to bound goroutine lifetimes
- Protect shared state with `sync.Mutex`; document which fields a mutex protects
- Prefer `sync.RWMutex` only when reads significantly outnumber writes
- Use `errgroup.WithContext` for parallel fan-out with error propagation:
 
```go
g, ctx := errgroup.WithContext(ctx)
g.Go(func() error { return doA(ctx) })
g.Go(func() error { return doB(ctx) })
if err := g.Wait(); err != nil {
    return err
}
```
 
- Never start goroutines in constructors or `init()`
- Avoid `time.Sleep` in production code — use tickers, timers, or context deadlines
 
---
 
## 🌐 Context
 
- `context.Context` is **always** the first parameter of any function that does I/O, calls external services, or may block
- Never store context in a struct field
- Never pass `context.Background()` inside business logic — only at the entry point (main, handler, test)
- Propagate cancellation: if a handler's context is cancelled, stop downstream work
- Use context values sparingly — only for request-scoped data (request ID, auth token). Never for passing dependencies
- Always check `ctx.Err()` in long loops:
 
```go
for _, item := range items {
    if err := ctx.Err(); err != nil {
        return err
    }
    // process item
}
```
 
---
 
## ✅ Testing Strategy
 
### Pyramid
- **Unit tests**: domain logic, service layer, pure functions — no external I/O, fast (<1ms each)
- **Integration tests**: repository layer against real DB via testcontainers, HTTP handlers via `httptest`
- **E2E tests**: critical user journeys only, run in CI against a full stack
 
### Rules
- Minimum coverage: **80%** for `internal/domain` and `internal/service`
- Tests live in the same package as the code they test (`_test.go` files)
- Use `_test` package suffix for black-box tests of exported APIs
- No `time.Sleep` in tests — use channels, `WaitGroup`, or polling helpers
- Table-driven tests for all functions with multiple input cases:
 
```go
func TestValidateEmail(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantErr bool
    }{
        {"valid email", "user@example.com", false},
        {"missing @", "userexample.com", true},
        {"empty", "", true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateEmail(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("got err=%v, wantErr=%v", err, tt.wantErr)
            }
        })
    }
}
```
 
### Mocking
- Mock via interfaces — never mock concrete types
- Use `testify/mock` or hand-written fakes; avoid heavy mock generation for domain logic
- Integration tests use real dependencies via testcontainers — not mocks
 
### Test Utilities
- Shared test helpers go in `internal/testutil/`
- Use `t.Helper()` in all assertion helpers
- `t.Cleanup()` instead of defer for resource teardown in tests
 
---
 
## 📊 Logging & Monitoring
 
### Logging
- Use **structured logging** with `go.uber.org/zap` — no `fmt.Println`, no `log.Printf` in production code
- Log levels:
  - `Debug` — internal state useful for development only (disabled in prod)
  - `Info` — significant business events (request received, job completed)
  - `Warn` — recoverable anomalies (retry attempt, deprecated API used)
  - `Error` — failures requiring attention; always include `zap.Error(err)`
- Every log entry at service boundaries must include: `request_id`, `user_id` (if applicable), `duration_ms`
- Never log secrets, passwords, PII, or full request bodies
 
```go
logger.Info("user created",
    zap.String("user_id", user.ID.String()),
    zap.String("request_id", requestID),
    zap.Duration("duration", time.Since(start)),
)
```
 
### Metrics & Monitoring
Define monitoring signals **before** implementation, not after. For every new feature identify:
- **RED metrics**: Rate, Errors, Duration (for every HTTP/gRPC endpoint)
- **USE metrics**: Utilization, Saturation, Errors (for every resource: DB pool, cache, queue)
- **Business metrics**: domain-specific counters (orders placed, users registered)
 
Expose metrics via `/metrics` endpoint (Prometheus format).
 
Key alerts to define upfront:
- Error rate > 1% over 5 minutes
- p99 latency > SLA threshold
- DB connection pool saturation > 80%
 
---
 
## ⚡ Performance
 
- Profile before optimizing — never guess; use `pprof`
- Benchmark critical paths with `go test -bench`
- Allocations matter: use `sync.Pool` for hot-path objects; avoid unnecessary heap allocations in tight loops
- Prefer `strings.Builder` over `+` concatenation in loops
- Use `[]byte` where string conversion is unnecessary
- DB queries: always use parameterized queries; avoid N+1 with joins or batch loading; add indexes before shipping
 
---
 
## ⚙️ Configuration
 
- All configuration via environment variables — no hardcoded values in code
- Load and validate config at startup; **fail fast** if required values are missing
- Use a dedicated `internal/config` package with a typed `Config` struct
- Config struct validates itself on load (required fields, value ranges, URL formats)
- Never read `os.Getenv` outside of the `config` package
- Secrets (DB passwords, API keys) via environment variables or a secrets manager — never in source code, never in logs
 
```go
type Config struct {
    HTTPPort    int           `env:"HTTP_PORT,required"`
    DatabaseURL string        `env:"DATABASE_URL,required"`
    LogLevel    string        `env:"LOG_LEVEL" envDefault:"info"`
    Timeout     time.Duration `env:"REQUEST_TIMEOUT" envDefault:"30s"`
}
```
 
---
 
## 🔒 Security
 
- **Never** store secrets in code, config files, or logs
- Sanitize and validate all external input before use
- Use parameterized queries exclusively — no string interpolation in SQL
- Implement rate limiting on all public endpoints
- Set security headers: `Content-Security-Policy`, `X-Frame-Options`, `X-Content-Type-Options`
- Use `crypto/rand` for all random values — never `math/rand`
- Hash passwords with `bcrypt` (cost ≥ 12) or `argon2id` — never MD5/SHA1
- JWT: validate `exp`, `iss`, `aud` claims; use RS256 or ES256 — never HS256 with weak secrets
- TLS: minimum 1.2, prefer 1.3; enforce in production
- Dependencies: run `govulncheck ./...` in CI; block merge on known vulnerabilities
 
---
 
## 🧹 Linters & Static Analysis
 
Run the full linter suite before every commit and in CI. Zero warnings policy.
 
```bash
# Install
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
 
# Run
golangci-lint run ./...
```
 
Required linters (`.golangci.yml`):
```yaml
linters:
  enable:
    - gofmt
    - goimports
    - govet
    - staticcheck
    - errcheck
    - gosec
    - revive
    - exhaustive
    - godot
    - noctx
    - bodyclose
    - sqlclosecheck
    - nilerr
    - unparam
```
 
Additional tools:
```bash
govulncheck ./...    # vulnerability scanning
go test -race ./...  # race condition detection (run in CI)
```
 
---
 
## 🔀 Git Workflow
 
### Commit Messages
Use [Conventional Commits](https://www.conventionalcommits.org/):
 
```
<type>(<scope>): <short description>
 
<optional body>
```
 
Types: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`, `ci`, `perf`
Scope: package or feature name (e.g., `feat(users): add email verification`)
 
Rules:
- One logical change per commit
- Subject line: imperative mood, ≤72 characters, no period at end
- Body: explain **why**, not **what** (the diff shows what)
 
### Branches
- `main` — always deployable; protected; requires passing CI + 1 review
- `feature/*` — feature branches; squash merge into main
- `hotfix/*` — emergency fixes; fast-track review
 
### For Claude Code
- Create a feature branch before starting work: `git checkout -b feature/<name>`
- Commit after each logical unit is complete and tests pass
- Write clear PR descriptions: what changed, why, how to test
- Do not force-push to shared branches
 
---
 
## 🚀 CI/CD
 
Every pull request must pass the full pipeline before merge.
 
```yaml
# .github/workflows/ci.yml — required checks
steps:
  - go build ./...
  - gofmt -l .                          # fail if any files need formatting
  - golangci-lint run ./...
  - govulncheck ./...
  - go test -race -coverprofile=coverage.out ./...              # unit tests
  - go test -race -tags=integration ./... # integration via testcontainers
  - go tool cover -func=coverage.out    # fail if < 80% on domain/service
```
 
CI uses **testcontainers** mode for integration tests — no pre-running infrastructure needed.
Local development uses **docker-compose** mode for faster iteration.
Both modes share the same migration files and test logic — only the container lifecycle differs.
 
### Deployment
- Build a single statically linked binary: `CGO_ENABLED=0 GOOS=linux go build`
- Docker image based on `scratch` or `gcr.io/distroless/static`
- Zero-downtime deploys via graceful shutdown with `signal.NotifyContext`
- Database migrations run as a separate job before deployment, never inside the app
 
```go
// Graceful shutdown
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()
// ... start server ...
<-ctx.Done()
// shutdown with timeout
shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
server.Shutdown(shutdownCtx)
```
 
---
 
## 🖥️ Frontend
 
Frontend lives in `web/` — see `web/CLAUDE.md` for all frontend-specific rules.
 
Key boundaries between backend and frontend:
- Frontend communicates with backend **exclusively** via REST API (`/api/v1/`)
- No shared code between Go and frontend except the OpenAPI spec in `api/`
- API contract changes require updating the OpenAPI spec first, then both sides
- Backend never serves frontend assets in development — use separate dev servers
 
Makefile commands for frontend:
```makefile
make web-install   # install frontend dependencies (npm ci)
make web-dev       # start frontend dev server with API proxy to backend
make web-build     # production build → web/dist/
make web-test      # run frontend tests
make web-lint      # run frontend linter (ESLint)
make dev           # start both backend and frontend dev servers
```
 
---
 
## ❌ Forbidden Patterns
 
These patterns are never acceptable. If you think one is necessary, raise it for discussion first.
 
**Code:**
- `panic()` in production code (only allowed in `main()` for unrecoverable startup errors)
- `os.Exit()` outside of `main()`
- Ignoring errors with `_` without an explanatory comment
- Global mutable variables
- `init()` functions with side effects
- Goroutines without a defined shutdown path
- Direct DB access from `transport` layer
- Business logic in HTTP handlers
- Hardcoded timeouts as magic numbers (use named constants or config)
- `context.TODO()` committed to main (acceptable only as a temporary placeholder in WIP branches)
- `interface{}` / `any` in domain types without strong justification
- Hardcoded connection strings (`localhost:5432`, `localhost:6379`, etc.) anywhere in code
 
**Infrastructure:**
- Running `docker compose` directly — always use `make` commands
- Running `docker compose down -v` — use `make env-reset`
- Modifying `docker-compose.yml` without explicit instruction
- Auto-migrating inside the application on startup
- Assuming a container is ready because it is running — always wait for healthy status
- Connecting to production databases from local environment
 
**Secrets & Config:**
- Committing `.env.local` or any file containing real credentials
- Hardcoding secrets in source code or config files
- Logging secrets, passwords, PII, or full request bodies