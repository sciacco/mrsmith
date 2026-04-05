# Budget Management — Migration Specification

**Application:** Budget Management  
**Audit source:** `apps/budget/APPSMITH_AUDIT.md` (Appsmith export)  
**API reference:** `docs/mistra-dist.yaml` (Mistra NG Internal API v2.7.14)  
**Spec date:** 2026-04-05  
**Status:** Complete — all expert questions resolved  
**Phase documents:** `budget-migspec-phaseA.md` through `budget-migspec-phaseD.md`

---

## 1. Summary

Budget Management is a manager-only internal tool for managing budgets, cost allocations (user and cost-center), approval rules, cost centers, and groups. It consumes the Arak REST API exclusively.

**Scope:** Replicate and improve the current Appsmith application. No self-service user views, no user CRUD, no role management.

**Key migration decisions:**
- Stripe-level clean design (not generic admin UI)
- Top horizontal tabs + contextual breadcrumbs (replaces Appsmith sidebar)
- Two-page drill-down for budget detail (list → full-width detail)
- Read-only detail panels + modal edit (app-wide pattern)
- Italian UI labels preserved for compatibility
- Small datasets — no pagination needed
- Shared query cache for reference data (users, cost centers, groups)
- BFF pattern: Go backend proxies Arak API 1:1, frontend calls `/api/...`
- Development starts with mock fixtures in Go handlers, UI-first with WOW effect

---

## 2. Development Strategy

### 2.1 Approach: UI-First with Mocked BFF

Development starts with **mocked data sources** and focuses on building the **WOW effect UI/UX** from the beginning. The goal is to establish high-quality interaction patterns, animations, and visual design that will be reused across all future mini-apps.

### 2.2 Architecture: Go BFF (Backend-For-Frontend)

The React frontend never calls Arak directly. All API calls go through the Go backend as a proxy:

```
React frontend → /api/budget/v1/... → Go backend → Arak REST API
```

**Contract:** The Go BFF exposes Arak endpoints **1:1** — same paths, same request/response shapes, same types. No reshaping or simplification at the BFF layer. The single source of truth for the API contract remains `docs/mistra-dist.yaml`.

**Why 1:1:** Maintaining a second contract at the BFF layer creates double maintenance. Small frontend concerns (envelope unwrapping, string→number formatting) belong in the frontend API client layer, not in Go.

**Mock phase:** Go handlers return static fixture data matching the real API response shapes.  
**Real phase:** Go handlers proxy to Arak. Frontend code does not change.

### 2.3 Build Order

Development follows the UI complexity ladder — simplest view first, establishing patterns that carry forward:

| Phase | View | Go Handlers | UI Focus |
|-------|------|-------------|----------|
| **1** | **Gruppi** | 3 handlers (list, details, CRUD) + Users (list) | Baseline CRUD pattern, master-detail, modal edit, navigation shell, WOW effect foundations (transitions, micro-interactions, typography, skeleton loaders) |
| **2** | **Centri di costo** | 3 handlers (list, details, CRUD) + Groups (list, reuse) | Refine master-detail pattern, multi-select forms, disable/enable flow, confirmation with impact preview |
| **3** | **Voci di costo** | 9 handlers (budget CRUD, allocations, approval rules) | Two-page drill-down, tabbed content, row-expansion for approval rules, most complex forms |
| **4** | **Home** | 2 handlers (reports) | Dashboard layout, parameterized reports, cross-view navigation (alert rows → budget detail) |

Each phase follows the same cycle:
1. Write Go handlers with fixture data
2. Build the UI with full WOW effect
3. Validate the interaction patterns
4. Move to next phase

### 2.4 WOW Effect from Day One

The UI/UX quality bar is set from Phase 1 (Gruppi). This means:

- **Transitions and animations** — smooth page/modal transitions, subtle hover states, content entrance animations
- **Loading states** — skeleton loaders (not spinners), optimistic UI where appropriate
- **Typography and spacing** — Stripe-caliber whitespace, font hierarchy, visual rhythm
- **Micro-interactions** — button feedback, toast notifications, form validation animations
- **Empty states** — designed empty states (not just "no data"), guiding the user to take action
- **Error states** — graceful error presentation with clear recovery actions

