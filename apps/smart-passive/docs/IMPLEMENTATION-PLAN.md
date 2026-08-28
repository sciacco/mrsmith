# Smart Passive Implementation Plan

## Scope

Shell scaffold for a new mini-app under `apps/smart-passive`. The app is the future control layer of the passive cycle (invoice-PO matching, anomaly resolution, budget consumption), but this plan covers only the empty shell: app registration, routing, auth, a single Dashboard placeholder page, and all repo-fit wiring. Business pages arrive in later iterations.

Mission context (not implemented in this scaffold):
- Mr Smith becomes the governance and matching system for the passive cycle; Alyante remains the official accounting and treasury system.
- Phases owned by this app in future iterations: Fattura (pulled from Alyante) -> matching vs PO (from `ordini`) -> anomalies -> budget consumption.
- Phases that stay in existing apps: richieste acquisto (`rda`), PO (`ordini`), budget definitions (`budget`), fornitori (`fornitori`).

## Comparable Apps Audit

- Reference 1: `apps/kit-products/src/App.tsx` + `apps/kit-products/src/views/WorkspacePlaceholder.tsx`
- Reference 2: `apps/training/src/App.tsx` + `apps/training/src/routes.tsx`
- Reused patterns:
  - `AppShell` + `TabNav` shell with auth gate via `getAppAccessState` + `AccessNotice` (kit-products, training)
  - `main.tsx` bootstrap: `/config` fetch -> `setRuntimeConfig` -> `AuthProvider` -> `QueryClientProvider` -> `BrowserRouter` -> `ToastProvider` (kit-products)
  - `WorkspacePlaceholder` component for the placeholder page (kit-products pattern, app-local copy)
  - `runtimeConfig.ts` with `RuntimeConfig` + `setRuntimeConfig`/`getRuntimeConfig` (kit-products)
  - `vite.config.ts` via `defineMrSmithAppConfig({ appSlug, port })` (kit-products)
  - Catalog `Definition` with `Status: "dev"` and role-gated `AccessRoles` (training, aenad)
- Rejected patterns:
  - `TabNavGroup` grouped nav (training) — premature for a single-page shell; use flat `TabNav` with one tab
  - Feature-gated route wrappers like `ArakFeature` (kit-products) — no feature flags in the scaffold
  - Multi-route worklist/detail layouts — no business pages yet

## Archetype Choice

- Selected archetype: `data_workspace`
- Why it fits: the app is a workspace that will coordinate multiple related surfaces (matching, anomalies, budget consumption) as sub-routes under a unified shell. The scaffold starts with one placeholder route (`/dashboard`) and the shell is ready to receive additional sub-routes in later iterations without re-wiring.
- Required states:
  - Loading: `AccessNotice` bootstrap state during auth init
  - Access denied: `AccessNotice` with `state` from `getAppAccessState`
  - Empty/placeholder: `WorkspacePlaceholder` on `/dashboard`
  - Fatal bootstrap error: inline error render in `main.tsx` (kit-products pattern)

## User Copy Rules

- Allowed copy style: business-user-facing Italian. App name "Smart Passive". Dashboard placeholder copy describes the app mission in business terms (controllo del ciclo passivo, matching fatture-PO, anomalie, consumi budget).
- Forbidden copy risks: technical jargon ("DSN", "BFF", "endpoint"), implementation mechanics, placeholder lorem ipsum.
- Metrics allowed: none in the scaffold. No KPI cards, no summary numbers. Future iterations must justify any metric against real feature data.

## Repo-Fit

- Route/base path: `/apps/smart-passive/` (Vite base in build mode via `defineMrSmithAppConfig`); client routes under `BrowserRouter` basename.
- API prefix: `/api/smart-passive/v1/...` (no backend module registered in this scaffold — no API routes yet. When the first business page lands, create `backend/internal/smartpassive/handler.go` with `RegisterRoutes` on the `api` sub-mux, pattern `GET /smart-passive/v1/...`).
- Access role: `app_smartpassive_access` (Keycloak role, `app_{appname}_access` convention).
- Dev port: 5197 (next free after 5196 stats-rda).
- Proxy notes: shared `defineMrSmithAppConfig` proxies `/api` + `/config` to backend. No app-specific proxy needed.
- Static hosting: `deploy/Dockerfile` copies `apps/smart-passive/dist` to `/static/apps/smart-passive`. `staticspa` handler auto-resolves `/apps/smart-passive/` — no per-app registration.
- Catalog entry: `SmartPassiveAppID = "smart-passive"`, `SmartPassiveAppHref = "/apps/smart-passive/"`, `Icon: "funnel"`, `Status: "dev"`, `CategoryID: "acquisti"`, `CategoryTitle: "Acquisti"`.
- Env contract: `SMART_PASSIVE_APP_URL` (optional split-server override) in `config.go`. No DSN in the scaffold — when the first data-backed page lands, decide explicitly between a new `SMART_PASSIVE_DSN` or reusing an existing DB handle, and update `.env.example` + `.env.preprod.example`.

