# Budget Management — Implementation Phase 1: Gruppi

**Goal:** Build the Groups view end-to-end with mocked BFF, establishing the navigation shell, baseline CRUD pattern, and the query/cache layer that all views will share.

**Principle:** Product-first, not design-system-first. Build only the UI components that Gruppi actually needs. Generic abstractions emerge after real usage, not before.

---

## Prerequisites

- Node 20+, pnpm, Go 1.23+
- Existing monorepo infrastructure: `@mrsmith/auth-client`, `@mrsmith/api-client`, `@mrsmith/ui`

---

## Step 0: Infrastructure Prerequisites

These must be completed **before** any feature work in Phase 1. They unblock error handling, auth, and shared component wiring for all subsequent work.

### 0.1 Upgrade `@mrsmith/api-client` error model

The current client throws a generic `Error` string and discards HTTP status and response body. This prevents the app from showing server error messages in toasts or distinguishing network failures from API errors.

**Add to `packages/api-client/src/client.ts`:**

```typescript
export class ApiError extends Error {
  constructor(
    public status: number,
    public statusText: string,
    public path: string,
    public body?: unknown,
  ) {
    super(`API ${status} ${statusText}: ${path}`);
  }
}
```

**Update the `request` function:**
```typescript
if (!res.ok) {
  let body: unknown;
  try { body = await res.json(); } catch { /* no body */ }
  throw new ApiError(res.status, res.statusText, path, body);
}
```

**Export from `packages/api-client/src/index.ts`:**
```typescript
export { createApiClient, ApiError, type ApiClient, type ApiClientOptions } from './client';
```

This enables:
- Toast messages from server: `(error as ApiError).body?.message`
- Status-aware handling: `(error as ApiError).status === 404` → redirect
- Network vs API distinction: `ApiError` = server responded, generic `Error` = network/parse failure

### 0.2 Auth runtime config — backend-served

`AuthProvider` requires `keycloakUrl`, `realm`, and `clientId`. Instead of baking these into the frontend via `VITE_*` env vars, the Go backend serves them via an unprotected endpoint.

**Backend: add `GET /config` endpoint (no auth)**

Register directly on the root mux alongside health probes — NOT behind the `/api/` auth middleware:

```go
// In cmd/server/main.go, alongside health.Register(mux):
mux.HandleFunc("GET /config", handleConfig)
```

Handler returns the **frontend** Keycloak public client config:

```go
func handleConfig(w http.ResponseWriter, r *http.Request) {
    httputil.JSON(w, http.StatusOK, map[string]string{
        "keycloakUrl": cfg.KeycloakFrontendURL,
        "realm":       cfg.KeycloakFrontendRealm,
        "clientId":    cfg.KeycloakFrontendClientId,
    })
}
```

**Backend config (`config.go`)** — add three new env vars:

```env
# Frontend Keycloak (public client, no secret — served to browser)
KEYCLOAK_FRONTEND_URL=https://keycloak.example.com
KEYCLOAK_FRONTEND_REALM=mrsmith-dev
KEYCLOAK_FRONTEND_CLIENT_ID=mrsmith-budget
```

These are separate from the existing `KEYCLOAK_ISSUER_URL` which is used for backend token validation. The backend holds two Keycloak configs:

| Config | Purpose | Sensitive |
|--------|---------|-----------|
| `KEYCLOAK_ISSUER_URL` | Backend validates user tokens | No (public issuer URL) |
| `KEYCLOAK_FRONTEND_*` | Frontend OAuth2 login | No (public client, no secret) |
| Service credentials (Phase post-4) | BFF → Arak client credentials grant | **Yes** (client secret) |

**Frontend: fetch config at startup**

In `src/main.tsx`, fetch `/config` before rendering:

