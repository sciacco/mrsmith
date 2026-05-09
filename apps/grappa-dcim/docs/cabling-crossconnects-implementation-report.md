# Cabling Crossconnects Implementation Report

## Files Changed

- Backend:
  - `backend/internal/grappadcim/handler.go`
  - `backend/internal/grappadcim/cabling_types.go`
  - `backend/internal/grappadcim/plenums.go`
  - `backend/internal/grappadcim/cables.go`
  - `backend/internal/grappadcim/fibers.go`
  - `backend/internal/grappadcim/xcon.go`
- Frontend:
  - `apps/grappa-dcim/src/api/types.ts`
  - `apps/grappa-dcim/src/api/queries.ts`
  - `apps/grappa-dcim/src/routes.tsx`
  - `apps/grappa-dcim/src/features/cabling/CablingPages.tsx`
  - `apps/grappa-dcim/src/features/cabling/cabling.module.css`
  - `apps/grappa-dcim/src/features/xcon/XconPages.tsx`
- Report:
  - `apps/grappa-dcim/docs/cabling-crossconnects-implementation-report.md`

## Behavior Implemented

- Replaced the `Plenum`, `Cavi e fibre`, and `Cross connect` stubs with working data workspaces.
- Viewer paths are read-only: mutation controls render only when `meta.canOperate` is true, and backend writes remain behind `RequireOperativo`.
- Plenum list/detail, create/update/delete, matrix read, and explicit matrix initialization are implemented.
- Cable list/detail, create/update/delete, generated fiber listing, and fiber assignment are implemented.
- Port list supports plenum/status filtering and assignment-safe filtering.
- Cross connect list/detail, active/ceased tabs, create/update, product option loading, and ordered hop replacement are implemented.
- No automated tests were added.

## Endpoint List Implemented

- `GET /grappa-dcim/v1/plenums`
- `POST /grappa-dcim/v1/plenums`
- `GET /grappa-dcim/v1/plenums/{id}`
- `PATCH /grappa-dcim/v1/plenums/{id}`
- `DELETE /grappa-dcim/v1/plenums/{id}`
- `GET /grappa-dcim/v1/plenums/{id}/matrix`
- `POST /grappa-dcim/v1/plenums/{id}/initialize-matrix`
- `GET /grappa-dcim/v1/cables`
- `POST /grappa-dcim/v1/cables`
- `GET /grappa-dcim/v1/cables/{id}`
- `PATCH /grappa-dcim/v1/cables/{id}`
- `DELETE /grappa-dcim/v1/cables/{id}`
- `GET /grappa-dcim/v1/cables/{id}/fibers`
- `PATCH /grappa-dcim/v1/fibers/{id}/assignment`
- `GET /grappa-dcim/v1/ports`
- `GET /grappa-dcim/v1/xcon`
- `POST /grappa-dcim/v1/xcon`
- `GET /grappa-dcim/v1/xcon/{id}`
- `PATCH /grappa-dcim/v1/xcon/{id}`
- `PUT /grappa-dcim/v1/xcon/{id}/hops`
- `GET /grappa-dcim/v1/xcon/product-options`

## Route List Implemented

- `/plenum`
- `/plenum/:plenumId`
- `/cavi-fibre`
- `/cavi-fibre/:cableId`
- `/cross-connect`
- `/cross-connect/:xconId`

## Matrix Initialization Behavior

- Plenum create inserts only into `plenums`; it does not create `pl_slots`.
- Matrix read calculates the approved 288-cell view from 2 cables x 12 termination points x 12 fibers.
- Missing `pl_slots` render as incomplete configuration and missing cells, not as free fibers.
- `POST /plenums/{id}/initialize-matrix` runs in a transaction, locks the plenum row, locks existing `pl_slots`, and inserts only missing `(cable, num)` pairs for cable 1/2 and num 1..12.
- Initialization is idempotent at the application level: existing cable/num rows are preserved.

## Fiber Assignment Transaction Behavior

- Cable create runs in one transaction: it inserts the cable and generates fibers `1..N` with status `Libera`.
- Cable delete runs in one transaction and is blocked with `409 cable_fibers_assigned` unless every fiber is `Libera`, has no left/right port assignment, and has no `ports.cable_fiber_id` references.
- Fiber assignment runs in one transaction: it locks the fiber, locks old and target ports, rejects target ports already assigned to another fiber with `409 fiber_assignment_conflict`, clears old port links, sets new port links, and updates the fiber status to `Occupata` or `Libera`.

## Xcon Tab Semantics

