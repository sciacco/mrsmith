# Orchestrated Implementation Plan Prompt

You are to produce a very detailed implementation plan for the following project.

This source file is a prescriptive implementation plan for the MrSmith monorepo. Treat it as an execution contract, not as loose background context. Preserve exact strings, named sections, rationale, explicit prohibitions, and repo-fit details. Do not soften `must`, `do not`, `preserve`, `keep`, `remove`, `leave untouched`, `exact`, `not required`, or similar wording. Preserve the source slice names and map any finer decomposition back to them.

## Project Brief (Source Of Truth, Preserve As Named Concepts)

# Customer Portal Back-office Implementation Plan (`cp-backoffice`)

Source: `apps/customer-portal/SPEC.md`

Status: Draft for pre-gate review

Checked against:
- `docs/IMPLEMENTATION-PLANNING.md`
- `docs/IMPLEMENTATION-KNOWLEDGE.md`
- `docs/UI-UX.md`

## Comparable Apps Audit

- Reference 1:
  - `apps/budget/src/views/gruppi/GruppiPage.tsx`
  - `apps/budget/src/App.tsx`
  - `apps/budget/src/routes.tsx`
  - Why it is relevant: compact admin workspace with one primary list surface, strong empty states, bounded modal actions, and a clean mini-app shell.
- Reference 2:
  - `apps/listini-e-sconti/src/pages/GruppiScontoPage.tsx`
  - `apps/listini-e-sconti/src/App.tsx`
  - `apps/listini-e-sconti/src/routes.tsx`
  - Why it is relevant: customer selection gates dependent data, the page stays table-first, and mutations remain bounded to a modal instead of inventing dashboard chrome.
- Reference 3:
  - `apps/compliance/src/views/blocks/BlocksPage.tsx`
  - `apps/compliance/src/App.tsx`
  - `apps/compliance/src/routes.tsx`
  - Why it is relevant: admin registry with list/detail behavior, search and filter support, and the same clean master/detail family expected for new mini-apps.
- Rejected pattern reference:
  - `apps/reports/src/pages/OrdiniPage.tsx`
  - Rejected because its summary metrics and report-explorer framing are correct for reports but wrong for this admin companion app.

- Reused patterns:
  - `AppShell` with standard mini-app navigation, not a bespoke launcher or Appsmith clone.
  - Compact page header with one business subtitle.
  - One primary table/list surface per route.
  - Empty and no-selection prompts that explain the next business action.
  - Modal-backed mutations for bounded write flows.
  - Clean-theme spacing, surfaces, and typography already used by budget, compliance, and listini.
- Rejected patterns:
  - KPI rows, stat cards, or report-style summaries.
  - Launcher-style hero banners or visual treatments.
  - A bespoke left sidebar copied from Appsmith.
  - A full detail page or sticky save bar for flows that only need a modal or row-level save.

## Archetype Choice

- Selected archetype: `master_detail_crud`
- Why it fits:
  - The app is an admin workspace made of three registry-style routes: customer state management, user management, and biometric request management.
  - Each route is table-first and built around select, inspect, and bounded update actions.
  - `master_detail_crud` is the smallest approved archetype that fits the work. `data_workspace` would be broader than needed and would make it easier to justify summary shells that this app does not need.
  - The fact that the app has multiple routes does not change the route-level shape: each route still behaves like a CRUD/admin screen.
- Required states:
  - Populated desktop state for all three routes.
  - Loading and upstream-unavailable state for all three routes.
  - No-selection state for `Gestione Utenti`.
  - Empty-data state for each table when the backend returns zero rows.
  - Modal-open state for `Stato Aziende` and `Nuovo Admin`.
  - Row-edit state for `Accessi Biometrico` with Save and Discard visible.
  - Narrow viewport state with horizontal table scrolling.
  - Destructive-confirm state: not required in v1 because the app has no delete flow.

## User Copy Rules

- Allowed copy style: `business-user-only`, in Italian.
- Preserve source-facing business labels where they are already task-oriented:
  - `Stato Aziende`
  - `Gestione Utenti`
  - `Accessi Biometrico`
  - `Aggiorna`
  - `Conferma`
  - `Nuovo Admin`
  - `Crea`
  - `Perfetto, stato biometrico cambiato`
  - `Qualcosa e' andato storto`
- Preserve the existing weak but business-facing biometric column labels verbatim for the 1:1 port:
  - `nome`
  - `cognome`
  - `email`
  - `azienda`
  - `tipo_richiesta`
  - `stato_richiesta`
  - `data conferma`
  - `data della richiesta`
