# Budget Management - Appsmith Application Audit

**Source:** `budget-management-main.zip` (Appsmith export)  
**Audit date:** 2026-04-04  
**Format version:** 5, Client schema: 2, Server schema: 11  
**API reference:** `docs/mistra-dist.yaml` (Mistra NG Internal API v2.7.14)

---

## 1. Application Inventory

| Field | Value |
|-------|-------|
| **Application name** | Budget Management |
| **Source type** | Appsmith Git export (ZIP) |
| **Layout** | FLUID, sidebar navigation |
| **Icon** | euros |
| **Evaluation version** | 2 |

### Pages (4 in scope)

| Page | Default | Purpose | Datasource |
|------|---------|---------|------------|
| **Home** | Yes | Dashboard: budget utilization reports, unassigned users | Arak REST API |
| **Voci di costo** | No | Full budget lifecycle: budgets, cost centers, users, approval rules | Arak REST API |
| **Centri di costo** | No | Cost center CRUD with manager, users, groups | Arak REST API |
| **Gruppi** | No | Group CRUD with user membership | Arak REST API |

### Datasource

| Name | Plugin | Purpose |
|------|--------|---------|
| **Arak (mistra-ng-int)** | restapi-plugin | REST API gateway for budget, users, cost-center, group, and approval-rule services |

### Global Notes

- All production pages use a single REST API datasource (Arak)
- Italian-language UI throughout (labels, messages, column headers)
- No i18n framework; all text hardcoded
- Theme-bound styling across all widgets via `appsmith.theme.*` tokens

---

## 2. Page Audits

### 2.1 Home (Dashboard)

**Purpose:** Read-only dashboard showing budgets exceeding a configurable percentage threshold and users not assigned to any budget.

**Widgets:**
- `Text1` - Page title "BACKEND BUDGET MANAGEMENT"
- `report_budget_over_80_perc` - Container (conditionally visible: `GetBudgetOverPercent.data.total_number > 0`)
  - `i_percent` - Number input (default "80.1", label "Percentuale")
  - `Text3` - Dynamic title: "Budget oltre il {{i_percent.text}} %"
  - `Table2` - Budget data (columns: id[hidden], current, limit, name, year)
- `report_unassigned_users` - Container (conditionally visible: `UnassignedUsers.data.total_number > 0`)
  - `Text2` - Static title "Utenti non assegnati a nessun Budget"
  - `Table1` - User data with derived columns extracting `state.enabled` and `state.name`

**Queries:**
| Query | Method | Endpoint | operationId | Params | Auto-load |
|-------|--------|----------|-------------|--------|-----------|
| GetBudgetOverPercent | GET | `/arak/budget/v1/report/budget-used-over-percentage` | GetAllBudgetsUsedOverPercentage | page_number=1, disable_pagination=true, percentage={{i_percent.text}} | Yes |
| UnassignedUsers | GET | `/arak/budget/v1/report/unassigned-users` | GetUnassignedArakInternalUser | page_number=1, disable_pagination=true, enabled=true | Yes |

**Hidden logic:**
- Container visibility driven by `total_number > 0` from API response
- Derived columns use IIFE pattern to extract nested `state.enabled` (checkbox) and `state.name` (text)
- No event handlers; purely data-display with parameter input

**Migration notes:**
- No error handling or loading indicators
- Default percentage 80.1% hardcoded in widget
- API `percentage` param is `float` (required) per spec; Appsmith sends it as text from `i_percent.text`

---

### 2.2 Voci di costo (Cost Items)

**Purpose:** Full budget management via REST API: create/edit/delete budgets, associate cost centers and users with spending limits, define multi-level approval rules.

**Widgets:**
- `t_budget` - Master budget table (columns: year, name, limit, current, computed "active" checkbox)
- `t_user_budget` - User allocations for selected budget
- `t_cost_center_budget` - Cost center allocations for selected budget
- `t_user_budget_app_rule` - Approval rules for selected user-budget
- `t_cost_center_budget_app_rule` - Approval rules for selected CC-budget
- `bg_budget` - Button group (Refresh, Edit, Delete)
- 12+ modals for CRUD on budgets, allocations, and approval rules

**Queries (19, all REST API via Arak):**

| Category | Queries | Endpoints |
|----------|---------|-----------|
| GET (auto-load) | GetAllBudgets, GetAllUsers, GetAllCostCenters | `/budget`, `/user`, `/cost-center` |
| GET (on select) | GetBudgetDetails, GetAllRuleUser, GetAllRuleCC | `/budget/{id}`, `/approval-rules/user-budget`, `/approval-rules/cost-center-budget` |
| POST | NewBudget, NewUserBudget, NewCostCenterBudget, NewRuleUser, NewRuleCC | Various endpoints |
| PUT | Editbudget, UpdateUserBudget, UpdateCostCenterBudget, EditRuleUser, EditRuleCC | Various endpoints |
| DELETE | DeleteBudget, DeleteRuleUser, DeleteRuleCC | Various endpoints |

