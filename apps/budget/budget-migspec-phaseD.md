# Budget Management — Phase D: Integration and Data Flow

**Source:** `apps/budget/APPSMITH_AUDIT.md` + Phase A/B/C resolved decisions  
**Date:** 2026-04-05  
**Status:** Draft — awaiting expert review

---

## 1. External Systems

| System | Protocol | Purpose | Auth | Used by |
|--------|----------|---------|------|---------|
| **Arak (mistra-ng-int)** | REST API (HTTPS) | All budget, cost center, group, approval rule, and user operations | OAuth2 (`openid` + `profile` scopes) | All views |
| **Keycloak** | OAuth2/OIDC | Authentication provider | — | App-wide (via `@mrsmith/auth-client`) |

**No other integrations found.** The audit shows a single datasource ("Arak REST API"). No WebSocket connections, no file uploads, no third-party services, no background jobs.

---

## 2. API Integration Map

### Base path: `/arak/budget/v1/` (budget domain) + `/arak/users-int/v1/` (user domain)

### Endpoints in scope (from Phase A):

| Endpoint | Methods | Used by views | Response type |
|----------|---------|---------------|---------------|
| `/budget` | GET, POST | Voci di costo | Paginated `budget[]` / `budget` |
| `/budget/{budget_id}` | GET, PUT, DELETE | Voci di costo | `budget-details` |
| `/budget/{budget_id}/user` | POST, PUT | Voci di costo | `user-budget` |
| `/budget/{budget_id}/cost-center` | POST, PUT | Voci di costo | `cost_center-budget` |
| `/approval-rules/user-budget` | GET, POST | Voci di costo | Paginated `user-budget-approval-rule[]` |
| `/approval-rules/user-budget/{rule_id}` | PUT, DELETE | Voci di costo | `user-budget-approval-rule` |
| `/approval-rules/cost-center-budget` | GET, POST | Voci di costo | Paginated `cc-budget-approval-rule[]` |
| `/approval-rules/cost-center-budget/{rule_id}` | PUT, DELETE | Voci di costo | `cc-budget-approval-rule` |
| `/cost-center` | GET, POST | Centri di costo, Voci di costo | Paginated `cost-center[]` |
| `/cost-center/{cost_center_id}` | GET, PUT | Centri di costo | `cost-center-details` |
| `/group` | GET, POST | Gruppi, Centri di costo | Paginated `group[]` |
| `/group/{group_id}` | GET, PUT, DELETE | Gruppi | `group-details` |
| `/report/budget-used-over-percentage` | GET | Home | Paginated `budget[]` |
| `/report/unassigned-users` | GET | Home | Paginated `arak-int-user[]` |
| `/user` (users-int) | GET | All views (reference data) | Paginated `arak-int-user[]` |

### Endpoints explicitly out of scope (Phase A decisions):

| Endpoint | Reason |
|----------|--------|
| `/budget-for-user` | Manager-only app, no self-service (Q9) |
| `/user` POST/PUT/DELETE | User CRUD managed elsewhere (Q7) |
| `/role` | Roles not relevant to budget app (Q8) |

---

## 3. Cross-View User Journeys

### Journey 1: Review budget health (Home)

```
User opens app
  → Home tab (default)
  → Auto-fetch: GetBudgetOverPercent (default 80.1%), UnassignedUsers
  → User sees alert table (if any budgets over threshold)
  → User sees unassigned users table (if any)
  → User adjusts percentage input → re-fetch GetBudgetOverPercent
  → END (read-only, no further navigation from Home)
```

**Data flow:** Two independent GET requests on page load. Percentage input triggers re-fetch of one endpoint. No cross-view state needed.

**Budget alert rows are clickable → navigate to `/budgets/:id`.** Direct consequence of the drill-down design (Phase B, Q13).

---

### Journey 2: Create and configure a new budget (Voci di costo)

```
User navigates to Voci di costo tab
  → Auto-fetch: GetAllBudgets, GetAllUsers, GetAllCostCenters
  → User clicks "Create" → modal: enters name, year → POST NewBudget
  → Budget list refreshes (invalidate budget list query)
  → User selects new budget row → navigates to /budgets/:id
  → Auto-fetch: GetBudgetDetails (returns empty allocations)
  → User adds user allocation → modal: select user, enter limit → POST NewUserBudget
  → Budget details refresh (invalidate budget details query)
  → User adds CC allocation → modal: select CC, enter limit → POST NewCostCenterBudget
  → Budget details refresh
  → User expands user allocation row → empty approval rules
  → User adds approval rule → modal: select level (1-3), threshold, approver, send_email toggle (default true) → POST NewRuleUser
  → Approval rules refresh
  → Repeat for additional rules/allocations
```

**Data flow:**
- **Reference data needed:** Users list (for approver + allocation dropdowns), Cost centers list (for CC allocation dropdown)
- **Mutation → invalidation chain:** Each POST invalidates the parent entity's detail query
- **Cross-entity dependency:** Budget must exist before allocations. Allocation must exist before approval rules. Sequential by nature.

---

### Journey 3: Edit budget allocation and approval rules (Voci di costo)

```
User navigates to Voci di costo tab → selects existing budget → /budgets/:id
  → Sees user allocations + CC allocations (tabbed)
  → User clicks edit on a user allocation → modal: adjust limit, toggle enabled → PUT UpdateUserBudget
  → Budget details refresh
  → User expands allocation row → sees approval rules
  → User edits rule → modal: adjust threshold, change approver → PUT EditRuleUser
  → Approval rules refresh
  → User deletes rule → confirm → DELETE DeleteRuleUser
  → Approval rules refresh
```

