# Budget Management — Implementation Phase 3: Voci di costo

**Goal:** Build the budget list and detail pages with two-page drill-down, tabbed allocations, row-expansion for approval rules, and CRUD for 5 entity types.

**Depends on:** Phase 2 complete — shared data layer, extracted `@mrsmith/ui` components, master-detail pattern proven.

**Structure:** This phase is split into three sub-slices to manage risk. Each sub-slice is independently testable before proceeding.

---

## Sub-slice 3A: Budget List + Detail Header + Navigation

### 3A.1 Go BFF — Budget handlers (5)

```
backend/internal/budget/fixtures/
└── budgets.go     # NEW: budget list + details
```

**Budget fixtures:**
- 6 budgets across years 2025 + 2026 (to test "active" flag)
- List items per spec `budget`: `{ "id": 1, "name": "Marketing", "year": 2026, "limit": "50000.00", "current": "32100.50" }`
- **`limit` and `current` are strings** — fixture data must use quoted strings, not numbers
- Details per spec `budget-details`: budget fields + `user_budgets: []` + `cost_center_budgets: []` (empty for now — allocations added in 3B fixtures)

| Route registration | Method | Handler | Status | Response body |
|--------------------|--------|---------|--------|---------------|
| `GET /budget/v1/budget` | GET | `handleGetAllBudgets` | 200 | Paginated `budget[]` envelope |
| `GET /budget/v1/budget/{budget_id}` | GET | `handleGetBudgetDetails` | 200 | `budget-details` |
| `POST /budget/v1/budget` | POST | `handleNewBudget` | 200 | `{ "id": <int64> }` (`id-object`) |
| `PUT /budget/v1/budget/{budget_id}` | PUT | `handleEditBudget` | 200 | `{ "message": "budget updated" }` |
| `DELETE /budget/v1/budget/{budget_id}` | DELETE | `handleDeleteBudget` | 200 | `{ "message": "budget deleted" }` |

**Contract notes:**
- `budget_id` path param is `integer (int64)` — not string like groups/cost centers
- **NewBudget returns `id-object`** (`{ "id": int64 }`), NOT `message`. This is the only budget mutation that returns an ID.
- EditBudget and DeleteBudget return `message`
- GetAllBudgets accepts query params: `page_number` (required), `disable_pagination`, `search_string`, `year`
- **Preserve room for `search_string` and `year` filters**: handlers must accept these params. UI does not use them now, but the handler should not reject them. Given the open TODO about budget year-end lifecycle, the list view should be structured so adding a year filter later is trivial (e.g., filter bar area in the layout).

### 3A.2 API client error model (already built)

`ApiError` was built in Phase 1 Step 0.1. It provides `status`, `statusText`, `path`, and parsed `body` (the full server error payload as JSON, enabling toast messages from backend errors). This phase uses it for:
- Detail page: catch `ApiError` with `status === 404` → navigate to `/budgets` + "Budget non trovato" toast
- Delete navigation: after delete → navigate to `/budgets` (no 404 handling needed, just redirect)

### 3A.3 TypeScript types

Add to `src/api/types.ts`:

```typescript
/** schema: id-object — returned by NewBudget, NewUserBudgetApprovalRule, NewCCBudgetApprovalRule */
export interface IdResponse {
  id: number;
}

/** schema: budget */
export interface Budget {
  id: number;
  name: string;
  year: number;
  limit: string;   // decimal as string — DO NOT parse to number in state
  current: string;  // decimal as string — DO NOT parse to number in state
}

/** schema: budget-details */
export interface BudgetDetails extends Budget {
  cost_center_budgets: CostCenterBudgetAllocation[];
  user_budgets: UserBudgetAllocation[];
}

/** schema: budget-new */
export interface BudgetNew {
  name: string;
  year: number;
}

/** schema: budget-edit */
export interface BudgetEdit {
  name?: string;
  year?: number;
}
```

### 3A.4 Monetary string display

`limit`, `current`, and `threshold` are **strings in the API and strings in state**. No parsing to `number` anywhere in the data layer.

Formatting is **presentational only** — a display utility, not a transform:

