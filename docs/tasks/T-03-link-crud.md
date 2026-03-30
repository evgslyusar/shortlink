# T-03: Link CRUD

**Status:** Done (PR #4)
**Branch:** `feature/t-03-link-crud`
**Deps:** T-01, T-02

---

## Summary

Link persistence, link service (create with auto/custom slug, list by user, delete),
HTTP handlers for `POST /v1/links` (201), `GET /v1/links` (200 paginated), `DELETE /v1/links/{slug}` (204).

---

## Files Created

| File | Description |
|------|-------------|
| `internal/repository/link_postgres.go` | `CreateLink`, `FindBySlug`, `ListByUser`, `DeleteBySlugAndUser` |
| `internal/service/link.go` | `LinkService` — CreateLink, ListLinks, DeleteLink |
| `internal/service/link_test.go` | Comprehensive tests with fakes |
| `internal/transport/link_handler.go` | `LinkHandler` — Create (201), List (200), Delete (204) |

---

## Interfaces

```go
// service/link.go — consumer of persistence
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
    DeleteBySlugAndUser(ctx context.Context, slug, userID string) error
}

// transport/link_handler.go — consumer of service
type LinkCreatorSvc interface {
    CreateLink(ctx context.Context, ownerID, rawURL, customSlug string) (*domain.Link, error)
}
type LinkListerSvc interface {
    ListLinks(ctx context.Context, userID string, page, perPage int) ([]domain.Link, int, error)
}
type LinkDeleterSvc interface {
    DeleteLink(ctx context.Context, userID, slug string) error
}
```

## Business Logic

- **CreateLink:**
  - URL validation: `net/url.ParseRequestURI`, scheme must be http/https
  - Custom slug → `ValidateCustomSlug()`, single attempt
  - Auto slug → `GenerateSlug()`, retry up to 3x on collision (`maxSlugRetries = 3`)
  - Guest (ownerID == "") → `ExpiresAt = now + 7d` (`guestLinkExpiry`)
  - Owned → `ExpiresAt = nil`

- **ListLinks:**
  - Pagination: page < 1 → 1, perPage < 1 → 20, perPage > 100 → 100
  - Returns `([]Link, totalCount, error)`

- **DeleteLink:**
  - Atomic: `DELETE WHERE slug = $1 AND user_id = $2`
  - On success: async cache invalidation (detached context, 5s timeout)
  - On ErrNotFound: re-query FindBySlug to distinguish 404 vs 403

## Constants

```go
maxSlugRetries  = 3
guestLinkExpiry = 7 * 24 * time.Hour
```

## HTTP Endpoints

| Method | Path | Auth | Status Codes |
|--------|------|------|-------------|
| POST | `/v1/links` | Optional (guest ok) | 201, 400, 409, 422, 500 |
| GET | `/v1/links` | Required | 200, 401, 500 |
| DELETE | `/v1/links/{slug}` | Required | 204, 401, 403, 404, 500 |

## Request/Response

```json
// POST /v1/links request
{ "url": "https://example.com", "slug": "custom" }

// POST /v1/links response (201)
{ "data": { "slug": "abc123", "short_url": "http://localhost:8080/abc123",
            "original_url": "https://example.com", "expires_at": null } }

// GET /v1/links response (200)
{ "data": { "items": [...] },
  "meta": { "request_id": "...", "page": 1, "per_page": 20, "total": 42 } }
```

## Tests

**TestCreateLink:** guest expiry (~7d), user no expiry, custom slug valid/invalid,
invalid URL, collision retry (3 attempts), all retries exhausted

**TestDeleteLink:** owner deletes, non-owner forbidden, slug not found

**TestResolveSlug:** (added in T-04) cache hit, cache miss + populate, cache error fallback,
not found, expired, TTL computation, graceful degradation

**TestListLinks:** returns items+total, clamps perPage to 100

---

## Acceptance Criteria

- [x] Auto slug: 201 with 6-char slug; custom slug: 201 when valid, 422 when invalid
- [x] Duplicate slug: 409
- [x] Guest link: expires_at set to now+7d; user link: no expiry
- [x] `GET /v1/links`: paginated with page, per_page, total in meta
- [x] Delete by owner: 204; by non-owner: 403
- [x] Unit tests for collision retry and guest expiry logic