- Greeting copy may use the operator display name or email from Keycloak, but it should talk about the current task, not about auth mechanics.
- Planned greeting copy uses a minimal clarity patch and drops the ambiguous reference to the end-user app:
  - `Ciao {operator.name || operator.email}, in questa applicazione vengono visualizzati tutti gli utenti inseriti per l'azienda selezionata - da indicare tramite la select`
- The lowercase biometric labels are a deliberate v1 parity decision, not a design endorsement. Post-port polish is tracked in `docs/TODO.md`.
- Forbidden copy risks:
  - `server-side`
  - `datasource`
  - `widget`
  - `record`
  - `id.asc`
  - `Arak`
  - `Mistra`
  - `Keycloak`
  - `replica dell'app originale`
  - any text that explains implementation mechanics instead of the task the operator is performing
- Metrics allowed: none.

## Repo-Fit

- Frontend app path:
  - Implement the SPA in `apps/cp-backoffice/`.
  - Package name should follow the existing pattern: `mrsmith-cp-backoffice`.
  - `pnpm-workspace.yaml` already includes `apps/*`, so no workspace config change is needed.
  - Keep the migration workspace in `apps/customer-portal/` and add `apps/customer-portal/README.md` pointing to `apps/cp-backoffice/`, following the same split-workspace pattern already used by `apps/zammu/`.
- Route/base path:
  - Build base: `/apps/cp-backoffice/`
  - Dev base: `/`
  - Client routes:
    - `/stato-aziende`
    - `/gestione-utenti`
    - `/accessi-biometrico`
  - Index route redirects to `/stato-aziende`.
  - The dropped `Home` page is not reintroduced.
- App shell and navigation:
  - Use the standard mini-app shell with top navigation.
  - For three routes, `TabNav` is sufficient; `TabNavGroup` is not necessary.
  - Route labels should remain the business labels above.
- API prefix:
  - Use repo-standard versioned routes under `/api/cp-backoffice/v1/`.
  - This is a deliberate slug = API-prefix choice. The repo has exceptions such as `listini` and `panoramica`, but this app follows the same full-slug namespace pattern as `afc-tools`, `energia-dc`, and `kit-products`.
  - Proposed backend paths:
    - `GET /api/cp-backoffice/v1/customers`
    - `GET /api/cp-backoffice/v1/customer-states`
    - `PUT /api/cp-backoffice/v1/customers/{id}/state`
    - `GET /api/cp-backoffice/v1/users?customer_id=...`
    - `POST /api/cp-backoffice/v1/admins`
    - `GET /api/cp-backoffice/v1/biometric-requests`
    - `POST /api/cp-backoffice/v1/biometric-requests/{id}/completion`
- Backend module shape:
  - Add `backend/internal/cpbackoffice/`.
  - Register routes via `cpbackoffice.RegisterRoutes(...)` from `backend/cmd/server/main.go`.
  - Dependency shape should match repo practice:
    - `Arak *arak.Client` for Mistra NG calls
    - `Mistra *sql.DB` for biometric SQL
  - Add helper guards such as `requireArak`, `requireMistra`, and `dbFailure` instead of package-global state.
- Access role:
  - Planned app role: `app_cpbackoffice_access`
  - This follows the repo pattern of compact, hyphen-free role ids.
- Identifier strategy:
  - All identifiers are upstream-owned.
  - This app creates no primary keys.
  - Request bodies stay DTO-shaped for the upstream Mistra NG contracts.
- Dev port / proxy notes:
  - Vite port: `5187`
  - Proxy both `/api` and `/config` to `process.env.VITE_DEV_BACKEND_URL || http://localhost:8080`
  - Add `http://localhost:5187` to the default CORS origins in `backend/internal/platform/config/config.go`
  - Add root scripts and targets:
    - `package.json` -> `dev:cp-backoffice`
    - root `dev` concurrently command -> include `cp-backoffice`
    - root `package.json` concurrently `--names` and `--prefix-colors` lists must grow in lockstep
    - `Makefile` -> `dev-cp-backoffice`
- Launcher/catalog wiring:
  - Add a SMART APPS entry in `backend/internal/platform/applaunch/catalog.go`
  - App id: `cp-backoffice`
  - Href: `/apps/cp-backoffice/`
  - Icon: `users`
  - Status: `ready`
  - Access roles: `CPBackofficeAccessRoles()`
  - Remove the superseded commented `customer-portal` placeholder entry.
  - Leave the commented `customer-portal-settings` placeholder untouched until a separate spec exists for that distinct app.
  - Add split-server href override in `backend/cmd/server/main.go` to `http://localhost:5187` when `StaticDir == ""`
