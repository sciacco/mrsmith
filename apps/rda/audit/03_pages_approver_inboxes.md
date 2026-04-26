# Page audit — Approver inboxes

The five approver-only pages share the same shape: a single container, a title and explanation text, and a `tbl_po` table similar to the one on `RDA`, bound to a state-specific list endpoint. Each row links to `PO Details` via `customColumn1` "Gestisci" (icon-button "edit"). The pages have no create/delete buttons and no modals.

The columns visible on these tables are essentially a subset of `RDA.tbl_po`: requester, created (DD/MM/YYYY), code, provider, project, total_price (currency EUR, 2 decimals), and state (translated via `LabelsJS.stateLabel` / `LabelJs.stateLabel` — see Findings: state-map duplication).

## `App. I - II LIV` (slug `app-i-ii-liv`)

- **onLoad:** `get_rda_to_approve` GET `/arak/rda/v1/po/pending-approval` (header `Requester-Email`, `disable_pagination=true`).
- **Title:** "Approvazioni I° / II° livello"
- **Explanation:** "RDA che <u><b>necessitano ancora</b></u> di approvazioni di I° e/o II° livello. Quando l'RDA è approvata per entrambi i livelli sarà rimossa da questa lista."
- **Per-row action:** "Gestisci" → navigate to `PO Details` with `po_id`.
- **JSObject `LabelsJS`:** another duplicate of the state-map; the `stateLabel` JS is identical to RDA's.
- **Permission gating:** none in the UI; the backend list filters by `Requester-Email` so it returns only POs the user can approve. *The new app should rely on backend filtering + role-based route guard, not on a "show table to everyone" pattern.*

## `App. Leasing` (slug `app-leasing`)

- **onLoad:** `get_rda_pendingLeasing` GET `/arak/rda/v1/po/pending-leasing`.
- **Title:** "Approvazioni Leasing".
- Otherwise structurally identical to `App. I - II LIV`. No `LabelsJS` JSObject on this page (state column uses Italian-only or raw); the table copy is essentially the same.

## `App.  incremento Budget` (slug `app-incremento-budget`)

> Two leading spaces in the page name preserved exactly.

- **onLoad:** `get_rda_to_improveBudget` GET `/arak/rda/v1/po-pending-budget-increment` (note the path: it is *not* under `/po/...` — it's `/po-pending-budget-increment` at the same level. **Anomalous endpoint** worth confirming with the backend.)
- The page also keeps a small `Utils` JSObject with an `extractApproverList` helper and the empty stubs `myFun1`/`myFun2`. The helper is the same as the one on `RDA.Utils.extractApproverList` (third copy).

## `App. metodo pagamento` (slug `app-metodo-pagamento`)

- **onLoad:** `get_payment_method` GET `/arak/rda/v1/po/pending-approval-payment-method`.
- Title: "Approvazioni metodo pagamento".

## `App. no Leasing` (slug `app-no-leasing`)

- **onLoad:** `get_rda_pendingLeasing` GET `/arak/rda/v1/po/pending-approval-no-leasing` (note: same JS variable name as the leasing page, but a different REST endpoint).
- A `JSObject1` JSObject duplicates the state-map *again* (fourth copy).

## Migration notes

- All five pages are variants of the same "approver inbox" view. Build a single `<ApproverInbox>` page in React, parameterised by:
  - the API list endpoint
  - the title/copy
  - the route to navigate to (always `PO Details`).
- Hide the inbox entirely from users who don't have the relevant permission flag, using Keycloak roles per `CLAUDE.md` "Keycloak Roles" naming convention (e.g. `app_rda_approver_l1l2`, `app_rda_approver_afc`, `app_rda_approver_no_leasing`, `app_rda_approver_extra_budget`).
- Confirm the URL inconsistency for budget-increment (`/po-pending-budget-increment` vs the rest under `/po/...`) — likely a backend tidy-up opportunity.
- Strip duplicated state-label modules into `packages/ui` or a `apps/rda/src/labels.ts` and share with PO Details + RDA.
