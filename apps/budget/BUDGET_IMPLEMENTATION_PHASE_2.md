# Budget Management — Implementation Phase 2: Centri di costo

**Goal:** Build the Cost Centers view with complex forms (multi-select users + groups, manager dropdown), disable/enable flow with impact preview, and validate cross-view cache sharing.

**Depends on:** Phase 1 complete — navigation shell, TanStack Query setup, Gruppi view working, shared query hooks established.

**Hard prerequisites from Phase 1:**
- `ApiError` class in `@mrsmith/api-client` (Phase 1, Step 0.1) — required for structured error toasts
- `@mrsmith/ui` package wiring established (Phase 1, Step 0.4) — required for component extraction in this phase
- Fixture handlers validate `page_number` as required (Phase 1, Step 1.3) — same rule applies to all new handlers in this phase

---

## Step 1: Shared Data Layer — Promote to App Level

### 1.1 Shared reference data hooks

Phase 1 created `useUsers()` and `useGroups()` hooks in Gruppi's `queries.ts`. Phase 2 adds a third (`useCostCenters()`). These must now be **promoted to app-level shared hooks** before building the view.

Move to `src/api/shared-queries.ts`:

```typescript
// Shared reference data hooks — used across multiple views.
// These use TanStack Query with stable keys so cross-view cache sharing works automatically.

const sharedKeys = {
  users: ['budget', 'users'] as const,
  groups: ['budget', 'groups'] as const,
  costCenters: ['budget', 'cost-centers'] as const,
};

useUsers()        → queryKey: sharedKeys.users
                  → GET /users-int/v1/user?page_number=1&disable_pagination=true&enabled=true
                  → staleTime: 5 min (reference data, rarely changes)

useGroups()       → queryKey: sharedKeys.groups
                  → GET /budget/v1/group?page_number=1&disable_pagination=true
                  → staleTime: 5 min

useCostCenters()  → queryKey: sharedKeys.costCenters
                  → GET /budget/v1/cost-center?page_number=1&disable_pagination=true
                  → staleTime: 5 min
```

**Why now, not later:** This is an architectural dependency, not a validation step. Centri di costo reuses `useUsers()` and `useGroups()` from Gruppi. If these remain view-local, Phase 2 either duplicates them or forces a mid-feature refactor. Promote once, use everywhere.

Gruppi's `queries.ts` should import from `shared-queries.ts` after this refactor. Group-specific queries (details, mutations) stay in `views/gruppi/queries.ts`.

### 1.2 Invalidation from mutations

When Gruppi mutations invalidate the groups list, all views consuming `useGroups()` (including Centri di costo dropdowns) automatically re-fetch. This is a TanStack Query guarantee — no extra wiring needed, but it must be verified.

---

## Step 2: Go BFF — Cost Center Fixture Handlers

### 2.1 Add cost center fixtures

```
backend/internal/budget/fixtures/
├── groups.go           # (Phase 1)
├── users.go            # (Phase 1)
└── cost_centers.go     # NEW
```

**Cost center list fixture:**
- 5 sample cost centers: "Ricerca e Sviluppo", "Marketing", "Vendite", "Risorse Umane", "Amministrazione"
- Shape per spec `cost-center`: `{ "name": string, "manager_email": string, "user_count": integer, "enabled": boolean }`
- **Include one disabled CC** ("Amministrazione" with `enabled: false`) for testing disable/enable flows
- Wrapped in paginated envelope

**Cost center details fixture per name:**
- Shape per spec `cost-center-details`: `{ "name": string, "manager": arak-int-user, "users": arak-int-user[], "groups": group-details[], "enabled": boolean }`
- Reference users and groups from existing Phase 1 fixtures

### 2.2 New handlers

Routes register **without** `/api` prefix (stripped by `StripPrefix`).

| Route registration | Method | Handler | Status | Response body |
|--------------------|--------|---------|--------|---------------|
| `GET /budget/v1/cost-center` | GET | `handleGetAllCostCenters` | 200 | Paginated `cost-center[]` envelope |
| `GET /budget/v1/cost-center/{cost_center_id}` | GET | `handleGetCostCenterDetails` | 200 | `cost-center-details` |
| `POST /budget/v1/cost-center` | POST | `handleNewCostCenter` | 200 | `{ "message": "cost center created" }` |
| `PUT /budget/v1/cost-center/{cost_center_id}` | PUT | `handleEditCostCenter` | 200 | `{ "message": "cost center updated" }` |

