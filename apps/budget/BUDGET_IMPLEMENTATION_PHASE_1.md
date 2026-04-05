# Budget Management — Implementation Phase 1: Gruppi

**Goal:** Build the Groups view end-to-end with mocked BFF, establishing the navigation shell, baseline CRUD pattern, and WOW effect foundations reusable across all views and future mini-apps.

---

## Prerequisites

- Node 20+, pnpm, Go 1.23+
- Existing monorepo infrastructure: `@mrsmith/auth-client`, `@mrsmith/api-client`, `@mrsmith/ui`

---

## Step 1: Go BFF — Fixture Handlers

### 1.1 Create budget module structure

```
backend/internal/budget/
├── handler.go          # RegisterRoutes(mux) + route definitions
├── fixtures/
│   ├── groups.go       # Group list + details fixture data
│   └── users.go        # User list fixture data
└── routes.go           # Route constants
```

**Pattern:** Follow `internal/portal/handler.go` — register routes via `RegisterRoutes(mux *http.ServeMux)`.

### 1.2 Fixture data (matching API spec exactly)

**Users fixture** (`fixtures/users.go`):
- 8–10 sample users with full `arak-int-user` shape
- All `state.enabled: true`
- Nested `state: { name, enabled }` and `role: { name, created, updated }`
- Wrapped in paginated envelope: `{ total_number, current_page: 1, total_pages: 1, items: [...] }`

**Groups fixture** (`fixtures/groups.go`):
- 4–5 sample groups: "Sviluppo", "Marketing", "Vendite", "Amministrazione", "Supporto"
- List response: `{ name, user_count }` per group, wrapped in paginated envelope
- Details response per group: `{ name, users: arak-int-user[] }` with 2–4 users each

### 1.3 Handlers

| Route | Method | Handler | Response |
|-------|--------|---------|----------|
| `GET /api/users-int/v1/user` | GET | `handleGetAllUsers` | Users fixture (paginated envelope) |
| `GET /api/budget/v1/group` | GET | `handleGetAllGroups` | Groups list fixture |
| `GET /api/budget/v1/group/{name}` | GET | `handleGetGroupDetails` | Group details fixture (lookup by name from URL, `encodeURIComponent`-decoded) |
| `POST /api/budget/v1/group` | POST | `handleNewGroup` | Echo back created group (201) |
| `PUT /api/budget/v1/group/{name}` | PUT | `handleEditGroup` | Echo back updated group (200) |
| `DELETE /api/budget/v1/group/{name}` | DELETE | `handleDeleteGroup` | 204 No Content |

**Notes:**
- Use `net/http` standard mux (project pattern)
- Parse path params via `r.PathValue("name")` (Go 1.22+ routing)
- POST/PUT: decode request body, return plausible response
- Auth middleware applied (use `auth.GetClaims(ctx)` to extract token for future Arak proxy)
- Use `httputil.RespondJSON` for responses

### 1.4 Register routes in main.go

Add `budget.RegisterRoutes(api)` in `cmd/server/main.go` alongside existing portal routes.

---

## Step 2: React App Scaffold

### 2.1 Create `apps/budget/` app structure

```
apps/budget/
├── package.json            # mrsmith-budget, deps: react, react-dom, react-router-dom, @mrsmith/ui, @mrsmith/auth-client, @mrsmith/api-client
├── vite.config.ts          # React plugin, proxy /api → localhost:8080
├── tsconfig.json           # Extends @mrsmith/tsconfig/react.json
├── index.html              # Entry HTML
└── src/
    ├── main.tsx            # React root + AuthProvider + Router
    ├── App.tsx             # Layout shell + routes
    ├── routes.tsx          # Route definitions
    ├── api/                # API client setup + typed query hooks
    │   ├── client.ts       # createApiClient() with auth token
    │   └── types.ts        # TypeScript types matching API spec
    ├── components/         # App-specific components
    ├── views/              # Page components
    │   └── gruppi/         # Phase 1 view
    └── styles/
        └── global.css      # App-level styles, imports @mrsmith/ui theme
```

**Follow `apps/portal/` patterns for:**
- `package.json` structure and scripts (`dev`, `build`, `lint`)
- `vite.config.ts` proxy configuration
- `tsconfig.json` extending shared config

### 2.2 TypeScript types (`src/api/types.ts`)

Define types for Phase 1 entities only (add more in later phases):

```typescript
// Paginated envelope
interface PaginatedResponse<T> {
  total_number: number;
  current_page: number;
  total_pages: number;
  items: T[];
}

// User (reference entity)
interface ArakIntUser {
  id: number;
  first_name: string;
  last_name: string;
  email: string;
  created: string;
  updated: string;
  state: { name: string; enabled: boolean };
  role: { name: string; created: string; updated: string };
}

// Group
interface Group {
  name: string;
  user_count: number;
}

interface GroupDetails {
  name: string;
  users: ArakIntUser[];
}

interface GroupNew {
  name: string;
  user_ids: number[];
}

interface GroupEdit {
  new_name?: string;
  user_ids?: number[];
}
```