```typescript
// In apps/budget/src/utils/format.ts (budget-domain, NOT @mrsmith/ui)
// Formats API string "50000.00" for Italian display "50.000,00"
export function formatMoneyDisplay(apiValue: string): string;
```

This is a budget-domain utility, not a generic UI component. It does not belong in `@mrsmith/ui` because it encodes locale assumptions (Italian number formatting) and the API's decimal-as-string convention.

**Input fields:** Users enter monetary values as **raw decimal strings in API format** (e.g., `50000.00`, not `50.000,00`). The input is a plain text field with:
- Placeholder showing the expected format: `es. 50000.00`
- Inline help text: "Inserire il valore in formato decimale (es. 1500.00)"
- Client-side validation before submit: must match `/^\d+(\.\d{1,2})?$/` (digits, optional dot, up to 2 decimal places). Reject and show error if invalid.
- Value sent to API as-is (string, no conversion).

**Why raw format, not Italian locale:** The API expects a specific decimal string format. Introducing locale-formatted input (`50.000,00`) requires a bidirectional conversion layer (display→API, API→display) that is error-prone with edge cases (thousand separators, comma vs dot). Raw format is unambiguous and matches the API contract directly. `formatMoneyDisplay` is used only for read-only display in tables and panels, never for input.

No `NumberInput` component — that conflates locale formatting with data entry and invites float conversion bugs.

### 3A.5 Budget list page (`/budgets`)

**View structure:**
```
src/views/voci-di-costo/
├── BudgetListPage.tsx
├── BudgetDetailPage.tsx    # (3A.6)
├── BudgetCreateModal.tsx
├── BudgetEditModal.tsx
├── BudgetDeleteConfirm.tsx
└── queries.ts
```

**Layout:**
```
┌─────────────────────────────────────────────────────────┐
│ [Home] [▶Voci di costo] [Centri di costo] [Gruppi]     │
├─────────────────────────────────────────────────────────┤
│                                                         │
│ Voci di costo                        [+ Nuovo budget]   │
│                                                         │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ Nome        Anno   Limite      Corrente    Attivo   │ │
│ │ Marketing   2026   50.000,00   32.100,50   ●       → │
│ │ IT          2026   30.000,00   18.200,00   ●       → │
│ │ HR          2025   20.000,00   19.800,00           → │
│ └─────────────────────────────────────────────────────┘ │
│                                                         │
│ ┌ filter area (empty now, reserved for future year/     │
│   search filters per TODO)                              │
└─────────────────────────────────────────────────────────┘
```

**Columns:** Nome, Anno, Limite (`formatMoneyDisplay`), Corrente (`formatMoneyDisplay`), Attivo (green dot if `year === currentYear`)

**Row click** → `navigate(`/budgets/${budget.id}`)`

**Create budget** → modal: name (required), year (required, default current year) → POST → returns `{ id }` → toast → invalidate budget list. Optionally navigate to `/budgets/${response.id}`.

**Query hooks:**
```typescript
// Query key includes a filters object so adding search_string/year later
// does not require a cache-key redesign — just extend the filters type.
interface BudgetListFilters {
  // Currently empty. Future: search_string?: string; year?: number;
}

const budgetKeys = {
  list: (filters: BudgetListFilters = {}) => ['budget', 'budgets', filters] as const,
  details: (id: number) => ['budget', 'budget-details', id] as const,
};

useBudgets(filters?: BudgetListFilters)
  → queryKey: budgetKeys.list(filters)
  → GET /budget/v1/budget?page_number=1&disable_pagination=true (+ future filter params)

useCreateBudget() → POST /budget/v1/budget → returns IdResponse
                  → onSuccess: invalidate budgetKeys.list
```

### 3A.6 Budget detail page (`/budgets/:id`)

**Breadcrumbs:** Build the `Breadcrumbs` component in `@mrsmith/ui` now (deferred from Phase 1, needed here).