**Cascading selection flow:**
```
t_budget row select -> GetBudgetDetails
  -> populates t_user_budget, t_cost_center_budget
    -> t_user_budget row select -> GetAllRuleUser -> t_user_budget_app_rule
    -> t_cost_center_budget row select -> GetAllRuleCC -> t_cost_center_budget_app_rule
```

**Computed "active" column:**
```javascript
currentRow.year == new Date().getFullYear()
```

**Hidden logic:**
- `Editbudget` body has commented-out conditional field inclusion (currently sends both fields unconditionally)
- `send_email: true` hardcoded in all approval rule creation bodies
- `String()` coercion on numeric limit/threshold values in POST/PUT bodies
- Edit modals use hidden input fields to pass IDs (e.g., `i_edit_rule_user_rule_id`)

**Bugs found:**
1. **EditRuleUser and EditRuleCC** bodies have trailing commas in JSON (syntax error)
2. **Widget naming inconsistency:** `i_edit_rule_cc__rule_id` (double underscore)

**Migration notes:**
- Most feature-rich page; 19 queries, 12+ modals, 30+ form inputs
- All mutations use `.then()` chaining; inconsistent refresh strategies
- No loading indicators during async operations
- Delete button initially disabled with no visible enable logic
- Appsmith does not use `search_string` or `year` query params available on GetAllBudgets

---

### 2.3 Centri di costo (Cost Centers)

**Purpose:** Cost center CRUD via REST API with manager assignment, user membership, and group association.

**Widgets:**
- `t_cost_center` - Master table (columns: name, enabled, manager_email, user_count)
- `t_user_cost_center` - Users in selected cost center (data from `utils.user_list`)
- `c_cost_center_details` - Detail panel (name, manager select, active switch - all read-only)
- `bg_user_cost_center` - Button group (Refresh, Edit[disabled if CC disabled], Disable[disabled if CC disabled])
- Modals: `m_new_cc`, `m_edit_cc`, `m_disable_cc`

**Queries (7, all REST API):**
| Query | Method | Endpoint |
|-------|--------|----------|
| GetAllCostCenters | GET | `/arak/budget/v1/cost-center` |
| GetCostCenterDetails | GET | `/arak/budget/v1/cost-center/{{encodeURIComponent(name)}}` |
| GetAllUsers | GET | `/arak/users-int/v1/user` |
| GetAllGroups | GET | `/arak/budget/v1/group` |
| NewCostCenter | POST | `/arak/budget/v1/cost-center` |
| EditCostCenter | PUT | `/arak/budget/v1/cost-center/{{encodeURIComponent(name)}}` |
| DisableCostCenter | PUT | `/arak/budget/v1/cost-center/{{encodeURIComponent(name)}}` (body: `{enabled: false}`) |

**JSObject (utils):**
- `utils.user_list` - Transforms/filters user data for display (executeOnLoad: true)
- `utils.test` - Test utility (not used in UI)

**Hidden logic:**
- Edit form uses conditional spread for optional name change: `...(name.length > 0 ? {new_name: name} : {})`
- Manager/users/groups pre-populated via IIFE pattern from `GetCostCenterDetails.data`
- Disable modal shows affected users table before confirmation
- Soft-disable pattern (sets `enabled: false`) instead of hard delete

**Bug found:**
- `i_edit_cc_name` validation references **wrong widget**: `{{ i_new_cc_name.text.length > 0 }}` instead of `i_edit_cc_name`

**Migration notes:**
- Cost centers identified by name (not numeric ID) in URL paths
- `encodeURIComponent()` used for URL-safe names
- No error handling in query chains (no `.catch()`)
- Edit/Disable buttons disabled when cost center is already disabled

---

### 2.4 Gruppi (Groups)

**Purpose:** Group CRUD via REST API with user membership management.

**Widgets:**
- `t_group` - Master table (columns: name, user_count)
- `t_user_group` - Members of selected group
- `c_group_details` - Detail panel (read-only name)
- `bg_user_group` - Button group (Refresh, Edit, Delete)
- Modals: `m_new_group`, `m_update_group`, `m_delete_group`

