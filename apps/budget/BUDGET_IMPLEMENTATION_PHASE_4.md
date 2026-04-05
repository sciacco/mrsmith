# Budget Management — Implementation Phase 4: Home Dashboard + Finalization

**Goal:** Build the Home dashboard with parameterized budget alerts and unassigned users report, add cross-view navigation, and finalize the full app.

**Depends on:** Phase 3 complete — `ApiError` class in api-client (built in 3A), budget detail deep linking working, all CRUD views operational.

**Prerequisite check:** `ApiError` in `@mrsmith/api-client` (built in Phase 1 Step 0.1) and budget detail deep linking (built in Phase 3A) must both be working.

---

## Step 1: Go BFF — Report Fixture Handlers

### 1.1 Add report fixtures

```
backend/internal/budget/fixtures/
└── reports.go     # NEW: budget-over-percentage + unassigned users
```

**Budget over percentage fixture:**
- Return 2–3 budgets that exceed a given threshold
- Handler must parse `percentage` query param (float) and filter: include budgets where `(parsedCurrent / parsedLimit * 100) > percentage`
- Shape: `budget` items in paginated envelope (same schema as budget list)

**Unassigned users fixture:**
- Return 2–3 users from the users fixture that are NOT referenced in any budget allocation fixture
- Shape: `arak-int-user[]` in paginated envelope

### 1.2 Handlers

| Route registration | Method | Handler | Status | Response body |
|--------------------|--------|---------|--------|---------------|
| `GET /budget/v1/report/budget-used-over-percentage` | GET | `handleGetBudgetOverPercent` | 200 | Paginated `budget[]` envelope |
| `GET /budget/v1/report/unassigned-users` | GET | `handleGetUnassignedUsers` | 200 | Paginated `arak-int-user[]` envelope |

**Query params (from `docs/mistra-dist.yaml`):**

Budget over percentage:
- `percentage` (number, float format, required)
- `page_number` (integer, required)
- `disable_pagination` (boolean, optional)

Unassigned users:
- `enabled` (boolean, optional — frontend always sends `true`)
- `page_number` (integer, required)
- `disable_pagination` (boolean, optional)

**Fixture handler validation:**
- `page_number` required on both handlers — return 400 if missing (same rule as all phases)
- `percentage` required on budget-over-percentage — return 400 if missing, non-numeric, negative, or > 100. This ensures the frontend's threshold validation is tested against a realistic mock.

---

## Step 2: Home Dashboard View

### 2.1 View structure

```
src/views/home/
├── HomePage.tsx                # Page component
├── BudgetAlertSection.tsx      # Budget over-threshold report section
├── UnassignedUsersSection.tsx  # Unassigned users report section
├── ThresholdInput.tsx          # Validated percentage input with debounce
└── queries.ts                  # Report query hooks
```

### 2.2 Data fetching (`queries.ts`)

```typescript
const reportKeys = {
  budgetAlerts: (percentage: number) =>
    ['budget', 'report-over-percentage', percentage] as const,
  unassignedUsers: ['budget', 'report-unassigned-users'] as const,
};

useBudgetAlerts(percentage: number | null)
  → queryKey: reportKeys.budgetAlerts(normalizedPercentage)
  → GET /budget/v1/report/budget-used-over-percentage
      ?percentage={percentage}&page_number=1&disable_pagination=true
  → enabled: percentage !== null  (suppress request when input is invalid)

// Normalize percentage to avoid cache churn from float equivalence:
// Round to 1 decimal place before using as query key.
// e.g., 80.10000001 and 80.1 → both become 80.1
const normalizedPercentage = percentage !== null
  ? Math.round(percentage * 10) / 10
  : null;

useUnassignedUsers()
  → queryKey: reportKeys.unassignedUsers
  → GET /budget/v1/report/unassigned-users
      ?enabled=true&page_number=1&disable_pagination=true
```

### 2.3 Threshold input validation

The `percentage` API param is a required float. The input must handle edge cases explicitly:

**ThresholdInput behavior:**
- Default value: `80.1`
- Valid range: `0 < value <= 100`
- Accepts decimal input (float)
- **Invalid states:** empty string, non-numeric text, negative values, values > 100, zero
- While input is invalid: do NOT fire API request (`useBudgetAlerts` receives `null` → `enabled: false`)
- Show inline validation hint when invalid: "Inserire un valore tra 0.1 e 100"
- **Debounce:** 500ms after last keystroke, if value is valid → fire request
- The input sends the parsed float to the query hook, never the raw text