**Layout (header only for 3A — tabs added in 3B):**
```
┌─────────────────────────────────────────────────────────┐
│ Voci di costo / Marketing 2026                          │
├─────────────────────────────────────────────────────────┤
│ Marketing 2026                          ● Attivo        │
│ Limite: 50.000,00  |  Corrente: 32.100,50               │
│                                      [Modifica] [Elim.] │
│                                                         │
│ (allocation tabs placeholder — built in 3B)             │
└─────────────────────────────────────────────────────────┘
```

**Route param:** Parse `id` from URL, pass to `useBudgetDetails(id)`.

**Error handling:**
- `ApiError` with `status === 404` → navigate to `/budgets` + toast "Budget non trovato"
- Network error → toast "Errore di connessione"

**Edit budget** → modal: name (optional), year (optional) — send only changed fields → PUT → invalidate `budgetKeys.list` + `budgetKeys.details(id)` → toast

**Delete budget** → confirm → DELETE → invalidate `budgetKeys.list` → navigate to `/budgets` → toast

**3A validation checkpoint:**
- [ ] Budget list loads, monetary values formatted correctly
- [ ] "Active" badge correct (current year only)
- [ ] Row click → navigates to `/budgets/:id`
- [ ] Create → list refreshes, returns `{ id }`
- [ ] Detail page loads from URL (deep link)
- [ ] Edit budget (partial update) → header updates
- [ ] Delete → navigates back to list
- [ ] Invalid ID → 404 handling → redirect + toast
- [ ] Breadcrumb "Voci di costo" → back to list
- [ ] Browser back/forward works

---

## Sub-slice 3B: Allocations (User + Cost Center)

### 3B.1 Go BFF — Allocation handlers (4)

Update budget detail fixtures to include populated `user_budgets[]` and `cost_center_budgets[]`.

| Route registration | Method | Handler | Status | Response body |
|--------------------|--------|---------|--------|---------------|
| `POST /budget/v1/budget/{budget_id}/user` | POST | `handleNewUserBudget` | 200 | `{ "message": "..." }` |
| `PUT /budget/v1/budget/{budget_id}/user` | PUT | `handleEditUserBudget` | 200 | `{ "message": "..." }` |
| `POST /budget/v1/budget/{budget_id}/cost-center` | POST | `handleNewCcBudget` | 200 | `{ "message": "..." }` |
| `PUT /budget/v1/budget/{budget_id}/cost-center` | PUT | `handleEditCcBudget` | 200 | `{ "message": "..." }` |

All allocation mutations return `message` — refetch-driven.

### 3B.2 TypeScript types

```typescript
/** schema: user-budget */
export interface UserBudgetAllocation {
  limit: string;        // string — decimal
  current: string;      // string — decimal
  user_id: number;
  user_email: string;
  budget_id: number;
  enabled: boolean;
}

/** schema: user-budget-upsert */
export interface UserBudgetNew {
  limit: string;        // string — sent as-is
  user_id: number;
}

/** schema: user-budget-edit */
export interface UserBudgetEdit {
  user_id: number;      // required — identifies allocation
  limit?: string;
  enabled?: boolean;
}

// CC variants — same pattern with cost_center: string instead of user_id
/** schema: cost_center-budget */
export interface CostCenterBudgetAllocation { ... }
/** schema: cost_center-budget-upsert */
export interface CostCenterBudgetNew { limit: string; cost_center: string; }
/** schema: cost_center-budget-edit */
export interface CostCenterBudgetEdit { cost_center: string; limit?: string; enabled?: boolean; }
```

### 3B.3 Tabs component → `@mrsmith/ui`

Build `Tabs` in `packages/ui/src/components/`:
- Props: `items: { label: string, key: string }[]`, `activeKey`, `onChange`
- Active tab indicator with smooth slide animation
- Content area with crossfade transition

### 3B.4 Allocation tabs on detail page

Add below budget header:

```
┌─────────────────────────────────────────────────────────┐
│ [▶ Utenti]  [Centri di costo]                           │
├─────────────────────────────────────────────────────────┤
│                                        [+ Allocazione]  │
│ Utente          Limite     Corrente   Attivo            │
│ mario@acme      5.000,00   3.200,00   ✓                 │
│ giulia@acme     3.000,00   1.800,00   ✓                 │
│ paolo@acme      2.000,00   900,00     ✓            [▼]  │
└─────────────────────────────────────────────────────────┘
```

