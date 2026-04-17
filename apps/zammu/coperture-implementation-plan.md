# Coperture Implementation Plan

> Source spec: `apps/zammu/coperture-spec.md` (approved 2026-04-17).
> Shared boundary context: `apps/zammu/zammu-shared-boundary.md`.
> This plan is the pre-gate artifact for `portal-miniapp-ui-review`.

## Comparable Apps Audit

### Reference 1 — `apps/panoramica-cliente/src/pages/AccessiPage.tsx`
- **Why comparable:** filter toolbar (multi-selects) + explicit "Cerca" button + results table with empty/loading/error states. Read-only view. Closest UX shape to Coperture.
- **Key files inspected:**
  - `apps/panoramica-cliente/src/pages/AccessiPage.tsx`
  - `apps/panoramica-cliente/src/pages/shared.module.css` (styles)
  - `apps/panoramica-cliente/vite.config.ts`
- **Reused patterns (adopt for Coperture):**
  - Top-level `<div className={s.page}>` with a single-row `<div className={s.toolbar}>` holding filters + primary action button.
  - Button stays disabled until all required filter levels are chosen.
  - Empty state triggered by a `searchTriggered` boolean; hint copy guides the user ("Seleziona ... poi premi Cerca").
  - Table wrapped in `<div className={s.tableWrap}>` with `rowEnter` staggered animation via inline `animationDelay`.
  - `ApiError`-aware handling (`status === 503`) and a shared `ServiceUnavailable` component for database-down screens.
- **Rejected patterns (do NOT adopt for Coperture):**
  - CSV/export button (Coperture spec §API has no export in v1).
  - Sortable columns + client-side `useTableFilter` (Coperture results are small — per-address coverage is typically ≤ 10 rows; column sort would be noise).

### Reference 2 — `apps/reports/src/pages/OrdiniPage.tsx`
- **Why comparable:** the explicit reference app for `report_explorer` in `references/archetypes.md`; same filter → preview → table sequence.
- **Key files inspected:**
  - `apps/reports/src/pages/OrdiniPage.tsx`
  - `apps/reports/src/pages/OrdiniPage.module.css`
  - `apps/reports/src/pages/shared.module.css`
  - `apps/reports/vite.config.ts`
- **Reused patterns (adopt):**
  - `shared.module.css` scaffolding: `.page`, `.title`, `.toolbar`, `.field`, `.tableWrap`, `.table`, `.empty`, `.btnSecondary`.
  - `useCallback` wrappers for the search and any export handlers; `useMemo` for derived display data.
  - Inline `animationDelay` capped (e.g. `Math.min(i * 15, 300)`) for staggered row entry.
- **Rejected patterns (do NOT adopt):**
  - `styles.metrics` / `.metric` KPI cards — Coperture has no aggregate that is user-relevant at single-address scale.
  - `styles.summary` / `.chips` status breakdown — no analogous grouping exists for coverage rows.
  - XLSX export button — out of scope.

## Archetype Choice

- **Selected archetype:** `report_explorer`
- **Why it fits:** Coperture is a filter-driven, read-only preview surface. Default composition of `report_explorer` matches the spec one-for-one:
  - concise header ("Ricerca copertura")
  - report filters (the 4-level cascading select + "Cerca")
  - preview surface (the results list rendering operator logo, tech, profiles, detail list)
  - side actions: **none in v1** (export is deferred — see Exceptions)
  - metrics: **none** (per gate rules, metrics are only allowed when user-relevant and justified by the spec; none qualifies here)
- **Why not the others:**
  - `master_detail_crud`: no create/update/delete operation exists in the spec.
  - `data_workspace`: requires "multiple related data panels"; Coperture has a single working surface.
  - `wizard_flow`: the cascading select is auto-advancing on change, not stepped with back/forward navigation.
  - `settings_form`: screen is not configuration-shaped.
- **Required states (for UI review):**
  - populated desktop (filters filled, results rendered)
  - empty before search ("Seleziona un indirizzo completo e premi Cerca per visualizzare i profili disponibili.")
  - empty after search with zero results ("Nessuna copertura disponibile per questo civico.")
  - loading (skeleton/shimmer on the results region)
  - error (network / 5xx — generic "Errore nel caricamento dei dati. Riprova.")
  - 503 service unavailable (when the `dbcoperture` connection is not reachable — use a `ServiceUnavailable` pattern similar to Panoramica)
  - narrow-viewport / responsive stack (toolbar wraps; table becomes horizontally scrollable via `tableWrap`)

## User Copy Rules