- Static hosting / deployment notes:
  - `deploy/Dockerfile` must add:
    - `COPY --from=frontend /app/apps/cp-backoffice/dist /static/apps/cp-backoffice`
  - Add Go field `CPBackofficeAppURL` and env var `CP_BACKOFFICE_APP_URL` to:
    - `backend/internal/platform/config/config.go`
    - `backend/.env.example`
    - `.env.preprod.example`
  - No new DSN env var is required because the app reuses:
    - `MISTRA_DSN`
    - `ARAK_BASE_URL`
    - `ARAK_SERVICE_CLIENT_ID`
    - `ARAK_SERVICE_CLIENT_SECRET`
    - `ARAK_SERVICE_TOKEN_URL`
- Runtime visibility gating:
  - Hide the launcher tile when the app cannot work end to end.
  - Minimum dependency rule for catalog visibility: `arakCli != nil` and `MistraDSN` present.
  - Individual handlers should still return `503` when a dependency is missing, but the launcher should not advertise a broken app.
- Observability and error surfacing:
  - Internal 5xx responses use `httputil.InternalError`.
  - Server logs keep the real cause with `component="cpbackoffice"` plus an `operation` field.
  - Access log, request ID, recover middleware, and auth middleware all apply automatically because the module mounts under the shared `/api` mux in `backend/cmd/server/main.go`.

## Pre-Code Verifications

- Confirm the Mistra NG error body shape still exposes a `message` field for the user-facing toast formats pinned in the spec.
- Confirm Vite port `5187` is not already claimed by any local override or in-flight app scaffolding.

## Implementation Slices

### Slice 1: App Scaffolding And Shell

- Create the frontend scaffold in `apps/cp-backoffice/` with the standard Vite + React mini-app shape already used by budget, compliance, and listini.
- Reuse the standard auth bootstrap from `/config` and the clean theme from `@mrsmith/ui`.
- Implement:
  - `src/main.tsx`
  - `src/App.tsx`
  - `src/routes.tsx`
  - `src/navigation.ts`
  - `apps/customer-portal/README.md`
- Use `AppShell` plus a simple `TabNav` with:
  - `Stato Aziende`
  - `Gestione Utenti`
  - `Accessi Biometrico`
- Keep the shell consistent with the existing mini-app family instead of reproducing Appsmith layout chrome.

### Slice 2: Backend Package And Contract Boundaries

- Add `backend/internal/cpbackoffice/handler.go` as the mount point and route registration file.
- Keep contract code typed rather than anonymous pass-through maps where practical.
- Split responsibilities cleanly:
  - Arak-backed handlers for customers, states, users, and admin creation
  - DB-backed handlers for biometric list and completion
- Reuse repo-standard helpers for:
  - ACL enforcement
  - request validation
  - dependency guards
  - sanitized internal-error responses with server logs carrying the real cause
  - `httputil.InternalError` with `component="cpbackoffice"` and route-level `operation` values

### Slice 3: Mistra NG Proxy Flows

- Implement typed handlers for:
  - customer list
  - customer-state list
  - customer state update
  - user list by customer
  - admin creation
- Use the existing `backend/internal/platform/arak` client for every upstream REST call.
- Preserve these non-negotiable upstream semantics:
  - `disable_pagination=true`
  - full list responses, no frontend pagination added in v1
  - upstream `message` surfaced for business errors
- Add an explicit backend guard for `GET /users`:
  - reject missing or empty `customer_id`
  - do not proxy an invalid empty request upstream
- For `createAdmin`, the v1 UI does not expose the hidden Appsmith `skip_keycloak` switch.
  - Request assembly pins `skip_keycloak: false` to match observed operator behavior.
  - The re-enablement path is tracked in `docs/TODO.md`.

### Slice 4: Biometric Request DB Flows

- Add a DB-backed query for `GET /biometric-requests` using the exact source join and alias shape:
  - `customers.biometric_request`
  - `customers.user_struct`
  - `customers.customer`
  - `customers.user_entrance_detail`
- Preserve exact response keys because the frontend should not invent a second DTO vocabulary:
  - `id`
  - `nome`
  - `cognome`
  - `email`
  - `azienda`
  - `tipo_richiesta`
  - `stato_richiesta`
  - `data_richiesta`
  - `data_approvazione`
  - `is_biometric_lenel`
- Preserve these behavioral rules:
  - `ORDER BY data_richiesta DESC`
  - `stato_richiesta` stays boolean end to end
  - `is_biometric_lenel` is returned but not rendered
- For completion updates, call `customers.biometric_request_set_completed($1::bigint, $2::boolean)` and return `{ ok: true }` on success.
- V1 preserves the unpaginated biometric list from source.
  - This is accepted as a post-port risk, not an accidental omission.
  - A defensive ceiling / filtering follow-up is tracked in `docs/TODO.md`.

### Slice 5a: Stato Aziende

