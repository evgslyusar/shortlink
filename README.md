# Shortlink

A production-ready URL shortening service with REST API, Telegram bot (@SlinkBot), and web UI.
Users can shorten links via any interface. Anonymous users get temporary short links;
authenticated users get click analytics, custom slugs, and link management.

## Features

- **URL Shortening** — auto-generated 6-char base62 slugs or custom slugs (4-12 chars)
- **Guest Mode** — shorten URLs without registration (links expire in 7 days)
- **Click Analytics** — total clicks, breakdown by day and country
- **Telegram Bot** — shorten links, view stats, and manage account via @SlinkBot
- **Web Dashboard** — React SPA for link management and analytics
- **Redis Cache** — sub-50ms redirect latency for hot links
- **Async Click Tracking** — buffered channel + background worker, never blocks redirects
- **JWT Auth (RS256)** — access/refresh token rotation with replay detection
- **Rate Limiting** — Redis-based, 10/hr guests, 100/hr authenticated users

## Tech Stack

| Layer       | Technology                                            |
|-------------|-------------------------------------------------------|
| Language    | Go 1.24                                               |
| Router      | chi/v5                                                |
| Database    | PostgreSQL 16 (pgx/v5)                                |
| Cache       | Redis 7 (go-redis/v9)                                 |
| Migrations  | golang-migrate/v4                                     |
| Auth        | golang-jwt/v5 (RS256)                                 |
| Logging     | go.uber.org/zap                                       |
| Bot         | go-telegram-bot-api/v5                                |
| Frontend    | React 19, TypeScript 5, Vite, React Query, Zustand    |
| Config      | caarlos0/env/v11                                      |

## Project Structure

```
cmd/
  slinkapi/          HTTP API server
  slinkbot/          Telegram bot
internal/
  config/            Configuration loading and validation
  domain/            Core business types (zero external imports)
  service/           Use cases and orchestration
  repository/        Data access (Postgres, Redis)
  transport/         HTTP handlers and middleware
  telegram/          Telegram bot handlers and service
  testutil/          Shared test helpers
web/                 React SPA (see web/CLAUDE.md)
migrations/          SQL migration files
api/                 OpenAPI spec
```

## Quick Start

### Prerequisites

- Go 1.24+
- Docker & Docker Compose
- Node.js 20+ (for web UI)
- [golang-migrate](https://github.com/golang-migrate/migrate) CLI

### Setup

```bash
# Clone and enter the project
git clone https://github.com/evgslyusar/shortlink.git
cd shortlink

# Copy env template
cp .env.example .env.local

# Start infrastructure (Postgres, Redis)
make env-up

# Run database migrations
make migrate-up

# Generate JWT signing keys
make gen-keys

# Start the API server
make dev-api
```

The API is available at `http://localhost:8080`.

### Telegram Bot

```bash
# Set TELEGRAM_TOKEN in .env.local, then:
make dev-bot
```

### Web UI

```bash
make web-install   # install dependencies
make web-dev       # start dev server with API proxy
```

Or start both backend and frontend together:

```bash
make dev
```

## API Overview

| Method | Endpoint                 | Auth     | Description             |
|--------|--------------------------|----------|-------------------------|
| POST   | /v1/auth/register        | No       | Register a new user     |
| POST   | /v1/auth/login           | No       | Log in, get tokens      |
| POST   | /v1/auth/refresh         | No       | Refresh access token    |
| POST   | /v1/auth/logout          | Yes      | Revoke refresh token    |
| POST   | /v1/links                | Optional | Create a short link     |
| GET    | /v1/links                | Yes      | List your links         |
| DELETE | /v1/links/{slug}         | Yes      | Delete a link           |
| GET    | /v1/links/{slug}/stats   | Yes      | Click analytics         |
| GET    | /{slug}                  | No       | Redirect (302)          |
| GET    | /healthz                 | No       | Health check            |

## Make Commands

```bash
# Infrastructure
make env-up           # Start Postgres + Redis
make env-down         # Stop containers
make env-reset        # Stop, wipe volumes, restart
make env-status       # Show container health

# Backend
make build            # Build slinkapi + slinkbot binaries
make dev-api          # Run API server (starts infra first)
make dev-bot          # Run Telegram bot
make gen-keys         # Generate RSA keypair for JWT

# Database
make migrate-up       # Apply pending migrations
make migrate-down     # Roll back one migration
make migrate-status   # Show current version
make migrate-new name=...  # Create new migration files

# Testing & Quality
make test-unit        # Unit tests with race detector
make test-int         # Integration tests (requires infra)
make test-all         # All tests
make lint             # golangci-lint

# Frontend
make web-install      # npm ci
make web-dev          # Vite dev server
make web-build        # Production build
make web-test         # Frontend tests
make web-lint         # ESLint + TypeScript check
make dev              # Backend + frontend together
```

## License

Private — all rights reserved.