**Data flow:** Same invalidation pattern. Edit/delete modals pre-populated from current row data (Phase C, O3).

---

### Journey 4: Manage cost centers (Centri di costo)

```
User navigates to Centri di costo tab
  → Auto-fetch: GetAllCostCenters, GetAllUsers, GetAllGroups
  → User selects CC row → GetCostCenterDetails
  → Detail panel shows: name, manager, active status (read-only)
  → User list shows members
  → User clicks Edit → modal: change name, manager, users, groups → PUT EditCostCenter
  → CC list + details refresh
  → User clicks Disable → confirm modal (shows affected users) → PUT DisableCostCenter (enabled: false)
  → CC list + details refresh
  → User clicks Enable (NEW in migration) → PUT EditCostCenter (enabled: true)
  → CC list + details refresh
```

**Data flow:**
- **Reference data needed:** Users list (manager + members), Groups list
- Enable action is new — Appsmith only had Disable (Phase A, Q4)
- Disable confirmation should show affected users from current detail data (no extra fetch needed)

---

### Journey 5: Manage groups (Gruppi)

```
User navigates to Gruppi tab
  → Auto-fetch: GetAllGroups, GetAllUsers
  → User selects group row → GetGroupDetails
  → Detail panel shows: name (read-only)
  → Member list shows users
  → User clicks Edit → modal: rename, update members → PUT UpdateGroup
  → Group list + details refresh
  → User clicks Delete → confirm modal → DELETE DeleteGroup
  → Group list refresh, detail panel clears
```

**Data flow:** Simplest journey. Two reference queries, straightforward CRUD cycle.

---

## 4. Shared / Reference Data

| Data | Source endpoint | Used in views | Fetch strategy |
|------|----------------|---------------|----------------|
| **Users** (enabled only) | `GET /user?enabled=true&disable_pagination=true` | Voci di costo, Centri di costo, Gruppi | Shared query cache, fetch once on app load, invalidate rarely |
| **Cost centers** | `GET /cost-center?disable_pagination=true` | Voci di costo, Centri di costo | Shared query cache, invalidate after CC mutations |
| **Groups** | `GET /group?disable_pagination=true` | Centri di costo, Gruppi | Shared query cache, invalidate after group mutations |

**Recommendation:** These three datasets are small (Phase A Q11) and used across views. Fetch at app level, share via query cache. Invalidate cost centers/groups cache when their respective CRUD operations succeed.

---

## 5. Data Invalidation Map

After each mutation, which queries need to be invalidated:

| Mutation | Invalidate |
|----------|------------|
| NewBudget | Budget list |
| EditBudget | Budget list, Budget details |
| DeleteBudget | Budget list |
| NewUserBudget | Budget details |
| UpdateUserBudget | Budget details |
| NewCostCenterBudget | Budget details |
| UpdateCostCenterBudget | Budget details |
| NewRuleUser | User approval rules list |
| EditRuleUser | User approval rules list |
| DeleteRuleUser | User approval rules list |
| NewRuleCC | CC approval rules list |
| EditRuleCC | CC approval rules list |
| DeleteRuleCC | CC approval rules list |
| NewCostCenter | Cost center list (shared cache) |
| EditCostCenter | Cost center list (shared cache), Cost center details |
| DisableCostCenter / EnableCostCenter | Cost center list (shared cache), Cost center details |
| NewGroup | Group list (shared cache) |
| UpdateGroup | Group list (shared cache), Group details |
| DeleteGroup | Group list (shared cache) |

---

## 6. Hidden Triggers, Timers, Automation

**None found.** The audit shows no:
- Polling or auto-refresh intervals
- WebSocket subscriptions
- Background timers
- Scheduled actions
- Event-driven triggers beyond user interaction

All data fetching is user-initiated (page load or row selection).

---

## 7. Integrations the Audit Cannot Reveal

| Area | What's unknown | Impact |
|------|---------------|--------|
| **Audit logging** | Audit finding (section 4): no audit logging for budget/approval changes. Is this handled server-side, or is it a real gap? | If server-side, no frontend work. If missing, out of scope for frontend but should be flagged to backend team. |
| **Email delivery** | `send_email` flag on approval rules — who sends the email? API or a separate service? | Frontend just sets the flag. Delivery is backend concern. |
| **User provisioning** | How do users get into the system? Keycloak sync? Manual? | Out of scope (Q7), but affects whether user list can become stale. |
| **Budget year rollover** | What happens at year boundary? Are budgets archived? Copied? | Not visible in audit. May affect how budget list grows over time. |

**Q19: Is audit logging for budget changes handled server-side, or is it a known gap?**

**Q20: What happens to budgets at year-end? Are old-year budgets archived, or do they accumulate in the list indefinitely?**

---

## Expert Questions (Phase D)

| # | Question | Context |
|---|----------|---------|
| Q18 | ✅ **Resolved.** Yes — direct consequence of drill-down design. Budget rows in Home alert table link to `/budgets/:id`. | Home → Voci di costo navigation |
| Q19 | ✅ **Resolved.** Out of scope — audit logging is a separate implementation. Tracked in `docs/TODO.md`. | Audit logging |
| Q20 | ✅ **Resolved.** Unresolved domain decision — deferred. Tracked in `docs/TODO.md`. | Budget year-end lifecycle |

---

**Next:** After expert answers, proceed to **Phase E: Specification Assembly**.
