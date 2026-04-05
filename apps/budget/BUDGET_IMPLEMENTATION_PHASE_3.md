# Budget Management — Implementation Phase 3: Voci di costo

**Goal:** Build the most complex view — budget list and detail pages with two-page drill-down, tabbed allocations, row-expansion for approval rules, and the full CRUD flows for 5 entity types.

**Depends on:** Phase 2 complete (all `@mrsmith/ui` components, shared cache, master-detail pattern proven)

---

## Step 1: Go BFF — Budget Domain Fixture Handlers

### 1.1 Add budget fixtures

```
backend/internal/budget/fixtures/
├── groups.go           # (Phase 1)
├── users.go            # (Phase 1)
├── cost_centers.go     # (Phase 2)
├── budgets.go          # NEW: budget list + details
└── approval_rules.go   # NEW: user + CC approval rules
```

**Budget list fixture:**
- 5–6 sample budgets across years (2025, 2026) to test "active" flag
- Shape: `{ id, name, year, limit, current }` — note `limit` and `current` are **strings**
- Example: `{ id: 1, name: "Marketing", year: 2026, limit: "50000.00", current: "32100.50" }`
- Wrapped in paginated envelope

**Budget details fixture per ID:**
- Shape: `budget-details` with nested `user_budgets[]` and `cost_center_budgets[]`
- 2–3 user allocations and 1–2 CC allocations per budget
- `limit` and `current` as strings in allocations too

**Approval rules fixtures:**
- 2–3 rules per user-budget (levels 1, 2, 3)
- 1–2 rules per CC-budget
- `threshold` as string
- `send_email` mix of true/false

### 1.2 New handlers (9 total for budget domain)

| Route | Method | Handler | Response |
|-------|--------|---------|----------|
| `GET /api/budget/v1/budget` | GET | `handleGetAllBudgets` | Budget list |
| `GET /api/budget/v1/budget/{budget_id}` | GET | `handleGetBudgetDetails` | Budget details with allocations |
| `POST /api/budget/v1/budget` | POST | `handleNewBudget` | Created budget (201) |
| `PUT /api/budget/v1/budget/{budget_id}` | PUT | `handleEditBudget` | Updated budget |
| `DELETE /api/budget/v1/budget/{budget_id}` | DELETE | `handleDeleteBudget` | 204 |
| `POST /api/budget/v1/budget/{budget_id}/user` | POST | `handleNewUserBudget` | Created allocation |
| `PUT /api/budget/v1/budget/{budget_id}/user` | PUT | `handleEditUserBudget` | Updated allocation |
| `POST /api/budget/v1/budget/{budget_id}/cost-center` | POST | `handleNewCostCenterBudget` | Created allocation |
| `PUT /api/budget/v1/budget/{budget_id}/cost-center` | PUT | `handleEditCostCenterBudget` | Updated allocation |

**Approval rule handlers (4):**

| Route | Method | Handler | Response |
|-------|--------|---------|----------|
| `GET /api/budget/v1/approval-rules/user-budget` | GET | `handleGetAllRulesUser` | Rules list (filter by `budget_id` + `user_id` query params) |
| `POST /api/budget/v1/approval-rules/user-budget` | POST | `handleNewRuleUser` | Created rule (201) |
| `PUT /api/budget/v1/approval-rules/user-budget/{rule_id}` | PUT | `handleEditRuleUser` | Updated rule |
| `DELETE /api/budget/v1/approval-rules/user-budget/{rule_id}` | DELETE | `handleDeleteRuleUser` | 204 |

**CC approval rule handlers (4):** Same pattern under `/approval-rules/cost-center-budget`.

**Total new handlers: 17** (9 budget + 4 user rules + 4 CC rules)

---

## Step 2: TypeScript Types

Add to `src/api/types.ts`:

