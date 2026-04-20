# Customer Portal Back-office (`cp-backoffice`) — Application Specification

Platform-neutral spec ready to hand to `portal-miniapp-generator` (or any implementation team) for a 1:1 port of the Appsmith source, with dead features excluded.

## Summary
- **Application name / display**: Customer Portal Back-office (back-office admin companion to the end-user Customer Portal). Retains Italian copy.
- **Slug**: `cp-backoffice` (used uniformly for the Vite app folder, package.json script `dev:cp-backoffice`, Makefile target `dev-cp-backoffice`, catalog `ID: "cp-backoffice"`, `Href: "/apps/cp-backoffice/"`, Go config field `CPBackofficeAppURL`, env var `CP_BACKOFFICE_APP_URL`).
- **Keycloak access role**: `app_cpbackoffice_access` (CLAUDE.md convention, collapsed form matching `app_kitproducts_access`, `app_afctools_access`, etc.).
- **Audit source**: `apps/customer-portal/audit/` (APPLICATION_INVENTORY.md, DATASOURCE_CATALOG.md, PAGE_AUDITS.md, FINDINGS.md), cross-checked against `docs/mistra-dist.yaml` and `docs/mistradb/mistra_customers.json`.
- **Spec status**: ready for implementation planning.
- **Last updated decisions** (expert):
  - Gestione Utenti: **defer** the user fetch until a customer is selected (fix the unguarded on-load fetch).
  - Accessi Biometrico: **drop** the `howAlert('success')` dead toast and the `onCheckChange` defensive no-op.
  - Home page: **drop**; default route is Stato Aziende.
  - Mistra NG auth: **use the existing `backend/internal/platform/arak` client** (already consumed by `afctools`). Nothing new to build on the auth front.
  - `Nuovo Admin` button is **disabled until a customer is selected** in `select_customer`.
  - Hidden `skip_keycloak` Appsmith switch stays omitted in the v1 port; `createAdmin` request assembly pins `skip_keycloak=false`.

## Current-State Evidence
- **Source pages/views**: Home, Stato Aziende, Gestione Utenti, Accessi Biometrico, Area documentale.
- **Source entities and operations**: Customer (list + edit state), CustomerState (list), User/Admin (list by customer + create admin), BiometricRequest (list + set completed). All other entities referenced by the export are join-only (`user_struct`, `user_entrance_detail`) or out-of-scope (Category, Document — from the dead `Area documentale`).
- **Source integrations and datasources**:
  - REST: Mistra NG Internal API (`https://gw-int.cdlan.net`).
  - SQL: Mistra PostgreSQL direct connection (`10.129.32.20`, `customers` schema).
- **Known audit gaps or ambiguities**:
  - Auth model for `gw-int.cdlan.net`: **already solved in-repo.** `backend/internal/platform/arak` implements an OAuth2 client-credentials client (Keycloak service account → bearer token with caching, refresh, and 401 retry) and is the canonical way to call Mistra NG from the Go backend. Config env vars: `ARAK_BASE_URL`, `ARAK_SERVICE_CLIENT_ID`, `ARAK_SERVICE_CLIENT_SECRET`, `ARAK_SERVICE_TOKEN_URL`. Reference consumer: `backend/internal/afctools/gateway.go` (`proxyGatewayPDF`).
  - **Resolved**: the stored function `customers.biometric_request_set_completed(bigint, boolean)` performs only `UPDATE biometric_request SET request_completed=$2, request_approval_date=now()`. No Lenel sync, no notifications, no audit log. The audit's "likely has hidden side effects" concern does not apply; sibling functions `biometric_request_set_notified_dc/_user` are not called by this app.

## Scope rule
**1:1 port of the wired flows; dead features ignored.**

### Excluded from the port (dead in source)
- `Area documentale` page (no bound data, no button handlers).
- `JSObject1` and `JSObject2` plus every `UNUSED_DATASOURCE` actionList entry on Accessi Biometrico (`onSave`, `onToggle`, `onDiscard`, `onToggleCompletion`, `onSaveCompletion`, `Table1primaryColumnsstato_richiestaonCheckChange`).
- `Api1` (empty-path REST action).
- `Query1` (`SELECT * FROM accounting."acct_log" LIMIT 10` — scratch).
- `Home` page (decision above).
- `EditActions1.onSave` typo `howAlert('success')` post-refetch toast (never fires).
- `stato_richiesta.onCheckChange` defensive no-op.