- Table-first route.
- Selected-row call to action uses `Aggiorna {selectedCustomer.name}`.
- Modal select is backed by the prefetched customer-state list.
- On success: refetch list, close modal.
- On error: preserve the business-facing toast format with HTTP status and message.

### Slice 5b: Gestione Utenti

- Customer select first, user table second.
- No user request runs until a customer is selected.
- `Nuovo Admin` is disabled until a customer is selected.
- The greeting keeps the task framing but drops the ambiguous phrase `sul Customer Portal`.
- The modal includes:
  - `Nome`
  - `Cognome`
  - `Em@il`
  - `Telefono`
  - notification checkboxes
- UI keys `'maintenance'` and `'marketing'` are not part of the DTO.
  - They map locally onto `maintenance_on_primary_email` and `marketing_on_primary_email`.
- The hidden Appsmith `skip_keycloak` switch is not rendered in v1.
- Request assembly sets `skip_keycloak: false`.

### Slice 5c: Accessi Biometrico

- Flat table with editable checkbox column for `stato_richiesta`.
- Row-level Save and Discard actions mirror the current flow.
- Save triggers mutation then refetch.
- Discard is local only.
- The table must remain usable on narrow widths via horizontal scroll, not cardification.
- The UI keeps the lowercase source column labels in v1 for parity; post-port polish is tracked in `docs/TODO.md`.

## Contract Locks

- The browser must never call `gw-int.cdlan.net` directly.
- The browser must never connect to Mistra PostgreSQL directly.
- The app must not reintroduce the dropped `Home` page.
- `Area documentale` remains out of scope.
- Dead source handlers are not ported:
  - `howAlert('success')`
  - `onCheckChange`
  - `JSObject1`
  - `JSObject2`
  - `Api1`
  - `Query1`
- `Nuovo Admin` is disabled until a customer is selected.
- The user list fetch is deferred until a customer is selected.
- The hidden Appsmith `skip_keycloak` switch stays omitted in v1.
- `createAdmin` request assembly pins `skip_keycloak: false`.
- `BiometricRequestRow` keys and boolean types are fixed.
- No KPI cards or decorative summaries are added.

## Exceptions

- Exception 1:
  - The source app uses a sidebar-like navigation shell.
  - The implementation should use the standard MrSmith mini-app top navigation instead.
  - User benefit: consistency with the rest of the mini-app family and less bespoke UI surface to maintain.
- Exception 2:
  - `Accessi Biometrico` keeps inline row Save and Discard actions inside a `master_detail_crud` app.
  - User benefit: preserves the current operator flow without forcing a modal or full detail page that the task does not need.
- Exception 3:
  - `Accessi Biometrico` keeps the lowercase source column labels in v1 despite the audit's presentation-gap note.
  - User benefit: preserves exact operator-facing labels during the 1:1 port window.
  - Follow-up polish is tracked in `docs/TODO.md`.

## Verification

- UI review checks:
  - Comparable-app references are concrete and repo-local.
  - Exactly one primary archetype is chosen: `master_detail_crud`.
  - No KPI cards or stat rows are introduced.
  - Copy remains business-facing, with the hidden `skip_keycloak` switch omitted and the greeting ambiguity resolved.
  - Layout stays inside the clean mini-app family.
  - Route, API prefix, role shape, dev port, and static path are all explicit.
- Runtime and auth checks:
  - `GET /config` bootstrap works in split-server dev on port `5187`.
  - Deep-link refresh works at `/apps/cp-backoffice/` and at nested routes.
  - All `/api/cp-backoffice/v1/*` routes require `app_cpbackoffice_access`.
  - The launcher tile is hidden when Arak or Mistra DB configuration is missing.
  - Browser network traffic shows only local `/api` calls, never direct gateway or DB access.
  - `createAdmin` request assembly sends `skip_keycloak: false` in v1.
  - Internal failures surface via `httputil.InternalError`, with the real cause preserved in server logs under `component="cpbackoffice"`.
- Tests:
  - Backend handler test for auth gating on the new route group.
  - Backend test for Arak proxy request composition:
    - correct path
    - correct query string
    - correct request body for state update and admin creation, including `skip_keycloak: false`
  - Backend test for biometric list scanning and ordering, including nullable approval date handling.
  - Backend test for biometric completion mutation calling the stored function with `bigint + boolean`.
  - No broad snapshot or copy-only tests.
- Manual review artifacts required before implementation signoff:
  - Populated state for all three routes.
  - Empty and no-selection state.
  - Upstream error state.
  - Modal-open state.
  - Inline row-edit state for biometric requests.
  - Narrow viewport state.

## Extracted Planning Fields