```typescript
interface Budget {
  id: number;
  name: string;
  year: number;
  limit: string;   // decimal as string
  current: string;  // decimal as string
}

interface BudgetDetails extends Budget {
  cost_center_budgets: CostCenterBudgetAllocation[];
  user_budgets: UserBudgetAllocation[];
}

interface BudgetNew {
  name: string;
  year: number;
}

interface BudgetEdit {
  name?: string;
  year?: number;
}

interface UserBudgetAllocation {
  limit: string;
  current: string;
  user_id: number;
  user_email: string;
  budget_id: number;
  enabled: boolean;
}

interface UserBudgetNew {
  limit: string;
  user_id: number;
}

interface UserBudgetEdit {
  user_id: number;
  limit?: string;
  enabled?: boolean;
}

interface CostCenterBudgetAllocation {
  limit: string;
  current: string;
  cost_center: string;
  budget_id: number;
  enabled: boolean;
}

interface CostCenterBudgetNew {
  limit: string;
  cost_center: string;
}

interface CostCenterBudgetEdit {
  cost_center: string;
  limit?: string;
  enabled?: boolean;
}

interface UserBudgetApprovalRule {
  id: number;
  threshold: string;
  approver_id: number;
  approver_email: string;
  budget_id: number;
  user_id: number;
  level: number;
  send_email: boolean;
}

interface UserBudgetApprovalRuleNew {
  threshold: string;
  approver_id: number;
  budget_id: number;
  user_id: number;
  level: number;
  send_email: boolean;
}

interface UserBudgetApprovalRuleEdit {
  threshold?: string;
  approver_id?: number;
  budget_id?: number;
  user_id?: number;
  level?: number;
  send_email?: boolean;
}

// CC variants mirror user variants with cost_center: string instead of user_id
```

### Monetary string utilities

Add to `src/api/` or `@mrsmith/api-client`:

```typescript
// Format string monetary value for display: "50000.00" → "50.000,00"
function formatMoney(value: string): string;

// Parse display input back to API string: "50.000,00" → "50000.00"
function parseMoney(display: string): string;
```

---

## Step 3: Voci di costo — Budget List View (`/budgets`)

### 3.1 View structure

```
src/views/voci-di-costo/
├── BudgetListPage.tsx          # /budgets — list page
├── BudgetDetailPage.tsx        # /budgets/:id — detail page
├── BudgetTable.tsx             # Master table
├── BudgetCreateModal.tsx       # Create budget modal
├── BudgetEditModal.tsx         # Edit budget modal
├── BudgetDeleteConfirm.tsx     # Delete confirmation
├── AllocationTabs.tsx          # Tabbed allocations container
├── UserAllocationsTab.tsx      # User allocations table + expansion
├── CcAllocationsTab.tsx        # CC allocations table + expansion
├── AllocationCreateModal.tsx   # Create allocation (user or CC)
├── AllocationEditModal.tsx     # Edit allocation
├── ApprovalRuleList.tsx        # Rules in row expansion
├── ApprovalRuleCreateModal.tsx # Create rule
├── ApprovalRuleEditModal.tsx   # Edit rule
├── ApprovalRuleDeleteConfirm.tsx
└── useBudgets.ts               # Data fetching hooks
```