```typescript
import { AuthProvider } from '@mrsmith/auth-client';

async function bootstrap() {
  const res = await fetch('/config');
  const config = await res.json();

  const root = ReactDOM.createRoot(document.getElementById('root')!);
  root.render(
    <AuthProvider
      keycloakUrl={config.keycloakUrl}
      realm={config.realm}
      clientId={config.clientId}
    >
      <App />
    </AuthProvider>
  );
}

bootstrap();
```

**Why this approach:**
- No `.env` files in frontend apps — single config source in the backend
- No build-time baking — same frontend build works in dev, staging, production
- Not sensitive — same data that would be in the JS bundle with `VITE_*` vars
- Unprotected endpoint — frontend needs it before auth is initialized (same as health probes)

**Dev environment note:** During local development, Keycloak must be reachable at the configured URL. If working fully offline, the Go backend auth middleware can be temporarily bypassed with a dev-mode flag.

### 0.3 Auth architecture note — BFF to Arak

The Go BFF uses **client credentials grant** (service-to-service) to call Arak. The user's browser token is NOT forwarded to Arak. This means:

- The Go backend validates the user's token (via `KEYCLOAK_ISSUER_URL`)
- The Go backend obtains its own service token for Arak (client credentials — client ID + secret)
- The service credentials config is added post-Phase 4 when mock-to-real transition happens

This is documented here for awareness. Phase 1–4 use fixture handlers (no Arak calls), so the service credentials are not needed yet. The `auth.GetClaims(r.Context()).RawToken` is used only for user identity, not for Arak proxying.

### 0.4 `@mrsmith/ui` package wiring

Before adding components, establish the file structure and export pattern:

**Component file structure:**
```
packages/ui/src/
├── components/
│   ├── AppShell/
│   │   ├── AppShell.tsx
│   │   └── AppShell.css       # Colocated CSS (CSS modules or plain)
│   ├── TabNav/
│   │   ├── TabNav.tsx
│   │   └── TabNav.css
│   └── index.ts               # Re-exports all components
├── themes/
│   ├── clean.css
│   └── matrix.css
└── index.ts                    # Package entry: re-exports from components/index.ts + themes
```

**Export pattern in `packages/ui/src/index.ts`:**
```typescript
export { AppShell } from './components/AppShell/AppShell';
export { TabNav } from './components/TabNav/TabNav';
// Add new components here as they are built
```

**CSS strategy:** Colocated CSS files per component. Consumers import the component (which imports its own CSS). Theme tokens from `themes/clean.css` are imported once at the app level.

This must be wired before building AppShell and TabNav so the first consumer (budget app) can import directly without package housekeeping churn.

---

## Step 1: Go BFF — Fixture Handlers

### 1.1 Create budget module structure

```
backend/internal/budget/
├── handler.go          # RegisterRoutes(mux) + handler functions
└── fixtures/
    ├── groups.go       # Group list + details fixture data
    └── users.go        # User list fixture data
```

**Pattern:** Follow `internal/portal/handler.go`:
- `RegisterRoutes(mux *http.ServeMux)` registers all routes
- Use `httputil.JSON(w, status, data)` for JSON responses
- Use `httputil.Error(w, status, message)` for errors
- Read auth claims via `auth.GetClaims(r.Context())`

### 1.2 Fixture data

Fixture data must match `docs/mistra-dist.yaml` response shapes exactly.

**Users fixture** (`fixtures/users.go`):
- 8–10 sample users with full `arak-int-user` shape:
  ```json
  {
    "id": 1, "first_name": "Mario", "last_name": "Rossi",
    "email": "mario.rossi@acme.com",
    "created": "2024-01-15T10:00:00Z", "updated": "2025-03-20T14:30:00Z",
    "state": { "name": "active", "enabled": true },
    "role": { "name": "manager", "created": "2024-01-01T00:00:00Z", "updated": "2024-01-01T00:00:00Z" }
  }
  ```
- All `state.enabled: true`
- Wrapped in paginated envelope: `{ "total_number": 10, "current_page": 1, "total_pages": 1, "items": [...] }`