These patterns are built as reusable components in `@mrsmith/ui` from Phase 1 onward, so they carry to all future mini-apps without re-implementation.

### 2.5 Mock-to-Real Transition

The switch from fixtures to live Arak API happens **handler by handler**, not as a big bang:

1. Go handler currently returns fixture → change to proxy Arak
2. Frontend code: zero changes required
3. Test with real data, fix any edge cases
4. Repeat for next handler

This can happen per-view (all Gruppi handlers at once) or per-handler. No coordination required between frontend and backend.

---

## 3. Entity Catalog

### 2.1 Budget

**Purpose:** Primary financial entity — yearly budget with allocated spending limits.

**Operations:**

| Operation | Method | Endpoint | Notes |
|-----------|--------|----------|-------|
| List | GET | `/budget` | Auto-load on view |
| Details | GET | `/budget/{budget_id}` | On row select / route navigation |
| Create | POST | `/budget` | Body: `{ name, year }` |
| Edit | PUT | `/budget/{budget_id}` | Partial update — send only changed fields |
| Delete | DELETE | `/budget/{budget_id}` | With confirmation |
| Report: over % | GET | `/report/budget-used-over-percentage` | Home dashboard, `percentage` param as float |
| Report: unassigned | GET | `/report/unassigned-users` | Home dashboard, `enabled=true` |

**Fields:**

| Field | Type | Mutability | Notes |
|-------|------|------------|-------|
| id | integer (int64) | Read-only | Server-assigned |
| name | string | Create (required), Edit (optional) | — |
| year | integer | Create (required), Edit (optional) | — |
| limit | string | Read-only | Computed server-side via DB triggers (sum of allocations) |
| current | string | Read-only | Computed server-side |
| cost_center_budgets | cost_center-budget[] | Read-only | In details response only |
| user_budgets | user-budget[] | Read-only | In details response only |

**Derived:**
- `active` — computed frontend-side: `year === currentYear`

**Relationships:**
- Has many UserBudget, CostCenterBudget, UserBudgetApprovalRule, CostCenterBudgetApprovalRule

---

### 2.2 UserBudget

**Purpose:** Spending allocation linking a user to a budget.

**Operations:**

| Operation | Method | Endpoint | Notes |
|-----------|--------|----------|-------|
| Create | POST | `/budget/{budget_id}/user` | Body: `{ limit, user_id }` |
| Edit | PUT | `/budget/{budget_id}/user` | Body: `{ user_id, limit?, enabled? }` |

No standalone list (nested in budget details) or delete (disable only via `enabled: false`).

**Fields:**

| Field | Type | Mutability | Notes |
|-------|------|------------|-------|
| limit | string | Create (required), Edit (optional) | String type — format as decimal text |
| current | string | Read-only | — |
| user_id | integer (int64) | Create (required), Edit (required) | — |
| user_email | string | Read-only | Resolved server-side |
| budget_id | integer (int64) | Read-only | From URL path |
| enabled | boolean | Edit (optional) | Disable-only lifecycle |

---

### 2.3 CostCenterBudget

**Purpose:** Spending allocation linking a cost center to a budget.

**Operations:** Same pattern as UserBudget — Create (POST), Edit (PUT). No list or delete.

**Fields:** Same as UserBudget but with `cost_center: string` (name-based FK) instead of `user_id`.

---

### 2.4 UserBudgetApprovalRule

**Purpose:** Defines an approval threshold and approver for a user's spending within a budget.

**Operations:**

| Operation | Method | Endpoint |
|-----------|--------|----------|
| List | GET | `/approval-rules/user-budget` |
| Create | POST | `/approval-rules/user-budget` |
| Edit | PUT | `/approval-rules/user-budget/{rule_id}` |
| Delete | DELETE | `/approval-rules/user-budget/{rule_id}` |

**Fields:**

