# URL Shortener — Specification

## 1. Overview

A production-ready URL shortening service with user authentication
and a Telegram bot interface. Users can shorten links via REST API,
web UI, or Telegram bot. Anonymous users can follow short links;
authenticated users get click analytics and custom slugs.

---

## 2. User Stories

### Anonymous user
- US-01: I can follow a short link and be redirected immediately
- US-02: I can create a short link without registering (guest mode,
         no analytics, expires in 7 days)

### Registered user
- US-03: I can register with email + password
- US-04: I can log in and receive a JWT access token
- US-05: I can create a short link with an optional custom slug
- US-06: I can set an expiration date on a link
- US-07: I can view click stats for my links (total, by day, by country)
- US-08: I can list and delete my links
- US-09: I can link my Telegram account to my profile

### Telegram bot user
- US-10: I can send a URL to the bot and receive a short link
- US-11: I can use /stats  to see click count
- US-12: I can use /account to link my Telegram to a registered account
- US-13: I can use /list to see my 5 most recent links

---

## 3. Domain Model

### User
| Field        | Type      | Constraints                  |
|--------------|-----------|------------------------------|
| id           | uuid      | PK                           |
| email        | string    | unique, not null             |
| password     | string    | bcrypt hash, not null        |
| telegram_id  | int64     | nullable, unique             |
| created_at   | timestamp | not null                     |

### Link
| Field        | Type      | Constraints                         |
|--------------|-----------|-------------------------------------|
| id           | uuid      | PK                                  |
| slug         | string    | unique, 4-12 chars, [a-zA-Z0-9_-]  |
| original_url | string    | not null, valid URL                 |
| user_id      | uuid      | nullable (guest links)              |
| expires_at   | timestamp | nullable                            |
| created_at   | timestamp | not null                            |

### Click
| Field      | Type      | Constraints     |
|------------|-----------|-----------------|
| id         | uuid      | PK              |
| link_id    | uuid      | FK → Link       |
| clicked_at | timestamp | not null        |
| country    | string    | nullable (2-chr)|
| referer    | string    | nullable        |
| user_agent | string    | nullable        |

---

## 4. API Contract

Base URL: https://api.shortlink.example.com/v1

### Auth
POST   /auth/register    → 201 { user_id, email }
POST   /auth/login       → 200 { access_token, refresh_token, expires_in }
POST   /auth/refresh     → 200 { access_token, expires_in }
POST   /auth/logout      → 204

### Links
POST   /links            → 201 { slug, short_url, expires_at }
GET    /links            → 200 { items: [...], total, page }
DELETE /links/:slug      → 204
GET    /links/:slug/stats→ 200 { total_clicks, by_day: [...], by_country: [...] }

### Redirect (no /v1 prefix, top-level)
GET    /:slug            → 302 Location: original_url
                           404 if not found or expired

### Telegram
POST   /telegram/webhook → 200 (Telegram Bot API webhook endpoint)

---

## 5. Request / Response Shapes

### POST /auth/register
Request:  { "email": "user@example.com", "password": "min8chars" }
Response: { "user_id": "uuid", "email": "user@example.com" }
Errors:   409 email already exists | 422 validation failed

### POST /auth/login
Request:  { "email": "...", "password": "..." }
Response: { "access_token": "jwt", "refresh_token": "opaque",
            "expires_in": 900 }
Errors:   401 invalid credentials

### POST /links
Request:  { "url": "https://...", "slug": "my-slug" (opt),
            "expires_at": "2025-12-31T00:00:00Z" (opt) }
Response: { "slug": "abc123", "short_url": "https://s.example.com/abc123",
            "expires_at": null }
Errors:   409 slug taken | 422 invalid URL | 401 unauthenticated (guest ok)

---

## 6. Business Rules

- BR-01: Slug auto-generation uses 6 random base62 chars
- BR-02: Custom slugs must match [a-zA-Z0-9_-], 4–12 chars
- BR-03: Guest links expire after 7 days; user links never expire by default
- BR-04: Redirect latency target: p99 < 50ms (Redis cache for hot links)
- BR-05: Rate limit: 10 link creations / hour per IP for guests,
          100 / hour for authenticated users
- BR-06: Click tracking is async (write to queue, process in background)
- BR-07: Telegram user must verify with /account connect 

---

## 7. Non-Functional Requirements

- NFR-01: Go 1.23+, Chi router, sqlx + pgx, goose migrations
- NFR-02: PostgreSQL 16, Redis 7
- NFR-03: JWT RS256 (asymmetric), access token TTL 15m, refresh 7d
- NFR-04: All endpoints return JSON with Content-Type: application/json
- NFR-05: Structured logging with slog, correlation-id per request
- NFR-06: Health check: GET /healthz → 200 { status: "ok" }
- NFR-07: Docker + docker-compose for local dev
- NFR-08: Deploy target: Railway (or any VPS with Docker)
- NFR-09: CI: GitHub Actions — lint (golangci-lint) + test on PR

---

## 8. Out of Scope (v1)

- Web UI (API only in v1)
- Link preview / Open Graph
- QR code generation
- Team / organization accounts
- Paid tiers