- Project summary:
  - Build and launch a new MrSmith mini-app SPA `cp-backoffice` that ports the back-office surface from the migration workspace into a standard top-nav mini-app with three registry-style routes, a dedicated `/api/cp-backoffice/v1/` backend module, launcher wiring, deployment/static hosting updates, and parity-preserving operator workflows.
- In scope:
  - Frontend scaffold in `apps/cp-backoffice/`
  - App shell, routes, navigation, and route-first UX for `Stato Aziende`, `Gestione Utenti`, and `Accessi Biometrico`
  - Backend package `backend/internal/cpbackoffice/` and route registration
  - Arak-backed customer/state/user/admin flows
  - Mistra DB-backed biometric list and completion flows
  - Root dev wiring, Vite proxy/base config, CORS, launcher/catalog registration, split-server href override, Docker static copy, config/env exposure, and dependency-based launcher visibility gating
  - Verification artifacts and the explicitly requested backend tests
- Out of scope:
  - Reintroducing the dropped `Home` page
  - `Area documentale`
  - KPI cards, stat rows, report-style summaries, hero banners, or bespoke Appsmith-like shell chrome
  - Browser-direct access to `gw-int.cdlan.net` or Mistra PostgreSQL
  - Exposing the hidden Appsmith `skip_keycloak` switch in the v1 UI
  - Porting dead source handlers `howAlert('success')`, `onCheckChange`, `JSObject1`, `JSObject2`, `Api1`, `Query1`
  - Post-port polish already tracked in `docs/TODO.md`, including biometric label cleanup and biometric list hardening
- Tech stack:
  - Monorepo with pnpm workspaces
  - Vite + React frontend mini-app
  - Shared theme/components from `@mrsmith/ui`
  - Go backend monolith with shared `/api` mux
  - `backend/internal/platform/arak` client for Mistra NG
  - `sql.DB` access to Mistra PostgreSQL for biometric flows
  - `/config` bootstrap and shared auth middleware / Keycloak-backed access control
  - Docker static hosting under `/static/apps/cp-backoffice`
- Repo context:
  - Frontend app path `apps/cp-backoffice/`; migration workspace retained in `apps/customer-portal/`
  - Backend package `backend/internal/cpbackoffice/`
  - Route registration in `backend/cmd/server/main.go`
  - Catalog wiring in `backend/internal/platform/applaunch/catalog.go`
  - Config/env wiring in `backend/internal/platform/config/config.go`, `backend/.env.example`, `.env.preprod.example`
  - Docker deployment update in `deploy/Dockerfile`
  - Root dev wiring in `package.json` and `Makefile`
- Non-functional requirements:
  - UI must stay inside the approved clean mini-app family and selected `master_detail_crud` archetype
  - All app routes under `/api/cp-backoffice/v1/*` require `app_cpbackoffice_access`
  - Launcher tile hidden unless `arakCli != nil` and `MistraDSN` is present
  - Individual handlers return `503` when dependencies are missing
  - Internal failures use `httputil.InternalError` with server logs carrying `component="cpbackoffice"` and `operation`
  - Deep-link refresh must work under `/apps/cp-backoffice/` and nested routes
  - Narrow-width tables use horizontal scroll, not cardification
  - Browser traffic must stay on local `/api` calls only
  - v1 preserves upstream unpaginated behavior where explicitly locked
- Milestones/deadlines:
  - No explicit external deadline is given in the brief.
  - Assume the desired output is an execution-ready plan suitable for pre-gate review, then slice execution with per-slice QA and a final end-to-end QA pass.
- Documentation requirements:
  - Add `apps/customer-portal/README.md` pointing to `apps/cp-backoffice/`
  - Reflect required env/config additions in the config/example files named above
  - Preserve and reference follow-up items already tracked in `docs/TODO.md`
  - Make documentation updates explicit in each slice of the generated plan
- Known risks:
  - Need to verify that Mistra NG error bodies still expose a `message` field
  - Vite port `5187` may already be claimed locally
  - Biometric list remains unpaginated in v1 by deliberate tradeoff
  - Lowercase biometric labels are intentionally preserved for v1 parity even though they are a presentation gap
  - The hidden `skip_keycloak` behavior is pinned off in v1 and must not accidentally reappear