### 2.3 API client setup (`src/api/client.ts`)

- Use `createApiClient` from `@mrsmith/api-client`
- Wire `getToken` from `useAuth()` hook
- Add helper to unwrap paginated envelope: `response.items`
- Configure base URL via Vite env or default `/api`

### 2.4 Routing (`src/routes.tsx`)

```
/               → redirect to /home
/home           → Home (placeholder for Phase 4)
/budgets        → Budget list (placeholder for Phase 3)
/budgets/:id    → Budget detail (placeholder for Phase 3)
/cost-centers   → Cost Centers (placeholder for Phase 2)
/groups         → Gruppi (Phase 1 — build now)
```

Placeholder views show a styled empty state: "Coming soon" with the view name.

### 2.5 Add to workspace

- Add `apps/budget` to `pnpm-workspace.yaml` (already covered by `apps/*` glob)
- Add dev script to root `package.json` or Makefile: `make dev-budget`
- Update `docker-compose.dev.yaml` if needed

---

## Step 3: Navigation Shell

### 3.1 Top bar component → `@mrsmith/ui`

Build in `packages/ui/src/components/` for reuse across all mini-apps:

**`AppShell`** — Main layout wrapper:
- Slim top bar (height ~56px)
- Left: MrSmith logo/icon → links to portal (`/`)
- Center: horizontal tab navigation (receives tab config as props)
- Right: user area (name/avatar from auth context)
- Below top bar: optional breadcrumb strip (slot/children)
- Content area fills remaining viewport

**`TabNav`** — Horizontal tab navigation:
- Accepts `items: { label: string, path: string }[]`
- Active tab highlighted based on current route (prefix match for nested routes like `/budgets/:id`)
- Smooth underline indicator animation on tab switch

**`Breadcrumbs`** — Contextual breadcrumb strip:
- Accepts `items: { label: string, path?: string }[]`
- Last item is current (no link)
- Separator character (e.g., `/` or `>`)
- Only renders when items.length > 1

### 3.2 WOW effect foundations → `@mrsmith/ui`

These are built in Phase 1 and reused everywhere:

**`Modal`** — Modal dialog:
- Backdrop with fade-in
- Content with scale+fade entrance animation
- Close on backdrop click, Escape key
- Focus trap
- Header (title + close button), body (scrollable), footer (action buttons)

**`DataTable`** — Table component:
- Column definitions (accessor, header, width, render)
- Row selection (single) with highlight
- Row click handler
- Skeleton loading state (animated placeholder rows)
- Empty state slot
- Smooth row entrance animation on data load

**`DetailPanel`** — Side panel for master-detail:
- Slide-in animation from right
- Header with entity name
- Body (read-only field list)
- Sticky action bar at bottom

**`Button`** — Button with variants:
- Variants: primary, secondary, danger, ghost
- Sizes: sm, md
- Loading state (spinner + disabled)
- Hover/active micro-interactions

**`FormField`** — Form input wrapper:
- Label, input slot, error message, help text
- Validation state (error highlight + message animation)

**`MultiSelect`** — Multi-select dropdown:
- Search/filter within options
- Selected items as chips
- Select all / clear all
- Dropdown with smooth open/close animation

**`Toast`** — Notification system:
- Success, error, info variants
- Slide-in from top-right
- Auto-dismiss with progress bar
- Stack multiple toasts

**`ConfirmDialog`** — Confirmation modal:
- Extends Modal
- Title, message, confirm/cancel buttons
- Danger variant for destructive actions

**`Skeleton`** — Loading placeholder:
- Animated shimmer effect
- Text, rectangle, and circle variants
- Compose into skeleton layouts

**`EmptyState`** — Empty state display:
- Icon/illustration slot
- Title + description
- Optional action button

### 3.3 Theme integration

- Import `@mrsmith/ui` clean theme (`clean.css`) in budget app
- All components use CSS custom properties from theme tokens
- Stripe-inspired palette: white backgrounds, `--color-accent: #635bff`, clean typography

---

## Step 4: Gruppi View

### 4.1 View structure

```
src/views/gruppi/
├── GruppiPage.tsx          # Main page component
├── GroupTable.tsx           # Master table (uses DataTable)
├── GroupDetailPanel.tsx     # Side panel (uses DetailPanel)
├── GroupCreateModal.tsx     # Create form modal
├── GroupEditModal.tsx       # Edit form modal
├── GroupDeleteConfirm.tsx   # Delete confirmation dialog
└── useGroups.ts            # Data fetching hooks
```

### 4.2 Data fetching (`useGroups.ts`)