- Active tab query is `LOWER(TRIM(stato)) <> 'cessata'`.
- Ceased tab query is `LOWER(TRIM(stato)) = 'cessata'`.
- `annullato` remains in the active query and is offered as a status option.
- Xcon writes touch only `xcon`; hop replacement touches only `xcon_hop`.
- Hop replacement is transactional: it locks the xcon row, deletes existing hops for that xcon, and inserts the submitted ordered hop set.

## Map-Only Crossconnects Validation Result

- Schema evidence confirms `crossconnects` is a separate map/report table with MMR, fiber, status, and service fields. It is not used as the V1 cross-connect write target.
- The implementation does not mutate `crossconnects`.
- Plenum matrix read exposes a separate `mapOnlyRecords` count for `crossconnects.mmr_id = plenum.datacenter_id`; it does not derive xcon state, endpoint ownership, or inventory side effects from that table.
- Live reconciliation between `crossconnects` and `xcon` was not validated because no live Grappa database/browser session was available.

## Contracts Preserved

- Viewer remains read-only in frontend and backend.
- Operativo-only writes are enforced through `RequireOperativo`.
- Plenum matrix initialization, cable create/delete, fiber assignment, and xcon hop replace are transactional.
- Cable delete is blocked unless all fibers are free and unassigned.
- Fiber assignment rejects double assignment conflicts.
- Xcon writes mutate only `xcon` and `xcon_hop`.
- Active/ceased xcon tab semantics are preserved exactly, including `annullato` in active.
- No fiber rings/topology/artifacts, CWDM, TIM GEA, Hive sync, polling, alerting, or first-class `cassetti_ottici` UI/API was implemented.
- User-facing copy stays operational Italian and avoids raw table/source-of-truth explanations.

## Commands Run

- `pnpm --filter mrsmith-grappa-dcim build`
  - PASS. TypeScript compiled and Vite built successfully.
- `go build ./cmd/server` from `backend`
  - PASS. Backend compiled successfully with no output.
- `gofmt -w backend/internal/grappadcim/...slice files...`
  - PASS. Formatting applied.
- `gofmt -l backend/internal/grappadcim`
  - PASS. No files listed.
- `rg --files apps/grappa-dcim backend/internal/grappadcim | rg '(_test\.go|\.test\.|\.spec\.)'`
  - PASS. No automated test files found.
- `lsof -nP -iTCP:5191 -sTCP:LISTEN`, `lsof -nP -iTCP:8080 -sTCP:LISTEN`, `lsof -nP -iTCP:5173 -sTCP:LISTEN`
  - No suitable dev server listener found.

## Manual and Browser Checks

- Browser checks were not run because no suitable Grappa DCIM dev server or backend was already listening, and the local instruction says to reuse an existing server before Playwright/browser checks rather than starting a second server.
- Populated plenum matrix, incomplete matrix, fiber conflict, active/ceased xcon tabs, destructive confirmations, and narrow matrix behavior were reviewed from code only.
- Live DB/API behavior was not exercised.

## UI Review

- Post-implementation UI review status: approved by code-first review.
- Evidence: approved `data_workspace` plan, explicit plenum matrix and cross-connect exceptions, comparable screens inspected (`apps/energia-dc/src/pages/SituazioneRackPage.tsx`, `apps/manutenzioni/src/pages/MaintenanceDetailPage.tsx`, `apps/rda/src/components/POWorkspacePanels.tsx`), and implementation files listed above.
- Residual UI risk: no screenshots were available, so rendered spacing, populated live data density, and narrow viewport behavior remain browser verification gaps.

## Unresolved Source Validations

- Live SQL behavior against a populated Grappa database remains unproven.
- Exact legacy default values for plenum/cable status and xcon status casing should be verified with production data.
- `crossconnects` to `xcon` reconciliation remains schema-only; no live map report parity was validated.
- Port status side effects are implemented with `Empty` and `Linked`; legacy usages of `Used` and `Xcon` are preserved when read, but write transitions beyond fiber assignment were not expanded.

## Deviations From Plan

- The frontend uses an app-local authenticated `DELETE` helper for destructive deletes because the shared API client exposes `delete(path)` without a JSON body parameter. Backend endpoint shapes remain the planned `DELETE` routes and still require double confirmation.
- Cross-connect detail implements the planned tabs visually, but all detail sections render in one detail panel for this slice; no separate nested tab state was added.
- Map-only `crossconnects` is represented as a separate count only. No matrix visual indicators are derived from it until live reconciliation is validated.

## Recommended Tests Not Added

- Transaction rollback for plenum matrix initialization.
- Cable create fiber generation and cable delete blocking.
- Fiber double-assignment conflict.
- Xcon hop replacement rollback.
- Viewer/Operativo backend authorization for each new write route.