| Field | Type | Mutability | Notes |
|-------|------|------------|-------|
| id | integer (int64) | Read-only | — |
| threshold | string | Create (required), Edit (optional) | String type — decimal text |
| approver_id | integer (int64) | Create (required), Edit (optional) | — |
| approver_email | string | Read-only | — |
| budget_id | integer (int64) | Create (required), Edit (optional) | — |
| user_id | integer (int64) | Create (required), Edit (optional) | — |
| level | integer | Create (required), Edit (optional) | Select box: Livello 1, 2, 3 |
| send_email | boolean | Create (required), Edit (optional) | Toggle, default `true` |

---

### 2.5 CostCenterBudgetApprovalRule

**Purpose:** Same as UserBudgetApprovalRule but scoped to a cost center allocation.

**Operations:** Full CRUD, same endpoints pattern under `/approval-rules/cost-center-budget`.

**Fields:** Same as 2.4 but with `cost_center: string` instead of `user_id`.

---

### 2.6 CostCenter

**Purpose:** Organizational unit with manager, user membership, and group membership.

**Operations:**

| Operation | Method | Endpoint | Notes |
|-----------|--------|----------|-------|
| List | GET | `/cost-center` | — |
| Details | GET | `/cost-center/{name}` | URL-encoded name |
| Create | POST | `/cost-center` | Body: `{ name, manager_id, user_ids, group_names, enabled }` |
| Edit | PUT | `/cost-center/{name}` | All optional: `new_name`, `manager_id`, `user_ids`, `group_names`, `enabled` |
| Disable | PUT | `/cost-center/{name}` | `{ enabled: false }` — with confirmation showing affected users |
| Enable | PUT | `/cost-center/{name}` | `{ enabled: true }` — **new vs Appsmith** |

**Fields:**

| Field | Type | Mutability | Notes |
|-------|------|------------|-------|
| name | string | Create (required) | Primary identifier (not numeric) |
| new_name | string | Edit (optional) | Rename — include only if changed |
| manager_id | integer (int64) | Create (required), Edit (optional) | — |
| manager_email | string | Read-only | — |
| user_ids | integer[] | Create (required), Edit (optional) | — |
| group_names | string[] | Create (required), Edit (optional) | — |
| user_count | integer (int64) | Read-only | — |
| enabled | boolean | Create (required), Edit (optional) | — |

**Details response adds:** `manager` (full user), `users` (full user[]), `groups` (group-details[])

**Note:** Identified by `name` — API paths use `encodeURIComponent(name)`.

---

### 2.7 Group

**Purpose:** Named collection of users, referenced by cost centers.

**Operations:**

| Operation | Method | Endpoint | Notes |
|-----------|--------|----------|-------|
| List | GET | `/group` | — |
| Details | GET | `/group/{name}` | URL-encoded name |
| Create | POST | `/group` | Body: `{ name, user_ids }` |
| Edit | PUT | `/group/{name}` | Optional: `new_name`, `user_ids` |
| Delete | DELETE | `/group/{name}` | Hard delete — cascading handled by API |

**Fields:**

| Field | Type | Mutability | Notes |
|-------|------|------------|-------|
| name | string | Create (required) | Primary identifier |
| new_name | string | Edit (optional) | Rename — include only if changed |
| user_ids | integer[] | Create (required), Edit (optional) | — |
| user_count | integer (int64) | Read-only | — |

**Details response adds:** `users` (full user[])

---

### 2.8 User (reference entity, read-only)

**Purpose:** User records consumed as reference data across all views (dropdowns, membership tables, approver selection).

**Operations:** List only — `GET /user?enabled=true&disable_pagination=true`

**Fields:** `id`, `first_name`, `last_name`, `email`, `created`, `updated`, `state: { name, enabled }`, `role: { name, created, updated }`

**Scope:** Read-only. User CRUD managed elsewhere. Always filter `enabled=true`.

---

### 2.9 Entity Relationship Diagram

```
Budget (1) ──→ (N) UserBudget ──→ (N) UserBudgetApprovalRule
  │                  │
  │                  └── references User
  │
  ├──→ (N) CostCenterBudget ──→ (N) CostCenterBudgetApprovalRule
  │            │
  │            └── references CostCenter
  │
CostCenter ──→ (N) Users (membership)
           ──→ (N) Groups (membership)
           ──→ (1) User (manager)

Group ──→ (N) Users (membership)
      ←── referenced by CostCenter
```