**Queries (6, all REST API):**
| Query | Method | Endpoint | Error handling |
|-------|--------|----------|----------------|
| GetAllGroups | GET | `/arak/budget/v1/group` | None |
| GetGroupDetails | GET | `/arak/budget/v1/group/{{encodeURIComponent(name)}}` | **Yes** - showAlert with status code |
| GetAllUsers | GET | `/arak/users-int/v1/user` | None |
| NewGroup | POST | `/arak/budget/v1/group` | **Yes** - showAlert with error message |
| UpdateGroup | PUT | `/arak/budget/v1/group/{{encodeURIComponent(name)}}` | **None** |
| DeleteGroup | DELETE | `/arak/budget/v1/group/{{encodeURIComponent(name)}}` | **None** |

**Hidden logic:**
- Update body uses conditional spread: `...(name.length > 0 ? {new_name: name} : {})`
- Multi-select pre-populates current members: `t_user_group.tableData.map(item => item.id)`
- Groups identified by name in URL paths (same pattern as cost centers)

**Migration notes:**
- Only page with partial error handling (GetGroupDetails and NewGroup have `.catch()`)
- UpdateGroup and DeleteGroup fail silently
- `i_update_group_name` has no `resetOnSubmit` (stale data on reopen)

---

## 3. API Contract Reference (from `docs/mistra-dist.yaml`)

### 3.1 Endpoints Used by Appsmith

| Endpoint | operationId | Methods | Used By |
|----------|-------------|---------|---------|
| `/arak/budget/v1/budget` | GetAllBudgets / NewBudget | GET, POST | Home, Voci di costo |
| `/arak/budget/v1/budget/{budget_id}` | GetBudgetDetails / EditBudget / DeleteBudget | GET, PUT, DELETE | Voci di costo |
| `/arak/budget/v1/budget/{budget_id}/user` | NewUserBudget / EditUserBudget | POST, PUT | Voci di costo |
| `/arak/budget/v1/budget/{budget_id}/cost-center` | NewCostCenterBudget / EditCostCenterBudget | POST, PUT | Voci di costo |
| `/arak/budget/v1/approval-rules/user-budget` | GetAllUserBudgetApprovalRule / NewUserBudgetApprovalRule | GET, POST | Voci di costo |
| `/arak/budget/v1/approval-rules/user-budget/{rule_id}` | EditUserBudgetApprovalRule / DeleteUserBudgetApprovalRule | PUT, DELETE | Voci di costo |
| `/arak/budget/v1/approval-rules/cost-center-budget` | GetAllCostCenterBudgetApprovalRule / NewCostCenterBudgetApprovalRule | GET, POST | Voci di costo |
| `/arak/budget/v1/approval-rules/cost-center-budget/{rule_id}` | EditCostCenterBudgetApprovalRule / DeleteCostCenterBudgetApprovalRule | PUT, DELETE | Voci di costo |
| `/arak/budget/v1/cost-center` | GetAllBudgetCostCenters / NewBudgetCostCenter | GET, POST | Voci di costo, Centri di costo |
| `/arak/budget/v1/cost-center/{cost_center_id}` | GetBudgetCostCenterDetails / EditBudgetCostCenter | GET, PUT | Centri di costo |
| `/arak/budget/v1/group` | GetAllBudgetGroups / NewBudgetGroup | GET, POST | Centri di costo, Gruppi |
| `/arak/budget/v1/group/{group_id}` | GetBudgetGroupDetails / EditBudgetGroup / DeleteBudgetGroup | GET, PUT, DELETE | Gruppi |
| `/arak/budget/v1/report/budget-used-over-percentage` | GetAllBudgetsUsedOverPercentage | GET | Home |
| `/arak/budget/v1/report/unassigned-users` | GetUnassignedArakInternalUser | GET | Home |
| `/arak/users-int/v1/user` | GetAllArakInternalUser | GET | Voci di costo, Centri di costo, Gruppi |

### 3.2 Available Endpoints NOT Used by Appsmith

These exist in the API spec but the Appsmith app never calls them. They represent available capabilities for the new frontend:

| Endpoint | operationId | Method | Purpose |
|----------|-------------|--------|---------|
| `/arak/budget/v1/budget-for-user` | GetAllBudgetsForUser | GET | Get budgets for a specific user (by `user_email` header) |
| `/arak/users-int/v1/user` | NewArakInternalUser | POST | Create user |
| `/arak/users-int/v1/user/{user_id}` | EditArakInternalUser | PUT | Edit user |
| `/arak/users-int/v1/user/{user_id}` | DeleteArakInternalUser | DELETE | Soft-delete user |
| `/arak/users-int/v1/role` | GetAllArakInternalRoles | GET | List all roles |

### 3.3 Unused Query Parameters

The API supports these filters that Appsmith never uses:

