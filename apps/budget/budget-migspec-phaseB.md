# Budget Management — Phase B: UX Pattern Map

**Source:** `apps/budget/APPSMITH_AUDIT.md` + Phase A resolved decisions  
**Date:** 2026-04-05  
**Status:** Draft — awaiting expert review

---

## View Inventory

4 views extracted from audit, mapped to interaction patterns.

---

### View 1: Home (Dashboard)

**User intent:** Monitor budget health at a glance — spot budgets exceeding a threshold and users without budget allocation.

**Interaction pattern:** Read-only dashboard with parameterized report

**UI Sections:**

| Section | Widgets (Appsmith) | Purpose | Data source |
|---------|--------------------|---------|-------------|
| Header | `Text1` | Page title "BACKEND BUDGET MANAGEMENT" | Static |
| Budget alert report | `report_budget_over_80_perc` container | Budgets exceeding configurable % threshold | GetBudgetOverPercent |
| — Threshold input | `i_percent` (number input, default 80.1) | User adjusts alert percentage | Local param → API `percentage` |
| — Alert title | `Text3` | Dynamic: "Budget oltre il {{%}} %" | Bound to input |
| — Alert table | `Table2` | Columns: name, year, limit, current | API response |
| Unassigned users report | `report_unassigned_users` container | Users not assigned to any budget | UnassignedUsers |
| — Title | `Text2` | Static: "Utenti non assegnati a nessun Budget" | Static |
| — Users table | `Table1` | Derived columns extracting `state.enabled`, `state.name` | API response |

**Visibility logic:**
- Each report container hidden when `total_number == 0` (no results = no section)

**Actions:** None — no navigation, no mutations, no row selection

**Flags:**
- ⚠ No error handling or loading states
- ✅ Default 80.1% — per-session input, no saved preferences needed.
- ⚠ `percentage` sent as text but API expects `float` — new frontend must send as number

---

### View 2: Voci di costo (Cost Items / Budget Management)

**User intent:** Full budget lifecycle management — create budgets, allocate spending to users and cost centers, define approval chains.

**Interaction pattern:** Master-detail-detail (three-level cascading selection)

**UI Sections:**

| Section | Widgets (Appsmith) | Purpose | Pattern |
|---------|--------------------|---------|---------|
| Budget list (master) | `t_budget` | All budgets with computed "active" column | Table with row selection |
| Budget actions | `bg_budget` (Refresh, Edit, Delete) | CRUD triggers for selected budget | Button group |
| User allocations (detail 1) | `t_user_budget` | Users allocated to selected budget with limits | Detail table, appears on budget select |
| Cost center allocations (detail 2) | `t_cost_center_budget` | Cost centers allocated to selected budget | Detail table, appears on budget select |
| User approval rules (detail 3) | `t_user_budget_app_rule` | Approval chain for selected user-budget | Sub-detail table, appears on user-budget select |
| CC approval rules (detail 3) | `t_cost_center_budget_app_rule` | Approval chain for selected CC-budget | Sub-detail table, appears on CC-budget select |
| CRUD modals (12+) | Various `m_*` modals | Create/edit forms for budgets, allocations, rules | Modal dialogs with form inputs |

**Cascading selection flow:**
```
t_budget row select
  → GetBudgetDetails
    → populates t_user_budget, t_cost_center_budget
      → t_user_budget row select → GetAllRuleUser → t_user_budget_app_rule
      → t_cost_center_budget row select → GetAllRuleCC → t_cost_center_budget_app_rule
```

**Actions per level:**

| Level | Create | Edit | Delete/Disable |
|-------|--------|------|----------------|
| Budget | ✅ NewBudget | ✅ Editbudget | ✅ DeleteBudget |
| User allocation | ✅ NewUserBudget | ✅ UpdateUserBudget | ❌ (disable only via edit) |
| CC allocation | ✅ NewCostCenterBudget | ✅ UpdateCostCenterBudget | ❌ (disable only via edit) |
| User approval rule | ✅ NewRuleUser | ✅ EditRuleUser | ✅ DeleteRuleUser |
| CC approval rule | ✅ NewRuleCC | ✅ EditRuleCC | ✅ DeleteRuleCC |

**Flags:**
- ⚠ Most complex view: 19 queries, 12+ modals, 30+ form inputs
- ⚠ No loading indicators during async operations
- ⚠ Delete button initially disabled with no visible enable logic
- ⚠ Inconsistent refresh strategies after mutations (`.then()` chaining)
- ⚠ Bugs: trailing comma JSON in EditRuleUser/EditRuleCC; commented-out conditional logic in Editbudget
- ⚠ **Mixed pattern:** This single page handles 5 entity types across 3 cascading levels. **Q13: Should this remain one view, or be split? For example: budget CRUD as one view, allocation + approval rules as a separate detail view navigated from budget selection?**

---

### View 3: Centri di costo (Cost Centers)

**User intent:** Manage cost centers — create, edit membership (users + groups), assign manager, disable/enable.

**Interaction pattern:** Master-detail with side panel

**UI Sections:**

| Section | Widgets (Appsmith) | Purpose | Pattern |
|---------|--------------------|---------|---------|
| Cost center list (master) | `t_cost_center` | All cost centers: name, enabled, manager_email, user_count | Table with row selection |
| Detail panel | `c_cost_center_details` | Selected CC: name, manager (select), active (switch) — all read-only | Side panel |
| User list (detail) | `t_user_cost_center` | Users in selected CC (from `utils.user_list`) | Detail table |
| Actions | `bg_user_cost_center` (Refresh, Edit, Disable) | Edit/Disable disabled when CC already disabled | Button group |
| Create modal | `m_new_cc` | New cost center form | Modal |
| Edit modal | `m_edit_cc` | Edit CC: name, manager, users, groups | Modal |
| Disable modal | `m_disable_cc` | Confirmation with affected users table | Modal |