**User allocations tab:** Table from `budgetDetails.user_budgets`. Columns: user_email, limit (formatted), current (formatted), enabled.
- Per-row edit action → modal: limit (text input, string), enabled (toggle) → PUT
- "Allocazione" → modal: user (select, exclude already-allocated), limit (text, required) → POST

**CC allocations tab:** Same pattern with `budgetDetails.cost_center_budgets`, cost_center name.

**Allocation mutations** → invalidate `budgetKeys.details(budgetId)` (re-fetches detail with updated allocations).

**3B validation checkpoint:**
- [ ] Tabs render with smooth switching
- [ ] User allocations table shows from budget details
- [ ] Create user allocation → detail refreshes, new allocation appears
- [ ] Edit user allocation (limit, enabled) → detail refreshes
- [ ] CC allocations same pattern
- [ ] Monetary values formatted correctly in allocation tables

---

## Sub-slice 3C: Approval Rules (Row Expansion)

### 3C.1 Go BFF — Approval rule handlers (8)

```
backend/internal/budget/fixtures/
└── approval_rules.go   # NEW: user + CC approval rules
```

| Route registration | Method | Status | Response body |
|--------------------|--------|--------|---------------|
| `GET /budget/v1/approval-rules/user-budget` | GET | 200 | Paginated `user-budget-approval-rule[]` |
| `POST /budget/v1/approval-rules/user-budget` | POST | 200 | `{ "id": <int64> }` (`id-object`) |
| `PUT /budget/v1/approval-rules/user-budget/{rule_id}` | PUT | 200 | `{ "message": "..." }` |
| `DELETE /budget/v1/approval-rules/user-budget/{rule_id}` | DELETE | 200 | `{ "message": "..." }` |
| `GET /budget/v1/approval-rules/cost-center-budget` | GET | 200 | Paginated `cc-budget-approval-rule[]` |
| `POST /budget/v1/approval-rules/cost-center-budget` | POST | 200 | `{ "id": <int64> }` (`id-object`) |
| `PUT /budget/v1/approval-rules/cost-center-budget/{rule_id}` | PUT | 200 | `{ "message": "..." }` |
| `DELETE /budget/v1/approval-rules/cost-center-budget/{rule_id}` | DELETE | 200 | `{ "message": "..." }` |

**Contract notes:**
- Rule **create** returns `id-object` (`{ "id": int64 }`), not `message`
- Rule **edit** and **delete** return `message`
- `rule_id` path param is `integer (int64)`
- GET list accepts: `page_number`, `disable_pagination`, `level`, `budget_id`, `user_id` (or `cost_center`)

### 3C.2 TypeScript types

```typescript
/** schema: user-budget-approval-rule */
export interface UserBudgetApprovalRule {
  id: number;
  threshold: string;       // decimal as string
  approver_id: number;
  approver_email: string;
  budget_id: number;
  user_id: number;
  level: number;
  send_email: boolean;
}

/** schema: user-budget-approval-rule-new */
export interface UserBudgetApprovalRuleNew {
  threshold: string;
  approver_id: number;
  budget_id: number;
  user_id: number;
  level: number;
  send_email: boolean;
}

/** schema: user-budget-approval-rule-edit — MUTABLE FIELDS ONLY */
export interface UserBudgetApprovalRuleEdit {
  threshold?: string;
  approver_id?: number;
  level?: number;
  send_email?: boolean;
}

// CC variants — same pattern with cost_center: string instead of user_id
```

**Frozen parent identifiers in edit flows:** The edit types for approval rules include only mutable fields: `threshold`, `approver_id`, `level`, `send_email`. The parent identifiers (`budget_id`, `user_id` / `cost_center`) are **NOT included** in the edit type and **NOT shown as editable fields** in the edit modal. These values are context (determined by which allocation row is expanded), not user inputs. This prevents accidental "reparenting" of rules.

### 3C.3 ExpandableRow component → `@mrsmith/ui`

