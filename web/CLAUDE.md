# CLAUDE.md — React Frontend Rules

This file extends the root `CLAUDE.md` with frontend-specific rules.
Root rules (priority hierarchy, collaboration protocol, autonomy levels, security, git workflow) still apply.

---

## 🎯 Frontend Context

> **Fill this section in for each new project.**

- **Framework**: React 19 + TypeScript 5
- **Build tool**: Vite
- **Styling**: CSS Modules
- **State management**: React Query + Zustand
- **Routing**: React Router v7
- **API communication**: REST via OpenAPI spec in `../api/`

---

## 🧭 Frontend Priority Hierarchy

When frontend rules conflict, resolve in this order:

1. **Accessibility** — the app must be usable by everyone
2. **Correctness** — the UI must reflect the actual data state
3. **Performance** — perceived speed matters more than benchmark speed
4. **Readability** — the next developer (or Claude Code) must understand the code
5. **DRY** — only abstract after three occurrences, same as the backend rule

---

## 📁 Project Structure

```
web/
├── src/
│   ├── app/                # app shell: router, providers, global layout
│   │   ├── App.tsx
│   │   ├── router.tsx
│   │   └── providers.tsx
│   ├── features/           # feature modules (the main home for new code)
│   │   └── users/
│   │       ├── components/  # UI components specific to this feature
│   │       ├── hooks/       # feature-specific hooks
│   │       ├── api.ts       # API calls for this feature
│   │       ├── types.ts     # types for this feature
│   │       └── index.ts     # public API of the feature
│   ├── shared/             # reusable across features
│   │   ├── components/      # generic UI components (Button, Modal, Table)
│   │   ├── hooks/           # generic hooks (useDebounce, useMediaQuery)
│   │   ├── lib/             # utilities, helpers, constants
│   │   └── types/           # shared TypeScript types
│   ├── api/                # API client setup, interceptors, base config
│   └── main.tsx            # entry point
├── public/                 # static assets
├── tests/                  # e2e tests (Playwright)
├── package.json
├── tsconfig.json
├── vite.config.ts
├── eslint.config.js
└── CLAUDE.md               # this file
```

Rules:
- New code goes into `src/features/<feature>/` by default
- `shared/` is only for code used by **three or more** features
- Features never import from other features directly — extract to `shared/` first
- Every feature has an `index.ts` that exports its public API — import from the barrel, not from internals
- No business logic in components — extract to hooks or utility functions

---

## 🏗️ Architecture Principles

### Component Design
- **Prefer function components** — no class components for new code
- **Separation of concerns**: split into presentational (UI) and container (logic) components when complexity warrants it
- **Props over context**: pass data explicitly via props; use context only for truly global state (theme, auth, locale)
- **Composition over configuration**: build complex UI by composing simple components, not by adding props flags

### Component Size Rule
If a component exceeds ~150 lines, split it. Signals to split:
- Multiple `useState` / `useEffect` calls → extract a custom hook
- Multiple conditional render blocks → extract sub-components
- Mix of data fetching + rendering → separate container and presentational

### State Management Layers

| Layer | Tool | Example |
|---|---|---|
| Server state (API data) | React Query (`@tanstack/react-query`) | Users list, order details |
| Client state (global) | Zustand (or Context for simple cases) | Auth, theme, sidebar open |
| Local state (component) | `useState` / `useReducer` | Form inputs, modal open/close |

Rules:
- **Server state is not client state.** Never copy API response into Zustand/Redux — let React Query own it.
- Prefer React Query's cache as the single source of truth for server data.
- Zustand stores should be small and focused — one store per concern, not one god-store.

---

## ✍️ Code Style & Naming

### TypeScript
- **Strict mode** — `"strict": true` in `tsconfig.json`, no exceptions
- **No `any`** — use `unknown` + type narrowing if the type is truly unknown
- **No type assertions** (`as`) unless interfacing with an untyped library — always add a comment explaining why
- **Prefer `interface` for object shapes**, `type` for unions, intersections, and primitives
- **No `enum`** — use `as const` objects or union types instead:

```typescript
// Good
const Status = { ACTIVE: 'active', INACTIVE: 'inactive' } as const;
type Status = typeof Status[keyof typeof Status];

// Bad
enum Status { ACTIVE = 'active', INACTIVE = 'inactive' }
```

### Naming Conventions

| Element | Convention | Example |
|---|---|---|
| Components | PascalCase | `UserProfile`, `OrderList` |
| Hooks | camelCase, `use` prefix | `useAuth`, `useUserList` |
| Utilities / helpers | camelCase | `formatDate`, `parseError` |
| Types / Interfaces | PascalCase | `User`, `OrderResponse` |
| Constants | UPPER_SNAKE_CASE | `MAX_RETRIES`, `API_BASE_URL` |
| Files — components | PascalCase | `UserProfile.tsx` |
| Files — hooks | camelCase | `useAuth.ts` |
| Files — utilities | camelCase | `formatDate.ts` |
| Directories | kebab-case | `user-profile/`, `order-list/` |

### Import Order
Group imports with blank lines between groups:
1. React / framework
2. External libraries
3. `shared/` imports (aliased `@/shared/...`)
4. Relative feature imports

---

## 🌐 API Integration

### API Client
- Single API client instance configured in `src/api/client.ts`
- Base URL from environment variable (`VITE_API_BASE_URL`), never hardcoded
- Attach `Authorization` header and `X-Request-ID` via interceptor
- Global error interceptor: handle 401 (redirect to login), 403, 500