---

## 4. View Specifications

### 4.1 Navigation Shell

**Pattern:** Top horizontal tabs + contextual breadcrumbs  
**Layout:** Slim top bar with: MrSmith logo (portal return, left) → section tabs (Home, Voci di costo, Centri di costo, Gruppi) → user area (right). Breadcrumb strip appears below top bar on drill-down pages only.

---

### 4.2 Home (Dashboard)

**User intent:** Monitor budget health — spot over-threshold budgets and unassigned users.

**Interaction pattern:** Read-only dashboard with parameterized report

**Sections:**
1. **Budget alert report** — Table showing budgets exceeding configurable percentage threshold
   - Number input for threshold (default 80.1%, per-session, no persistence)
   - Dynamic title: "Budget oltre il {n} %"
   - Columns: name, year, limit, current
   - **Rows are clickable → navigate to `/budgets/:id`**
   - Section hidden when no results (`items.length === 0`)
2. **Unassigned users report** — Table showing enabled users not assigned to any budget
   - Columns: user fields (extract `state.name`, `state.enabled` from nested object)
   - Section hidden when no results

**Data:**
- `GetAllBudgetsUsedOverPercentage` — `percentage` param sent as float (not text)
- `GetUnassignedArakInternalUser` — `enabled=true`
- Two independent queries, no cross-dependency

**Improvements vs Appsmith:**
- Error handling and loading states (Appsmith had none)
- Send `percentage` as number (Appsmith sent as text)
- Clickable budget rows → drill-down

---

### 4.3 Voci di costo — Budget List (`/budgets`)

**User intent:** Browse all budgets, create new budgets, navigate to detail.

**Interaction pattern:** List page (first page of two-page drill-down)

**Sections:**
1. **Budget table** — All budgets
   - Columns: name, year, limit, current, active (derived: `year === currentYear`)
   - Row click → navigate to `/budgets/:id`
   - Create button → modal: name (required), year (required) → POST
2. **Create modal** — Form for new budget

**Data:**
- `GetAllBudgets` — auto-load, `disable_pagination=true`

---

### 4.4 Voci di costo — Budget Detail (`/budgets/:id`)

**User intent:** Manage allocations and approval rules for a specific budget.

**Interaction pattern:** Full-width detail page with tabbed content and row-expansion

**Sections:**
1. **Breadcrumb** — "Voci di costo > {budget name}"
2. **Budget header** — Name, year, limit (read-only), current (read-only), active badge
   - Edit button → modal (partial update: name and/or year)
   - Delete button → confirmation → DELETE
3. **Allocation tabs** — Two tabs: "Utenti" / "Centri di costo"
4. **User allocations tab** — Table of user-budget allocations
   - Columns: user_email, limit, current, enabled
   - Create button → modal: select user, enter limit → POST
   - Edit action per row → modal: adjust limit, toggle enabled → PUT
   - **Row expansion (accordion)** → shows approval rules for this user-budget
5. **CC allocations tab** — Table of cost-center-budget allocations
   - Same pattern as user allocations but with cost_center instead of user
   - Row expansion → CC approval rules
6. **Approval rules (in row expansion)** — Ordered list of rules
   - Display: level, threshold, approver_email, send_email
   - Create → modal: level (select: Livello 1–3), threshold, approver (select), send_email (toggle, default true)
   - Edit → modal (pre-populated)
   - Delete → confirmation

**Data:**
- `GetBudgetDetails` — auto-load on route entry
- `GetAllUserBudgetApprovalRule` / `GetAllCostCenterBudgetApprovalRule` — on row expansion
- Reference data: Users list, Cost centers list (from shared cache)

**Improvements vs Appsmith:**
- Full-width detail with breathing room (was crammed in single page)
- Deep-linkable URL (`/budgets/:id`)
- Row expansion for approval rules (was cascading table selection)
- 3 approval levels (was 2)
- `send_email` user-configurable (was hardcoded true)
- Proper error handling and loading states
- Partial updates on edit (send only changed fields)

