# Budget Management — Phase A: Entity-Operation Model

**Source:** `apps/budget/APPSMITH_AUDIT.md`  
**Date:** 2026-04-05  
**Status:** Draft — awaiting expert review

---

## Extracted Entities

### 1. Budget

**Role:** Primary entity  
**Evidence:** Voci di costo page (master table `t_budget`), Home page (report tables)

**Operations (from audit):**

| Operation | Verb | Appsmith Query | API operationId | Evidence |
|-----------|------|----------------|-----------------|----------|
| List all | GET | GetAllBudgets | GetAllBudgets | Auto-load, Voci di costo |
| Get details | GET | GetBudgetDetails | GetBudgetDetails | On row select |
| Create | POST | NewBudget | NewBudget | Modal form |
| Edit | PUT | Editbudget | EditBudget | Modal form |
| Delete | DELETE | DeleteBudget | DeleteBudget | Button group action |
| Report: over % | GET | GetBudgetOverPercent | GetAllBudgetsUsedOverPercentage | Home dashboard |
| Report: unassigned users | GET | UnassignedUsers | GetUnassignedArakInternalUser | Home dashboard |

**Fields (from API spec `budget` + `budget-details` + `budget-new` + `budget-edit`):**

| Field | Type | In response | In create | In edit | Notes |
|-------|------|-------------|-----------|---------|-------|
| id | integer (int64) | yes | — | — | Server-assigned |
| name | string | yes | required | optional | — |
| year | integer | yes | required | optional | — |
| limit | **string** | yes | — | — | ⚠ String, not number. Not in create body — see question Q1 |
| current | **string** | yes | — | — | ⚠ String, not number. Read-only (computed server-side) |
| cost_center_budgets | cost_center-budget[] | details only | — | — | Nested in details response |
| user_budgets | user-budget[] | details only | — | — | Nested in details response |

**Computed/derived (frontend-only):**
- `active` — `currentRow.year == new Date().getFullYear()` (Appsmith computed column)

**Relationships:**
- Has many **UserBudget** (via `budget_id`)
- Has many **CostCenterBudget** (via `budget_id`)
- Has many **UserBudgetApprovalRule** (via `budget_id`)
- Has many **CostCenterBudgetApprovalRule** (via `budget_id`)

**Flags:**
- ✅ `budget-new` requires only `name` and `year` — no `limit`. **Resolved:** `limit` is computed server-side via database triggers (sum of allocations). The field is read-only in the API response. No frontend input needed.
- ⚠ `budget-edit` has all fields optional (partial update). Appsmith sends both fields unconditionally (commented-out conditional logic noted as bug).
- ⚠ Appsmith never uses `search_string` or `year` query params on GetAllBudgets.

---

### 2. UserBudget

**Role:** Join entity (Budget ↔ User allocation)  
**Evidence:** Voci di costo, `t_user_budget` table

**Operations:**

| Operation | Verb | Appsmith Query | API operationId |
|-----------|------|----------------|-----------------|
| Create | POST | NewUserBudget | NewUserBudget |
| Edit | PUT | UpdateUserBudget | EditUserBudget |

**Note:** No standalone list or delete operations — user budgets are returned nested in `GetBudgetDetails` response as `user_budgets[]`.

**Fields (from API spec `user-budget` response + `user-budget-upsert` + `user-budget-edit`):**

| Field | Type | In response | In create | In edit | Notes |
|-------|------|-------------|-----------|---------|-------|
| limit | **string** | yes | required | optional | ⚠ String type |
| current | **string** | yes | — | — | Read-only |
| user_id | integer (int64) | yes | required | required | — |
| user_email | string | yes | — | — | Read-only (resolved server-side) |
| budget_id | integer (int64) | yes | — | — | From URL path param |
| enabled | boolean | yes | — | optional | — |

**Relationships:**
- Belongs to **Budget** (via `budget_id`)
- References **User** (via `user_id`)
- Has many **UserBudgetApprovalRule**

---

