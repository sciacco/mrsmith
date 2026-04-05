# Budget Management — Implementation Phase 4: Home Dashboard

**Goal:** Build the Home dashboard view with parameterized budget alerts and unassigned users report, add cross-view navigation (alert rows → budget detail), and finalize the app for handoff.

**Depends on:** Phase 3 complete (budget detail route exists at `/budgets/:id` for cross-view linking)

---

## Step 1: Go BFF — Report Fixture Handlers

### 1.1 Add report fixtures

```
backend/internal/budget/fixtures/
├── groups.go           # (Phase 1)
├── users.go            # (Phase 1)
├── cost_centers.go     # (Phase 2)
├── budgets.go          # (Phase 3)
├── approval_rules.go   # (Phase 3)
└── reports.go          # NEW: budget-over-percentage + unassigned users
```

**Budget over percentage fixture:**
- Returns 2–3 budgets that exceed a threshold
- Must respect the `percentage` query param: filter budgets where `(current/limit * 100) > percentage`
- Shape: same `budget` response (`id`, `name`, `year`, `limit`, `current`) in paginated envelope
- Use budgets from existing fixture with high utilization ratios

**Unassigned users fixture:**
- Returns 2–3 users not assigned to any budget
- Shape: `arak-int-user[]` in paginated envelope
- Use distinct users not referenced in budget allocation fixtures

### 1.2 New handlers

| Route | Method | Handler | Response |
|-------|--------|---------|----------|
| `GET /api/budget/v1/report/budget-used-over-percentage` | GET | `handleGetBudgetOverPercent` | Filtered budget list |
| `GET /api/budget/v1/report/unassigned-users` | GET | `handleGetUnassignedUsers` | Unassigned user list |

**Query params for budget-over-percentage:**
- `percentage` (float, required) — threshold value
- `page_number` (integer, default 1)
- `disable_pagination` (boolean, default false)

**Query params for unassigned-users:**
- `enabled` (boolean) — always `true` from frontend
- `page_number`, `disable_pagination`

**Total new handlers: 2** (lightest phase for backend)

---

## Step 2: TypeScript Types

No new types needed — report endpoints return existing types (`Budget[]` and `ArakIntUser[]`) in the standard paginated envelope. Types from Phase 1 and Phase 3 cover everything.

---

## Step 3: Home Dashboard View

### 3.1 View structure

```
src/views/home/
├── HomePage.tsx                # Main page component
├── BudgetAlertSection.tsx      # Budget over-threshold report
├── UnassignedUsersSection.tsx  # Unassigned users report
├── ThresholdInput.tsx          # Percentage input with debounce
└── useReports.ts               # Data fetching hooks
```

### 3.2 Data fetching (`useReports.ts`)

- `useBudgetAlerts(percentage)` — fetches budget-over-percentage report
  - Re-fetches when `percentage` changes (debounced)
  - `percentage` sent as float (not text)
  - Returns `{ budgets, totalNumber, isLoading, error }`
- `useUnassignedUsers()` — fetches unassigned users
  - Auto-load on mount, `enabled=true`
  - Returns `{ users, totalNumber, isLoading, error }`

### 3.3 Page layout