---

### 4.5 Centri di costo (Cost Centers)

**User intent:** Manage cost centers — create, edit membership, assign manager, disable/enable.

**Interaction pattern:** Master-detail with read-only side panel + modal edit

**Sections:**
1. **Cost center table (master)** — Columns: name, enabled, manager_email, user_count
   - Row select → fetch details → populate side panel
   - Create button → modal
2. **Detail panel (side)** — Read-only: name, manager, active status
3. **Member list** — Users in selected cost center
4. **Action buttons** — Edit, Disable, Enable (new)
   - Edit/Disable/Enable disabled when no selection
   - Edit → modal: name, manager (select), users (multi-select), groups (multi-select)
   - Disable → confirmation modal showing affected users → `{ enabled: false }`
   - Enable → `{ enabled: true }` (new vs Appsmith)
5. **Create modal** — name, manager_id, user_ids, group_names, enabled

**Data:**
- `GetAllCostCenters` — auto-load (shared cache)
- `GetCostCenterDetails` — on row select
- Reference data: Users list, Groups list (from shared cache)

**Improvements vs Appsmith:**
- Enable action (was disable-only)
- Error handling (Appsmith had none)
- Correct validation wiring (Appsmith had bug: wrong widget reference)

---

### 4.6 Gruppi (Groups)

**User intent:** Manage user groups — create, rename, manage membership, delete.

**Interaction pattern:** Master-detail with read-only side panel + modal edit (same as Cost Centers)

**Sections:**
1. **Group table (master)** — Columns: name, user_count
   - Row select → fetch details → populate side panel
   - Create button → modal
2. **Detail panel (side)** — Read-only: name
3. **Member list** — Users in selected group
4. **Action buttons** — Edit, Delete
   - Edit → modal: new_name (optional), user_ids (multi-select, pre-populated)
   - Delete → confirmation → hard DELETE
5. **Create modal** — name, user_ids (multi-select)

**Data:**
- `GetAllGroups` — auto-load (shared cache)
- `GetGroupDetails` — on row select
- Reference data: Users list (from shared cache)

**Improvements vs Appsmith:**
- Consistent error handling (Appsmith was partial — only GetGroupDetails and NewGroup had `.catch()`)
- Form reset on modal close (Appsmith had stale data bug)

---

## 5. Logic Allocation

### Backend (no changes — already in place)
- Budget `limit` computation via DB triggers
- All API validation, cascading deletes, data integrity
- Email delivery triggered by `send_email` flag
- Audit logging (separate implementation, tracked in `docs/TODO.md`)

### Frontend — Business Rules
| Rule | Implementation |
|------|---------------|
| Budget "active" flag | Derive at render: `year === new Date().getFullYear()` |
| `send_email` default | Form toggle, default `true` |
| Approval level options | Pre-filled select: Livello 1, 2, 3 |
| Alert threshold default | Number input, default 80.1, per-session |
| Enabled-only users | Always send `enabled=true` query param |

### Frontend — Orchestration
| Pattern | Implementation |
|---------|---------------|
| Cascading data fetch | Route-based: budget list on view load, details on `/budgets/:id`, rules on row expansion |
| Post-mutation refresh | Query cache invalidation (see invalidation map below) |
| Modal pre-population | Pass selected entity as form initial values |
| Destructive action confirmation | Confirm dialog; CC disable shows affected users |
| Reference data sharing | Shared query cache: Users, CostCenters, Groups fetched once at app level |
| Conditional section render | Hide report sections when `items.length === 0` |

### Frontend — Formatting
| Concern | Implementation |
|---------|---------------|
| Monetary values | Parse/format `limit`, `current`, `threshold` as decimal strings. Shared formatter in `@mrsmith/api-client`. |
| Paginated response | Unwrap `{ items }` in API client layer. All queries use `disable_pagination=true`. |
| Name-based URL paths | `encodeURIComponent(name)` for CostCenter and Group API calls |
| Optional rename | Include `new_name` in PUT body only if user entered a value |
| Nested user fields | Access `state.enabled`, `state.name` via dot-path (replace Appsmith IIFE pattern) |

