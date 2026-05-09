# Cabling Crossconnects Run Contract

## Status

- Iteration: 1
- Dependency status: foundation, facilities-layout, and equipment-compute-storage accepted with `Status: PASS` QA reports.
- Allowed write scope:
  - `backend/internal/grappadcim/plenums*.go`
  - `backend/internal/grappadcim/cables*.go`
  - `backend/internal/grappadcim/fibers*.go`
  - `backend/internal/grappadcim/ports*.go`
  - `backend/internal/grappadcim/xcon*.go`
  - narrowly scoped additions to `backend/internal/grappadcim/handler.go`, `helpers.go`, `types.go`, and shared dependency helpers needed to register and share this slice
  - `apps/grappa-dcim/src/features/cabling/*`
  - `apps/grappa-dcim/src/features/xcon/*`
  - narrowly scoped additions to `apps/grappa-dcim/src/routes.tsx`, API query/type files, and shared app-local styles/components needed by this slice
  - `apps/grappa-dcim/docs/cabling-crossconnects-implementation-report.md`
- Disallowed write scope:
  - Facilities/rack, equipment/server/storage/camera behavior except selector consumption.
  - Fiber rings/topology/artifacts, CWDM, TIM GEA, Hive sync, polling, alerting, or first-class `cassetti_ottici` implementation.
  - Repo-wide wiring already completed by foundation unless a build break caused by this slice requires a minimal fix.
  - Automated tests unless the human explicitly approves them.

## Required Reading

- `apps/grappa-dcim/docs/grappa-dcim-spec.md`
- `apps/grappa-dcim/docs/foundation-qa.md`
- `apps/grappa-dcim/docs/facilities-layout-qa.md`
- `apps/grappa-dcim/docs/equipment-compute-storage-qa.md`
- `apps/grappa-dcim/docs/cabling-crossconnects-impl.md`
- `apps/grappa-dcim/docs/planning-ui-review.md`
- `docs/UI-UX.md`
- `docs/IMPLEMENTATION-PLANNING.md`
- `docs/IMPLEMENTATION-KNOWLEDGE.md`
- `docs/grappa/GRAPPA.md`
- Schema evidence:
  - `docs/grappa/grappa_plenums.json`
  - `docs/grappa/grappa_pl_slots.json`
  - `docs/grappa/grappa_slots.json`
  - `docs/grappa/grappa_ports.json`
  - `docs/grappa/grappa_cables.json`
  - `docs/grappa/grappa_fibers.json`
  - `docs/grappa/grappa_xcon.json`
  - `docs/grappa/grappa_xcon_hop.json`
  - `docs/grappa/grappa_crossconnects.json`

## Implementation Target

- Implement the cabling, fiber assignment, and cross-connect slice from `apps/grappa-dcim/docs/cabling-crossconnects-impl.md`.
- Replace foundation stubs for `Plenum`, `Cavi e fibre`, and `Cross connect` with working workspaces.
- Implement backend endpoints listed in the cabling plan where source evidence is sufficient.
- Viewer can read matrices, cable/fiber inventory, port states, and xcon details only.
- Operativo can initialize matrices, create/update/delete unused cables, assign fibers, update ports, and create/update xcon/hops.
- Plenum create must not implicitly create `pl_slots`; matrix initialization must be explicit and transactional.
- Cable create must generate fibers transactionally; cable delete must be blocked unless all fibers are free and unassigned.
- Fiber assignment must be transactional, clear old port links, set new links atomically, and reject double assignment conflicts.
- Xcon writes must mutate only `xcon` and `xcon_hop`; no inventory side effects.
- Preserve active/ceased semantics exactly: active is `stato != 'cessata'`, ceased is `stato='cessata'`, and `annullato` remains in the active query.
- Do not add tests; record recommended tests in the implementation report.

## Verification Required

- Commands:
  - `pnpm --filter mrsmith-grappa-dcim build`
  - `go build ./cmd/server` from `backend`
- Manual/browser checks:
  - If a suitable dev server is already running, reuse it; otherwise do not start a second server.
  - Record whether populated/incomplete/conflict/xcon states were checked in browser. If not checked, explain why.
- UI review states:
  - populated plenum matrix state
  - incomplete matrix state
  - fiber assignment conflict state
  - active and ceased xcon tabs
  - mobile/narrow matrix behavior, checked by browser when feasible or recorded as a residual risk

## Reporting Required

Write `apps/grappa-dcim/docs/cabling-crossconnects-implementation-report.md` with:
- files changed
- behavior implemented
- endpoint list implemented
- route list implemented
- matrix initialization behavior
- fiber assignment transaction behavior
- xcon tab semantics
- map-only crossconnects validation result
- contracts preserved
- commands run and outputs summarized
- manual/browser checks run or skipped with reason
- unresolved source validations
- deviations from plan
