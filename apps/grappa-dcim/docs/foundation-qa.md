# Grappa DCIM Foundation QA

Status: PASS

## Source Docs Checked

- `apps/grappa-dcim/docs/foundation-run.md`
- `apps/grappa-dcim/docs/foundation-implementation-report.md`
- `apps/grappa-dcim/docs/foundation-impl.md`
- `apps/grappa-dcim/docs/grappa-dcim-spec.md`
- `apps/grappa-dcim/docs/planning-ui-review.md`
- `docs/UI-UX.md`
- `docs/IMPLEMENTATION-PLANNING.md`
- `docs/IMPLEMENTATION-KNOWLEDGE.md`
- `docs/grappa/GRAPPA.md`
- `.agents/skills/portal-miniapp-ui-review/SKILL.md`
- `.agents/skills/portal-miniapp-ui-review/references/blocking-gates.md`
- `.agents/skills/portal-miniapp-ui-review/references/evidence-checklist.md`

## Changed Files Inspected

- Frontend foundation:
  - `apps/grappa-dcim/package.json`
  - `apps/grappa-dcim/index.html`
  - `apps/grappa-dcim/tsconfig.json`
  - `apps/grappa-dcim/vite.config.ts`
  - `apps/grappa-dcim/src/App.tsx`
  - `apps/grappa-dcim/src/App.module.css`
  - `apps/grappa-dcim/src/main.tsx`
  - `apps/grappa-dcim/src/routes.tsx`
  - `apps/grappa-dcim/src/vite-env.d.ts`
  - `apps/grappa-dcim/src/api/client.ts`
  - `apps/grappa-dcim/src/api/queries.ts`
  - `apps/grappa-dcim/src/api/types.ts`
  - `apps/grappa-dcim/src/components/ServiceUnavailable.tsx`
  - `apps/grappa-dcim/src/components/ViewState.tsx`
  - `apps/grappa-dcim/src/components/WorkspaceStub.tsx`
  - `apps/grappa-dcim/src/hooks/useOptionalAuth.ts`
  - `apps/grappa-dcim/src/lib/roles.ts`
  - `apps/grappa-dcim/src/styles/global.css`
  - `apps/grappa-dcim/src/styles/shared.module.css`
- Backend foundation:
  - `backend/internal/grappadcim/handler.go`
  - `backend/internal/grappadcim/helpers.go`
  - `backend/internal/grappadcim/types.go`
- Repo/runtime wiring:
  - `backend/cmd/server/main.go`
  - `backend/internal/platform/applaunch/catalog.go`
  - `backend/internal/platform/config/config.go`
  - `backend/.env.example`
  - `deploy/Dockerfile`
  - `Makefile`
  - `package.json`
  - `pnpm-lock.yaml`
  - `pnpm-workspace.yaml`
- Orchestration docs present in the diff/worktree:
  - `apps/grappa-dcim/docs/foundation-run.md`
  - `apps/grappa-dcim/docs/foundation-implementation-report.md`
  - `apps/grappa-dcim/docs/planning-ui-review.md`
  - `apps/grappa-dcim/docs/orchestration-state.md`

## Product Behavior Findings

- PASS. The implementation report exists and records files changed, implemented behavior, contracts preserved, commands run, manual checks, unresolved questions, and deviations from plan.
- PASS. The foundation is shell/stub only. No domain feature modules, CRUD behavior, lifecycle flows, polling, alerting, Hive sync, TIM GEA, CWDM, or first-class `cassetti_ottici` UI were implemented.
- PASS. The app exposes the approved navigation groups for `Infrastruttura`, `Asset`, `Connettivita`, and `Topologia`, with plain Italian stub workspaces for the planned V1 areas.
- PASS. Default, route-not-found, loading, forbidden/unauthenticated, generic error, and Grappa DB-not-configured states are represented in code. The route-not-found path redirects to the foundation default route.

## Repo/Runtime Findings

- PASS. Frontend route/base wiring is consistent: Vite build base is `/apps/grappa-dcim/`, `BrowserRouter` derives its basename from `import.meta.env.BASE_URL`, and the launcher href is `/apps/grappa-dcim/` in production.
- PASS. API wiring is consistent: browser calls use `/api/grappa-dcim/v1/...`, while the backend registers `/grappa-dcim/v1/...` behind the existing `/api` strip-prefix mux.
- PASS. Dev/runtime wiring is present: Vite port `5191`, `/api` and `/config` proxies, root `dev:grappa-dcim` script, Makefile `dev-grappa-dcim`, backend CORS default for `http://localhost:5191`, and split-server launcher override through `GRAPPA_DCIM_APP_URL`.
- PASS. Static deployment wiring is present: Docker copies `apps/grappa-dcim/dist` to `/static/apps/grappa-dcim`, which matches `staticspa` app deep-link resolution.
- PASS. `pnpm-workspace.yaml` already includes `apps/*`, and `pnpm-lock.yaml` includes the new workspace importer.
- PASS. No automated tests were added without approval.