### Not Ported (bugs, dead code, workarounds)
- Trailing comma JSON in approval rule edit bodies (bug)
- Wrong validation widget reference in CC edit form (bug)
- Commented-out conditional spread in budget edit (dead code)
- `utils.test` method (dead code)
- Hidden input fields for passing IDs (Appsmith workaround)

---

## 6. API Contract Summary

### Authentication
- OAuth2 with `openid` + `profile` scopes via Keycloak
- All endpoints require authentication

### Response Envelope
All list endpoints return: `{ total_number, current_page, total_pages, items: T[] }`

### String-Typed Monetary Values
`limit`, `current`, `threshold` are **string** in the API (not number). Likely decimal values serialized as text for precision. Frontend must:
- Display: parse as number for formatting
- Send: serialize as string in POST/PUT bodies

### Endpoints (15 in scope)

| # | Endpoint | Methods | View |
|---|----------|---------|------|
| 1 | `/arak/budget/v1/budget` | GET, POST | Voci di costo |
| 2 | `/arak/budget/v1/budget/{budget_id}` | GET, PUT, DELETE | Voci di costo |
| 3 | `/arak/budget/v1/budget/{budget_id}/user` | POST, PUT | Voci di costo |
| 4 | `/arak/budget/v1/budget/{budget_id}/cost-center` | POST, PUT | Voci di costo |
| 5 | `/arak/budget/v1/approval-rules/user-budget` | GET, POST | Voci di costo |
| 6 | `/arak/budget/v1/approval-rules/user-budget/{rule_id}` | PUT, DELETE | Voci di costo |
| 7 | `/arak/budget/v1/approval-rules/cost-center-budget` | GET, POST | Voci di costo |
| 8 | `/arak/budget/v1/approval-rules/cost-center-budget/{rule_id}` | PUT, DELETE | Voci di costo |
| 9 | `/arak/budget/v1/cost-center` | GET, POST | Centri di costo |
| 10 | `/arak/budget/v1/cost-center/{name}` | GET, PUT | Centri di costo |
| 11 | `/arak/budget/v1/group` | GET, POST | Gruppi |
| 12 | `/arak/budget/v1/group/{name}` | GET, PUT, DELETE | Gruppi |
| 13 | `/arak/budget/v1/report/budget-used-over-percentage` | GET | Home |
| 14 | `/arak/budget/v1/report/unassigned-users` | GET | Home |
| 15 | `/arak/users-int/v1/user` | GET | All (reference) |

### Excluded Endpoints
| Endpoint | Reason |
|----------|--------|
| `GET /budget-for-user` | Manager-only app, no self-service (Q9) |
| `POST/PUT/DELETE /user` | User CRUD managed elsewhere (Q7) |
| `GET /role` | Not relevant to budget app (Q8) |

---

## 7. Integrations and Data Flow

### External Systems
| System | Purpose |
|--------|---------|
| Arak REST API | Single data source for all operations |
| Keycloak | OAuth2/OIDC authentication |

### Shared Reference Data
| Data | Endpoint | Shared across | Cache strategy |
|------|----------|---------------|----------------|
| Users (enabled) | `GET /user?enabled=true` | All CRUD views | Fetch on app load, rarely invalidated |
| Cost centers | `GET /cost-center` | Voci di costo, Centri di costo | Invalidate after CC mutations |
| Groups | `GET /group` | Centri di costo, Gruppi | Invalidate after group mutations |

### Data Invalidation Map

| Mutation | Invalidate |
|----------|------------|
| NewBudget | Budget list |
| EditBudget | Budget list, Budget details |
| DeleteBudget | Budget list |
| NewUserBudget | Budget details |
| UpdateUserBudget | Budget details |
| NewCostCenterBudget | Budget details |
| UpdateCostCenterBudget | Budget details |
| NewRuleUser / EditRuleUser / DeleteRuleUser | User approval rules list |
| NewRuleCC / EditRuleCC / DeleteRuleCC | CC approval rules list |
| NewCostCenter / EditCostCenter / Disable / Enable | Cost center list (shared), Cost center details |
| NewGroup / UpdateGroup / DeleteGroup | Group list (shared), Group details |