- Decision rationale / intent that must survive decomposition:
  - Comparable-app review anchors and why they matter:
    - `budget` for compact admin shell, one primary list surface, bounded modal actions, strong empty states
    - `listini-e-sconti` for customer selection gating and modal-bounded mutations
    - `compliance` for admin registry behavior, search/filter affordances, and clean master/detail family fit
    - `reports/OrdiniPage` is explicitly rejected because report framing and summary metrics are wrong for this app
  - Archetype choice rationale / why it fits:
    - `master_detail_crud` is the smallest approved archetype that matches the three route-level CRUD/admin screens without inviting report shells or dashboards
  - Repo-fit pattern rationale / why specific conventions were chosen:
    - Use full slug `cp-backoffice` for app id, base path, and API prefix
    - Keep standard mini-app top navigation and `TabNav`
    - Reuse existing Arak and Mistra dependency patterns and shared middleware instead of bespoke infrastructure
  - Exception rationale / user benefit:
    - Top nav replaces source sidebar for family consistency and lower maintenance
    - Inline Save/Discard remains in biometric route to preserve existing operator flow
    - Lowercase biometric labels remain in v1 to preserve exact operator-facing parity during the port window
  - Accepted v1 tradeoffs, follow-up pointers, and tracked TODOs:
    - Biometric label polish deferred to `docs/TODO.md`
    - `skip_keycloak` re-enablement path deferred to `docs/TODO.md`
    - Biometric list defensive ceiling/filtering deferred to `docs/TODO.md`
  - Pre-gate or review framing that affects acceptance or signoff:
    - The brief is explicitly a draft for pre-gate review
    - Verification requires comparable-app proof, archetype proof, runtime/auth checks, backend tests, and named manual review artifacts before signoff

## Locked Implementation Details To Preserve Exactly

- User copy and label rules:
  - Allowed copy style is `business-user-only`, in Italian
  - Preserve exact route/action/success/error labels:
    - `Stato Aziende`
    - `Gestione Utenti`
    - `Accessi Biometrico`
    - `Aggiorna`
    - `Conferma`
    - `Nuovo Admin`
    - `Crea`
    - `Perfetto, stato biometrico cambiato`
    - `Qualcosa e' andato storto`
  - Preserve exact biometric column labels in v1:
    - `nome`
    - `cognome`
    - `email`
    - `azienda`
    - `tipo_richiesta`
    - `stato_richiesta`
    - `data conferma`
    - `data della richiesta`
  - Preserve greeting intent and the exact planned greeting copy:
    - `Ciao {operator.name || operator.email}, in questa applicazione vengono visualizzati tutti gli utenti inseriti per l'azienda selezionata - da indicare tramite la select`
  - Never introduce forbidden copy such as `server-side`, `datasource`, `widget`, `record`, `id.asc`, `Arak`, `Mistra`, `Keycloak`, `replica dell'app originale`, or implementation-mechanics language
  - Metrics are not allowed
- Repo-fit specifics:
  - Frontend app lives in `apps/cp-backoffice/`
  - Keep `apps/customer-portal/` as migration workspace and add `apps/customer-portal/README.md` pointer
  - Package name `mrsmith-cp-backoffice`
  - Build base `/apps/cp-backoffice/`; dev base `/`
  - Client routes `/stato-aziende`, `/gestione-utenti`, `/accessi-biometrico`; index redirects to `/stato-aziende`
  - Use `AppShell` and `TabNav`, not `TabNavGroup`
  - API prefix `/api/cp-backoffice/v1/`
  - Add backend package `backend/internal/cpbackoffice/`
  - Register `cpbackoffice.RegisterRoutes(...)` from `backend/cmd/server/main.go`
  - Dependency shape uses `Arak *arak.Client` and `Mistra *sql.DB`
  - Use helper guards `requireArak`, `requireMistra`, `dbFailure`
  - Access role `app_cpbackoffice_access`
  - Vite port `5187`
  - Proxy `/api` and `/config` to `process.env.VITE_DEV_BACKEND_URL || http://localhost:8080`
  - Add `http://localhost:5187` to default CORS origins
  - Add root scripts/targets:
    - `package.json` -> `dev:cp-backoffice`
    - root `dev` concurrently command includes `cp-backoffice`
    - root `--names` and `--prefix-colors` lists grow in lockstep
    - `Makefile` -> `dev-cp-backoffice`
  - Add SMART APPS catalog entry with:
    - app id `cp-backoffice`
    - href `/apps/cp-backoffice/`
    - icon `users`
    - status `ready`
    - access roles `CPBackofficeAccessRoles()`
  - Remove superseded commented `customer-portal` placeholder; leave commented `customer-portal-settings` placeholder untouched
  - Add split-server href override to `http://localhost:5187` when `StaticDir == ""`
  - Add Docker static copy:
    - `COPY --from=frontend /app/apps/cp-backoffice/dist /static/apps/cp-backoffice`
  - Add `CPBackofficeAppURL` / `CP_BACKOFFICE_APP_URL` to config and env example files
  - Reuse existing env vars `MISTRA_DSN`, `ARAK_BASE_URL`, `ARAK_SERVICE_CLIENT_ID`, `ARAK_SERVICE_CLIENT_SECRET`, `ARAK_SERVICE_TOKEN_URL`
  - Hide launcher tile unless `arakCli != nil` and `MistraDSN` is present
  - Use `httputil.InternalError` and structured logging with `component="cpbackoffice"` and `operation`