### 3.2 Budget list page (`/budgets`)

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
│ │ ─────────────────────────────────────────────────── │ │
│ │ Marketing   2026   50.000,00   32.100,50   ●       → │
│ │ IT          2026   30.000,00   18.200,00   ●       → │
│ │ HR          2025   20.000,00   19.800,00           → │
│ └─────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘
```

**Columns:**
- Nome (name)
- Anno (year)
- Limite (limit — formatted from string)
- Corrente (current — formatted from string)
- Attivo (active — derived: `year === currentYear`, shown as green dot)
- Row click → navigate to `/budgets/:id`

**Actions:**
- "Nuovo budget" → modal: name (required), year (required, default current year) → POST

### 3.3 New `@mrsmith/ui` components

**`Tabs`** — Tab container for allocations:
- Tab definitions: `{ label, key }[]`
- Active tab indicator with smooth slide animation
- Tab content area with crossfade transition

**`ExpandableRow`** — Row expansion for DataTable:
- Expand/collapse toggle per row (chevron icon)
- Smooth height animation on expand/collapse
- Expanded content area below the row (full table width)
- Only one row expanded at a time (accordion behavior)

**`NumberInput`** — Numeric input with formatting:
- Accepts string value (for monetary strings)
- Formats on blur, parses on focus
- Optional prefix/suffix ("€")

---

## Step 4: Voci di costo — Budget Detail View (`/budgets/:id`)

### 4.1 Layout

```
┌─────────────────────────────────────────────────────────┐
│ [Home] [▶Voci di costo] [Centri di costo] [Gruppi]     │
│ Voci di costo / Marketing 2026                          │
├─────────────────────────────────────────────────────────┤
│                                                         │
│ Marketing 2026                          ● Attivo        │
│ Limite: 50.000,00  |  Corrente: 32.100,50               │
│                                      [Modifica] [Elim.] │
│                                                         │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ [▶ Utenti]  [Centri di costo]                       │ │
│ ├─────────────────────────────────────────────────────┤ │
│ │                                    [+ Allocazione]  │ │
│ │ Utente          Limite     Corrente   Attivo        │ │
│ │ ─────────────────────────────────────────────────── │ │
│ │ ▼ mario@acme    5.000,00   3.200,00   ✓            │ │
│ │ ┌─────────────────────────────────────────────────┐ │ │
│ │ │ Regole di approvazione              [+ Regola]  │ │ │
│ │ │                                                 │ │ │
│ │ │ Liv. 1 │ Soglia: 500,00  │ anna@  │ Email ✓ │✎│✕│ │
│ │ │ Liv. 2 │ Soglia: 2000,00 │ luca@  │ Email ✓ │✎│✕│ │
│ │ └─────────────────────────────────────────────────┘ │ │
│ │ ▶ giulia@acme   3.000,00   1.800,00   ✓            │ │
│ │ ▶ paolo@acme    2.000,00   900,00     ✓            │ │
│ └─────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘
```

### 4.2 Budget header section

- Budget name + year (large heading)
- Active badge (derived from year)
- Limit and current (formatted monetary strings)
- "Modifica" → modal: name (optional), year (optional) — partial update, send only changed fields
- "Elimina" → confirm → DELETE → navigate back to `/budgets`

### 4.3 Allocation tabs

**Tab 1: Utenti (User allocations)**

Table columns: user_email, limit (formatted), current (formatted), enabled (checkbox/badge)

Per-row actions:
- "Modifica" → modal: limit (string input), enabled (toggle) → PUT
- Expand row → shows approval rules

"Allocazione" button → modal:
- User (single-select from users list, excluding already-allocated users)
- Limit (string input, required)
- → POST

**Tab 2: Centri di costo (CC allocations)**

Same pattern as user allocations but with cost_center name instead of user_email.

"Allocazione" button → modal:
- Cost center (single-select from cost centers list, excluding already-allocated)
- Limit (string input, required)
- → POST

### 4.4 Approval rules (row expansion)

Displayed as an ordered list inside the expanded row area. Each rule shows:

| Level | Threshold | Approver | Email | Actions |
|-------|-----------|----------|-------|---------|
| Livello 1 | 500,00 | anna@acme.com | ✓ | [✎] [✕] |
| Livello 2 | 2.000,00 | luca@acme.com | ✓ | [✎] [✕] |

"Nuova regola" button → modal:
- Level (select: Livello 1, 2, 3)
- Threshold (string input, required)
- Approver (single-select from users list)
- Send email (toggle, default true)
- → POST

Edit rule → modal (pre-populated):
- Same fields as create, all optional
- → PUT

Delete rule → confirm → DELETE

### 4.5 Data fetching (`useBudgets.ts`)

- `useBudgets()` — budget list
- `useBudgetDetails(id)` — details with nested allocations (route param)
- `useCreateBudget()` → invalidate budget list → toast → optionally navigate to new budget
- `useEditBudget(id)` → invalidate budget list + details → toast
- `useDeleteBudget(id)` → invalidate budget list → toast → navigate to `/budgets`
- `useCreateUserBudget(budgetId)` → invalidate budget details → toast
- `useEditUserBudget(budgetId)` → invalidate budget details → toast
- `useCreateCcBudget(budgetId)` → invalidate budget details → toast
- `useEditCcBudget(budgetId)` → invalidate budget details → toast
- `useUserApprovalRules(budgetId, userId)` — fetch on row expand
- `useCreateUserRule()` → invalidate rules list → toast
- `useEditUserRule(ruleId)` → invalidate rules list → toast
- `useDeleteUserRule(ruleId)` → invalidate rules list → toast
- Same 4 hooks for CC approval rules

### 4.6 Interaction flow

```
/budgets → budget list → click row → /budgets/:id
  → budget header + tabbed allocations load
    → "Utenti" tab (default) shows user allocations
      → expand row → fetch approval rules → show in expansion
        → CRUD on rules within expansion
      → collapse row
    → "Centri di costo" tab → CC allocations (same pattern)
  → breadcrumb "Voci di costo" → back to /budgets