**Contract notes (from `docs/mistra-dist.yaml`):**
- All mutations return `200` with `{ "message": string }` — not entity echo
- Path param is `cost_center_id` (string — cost center name, URL-encoded)
- GET list accepts `page_number` (required) + `disable_pagination` (optional)
- Disable and Enable both use the PUT handler (body `{ "enabled": false }` or `{ "enabled": true }`)
- No separate DELETE endpoint for cost centers — soft-disable only
- Fixture handlers validate `page_number` as required on GET list — return 400 if missing (same rule as Phase 1)

### 2.3 Reuse existing handlers

- `GET /users-int/v1/user` — Phase 1
- `GET /budget/v1/group` — Phase 1

---

## Step 3: TypeScript Types

Add to `src/api/types.ts` with schema references:

```typescript
/** schema: cost-center */
export interface CostCenter {
  name: string;
  manager_email: string;
  user_count: number;
  enabled: boolean;
}

/** schema: cost-center-details */
export interface CostCenterDetails {
  name: string;
  manager: ArakIntUser;
  users: ArakIntUser[];
  groups: GroupDetails[];
  enabled: boolean;
}

/** schema: cost-center-new */
export interface CostCenterNew {
  name: string;
  manager_id: number;
  user_ids: number[];
  group_names: string[];
  enabled: boolean;
}

/** schema: cost-center-edit */
export interface CostCenterEdit {
  new_name?: string;
  manager_id?: number;
  user_ids?: number[];
  group_names?: string[];
  enabled?: boolean;
}
```

---

## Step 4: Centri di costo View

### 4.1 View structure

```
src/views/centri-di-costo/
├── CentriDiCostoPage.tsx       # Page layout: table + panel
├── CostCenterCreateModal.tsx    # Create form
├── CostCenterEditModal.tsx      # Edit form with rename handling
├── CostCenterDisableConfirm.tsx # Disable confirmation with impact preview
└── queries.ts                   # CC-specific query hooks (details + mutations)
```

**Same Phase 1 principle:** Build table and panel directly in `CentriDiCostoPage.tsx`. If Phase 1 produced reusable table/panel components from Gruppi, use them. If not, build inline and extract after Phase 2 proves the pattern.

### 4.2 Data fetching (`queries.ts`)

```typescript
const costCenterKeys = {
  details: (name: string) => ['budget', 'cost-center-details', name] as const,
};

// Queries
useCostCenterDetails(name) → queryKey: costCenterKeys.details(name)
                            → GET /budget/v1/cost-center/{name}?  (URL-encoded)
                            → enabled: !!name

// Mutations — all return MessageResponse, trigger refetch-based flows
useCreateCostCenter()   → POST /budget/v1/cost-center
                        → onSuccess: invalidate sharedKeys.costCenters

useEditCostCenter()     → PUT /budget/v1/cost-center/{name}
                        → onSuccess: see rename handling (4.3)

useDisableCostCenter()  → PUT /budget/v1/cost-center/{name} with { enabled: false }
                        → onSuccess: invalidate sharedKeys.costCenters + costCenterKeys.details(name)

useEnableCostCenter()   → PUT /budget/v1/cost-center/{name} with { enabled: true }
                        → onSuccess: invalidate sharedKeys.costCenters + costCenterKeys.details(name)
```

**All mutations are refetch-driven, not response-driven.** The API returns `{ "message": string }`, so the UI must invalidate and re-fetch to get updated data. Toast messages use `response.message`.

### 4.3 Rename handling (name-keyed entity)

Same pattern as Gruppi (Phase 1), adapted for cost centers:

1. `onSuccess` of `useEditCostCenter`:
   - Invalidate `sharedKeys.costCenters` (list re-fetches with new name)
   - If `new_name` was provided in the request body:
     - Remove old detail cache: `queryClient.removeQueries({ queryKey: costCenterKeys.details(oldName) })`
     - Update selection state to `new_name` → triggers `useCostCenterDetails(newName)` to fetch fresh data
   - If no rename:
     - Invalidate `costCenterKeys.details(name)` to re-fetch current data

2. Track `selectedCostCenterName` as state. After rename, set to `new_name`.

### 4.4 Disabled cost center edit rules

**Business rule decision (explicit):** In the Appsmith app, Edit was disabled for disabled cost centers. **The new app changes this behavior:** Edit is available for disabled cost centers.

**Rationale:** A manager may need to edit membership or rename a disabled cost center before re-enabling it. The API does not reject edits on disabled entities. The re-enable flow (Phase A, Q4) implies the ability to modify before re-enabling.

**Action button matrix:**

| CC state | Modifica | Disabilita | Abilita |
|----------|----------|------------|---------|
| Enabled  | Shown    | Shown      | Hidden  |
| Disabled | Shown    | Hidden     | Shown   |