### React Query Conventions
- **Query keys**: use a factory pattern per feature:
```typescript
// src/features/users/api.ts
export const userKeys = {
    all: ['users'] as const,
    lists: () => [...userKeys.all, 'list'] as const,
    list: (filters: UserFilters) => [...userKeys.lists(), filters] as const,
    details: () => [...userKeys.all, 'detail'] as const,
    detail: (id: string) => [...userKeys.details(), id] as const,
};
```
- **Stale time**: set sensible defaults per query, not globally (e.g., user profile = 5 min, dashboard metrics = 30 sec)
- **Error handling**: use `onError` callback or error boundaries — never silently swallow errors
- **Mutations**: always invalidate related queries on success

### Data Flow
```
Component → useQuery/useMutation hook → API client → Backend REST API
```
- Components never call `fetch` or the API client directly — always through React Query hooks
- Feature-specific hooks wrap React Query calls: `useUserList()`, `useCreateOrder()`

---

## ✅ Testing Strategy

### Pyramid
- **Unit tests**: hooks, utilities, pure functions — fast, no DOM
- **Component tests**: render components with React Testing Library, mock API via MSW
- **E2E tests**: critical user flows only, via Playwright

### Rules
- Test **behavior**, not implementation — never test internal state or component methods
- Use `screen.getByRole`, `getByLabelText`, `getByText` — avoid `getByTestId` unless no semantic alternative exists
- No `time.Sleep` / hardcoded waits — use `waitFor`, `findBy*` queries
- Mock API at the network level with MSW (Mock Service Worker) — never mock `fetch` directly

```typescript
// Good — tests what the user sees
test('shows error message when login fails', async () => {
    server.use(
        http.post('/api/v1/auth/login', () => HttpResponse.json(
            { error: { code: 'INVALID_CREDENTIALS', message: 'Wrong password' } },
            { status: 401 }
        ))
    );

    render(<LoginForm />);
    await userEvent.type(screen.getByLabelText('Email'), 'user@test.com');
    await userEvent.type(screen.getByLabelText('Password'), 'wrong');
    await userEvent.click(screen.getByRole('button', { name: 'Sign in' }));

    expect(await screen.findByText('Wrong password')).toBeInTheDocument();
});
```

### Test File Location
- Tests live next to the code they test: `UserProfile.tsx` → `UserProfile.test.tsx`
- E2E tests live in `web/tests/`
- Shared test utilities in `web/src/shared/test-utils/`

---

## ♿ Accessibility

- Every interactive element must be keyboard accessible
- Use semantic HTML: `<button>` not `<div onClick>`, `<nav>`, `<main>`, `<article>`
- Every `<img>` must have `alt` text (empty `alt=""` for decorative images)
- Form inputs must have associated `<label>` elements
- Color contrast: minimum WCAG AA (4.5:1 for text, 3:1 for large text)
- Test with keyboard navigation — no mouse-only interactions
- Use `aria-*` attributes only when semantic HTML is insufficient

---

## ⚡ Performance

- **Lazy load** routes and heavy features with `React.lazy` + `Suspense`
- **Memoize** expensive computations with `useMemo` — but don't memoize everything; measure first
- **React.memo** only when profiling shows unnecessary re-renders — premature memo is worse than no memo
- **Image optimization**: use proper formats (WebP), lazy loading, explicit `width`/`height`
- **Bundle analysis**: run `npx vite-bundle-visualizer` before adding large dependencies
- No dependency over 50KB gzipped without explicit justification

---

## 🧹 Linting & Formatting

```bash
# Lint
npx eslint .

# Format
npx prettier --write .

# Type check
npx tsc --noEmit
```

All three must pass before commit. ESLint config should include:
- `eslint:recommended`
- `@typescript-eslint/recommended-type-checked`
- `eslint-plugin-react-hooks` (rules of hooks)
- `eslint-plugin-jsx-a11y` (accessibility)
- `eslint-plugin-import` (import order)

---

## 🔒 Frontend Security

- **Never** store tokens in localStorage — use httpOnly cookies set by the backend
- **Never** trust client-side validation alone — the backend must validate everything
- Sanitize any user-generated content rendered as HTML (use DOMPurify if needed)
- **Never** expose secrets or API keys in frontend code — everything in `VITE_*` env vars is public
- CSP headers are set by the backend — frontend must be compatible (no inline scripts/styles in production)

---

## ⚙️ Environment Variables

Frontend variables use the `VITE_` prefix (Vite exposes only these to the client):

```bash
# web/.env.example
VITE_API_BASE_URL=http://localhost:8080/api/v1
VITE_APP_TITLE=MyApp
```

Rules:
- **Never** put secrets in `VITE_*` variables — they are embedded in the bundle and visible to anyone
- `.env.example` — committed; `.env.local` — never committed
- Access via `import.meta.env.VITE_API_BASE_URL` — never hardcode URLs

---

## ❌ Forbidden Patterns

**Code:**
- `any` type without explicit justification comment
- `enum` — use `as const` objects instead
- `// @ts-ignore` or `// @ts-expect-error` without a comment explaining why
- Direct DOM manipulation (`document.querySelector`, etc.) — use refs
- `useEffect` for derived state — compute during render or use `useMemo`
- Fetching data inside `useEffect` — use React Query
- `index` as `key` in lists where items can be reordered, added, or removed
- Nested ternaries in JSX — extract to variables or sub-components
- CSS-in-JS runtime overhead in render path (e.g., styled-components in hot loops)

**Architecture:**
- Cross-feature imports (feature A importing from feature B's internals)
- API calls outside of React Query hooks
- Business logic inside components — extract to hooks or utilities
- God-components over 200 lines
- Prop drilling through more than 3 levels — use composition or context

**Security:**
- Tokens in localStorage or sessionStorage
- Secrets in `VITE_*` environment variables
- `dangerouslySetInnerHTML` without DOMPurify sanitization
- Disabling ESLint rules for convenience (`eslint-disable` without justification)