## Data/Auth Findings

- PASS. The backend reuses the existing `GRAPPA_DSN`/`grappaDB`; no second Grappa DSN was introduced.
- PASS. The launcher tile is filtered out when `GRAPPA_DSN` is empty.
- PASS. `GET /grappa-dcim/v1/meta` and `GET /grappa-dcim/v1/lookups` are protected by Viewer-or-Operativo access. The shared backend authz helper preserves the `app_devadmin` override.
- PASS. Viewer/Operativo separation is preserved in the foundation contract: `meta.canOperate` and `meta.canViewCredentials` are true only for Operativo or `app_devadmin`; there are no mutation endpoints in this slice.
- PASS. The frontend gates route rendering before rendering navigation/data states by using `getAppAccessState` with local Grappa DCIM role constants. The shared frontend helper preserves the `app_devadmin` override.
- PASS. Backend DB absence returns `503 grappa_dcim_database_not_configured`; the frontend converts that into a business-facing unavailable state instead of showing the raw backend code.

## UI Findings

- PASS. Review phase: post-implementation code-first UI gate for the foundation shell.
- PASS. Evidence is sufficient for code-first approval: the approved plan, selected `data_workspace` archetype, explicit foundation exception, comparable screens, implementation files, routes, and state components are present. Screenshots were not available because no suitable dev server was running.
- PASS. The shell follows the mini-app family through `AppShell`, grouped tab navigation, compact workspace headers, and clean-theme styling. It does not use Matrix launcher visuals, a landing/hero shell, dashboard cards, ornamental metrics, or marketing copy.
- PASS. The app `global.css` uses the approved clean mini-app background from `docs/UI-UX.md` and sets `document.documentElement.dataset.theme = 'clean'`.
- PASS. Stub copy is concise, Italian, and domain-facing. No raw role names, SQL/table names, HTTP status text, framework names, credential details, or source-route mechanics are shown in the implemented UI.
- PASS. No KPI/stat cards were introduced. The only framed surfaces are empty/state panels appropriate for the foundation stub.

## Verification Commands Run

- `pnpm --filter mrsmith-grappa-dcim build`
  - PASS. TypeScript and Vite production build completed successfully.
- `go build ./cmd/server` from `backend`
  - PASS. Backend build completed successfully.
- `lsof -nP -iTCP:5191 -sTCP:LISTEN`
  - No listener found.
- `lsof -nP -iTCP:8080 -sTCP:LISTEN`
  - No listener found.
- `lsof -nP -iTCP:5173 -sTCP:LISTEN`
  - No listener found.
- `rg` checks for V2 leakage, forbidden user-facing/raw copy terms, credentials/password terms, and test/spec additions across the foundation implementation.
  - PASS. Matches were limited to backend-internal helper names/API serialization or existing source docs, not user-facing implemented UI or unauthorized test additions.

## Manual/Browser Checks

- `/apps/grappa-dcim/` shell: not run in browser. No suitable Vite or backend dev server was already listening, and the run contract did not require starting a new server.
- `/config` proxy assumptions: not run in browser for the same reason. Verified from `vite.config.ts` proxy configuration.
- `/api/grappa-dcim/v1/meta` over HTTP: not run manually for the same reason. Verified from backend route registration and successful backend build.
- UI states: reviewed from implementation code only:
  - allowed/role-ready shell
  - forbidden/unauthenticated state through `AccessNotice`
  - default route and plain stub routes
  - backend-not-configured state through `ServiceUnavailable`

## Residual Risks

- Browser rendering, responsive behavior, and live auth/API smoke were not manually exercised because no reusable dev server was active. This is a verification gap, not a known defect.
- `backend/.env.preprod.example` was requested by the plan but does not exist in this checkout; `backend/.env.example` was updated and the implementation report records the absence.
- No backend ACL/503/deep-link tests were added, consistent with the no-tests-without-approval rule. The implementation report records recommended future tests for those boundaries.