### Touchpoint checklist (new app)

- [ ] `apps/smart-passive/package.json` — name `mrsmith-smart-passive`, scripts (dev/build/lint/preview), deps matching kit-products minus unneeded packages
- [ ] `apps/smart-passive/vite.config.ts` — `defineMrSmithAppConfig({ appSlug: 'smart-passive', port: 5197 })`
- [ ] `apps/smart-passive/tsconfig.json` — copy kit-products verbatim
- [ ] `apps/smart-passive/index.html` — title "Smart Passive", same font links
- [ ] `apps/smart-passive/src/main.tsx` — bootstrap pattern from kit-products
- [ ] `apps/smart-passive/src/App.tsx` — `AppShell` + `TabNav` with one "Dashboard" tab, auth gate
- [ ] `apps/smart-passive/src/App.module.css` — nav row layout (kit-products pattern)
- [ ] `apps/smart-passive/src/routes.tsx` — `/dashboard` placeholder route, redirect index and catch-all to `/dashboard`
- [ ] `apps/smart-passive/src/runtimeConfig.ts` — `RuntimeConfig` (keycloakUrl, realm, clientId) + setters
- [ ] `apps/smart-passive/src/hooks/useOptionalAuth.ts` — copy kit-products pattern
- [ ] `apps/smart-passive/src/views/DashboardPage.tsx` — placeholder page (mission copy, no KPIs)
- [ ] `apps/smart-passive/src/styles/global.css` — page background recipe (§4.3), entrance keyframes (§8.2), reduced-motion block (§8.4)
- [ ] Root `package.json` — add `dev:smart-passive` script
- [ ] `Makefile` — add `dev-smart-passive` target + `.PHONY`
- [ ] `docker-compose.dev.yaml` — add `smart-passive` service + volume
- [ ] `deploy/Dockerfile` — add `COPY --from=frontend .../smart-passive/dist /static/apps/smart-passive`
- [ ] `backend/internal/platform/config/config.go` — add `SmartPassiveAppURL` field + `envOr("SMART_PASSIVE_APP_URL", "")`
- [ ] `backend/internal/platform/applaunch/catalog.go` — `SmartPassiveAppID`/`SmartPassiveAppHref` const, `smartPassiveAccessRoles` var, `Definition` entry, `SmartPassiveAccessRoles()` func, add to `AllRoles`
- [ ] `backend/cmd/server/main.go` — `hrefOverrides` for `SmartPassiveAppID` (default `http://localhost:5197`), catalog visibility filter (hide when not needed — no DSN gate in scaffold, so always visible to role holders)
- [ ] No `backend/internal/smartpassive/` package in the scaffold (no API routes yet)
- [ ] No `.env.example` / `.env.preprod.example` DSN additions in the scaffold
- [ ] `catalog_test.go` — no manual test needed; `allCatalogRoles` + `TestVisibleCategoriesDevAdminSeesEverything` auto-cover new entries
- [ ] `staticspa/handler_test.go` — no per-app test needed; auto-resolves by slug

## Exceptions

- No backend module in the scaffold: the app registers in the catalog and config but has no API routes. This is an exception to the usual DSN-backed app pattern; it is justified because the shell is a placeholder and business endpoints arrive with the first real page. Documented here so the fixer/reviewer does not treat the missing `backend/internal/smartpassive/` package as an oversight.
- No DSN env var: the catalog visibility filter does not hide the app based on a missing DSN (unlike kit-products which hides when `MistraDSN == ""`). The app is always visible to role holders in dev. When a DSN-backed page lands, revisit the filter.
- `WorkspacePlaceholder` is app-local, not shared: kit-products has its own app-local copy in `apps/kit-products/src/views/`. Smart-passive copies the same pattern rather than importing from kit-products (cross-app import is not repo-fit). If a future app needs the same placeholder, consider promoting to `@mrsmith/ui`.

## Verification

- UI review checks (pre-gate):
  - Shell renders with `clean` theme, approved page background (§4.3), content max-width 1400px inside `AppShell`
  - Single "Dashboard" tab in `TabNav`, active state on `/dashboard`
  - Placeholder page has business-user copy, no KPI cards, no hero banner, no decorative metrics
  - `AccessNotice` renders for denied/loading states
  - Entrance animations on navigation only, reduced-motion block present
- Runtime / auth checks:
  - `/config` bootstrap returns Keycloak config; fatal error renders inline if bootstrap fails
  - Role `app_smartpassive_access` grants access; absence shows `AccessNotice` denied state
  - Deep link to `/apps/smart-passive/dashboard` resolves; refresh works
  - Catalog entry visible to role holders in the portal launcher
- Tests:
  - `pnpm --filter mrsmith-smart-passive exec tsc --noEmit` passes
  - `pnpm --filter mrsmith-smart-passive build` succeeds
  - `go test ./internal/platform/applaunch/...` passes (catalog auto-coverage)