**State machine:**
```
User types → validate → invalid? show hint, suppress query
                      → valid?   debounce 500ms → fire query
```

### 2.4 Page layout and state model

```
┌─────────────────────────────────────────────────────────┐
│ [▶Home] [Voci di costo] [Centri di costo] [Gruppi]     │
├─────────────────────────────────────────────────────────┤
│                                                         │
│ Budget Management                                       │
│                                                         │
│ ┌─ Budget Alert Section ──────────────────────────────┐ │
│ │ Budget oltre il [80.1] %                            │ │
│ │ (table or empty — see state model below)            │ │
│ └─────────────────────────────────────────────────────┘ │
│                                                         │
│ ┌─ Unassigned Users Section ──────────────────────────┐ │
│ │ Utenti non assegnati a nessun Budget                │ │
│ │ (table or empty — see state model below)            │ │
│ └─────────────────────────────────────────────────────┘ │
│                                                         │
│ ┌─ All Clear (conditional) ───────────────────────────┐ │
│ │ ✓ Nessun problema rilevato                          │ │
│ └─────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘
```

**Section state model (per section):**

| Query state | Section behavior |
|-------------|-----------------|
| Loading (first fetch) | Show skeleton placeholder (section stays mounted, no layout jump) |
| Loading (refetch after threshold change) | Keep previous data visible + subtle loading indicator (no hide/show flash) |
| Success, `items.length > 0` | Show table |
| Success, `items.length === 0` | Collapse section with smooth height animation (not abrupt hide) |
| Error | Show inline error message in section area |

**Key rule: sections stay mounted during refetch.** When the user changes the threshold and the budget alert query refetches, the section does NOT hide and reappear. It shows the previous data with a subtle loading state (e.g., reduced opacity) until new data arrives. This prevents the "flickering" problem near threshold boundaries.

**All-clear state:** "Nessun problema rilevato" with check icon appears ONLY when:
- Both queries are settled (not loading, not fetching)
- Both `items.length === 0`

If either query is still loading, the all-clear state does NOT render (avoids false positive flash).

### 2.5 Budget alert table

**Columns:** Nome, Anno, Limite (`formatMoneyDisplay`), Corrente (`formatMoneyDisplay`)

**Rows are clickable → `navigate(`/budgets/${budget.id}`)`**
- Row hover: subtle highlight + arrow affordance
- This reuses Phase 3's deep-link behavior. The dashboard is NOT the first place deep linking is tested (Phase 3A already validated it).

### 2.6 Unassigned users table

**Columns:** Nome (`${first_name} ${last_name}`), Email, Stato (`state.name`)

Read-only, no row actions. Nested field access via dot-path in column definition.

### 2.7 Italian labels

- "Budget Management" (page title — keeping English per Appsmith original)
- "Budget oltre il {n} %" (alert section title)
- "Inserire un valore tra 0.1 e 100" (validation hint)
- "Utenti non assegnati a nessun Budget" (unassigned section title)
- "Nome", "Anno", "Limite", "Corrente", "Email", "Stato" (column headers)
- "Nessun problema rilevato" (all-clear state)

---

## Step 3: App-Wide Finalization

### 3.1 Error handling audit

Review all 4 views for consistent error handling using `ApiError`:

- [ ] API fetch errors → toast with error message (extracted from `ApiError` or generic fallback)
- [ ] Mutation errors → toast, modal stays open for retry
- [ ] Network failure → "Errore di connessione" toast
- [ ] 404 on `/budgets/:id` → redirect to `/budgets` + "Budget non trovato" toast
- [ ] Error toasts are consistent style across all views

### 3.2 Loading state audit

- [ ] Every data-dependent section has skeleton loaders (never blank on first load)
- [ ] Skeletons match the shape of actual content
- [ ] Refetch states show previous data with subtle loading indicator (not skeleton again)
- [ ] Transition from skeleton → data is smooth

### 3.3 Empty state audit

- [ ] Gruppi: "Nessun gruppo trovato" (table), "Seleziona un gruppo" (panel)
- [ ] Centri di costo: "Nessun centro di costo trovato" (table), "Seleziona un centro di costo" (panel)
- [ ] Budget list: "Nessun budget trovato"
- [ ] Budget detail allocation tabs: "Nessuna allocazione"
- [ ] Approval rules expansion: "Nessuna regola definita"
- [ ] Home: "Nessun problema rilevato" (both reports empty and settled)
- [ ] All empty states have designed visuals (icon + text + optional action)

