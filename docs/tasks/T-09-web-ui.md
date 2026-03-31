# T-09: Web UI

**Status:** Todo
**Branch:** `feature/t-09-web-ui`
**Deps:** T-06, T-08

---

## Summary

React 19 SPA in `web/`. Pages: Home (guest shorten), Login, Register,
Dashboard (link list + stats + delete), 404. API client via React Query,
auth state in Zustand, routing via React Router v7. Backend changes:
CORS middleware, cookie-based auth (httpOnly), CSP header update.

---

## User Stories Covered

| ID | Story |
|----|-------|
| US-14 | Shorten a URL from homepage without registering |
| US-15 | Register and log in via web interface |
| US-16 | Dashboard with list of links and stats |
| US-17 | Create link with optional custom slug and expiry |
| US-18 | Delete a link from dashboard |

---

## Tech Stack (from SPEC NFR-10..NFR-14)

| Concern | Choice |
|---------|--------|
| Framework | React 19 + TypeScript 5 |
| Build | Vite |
| Styling | CSS Modules |
| Server state | React Query (`@tanstack/react-query`) |
| Client state | Zustand (auth store) |
| Routing | React Router v7 (SPA, client-side) |
| Auth tokens | httpOnly cookies (no localStorage) |

---

## Backend Changes

### 1. CORS Middleware

Add CORS middleware to `cmd/slinkapi/main.go` (before auth middleware).
Use `github.com/go-chi/cors` (chi ecosystem, minimal dep).

Configurable via env:
```
CORS_ALLOWED_ORIGINS=http://localhost:5173
```

Settings:
- `AllowedOrigins`: from config (comma-separated)
- `AllowedMethods`: GET, POST, PUT, DELETE, OPTIONS
- `AllowedHeaders`: Content-Type, Authorization, X-Request-ID
- `AllowCredentials`: true (required for httpOnly cookies)
- `MaxAge`: 300

### 2. Cookie-Based Auth

Extend `internal/transport/auth_handler.go`:
- On login/refresh success: set `access_token` and `refresh_token` as httpOnly, Secure, SameSite=Lax cookies
- On logout: clear both cookies (Max-Age=0)
- JSON response body stays the same (API clients still work)

Extend auth middleware:
- Read token from `Authorization` header first (API clients)
- Fallback: read from `access_token` cookie (web UI)

### 3. CSP Header Update

Current CSP `default-src 'none'` will block the SPA.
Make CSP configurable or apply different CSP to API vs web routes:
- API routes: keep strict `default-src 'none'`
- Web/redirect routes: appropriate CSP for React SPA

### 4. Config Changes

Add to `internal/config/config.go`:
```go
CORSAllowedOrigins []string `env:"CORS_ALLOWED_ORIGINS" envSeparator:"," envDefault:"http://localhost:5173"`
```

Add to `.env.example`:
```
CORS_ALLOWED_ORIGINS=http://localhost:5173
```

---

## Files to Create (Frontend)

### Project Setup

| File | Description |
|------|-------------|
| `web/package.json` | Dependencies: react 19, react-dom, react-router, @tanstack/react-query, zustand, typescript, vite |
| `web/tsconfig.json` | Strict mode, path aliases (`@/` -> `src/`) |
| `web/vite.config.ts` | Dev server on :5173, API proxy to :8080, path alias |
| `web/eslint.config.js` | TS + react-hooks + jsx-a11y + import rules |
| `web/.prettierrc` | Consistent formatting |
| `web/.env.example` | `VITE_API_BASE_URL=http://localhost:8080/api/v1` |
| `web/index.html` | Vite entry HTML |

### App Shell

| File | Description |
|------|-------------|
| `web/src/main.tsx` | ReactDOM.createRoot entry point |
| `web/src/app/App.tsx` | Root component with Providers + Router outlet |
| `web/src/app/router.tsx` | Route definitions: `/`, `/login`, `/register`, `/dashboard`, `*` (404) |
| `web/src/app/providers.tsx` | QueryClientProvider + other providers |

### Feature: Auth (`web/src/features/auth/`)

| File | Description |
|------|-------------|
| `components/LoginForm.tsx` | Email + password form, calls useLogin hook |
| `components/RegisterForm.tsx` | Email + password + confirm, calls useRegister hook |
| `hooks/useAuth.ts` | Zustand store: isAuthenticated, user, login/logout actions |
| `hooks/useLogin.ts` | React Query mutation wrapping POST /auth/login |
| `hooks/useRegister.ts` | React Query mutation wrapping POST /auth/register |
| `hooks/useLogout.ts` | React Query mutation wrapping POST /auth/logout |
| `api.ts` | API calls: login, register, logout, refresh |
| `types.ts` | LoginRequest, RegisterRequest, AuthResponse |
| `index.ts` | Public exports |

### Feature: Links (`web/src/features/links/`)

| File | Description |
|------|-------------|
| `components/ShortenForm.tsx` | URL input + optional slug/expiry, used on Home and Dashboard |
| `components/LinkList.tsx` | Table/list of user links with delete button |
| `components/LinkRow.tsx` | Single link row: slug, original URL, clicks, created, delete |
| `components/LinkStats.tsx` | Clicks by day chart/table for a link |
| `hooks/useCreateLink.ts` | React Query mutation wrapping POST /links |
| `hooks/useLinks.ts` | React Query query wrapping GET /links (paginated) |
| `hooks/useDeleteLink.ts` | React Query mutation wrapping DELETE /links/:slug |
| `hooks/useLinkStats.ts` | React Query query wrapping GET /links/:slug/stats |
| `api.ts` | API calls: createLink, listLinks, deleteLink, getLinkStats |
| `types.ts` | Link, CreateLinkRequest, LinkStats, PaginatedLinks |
| `index.ts` | Public exports |