### 3. CostCenterBudget

**Role:** Join entity (Budget ↔ CostCenter allocation)  
**Evidence:** Voci di costo, `t_cost_center_budget` table

**Operations:**

| Operation | Verb | Appsmith Query | API operationId |
|-----------|------|----------------|-----------------|
| Create | POST | NewCostCenterBudget | NewCostCenterBudget |
| Edit | PUT | UpdateCostCenterBudget | EditCostCenterBudget |

**Note:** No standalone list or delete — nested in `GetBudgetDetails` as `cost_center_budgets[]`.

**Fields (from API spec):**

| Field | Type | In response | In create | In edit | Notes |
|-------|------|-------------|-----------|---------|-------|
| limit | **string** | yes | required | optional | ⚠ String type |
| current | **string** | yes | — | — | Read-only |
| cost_center | string | yes | required | required | Name-based FK |
| budget_id | integer (int64) | yes | — | — | From URL path param |
| enabled | boolean | yes | — | optional | — |

**Relationships:**
- Belongs to **Budget** (via `budget_id`)
- References **CostCenter** (via `cost_center` name)
- Has many **CostCenterBudgetApprovalRule**

---

### 4. UserBudgetApprovalRule

**Role:** Business rule entity  
**Evidence:** Voci di costo, `t_user_budget_app_rule` table

**Operations:**

| Operation | Verb | Appsmith Query | API operationId |
|-----------|------|----------------|-----------------|
| List | GET | GetAllRuleUser | GetAllUserBudgetApprovalRule |
| Create | POST | NewRuleUser | NewUserBudgetApprovalRule |
| Edit | PUT | EditRuleUser | EditUserBudgetApprovalRule |
| Delete | DELETE | DeleteRuleUser | DeleteUserBudgetApprovalRule |

**Fields (from API spec):**

| Field | Type | In response | In create | In edit | Notes |
|-------|------|-------------|-----------|---------|-------|
| id | integer (int64) | yes | — | — | Server-assigned |
| threshold | **string** | yes | required | optional | ⚠ String type |
| approver_id | integer (int64) | yes | required | optional | — |
| approver_email | string | yes | — | — | Read-only |
| budget_id | integer (int64) | yes | required | optional | — |
| user_id | integer (int64) | yes | required | optional | — |
| level | integer | yes | required | optional | Approval hierarchy level |
| send_email | boolean | yes | required | optional | ⚠ Hardcoded `true` in Appsmith |

**Relationships:**
- Belongs to **Budget** (via `budget_id`)
- Belongs to **UserBudget** (via `budget_id` + `user_id`)
- References **User** as approver (via `approver_id`)

**Flags:**
- ✅ `send_email` — **Resolved:** Make user-configurable with default `true`. Appsmith hardcoded it; new frontend should expose as a toggle.
- ✅ Approval `level` is `int4` in DB — supports N levels. **Resolved:** Current Appsmith UI is hardcoded to two levels, but the backend has no such limit. New frontend should support dynamic N-level approval chains.
- ⚠ Appsmith never uses `level` query param for filtering

---

### 5. CostCenterBudgetApprovalRule

**Role:** Business rule entity (mirrors UserBudgetApprovalRule for cost centers)  
**Evidence:** Voci di costo, `t_cost_center_budget_app_rule` table

**Operations:** Same CRUD pattern as UserBudgetApprovalRule (List, Create, Edit, Delete)

**Fields:** Same as UserBudgetApprovalRule but with `cost_center: string` instead of `user_id: integer`.

**Same flags as entity 4 apply (Q2, Q3).**

---

### 6. CostCenter

**Role:** Primary entity  
**Evidence:** Centri di costo page (master table `t_cost_center`)

**Operations:**

| Operation | Verb | Appsmith Query | API operationId |
|-----------|------|----------------|-----------------|
| List all | GET | GetAllCostCenters | GetAllBudgetCostCenters |
| Get details | GET | GetCostCenterDetails | GetBudgetCostCenterDetails |
| Create | POST | NewCostCenter | NewBudgetCostCenter |
| Edit | PUT | EditCostCenter | EditBudgetCostCenter |
| Disable | PUT | DisableCostCenter | EditBudgetCostCenter |

