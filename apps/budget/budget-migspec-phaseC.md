# Budget Management — Phase C: Logic Placement

**Source:** `apps/budget/APPSMITH_AUDIT.md` + Phase A/B resolved decisions  
**Date:** 2026-04-05  
**Status:** Draft — awaiting expert review

---

## Logic Inventory

All non-trivial logic extracted from the audit, classified by type and recommended placement.

---

### 1. Domain Logic (business rules)

| # | Logic | Current location | Current behavior | Recommended placement | Notes |
|---|-------|-----------------|------------------|----------------------|-------|
| D1 | **Budget "active" flag** | Voci di costo, computed column | `currentRow.year == new Date().getFullYear()` | **Frontend** | Pure derivation from `year` field — no server state needed. Compute at render time. |
| D2 | **Budget limit computation** | Not in Appsmith — DB triggers | Sum of allocations, computed server-side | **Backend** (already there) | Phase A Q1 resolved: stays server-side. Frontend treats `limit` as read-only. |
| D3 | **`send_email` default** | Voci di costo, hardcoded `true` in POST bodies | Always `true`, never user-configurable | **Frontend** (form default) | Phase A Q2 resolved: expose as toggle, default `true`. Default is a UI concern, API accepts whatever is sent. |
| D4 | **Approval level assignment** | Voci di costo, hardcoded level 1/2 in modals | Two fixed levels | **Frontend** (dynamic form) | Phase A Q3 resolved: support N levels. Frontend auto-increments level on "add". No server validation of level order implied by audit. |
| D5 | **Percentage threshold for alerts** | Home, `i_percent` widget default 80.1 | User-adjustable per session, sent as API param | **Frontend** (input with default) | Phase A Q12 resolved: per-session, no persistence. |
| D6 | **Only enabled users in dropdowns** | All pages, query param `enabled=true` | Hard filter on API call | **Frontend** (query param) | Phase A Q6 resolved: keep this behavior. Frontend always sends `enabled=true`. |

**Flags:**
- ✅ **Resolved:** Level input is a pre-filled select box with 3 options (Livello 1, 2, 3). Sequential, no free-form. Expands from Appsmith's 2 to 3 levels.

---

### 2. Orchestration Logic (data flow, sequencing, refresh)

| # | Logic | Current location | Current behavior | Recommended placement | Notes |
|---|-------|-----------------|------------------|----------------------|-------|
| O1 | **Cascading data fetch on budget select** | Voci di costo | Row select → `GetBudgetDetails` → populates allocation tables → row select → fetch approval rules | **Frontend** (query/state management) | Natural fit for React state + data fetching. Trigger detail fetch on selection change. |
| O2 | **Post-mutation refresh chains** | All CRUD pages | `.then()` chains: create/edit/delete → re-fetch list and/or details | **Frontend** (cache invalidation) | Appsmith uses inconsistent manual refresh. New frontend should use query invalidation (e.g., invalidate budget list after create, invalidate details after allocation edit). |
| O3 | **Modal open → pre-populate from selection** | Voci di costo, Centri di costo, Gruppi | Edit modals read current row data via hidden inputs or widget bindings | **Frontend** (form state) | Pass selected entity data to modal form as initial values. No hidden input hacks needed. |
| O4 | **Confirmation before destructive action** | Centri di costo (disable shows affected users), Gruppi (delete confirmation) | Modal with impact preview before confirm | **Frontend** (UI pattern) | Standard confirm dialog. Centri di costo disable modal should fetch/show affected users before confirm. |
| O5 | **Duplicated queries across pages** | GetAllUsers, GetAllCostCenters, GetAllGroups fetched per page | Same API call repeated on each page load | **Frontend** (shared data layer) | Deduplicate via shared query cache. These are reference data used across views. |
| O6 | **Container visibility based on `total_number`** | Home dashboard | Report containers hidden when `total_number == 0` | **Frontend** (conditional render) | Simple: render section only if `items.length > 0`. |

---

### 3. Presentation Logic (formatting, display)