This is a deliberate change from Appsmith. If the business later disagrees, the only change is hiding the Edit button when `enabled === false`.

### 4.5 Page layout

```
┌────────────────────────────────┬───────────────────────────┐
│ Cost center table (master)     │ Detail panel (side)       │
│                                │                           │
│ [+ Nuovo centro di costo]     │ Nome: Ricerca e Sviluppo  │
│                                │ Manager: mario@acme.com   │
│ ┌────────────────────────────┐ │ Stato: Attivo ●           │
│ │ Nome      Att. Manager  N. │ │                           │
│ │ ──────────────────────────│ │ Membri:                   │
│ │ ▶R&S      ✓   mario@  5  │ │ ┌───────────────────────┐ │
│ │  Mktg     ✓   anna@   3  │ │ │ Nome    Email         │ │
│ │  Vendite  ✓   luca@   4  │ │ │ Mario   mario@...     │ │
│ │  Amm.     ✗   —       0  │ │ │ Giulia  giulia@...    │ │
│ └────────────────────────────┘ │ └───────────────────────┘ │
│                                │                           │
│                                │ [Modifica] [Disabilita]   │
│                                │      or [Abilita]         │
└────────────────────────────────┴───────────────────────────┘
```

### 4.6 Interaction flow

1. **Page load** → skeleton → `useCostCenters()` (shared) + `useUsers()` (shared) + `useGroups()` (shared) → animate in
2. **Row select** → set `selectedCostCenterName` → `useCostCenterDetails(name)` → panel slides in
3. **No selection** → panel: "Seleziona un centro di costo"
4. **"Nuovo centro di costo"** → modal:
   - Name (text, required)
   - Manager (single-select from users list)
   - Users (multi-select from users list)
   - Groups (multi-select from groups list)
   - Enabled (toggle, default true)
   - → POST → toast (`response.message`) → invalidate CC list
5. **"Modifica"** → modal (pre-populated from detail data):
   - New name (text, optional — include `new_name` in body only if different from current)
   - Manager (single-select, pre-selected: `details.manager.id`)
   - Users (multi-select, pre-selected: `details.users.map(u => u.id)`)
   - Groups (multi-select, pre-selected: `details.groups.map(g => g.name)`)
   - → PUT → rename handling → toast → refresh
6. **"Disabilita"** → confirm dialog (danger):
   - Header: "Disabilitare {name}?"
   - Impact: "Questo centro di costo ha {n} utenti assegnati:" + user list from **current detail data**
   - **Stale data guard:** The detail query is the authoritative source. The confirm modal should only open if the detail query is not stale (i.e., `isFetching === false`). If the user clicks Disable while details are refetching, show a loading state in the button until details are fresh.
   - → PUT `{ "enabled": false }` → toast → invalidate CC list + details
7. **"Abilita"** → PUT `{ "enabled": true }` → toast → invalidate CC list + details (no confirmation — non-destructive)

### 4.7 Component extraction from Phase 1

After building Centri di costo, both Gruppi and CC share these patterns:

- Master table with row selection + skeleton + empty state
- Slide-in detail panel with read-only fields + action buttons
- Modal with form fields
- Multi-select dropdown
- Toast notifications
- Confirm dialog

**Now extract to `@mrsmith/ui`** the components that are genuinely identical between the two views. Likely candidates:
- DataTable (if the column definition API stabilized)
- DetailPanel (if the layout is the same)
- Modal, ConfirmDialog, Toast (almost certainly reusable)
- MultiSelect, Select (dropdown behavior)
- Button variants
- StatusBadge, EmptyState

**Do NOT extract** components that differ between views — keep those view-local.

**Export wiring:** Each extracted component follows the structure from Phase 1 Step 0.4: own directory under `packages/ui/src/components/`, colocated CSS, and re-exported from `packages/ui/src/index.ts`. This is already established — extraction is just moving files and adding export lines.

### 4.8 Italian labels

- "Nuovo centro di costo", "Modifica", "Disabilita", "Abilita", "Conferma", "Annulla"
- "Attivo", "Disabilitato" (status badges)
- "Nome", "Manager", "Utenti", "Gruppi", "Stato" (form/panel labels)
- "Seleziona un centro di costo" (empty panel)
- "Nessun centro di costo trovato" (empty table)
- "Questo centro di costo ha {n} utenti assegnati:" (disable confirmation)
- Toast messages: use `response.message` from API

### 4.9 Error handling

Same pattern as Phase 1:
- API errors → toast with error message
- Mutation errors → toast, modal stays open
- Network failure → "Errore di connessione" toast
- Loading → skeleton (page), spinner (buttons during mutation)