**Groups fixture** (`fixtures/groups.go`):
- 5 sample groups: "Sviluppo", "Marketing", "Vendite", "Amministrazione", "Supporto"
- List items: `{ "name": "Sviluppo", "user_count": 5 }`
- Details per group: `{ "name": "Sviluppo", "users": [arak-int-user, ...] }` with 2–4 users each

### 1.3 Handlers

Routes register **without** `/api` prefix — `http.StripPrefix("/api", api)` in `main.go` removes it before handlers see the request.

| Route registration | Method | Handler | Status | Response body |
|--------------------|--------|---------|--------|---------------|
| `GET /users-int/v1/user` | GET | `handleGetAllUsers` | 200 | Paginated `arak-int-user[]` envelope |
| `GET /budget/v1/group` | GET | `handleGetAllGroups` | 200 | Paginated `group[]` envelope |
| `GET /budget/v1/group/{group_id}` | GET | `handleGetGroupDetails` | 200 | `group-details` |
| `POST /budget/v1/group` | POST | `handleNewGroup` | 200 | `{ "message": "group created" }` |
| `PUT /budget/v1/group/{group_id}` | PUT | `handleEditGroup` | 200 | `{ "message": "group updated" }` |
| `DELETE /budget/v1/group/{group_id}` | DELETE | `handleDeleteGroup` | 200 | `{ "message": "group deleted" }` |

**Contract notes (from `docs/mistra-dist.yaml`):**
- POST, PUT, DELETE all return `200` with `{ "message": string }` — NOT entity echo, NOT 201, NOT 204
- Path param is named `group_id` but is a **string** (group name, URL-encoded)
- GET list endpoints accept query params: `page_number` (integer, required), `disable_pagination` (boolean, optional)
- GET list handler must read and accept these query params even in fixture mode

**Handler implementation notes:**
- Parse path params: `r.PathValue("group_id")`
- Parse query params: `r.URL.Query().Get("page_number")`, `r.URL.Query().Get("disable_pagination")`
- POST/PUT: `json.NewDecoder(r.Body).Decode(&body)` to validate request shape, but return fixed `message` response
- GET details: lookup by name from fixtures, return 404 if not found

**Query param validation in fixture mode:** Fixture handlers **validate `page_number` as required** — return 400 if missing. This matches the real API contract and ensures the frontend sends it from day one. `disable_pagination` is optional and defaults to false if absent (fixture handlers return all items regardless, but accept the param).

### 1.4 Register in main.go

Add to `cmd/server/main.go`:

```go
import "github.com/sciacco/mrsmith/internal/budget"

// In main(), after portal.RegisterRoutes(api):
budget.RegisterRoutes(api)
```

---

## Step 2: React App Scaffold

### 2.1 Create `apps/budget/` app structure

```
apps/budget/
├── package.json
├── vite.config.ts
├── tsconfig.json
├── index.html
└── src/
    ├── main.tsx
    ├── App.tsx
    ├── routes.tsx
    ├── api/
    │   ├── client.ts       # Hook-based API client provider
    │   ├── queries.ts      # TanStack Query hooks
    │   └── types.ts        # Types (see 2.3)
    ├── views/
    │   ├── gruppi/
    │   ├── home/            # Placeholder
    │   ├── voci-di-costo/   # Placeholder
    │   └── centri-di-costo/ # Placeholder
    └── styles/
        └── global.css
```

**Follow `apps/portal/` patterns exactly:**
- `package.json`: name `mrsmith-budget`, scripts `dev` (vite), `build` (tsc -b && vite build), `lint` (tsc --noEmit)
- `vite.config.ts`: React plugin, proxy `/api` → `http://localhost:8080`
- `tsconfig.json`: extends `@mrsmith/tsconfig/react.json`

### 2.2 Query/cache layer decision: TanStack Query

The feedback correctly identifies that "React state + useEffect or a lightweight query library" is too vague. **Decision: use TanStack Query (React Query).**

