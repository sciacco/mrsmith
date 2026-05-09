# Grappa DCIM Foundation Implementation Report

## Files Changed

- Frontend app foundation:
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

## Behavior Implemented

- Created the Grappa DCIM Vite React mini-app at `/apps/grappa-dcim/` with clean mini-app styling, grouped navigation, auth-gated rendering, and plain stub workspaces.
- Added nav groups for `Infrastruttura`, `Asset`, `Connettivita`, and `Topologia`.
- Added frontend API client and React Query hooks for `/api/grappa-dcim/v1/meta` and `/api/grappa-dcim/v1/lookups`.
- Added role constants for Viewer and Operativo:
  - `app_grappadcim_viewer`
  - `app_grappadcim_operativo`
- Added backend route registration under `/grappa-dcim/v1`.
- Implemented:
  - `GET /grappa-dcim/v1/meta`
  - `GET /grappa-dcim/v1/lookups`
- Added backend foundation helpers for DB requirement checks, sanitized internal errors, JSON decoding, path/query parsing, nullable scanners, placeholder generation, transactions, destructive-action request shape, and Operativo-only middleware helper.
- Wired the backend package into `cmd/server`.
- Added launcher catalog entry under `TECH`, with `database` icon and split-server default URL `http://localhost:5191`.
- Added `GRAPPA_DCIM_APP_URL`, CORS origin `http://localhost:5191`, Docker static copy to `/static/apps/grappa-dcim`, root dev script, and Makefile target.

## Contracts Preserved

- V1 scope remains shell/stub only. No domain feature modules were implemented for facilities, equipment, cabling, xcon, rings, or artifacts.
- No V2 features were added: CWDM, TIM GEA, Hive sync, polling, alerting, and first-class `cassetti_ottici` UI remain out of scope.
- Grappa DCIM reuses the existing `GRAPPA_DSN`; no second Grappa DSN was introduced.
- Backend browser prefix remains `/api/grappa-dcim/v1/...`; backend mux prefix remains `/grappa-dcim/v1/...`.
- Frontend build base is `/apps/grappa-dcim/`.
- Backend returns `503 grappa_dcim_database_not_configured` when Grappa DB is not configured.
- The launcher tile is filtered out when `GRAPPA_DSN` is empty.
- No automated tests were added.

## Commands Run

- `pnpm --filter mrsmith-grappa-dcim build`
  - First run failed before TypeScript because the new workspace importer had not been installed: `tsc: command not found`.
- `pnpm install`
  - Completed successfully. Registered the new `apps/grappa-dcim` importer in `pnpm-lock.yaml`; no packages were downloaded.
- `pnpm --filter mrsmith-grappa-dcim build`
  - Passed. TypeScript compiled and Vite built `dist/` successfully.
- `go build ./cmd/server` from `backend`
  - Passed.
- `gofmt -w backend/internal/grappadcim backend/internal/platform/applaunch/catalog.go backend/internal/platform/config/config.go backend/cmd/server/main.go`
  - Completed with no output.
- `lsof -nP -iTCP:5191 -sTCP:LISTEN`, `lsof -nP -iTCP:8080 -sTCP:LISTEN`, and `lsof -nP -iTCP:5173 -sTCP:LISTEN`
  - No listeners found.

## Manual Checks

- `/apps/grappa-dcim/` shell: not manually checked in browser because no suitable dev server was already running.
- `/config` proxy assumptions: not manually checked in browser because no suitable dev server was already running.
- `/api/grappa-dcim/v1/meta` wiring: not manually checked over HTTP because no backend server was already running.
- UI states reviewed from implementation only:
  - allowed/role-ready shell path is gated before nav rendering.
  - unauthenticated/forbidden states use shared `AccessNotice`.
  - default and domain routes render plain empty/stub workspaces.
  - backend-not-configured state is surfaced from `GET /meta` as `ServiceUnavailable`.

## Unresolved Questions

- `backend/.env.preprod.example` is referenced by the foundation plan, but no such file exists in this checkout. Only `backend/.env.example` was updated.
- The app-local frontend role list preserves the required role contract without changing `packages/auth-client`; a future shared-auth cleanup may add `grappa-dcim` to `APP_ACCESS_ROLES` if that shared package is opened for edits.
- Recommended tests, not added in this run: backend ACL boundaries for Viewer vs Operativo, `503` DB-not-configured behavior, launcher filtering when `GRAPPA_DSN` is empty, and static deep-link hosting.

## Deviations From Plan

- `pnpm-lock.yaml` was updated by `pnpm install` to register the new workspace importer. This was necessary for the required frontend build and future frozen-lockfile Docker installs.
- `packages/auth-client` was not modified because it was outside the slice write scope. The Grappa DCIM frontend still uses `getAppAccessState`, but passes app-local access roles instead of `APP_ACCESS_ROLES['grappa-dcim']`.
- `pnpm-workspace.yaml` did not need a change because it already includes `apps/*`.
- No browser/dev-server smoke was performed because no suitable server was running and the verification contract did not require starting one.
