# Energia in DC Implementation Plan

> Source spec: `apps/zammu/energia-in-dc-spec.md` (ready for hand-off as of 2026-04-18).
> Shared boundary context: `apps/zammu/zammu-shared-boundary.md`.
> This plan is the pre-gate artifact for `portal-miniapp-ui-review`.

## Comparable Apps Audit

### Reference 1 — `apps/coperture/src/pages/CoverageLookupPage.tsx`
- **Why comparable:** closest repo precedent for cascading lookup filters, explicit search/reset actions, and a read-only workspace that stays compact instead of turning into a dashboard.
- **Key files inspected:**
  - `apps/coperture/src/pages/CoverageLookupPage.tsx`
  - `apps/coperture/src/App.tsx`
  - `apps/coperture/src/routes.tsx`
  - `apps/coperture/src/main.tsx`
  - `apps/coperture/vite.config.ts`
- **Reused patterns (adopt for Energia in DC):**
  - App shell + top navigation driven by real routes, not a one-page local-tab state machine.
  - Cascading `SingleSelect` filters with downstream reset/disable behavior.
  - Explicit submit/reset actions so the user controls when the working surface refreshes.
  - Separate empty / loading / error / `503` service-unavailable states.
  - Bootstrap pattern reused exactly: `AuthProvider`, `QueryClientProvider`, `BrowserRouter`, `ToastProvider`.
- **Rejected patterns (do NOT adopt):**
  - Coverage hero cards, operator ranking, and badge-heavy result presentation.
  - Decorative highlight panels that visually outweigh the working data.

### Reference 2 — `apps/reports/src/pages/AovPage.tsx`
- **Why comparable:** closest repo precedent for a multi-surface analytic view that combines filters, charts, secondary tabs, and CSV export in one clean workspace.
- **Key files inspected:**
  - `apps/reports/src/pages/AovPage.tsx`
  - `apps/reports/src/App.tsx`
  - `apps/reports/src/routes.tsx`
  - `apps/reports/src/main.tsx`
  - `apps/reports/vite.config.ts`
- **Reused patterns (adopt):**
  - Filter row with an explicit `Aggiorna` action before loading chart data.
  - Secondary tabs used only when they switch between genuinely different data slices.
  - Client-side CSV export of the visible dataset via `Blob`, avoiding a separate authenticated export transport when the current rows are already loaded.
  - Chart + table surfaces sharing one workspace without a dashboard shell around them.
- **Rejected patterns (do NOT adopt):**
  - KPI/stat-card summary row at page level. Energia in DC is task-led, not metric-led.
  - Decorative summary chips that repeat information already visible in the table/chart.

### Reference 3 — `apps/panoramica-cliente/src/pages/IaaSPayPerUsePage.tsx`
- **Why comparable:** best repo precedent for table-selection driving a secondary detail surface, plus chart/table coexistence inside a single clean mini-app page.
- **Key files inspected:**
  - `apps/panoramica-cliente/src/pages/IaaSPayPerUsePage.tsx`
  - `apps/panoramica-cliente/src/App.tsx`
  - `apps/panoramica-cliente/src/main.tsx`
  - `apps/panoramica-cliente/vite.config.ts`
- **Reused patterns (adopt):**
  - Selected-row highlight in a master table that drives a detail panel/table.
  - Side-by-side data surfaces when both are directly part of the task.
  - Simple inline result counts instead of separate metric cards.
  - Chart rendering kept inside the clean theme and subordinate to the data task.
- **Rejected patterns (do NOT adopt):**
  - Auto-selecting the first master row on load. For "Rack senza addebito variabile", detail loading should stay user-driven.
  - Broad account-first landing table before the user expresses intent. Energia in DC needs task-specific route entry points instead.

### Consolidated Reused Patterns
- Route-driven clean app shell using the established `AppShell` + auth bootstrap pattern.
- Filters and actions placed directly next to the data they affect.
- Explicit search/refresh flows for views that combine multiple parameters.
- `SingleSelect`, `Skeleton`, `ToastProvider`, and service-unavailable handling from the existing mini-app family.
- Inline counts and export affordances only when they summarize the exact visible dataset.