Rationale:
- Cross-view cache sharing (users, cost centers, groups) is a core requirement
- Cache invalidation after mutations is needed in every view
- Deduplication of concurrent requests
- Built-in loading/error states
- The budget app has 15+ endpoints with well-defined invalidation rules — this warrants a real query library

Add `@tanstack/react-query` as a dependency of `apps/budget/`.

**`src/api/client.ts`** — Hook-based client creation:

```typescript
// The API client must be created inside a React component/hook context
// because getToken comes from useAuth() which requires AuthProvider.
//
// Pattern: create client in a provider, expose via context or pass to QueryClient.

import { createApiClient } from '@mrsmith/api-client';
import { useAuth } from '@mrsmith/auth-client';
import { useMemo } from 'react';

export function useApiClient() {
  const { token } = useAuth();
  return useMemo(
    () => createApiClient({
      baseUrl: '/api',
      getToken: () => token,
    }),
    [token]
  );
}
```

**`src/api/queries.ts`** — Query hooks using TanStack Query:

```typescript
// All hooks use useApiClient() internally.
// Query keys follow a namespace convention for targeted invalidation.
//
// Key structure: ['budget', entity, ...params]
// Examples:
//   ['budget', 'groups']
//   ['budget', 'group-details', groupName]
//   ['budget', 'users']
```

### 2.3 TypeScript types (`src/api/types.ts`)

Types are hand-written to match `docs/mistra-dist.yaml` schemas. **Drift prevention:** a comment at the top of the file states the source of truth and the schemas each type maps to. If the API spec changes, this file must be updated to match.

```typescript
/**
 * API types for Budget Management.
 * Source of truth: docs/mistra-dist.yaml (Mistra NG Internal API v2.7.14)
 * 
 * Each type references its OpenAPI schema name in a JSDoc comment.
 * If mistra-dist.yaml is updated, sync these types manually.
 */

/** Standard paginated response envelope (all list endpoints) */
export interface PaginatedResponse<T> {
  total_number: number;
  current_page: number;
  total_pages: number;
  items: T[];
}

/** Standard mutation response (POST/PUT/DELETE) — schema: message */
export interface MessageResponse {
  message: string;
}

/** schema: arak-int-user */
export interface ArakIntUser {
  id: number;
  first_name: string;
  last_name: string;
  email: string;
  created: string;
  updated: string;
  state: ArakIntUserState;
  role: ArakIntRole;
}

/** schema: arak-int-user-state */
export interface ArakIntUserState {
  name: string;
  enabled: boolean;
}

/** schema: arak-int-role */
export interface ArakIntRole {
  name: string;
  created: string;
  updated: string;
}

/** schema: group */
export interface Group {
  name: string;
  user_count: number;
}

/** schema: group-details */
export interface GroupDetails {
  name: string;
  users: ArakIntUser[];
}

/** schema: group-new */
export interface GroupNew {
  name: string;
  user_ids: number[];
}

/** schema: group-edit */
export interface GroupEdit {
  new_name?: string;
  user_ids?: number[];
}
```

### 2.4 Routing (`src/routes.tsx`)

```
/               → redirect to /home
/home           → Home (placeholder)
/budgets        → Budget list (placeholder)
/budgets/:id    → Budget detail (placeholder)
/cost-centers   → Cost Centers (placeholder)
/groups         → Gruppi (Phase 1)
```

Placeholder views: styled empty state with view name — "In arrivo" (not "Coming soon").

### 2.5 Workspace integration

**Root `package.json`** — add scripts:
```json
"dev:budget": "pnpm --filter mrsmith-budget dev"
```

**Update `dev` script** to include budget app:
```json
"dev": "concurrently --names backend,portal,budget --prefix-colors blue,green,magenta \"cd backend && air\" \"pnpm --filter mrsmith-portal dev\" \"pnpm --filter mrsmith-budget dev\""
```

**`Makefile`** — add:
```makefile
dev-budget:           ## Solo budget app
	pnpm --filter mrsmith-budget dev
```

Update `.PHONY` to include `dev-budget`.