```
┌─────────────────────────────────────────────────────────┐
│ [▶Home] [Voci di costo] [Centri di costo] [Gruppi]     │
├─────────────────────────────────────────────────────────┤
│                                                         │
│ Budget Management                                       │
│                                                         │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ Budget oltre il [80.1] %                            │ │
│ │                                                     │ │
│ │ ┌─────────────────────────────────────────────────┐ │ │
│ │ │ Nome        Anno   Limite      Corrente         │ │ │
│ │ │ ─────────────────────────────────────────────── │ │ │
│ │ │ Marketing   2026   50.000,00   48.100,50     →  │ │ │
│ │ │ HR          2025   20.000,00   19.800,00     →  │ │ │
│ │ └─────────────────────────────────────────────────┘ │ │
│ └─────────────────────────────────────────────────────┘ │
│                                                         │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ Utenti non assegnati a nessun Budget                │ │
│ │                                                     │ │
│ │ ┌─────────────────────────────────────────────────┐ │ │
│ │ │ Nome          Email           Stato              │ │ │
│ │ │ ─────────────────────────────────────────────── │ │ │
│ │ │ Paolo Verdi   paolo@acme.com  Attivo             │ │ │
│ │ │ Sara Neri     sara@acme.com   Attivo             │ │ │
│ │ └─────────────────────────────────────────────────┘ │ │
│ └─────────────────────────────────────────────────────┘ │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### 3.4 Budget alert section

**Header:** "Budget oltre il {n} %" — with inline number input

**Threshold input:**
- Number input, default 80.1
- Debounced: re-fetch after 500ms of no typing (avoid API calls on every keystroke)
- Send as float to API (not text)
- Minimum 0, maximum 100

**Table:**
- Columns: Nome, Anno, Limite (formatted), Corrente (formatted)
- **Rows are clickable → navigate to `/budgets/:id`** (cross-view link)
- Row hover shows subtle navigation affordance (arrow icon or highlight)
- Skeleton loading during fetch
- Section hidden entirely when `items.length === 0` (no results = section not shown)

### 3.5 Unassigned users section

**Header:** "Utenti non assegnati a nessun Budget"

**Table:**
- Columns: Nome (`first_name last_name`), Email, Stato (`state.name`)
- Read-only, no row actions
- Skeleton loading during fetch
- Section hidden when `items.length === 0`

**Nested field access:**
- `state.enabled` → not displayed (all are enabled per query filter)
- `state.name` → shown as status text
- No IIFE pattern — simple dot-path accessor in column definition

### 3.6 Conditional visibility

Both sections use the same pattern:
- During loading: show skeleton
- After load, if `items.length === 0`: hide section entirely (don't render)
- After load, if `items.length > 0`: show section with animated entrance

If both sections are empty, show a positive empty state: "Nessun problema rilevato" (no issues found) with a check icon.

### 3.7 Italian labels

- "Budget Management" (page title — keeping English per Appsmith original)
- "Budget oltre il {n} %" (alert section title)
- "Percentuale" (threshold input label)
- "Utenti non assegnati a nessun Budget" (unassigned section title)
- "Nome", "Anno", "Limite", "Corrente", "Email", "Stato" (column headers)
- "Nessun problema rilevato" (all-clear empty state)

---

## Step 4: Cross-View Navigation

### 4.1 Budget alert → budget detail

Budget alert table rows link to `/budgets/:id`:
- Click handler: `navigate(`/budgets/${budget.id}`)`
- Row styling: cursor pointer, hover highlight, subtle arrow icon on right
- Tab indicator: "Voci di costo" tab highlights when on `/budgets/:id`

### 4.2 Tab active state refinement

Verify tab highlighting rules work correctly for all routes:

| Route | Active tab |
|-------|-----------|
| `/home` | Home |
| `/budgets` | Voci di costo |
| `/budgets/:id` | Voci di costo |
| `/cost-centers` | Centri di costo |
| `/groups` | Gruppi |

The "Voci di costo" tab must use prefix matching (`/budgets`) to stay active on the detail page.

---

## Step 5: App-Wide Finalization

### 5.1 Error handling audit

Review all 4 views and verify consistent error handling:

- [ ] API fetch errors → toast with Italian message
- [ ] Mutation errors → toast, modal stays open for retry
- [ ] Network failure → "Errore di connessione" toast
- [ ] 404 on `/budgets/:id` (invalid ID) → redirect to `/budgets` with error toast

### 5.2 Loading state audit

- [ ] Every data-dependent section has skeleton loaders (never blank, never spinner)
- [ ] Skeletons match the shape of the actual content
- [ ] Transition from skeleton → data is smooth (fade)

### 5.3 Empty state audit

- [ ] Gruppi: "Nessun gruppo trovato" when no groups
- [ ] Centri di costo: "Nessun centro di costo trovato" when no CCs
- [ ] Voci di costo list: "Nessun budget trovato" when no budgets
- [ ] Voci di costo detail tabs: "Nessuna allocazione" when no allocations
- [ ] Approval rules expansion: "Nessuna regola definita" when no rules
- [ ] Home: "Nessun problema rilevato" when both reports empty
- [ ] All empty states have designed visuals (icon + text + optional action)

### 5.4 Navigation completeness

- [ ] All 4 tabs work and highlight correctly
- [ ] Placeholder views replaced with actual views
- [ ] MrSmith logo → portal return works
- [ ] Budget detail breadcrumb → back to list works
- [ ] Home alert rows → budget detail navigation works
- [ ] Browser back/forward across all views
- [ ] Direct URL access (deep linking) for all routes

### 5.5 Responsive behavior

- [ ] Tables handle narrow viewports (horizontal scroll or column priority)
- [ ] Detail panels adapt to available width
- [ ] Modals are centered and don't overflow on small screens
- [ ] Top bar tabs don't wrap or overflow

---

## Step 6: Validation & Polish

### 6.1 Functional testing

- [ ] Budget alerts load with correct threshold filtering
- [ ] Threshold input re-fetches after debounce
- [ ] Unassigned users load (nested `state.name` displays correctly)
- [ ] Both sections hide when empty
- [ ] All-clear state shows when both sections empty
- [ ] Alert row click → navigates to correct budget detail

### 6.2 WOW effect final checklist

- [ ] Dashboard sections have stagger entrance animation
- [ ] Section show/hide transitions are smooth (not abrupt)
- [ ] Threshold input has clean focus state
- [ ] Clickable budget rows have clear hover affordance
- [ ] All-clear empty state feels polished (not just text)
- [ ] Consistent animation timing across all 4 views
- [ ] No janky transitions or layout shifts anywhere in the app

### 6.3 Full app walkthrough

End-to-end verification following the 5 user journeys from the migration spec (Phase D):

1. **Journey 1:** Open app → Home → see alerts → adjust threshold → click alert row → budget detail
2. **Journey 2:** Voci di costo → create budget → detail → add allocations → add rules
3. **Journey 3:** Select existing budget → edit allocation → edit/delete rules
4. **Journey 4:** Centri di costo → create CC → edit → disable → enable
5. **Journey 5:** Gruppi → create group → edit → delete

---

## Deliverables

| Deliverable | Location |
|-------------|----------|
| Go BFF handlers (2 report handlers) | `backend/internal/budget/` |
| Home dashboard view | `apps/budget/src/views/home/` |
| Cross-view navigation (alerts → budget detail) | `apps/budget/src/views/home/` |
| Error/loading/empty state audit (all views) | App-wide |
| Full app walkthrough validation | — |

**Phase 4 is complete when:** All 4 views are functional, all 5 user journeys pass end-to-end, WOW effect is consistent across the entire app, and the application is ready for the mock-to-real API transition.

---

## What Comes Next (Post Phase 4)

The app is feature-complete with mocked data. The next steps (not covered in these phase plans):

1. **Mock-to-real transition** — Replace Go fixture handlers with Arak API proxy, handler by handler
2. **Auth integration** — Wire Keycloak token passthrough from frontend → Go BFF → Arak
3. **Build & deploy** — Production build, Docker image, K8s manifests
4. **User acceptance testing** — Real data, real users, real workflows