| # | Logic | Current location | Current behavior | Recommended placement | Notes |
|---|-------|-----------------|------------------|----------------------|-------|
| P1 | **IIFE pattern for nested `state.enabled` / `state.name`** | Home (`Table1`), repeated 10+ times | `(function() { return currentRow.state.enabled })()` | **Frontend** (column accessor) | Replace IIFE with simple dot-path accessor or column definition. Trivial in React table. |
| P2 | **`String()` coercion on limit/threshold** | Voci di costo POST/PUT bodies | `String(inputValue)` before sending | **Frontend** (serialization) | API expects strings for `limit`, `current`, `threshold`. Format numbers as strings before POST/PUT. Define a shared formatter. |
| P3 | **Derived "user_count" display** | Centri di costo, Gruppi tables | Shown in list table, comes from API response | **None** (API provides it) | Already a response field. No frontend computation needed. |
| P4 | **Multi-select pre-population** | Centri di costo, Gruppi edit modals | `t_user_group.tableData.map(item => item.id)` to pre-select current members | **Frontend** (form initialization) | Map detail response users/groups to ID arrays for multi-select default values. |
| P5 | **Conditional spread for optional rename** | Centri di costo, Gruppi edit bodies | `...(name.length > 0 ? {new_name: name} : {})` | **Frontend** (request builder) | Only include `new_name` in PUT body if user entered a new name. Clean pattern — keep it. |
| P6 | **`encodeURIComponent()` for name-based paths** | Centri di costo, Gruppi | URL path uses name as ID, encoded for safety | **Frontend** (URL construction) | Keep. Cost centers and groups use name as identifier in API paths. |
| P7 | **Dynamic title interpolation** | Home, `Text3` | "Budget oltre il {{i_percent.text}} %" | **Frontend** (template/JSX) | Simple string interpolation in component. |

---

### 4. Logic to NOT port (bugs, dead code, workarounds)

| # | Logic | Current location | Why not port |
|---|-------|-----------------|--------------|
| X1 | **Trailing comma JSON in EditRuleUser/EditRuleCC** | Voci di costo | Bug — syntax error. Build proper request objects. |
| X2 | **Wrong validation reference (`i_new_cc_name` instead of `i_edit_cc_name`)** | Centri di costo | Bug — wire validation to correct form field. |
| X3 | **Commented-out conditional spread in Editbudget** | Voci di costo | Dead code. `budget-edit` has all optional fields — send only changed fields (partial update). |
| X4 | **`utils.test` method** | Centri di costo JSObject | Dead code — not referenced by any widget. |
| X5 | **Hidden input fields for passing IDs** | Voci di costo modals | Appsmith workaround. React passes data via props/state. |
| X6 | **`disable_pagination=true` everywhere** | All pages | Phase A Q11 resolved: small datasets, no pagination. But don't hardcode this as a magic param — pass it cleanly in the API client config. |

---

## Summary by Placement

### Backend (already there, no changes needed)
- Budget `limit` computation via DB triggers (D2)
- All API validation, cascading deletes, data integrity
- Approval rule level constraints (if any — Q17 pending)

### Frontend — Domain/Business
- Budget "active" derivation from year (D1)
- `send_email` default `true` on approval rule forms (D3)
- N-level approval chain UI with auto-increment (D4)
- Alert threshold input with 80.1 default (D5)
- `enabled=true` filter on user queries (D6)

### Frontend — Orchestration
- Cascading fetch on selection (O1)
- Query cache invalidation after mutations (O2)
- Modal form pre-population from selection (O3)
- Confirmation dialogs with impact preview (O4)
- Shared query cache for reference data (O5)
- Conditional section rendering (O6)

### Frontend — Presentation/Formatting
- Nested object accessors for user state (P1)
- Number-to-string serialization for API (P2)
- Multi-select default values from detail response (P4)
- Conditional `new_name` in PUT bodies (P5)
- `encodeURIComponent` for name-based paths (P6)
- String interpolation for dynamic titles (P7)

### Shared utility (candidate for `@mrsmith/api-client`)
- **Monetary string formatting** — parse/format `limit`, `current`, `threshold` strings consistently (P2)
- **Paginated response unwrapping** — all list endpoints return `{ total_number, current_page, total_pages, items }`. Extract `items` in the API client layer.

---

## Expert Questions (Phase C)

| # | Question | Context |
|---|----------|---------|
| Q17 | ✅ **Resolved.** Limit to pre-filled sequential select box with levels 1–3 ("Livello 1", "Livello 2", "Livello 3"). No free-form input. Expands Appsmith's hardcoded 2 levels to 3, without full N-level dynamic UI for now. | Approval rule level input |

---

**Next:** After expert answer, proceed to **Phase D: Integration and Data Flow**.