| Endpoint | Parameter | Type | Purpose |
|----------|-----------|------|---------|
| GetAllBudgets | `search_string` | string | Filter budgets by name |
| GetAllBudgets | `year` | integer | Filter budgets by year |
| GetAllArakInternalUser | `search_string` | string | Filter users by email text |
| GetAllUserBudgetApprovalRule | `level` | integer | Filter rules by approval level |
| GetAllCostCenterBudgetApprovalRule | `level` | integer | Filter rules by approval level |

### 3.4 Authoritative Schema Definitions

Source of truth from the OpenAPI spec. Key detail: **`limit`, `current`, and `threshold` are `string` type** in the API (not numbers).

#### `arak-int-user` (response)
| Field | Type | Required |
|-------|------|----------|
| id | integer (int64) | yes |
| first_name | string | yes |
| last_name | string | yes |
| email | string | yes |
| created | string (date-time) | yes |
| updated | string (date-time) | yes |
| state | `arak-int-user-state` | yes |
| role | `arak-int-role` | yes |

`arak-int-user-state`: `{ name: string, enabled: boolean }` (both required)  
`arak-int-role`: `{ name: string, created: date-time, updated: date-time }` (all required)

#### `budget` (response)
| Field | Type | Required |
|-------|------|----------|
| id | integer (int64) | yes |
| name | string | yes |
| year | integer | yes |
| limit | **string** | yes |
| current | **string** | yes |

#### `budget-new` (request body)
| Field | Type | Required |
|-------|------|----------|
| name | string | yes |
| year | integer | yes |

#### `budget-edit` (request body)
| Field | Type | Required |
|-------|------|----------|
| name | string | no |
| year | integer | no |

#### `budget-details` (response)
| Field | Type | Required |
|-------|------|----------|
| id | integer (int64) | yes |
| name | string | yes |
| year | integer | yes |
| limit | **string** | yes |
| current | **string** | yes |
| cost_center_budgets | `cost_center-budget[]` | yes |
| user_budgets | `user-budget[]` | yes |

#### `user-budget` (response)
| Field | Type | Required |
|-------|------|----------|
| limit | **string** | yes |
| current | **string** | yes |
| user_id | integer (int64) | yes |
| user_email | string | yes |
| budget_id | integer (int64) | yes |
| enabled | boolean | yes |

#### `user-budget-upsert` (request body for POST)
| Field | Type | Required |
|-------|------|----------|
| limit | **string** | yes |
| user_id | integer (int64) | yes |

#### `user-budget-edit` (request body for PUT)
| Field | Type | Required |
|-------|------|----------|
| limit | string | no |
| user_id | integer (int64) | yes |
| enabled | boolean | no |

#### `cost_center-budget` (response)
| Field | Type | Required |
|-------|------|----------|
| limit | **string** | yes |
| current | **string** | yes |
| cost_center | string | yes |
| budget_id | integer (int64) | yes |
| enabled | boolean | yes |

#### `cost_center-budget-upsert` (request body for POST)
| Field | Type | Required |
|-------|------|----------|
| limit | **string** | yes |
| cost_center | string | yes |

#### `cost_center-budget-edit` (request body for PUT)
| Field | Type | Required |
|-------|------|----------|
| cost_center | string | yes |
| limit | string | no |
| enabled | boolean | no |

#### `user-budget-approval-rule` (response)
| Field | Type | Required |
|-------|------|----------|
| id | integer (int64) | yes |
| threshold | **string** | yes |
| approver_id | integer (int64) | yes |
| approver_email | string | yes |
| budget_id | integer (int64) | yes |
| user_id | integer (int64) | yes |
| level | integer | yes |
| send_email | boolean | yes |

#### `user-budget-approval-rule-new` (request body)
| Field | Type | Required |
|-------|------|----------|
| threshold | **string** | yes |
| approver_id | integer (int64) | yes |
| budget_id | integer (int64) | yes |
| user_id | integer (int64) | yes |
| level | integer | yes |
| send_email | boolean | yes |

#### `user-budget-approval-rule-edit` (request body)
All fields optional: threshold (string), approver_id (int64), budget_id (int64), user_id (int64), level (integer), send_email (boolean)

#### `cc-budget-approval-rule` (response)
Same as user-budget-approval-rule but with `cost_center: string` instead of `user_id`

#### `cc-budget-approval-rule-new` / `cc-budget-approval-rule-edit` (request bodies)
Same pattern as user variants, with `cost_center: string` instead of `user_id`

#### `cost-center` (response)
| Field | Type | Required |
|-------|------|----------|
| name | string | yes |
| manager_email | string | yes |
| user_count | integer (int64) | yes |
| enabled | boolean | yes |

