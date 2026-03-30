# T-04: Redirect with Redis Cache

**Status:** Done (PR #5)
**Branch:** `feature/t-04-redirect-cache`
**Deps:** T-03

---

## Summary

Redis cache for slug→URL. Redirect handler at `GET /{slug}` (top-level, no `/v1/`).
Flow: check Redis → miss → check Postgres → populate Redis → 302.
Expired links: 404. Redis unavailable: falls back to Postgres.

---

## Files Created

| File | Description |
|------|-------------|
| `internal/repository/link_cache.go` | `LinkCache` — GetOriginalURL, SetOriginalURL, DeleteOriginalURL via Redis |
| `internal/transport/redirect_handler.go` | `RedirectHandler` — slug validation, 302 redirect |

## Files Modified

| File | Changes |
|------|---------|
| `internal/service/link.go` | Added `LinkCache` interface, `ResolveSlug` method, cache invalidation in `DeleteLink` |
| `internal/service/link_test.go` | Added ResolveSlug tests, cache invalidation tests |
| `cmd/slinkapi/main.go` | Wired Redis client, LinkCache, RedirectHandler; registered `GET /{slug}` route |

---

## Interfaces

```go
// service/link.go — consumer of cache
type LinkCache interface {
    GetOriginalURL(ctx context.Context, slug string) (string, error)
    SetOriginalURL(ctx context.Context, slug string, url string, ttl time.Duration) error
    DeleteOriginalURL(ctx context.Context, slug string) error
}

// transport/redirect_handler.go — consumer of service
type SlugResolver interface {
    ResolveSlug(ctx context.Context, slug string) (string, error)
}
```

## Cache Layer (link_cache.go)

- Key format: `slug:{slug}`
- **GetOriginalURL:** `redis.Get` → return URL or `ErrNotFound` on `redis.Nil`
- **SetOriginalURL:** `redis.Set` with TTL (0 = no expiration)
- **DeleteOriginalURL:** `redis.Del`

## ResolveSlug Flow

```
1. cache.GetOriginalURL(slug)
   ├── hit → return URL
   └── miss or error → log warn, continue
2. finder.FindBySlug(slug) from Postgres
   └── not found → return ErrNotFound
3. Compute TTL:
   ├── no ExpiresAt → defaultCacheTTL (24h)
   └── has ExpiresAt → time.Until(ExpiresAt)
       └── ttl ≤ 0 → return ErrNotFound (expired)
4. cache.SetOriginalURL(slug, url, ttl) — best-effort, log warn on error
5. Return original URL
```

## Redirect Handler

- Route: `GET /{slug}` — registered AFTER `/healthz` and `/v1/*` to avoid conflicts
- Validation: slug must be 1–12 chars, otherwise 404
- Success: `http.Redirect(w, r, url, 302)` (StatusFound)
- Not found: 404 JSON error
- Internal error: 500 JSON error

## Cache Invalidation (in DeleteLink)

- After successful DB delete: `cache.DeleteOriginalURL` with detached context (5s timeout)
- Cache error logged as warning, does not fail the request

## Constants

```go
defaultCacheTTL = 24 * time.Hour
slugKeyPrefix   = "slug:"
```

## Route Registration Order

```go
r.Get("/healthz", handleHealthz())
r.Route("/v1/auth", ...)
r.Route("/v1/links", ...)
r.Get("/{slug}", redirectHandler.Redirect)  // LAST — catch-all
```

---

## Acceptance Criteria

- [x] Valid slug: 302 with Location header
- [x] Missing/expired slug: 404
- [x] Second request hits Redis only (Postgres not called again)
- [x] `/healthz` and `/v1/*` routes unaffected (no routing conflict)
- [x] Redis unavailable: falls back to Postgres, logs warning