**`docker-compose.dev.yaml`** — add budget service following portal service pattern.

---

## Step 3: Navigation Shell

Build **only what Gruppi needs** in `@mrsmith/ui`. No speculative components.

### 3.1 Components for `packages/ui/src/components/`

**`AppShell`** — Main layout wrapper:
- Slim top bar (~56px)
- Left: MrSmith icon → links to portal root (`/`)
- Center: horizontal tab navigation (props: tab config)
- Right: user name/area
- Below top bar: optional breadcrumb slot (children)
- Content area fills remaining viewport height

**`TabNav`** — Horizontal tab navigation:
- Props: `items: { label: string, path: string }[]`
- Active tab: prefix match on current route (e.g., `/budgets` matches `/budgets/:id`)
- Animated underline indicator on tab switch

These two are the minimum for the navigation shell. Breadcrumbs are not needed until Phase 3 (budget detail drill-down) — build them then, not now.

### 3.2 Theme integration

- Budget app imports `@mrsmith/ui` clean theme (`clean.css`)
- Components use CSS custom properties from theme tokens
- Stripe palette: `--color-accent: #635bff`, white backgrounds, clean typography

---

## Step 4: Gruppi View

### 4.1 View structure

```
src/views/gruppi/
├── GruppiPage.tsx          # Page layout: table + panel
├── GroupCreateModal.tsx     # Create form
├── GroupEditModal.tsx       # Edit form with rename handling
├── GroupDeleteConfirm.tsx   # Delete confirmation
└── queries.ts              # TanStack Query hooks for groups
```

**No separate `GroupTable.tsx` / `GroupDetailPanel.tsx` wrappers** — build the table and panel directly in `GruppiPage.tsx` first. Extract components only if reuse is needed (Phase 2 will prove this).

### 4.2 Data fetching (`queries.ts`)

TanStack Query hooks:

```typescript
// Query keys
const groupKeys = {
  all: ['budget', 'groups'] as const,
  details: (name: string) => ['budget', 'group-details', name] as const,
};
const userKeys = {
  all: ['budget', 'users'] as const,
};

// Queries
useGroups()        → queryKey: groupKeys.all, queryFn: GET /budget/v1/group?page_number=1&disable_pagination=true
useGroupDetails(name) → queryKey: groupKeys.details(name), queryFn: GET /budget/v1/group/{name}, enabled: !!name
useUsers()         → queryKey: userKeys.all, queryFn: GET /users-int/v1/user?page_number=1&disable_pagination=true&enabled=true

// Mutations (all return MessageResponse, not entities)
useCreateGroup()   → POST /budget/v1/group → onSuccess: invalidate groupKeys.all
useEditGroup()     → PUT /budget/v1/group/{name} → onSuccess: see rename handling below
useDeleteGroup()   → DELETE /budget/v1/group/{name} → onSuccess: invalidate groupKeys.all, clear selection
```

**Pagination params:** All GET list queries include `page_number=1&disable_pagination=true` — matching real API contract from day one.

**Envelope unwrapping:** Query functions call the API client, then return `response.items` (for lists) or the response directly (for details). The unwrapping happens in the queryFn, not in a middleware.

### 4.3 Rename handling (name-keyed entity)

Groups are identified by `name` in URL paths. When a group is renamed via `new_name` in the edit body:

1. The PUT returns `{ "message": "group updated" }` — no new entity data
2. The old `groupKeys.details(oldName)` cache entry is stale
3. The list cache (`groupKeys.all`) is stale

**Post-rename flow:**
1. `onSuccess` of `useEditGroup`:
   - Invalidate `groupKeys.all` (list will re-fetch with new name)
   - Remove old detail cache: `queryClient.removeQueries({ queryKey: groupKeys.details(oldName) })`
   - If `new_name` was provided: update selected state to `new_name`, which triggers `useGroupDetails(newName)` to fetch fresh data
   - If no rename: invalidate `groupKeys.details(oldName)` to re-fetch

