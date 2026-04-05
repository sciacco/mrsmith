# Budget Management - Appsmith Application Audit

**Source:** `budget-management-main.zip` (Appsmith export)  
**Audit date:** 2026-04-04  
**Format version:** 5, Client schema: 2, Server schema: 11

---

## 1. Application Inventory

| Field | Value |
|-------|-------|
| **Application name** | Budget Management |
| **Source type** | Appsmith Git export (ZIP) |
| **Layout** | FLUID, sidebar navigation |
| **Icon** | euros |
| **Evaluation version** | 2 |
| **JS Libraries** | xmlParser (fast-xml-parser 3.17.5) |

### Pages (8 total)

| Page | Default | Hidden | Purpose | Datasource(s) |
|------|---------|--------|---------|---------------|
| **Home** | Yes | No | Dashboard: budget utilization reports, unassigned users | Arak REST API |
| **Utenti** | No | Yes | Direct user CRUD (admin) | arak_db PostgreSQL |
| **Centri di costo (mockup)** | No | Yes | Legacy: department + user assignment CRUD | arak_db PostgreSQL |
| **Budget (mockup)** | No | Yes | Legacy: budget CRUD with approver workflows | arak_db PostgreSQL |
| **Articoli Acquisto** | No | Yes | Read-only article catalog from ERP | arak_db PostgreSQL (+ dormant Alyante MSSQL) |
| **Voci di costo** | No | No | Full budget lifecycle: budgets, cost centers, users, approval rules | Arak REST API |
| **Centri di costo** | No | No | Cost center CRUD with manager, users, groups | Arak REST API |
| **Gruppi** | No | No | Group CRUD with user membership | Arak REST API |

### Datasources (3)

| Name | Plugin | Purpose |
|------|--------|---------|
| **Arak (mistra-ng-int)** | restapi-plugin | REST API gateway for budget, users, cost-center, group, and approval-rule services |
| **arak_db (salvo)** | postgres-plugin | Direct PostgreSQL access (legacy/mockup pages + article catalog) |
| **Alyante** | mssql-plugin | MSSQL ERP system (dormant, not currently used) |

### Global Notes

- Two architectural layers coexist: **direct DB** (mockup pages, Utenti, Articoli) and **REST API** (production pages: Voci di costo, Centri di costo, Gruppi, Home)
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
| Query | Method | Endpoint | Params | Auto-load |
|-------|--------|----------|--------|-----------|
| GetBudgetOverPercent | GET | `/arak/budget/v1/report/budget-used-over-percentage` | page_number=1, disable_pagination=true, percentage={{i_percent.text}} | Yes |
| UnassignedUsers | GET | `/arak/budget/v1/report/unassigned-users` | page_number=1, disable_pagination=true, enabled=true | Yes |

**Hidden logic:**
- Container visibility driven by `total_number > 0` from API response
- Derived columns use IIFE pattern to extract nested `state.enabled` (checkbox) and `state.name` (text)
- No event handlers; purely data-display with parameter input

**Migration notes:**
- No error handling or loading indicators
- Default percentage 80.1% hardcoded in widget
- API response must include `total_number` and `items` fields

---

### 2.2 Utenti (User Management)

**Purpose:** Direct CRUD on `public.users` PostgreSQL table. Hidden page (`isHidden: true`).