**Note:** No DELETE operation — soft-disable only (sets `enabled: false`).

**Fields (from API spec `cost-center` response + `cost-center-new` + `cost-center-edit`):**

| Field | Type | In response | In create | In edit | Notes |
|-------|------|-------------|-----------|---------|-------|
| name | string | yes | required | — | Primary identifier (not numeric) |
| new_name | string | — | — | optional | Rename operation |
| manager_email | string | yes | — | — | Read-only (resolved from manager_id) |
| manager_id | integer (int64) | — | required | optional | — |
| user_ids | integer[] | — | required | optional | — |
| group_names | string[] | — | required | optional | — |
| user_count | integer (int64) | yes | — | — | Read-only |
| enabled | boolean | yes | required | optional | — |

**Details response adds:**

| Field | Type | Notes |
|-------|------|-------|
| manager | arak-int-user | Full user object |
| users | arak-int-user[] | Full user objects |
| groups | group-details[] | Full group objects |

**Relationships:**
- Has one **User** as manager (via `manager_id`)
- Has many **Users** (via `user_ids`)
- Has many **Groups** (via `group_names`)
- Has many **CostCenterBudget** (referenced by name)

**Flags:**
- ⚠ Identified by `name` not numeric ID — URL uses `encodeURIComponent(name)`
- ✅ **Resolved:** Add re-enable capability. Appsmith only had Disable; new frontend should support both Disable and Enable (the API's `EditBudgetCostCenter` already accepts `enabled: boolean`).
- ⚠ Appsmith validation bug: edit form references wrong widget for name validation

---

### 7. Group

**Role:** Primary entity  
**Evidence:** Gruppi page (master table `t_group`)

**Operations:**

| Operation | Verb | Appsmith Query | API operationId |
|-----------|------|----------------|-----------------|
| List all | GET | GetAllGroups | GetAllBudgetGroups |
| Get details | GET | GetGroupDetails | GetBudgetGroupDetails |
| Create | POST | NewGroup | NewBudgetGroup |
| Edit | PUT | UpdateGroup | EditBudgetGroup |
| Delete | DELETE | DeleteGroup | DeleteBudgetGroup |

**Fields (from API spec):**

| Field | Type | In response | In create | In edit | Notes |
|-------|------|-------------|-----------|---------|-------|
| name | string | yes | required | — | Primary identifier |
| new_name | string | — | — | optional | Rename operation |
| user_ids | integer[] | — | required | optional | — |
| user_count | integer (int64) | yes | — | — | Read-only |

**Details response adds:**

| Field | Type | Notes |
|-------|------|-------|
| users | arak-int-user[] | Full user objects |

**Relationships:**
- Has many **Users** (via `user_ids`)
- Referenced by **CostCenter** (via `group_names`)

**Flags:**
- ⚠ Identified by `name` not numeric ID (same pattern as CostCenter)
- ✅ Hard delete is intentional. Cascading behavior is handled server-side by the API — not a frontend concern.

---

### 8. User (external, read-only in Appsmith)

**Role:** Supporting/reference entity  
**Evidence:** Used across all pages for dropdowns and membership tables

**Operations (Appsmith uses):**

| Operation | Verb | Appsmith Query | API operationId |
|-----------|------|----------------|-----------------|
| List all | GET | GetAllUsers | GetAllArakInternalUser |

**Operations (available but unused in Appsmith):**

| Operation | Verb | API operationId |
|-----------|------|-----------------|
| Create | POST | NewArakInternalUser |
| Edit | PUT | EditArakInternalUser |
| Delete | DELETE | DeleteArakInternalUser |

**Fields (from API spec `arak-int-user`):**

| Field | Type | Notes |
|-------|------|-------|
| id | integer (int64) | — |
| first_name | string | — |
| last_name | string | — |
| email | string | — |
| created | date-time | — |
| updated | date-time | — |
| state | `{ name: string, enabled: boolean }` | ⚠ Nested object; Appsmith uses IIFE to extract |
| role | `{ name: string, created: date-time, updated: date-time }` | ⚠ Nested object; audit notes Appsmith treats as flat string in some columns |

**Flags:**
- ✅ Always filter `enabled=true` — same as Appsmith. No disabled user visibility needed.
- ⚠ Appsmith never uses `search_string` query param
- ✅ User CRUD out of scope — managed elsewhere. Entity remains read-only (list only).

---

### 9. Role (external, unused)

**Role:** Reference entity  
**Evidence:** API spec only — `GetAllArakInternalRoles` endpoint exists but Appsmith never calls it

**Flags:**
- ✅ Roles out of scope — not used in Appsmith, not needed in new app. Endpoint `GetAllArakInternalRoles` excluded.

---

### 10. BudgetForUser (unused endpoint)

**Role:** Read-only view entity  
**Evidence:** API spec only — `GetAllBudgetsForUser` exists but Appsmith never calls it

**Fields:** `limit`, `current`, `budget_id`, `name`, `user_id` (optional), `cost_center` (optional)

**Flags:**
- ✅ Out of scope — manager-only app, no self-service view.

---

## Entity Relationship Summary

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

## Cross-Entity Observations

1. **Name-based vs ID-based identification:** Budget, User, and approval rules use numeric IDs. CostCenter and Group use `name` as identifier. This creates an asymmetry in URL patterns and FK references.

2. **No delete for UserBudget/CostCenterBudget:** The API spec and Appsmith both lack delete operations for these allocation entities. They can only be disabled (`enabled: false`). **Q10: Is this intentional? How are allocations removed if a user leaves or a cost center is restructured?**

3. **String-typed monetary values:** `limit`, `current`, `threshold` are all `string` in the API. This is likely for decimal precision (avoiding float rounding). The new frontend must parse/format these carefully.

4. **Pagination envelope:** All list endpoints return `{ total_number, current_page, total_pages, items }`. Appsmith disables pagination (`disable_pagination=true`) everywhere. **Q11: Should the new app implement proper pagination, or are datasets small enough to load fully?**

---

## Expert Questions (Phase A)

| # | Question | Context |
|---|----------|---------|
| Q1 | ✅ **Resolved.** Budget `limit` is computed server-side via DB triggers (sum of allocations). Read-only in API. | Budget create flow |
| Q2 | ✅ **Resolved.** `send_email` should be user-configurable, default `true`. | Approval rules |
| Q3 | ✅ **Resolved.** DB uses `int4` — supports N levels. Appsmith was limited to two. New frontend should support dynamic N-level approval chains. | Approval rules |
| Q4 | ✅ **Resolved.** Add Enable action. API already supports it (`enabled: true` via PUT). New feature vs Appsmith. | CostCenter lifecycle |
| Q5 | ✅ **Resolved.** Hard delete intentional. Cascading is API business — frontend just calls DELETE. | Group lifecycle |
| Q6 | ✅ **Resolved.** No. Keep filtering `enabled=true` only, same as Appsmith. | User filtering |
| Q7 | ✅ **Resolved.** No. User CRUD is out of scope — managed elsewhere. User entity remains read-only. | User management |
| Q8 | ✅ **Resolved.** Roles out of scope — not used in Appsmith, not needed in new app. | Roles endpoint |
| Q9 | ✅ **Resolved.** No. Budget management is a manager-only app. No self-service view needed. | App scope |
| Q10 | ✅ **Resolved.** Intentional — disable only, no delete for allocations. Preserve this behavior. | Allocation lifecycle |
| Q11 | ✅ **Resolved.** Small datasets — no pagination needed. Continue using `disable_pagination=true`. | Data volume |

---

**Next:** After expert answers to the above, proceed to **Phase B: UX Pattern Map**.