2. The UI must track `selectedGroupName` as state. After rename, set it to `new_name`.

### 4.4 Page layout

```
┌─────────────────────────────┬──────────────────────────┐
│ Group table (master)        │ Detail panel (side)      │
│                             │                          │
│ [+ Nuovo gruppo]            │ Nome: Sviluppo           │
│                             │                          │
│ ┌─────────────────────────┐ │ Membri:                  │
│ │ Nome        Utenti      │ │ ┌──────────────────────┐ │
│ │ ─────────────────────── │ │ │ Nome   Email         │ │
│ │ ▶Sviluppo   5          │ │ │ Mario  mario@...     │ │
│ │  Marketing  3          │ │ │ Giulia giulia@...    │ │
│ │  Vendite    4          │ │ └──────────────────────┘ │
│ └─────────────────────────┘ │                          │
│                             │ [Modifica] [Elimina]     │
└─────────────────────────────┴──────────────────────────┘
```

### 4.5 Interaction flow

1. **Page load** → skeleton in table area → `useGroups()` + `useUsers()` fetch → rows animate in
2. **Row select** → set `selectedGroupName` → `useGroupDetails(name)` → panel appears with member list
3. **No selection** → panel shows: "Seleziona un gruppo"
4. **"Nuovo gruppo"** → modal: name (text, required) + users (multi-select) → POST → toast (`response.message`) → list invalidates
5. **"Modifica"** → modal pre-populated: new_name (optional), users (pre-selected: `details.users.map(u => u.id)`) → PUT → rename handling → toast → refresh
6. **"Elimina"** → confirm: "Eliminare il gruppo {name}?" → DELETE → toast → list invalidates, selection clears

### 4.6 WOW effect (Gruppi-scoped)

Build these interactions directly in the Gruppi view components. Extract to `@mrsmith/ui` only if Phase 2 proves reuse:

- **Table:** skeleton loading rows (shimmer), row entrance animation on data load, selected row highlight
- **Detail panel:** slide-in from right on selection, slide-out on deselection
- **Modals:** backdrop fade-in, content scale+fade entrance, close on Escape/backdrop
- **Buttons:** primary/secondary/danger variants, hover/active states, loading spinner on mutation
- **Toasts:** success/error slide-in from top-right, auto-dismiss
- **Multi-select:** dropdown open/close animation, chip display for selected users, search filter
- **Empty states:** styled "Seleziona un gruppo" (panel), "Nessun gruppo trovato" (table)

### 4.7 Error handling

Using `ApiError` from Step 0.1:
- API errors (`error instanceof ApiError`) → toast with server message: `(error.body as any)?.message ?? error.statusText`
- Network/parse errors (plain `Error`) → toast "Errore di connessione"
- Mutation errors → toast error, modal stays open for retry
- Loading → skeleton (never spinners for page content; spinners only for button loading state)
- TanStack Query `onError` callbacks on mutations; `useQuery` errors handled via query state

### 4.8 Italian labels

- "Nuovo gruppo", "Modifica", "Elimina", "Conferma", "Annulla"
- "Seleziona un gruppo" (empty panel)
- "Nessun gruppo trovato" (empty table)
- Toast messages: use `response.message` from API (server-provided)
- "Nome", "Utenti", "Membri" (headers)
- "In arrivo" (placeholder views)

---

## Step 5: Validation & Polish

### 5.1 Functional testing

- [ ] Group list loads with skeleton → data (verify `page_number` and `disable_pagination` params sent)
- [ ] Row selection shows detail panel with slide animation
- [ ] Create group → list refreshes (new group appears)
- [ ] Edit group (rename) → list refreshes with new name, detail panel updates, selection tracks new name
- [ ] Edit group (change members, no rename) → detail panel refreshes
- [ ] Delete group → list refreshes, panel clears
- [ ] Error states: stop Go server → toast "Errore di connessione"
- [ ] Empty states render when no groups

### 5.2 WOW effect checklist