### 3.4 Navigation completeness

- [ ] All 4 tabs work and highlight correctly
- [ ] All placeholder views replaced with actual views
- [ ] MrSmith logo → portal return works
- [ ] Budget detail breadcrumb → back to list
- [ ] Home alert rows → budget detail
- [ ] Browser back/forward across all views
- [ ] Direct URL access (deep linking) for all routes

### 3.5 Responsive behavior

- [ ] Tables handle narrow viewports (horizontal scroll or column priority)
- [ ] Detail panels adapt to available width
- [ ] Modals centered and don't overflow on small screens
- [ ] Top bar tabs don't wrap or overflow

---

## Step 4: Full App Walkthrough

End-to-end verification following the 5 user journeys from the migration spec (Phase D):

1. **Journey 1:** Open app → Home → see alerts → adjust threshold (test invalid input, boundary values) → click alert row → budget detail
2. **Journey 2:** Voci di costo → create budget → detail → add allocations → add rules
3. **Journey 3:** Select existing budget → edit allocation → edit/delete rules
4. **Journey 4:** Centri di costo → create CC → edit → disable → enable
5. **Journey 5:** Gruppi → create group → edit (rename) → delete

### 4.1 WOW effect final checklist

- [ ] Dashboard sections have smooth entrance animation
- [ ] Section collapse/expand on threshold change: smooth height transition, no flash
- [ ] Refetch during threshold typing: previous data stays visible, no flicker
- [ ] All-clear state appears only when both queries settled
- [ ] Threshold input has clean focus state and validation hint
- [ ] Clickable budget rows have clear hover affordance
- [ ] Consistent animation timing across all 4 views
- [ ] No janky transitions or layout shifts anywhere in the app
- [ ] Skeleton → data transitions smooth everywhere
- [ ] Typography, spacing, and color feel Stripe-caliber throughout

---

## Deliverables

| Deliverable | Location |
|-------------|----------|
| Go BFF handlers (2 report handlers) | `backend/internal/budget/` |
| Home dashboard view | `apps/budget/src/views/home/` |
| ThresholdInput with validation | `apps/budget/src/views/home/ThresholdInput.tsx` |
| Cross-view navigation (alerts → budget detail) | `apps/budget/src/views/home/` |
| Error/loading/empty state audit (all views) | App-wide |
| Full app walkthrough validation | — |

**Phase 4 is complete when:** All 4 views are functional, all 5 user journeys pass end-to-end, the dashboard handles threshold edge cases gracefully without layout flicker, and the application is ready for mock-to-real API transition.

---

## What Comes Next (Post Phase 4)

1. **Mock-to-real transition** — Replace Go fixture handlers with Arak API proxy, handler by handler
2. **Service credentials** — Configure BFF client credentials grant (client ID + secret) for Arak API calls. User tokens are NOT forwarded — the BFF authenticates to Arak as a service.
3. **Build & deploy** — Production build, Docker image, K8s manifests
4. **User acceptance testing** — Real data, real users, real workflows

---

## Changes from original plan (feedback incorporation)

| Issue | Original | Revised |
|-------|----------|---------|
| API client error model | Mentioned 404 handling but no prerequisite | Explicit prerequisite: `ApiError` from Phase 3A must exist |
| Query params | Semantic only | Full contract: `page_number=1&disable_pagination=true`, `enabled=true`, `percentage` as float |
| Threshold validation | "Debounced number input" | Explicit rules: valid range 0.1–100, suppress query when invalid, show validation hint |
| Section hide/show | "Hidden when items.length === 0" | Sections stay mounted during refetch; collapse with smooth animation only when settled with 0 items |
| All-clear state | "If both empty" | Only when both queries settled (not loading) AND both empty — prevents false positive flash |
| Layout stability | Not addressed | Sections never unmount during refetch; previous data visible with loading indicator |
| Cross-view deep link | Dashboard as first test of deep linking | Phase 3A validates deep linking first; dashboard inherits proven behavior |
| ApiError body | Assumed sufficient | Phase 1 Step 0.1 includes parsed `body` field — confirmed covers server error toasts |
| Fixture `percentage` validation | Not specified | Handler validates: required, numeric, 0 < value ≤ 100 — returns 400 if malformed |
| Query key float churn | Raw float in key | Normalize to 1 decimal place before using as query key |