### Consolidated Rejected Patterns
- No launcher-style hero banner, premium card, or decorative summary panel.
- No page-level KPI cards for filler.
- No auto-refresh on half-complete form changes for the main work surfaces.
- No English technical labels such as "Site", "Room", "Socket status", or "No variable" in user-facing copy.

## Archetype Choice

- **Selected archetype:** `data_workspace`
- **Why it fits:** Energia in DC is one app coordinating five related, read-only work surfaces:
  - rack inspection with cascading filters, metadata, gauges, chart, and paginated readings
  - kW analytics with parameterized charting
  - billing rows with export
  - a master-detail audit surface
  - a threshold-driven anomaly table
  This is broader than `report_explorer` and not CRUD-shaped, so `data_workspace` is the smallest honest fit.
- **Why not the others:**
  - `report_explorer`: only 2 of the 5 views are primarily report-like; the rack inspection and no-variable audit views are not.
  - `master_detail_crud`: there are no create/update/delete flows.
  - `wizard_flow`: the app is not a sequential task with forward/back progression.
  - `settings_form`: the app is analytical/read-only, not configuration-led.
- **Planned navigation shape:** keep the source tab mental model, but implement it as app-level sub-routes under the shared shell:
  - `/situazione-rack`
  - `/consumi-kw`
  - `/addebiti`
  - `/senza-variabile`
  - `/consumi-bassi`
  Index route redirects to `/situazione-rack`.
- **Navigation component:** top-level `TabNav` in the app shell, wrapped in an app-specific horizontally scrollable nav row on narrow viewports so 5 peer routes remain usable without collapsing into a dropdown.
- **Required states (for UI review and implementation):**
  - app-shell auth states: loading, reauthenticating, access required
  - pre-search empty state where the view is submit-driven
  - populated desktop state for all 5 routes
  - loading state for each route
  - generic backend error state
  - `503` database-unavailable state
  - zero-results state where applicable
  - narrow/mobile navigation state with usable tab overflow and horizontal table scrolling

## User Copy Rules

- **Allowed copy style:** business-user-only Italian, consistent with the clean mini-app family.
- **Canonical labels to use:**
  - App nav:
    - `Situazione rack`
    - `Consumi kW`
    - `Addebiti`
    - `Senza variabile`
    - `Consumi < 1 A`
  - Filter labels:
    - `Cliente`
    - `Edificio`
    - `Sala`
    - `Rack`
    - `Letture dal`
    - `Letture al`
    - `Periodo`
    - `Cos φ`
    - `Soglia minima`
  - Action labels:
    - `Aggiorna`
    - `Reimposta`
    - `Esporta CSV`
    - `Cerca`
  - Table/section labels:
    - `Prese rack`
    - `Storico assorbimenti`
    - `Rack senza addebito variabile`
    - `Prese sotto soglia`
- **Preferred copy choices:**
  - Use `Edificio` instead of `Site`.
  - Use `Sala` instead of `Room`.
  - Use `Presa` or `Presa rack` in user-facing tables/cards while keeping `RackSocket` only in code/types.
  - Use `senza addebito variabile` instead of the English source shorthand `no variable`.
- **Forbidden copy risks:**
  - No implementation language such as `server-side`, `endpoint`, `datasource`, `prepared statement`, `widget`, `record`, `ID-keyed`.
  - No dashboard-like slogans or explanatory intros.
  - No copy that explains how Appsmith behaved or how the rewrite is implemented.
- **Metrics allowed:**
  - Inline result counts such as `24 letture`, `8 addebiti`, `12 prese trovate`, or `3 clienti`.
  - Chart titles/subtitles that reflect the active customer and `Cos φ`.
  - Per-socket utilization gauge is allowed because it is the actual domain signal for the view.
  - Summary cards are **not** allowed.

## Lightweight Validation Gate