**Actions:**

| Action | Operation | Notes |
|--------|-----------|-------|
| Create | POST NewCostCenter | manager_id, user_ids, group_names, enabled |
| Edit | PUT EditCostCenter | Conditional `new_name` spread; pre-populated from details |
| Disable | PUT DisableCostCenter | `{enabled: false}` — confirmation dialog shows impact |
| Enable | — | **New in migration** (Phase A, Q4) — not in Appsmith |

**Flags:**
- ⚠ Bug: `i_edit_cc_name` validation references wrong widget (`i_new_cc_name`)
- ⚠ No error handling in query chains
- ⚠ Detail panel is read-only — edit only via modal
- ⚠ JSObject `utils.user_list` transforms data on load; `utils.test` is unused dead code
- ⚠ **Q14: Should the detail panel remain read-only (view + modal edit), or become inline-editable?**

---

### View 4: Gruppi (Groups)

**User intent:** Manage user groups — create, rename, manage membership, delete.

**Interaction pattern:** Master-detail with side panel (same as Cost Centers)

**UI Sections:**

| Section | Widgets (Appsmith) | Purpose | Pattern |
|---------|--------------------|---------|---------|
| Group list (master) | `t_group` | All groups: name, user_count | Table with row selection |
| Detail panel | `c_group_details` | Selected group name (read-only) | Side panel |
| Member list (detail) | `t_user_group` | Users in selected group | Detail table |
| Actions | `bg_user_group` (Refresh, Edit, Delete) | — | Button group |
| Create modal | `m_new_group` | name + user multi-select | Modal |
| Edit modal | `m_update_group` | rename + user multi-select (pre-populated) | Modal |
| Delete modal | `m_delete_group` | Confirmation | Modal |

**Actions:**

| Action | Operation | Notes |
|--------|-----------|-------|
| Create | POST NewGroup | name, user_ids |
| Edit | PUT UpdateGroup | Conditional `new_name` spread; multi-select pre-populated |
| Delete | DELETE DeleteGroup | Hard delete (cascading handled by API, Phase A Q5) |

**Flags:**
- ⚠ Only page with partial error handling (GetGroupDetails and NewGroup have `.catch()`)
- ⚠ UpdateGroup and DeleteGroup fail silently
- ⚠ `i_update_group_name` has no `resetOnSubmit` — stale data on modal reopen
- ⚠ Simplest CRUD page — natural candidate for establishing the baseline pattern

---

## Cross-View Observations

### 1. Shared data across views

| Data | Used in views | Appsmith query name |
|------|---------------|---------------------|
| Users list | Voci di costo, Centri di costo, Gruppi | `GetAllUsers` (duplicated per page) |
| Cost centers list | Voci di costo, Centri di costo | `GetAllCostCenters` (duplicated) |
| Groups list | Centri di costo, Gruppi | `GetAllGroups` (duplicated) |

**Observation:** Appsmith duplicates these queries per page. New frontend should share this data (cache, store, or query deduplication).

### 2. Consistent interaction patterns

| Pattern | Views using it | Notes |
|---------|---------------|-------|
| Master table → row select → detail | All 4 | Core navigation pattern |
| Modal-based CRUD | Voci di costo, Centri di costo, Gruppi | Create/Edit/Delete via modal dialogs |
| Button group actions | Voci di costo, Centri di costo, Gruppi | Refresh + Edit + Delete/Disable |
| Conditional button disable | Centri di costo (disabled CC), Voci di costo (delete) | Context-sensitive actions |

### 3. Inconsistencies to normalize

| Inconsistency | Detail |
|----------------|--------|
| Error handling | Gruppi has partial `.catch()`, others have none |
| Refresh strategy | Varies per page — some re-run all queries, some only the affected one |
| Naming | `Editbudget` (lowercase b), `UpdateGroup` vs `EditCostCenter` — inconsistent verb choice |
| Dead code | `utils.test` in Centri di costo, commented-out logic in Voci di costo |

### 4. Navigation model

Appsmith uses sidebar navigation between the 4 pages. No cross-page links, no deep linking, no breadcrumbs. The only inter-page relationship is shared reference data (users, cost centers, groups).

---

## Expert Questions (Phase B)

| # | Question | Context |
|---|----------|---------|
| Q12 | ✅ **Resolved.** Per-session input is sufficient. No saved preferences needed. | Home view |
| Q13 | Should "Voci di costo" remain one view with 3 cascading levels, or be split into budget list + detail view? It currently handles 5 entity types, 19 queries, 12+ modals. | Voci di costo complexity |
| Q14 | Should detail panels (cost center, group) remain read-only with modal edit, or become inline-editable? | Centri di costo, Gruppi detail panels |
| Q15 | Should the sidebar navigation model be preserved, or should the new app use a different layout (e.g., tabs, breadcrumb drill-down)? | App-wide navigation |
| Q16 | The page name "Voci di costo" literally means "cost items" but the page manages budgets and their allocations. Should it be renamed to something clearer (e.g., "Budget", "Gestione Budget")? | Italian UI naming |

---

**Next:** After expert answers, proceed to **Phase C: Logic Placement**.