#### `cost-center-new` (request body)
| Field | Type | Required |
|-------|------|----------|
| name | string | yes |
| manager_id | integer (int64) | yes |
| user_ids | integer[] | yes |
| group_names | string[] | yes |
| enabled | boolean | yes |

#### `cost-center-edit` (request body)
All optional: new_name (string), manager_id (int64), user_ids (integer[]), group_names (string[]), enabled (boolean)

#### `cost-center-details` (response)
| Field | Type | Required |
|-------|------|----------|
| name | string | yes |
| manager | `arak-int-user` | yes |
| users | `arak-int-user[]` | yes |
| groups | `group-details[]` | yes |
| enabled | boolean | yes |

#### `group` (response)
| Field | Type | Required |
|-------|------|----------|
| name | string | yes |
| user_count | integer (int64) | yes |

#### `group-new` (request body)
| Field | Type | Required |
|-------|------|----------|
| name | string | yes |
| user_ids | integer[] | yes |

#### `group-edit` (request body)
All optional: new_name (string), user_ids (integer[])

#### `group-details` (response)
| Field | Type | Required |
|-------|------|----------|
| name | string | yes |
| users | `arak-int-user[]` | yes |

#### `budget-for-user` (response, unused endpoint)
| Field | Type | Required |
|-------|------|----------|
| limit | string | yes |
| current | string | yes |
| budget_id | integer (int64) | yes |
| name | string | yes |
| user_id | integer (int64) | no |
| cost_center | string | no |

### 3.5 Paginated Response Envelope

All list endpoints return a standard envelope:
```
{ total_number: integer, current_page: integer, total_pages: integer, items: T[] }
```

### 3.6 Authentication

- OAuth2 with `openid` + `profile` scopes (global security scheme)
- All endpoints require authentication

---

## 4. Findings Summary

### Appsmith vs API Mismatches

| Issue | Detail |
|-------|--------|
| **`limit`/`current`/`threshold` are strings** | API spec defines these as `string`, not `number`. Appsmith renders them in number-type table columns and uses `String()` coercion in POST/PUT bodies — this works by accident but the new frontend should treat them as strings (likely decimal values serialized as text). |
| **`budget-edit` fields are all optional** | API allows partial updates (name-only or year-only). Appsmith's commented-out conditional spread was the correct approach; current code sends both fields unconditionally. |
| **`percentage` param is required float** | Home page sends it as text from `i_percent.text`; API expects `number` type with `float` format. |
| **`role` is a nested object** | API returns `role: { name, created, updated }` but Appsmith treats `role` as a flat string in some table columns. |

### Embedded Business Rules

| Rule | Location | Classification |
|------|----------|---------------|
| Budget "active" = current year match | Voci di costo, t_budget computed column | **Business logic** (should be server-side) |
| Percentage threshold for budget alerts (default 80.1%) | Home, i_percent widget | **Business logic** |
| Two-level approval hierarchy (first + second approvers) | Voci di costo | **Business logic** |
| `send_email: true` hardcoded on approval rules | Voci di costo, NewRuleUser/NewRuleCC bodies | **Business logic** |
| Only enabled users shown in dropdowns | All pages (query param `enabled: true`) | **Frontend orchestration** |
| Edit/Disable buttons disabled for inactive cost centers | Centri di costo, bg_user_cost_center | **UI orchestration** |
| Container visibility based on `total_number > 0` | Home, report containers | **Presentation** |

### Duplication

1. **Multi-select IIFE pattern** repeated identically across 10+ widgets
2. **Error alert strings** inconsistent across pages (some Italian, some English, some missing)
3. **Table column computed value pattern** `Table.processedTableData.map(...)` repeated for every column

### Security Concerns

| Issue | Severity | Location |
|-------|----------|----------|
| No client-side authorization checks; all CRUD operations assume full access | Medium | All pages |
| No audit logging for budget/approval changes | High | All mutation pages |

### Bugs

| Bug | Impact | Location |
|-----|--------|----------|
| **Trailing comma JSON syntax** in EditRuleUser/EditRuleCC bodies | Queries will fail | Voci di costo |
| **Wrong validation reference** in i_edit_cc_name (references i_new_cc_name) | Edit form validation broken | Centri di costo |
| **Commented-out conditional logic** in Editbudget body | Full update sent regardless of field changes | Voci di costo |

### Next Step

Hand off this audit to **`appsmith-migration-spec`** (Phase 2) to produce the expert-led migration specification. That phase should address all findings above: API contract alignment, unused endpoint adoption, type handling, pagination, error handling, business rule extraction, and unified CRUD patterns.