> This app is simple enough that it does not need a heavy pre-implementation fixture phase. Use the Appsmith audit plus `docs/grappa/*.json` as the primary source of truth, then pin only the few behaviors most likely to drift.

- **Checks to pin before signoff:**
  - `get_power_readings` + `count_power_reading`: pagination, ordering, merged total count, and local `from` / `to` filtering behavior.
  - `racks_no_variable`: the rewritten detail route must stay keyed by customer ID, not display name.
  - `rack_basso_consumo`: empty-customer behavior must mean "all eligible customers".
  - Rack-reading date filters must accept Europe/Rome local `YYYY-MM-DDTHH:mm` values without timezone conversion, with inclusive `from` / `to` bounds.
- **Execution rule:** implementation can proceed directly from the documented schema in `docs/grappa/`; only the narrow regression checks above are mandatory before signoff.

## Repo-Fit

> Verified against `docs/IMPLEMENTATION-PLANNING.md` and cross-checked with current runtime wiring in `backend/cmd/server/main.go`, `backend/internal/platform/config/config.go`, `backend/internal/platform/applaunch/catalog.go`, `package.json`, `Makefile`, and `deploy/Dockerfile`.

- **Frontend app location:** `apps/energia-dc/`
- **Package name:** `mrsmith-energia-dc`
- **Route / base path:** build base `/apps/energia-dc/`, dev base `/`, with router deep links at `/apps/energia-dc/<route>`.
- **Portal catalog entry:** add `EnergiaDCAppID = "energia-dc"` and `EnergiaDCAppHref = "/apps/energia-dc/"` under category `smart-apps` / title `SMART APPS`.
- **API prefix:** `/api/energia-dc/v1/...`
- **Access role:** `app_energiadc_access`, enforced on every route via `acl.RequireRole(applaunch.EnergiaDCAccessRoles()...)`.
- **Database dependency:** existing `GRAPPA_DSN` only. No new DSN is required.
- **Misconfiguration behavior:** hide the launcher tile when `GRAPPA_DSN == ""`; if a direct route is hit anyway, handlers return `503 energia_dc_database_not_configured`.
- **Self-exclusion config:** add `ENERGIA_DC_EXCLUDED_CUSTOMER_IDS` (comma-separated, default example `3`) to avoid hardcoding the self-row in the no-variable flow. Apply this exclusion only where the spec requires it; do not silently filter the generic active-customer selector.
- **Split-server dev URL override:** add `EnergiaDCAppURL` / env `ENERGIA_DC_APP_URL`; default to `http://localhost:5184` when `StaticDir == ""`.
- **Dev port / proxy:** Vite port `5184`, with `/api` and `/config` proxied to `VITE_DEV_BACKEND_URL || http://localhost:8080`.
- **Backend default CORS:** extend the default `CORS_ORIGINS` list with `http://localhost:5184`.
- **Static hosting / deployment:** add `COPY --from=frontend /app/apps/energia-dc/dist /static/apps/energia-dc` to `deploy/Dockerfile`.
- **Dev wiring updates required:**
  - root `package.json`: add the app to the `dev` `concurrently` command and add `dev:energia-dc`
  - `Makefile`: add `dev-energia-dc`
  - `docker-compose.dev.yaml`: add an `energia-dc` frontend service on port `5184` with `VITE_DEV_BACKEND_URL=http://backend:8080`
  - `backend/internal/platform/config/config.go`: add `EnergiaDCAppURL` and `EnergiaDCExcludedCustomerIDs`
  - `backend/cmd/server/main.go`: add the href override, catalog filtering, and `energiadc.RegisterRoutes(...)`
  - `backend/internal/platform/applaunch/catalog.go`: add app ID/href/access-role definitions
  - `backend/internal/platform/applaunch/catalog_test.go`: update catalog expectations and override coverage
  - `backend/internal/portal/handler_test.go`: update portal launcher visibility expectations
  - `backend/internal/platform/staticspa/handler_test.go`: add a deep-link regression for `/apps/energia-dc/...`
  - `backend/.env.example` and `.env.preprod.example`: document `ENERGIA_DC_APP_URL` and `ENERGIA_DC_EXCLUDED_CUSTOMER_IDS`