### No Hidden Automation
No polling, WebSockets, timers, or background processes. All data fetching is user-initiated.

---

## 8. Constraints and Non-Functional Requirements

### Security
- OAuth2/OIDC via Keycloak (existing `@mrsmith/auth-client`)
- No client-side authorization checks — API enforces access control
- Audit logging out of scope (tracked in `docs/TODO.md`)

### Data
- Small datasets — no pagination required (`disable_pagination=true`)
- All monetary values are strings (decimal precision)

### UX
- Stripe-level clean design
- Italian UI labels (hardcoded, no i18n framework)
- Manager-only app (power users, small user base)
- Consistent error handling and loading states across all views (Appsmith had none/partial)

### Architecture
- Independent Vite+React app under `apps/budget/`
- Go BFF under `backend/internal/budget/` — proxies Arak API 1:1
- Shared packages: `@mrsmith/ui`, `@mrsmith/auth-client`, `@mrsmith/api-client`
- Frontend calls `/api/budget/v1/...` → Go backend → Arak (or fixtures during mock phase)
- Frontend Keycloak config served by backend via `GET /config` (unprotected, public client data)
- BFF → Arak uses client credentials grant (service-to-service token), NOT user token forwarding
- API client should handle: response envelope unwrapping, monetary string formatting, `disable_pagination=true` default

---

## 9. Open Questions and Deferred Decisions

| Item | Status | Tracked in |
|------|--------|------------|
| Audit logging for budget/approval changes | Out of scope — separate implementation | `docs/TODO.md` |
| Budget year-end lifecycle (archive/rollover) | Unresolved domain decision | `docs/TODO.md` |

---

## 10. Changes vs Appsmith (New Behavior)

| Change | Rationale | Decision ref |
|--------|-----------|-------------|
| Top tabs + breadcrumbs (replaces sidebar) | Stripe design alignment, maximize table width | Q15 |
| Two-page budget drill-down | Stripe design, deep-linking, reduce complexity | Q13 |
| Row expansion for approval rules | Replace cascading table selection, cleaner UX | Q13 |
| 3 approval levels (was 2) | DB supports it, business requested | Q3, Q17 |
| `send_email` toggle (was hardcoded true) | Make business rule explicit | Q2 |
| Cost center Enable action (was disable-only) | Useful addition, API already supports it | Q4 |
| Home alert rows → budget detail | Natural consequence of drill-down design | Q18 |
| Read-only panels + modal edit (app-wide) | Stripe pattern, clean view/edit separation | Q14 |
| Go BFF proxy (1:1 Arak) | Single API contract, mock→real transition without frontend changes | Dev strategy |
| UI-first with WOW effect | Establish reusable design patterns for all future mini-apps | Dev strategy |
| Shared query cache for reference data | Eliminate per-page duplication | Phase D |
| Consistent error handling | Appsmith had none/partial | Phase B/C |
| Proper type handling (float for %, string for money) | Fix Appsmith text/number mismatches | Phase C |

---

## 11. Acceptance Notes

### What the audit proved directly
- 4 pages, 1 datasource, 32+ queries, 8 entities in use
- All API endpoints and their parameters
- Widget structure, data bindings, and event flows
- 3 bugs, 4 dead code items, inconsistent patterns
- Embedded business rules and their locations

### What the expert confirmed
- Budget `limit` is server-computed (DB triggers)
- `send_email` should be user-configurable (default true)
- 3 approval levels (up from 2)
- Cost center re-enable is desired
- Manager-only app — no self-service, no user CRUD, no roles
- Small datasets — no pagination needed
- Hard delete for groups is intentional (cascading is API business)
- Keep "Voci di costo" name for compatibility
- Stripe-level design: top tabs, drill-down, read-only panels + modal edit

### What still needs validation
- Approval rule level sequencing constraints (server-side) — currently frontend limits to 1–3 select
- Budget year-end lifecycle — deferred domain decision
- Audit logging implementation — separate scope