### Behavior corrections (called out explicitly)
- Gestione Utenti: user fetch is deferred until a customer is selected. Observable UX is identical to today's empty-initial-state; the correction is that no invalid request hits the server on load.

## Entity Catalog

### Entity: Customer
- **Purpose**: corporate customer record as owned by Mistra NG.
- **Operations**: `list`, `editState`.
- **Fields and inferred types** (from `docs/mistra-dist.yaml#/components/schemas/customer`):
  - `id: int64`
  - `name: string`
  - `language: enum("it","en")`
  - `group: customer-group { id, name, is_default, is_partner, variables[] }`
  - `state: customer-state { id, name, enabled }`
  - `variables: { resource, access_type }[]`
- **Relationships**: owns `User/Admin` (via `customer_id`), owns `BiometricRequest` (via `customer_id`).
- **Constraints and business rules**: allowed state transitions enforced server-side. Only `state_id` is mutated from this app.
- **Open questions**: none for the port.

### Entity: CustomerState
- **Purpose**: lifecycle-state lookup for customers.
- **Operations**: `list`.
- **Fields**: `id: int64`, `name: string`, `enabled: bool`.
- **Relationships**: referenced by `Customer.state` and `customer-edit.state_id`.
- **Constraints and business rules**: list is consumed verbatim as select options; the UI does not filter by `enabled` (1:1 preserved).
- **Open questions**: none for the port.

### Entity: User / Admin
- **Purpose**: end-user (or admin) records scoped to a customer.
- **Operations**: `listByCustomer`, `createAdmin`.
- **Fields on the list DTO** (`user-brief`): `id`, `customer_id`, `first_name`, `last_name`, `email`, `enabled`, `role { id, name, color }`, `phone?`, `created`, `last_login?`.
- **Fields on the create DTO** (`user-admin-new` as composed by the UI): `first_name`, `last_name`, `email`, `customer_id`, `phone`, `maintenance_on_primary_email: bool`, `marketing_on_primary_email: bool`, `skip_keycloak: bool`. Unused by this app: `biometric` (default false at the API).
- **Relationships**: belongs to `Customer`; carries a `role` reference.
- **Constraints and business rules**:
  - The notification checkbox group uses hard-coded value keys `"maintenance"` and `"marketing"`; these are internal to the form and map directly to the two DTO booleans.
  - `skip_keycloak=true` suppresses Keycloak provisioning at the upstream API contract level, but the source widget is hidden (`isVisible=false`) and operators do not see this control today.
  - The v1 port does not render the switch and pins `skip_keycloak=false` in `createAdmin` requests to match observed operator behavior.
  - `enabled` is read-only in this app. No edit/delete/disable path.
  - The `Nuovo Admin` button is disabled while no customer is selected.
- **Open questions**: none for the port.

### Entity: BiometricRequest (+ joined context)
- **Purpose**: physical-access biometric requests to be marked completed/not-completed by back-office staff.
- **Operations**: `list`, `setCompleted`.
- **Fields on the list DTO** (exact aliases, not renamed under 1:1):
  - `id: int`
  - `nome: string` (`user_struct.first_name`)
  - `cognome: string` (`user_struct.last_name`)
  - `email: string` (`user_struct.primary_email`)
  - `azienda: string` (`customer.name`)
  - `tipo_richiesta: string` (enum value from `request_type_enum`)
  - `stato_richiesta: bool` (`request_completed`)
  - `data_richiesta: timestamptz`
  - `data_approvazione: timestamptz?`
  - `is_biometric_lenel: bool` (computed; included in the DTO but not displayed in the UI).
- **Order**: `ORDER BY data_richiesta DESC`. No pagination, no filters.
- **Relationships**: belongs to `Customer` and `UserStruct`; joined with `UserEntranceDetail` (on `email`) for the Lenel flag.
- **Constraints and business rules**:
  - `stato_richiesta` is **boolean** end-to-end. The dead "ok"/"pending" string form from JSObject1 is not part of this port.
  - `request_approval_date` reflects the last time `setCompleted` was called; the stored function sets it unconditionally to `now()`.
  - `is_biometric_lenel` is available in the DTO; rendering is hidden (matches source).
- **Open questions**: none — stored-function body verified.

