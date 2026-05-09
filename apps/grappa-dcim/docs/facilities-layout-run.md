# Facilities Layout Run Contract

## Status

- Iteration: 1
- Dependency status: foundation accepted with `apps/grappa-dcim/docs/foundation-qa.md` reporting `Status: PASS`.
- Allowed write scope:
  - `backend/internal/grappadcim/facilities*.go`
  - `backend/internal/grappadcim/racks*.go`
  - `backend/internal/grappadcim/power*.go`
  - `backend/internal/grappadcim/layout*.go`
  - narrowly scoped additions to `backend/internal/grappadcim/handler.go`, `helpers.go`, and `types.go` needed to register and share facilities/rack handlers
  - `apps/grappa-dcim/src/features/facilities/*`
  - `apps/grappa-dcim/src/features/racks/*`
  - narrowly scoped additions to `apps/grappa-dcim/src/routes.tsx`, `App.tsx`, API query/type files, and shared app-local styles/components needed by this slice
  - `apps/grappa-dcim/docs/facilities-layout-implementation-report.md`
- Disallowed write scope:
  - Equipment, cabling, xcon, fiber-ring, artifact, and credential feature implementation files.
  - Repo-wide wiring already completed by foundation unless a build break caused by this slice requires a minimal fix.
  - Source table semantics, schema docs, or V1 scope changes.
  - Automated tests unless the human explicitly approves them.

## Required Reading

- `apps/grappa-dcim/docs/grappa-dcim-spec.md`
- `apps/grappa-dcim/docs/foundation-impl.md`
- `apps/grappa-dcim/docs/foundation-run.md`
- `apps/grappa-dcim/docs/foundation-implementation-report.md`
- `apps/grappa-dcim/docs/foundation-qa.md`
- `apps/grappa-dcim/docs/facilities-layout-impl.md`
- `apps/grappa-dcim/docs/planning-ui-review.md`
- `docs/UI-UX.md`
- `docs/IMPLEMENTATION-PLANNING.md`
- `docs/IMPLEMENTATION-KNOWLEDGE.md`
- `docs/grappa/GRAPPA.md`
- Schema evidence:
  - `docs/grappa/grappa_dc_build.json`
  - `docs/grappa/grappa_datacenter.json`
  - `docs/grappa/grappa_islets.json`
  - `docs/grappa/grappa_positions.json`
  - `docs/grappa/grappa_racks.json`
  - `docs/grappa/grappa_units.json`
  - `docs/grappa/grappa_media.json`
  - `docs/grappa/grappa_rack_sockets.json`
  - `docs/grappa/grappa_rack_power_readings.json`
  - `docs/grappa/grappa_rack_power_daily_summary.json`

## Implementation Target

- Implement the facilities and rack layout slice from `apps/grappa-dcim/docs/facilities-layout-impl.md`.
- Replace the foundation stubs for `Edifici`, `Sale e MMR`, `Rack`, and `Isole e posizioni` with working facilities/rack workspace screens.
- Implement backend read and mutation endpoints listed in the facilities plan where source evidence is sufficient.
- Preserve Viewer as read-only and Operativo as the only role allowed to mutate, move, cease, initialize batch positions, or delete.
- Enforce backend dependency checks and double-confirm destructive-action shape for cease/delete operations implemented in this slice.
- Preserve legacy/free-text values on reads; validate only V1-created or edited values.
- Keep UI copy business-facing Italian. Do not expose source table names, generated-row/cascade mechanics, raw backend errors, or role names.
- Do not add tests; record recommended tests in the implementation report.

## Verification Required

- Commands:
  - `pnpm --filter mrsmith-grappa-dcim build`
  - `go build ./cmd/server` from `backend`
- Manual/browser checks:
  - If a suitable dev server is already running, reuse it; otherwise do not start a second server.
  - Record whether populated/empty/error states were checked in browser. If not checked, explain why.
- UI review states:
  - populated desktop state for buildings/datacenters/racks, or code-backed equivalent if no DB/browser is available
  - rack detail with unit map and socket panel
  - empty state for no positions/no racks
  - dependency-blocked destructive confirmation
  - mobile/narrow layout behavior, checked by browser when feasible or recorded as a residual risk

## Reporting Required

Write `apps/grappa-dcim/docs/facilities-layout-implementation-report.md` with:
- files changed
- behavior implemented
- endpoint list implemented
- route list implemented
- dependency checks implemented
- contracts preserved
- commands run and outputs summarized
- manual/browser checks run or skipped with reason
- unresolved source validations
- deviations from plan
