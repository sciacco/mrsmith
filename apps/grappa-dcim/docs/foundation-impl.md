# Grappa DCIM Foundation Implementation Plan

## Slice Contract

- Slice: `foundation`
- Purpose: create the repo/runtime foundation that every Grappa DCIM feature slice builds on.
- Approved source: `apps/grappa-dcim/docs/grappa-dcim-spec.md`.
- Required references read before execution: `docs/UI-UX.md`, `docs/IMPLEMENTATION-PLANNING.md`, `docs/IMPLEMENTATION-KNOWLEDGE.md`, `docs/grappa/GRAPPA.md`.
- Primary write ownership:
  - `apps/grappa-dcim/package.json`
  - `apps/grappa-dcim/vite.config.ts`
  - `apps/grappa-dcim/src/App.tsx`
  - `apps/grappa-dcim/src/main.tsx`
  - `apps/grappa-dcim/src/routes.tsx`
  - `apps/grappa-dcim/src/api/*`
  - `apps/grappa-dcim/src/hooks/*`
  - `apps/grappa-dcim/src/styles/*`
  - `backend/internal/grappadcim/*` foundation files only
  - repo wiring files listed under Repo-Fit
- Non-owners: domain slices own their feature modules and route additions after this slice is accepted.

## Comparable Apps Audit

- Reference 1: `apps/energia-dc/src/App.tsx`, `apps/energia-dc/src/routes.tsx`, `apps/energia-dc/src/pages/SituazioneRackPage.tsx`, `apps/energia-dc/src/pages/shared.module.css`.
- Reference 2: `apps/manutenzioni/src/App.tsx`, `apps/manutenzioni/src/pages/MaintenanceListPage.tsx`, `apps/manutenzioni/src/pages/MaintenanceDetailPage.tsx`.
- Additional reference: `apps/rda/src/App.tsx`, `apps/rda/src/pages/PoDetailPage.tsx`.
- Reused patterns:
  - `AppShell` with role-gated route rendering before data queries.
  - `TabNav` or grouped navigation under the shell header.
  - compact page headers, filter bars, skeleton/empty/error states, and business-facing Italian copy.
  - React Query plus `@mrsmith/api-client` with bearer-token preflight.
  - backend `RegisterRoutes` with `acl.RequireRole(...)`, `requireDB`, structured internal errors, and sanitized client errors.
- Rejected patterns:
  - launcher visual language, Matrix styling, hero banners, marketing sections, decorative summary cards.
  - fake KPI rows in CRUD/workspace screens.
  - UI copy that exposes implementation words such as `server-side`, `record`, `datasource`, or `inline update`.

## Archetype Choice

- Selected archetype: `data_workspace`.
- Why it fits: Grappa DCIM is a multi-surface operational workspace with registries, physical maps, tabbed details, lifecycle actions, and artifact/history panels. The app shell must coordinate those areas without becoming a dashboard.
- Required states:
  - authenticated allowed state
  - loading and reauthenticating state
  - unauthenticated and forbidden state through shared `AccessNotice`
  - empty route/default redirect
  - backend not configured state when `GRAPPA_DSN` is missing
  - global API error state with sanitized message

## User Copy Rules

- Allowed copy style: Italian, operational, concise, and domain-facing. Examples: `Rack`, `Sale e MMR`, `Cavi e fibre`, `Anelli fibra`, `Telecamere`, `Archivio storage`.
- Forbidden copy risks:
  - do not mention table names, SQL, handlers, generated child rows, source routes, or "replica legacy" in the UI.
  - do not explain that the app is using MySQL, Vite, Go, or a BFF.
  - do not show raw role names to users.
- Metrics allowed: none by default. Counts may appear only as table totals, pagination totals, or real workspace context such as number of sockets/fibers/racks visible in the current selection.

## Repo-Fit

- Route/base path: `/apps/grappa-dcim/`.
- API prefix: browser calls `/api/grappa-dcim/v1/...`; backend mux registers `/grappa-dcim/v1/...` because `/api` is stripped in `backend/cmd/server/main.go`.
- Access role:
  - Viewer: `app_grappadcim_viewer`, read-only and no sensitive credentials.
  - Operativo: `app_grappadcim_operativo`, includes read/write, allowed lifecycle actions, allowed hard deletes, and server credential access.
  - `app_devadmin` remains the centralized superuser override through shared auth helpers.
- Dev port / proxy notes:
  - Vite port: `5191`.
  - Proxy `/api` and `/config` to `VITE_DEV_BACKEND_URL || http://localhost:8080`.
  - Add `http://localhost:5191` to backend CORS defaults.
- Static hosting / deployment notes:
  - Vite build base is `/apps/grappa-dcim/`.
  - Docker final image must copy `apps/grappa-dcim/dist` to `/static/apps/grappa-dcim`.
  - Static deep links must resolve through `staticspa`.
- Root workspace wiring:
  - Add package filter `mrsmith-grappa-dcim`.
  - Add `dev:grappa-dcim` script in root `package.json` and Makefile.
  - Add app to root `pnpm dev` concurrently command after existing app ports.