- Exact files, paths, modules, and ownership anchors:
  - `apps/cp-backoffice/src/main.tsx`
  - `apps/cp-backoffice/src/App.tsx`
  - `apps/cp-backoffice/src/routes.tsx`
  - `apps/cp-backoffice/src/navigation.ts`
  - `apps/customer-portal/README.md`
  - `backend/internal/cpbackoffice/handler.go`
  - `backend/cmd/server/main.go`
  - `backend/internal/platform/applaunch/catalog.go`
  - `backend/internal/platform/config/config.go`
  - `backend/.env.example`
  - `.env.preprod.example`
  - `deploy/Dockerfile`
  - root `package.json`
  - root `Makefile`
- API, contract, query-param, flag, helper, env var, and SQL locks:
  - Endpoints:
    - `GET /api/cp-backoffice/v1/customers`
    - `GET /api/cp-backoffice/v1/customer-states`
    - `PUT /api/cp-backoffice/v1/customers/{id}/state`
    - `GET /api/cp-backoffice/v1/users?customer_id=...`
    - `POST /api/cp-backoffice/v1/admins`
    - `GET /api/cp-backoffice/v1/biometric-requests`
    - `POST /api/cp-backoffice/v1/biometric-requests/{id}/completion`
  - Preserve `disable_pagination=true`
  - `GET /users` must reject missing or empty `customer_id`
  - `createAdmin` must pin `skip_keycloak: false`
  - Biometric query must preserve table/join anchors:
    - `customers.biometric_request`
    - `customers.user_struct`
    - `customers.customer`
    - `customers.user_entrance_detail`
  - Biometric response keys are fixed:
    - `id`
    - `nome`
    - `cognome`
    - `email`
    - `azienda`
    - `tipo_richiesta`
    - `stato_richiesta`
    - `data_richiesta`
    - `data_approvazione`
    - `is_biometric_lenel`
  - Preserve `ORDER BY data_richiesta DESC`
  - `stato_richiesta` stays boolean end to end
  - Return `is_biometric_lenel` without rendering it
  - Completion mutation calls `customers.biometric_request_set_completed($1::bigint, $2::boolean)` and returns `{ ok: true }`
- Slice-level behavioral locks:
  - Slice 1:
    - Standard mini-app scaffold and auth bootstrap
    - `AppShell` + simple `TabNav`
    - No Appsmith shell recreation
  - Slice 2:
    - Typed backend contracts
    - Clean Arak-vs-DB split
    - Shared ACL/validation/dependency/error helpers
  - Slice 3:
    - Typed Mistra NG proxy handlers
    - Full list responses with no frontend pagination in v1
    - Business errors surface upstream `message`
    - `skip_keycloak` hidden in UI and forced false in requests
  - Slice 4:
    - Exact biometric DTO vocabulary
    - Unpaginated list preserved in v1
    - `is_biometric_lenel` returned but not rendered
  - Slice 5a:
    - Table-first route
    - CTA `Aggiorna {selectedCustomer.name}`
    - Success refetch+close
    - Error toast preserves HTTP status + message
  - Slice 5b:
    - No user fetch until customer selection
    - `Nuovo Admin` disabled until customer selection
    - Modal fields `Nome`, `Cognome`, `Em@il`, `Telefono`, plus notification checkboxes
    - Local UI keys `'maintenance'` and `'marketing'` map to `maintenance_on_primary_email` and `marketing_on_primary_email`
  - Slice 5c:
    - Flat table with editable checkbox on `stato_richiesta`
    - Row-level Save and Discard
    - Save refetches, Discard is local only
    - Horizontal scroll on narrow widths
    - Lowercase source labels preserved in v1
- Exceptions:
  - Replace source sidebar-like nav with standard mini-app top nav
  - Keep inline Save/Discard in biometric route inside `master_detail_crud`
  - Keep lowercase biometric labels in v1
  - Preserve the user-benefit rationale attached to each exception
- Verification and test specifics:
  - UI review must confirm comparable-app evidence, one archetype (`master_detail_crud`), no KPI/stat rows, business-facing copy, omitted `skip_keycloak`, clean mini-app family fit, and explicit route/API/role/port/static-path decisions
  - Runtime/auth checks must confirm `/config` bootstrap in split-server dev on `5187`, deep-link refresh, route auth, hidden launcher tile on missing deps, local-only browser API traffic, `skip_keycloak: false`, and structured internal error handling
  - Required tests:
    - backend auth gating test for the new route group
    - backend Arak proxy composition tests for path, query string, and request bodies including `skip_keycloak: false`
    - backend biometric list scanning and ordering test including nullable approval date handling
    - backend biometric completion mutation test with `bigint + boolean`
  - Required manual artifacts:
    - populated state for all three routes
    - empty and no-selection state
    - upstream error state
    - modal-open state
    - inline row-edit state for biometric requests
    - narrow viewport state