- **Frontend runtime bootstrap:** mirror `apps/coperture/src/main.tsx` / `apps/reports/src/main.tsx` exactly for auth bootstrap, query retry rules, and fatal bootstrap rendering.
- **Frontend navigation shell:** mirror the existing `AppShell` + `TabNav` pattern used by `coperture`, `budget`, `quotes`, and `kit-products`; do not invent a standalone shell.

## Implementation Breakdown

### Frontend Structure

- **App shell and routing**
  - `src/main.tsx`: same auth/bootstrap/query structure as Coperture/Reports.
  - `src/App.tsx`: `AppShell`, `TabNav`, `useOptionalAuth`, reauth/access-required cards.
  - `src/routes.tsx`: 5 route objects + index redirect + fallback redirect.
  - `src/navigation.ts` is optional; with 5 peer tabs, an inline nav array in `App.tsx` is acceptable.

- **Shared client modules**
  - `src/api/client.ts`: shared `useApiClient` wrapper.
  - `src/api/types.ts`: frontend types for customer/site/room/rack, socket status, reading page, chart rows, addebiti, no-variable customer/rack, low-consumption row.
  - `src/api/queries.ts`: React Query hooks with narrow keys per route.
  - `src/components/ServiceUnavailable.tsx`: reuse the existing app-local pattern from Coperture/Panoramica.
  - `src/components/ResultMeta.tsx` or equivalent small helper for inline counts/export row meta.

### View 1 — `SituazioneRackPage`

- **Routing:** `/situazione-rack`
- **Layout:** compact title + filter toolbar, then a two-row workspace:
  - metadata card
  - socket utilization list/grid
  - paginated readings table
  - 2-day ampere/kW trend chart
- **Filter behavior:**
  - Cascading selects: customer -> building -> room -> rack.
  - Date range fields default to yesterday -> now.
  - Date inputs use `datetime-local`; submitted values serialize as local Europe/Rome `YYYY-MM-DDTHH:mm` values without offset.
  - Downstream selections reset when upstream values change.
  - Main data queries do not run on every selector change; they use a submitted filter snapshot set by `Aggiorna`.
- **Hooks/data slices:**
  - `useCustomers()`
  - `useSites(customerId)`
  - `useRooms(siteId, customerId)`
  - `useRacks(roomId, customerId)`
  - `useRackDetail(submittedRackId)`
  - `useRackSocketStatus(submittedRackId)`
  - `useRackStatsLastDays(submittedRackId)`
  - `usePowerReadings({ rackId, from, to, page, size })`
- **UI rules:**
  - Socket gauge stays domain-led: percent is `ampere / (maxampere / 2) * 100`; red above 90%.
  - The paginated readings table follows the submitted `from` / `to` range, while the "ultimi due giorni" ampere/kW chart remains fixed to the last-two-days endpoint.
  - Metadata and table do not disappear until a new submitted filter starts loading; use draft-vs-submitted state to avoid flicker/stale mixed surfaces.
  - Table pagination stays server-driven.

### View 2 — `ConsumiKwPage`

- **Routing:** `/consumi-kw`
- **Layout:** compact title + controls row + bar chart surface.
- **Controls:** customer select, period select (`day|month` only), `Cos φ` slider 70-100, `Aggiorna`.
- **Data contract:** one endpoint `GET /api/energia-dc/v1/customers/{id}/kw?period=day|month&cosfi=`.
- **Chart behavior:**
  - Title/subtitle reflect customer and selected `Cos φ`.
  - Day/month share one response shape.
  - Weekly option is not rendered anywhere.
- **View state:** pre-submit empty copy plus loading/error/service-unavailable handling.

### View 3 — `AddebitiPage`