Build `ExpandableRow` (or adapt DataTable to support expansion):
- Expand/collapse toggle per row (chevron icon)
- Smooth height animation
- Only one row expanded at a time (accordion)

### 3C.4 Query keys and invalidation for approval rules

```typescript
const ruleKeys = {
  userRules: (budgetId: number, userId: number) =>
    ['budget', 'user-rules', budgetId, userId] as const,
  ccRules: (budgetId: number, costCenter: string) =>
    ['budget', 'cc-rules', budgetId, costCenter] as const,
};
```

**Why composite keys:** Rules are fetched per-allocation (filtered by `budget_id` + `user_id` or `cost_center`). Using `(budgetId, userId)` as the query key prevents cross-contamination when switching between expanded rows. Collapsing a row does NOT remove the cache — expanding the same row again hits cache.

**Mutations:**
- Create rule → invalidate `ruleKeys.userRules(budgetId, userId)` or `ruleKeys.ccRules(budgetId, costCenter)`
- Edit/delete rule → same invalidation

### 3C.5 Approval rules in row expansion

Inside expanded allocation row:

```
┌─────────────────────────────────────────────────────┐
│ Regole di approvazione                  [+ Regola]  │
│                                                     │
│ Liv. 1 │ Soglia: 500,00  │ anna@  │ Email ✓ │ ✎ ✕  │
│ Liv. 2 │ Soglia: 2000,00 │ luca@  │ Email ✓ │ ✎ ✕  │
│                                                     │
│ (empty: "Nessuna regola definita")                  │
└─────────────────────────────────────────────────────┘
```

**Fetch on expand:** When a user allocation row is expanded, `useUserApprovalRules(budgetId, userId)` fires (via `enabled: isExpanded`). GET request includes `budget_id` and `user_id` as query params.

**Create rule** → modal:
- Level (select: Livello 1, 2, 3)
- Threshold (text input, string)
- Approver (select from users list)
- Send email (toggle, default true)
- `budget_id` and `user_id` injected from context (NOT shown as form fields)
- → POST → returns `{ id }` → invalidate rules → toast

**Edit rule** → modal (pre-populated from rule data):
- Level, threshold, approver, send_email — all editable
- `budget_id`, `user_id` — **NOT shown, NOT editable** (frozen parent identifiers)
- → PUT → invalidate rules → toast

**Delete rule** → confirm → DELETE → invalidate rules → toast

### 3C.6 Validation checkpoint

- [ ] Row expansion loads rules with animation
- [ ] Rules filtered correctly per allocation (no cross-contamination)
- [ ] Expand different row → previous closes, new one loads correct rules
- [ ] Create rule (level select 1–3, threshold, approver, send_email toggle) → rules refresh
- [ ] Edit rule — only mutable fields shown, parent IDs frozen
- [ ] Delete rule → confirm → rules refresh
- [ ] Create returns `{ id }` — UI uses refetch, not response ID
- [ ] Threshold displayed with `formatMoneyDisplay`

---

## Step 4: Italian Labels (all sub-slices)

- "Nuovo budget", "Modifica", "Elimina"
- "Utenti", "Centri di costo" (tab labels)
- "Nuova allocazione", "Nuova regola"
- "Livello 1/2/3", "Soglia", "Approvatore", "Invio email"
- "Limite", "Corrente", "Attivo", "Nome", "Anno"
- "Regole di approvazione"
- "Nessuna regola definita" (empty rules)
- "Nessuna allocazione" (empty allocation tab)
- "Nessun budget trovato" (empty list)
- "Budget non trovato" (404 toast)
- Toast messages: use `response.message` from API where available

---

## Step 5: Final Validation

### 5.1 End-to-end (all sub-slices together)

- [ ] `/budgets` → list → click row → `/budgets/:id` → header + tabs
- [ ] Create budget → list + navigate to detail
- [ ] Edit budget (partial update) → header + list update
- [ ] Delete budget → navigate to list
- [ ] Deep link `/budgets/42` → loads correctly
- [ ] Deep link `/budgets/99999` → 404 → redirect to list + toast
- [ ] Tab switch (Utenti ↔ CC) → smooth animation
- [ ] Create/edit user allocation → detail refresh
- [ ] Create/edit CC allocation → detail refresh
- [ ] Expand user row → rules load → CRUD on rules
- [ ] Expand CC row → rules load → CRUD on rules
- [ ] Switch expanded row → correct rules, no cross-contamination
- [ ] Browser back/forward across list ↔ detail
- [ ] Breadcrumb "Voci di costo" → back to list
- [ ] Tab highlighting: "Voci di costo" active on both list and detail