- Backend config and launcher wiring:
  - Add `GrappaDCIMAppURL` loaded from `GRAPPA_DCIM_APP_URL`.
  - Add launcher definition under TECH with supported portal icon key. Use an existing supported key such as `database` or `wrench`; verify against `apps/portal/src/components/Icon/icons.tsx`.
  - Hide launcher tile when `GRAPPA_DSN == ""`.
  - During split-server dev, default launcher href to `http://localhost:5191` when `STATIC_DIR == ""`.
  - Update `backend/.env.example` and `.env.preprod.example` for `GRAPPA_DCIM_APP_URL`. Reuse existing `GRAPPA_DSN`; do not create a second Grappa DSN.
- Repo-fit checklist status:
  - Runtime, dev, auth, data contract, deployment, and verification are explicit here.
  - No schema migration is planned. V1 writes existing Grappa MySQL tables and must preserve legacy values.

## Implementation Work Packages

1. Frontend shell
   - Create the Vite React app matching the mini-app family.
   - Set `document.documentElement.dataset.theme = 'clean'` at startup as existing mini-apps do.
   - Use app-local `global.css` with the approved mini-app background from `docs/UI-UX.md`.
   - Gate all routes with `getAppAccessState(auth, APP_ACCESS_ROLES['grappa-dcim'])` before rendering nav or data hooks.
   - Define initial nav groups:
     - `Infrastruttura`: `Edifici`, `Sale e MMR`, `Rack`
     - `Asset`: `Apparati`, `Server`, `Storage`, `Telecamere`
     - `Connettivita`: `Plenum`, `Cavi e fibre`, `Cross connect`
     - `Topologia`: `Anelli fibra`
   - Stub routes may render business-facing empty states until owned feature slices fill them.

2. Frontend API layer
   - Add `useApiClient()` equivalent to `energia-dc`, base URL `/api`.
   - Add typed error helpers and lightweight query key conventions.
   - Keep API paths centralized by slice namespace.

3. Backend foundation
   - Create `backend/internal/grappadcim` package with `Deps`, `Handler`, `RegisterRoutes`, `requireDB`, parse helpers, nullable scanners, JSON helpers, transaction helper, and role helpers.
   - Implement read protection for Viewer or Operativo.
   - Implement mutation protection for Operativo only.
   - Add one health/meta endpoint: `GET /grappa-dcim/v1/meta` returning app capability flags and role booleans, without leaking raw env config.
   - Add lookup endpoint scaffolding for shared selectors that later slices can extend.

4. Shared domain safety contracts
   - Define a destructive action request shape used by all slices:
     - confirmation phrase or boolean pair required by UI and backend
     - reason optional unless a specific lifecycle action requires it
     - backend still checks dependencies and never trusts UI confirmation alone
   - Define update semantics:
     - omitted nullable field means unchanged on PATCH
     - explicit empty string/null clearing is allowed only where source contract permits it
   - Define unknown free-text policy:
     - UI picklists include known values from DB and preserve unknown stored values.

5. Written contracts
   - After implementation, the implementation agent must write `apps/grappa-dcim/docs/foundation-implementation-report.md`.
   - The report must list files changed, app URLs, endpoint prefixes, role names, and any deviations from this plan.

## API Contract

- `GET /grappa-dcim/v1/meta`
  - Auth: Viewer or Operativo.
  - Response: `{ canRead: true, canOperate: boolean, canViewCredentials: boolean, appVersion?: string }`.
- `GET /grappa-dcim/v1/lookups`
  - Auth: Viewer or Operativo.
  - Initial response may be empty sections. Later slices can extend through typed structs.
- Error behavior:
  - `503 grappa_dcim_database_not_configured` when `GRAPPA_DSN` is unavailable.
  - `400` for invalid path/query/payload values.
  - `403` from ACL for missing roles.
  - `500` responses are sanitized through `httputil.InternalError` with server-side operation labels.

## Verification

- UI review checks:
  - plan pre-gate must verify the app shell uses the mini-app family and not launcher visuals.
  - implemented post-gate must cover allowed, forbidden, empty route, and backend-not-configured states.
- Runtime / auth checks:
  - direct browser entry to `/apps/grappa-dcim/` renders only after app-role access is allowed.
  - `/config` proxy works in dev.
  - `/api/grappa-dcim/v1/meta` returns 403 without role and 200 with Viewer/Operativo.
  - launcher tile appears only when Grappa DB is configured and role is present.
- Tests:
  - No tests are authorized by this planning artifact.
  - Before adding tests, ask the human expert. Good candidates are ACL role boundaries, static deep-link routing, and destructive confirmation helpers because they protect business-critical safety rules.
- Build/manual commands for QA:
  - `pnpm --filter mrsmith-grappa-dcim build`
  - `go build ./cmd/server` from `backend`
  - browser smoke at Vite `http://localhost:5191` or production path `/apps/grappa-dcim/`

## Exceptions

- The foundation slice has no standalone business screen. The selected `data_workspace` archetype applies to the app shell and first-route workspace composition.
- The nav groups are allowed even before all feature slices are implemented because they define the written route contract for later agents. Stub pages must be plain empty states, not promotional placeholders.