- Explicit prohibitions / negative constraints:
  - Do not call `gw-int.cdlan.net` directly from the browser
  - Do not connect to Mistra PostgreSQL directly from the browser
  - Do not reintroduce the dropped `Home` page
  - Do not add KPI cards, stat rows, decorative summaries, launcher hero banners, or bespoke Appsmith clone chrome
  - Do not expose `skip_keycloak` in the v1 UI
  - Do not port dead source handlers listed above
  - Do not invent a second biometric DTO vocabulary
  - Do not change locked response keys, boolean types, labels, routes, role id, env var names, file paths, or SQL objects
- Assumptions for missing information:
  - No external deadline or fixed delivery date is specified; optimize the plan for clear sequencing, parallelism where safe, and pre-gate readiness
  - No additional hidden scope beyond the named routes, backend endpoints, and repo wiring should be introduced
  - If implementation requires more granular slicing than the source uses, keep the source slice names as canonical parents and map each derived slice back to them explicitly

## Work Rules

- Execution of this plan must be assigned to an Orchestrator agent.
- The Orchestrator must only supervise and orchestrate; it must not implement any slice.
- The Orchestrator must spawn specialized sub-agents to execute each slice.
- Each slice must be QA-reviewed by a QA sub-agent against the source brief, repo-fit constraints, and the rationale attached to locked decisions.
- If QA fails, iterate the slice until QA passes.
- After all slices are complete, QA passed, and documentation updated, the Orchestrator must spawn a final QA agent to review the entire work and produce a final report.
- Use the source `Implementation Slices` as the canonical baseline.
- You may split a source slice into smaller executable slices only when the split materially improves ownership, parallelism, or risk control.
- If you split a source slice, explicitly map every derived slice back to its source slice and show that all locked behavior remains covered.
- Preserve named source sections as named concepts when they affect execution, QA, or signoff, especially `Comparable Apps Audit`, `Archetype Choice`, `User Copy Rules`, `Repo-Fit`, `Pre-Code Verifications`, `Implementation Slices`, `Contract Locks`, `Exceptions`, and `Verification`.

## Output Requirements

- Provide a step-by-step plan with independent, well-scoped slices.
- Preserve rationale when it constrains decomposition, architecture, QA, or acceptance criteria. Do not keep only the locked value and drop the reason it was chosen.
- Do not generalize away exact labels, identifiers, file paths, env vars, helper names, role ids, package names, query params, flags, SQL objects, or explicit removal/retention instructions.
- Each slice must include:
  - Objective
  - Why this slice boundary is correct
  - Owned files/modules/packages
  - Inputs and dependencies
  - Implementation tasks (very detailed)
  - Acceptance criteria
  - Test plan
  - Manual QA or review artifacts required
  - Documentation updates required
  - Assigned executor sub-agent role
  - Assigned QA sub-agent role
  - QA checklist
  - Rollback or containment notes when the slice is risky
- Include a dependency graph or ordered sequencing between slices.
- Make parallelizable slices explicit and identify the critical path.
- Include a risk register with mitigations.
- Include a rollout plan and rollback plan.
- Include a final validation checklist.
- Explicitly call out assumptions or missing information.
- In the final validation checklist, require confirmation that all locked copy, labels, routes, APIs, flags, SQL objects, exceptions, tests, and repo-fit decisions survived intact, together with the rationale for why they exist.

## Orchestration Protocol

- Define the Orchestrator's responsibilities and limits.
- Define the QA workflow per slice and the final end-to-end QA process.
- Make iteration rules for QA failures explicit.
- Require QA to verify both correctness and repo fit.
- Require QA to verify exact preservation of:
  - copy rules and forbidden terms
  - archetype choice and comparable-app rationale
  - repo-fit paths, role id, app id, routes, API prefix, base paths, port, env vars, config fields, helper names, and deployment wiring
  - contract locks such as `disable_pagination=true`, `skip_keycloak: false`, and biometric DTO/SQL rules
  - explicit exceptions and the user-benefit rationale behind them
  - verification requirements, required tests, and manual review artifacts
- Require the final QA pass to confirm that locked source details and the rationale attached to them were preserved, including exact copy rules, exceptions, tests, repo-fit constraints, and why those constraints exist.

## Formatting

- Use clear headings and numbered sections.
- Avoid vague language; be concrete and specific.
- Assume a large, complex implementation.