**Widgets:**
- `Text1` - Title "User Management" (orange #e15615)
- `Container1` > `Table1` - Inline-editable table bound to `{{users.data}}`
  - Columns: EditActions1 (sticky left), id (hidden), name, username, email, active (checkbox), is_superuser (read-only checkbox)
  - Row-level save/discard buttons ("Salva"/"Annulla")

**Queries:**
| Query | SQL | Auto-load | Trigger |
|-------|-----|-----------|---------|
| users | `SELECT * FROM public."users"` | Yes | Page load + post-insert/update |
| ins_users | `INSERT INTO public."users" ("name","username","email","active") VALUES (...)` | No | Table1.onAddNewRowSave |
| upd_users | `UPDATE public."users" SET ... WHERE id = {{Table1.updatedRow.id}}` | No | Table1.EditActions1.onSave |

**Event flow:**
- Insert: `ins_users.run().then(() => users.run())`
- Update: `upd_users.run().then(() => users.run())`
- No delete functionality

**Hidden logic:**
- `is_superuser` visible but not editable (no admin interface to grant)
- Save/discard buttons enabled per-row via `Table1.updatedRowIndices.includes(currentIndex)`
- No `.catch()` error handlers on any query chain

**Migration notes:**
- `SELECT *` without pagination loads all users on every edit
- No input validation (empty names, invalid emails accepted)
- Direct SQL with `{{}}` bindings (Appsmith handles parameterization via `encodeParamsToggle`)
- INSERT omits password field; users created here may be unusable without separate password setup

---

### 2.3 Centri di costo (mockup) - Legacy

**Purpose:** Department CRUD with user assignment. Hidden (`isHidden: true`), superseded by "Centri di costo" page.

**Widgets:**
- Left panel: `data_table` (departments list with search/sort/pagination), `add_btn`, `refresh_btn`
- Right panel: `update_form` (JSON form, visible when row selected), `Container2` > `Table1` (department users)
- Modals: `Insert_Modal`, `Delete_Modal`, `mod_users`

**Queries (7, all PostgreSQL):**
| Query | Purpose | Dynamic Bindings |
|-------|---------|-----------------|
| SelectQuery | Department list with search/sort/pagination | `data_table.searchText`, `sortOrder`, `pageSize`, `pageNo` |
| InsertQuery | Create department | `insert_form.formData.*` |
| UpdateQuery | Edit department | `update_form.formData/sourceData` with visibility-conditional logic |
| DeleteQuery | Delete department | `data_table.triggeredRow.id` |
| getUsers | All users (for dropdown) | None |
| getDepartmentUsers | Users in selected dept (view: `v_department_users`) | `data_table.selectedRow.id` |
| InsertUser | Assign user to dept (`department_users` junction) | `data_table.selectedRow.id`, `Select1.selectedOptionValue` |

**Hidden logic:**
- UpdateQuery uses **field visibility state** to decide whether to save form value or source value: `update_form.fieldState.description.isVisible ? formData : sourceData`
- Insert form uses `_.omit(data_table.tableData[0], ...)` to derive schema from first row
- User assignment catches duplicate constraint: `showAlert('Errore, utente gi\u00e0 presente')`

**Migration notes:**
- Hard DELETE with no soft-delete pattern
- InsertQuery inserts id as string literal (type mismatch risk)
- ILIKE search without index optimization
- No debouncing on search input

---

### 2.4 Budget (mockup) - Legacy

**Purpose:** Full budget lifecycle with approval hierarchies. Most complex page (17 queries, 1 JSObject with 10 methods).

**Widgets:**
- `Container1` > `t_budget` (budget table), `ButtonNewBdg`, `IconButton3` (refresh)
- `form_budget` (edit form with 7 input fields + 4 multi-selects for departments, users, first/second approvers)
- `new_budget_modal` > `form_budgetNew` (create form, mirrors edit form)

**Queries (17, all PostgreSQL):**

| Category | Queries |
|----------|---------|
| SELECT (auto-load) | getBudgets, getDepartments, Select_public_users1 |
| SELECT (on row select) | getBudgetUsers, getBudgetDepartments, getFirstApprovers, getSecondApprovers |
| INSERT | insBudget (RETURNING id), insBudgetUsers, insBudgetDepartments, insFirstApprovers, insSecondApprovers |
| UPDATE | updBudget |
| DELETE | delBudgetUsers, delBudgetDepartments, delBudgetFirstApprovers, delBudgetSecondApprovers |

**JSObject (utils.js) methods:**
- `saveBudgetData()` / `insertBudgetData()` - Orchestrate multi-table save/insert
- `deleteRecords(budget_id)` - Smart diff: compares old vs new selections, deletes only if changed
- `save/newBudgetUsers/Departments/FirstApprovers/SecondApprovers()` - Parallel insert via `Promise.all()`

**Event flow (save existing):**
```
ButtonSaveBdg.onClick
  -> updBudget.run()
    -> getBudgets.run()
    -> utils.saveBudgetData()
      -> utils.deleteRecords() [conditional per-category delete]
      -> utils.saveBudgetDepartments/Users/FirstApprovers/SecondApprovers [parallel inserts]
```

**Hidden logic:**
- Delete-then-reinsert pattern: `deleteRecords()` compares arrays, runs full DELETE per category, then re-INSERTs selected values
- `Promise.all()` maps each selected option to individual INSERT query (N+1 pattern)
- Multi-select default values use IIFE: `((options, serverSideFiltering) => (query.data.map(...)))(widget.options, widget.serverSideFiltering)`
- `insBudget` uses `RETURNING id` to capture new budget ID for subsequent junction inserts

**Migration notes:**
- No transaction support; partial failures leave orphaned records
- N+1 query pattern: 10 selected users = 10 separate INSERT queries
- No error handling in utils.js methods (empty success/error callbacks)
- Delete button in `bg_budget` is disabled by default (no visible enable logic)
- Form duplication: edit form and new form are separate widget trees with parallel field names

**Database schema (inferred):**
- `budgets` (id, year, title, total_annual_amount, first_level_app_thresh, second_level_app_thresh, notify_percent, active)
- `budget_users` (budget_id, user_id) - ON CONFLICT DO NOTHING
- `budget_departments` (budget_id, department_id) - ON CONFLICT DO NOTHING
- `first_level_approvers` (budget_id, user_id) - ON CONFLICT DO NOTHING
- `second_level_approvers` (budget_id, user_id) - ON CONFLICT DO NOTHING

---

### 2.5 Articoli Acquisto (Purchase Articles)

**Purpose:** Read-only catalog of purchasable articles from ERP system.

**Widgets:**
- `Table1` - Article table bound to `{{get_articoli_aliante.data}}`
  - Visible columns: mg53_descrfam ("Famiglia"), mg54_descrsfam ("Tipo"), mg87_descart ("Descrizione")
  - Hidden columns: mg53_codfam, mg54_codsfam, mg66_codart, mg66_um1

**Queries:**
| Query | Datasource | Status | SQL |
|-------|-----------|--------|-----|
| get_articoli_aliante | arak_db (PostgreSQL) | **Active** (auto-load) | `SELECT * FROM public.alyante_articoli ORDER BY mg54_descrsfam, mg53_descrfam, mg87_descart` |
| get_articoli | Alyante (MSSQL) | **Dormant** (manual only) | Multi-table JOIN on MG53/MG66/MG87/MG54 tables with company/client filters |
| Query2 | arak_db | **Unused** | `SELECT * FROM public."budget_departments" LIMIT 10` (exploratory leftover) |

**Migration notes:**
- Dual-database pattern: PostgreSQL replica of MSSQL ERP data
- `SELECT *` without LIMIT; `defaultPageSize: 0` could load all records
- Table configured for row-level editing but all cells locked (`isCellEditable: false`)
- Legacy ERP column naming (mg53_, mg66_, mg87_)
- No event handlers; purely display

---

### 2.6 Voci di costo (Cost Items) - Production

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

---

### 2.7 Centri di costo (Cost Centers) - Production

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

### 2.8 Gruppi (Groups) - Production

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

## 3. Datasource & Query Catalog

### REST API Endpoints (Arak)

| Base Path | Domain | Methods | Used By |
|-----------|--------|---------|---------|
| `/arak/budget/v1/budget` | Budget CRUD | GET, POST, PUT, DELETE | Home, Voci di costo |
| `/arak/budget/v1/budget/{id}/user` | User-budget allocation | POST, PUT | Voci di costo |
| `/arak/budget/v1/budget/{id}/cost-center` | CC-budget allocation | POST, PUT | Voci di costo |
| `/arak/budget/v1/approval-rules/user-budget` | User approval rules | GET, POST, PUT, DELETE | Voci di costo |
| `/arak/budget/v1/approval-rules/cost-center-budget` | CC approval rules | GET, POST, PUT, DELETE | Voci di costo |
| `/arak/budget/v1/cost-center` | Cost center CRUD | GET, POST, PUT | Voci di costo, Centri di costo |
| `/arak/budget/v1/group` | Group CRUD | GET, POST, PUT, DELETE | Centri di costo, Gruppi |
| `/arak/budget/v1/report/budget-used-over-percentage` | Budget utilization report | GET | Home |
| `/arak/budget/v1/report/unassigned-users` | Unassigned users report | GET | Home |
| `/arak/users-int/v1/user` | User listing | GET | Voci di costo, Centri di costo, Gruppi |

### PostgreSQL Tables (Direct Access)

| Table/View | Operations | Used By |
|------------|-----------|---------|
| `public.users` | SELECT, INSERT, UPDATE | Utenti |
| `public.departments` | SELECT, INSERT, UPDATE, DELETE | Centri di costo (mockup) |
| `public.department_users` | INSERT | Centri di costo (mockup) |
| `v_department_users` (view) | SELECT | Centri di costo (mockup) |
| `public.budgets` | SELECT, INSERT, UPDATE | Budget (mockup) |
| `public.budget_users` | SELECT, INSERT, DELETE | Budget (mockup) |
| `public.budget_departments` | SELECT, INSERT, DELETE | Budget (mockup) |
| `public.first_level_approvers` | SELECT, INSERT, DELETE | Budget (mockup) |
| `public.second_level_approvers` | SELECT, INSERT, DELETE | Budget (mockup) |
| `public.alyante_articoli` | SELECT | Articoli Acquisto |

### Rewrite Recommendations

| Current | Recommendation |
|---------|---------------|
| Direct PostgreSQL queries (Utenti, mockup pages) | Move behind REST API (already done for production pages) |
| `SELECT *` without pagination | Add server-side pagination; explicit column lists |
| N+1 insert pattern (Budget mockup) | Batch INSERT with array values in single API call |
| Delete-then-reinsert pattern | Use PATCH/diff-based updates |
| Dormant MSSQL datasource (Alyante) | Remove or document sync mechanism |

---

## 4. Findings Summary

### Embedded Business Rules

| Rule | Location | Classification |
|------|----------|---------------|
| Budget "active" = current year match | Voci di costo, t_budget computed column | **Business logic** (should be server-side) |
| Percentage threshold for budget alerts (default 80.1%) | Home, i_percent widget | **Business logic** |
| Two-level approval hierarchy (first + second approvers) | Budget (mockup), Voci di costo | **Business logic** |
| `send_email: true` hardcoded on approval rules | Voci di costo, NewRuleUser/NewRuleCC bodies | **Business logic** |
| Only enabled users shown in dropdowns | All pages (query param `enabled: true`) | **Frontend orchestration** |
| Edit/Disable buttons disabled for inactive cost centers | Centri di costo, bg_user_cost_center | **UI orchestration** |
| Container visibility based on `total_number > 0` | Home, report containers | **Presentation** |
| Field visibility-conditional updates | Centri di costo (mockup), UpdateQuery | **Frontend orchestration** |

### Duplication

1. **Mockup vs production pages:** "Centri di costo (mockup)" duplicates "Centri di costo"; "Budget (mockup)" duplicates "Voci di costo"
2. **Edit/New form duplication:** Budget (mockup) has separate widget trees for edit and new forms with parallel field names
3. **Multi-select IIFE pattern** repeated identically across 10+ widgets
4. **Error alert strings** inconsistent across pages (some Italian, some English, some missing)
5. **Table column computed value pattern** `Table.processedTableData.map(...)` repeated for every column

### Security Concerns

| Issue | Severity | Location |
|-------|----------|----------|
| Direct SQL with `{{}}` interpolation (relies on Appsmith parameterization) | Medium | Utenti, mockup pages |
| `SELECT * FROM public.users` exposes all columns including potentially sensitive data | Medium | Utenti |
| No client-side authorization checks; all CRUD operations assume full access | Medium | All pages |
| No audit logging for budget/approval changes | High | All mutation pages |
| `is_superuser` column visible but not editable (information disclosure) | Low | Utenti |

### Migration Blockers

| Blocker | Impact | Resolution |
|---------|--------|------------|
| **Trailing comma JSON syntax** in EditRuleUser/EditRuleCC bodies | Queries will fail | Fix JSON syntax |
| **Wrong validation reference** in i_edit_cc_name (references i_new_cc_name) | Edit form validation broken | Fix widget reference |
| **No transaction support** for multi-table operations | Data integrity risk on partial failures | Implement server-side transactions |
| **Commented-out conditional logic** in Editbudget | Full update sent regardless of field changes | Uncomment or remove dead code |
| **Utils.js delete-then-reinsert** without rollback | Data loss on insert failure after delete | Redesign as atomic operation |

### Candidate Domain Entities

Based on the audit, these entities should be modeled in the backend:

1. **User** (id, name, username, email, active, is_superuser, state{enabled, name}, first_name, last_name, role)
2. **Budget** (id, year, name/title, total_annual_amount, first_level_app_thresh, second_level_app_thresh, notify_percent, active, limit, current)
3. **CostCenter** (name[PK], enabled, manager_id, users[], groups[])
4. **Group** (name[PK], users[], user_count)
5. **BudgetUserAllocation** (budget_id, user_id, limit, current, enabled)
6. **BudgetCostCenterAllocation** (budget_id, cost_center, limit, current, enabled)
7. **ApprovalRule** (id, budget_id, user_id/cost_center, threshold, approver_id, level, send_email)
8. **Article** (family_code, family_name, subfamily_code, subfamily_name, article_code, unit_of_measure, description)

### Recommended Next Steps

1. **Remove mockup pages** (Centri di costo mockup, Budget mockup) - superseded by production REST API pages
2. **Migrate Utenti** from direct PostgreSQL to REST API
3. **Fix identified bugs** (JSON trailing commas, validation cross-reference, commented-out code)
4. **Add error handling** across all query chains (most `.then()` chains lack `.catch()`)
5. **Implement server-side pagination** (all queries currently use `disable_pagination=true`)
6. **Add audit logging** for all budget and approval rule mutations
7. **Extract business rules** (active year computation, threshold defaults) to backend
8. **Design unified CRUD patterns** to eliminate form/modal duplication
9. **Hand off to `appsmith-migration-spec`** for Phase 2 specification work