Custom hooks wrapping API client:

- `useGroups()` — fetches group list, returns `{ groups, isLoading, error, refetch }`
- `useGroupDetails(name)` — fetches group details on name change
- `useUsers()` — fetches enabled users list (shared, will be reused)
- `useCreateGroup()` — POST mutation → invalidates group list on success → toast
- `useEditGroup(name)` — PUT mutation → invalidates group list + details → toast
- `useDeleteGroup(name)` — DELETE mutation → invalidates group list → toast

**Cache invalidation pattern:** After mutation success, refetch affected queries. Use simple state management (React state + useEffect) or a lightweight query library if warranted.

### 4.3 GruppiPage layout

```
┌─────────────────────────────┬──────────────────────────┐
│ Group table (master)        │ Detail panel (side)      │
│                             │                          │
│ [+ Nuovo gruppo]            │ Group name (read-only)   │
│                             │                          │
│ ┌─────────────────────────┐ │ Member list:             │
│ │ Nome        Utenti      │ │ ┌──────────────────────┐ │
│ │ ─────────────────────── │ │ │ name   email         │ │
│ │ ▶Sviluppo   5          │ │ │ Mario  mario@...     │ │
│ │  Marketing  3          │ │ │ Giulia giulia@...    │ │
│ │  Vendite    4          │ │ └──────────────────────┘ │
│ └─────────────────────────┘ │                          │
│                             │ [Modifica] [Elimina]     │
└─────────────────────────────┴──────────────────────────┘
```

### 4.4 Interaction flow

1. **Page load** → skeleton loaders in table and panel → fetch groups + users → animate rows in
2. **Row select** → fetch group details → panel slides in with member list
3. **No selection** → panel shows empty state: "Seleziona un gruppo"
4. **"Nuovo gruppo"** → modal: name input + user multi-select → POST → toast success → list refreshes
5. **"Modifica"** → modal pre-populated: name (optional rename), users (pre-selected current members) → PUT → toast → list + details refresh
6. **"Elimina"** → confirm dialog (danger variant): "Eliminare il gruppo {name}?" → DELETE → toast → list refreshes, panel clears

### 4.5 Error handling

- API errors → toast with error message
- Loading states → skeleton loaders (never spinners)
- Failed mutations → toast error, modal stays open (user can retry)
- Network failure → toast with "Errore di connessione"

### 4.6 Italian labels

All UI text in Italian:
- "Nuovo gruppo", "Modifica", "Elimina", "Conferma", "Annulla"
- "Seleziona un gruppo" (empty panel)
- "Nessun gruppo trovato" (empty table)
- "Gruppo creato", "Gruppo aggiornato", "Gruppo eliminato" (toasts)
- "Nome", "Utenti", "Membri" (table/panel headers)

---

## Step 5: Validation & Polish

### 5.1 Functional testing

- [ ] Group list loads with skeleton → data
- [ ] Row selection shows detail panel with slide animation
- [ ] Create group → appears in list
- [ ] Edit group (rename + change members) → list + detail update
- [ ] Delete group → removed from list, panel clears
- [ ] Error states display correctly (stop Go server, verify toast)
- [ ] Empty states render when no groups exist

### 5.2 WOW effect checklist

- [ ] Smooth page transitions between tabs
- [ ] Table row entrance animation on data load
- [ ] Detail panel slide-in/slide-out
- [ ] Modal fade+scale entrance/exit
- [ ] Button hover/active micro-interactions
- [ ] Skeleton shimmer during loading
- [ ] Toast slide-in with auto-dismiss
- [ ] Confirm dialog with danger styling
- [ ] Multi-select dropdown smooth open/close
- [ ] Typography and spacing feel Stripe-caliber
- [ ] Empty states are designed (not just text)

### 5.3 Reusability check

- [ ] All UI components live in `@mrsmith/ui`, not in `apps/budget/`
- [ ] Components use theme tokens, not hardcoded colors
- [ ] Navigation shell (AppShell, TabNav, Breadcrumbs) works with any tab config
- [ ] DataTable, Modal, DetailPanel are generic (no budget-specific logic)
- [ ] Toast system is app-agnostic

---

## Deliverables

| Deliverable | Location |
|-------------|----------|
| Go BFF handlers (groups + users fixtures) | `backend/internal/budget/` |
| Budget React app scaffold | `apps/budget/` |
| Navigation shell components | `packages/ui/src/components/` |
| WOW effect foundation components | `packages/ui/src/components/` |
| Gruppi view (complete CRUD) | `apps/budget/src/views/gruppi/` |
| TypeScript API types | `apps/budget/src/api/types.ts` |

**Phase 1 is complete when:** The Gruppi view is fully functional with mocked data, all WOW effect foundations are built and reusable, and the navigation shell works with placeholder tabs for the remaining views.