- **Allowed copy style:** business-user-only Italian, matching the repo default. Examples:
  - Page title: "Ricerca copertura"
  - Filter labels: "Provincia", "Comune", "Indirizzo", "Numero civico"
  - Primary action: "Cerca"
  - Secondary action: "Reimposta filtri"
  - Empty state (pre-search): "Seleziona un indirizzo completo e premi Cerca per visualizzare i profili disponibili."
  - Empty state (post-search, no rows): "Nessuna copertura disponibile per questo civico."
  - Error: "Errore nel caricamento dei dati. Riprova."
  - Results header hint: "<N> profili disponibili" (only when N > 0; plain, no decorative styling).
- **Forbidden copy risks (explicitly avoided):**
  - No implementation mechanics in UI copy (no "datasource", "server-side", "prepared statement", "endpoint", "API", "record", "widget", "id.asc").
  - No launcher-style tagline or marketing sub-title under the page title.
  - No explanatory panel about how coverage is matched.
- **Metrics allowed:** **none.** The spec defines no aggregate with user value at per-address scale.

## Contract Pinning Gate

> Per `docs/APPSMITH-MIGRATION-PLAYBOOK.md`, this migration does not move from plan to handler/UI implementation until the DB-backed source contracts are pinned from real `dbcoperture` artifacts.

- **Source artifacts to inspect and capture:**
  - `coperture.get_states()` definition or representative result row.
  - `coperture.get_coverage_details_types()` definition or representative result row.
  - `coperture.v_get_coverage` definition or representative rows covering `coverage_id`, `operator_id`, `tech`, `profiles[]`, and `details[]`.
- **Facts that must be frozen before coding:**
  - Exact JSON shape of `GET /api/coperture/v1/states`.
  - Final frontend/backend contract for `CoverageResult`, including the real `coverage_id` type.
  - Exact structure of `profiles[]` and `details[]` as returned by the source view.
  - Explicit rule for the trailing-`0000` normalization on detail values.
  - Whether `tech` remains free-text or should be treated as a constrained enum.
- **Required pre-implementation tests:**
  - Query-shape tests pinning the function/view access pattern and sort order.
  - Decoder/normalization tests using captured sample payloads or representative rows.
  - Regression tests for the chosen `coverage_id` type and the final `0000` formatting rule.
- **Execution rule:** if these facts are still implicit, implementation stops at scaffolding only. Handler logic and frontend rendering must wait until the contracts are pinned.

## Repo-Fit

> Verified against `docs/IMPLEMENTATION-PLANNING.md` §Repo-Fit Checklist. No "X or Y" placeholders.

- **Route / base path (frontend):** `/apps/coperture/` (Vite `base` in build; `/` in dev). Matches the existing `/apps/{app-id}/` pattern (budget, compliance, panoramica-cliente, reports, richieste-fattibilita, …).
- **Portal catalog entry:** `applaunch.CopertureAppID = "coperture"`, `applaunch.CopertureAppHref = "/apps/coperture/"`, category `smart-apps` / title `SMART APPS`. Replaces the commented-out legacy entry that uses the older `/apps/smart-apps/coperture` href.
- **API prefix (backend):** `/api/coperture/v1/…`, following the `/{module}/v1/…` convention used by `panoramica`, `rdf`, `reports`, etc. `apps/zammu/coperture-spec.md` still uses `/api/coperture/...` shorthand; implementation and tests must use `/api/coperture/v1/...`, and the spec should be reconciled to that path before coding. Concrete endpoints:
  - `GET /api/coperture/v1/states`
  - `GET /api/coperture/v1/states/{stateId}/cities`
  - `GET /api/coperture/v1/cities/{cityId}/addresses`
  - `GET /api/coperture/v1/addresses/{addressId}/house-numbers`
  - `GET /api/coperture/v1/house-numbers/{houseNumberId}/coverage`
  - (internal/optional) `GET /api/coperture/v1/operators`