### Entities intentionally out of scope
- **UserStruct**, **UserEntranceDetail**: join-only, no direct operations. No dedicated DTO in the new backend.
- **Category**, **Document**, **Area / Tipologia utenti**: referenced only by the dead `Area documentale` page. Not part of this migration.

## View Specifications

### View: Stato Aziende (default route)
- **User intent**: select a customer and change its lifecycle state.
- **Interaction pattern**: list + modal editor.
- **Main data shown or edited**:
  - Table columns (labels verbatim): `id`, `Lingua`, `Ragione sociale`, `Tipologia` (= `group.name`), `Stato` (= `state.name`).
  - Hidden-but-carried: `group`, `variables`, raw `state`.
- **Key actions**:
  - Row selection updates the button label: `Aggiorna {selectedRow.name}`.
  - Click `Aggiorna …` → opens modal.
  - In modal: pick new state (options = CustomerState list, labelled by `name`, valued by `id`).
  - Click `Conferma` → `editState(id, state_id)` → on success refetch + close; on error toast `"Failed to edit Customer, [HTTP {status}]: {data.message}"`.
- **Entry and exit points**: reached from sidebar; also the app's default route. Exits via sidebar.
- **Notes on current vs intended behavior**: no new `isDisabled` guard on the `Aggiorna …` button (1:1 preserved). Server rejects an empty-selection attempt.

### View: Gestione Utenti
- **User intent**: inspect users of a chosen customer and optionally create a new admin.
- **Interaction pattern**: master select → dependent list → modal form.
- **Main data shown or edited**:
  - `select_customer`: options from Customer list.
  - `t_user_list`: columns `Creato il` (raw `created`), `email`, `Accesso CP abilitato` (checkbox on `enabled`), `Nome`, `Cognome`, `nome ruolo` (= `role.name`). Hidden: `customer_id`, `id`, `phone`, raw `role`.
  - Greeting text: `Ciao {operator.name || operator.email}, in questa applicazione vengono visualizzati tutti gli utenti inseriti per l'azienda selezionata - da indicare tramite la select`. Operator identity comes from Keycloak.
- **Key actions**:
  - Changing the selected customer re-fetches users.
  - `Nuovo Admin` → modal with: Nome, Cognome, Em@il, Telefono, notifications checkbox group `[{label:"Manutenzioni", value:"maintenance"}, {label:"Marketing", value:"marketing"}]`.
  - `Crea` → `createAdmin(body)` → on success refetch users + close; on error toast `"Failed to create Admin, [HTTP {status}]: {data.message}"`.
- **Entry and exit points**: reached from sidebar; modal is the only secondary surface.
- **Notes on current vs intended behavior**:
  - **Corrected**: no user fetch fires until a customer is selected (the source fires one with empty `customer_id`).
  - `Nuovo Admin` is disabled while no customer is selected (expert-confirmed).
  - Greeting operator identity is sourced from Keycloak (`preferred_username` / `email`), replacing Appsmith's `appsmith.user`.
  - The hidden Appsmith `skip_keycloak` switch is omitted in v1; request assembly pins `skip_keycloak=false`.

### View: Accessi Biometrico
- **User intent**: mark biometric requests completed/not-completed.
- **Interaction pattern**: flat table with per-row inline toggle + Save/Discard (Appsmith `EditActions` pattern).
- **Main data shown or edited**:
  - Columns (labels verbatim): `nome`, `cognome`, `email`, `azienda`, `tipo_richiesta`, `stato_richiesta` (checkbox, editable), `data conferma` (formatted `data_approvazione`), `data della richiesta` (formatted `data_richiesta`), `Save / Discard` actions.
  - Dates formatted as `new Date(x).toLocaleString()` — browser-locale dependent, preserved 1:1.
  - Hidden: `id`, `is_biometric_lenel`.
- **Key actions**:
  - Toggle `stato_richiesta` → row enters edit mode.
  - Save → `setCompleted(id, completed)` → refetch, single success toast `"Perfetto, stato biometrico cambiato"`. On error: `"Qualcosa e' andato storto"`.
  - Discard → local revert (no server call).