### Shared (`web/src/shared/`)

| File | Description |
|------|-------------|
| `components/Button.tsx` | Reusable button with variants (primary, danger, ghost) |
| `components/Input.tsx` | Labeled input with error state |
| `components/Layout.tsx` | Page layout: header (nav + auth status) + main content |
| `components/ProtectedRoute.tsx` | Redirects to /login if not authenticated |
| `lib/constants.ts` | API_BASE_URL, query key factories |
| `types/api.ts` | Envelope types: ApiResponse<T>, ApiError, Meta |

### API Client (`web/src/api/`)

| File | Description |
|------|-------------|
| `client.ts` | Fetch wrapper with base URL, credentials: 'include', JSON headers, error interceptor (401 -> redirect to /login) |

### Pages

| File | Description |
|------|-------------|
| `web/src/pages/HomePage.tsx` | ShortenForm (guest mode) + result display |
| `web/src/pages/LoginPage.tsx` | LoginForm + link to register |
| `web/src/pages/RegisterPage.tsx` | RegisterForm + link to login |
| `web/src/pages/DashboardPage.tsx` | ShortenForm + LinkList + stats per link |
| `web/src/pages/NotFoundPage.tsx` | 404 page |

## Files to Create (Backend)

| File | Description |
|------|-------------|
| `internal/transport/middleware/cors.go` | CORS middleware wiring (thin wrapper around go-chi/cors) |

## Files to Modify (Backend)

| File | Changes |
|------|---------|
| `cmd/slinkapi/main.go` | Add CORS middleware to chain |
| `internal/config/config.go` | Add `CORSAllowedOrigins` field |
| `internal/transport/auth_handler.go` | Set/clear httpOnly cookies on login/refresh/logout |
| `internal/transport/middleware/auth.go` | Read token from cookie as fallback |
| `internal/transport/middleware/security.go` | Make CSP configurable for API vs web routes |
| `.env.example` | Add CORS_ALLOWED_ORIGINS |
| `Makefile` | Add `web-install`, `web-dev`, `web-build`, `web-test`, `web-lint`, `dev` targets |

---

## Route Map

| Path | Page | Auth Required | Description |
|------|------|---------------|-------------|
| `/` | HomePage | No | Guest URL shortening |
| `/login` | LoginPage | No (redirect to /dashboard if logged in) | Login form |
| `/register` | RegisterPage | No (redirect to /dashboard if logged in) | Register form |
| `/dashboard` | DashboardPage | Yes | Link management + stats |
| `*` | NotFoundPage | No | 404 catch-all |

---

## Auth Flow (Cookie-Based)

1. User submits login form -> `POST /api/v1/auth/login` with `credentials: 'include'`
2. Backend validates, returns JSON + sets httpOnly cookies (`access_token`, `refresh_token`)
3. Subsequent API calls include cookies automatically (`credentials: 'include'`)
4. On 401: API client redirects to `/login`, Zustand store clears auth state
5. Refresh: API client calls `POST /api/v1/auth/refresh` (cookie sent automatically)
6. Logout: `POST /api/v1/auth/logout` -> backend clears cookies (Max-Age=0)

Zustand store tracks `isAuthenticated` + `user` (email, id) for UI purposes only.
Tokens never touch JS — they live in httpOnly cookies.

---

## Key Design Decisions

1. **Fetch over axios**: native `fetch` with thin wrapper — no extra dependency
2. **Vite proxy in dev**: `web/vite.config.ts` proxies `/api` to `localhost:8080` — avoids CORS issues in dev
3. **Pages directory**: top-level `pages/` for route components, `features/` for domain logic — clear separation
4. **No SSR**: pure SPA, React Router client-side only
5. **CSS Modules over Tailwind**: per SPEC NFR-10, keeps bundle small, no utility class debate

---

## Makefile Targets

```makefile
web-install:    cd web && npm ci
web-dev:        cd web && npm run dev
web-build:      cd web && npm run build
web-test:       cd web && npm test
web-lint:       cd web && npm run lint && npx tsc --noEmit
dev:            make dev-api & make web-dev   # both servers
```

---

## New Dependency

| Package | Justification |
|---------|---------------|
| `github.com/go-chi/cors` | Chi-ecosystem CORS middleware, needed for cross-origin cookie auth. Small, well-maintained, stdlib-compatible. |

> Requires approval per CLAUDE.md rules (new dependency not on approved list).

---

## Acceptance Criteria

- [ ] `npm ci && npm run dev` starts Vite dev server on :5173
- [ ] Home: can shorten URL without login (guest), result shows short URL
- [ ] Register: form -> 201 -> redirect to login
- [ ] Login: form -> cookie set -> redirect to /dashboard
- [ ] Dashboard: shows user's links with click counts
- [ ] Dashboard: can create link with optional custom slug and expiry
- [ ] Dashboard: can delete a link
- [ ] Dashboard: can view click stats (by day) for a link
- [ ] 401 from API -> automatic redirect to /login
- [ ] `npx tsc --noEmit` passes with zero errors
- [ ] `eslint` + `prettier` pass with zero warnings
- [ ] CORS: frontend on :5173 can call API on :8080 with cookies
- [ ] httpOnly cookies: tokens not accessible via `document.cookie` in JS
- [ ] All existing backend tests still pass