- [ ] Tab switch highlights correct tab
- [ ] Table skeleton → data transition is smooth
- [ ] Detail panel slide-in/slide-out
- [ ] Modal entrance/exit animations
- [ ] Button hover/active states
- [ ] Toast slide-in with auto-dismiss
- [ ] Confirm dialog danger styling
- [ ] Multi-select dropdown animation
- [ ] Typography and spacing feel Stripe-caliber
- [ ] Empty states are designed (not just text)

### 5.3 Contract verification

- [ ] All GET list requests include `page_number=1&disable_pagination=true`
- [ ] All GET requests include `Authorization: Bearer {token}` header
- [ ] POST/PUT/DELETE responses are `{ "message": string }` — UI does not depend on echo
- [ ] Path param uses `group_id` (URL-encoded group name)
- [ ] Users list request includes `enabled=true`

### 5.4 Cache behavior verification

- [ ] Navigate away from Gruppi and back → users list not re-fetched (cached)
- [ ] Create/edit/delete → correct queries invalidated
- [ ] Rename → old detail cache removed, new name fetched

---

## Deliverables

| Deliverable | Location |
|-------------|----------|
| Go BFF handlers (groups + users fixtures) | `backend/internal/budget/handler.go`, `fixtures/` |
| Budget module registered in main.go | `backend/cmd/server/main.go` |
| Budget React app scaffold | `apps/budget/` |
| TanStack Query setup + hooks | `apps/budget/src/api/` |
| Navigation shell (AppShell, TabNav) | `packages/ui/src/components/` |
| Gruppi view (complete CRUD + rename handling) | `apps/budget/src/views/gruppi/` |
| TypeScript API types (with schema refs) | `apps/budget/src/api/types.ts` |
| Workspace integration (scripts, Makefile) | Root `package.json`, `Makefile` |

**Phase 1 is complete when:** The Gruppi view is fully functional with mocked data, TanStack Query cache sharing works, the rename flow handles name-keyed entity identity correctly, and the navigation shell works with placeholder tabs for remaining views.

---

## Changes from original plan (feedback incorporation)

| Issue | Original | Revised |
|-------|----------|---------|
| Response shapes | POST→201 echo, DELETE→204 | All mutations return `200` with `{ "message": string }` per spec |
| Route registration | `/api/budget/v1/...` | `/budget/v1/...` (StripPrefix removes `/api`) |
| Helper functions | `httputil.RespondJSON`, `auth.GetClaims(ctx)` | `httputil.JSON`, `auth.GetClaims(r.Context())` |
| Query params | Not mentioned | `page_number` (required) + `disable_pagination` on all list endpoints |
| Path param name | `{name}` | `{group_id}` (string, per spec) |
| Data layer | "React state + useEffect or lightweight library" | TanStack Query — explicit decision |
| API client wiring | "Wire getToken from useAuth()" | `useApiClient()` hook creates client inside React context |
| TypeScript types | Hand-written, no drift mention | Hand-written with schema refs + drift prevention comment |
| UI component scope | 13 components upfront | Only AppShell + TabNav. WOW effects built inline, extracted later |
| Rename flow | Not addressed | Explicit cache removal + selection tracking |
| Breadcrumbs | Built in Phase 1 | Deferred to Phase 3 (when budget detail drill-down needs them) |
| Workspace scripts | "or" between package.json/Makefile | Both: `dev:budget` in package.json, `dev-budget` in Makefile |
| API client errors | "toast with error message" but client throws generic Error | Step 0.1: `ApiError` class with status, body. Toasts extract server message. |
| Auth bootstrap | Assumed wirable | Step 0.2: Vite env vars for Keycloak URL/realm/clientId, wired in main.tsx |
| `@mrsmith/ui` wiring | Not specified | Step 0.3: component file structure, colocated CSS, explicit re-exports from index.ts |
| Query param validation | "read and accept" | Fixture handlers validate `page_number` as required (400 if missing) |