### 5.2 Contract verification

- [ ] NewBudget returns `{ "id": int64 }` (not `message`)
- [ ] NewUserBudgetApprovalRule / NewCCBudgetApprovalRule return `{ "id": int64 }`
- [ ] All other mutations return `{ "message": string }`
- [ ] All GET list requests include `page_number=1&disable_pagination=true`
- [ ] Rule GET requests include `budget_id` + `user_id` or `cost_center` filter params
- [ ] `budget_id` path param is integer, not string
- [ ] `rule_id` path param is integer
- [ ] Monetary values stored and sent as strings, never parsed to float

### 5.3 WOW effect checklist

- [ ] List → detail page transition is smooth
- [ ] Breadcrumb appears with subtle animation
- [ ] Tab indicator slides on switch
- [ ] Tab content crossfades
- [ ] Row expansion height animates smoothly
- [ ] Rules appear with stagger animation inside expansion
- [ ] Monetary values aligned consistently in tables
- [ ] Active badge clean and subtle

---

## Deliverables

| Deliverable | Location |
|-------------|----------|
| `ApiError` class in api-client | `packages/api-client/src/` |
| Go BFF handlers (17 total for budget domain) | `backend/internal/budget/` |
| Budget + allocation + rule TypeScript types | `apps/budget/src/api/types.ts` |
| Monetary display utility | `apps/budget/src/utils/format.ts` |
| Breadcrumbs component | `packages/ui/src/components/` |
| Tabs component | `packages/ui/src/components/` |
| ExpandableRow component | `packages/ui/src/components/` |
| Budget list page (`/budgets`) | `apps/budget/src/views/voci-di-costo/` |
| Budget detail page (`/budgets/:id`) | `apps/budget/src/views/voci-di-costo/` |

**Phase 3 is complete when:** All three sub-slices pass their validation checkpoints, the full budget CRUD + allocation + approval rule flow works end-to-end, deep linking and 404 handling work, and monetary values are string-based throughout.

---

## Changes from original plan (feedback incorporation)

| Issue | Original | Revised |
|-------|----------|---------|
| Phase size | One monolithic block (17 handlers + 2 pages + tabs + expansion + rules) | Split into 3 sub-slices: 3A (list+detail), 3B (allocations), 3C (rules) |
| Response shapes | All mutations described generically | Precise: NewBudget→`id-object`, NewRule→`id-object`, all others→`message` |
| Rule edit fields | Included `budget_id`, `user_id`, `cost_center` as editable | Frozen parent identifiers — edit types include only mutable fields |
| Rule query keys | Not specified | Composite keys: `(budgetId, userId)` / `(budgetId, costCenter)` to prevent cross-contamination |
| NumberInput component | Generic `@mrsmith/ui` component with formatting | Removed. Monetary values stay as strings. Plain text input + display-only formatting utility |
| List filters | Hard-coded unfiltered | Layout reserves filter area; handlers accept `search_string`/`year` params |
| Error handling | Not specified for deep links | `ApiError` class in api-client; 404 → redirect to list + toast |
| Monetary values | "Parse/format" with float conversion | Strings end-to-end. `formatMoneyDisplay` is presentation only, never touches state |
| Monetary input format | "Plain text, sent as-is" (ambiguous) | Raw API decimal format (`50000.00`), validated regex, placeholder + help text, no locale conversion |
| Budget list query keys | Static `['budget', 'budgets']` | Extensible: `['budget', 'budgets', filters]` with `BudgetListFilters` object for future `search_string`/`year` |
| ApiError body | Status only (Phase 1 Step 0.1) | Phase 1 Step 0.1 already includes parsed `body` field — confirmed sufficient for server error toasts |