```

### 4.7 Italian labels

- "Nuovo budget", "Modifica", "Elimina"
- "Utenti", "Centri di costo" (tab labels)
- "Nuova allocazione", "Nuova regola"
- "Livello 1/2/3", "Soglia", "Approvatore", "Invio email"
- "Limite", "Corrente", "Attivo"
- "Regole di approvazione"
- "Budget creato/aggiornato/eliminato" (toasts)
- "Allocazione creata/aggiornata" (toasts)
- "Regola creata/aggiornata/eliminata" (toasts)

---

## Step 5: Validation & Polish

### 5.1 Functional testing

**Budget list:**
- [ ] List loads with formatted monetary values
- [ ] "Active" badge shows for current-year budgets only
- [ ] Create budget → appears in list
- [ ] Row click → navigates to `/budgets/:id`

**Budget detail:**
- [ ] Breadcrumb shows "Voci di costo / {name}"
- [ ] Header shows budget info with formatted values
- [ ] Edit budget (partial update) → header updates
- [ ] Delete budget → confirm → navigate back to list
- [ ] Tab switch (Utenti ↔ Centri di costo) animates smoothly

**Allocations:**
- [ ] User allocations table loads from budget details
- [ ] Create allocation → table updates
- [ ] Edit allocation (limit, enabled) → table updates
- [ ] CC allocations same pattern

**Approval rules:**
- [ ] Row expand → rules load with animation
- [ ] Create rule (level select 1–3, threshold, approver, send_email) → rules update
- [ ] Edit rule (pre-populated) → rules update
- [ ] Delete rule → confirm → rules update
- [ ] Only one row expanded at a time

**Navigation:**
- [ ] Breadcrumb "Voci di costo" → back to `/budgets`
- [ ] Browser back/forward works correctly
- [ ] Direct URL `/budgets/42` loads correctly (deep link)
- [ ] Tab highlighting: "Voci di costo" active on both list and detail

### 5.2 WOW effect checklist

- [ ] Page transition from list → detail is smooth
- [ ] Breadcrumb appears with subtle animation
- [ ] Tab underline slides on tab switch
- [ ] Tab content crossfades
- [ ] Row expansion height animates smoothly
- [ ] Approval rules appear with stagger animation inside expansion
- [ ] Monetary values are well-formatted with consistent alignment
- [ ] Active badge has subtle color and weight

---

## Deliverables

| Deliverable | Location |
|-------------|----------|
| Go BFF handlers (17 budget domain handlers) | `backend/internal/budget/` |
| Budget + allocation + rule TypeScript types | `apps/budget/src/api/types.ts` |
| Monetary string utilities | `apps/budget/src/api/` or `@mrsmith/api-client` |
| Budget list page (`/budgets`) | `apps/budget/src/views/voci-di-costo/` |
| Budget detail page (`/budgets/:id`) | `apps/budget/src/views/voci-di-costo/` |
| Tabs component | `packages/ui/src/components/` |
| ExpandableRow component | `packages/ui/src/components/` |
| NumberInput component | `packages/ui/src/components/` |

**Phase 3 is complete when:** Both budget list and detail pages are fully functional with all 5 entity types (budget, user allocation, CC allocation, user rules, CC rules), the drill-down navigation works with deep linking, and the row-expansion approval rules pattern is polished.