- **Access role:** `app_coperture_access`. Enforced on every route via `acl.RequireRole(applaunch.CopertureAccessRoles()...)`. Role is registered in `backend/internal/platform/applaunch/catalog.go` alongside the existing role definitions (compact snake-case style per the rest of the file).
- **Dev port / proxy:** **Vite port 5183** (next free after reports=5180, rdf-backend=5181, richieste-fattibilita=5182). Proxies: `/api` → `VITE_DEV_BACKEND_URL || http://localhost:8080`, `/config` → same target. Mirror the exact config shape from `apps/panoramica-cliente/vite.config.ts`. Also extend the backend default `CORS_ORIGINS` list with `http://localhost:5183`.
- **Dev wiring (per CLAUDE.md new-app checklist):**
  - Root `package.json`: add the `coperture` name + unique color + filter to the `dev` `concurrently` command; add a `dev:coperture` script.
  - `Makefile`: add `dev-coperture` target and include it in `.PHONY`.
  - `docker-compose.dev.yaml`: add a `coperture` frontend service + named volume on port `5183`, with `VITE_DEV_BACKEND_URL=http://backend:8080`, so `make dev-docker` covers the app too.
  - `backend/cmd/server/main.go`: open `dbcoperture` via `database.New(database.Config{Driver: "postgres", DSN: cfg.DBCopertureDSN})` when the DSN is present; register `coperture.RegisterRoutes(api, dbCoperture)` on the existing `api` sub-mux; add the `CopertureAppURL` split-server override (or default `http://localhost:5183` when `StaticDir == ""`); and filter Coperture out of `appCatalog` when `cfg.DBCopertureDSN == ""`, matching the current dependency-backed app behavior.
  - `backend/internal/platform/config/config.go`: add `CopertureAppURL string` and `DBCopertureDSN string` fields; env keys `COPERTURE_APP_URL` and `DBCOPERTURE_DSN`; extend the default `CORS_ORIGINS` string with `http://localhost:5183`.
  - `backend/internal/platform/applaunch/catalog.go`: add the Coperture app ID / href / access-role constants and `CopertureAccessRoles()`.
  - `backend/internal/platform/applaunch/catalog_test.go`: update all-catalog expectations and add Coperture-specific role / href-override coverage so `go test ./...` stays green after the catalog change.
  - `backend/.env.example` and `.env.preprod.example`: document `DBCOPERTURE_DSN`; also add `http://localhost:5183` to the sample `CORS_ORIGINS` in `backend/.env.example`. `docker-compose.preprod.yaml` already consumes `.env.preprod`, so no compose-file change is needed there.
- **Backend module layout:** `backend/internal/coperture/` with `handler.go` (RegisterRoutes + HTTP handlers) and one query file per entity (`handler_states.go`, `handler_cities.go`, etc.), mirroring `backend/internal/panoramica/`. Handler struct holds only the injected `*sql.DB` — dependency-injected per `IMPLEMENTATION-PLANNING.md` guidance.
- **Static hosting / deployment:** `deploy/Dockerfile` must include `COPY --from=frontend /app/apps/coperture/dist /static/apps/coperture` — per `IMPLEMENTATION-KNOWLEDGE.md` "Backend-Served SPAs Must Be Copied Explicitly Into `/static/apps/<slug>`". `staticspa` deep-link test coverage mandatory.
- **Database:** net-new PostgreSQL DSN `DBCOPERTURE_DSN` pointing at the `dbcoperture` PostgreSQL instance (schema `coperture`). Existing tables/view/functions are read-only and consumed as-is. The app is hidden from the launcher when the DSN is absent; if the route is hit in a misconfigured environment, handlers return `503 coperture_database_not_configured`. A manual SQL migration is required for the new operator master table (see Data contract below).
- **Auth/transport (per `IMPLEMENTATION-KNOWLEDGE.md`):**
  - Frontend uses `@mrsmith/auth-client` + `@mrsmith/api-client`. First request must wait until `getAccessToken()` returns a value; optional-auth fallbacks must default to `unauthenticated`.
  - React Query retries must stay enabled for local auth-preflight 401s but disabled for real backend ACL failures.

## Data contract (spec-level)

- **Entities (from `coperture-spec.md` §Entity Catalog):** State, City, Address, HouseNumber, CoverageResult, Operator, CoverageDetailType. All read-only from the user's perspective.
- **Cross-DB concerns:** none. All data lives in `dbcoperture` (+ the new Operator table). No cross-database join required. Rule "Cross-Database Mini-App Summaries Must Merge In Code" from `IMPLEMENTATION-KNOWLEDGE.md` does not apply.
- **Identifier strategy:** State / City / Address / HouseNumber IDs remain native integer PKs from `dbcoperture`. The operator table is net-new with stable seeded integer IDs. `coverage_id` is explicitly **not** assumed yet: its real type must be pinned in the contract-gate step and then frozen in the API types/tests.
- **Active-vs-include-inactive defaults:** not applicable — the spec describes no "active" flag.
- **Nullable-text scan risk:** apply `sql.NullString` scanning to any text column that the schema marks nullable (per the precedent in `IMPLEMENTATION-KNOWLEDGE.md` for `fornitori_preferiti` and Panoramica summary fields). Confirm schema for `network_coverage_*` tables during implementation.
- **Operator master data migration:**
  - Final location: new table `coperture.network_operators` in `dbcoperture`.
  - Migration file: checked-in manual SQL `deploy/migrations/003_coperture_network_operators.sql`.
  - Apply rule: manual execution before deployment and before any live-DB/manual verification. There is no migration runner in this repo for this slice.
  - Dev/preprod contract: `DBCOPERTURE_DSN` must point to an instance where the SQL above has already been applied.
  - Seed rows: TIM (id 1), Fastweb (id 2), OpenFiber (id 3), OpenFiber CD (id 4) with `logo_url` pointing at the existing `static.cdlan.business` CDN assets.
  - Read-path join: `CoverageResult.operator_id -> network_operators.id`; backend returns `{operator_name, logo_url}` denormalized so the frontend has no hardcoded operator map.