- **Entry and exit points**: reached from sidebar; no secondary surfaces.
- **Notes on current vs intended behavior**: no `onCheckChange` handler is wired (the source's was a no-op); the single success toast fires immediately after the mutation resolves, matching observed behavior today.

### Views explicitly excluded
- **Home** (dropped by expert decision; default route redirects to Stato Aziende).
- **Area documentale** (dead scaffold; not part of this migration).

## Logic Allocation
- **Backend responsibilities** (Go `backend/`):
  - Terminate every Mistra call. Browser never hits `gw-int.cdlan.net` or Mistra PostgreSQL directly.
  - Use `platform/arak` for every Mistra NG REST call (customers, customer-states, users, admins). Inject the `*arak.Client` through the mini-app's handler dependencies, as `afctools` already does.
  - Own the biometric-requests list query and the `setCompleted` stored-function call against Mistra PostgreSQL (not via Arak — this is a direct DB path).
  - Pass-through semantics for all Mistra NG endpoints: the backend does not invent new validation, role checks, or transformations beyond what Mistra NG already enforces.
- **Frontend responsibilities** (React mini-app):
  - Page routing (default = Stato Aziende).
  - UI state: row selection, modal open/close, form inputs.
  - Form composition → DTO assembly (matches existing Mistra NG contracts exactly).
  - Presentation concerns: column labels (Italian, verbatim), nested-object unwraps (`group.name`, `state.name`, `role.name`), date formatting (`toLocaleString()`).
  - Error toasts with HTTP status + API `data.message`.
  - Deferred fetches (users list gated on selected customer).
- **Shared validation or formatting**: none. All field-level validation is server-side per the existing API; no shared contract layer is introduced.
- **Rules being revised rather than ported**:
  - Deferred user fetch (Gestione Utenti) — correctness patch.
  - Dead handlers (`onCheckChange` no-op, `howAlert` typo) — not reproduced.
  - Home page — dropped.

## Integrations and Data Flow

### External systems
- **Mistra NG Internal API** (`https://gw-int.cdlan.net`) — customers, customer-states, users, admins. Consumed through the existing `backend/internal/platform/arak` client (OAuth2 client-credentials, Keycloak-backed).
- **Mistra PostgreSQL** (`10.129.32.20`, `customers` schema) — biometric requests (via new backend endpoints).

### End-to-end user journeys
1. **Change customer state** — Stato Aziende: list customers + states → select row → open modal → pick new state → confirm → PUT → refetch.
2. **Create admin user** — Gestione Utenti: list customers → pick one → list users → open `Nuovo Admin` modal → fill → POST → refetch.
3. **Mark biometric request completed** — Accessi Biometrico: list requests → toggle row → Save → stored-function call → refetch.

### Background or triggered processes
- None observable. Strictly request-response.

### Data ownership boundaries
- Mistra NG owns all REST-exposed entities (customer, customer-state, user, admin).
- Mistra PostgreSQL owns `biometric_request` and its companion tables.
- The new frontend owns no persistent state; it holds UI state only.

## API Contract Summary
Suggested paths (names decided at implementation time; shapes are non-negotiable under 1:1).

Following `docs/API-CONVENTIONS.md` §"Namespacing": `/api/<app-prefix>/v1/...`. App prefix matches the slug: `cp-backoffice`. In module Go code routes are registered without the `/api` prefix (`backend/cmd/server/main.go:370` strips it off the mounted API mux).

| Method | Full URL | Upstream | Input → Output |
|---|---|---|---|
| GET | `/api/cp-backoffice/v1/customers` | Mistra NG `GET /customers/v2/customer?page_number=1&disable_pagination=true` | — → `{ items: customer[] }` |
| GET | `/api/cp-backoffice/v1/customer-states` | Mistra NG `GET /customers/v2/customer-state?page_number=1&disable_pagination=true` | — → `{ items: customer-state[] }` |
| PUT | `/api/cp-backoffice/v1/customers/{id}/state` | Mistra NG `PUT /customers/v2/customer/{id}` with `{ state_id }` | `{ state_id: int }` → Mistra `message` |
| GET | `/api/cp-backoffice/v1/users?customer_id=…` | Mistra NG `GET /users/v2/user?customer_id=…&disable_pagination=true` | `customer_id` → `{ items: user-brief[] }` |
| POST | `/api/cp-backoffice/v1/admins` | Mistra NG `POST /users/v2/admin` | `user-admin-new` → `{ id }` |
| GET | `/api/cp-backoffice/v1/biometric-requests` | Mistra PostgreSQL (same SELECT as source) | — → `{ items: BiometricRequestRow[] }` |
| POST | `/api/cp-backoffice/v1/biometric-requests/{id}/completion` | Mistra PostgreSQL `SELECT customers.biometric_request_set_completed($1::bigint, $2::boolean)` | `{ completed: bool }` → `{ ok: true }` |

All routes are gated by `app_cpbackoffice_access` via `acl.RequireRole(applaunch.CPBackofficeAccessRoles()...)`, matching the `afctools` pattern in `backend/internal/afctools/handler.go:37`. Frontend calls go through `@mrsmith/api-client` with `baseUrl: '/api'` (same-origin); auth bootstrap from `GET /config`; Vite proxies both `/api` and `/config` during dev.

Non-negotiable shapes under 1:1:
- `BiometricRequestRow` keys and types exactly as in Phase A.
- `stato_richiesta` and `completed` are booleans.
- `disable_pagination=true` semantics (no paging).
- `user-admin-new` remains the upstream-compatible shape; the v1 port pins `skip_keycloak=false` and omits the unused `biometric`/`state` fields.

## Constraints and Non-Functional Requirements
- **Security / compliance**:
  - Trust-boundary move is mandatory: neither Mistra PostgreSQL nor `gw-int.cdlan.net` may be called from the browser. All calls go through the Go backend.
  - The hidden Appsmith `skip_keycloak` switch is not exposed in v1; `createAdmin` requests pin `skip_keycloak=false`.
  - No host, IP, or credential may land in the frontend bundle.
- **Performance / scale**: unchanged. Mistra NG list endpoints use `disable_pagination=true`; this is acceptable at current data volumes (per source). No caching beyond what Mistra already provides. The biometric-request list remains unpaginated in v1 and a defensive-ceiling follow-up is tracked in `docs/TODO.md`.
- **Operational constraints**:
  - New app must follow the New App Checklist in `CLAUDE.md`. With slug `cp-backoffice`, that expands to:
    - `package.json` (root): add `dev:cp-backoffice` script and a `cp-backoffice` entry in the `dev` concurrently command (name + color + filter).
    - `Makefile`: add `dev-cp-backoffice` target and append it to `.PHONY`.
    - `backend/internal/platform/applaunch/catalog.go`: add `CPBackofficeAppID`, `CPBackofficeAppHref`, `CPBackofficeAccessRoles()`, and the catalog entry (category `SMART APPS` — matches the commented placeholder at line 243). Remove the superseded commented `customer-portal` placeholder; leave the distinct commented `customer-portal-settings` placeholder untouched until a separate spec exists for that app.
    - `backend/cmd/server/main.go`: add the package import, a `hrefOverrides["cp-backoffice"]` dev-port mapping, the catalog filter condition, and a `RegisterRoutes` call for the new handler.
    - `backend/internal/platform/config/config.go`: add `CPBackofficeAppURL` field + `CP_BACKOFFICE_APP_URL` env var.
  - Keycloak access role: `app_cpbackoffice_access`.
- **UX or accessibility expectations**:
  - Italian copy preserved verbatim, including weak column labels on Accessi Biometrico (`nome`, `cognome`, …) — relabeling is a deliberate post-migration decision.
  - Date rendering is `toLocaleString()` (browser locale) — preserved.
  - Must follow `docs/UI-UX.md` conventions for layout primitives (table, modal, sidebar), while preserving labels/copy as above.

## Open Questions and Deferred Decisions
None that block implementation planning.

## Acceptance Notes
- **What the audit proved directly**:
  - All live endpoints, SQL statements, widget bindings, and DTO shapes in Phases A–D.
  - Full inventory of dead features (listed in Scope rule).
- **What the expert confirmed** (this session):
  - Defer users fetch until a customer is selected.
  - Drop the `howAlert` and `onCheckChange` dead handlers.
  - Drop the Home page; default route is Stato Aziende.
  - Reuse the existing `platform/arak` client for Mistra NG auth.
  - `Nuovo Admin` button is disabled until a customer is selected.
  - Hidden `skip_keycloak` switch omitted in v1; `createAdmin` requests pin `skip_keycloak=false`.
  - Slug `cp-backoffice`, Keycloak role `app_cpbackoffice_access`.
- **What still needs validation**:
  - None for spec purposes. Implementation-time details (backend package layout, frontend routing library, shared UI components from `@mrsmith/ui`) are handed to `portal-miniapp-generator`.