- **Routing:** `/addebiti`
- **Layout:** customer filter row, inline result meta, results table.
- **Interaction:** selecting a customer is enough to load the table; no extra submit button is needed here.
- **Export:** CSV only in v1, generated client-side from the visible rows. No PDF action, no separate backend file endpoint.
  - CSV uses `;` as delimiter, includes a UTF-8 BOM, preserves the current visible row order, and uses the same date/number formatting shown in the table.
- **Columns:** start/end period, ampere, eccedenti, importo, PUN, coefficiente, fisso CU, importo eccedenti.
- **State rule:** export button appears only when rows exist and exports exactly what is visible.

### View 4 — `SenzaVariabilePage`

- **Routing:** `/senza-variabile`
- **Layout:** master-detail workspace with two adjacent tables on desktop and stacked tables on narrow screens.
- **Interaction:**
  - master table auto-loads the customer list on page entry
  - detail table loads only after explicit row selection
  - selected customer row stays highlighted
  - do not auto-select the first customer
- **API contract:**
  - `GET /api/energia-dc/v1/no-variable-billing/customers`
  - `GET /api/energia-dc/v1/no-variable-billing/customers/{id}/racks`
- **Business rule:** detail query is ID-keyed only; no display-name lookup remains anywhere in the flow.

### View 5 — `ConsumiBassiPage`

- **Routing:** `/consumi-bassi`
- **Layout:** threshold/customer filter row, `Cerca` action, results table.
- **Defaults:** threshold defaults to `1`.
- **Interaction:** customer filter is optional; empty means all eligible customers.
- **Scope:** read-only in v1. No bulk actions, no ticketing flow, no inline rack actions.

### Backend Module Layout

- **Package:** `backend/internal/energiadc/`
- **Recommended file split:**
  - `handler.go` — route registration, ACL, DB/config guards
  - `handler_lookups.go` — customers, sites, rooms, racks
  - `handler_rack.go` — rack detail, socket status, readings, last-days stats
  - `handler_kw.go` — kW summary endpoint
  - `handler_billing.go` — addebiti endpoint
  - `handler_audit.go` — no-variable and low-consumption endpoints
  - `types.go` — response/input structs
  - `config.go` or module-local config struct — excluded customer IDs, timezone location
- **Dependency pattern:** inject `*sql.DB` and module config explicitly; no package-global state.
- **Route registration pattern:** match `backend/internal/coperture/handler.go` and `backend/internal/reports/handler.go`.

### Data and Domain Rules

- **Keep confirmed business rules exactly as pinned by the spec:**
  - `kW = SUM(ampere) * 225 / 1000`
  - no weekly kW period
  - `Cos φ` is integer percent 70-100; backend applies `/ 100`
  - `maxampere / 2` remains the gauge safety-margin denominator
- **Backend-owned derivations:**
  - breaker mapping from `magnetotermico`
  - per-reading/page totals
  - kW aggregates
  - no-variable self-exclusion config
- **Recommended implementation detail:** keep breaker mapping as a module-owned lookup map with an explicit fallback of `32`, and pin it with tests. Do not expand scope into a new DB lookup table in v1.
- **Nested-resource invariants:**
  - `GET /customers/{id}/sites` returns only buildings that still have matching active rack-socket data for that customer.
  - `GET /sites/{id}/rooms?customerId=` must verify site/customer -> room ownership in SQL.
  - `GET /rooms/{id}/racks?customerId=` must verify room/customer -> rack ownership in SQL.
  - Detail/report endpoints stay ID-keyed after the constrained lookup chain; no display-label-based ownership checks are reintroduced.
- **Timezone contract:** parse and apply all incoming date/time filters as local Europe/Rome `YYYY-MM-DDTHH:mm` values; keep response timestamps/date strings aligned with that zone and perform no timezone conversion.
- **Nullable/scan discipline:** scan nullable text/numeric columns defensively where the schema or production history makes nulls plausible; do not assume legacy views are perfectly non-null.
- **Deterministic ordering:** every endpoint that feeds a selector/table should declare an explicit `ORDER BY`.

## Exceptions