---

## Step 5: Validation & Polish

### 5.1 Functional testing

- [ ] Cost center list loads with skeleton → data (verify `page_number` + `disable_pagination` sent)
- [ ] Row selection shows detail panel with member list
- [ ] Create CC with manager + users + groups → appears in list
- [ ] Edit CC (rename) → list refreshes with new name, panel updates, selection tracks new name
- [ ] Edit CC (change manager, members, no rename) → detail panel refreshes
- [ ] Disable CC → confirm shows affected users → status changes to "Disabilitato"
- [ ] Enable CC → status changes to "Attivo" (no confirmation)
- [ ] Disabled CC: "Modifica" + "Abilita" shown (not "Disabilita")
- [ ] Enabled CC: "Modifica" + "Disabilita" shown (not "Abilita")
- [ ] Disable confirm: if detail is refetching, button shows loading state
- [ ] Error states display correctly
- [ ] Empty states render

### 5.2 Cross-view cache validation

- [ ] Navigate Gruppi → Centri di costo → users not re-fetched (shared cache)
- [ ] Navigate Gruppi → Centri di costo → groups not re-fetched (shared cache)
- [ ] Create group in Gruppi → navigate to CC → group appears in CC group dropdown
- [ ] Delete group in Gruppi → navigate to CC → group gone from CC group dropdown
- [ ] Tab highlighting correct on both views

### 5.3 Contract verification

- [ ] All GET list requests include `page_number=1&disable_pagination=true`
- [ ] POST/PUT responses are `{ "message": string }` — UI uses refetch, not response data
- [ ] Path param uses `cost_center_id` (URL-encoded cost center name)
- [ ] PUT for disable sends `{ "enabled": false }`, for enable sends `{ "enabled": true }`

### 5.4 WOW effect checklist

- [ ] Phase 1 animations carry over (skeleton, panel slide, modal, toast)
- [ ] Disable confirmation has appropriate gravity (danger styling, impact user list)
- [ ] Enable action feels lightweight
- [ ] Status badges have clean visual weight
- [ ] Multi-select handles 10+ items (users + groups)
- [ ] Single-select (manager) search works smoothly

---

## Deliverables

| Deliverable | Location |
|-------------|----------|
| Shared reference data hooks | `apps/budget/src/api/shared-queries.ts` |
| Go BFF handlers (cost center fixtures) | `backend/internal/budget/handler.go`, `fixtures/cost_centers.go` |
| Cost center TypeScript types | `apps/budget/src/api/types.ts` |
| Centri di costo view (CRUD + disable/enable) | `apps/budget/src/views/centri-di-costo/` |
| Extracted `@mrsmith/ui` components (from Phase 1+2 usage) | `packages/ui/src/components/` |

**Phase 2 is complete when:** Centri di costo is fully functional with mocked data, shared reference data hooks are promoted to app level, cross-view cache sharing is verified between Gruppi and CC, the rename flow handles name identity correctly, and common UI components are extracted to `@mrsmith/ui` based on proven reuse.

---

## Changes from original plan (feedback incorporation)

| Issue | Original | Revised |
|-------|----------|---------|
| Response shapes | POST→201 echo, PUT→200 echo | All mutations return `200` with `{ "message": string }` — refetch-driven flows |
| Route registration | `/api/budget/v1/...` | `/budget/v1/...` (StripPrefix removes `/api`) |
| Path param name | `{name}` | `{cost_center_id}` (string, per spec) |
| Query params | Not mentioned | `page_number=1&disable_pagination=true` on all list endpoints |
| Shared data hooks | "From Phase 1, reused here" (assumed) | Explicitly promoted to `shared-queries.ts` as first step |
| Rename handling | Not addressed | Explicit cache removal + selection tracking (same pattern as Groups) |
| Disabled CC edit rule | Ambiguous ("Modifica" shown for disabled) | Explicit decision: Edit allowed for disabled CCs, documented as intentional change from Appsmith |
| Stale detail data | Not addressed | Disable confirm guards against stale data — waits for detail query to be fresh |
| Component extraction | Upfront (Phase 1) | After Phase 2 — extract only what both views prove is reusable |
| Cache sharing | "Validation step" | Architectural dependency — promoted hooks before building view |
| ApiError prerequisite | Implicit | Explicit hard prerequisite: Phase 1 Step 0.1 must be complete |
| Fixture param validation | Not stated | Handlers validate `page_number` as required — 400 if missing |
| `@mrsmith/ui` export wiring | Implicit | Extracted components follow Phase 1 Step 0.3 structure + re-export |
