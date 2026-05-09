# Foundation Run Contract

## Status

- Iteration: 1
- Dependency status: `apps/grappa-dcim/docs/planning-ui-review.md` reports `Status: PASS`; no prior slice dependency.
- Allowed write scope:
  - `apps/grappa-dcim/package.json`
  - `apps/grappa-dcim/index.html`
  - `apps/grappa-dcim/tsconfig*.json`
  - `apps/grappa-dcim/vite.config.ts`
  - `apps/grappa-dcim/src/App.tsx`
  - `apps/grappa-dcim/src/main.tsx`
  - `apps/grappa-dcim/src/routes.tsx`
  - `apps/grappa-dcim/src/api/*`
  - `apps/grappa-dcim/src/hooks/*`
  - `apps/grappa-dcim/src/styles/*`
  - `apps/grappa-dcim/src/components/*` only for foundation shell/shared empty-state components
  - `apps/grappa-dcim/src/lib/*` only for foundation shell/shared helpers
  - `backend/internal/grappadcim/*` foundation files only
  - repo wiring files needed by `apps/grappa-dcim/docs/foundation-impl.md`: root `package.json`, `Makefile`, `pnpm-workspace.yaml`, `backend/cmd/server/main.go`, backend platform config/launcher/CORS/static wiring, `deploy/Dockerfile`, and backend env examples.
- Disallowed write scope:
  - Domain feature modules for facilities, equipment, cabling, xcon, rings, artifacts, except plain stub route placeholders needed by the shell.
  - Schema semantics, migration files, or source docs outside the orchestration artifacts.
  - Automated tests unless the human explicitly approves them.

## Required Reading

- `apps/grappa-dcim/docs/grappa-dcim-spec.md`
- `apps/grappa-dcim/docs/foundation-impl.md`
- `apps/grappa-dcim/docs/planning-ui-review.md`
- `docs/UI-UX.md`
- `docs/IMPLEMENTATION-PLANNING.md`
- `docs/IMPLEMENTATION-KNOWLEDGE.md`
- `docs/grappa/GRAPPA.md`
- Comparable files cited by `foundation-impl.md`, especially:
  - `apps/energia-dc/src/App.tsx`
  - `apps/energia-dc/src/routes.tsx`
  - `apps/energia-dc/src/pages/SituazioneRackPage.tsx`
  - `apps/energia-dc/src/pages/shared.module.css`
  - `apps/manutenzioni/src/App.tsx`
  - `apps/manutenzioni/src/pages/MaintenanceListPage.tsx`
  - `apps/manutenzioni/src/pages/MaintenanceDetailPage.tsx`
  - `apps/rda/src/App.tsx`
  - `apps/rda/src/pages/PoDetailPage.tsx`

## Implementation Target

- Implement the foundation slice from `apps/grappa-dcim/docs/foundation-impl.md`.
- Create the Vite React mini-app shell at `/apps/grappa-dcim/` using the clean mini-app family, role-gated route rendering, grouped navigation, and plain business-facing stub workspaces.
- Add a foundation API client, app access helpers, and route constants for later slices.
- Add `backend/internal/grappadcim` with route registration, DB requirement handling, Viewer/Operativo authorization helpers, safe JSON/error helpers, `GET /grappa-dcim/v1/meta`, and `GET /grappa-dcim/v1/lookups`.
- Wire backend registration, config/launcher/CORS/static hosting, Docker static copy, root scripts, and package workspace entries as specified in the foundation plan.
- Preserve V1 boundaries: no CWDM, TIM GEA, Hive sync, polling, alerting, or first-class `cassetti_ottici` UI.
- Do not add tests in this run; record recommended tests in the implementation report instead.

## Verification Required

- Commands:
  - `pnpm --filter mrsmith-grappa-dcim build`
  - `go build ./cmd/server` from `backend`
- Manual/browser checks:
  - If a suitable dev server is already running, reuse it; otherwise do not start a server unless verification requires it.
  - Record whether `/apps/grappa-dcim/` shell, `/config` proxy assumptions, and `/api/grappa-dcim/v1/meta` wiring were manually checked or not checked.
- UI review states:
  - Allowed/role-ready shell state.
  - Forbidden/unauthenticated state via shared access handling.
  - Empty default route and plain stub routes.
  - Backend-not-configured state when Grappa DB is absent.

## Reporting Required

Write `apps/grappa-dcim/docs/foundation-implementation-report.md` with:
- files changed
- behavior implemented
- contracts preserved
- commands run and outputs summarized
- unresolved questions
- deviations from plan