- **Charting library deviation:** although the source Appsmith page used ECharts, implementation should reuse `recharts` first, matching the existing repo precedent in `apps/panoramica-cliente`. User benefit: one less charting stack, cleaner maintenance, and consistent clean-theme styling. Revisit only if the log-scale kW chart or dual-axis rack chart cannot be expressed cleanly enough.
- **Five source tabs become five sub-routes, not one local-tab page.** This is not a visual deviation from the source mental model; it is the repo-fit way to preserve deep links, refresh safety, and app-shell navigation consistency.
- **No automatic first-row selection in "Senza variabile".** User benefit: avoids unexpected detail queries and makes the audit surface feel deliberate.
- **No KPI cards anywhere in the app.** User benefit: the workspace stays focused on the operational data instead of filler summaries.

## Verification

### UI Review Checks
- Desktop populated state for all 5 routes.
- Pre-submit empty state for `Situazione rack`, `Consumi kW`, and `Consumi < 1 A`.
- Empty-results state for `Addebiti`, `Senza variabile` detail, and `Consumi < 1 A`.
- Generic backend error state and `503` database-unavailable state.
- Narrow/mobile state showing:
  - horizontally scrollable top nav
  - wrapped filter controls
  - horizontally scrollable tables
  - stacked master/detail layout for `Senza variabile`
- Verify no KPI cards, no hero panels, and no technical copy leaks into the UI.

### Runtime / Auth Checks
- Deep-link refresh works for:
  - `/apps/energia-dc/situazione-rack`
  - `/apps/energia-dc/consumi-kw`
  - `/apps/energia-dc/addebiti`
  - `/apps/energia-dc/senza-variabile`
  - `/apps/energia-dc/consumi-bassi`
- Launcher tile appears only for users with `app_energiadc_access`.
- Requests without auth return `401`; requests with auth but without role return `403`.
- With `GRAPPA_DSN` unset, the launcher tile is hidden and direct route hits return `503 energia_dc_database_not_configured`.
- `/config` bootstrap and `/api` requests both work in split-server dev on port `5184`.
- Date range filtering is verified against local Europe/Rome inputs without timezone conversion.

### Tests

- **Backend handler/query tests**
  - `RegisterRoutes` ACL coverage: missing claims, missing role, valid role.
  - nil-DB/service-unavailable coverage returning `energia_dc_database_not_configured`.
  - narrow regression tests for:
    - power-readings ordering, pagination, and total-count merge
    - no-variable detail keyed by customer ID
    - low-consumption optional-customer behavior
  - formula regression tests for:
    - breaker mapping
    - gauge input normalization helpers
    - `Cos φ` percent conversion
    - 225V kW aggregation helper
  - validation tests for invalid `period`, out-of-range `cosfi`, invalid pagination params, malformed date range inputs, and wrong local datetime shapes.
  - regression test asserting the no-variable detail route is keyed by customer ID, not display name.
  - regression test asserting excluded-customer config affects only the no-variable customer list.
  - regression tests for nested-resource invariants on `sites -> rooms` and `rooms -> racks`.

- **Platform wiring tests**
  - `backend/internal/platform/applaunch/catalog_test.go`
  - `backend/internal/portal/handler_test.go`
  - `backend/internal/platform/staticspa/handler_test.go`

- **Frontend verification**
  - Type-check/build: `pnpm --filter mrsmith-energia-dc lint` and `pnpm --filter mrsmith-energia-dc build`
  - Manual route checks for all 5 pages in split-server mode and static-served mode
  - Manual CSV export check on `Addebiti`
  - Manual cascade-reset check on `Situazione rack`
  - Manual local-datetime filter check on `Situazione rack` using Europe/Rome values without timezone conversion
  - Manual master-detail selection check on `Senza variabile`

### Handoff
- This document is the pre-gate planning artifact for `portal-miniapp-ui-review`.
- After pre-gate approval, implementation should move to `portal-miniapp-ui-fixer` with this plan, the cited comparable files, and the explicit exceptions above.
- Post-gate review is still required after implementation.