- **Detail-type inlining:** default to inlining `type_name` into each `CoverageResult.details[]` item so the frontend has zero lookup step. Expose `GET /api/coperture/v1/detail-types` only if a later need emerges.

## Exceptions

- **No export action.** `report_explorer` default composition allows but does not require export. Coperture v1 has no export in the spec. User benefit: the screen is visually simpler and the user's task completes on-screen.
- **No metrics.** `report_explorer` allows metrics "only if they summarize real report output." At single-address scale there is no aggregate worth surfacing (a lookup returns a handful of operator/tech combinations). User benefit: no visual noise above the results list.
- **Cascading selects are auto-advancing on change** (no explicit "next step" button between levels). This matches the source behaviour and the `AccessiPage` toolbar pattern; stepped interaction would slow the user down. User benefit: fewer clicks, same information flow.
- **Per-result sub-listing rendered as React components, not HTML strings.** The source app embeds `<table>`/`<ul>` via `formatProfili` / `formatDettagli`. This is not an exception to an archetype, but noted here so the UI reviewer does not flag the multi-row composition as an undeclared secondary panel.

## Verification

### UI review checks (pre-gate inputs)
- Populated desktop screenshot/mockup.
- Empty pre-search.
- Empty post-search zero-results.
- Skeleton loading on the results region.
- Error state (generic 5xx).
- 503 service-unavailable (matching Panoramica's `ServiceUnavailable` shape).
- Narrow viewport (~640 px): toolbar wraps, results table horizontally scrollable.

### Runtime / auth checks
- Deep-link refresh at `/apps/coperture/` returns the built index (verifies `staticspa` + Dockerfile `/static/apps/coperture/` copy).
- First page load does not emit any request before the Keycloak bearer is available (verified via network tab + backend access log — no `401 missing_bearer`).
- Accessing the app without `app_coperture_access` returns a 403 from the backend and no app tile in the portal launcher.
- With `DBCOPERTURE_DSN` unset, the Coperture tile is absent from the launcher; if the route is hit directly in that misconfigured environment, the backend returns `503 coperture_database_not_configured`.
- Optional-auth fallback in the app shell defaults to `unauthenticated` and the app gates route rendering on `authenticated` rather than `loading` alone.

### Tests
- **Pre-implementation contract tests (must land before feature logic):**
  - Query-shape tests pinning the `get_states()`, `get_coverage_details_types()`, and `v_get_coverage` access pattern plus `ORDER BY operator, tech`.
  - Decoder tests using captured sample rows / payloads for `states`, `profiles[]`, `details[]`, and `coverage_id`.
  - Regression tests for the final `0000` normalization rule and any pinned `tech` enum/domain decision.
- **Backend (Go):**
  - Handler tests via `httptest` for `503 coperture_database_not_configured` when the DB handle is nil and the route is hit directly.
  - Per-endpoint success / empty / internal-error coverage once the pinned fixtures exist.
  - Explicit regression test asserting that the coverage response includes `operator_name` + `logo_url` (denormalized), preventing drift back to a frontend-side operator map.
  - Auth/ACL coverage for unauthenticated and missing-role requests.
- **Platform wiring tests:**
  - `backend/internal/platform/applaunch/catalog_test.go` updated for Coperture role visibility and href override.
  - `staticspa` deep-link regression test for `/apps/coperture/...`.
- **Frontend (React):**
  - Component test: selects disable downstream levels until the upstream value is chosen.
  - Component test: "Cerca" button stays disabled until `houseNumberId` is set.
  - Component test: empty pre-search and empty post-search states render distinct copy.
  - Component test: error toast/state renders without crashing on 5xx.
- **Integration / manual:**
  - End-to-end run of the 4-step cascade in both `make dev-coperture` and `make dev-docker` against a `DBCOPERTURE_DSN` where `deploy/migrations/003_coperture_network_operators.sql` has already been applied.
  - Verify that the operator logo renders from the backend-supplied URL (not a frontend constant).
  - Confirm no CSP / mixed-content warning when loading logos from `static.cdlan.business`.

### Handoff
- This document is the pre-gate artifact for `portal-miniapp-ui-review`. Post-gate review is required again after the screen is implemented.
