# Grappa DCIM Cabling Crossconnects QA Rerun

Status: PASS

## Rerun Scope

- Rerun target: remediation 1 for the `cabling-crossconnects` slice.
- QA phase: post-implementation QA gate rerun with code-first UI review.
- Required rerun inputs:
  - `apps/grappa-dcim/docs/cabling-crossconnects-run.md`
  - `apps/grappa-dcim/docs/cabling-crossconnects-implementation-report.md`
  - `apps/grappa-dcim/docs/cabling-crossconnects-remediation-1.md`
  - `apps/grappa-dcim/docs/cabling-crossconnects-fix-1-report.md`
  - previous `apps/grappa-dcim/docs/cabling-crossconnects-qa.md`
- Source/spec references checked:
  - `apps/grappa-dcim/docs/cabling-crossconnects-impl.md`
  - `apps/grappa-dcim/docs/grappa-dcim-spec.md`
  - `apps/grappa-dcim/docs/planning-ui-review.md`
  - `docs/UI-UX.md`
  - `docs/IMPLEMENTATION-PLANNING.md`
  - `docs/IMPLEMENTATION-KNOWLEDGE.md`
  - `docs/grappa/GRAPPA.md`
  - `.agents/skills/portal-miniapp-ui-review/SKILL.md`
  - `.agents/skills/portal-miniapp-ui-review/references/blocking-gates.md`
  - `.agents/skills/portal-miniapp-ui-review/references/evidence-checklist.md`
- Schema evidence checked:
  - `docs/grappa/grappa_plenums.json`
  - `docs/grappa/grappa_pl_slots.json`
  - `docs/grappa/grappa_slots.json`
  - `docs/grappa/grappa_ports.json`
  - `docs/grappa/grappa_cables.json`
  - `docs/grappa/grappa_fibers.json`
  - `docs/grappa/grappa_xcon.json`
  - `docs/grappa/grappa_xcon_hop.json`
  - `docs/grappa/grappa_crossconnects.json`

## Changed Files Inspected

- Current worktree note:
  - `git diff --name-only` shows tracked repo/runtime files from foundation work, while the Grappa DCIM app, docs, and backend package are still untracked. The rerun therefore inspected the cabling slice files directly rather than relying on `git diff` alone.
- Remediation file inspected:
  - `backend/internal/grappadcim/cables.go`
- Cabling/crossconnect implementation files inspected:
  - `backend/internal/grappadcim/handler.go`
  - `backend/internal/grappadcim/cabling_types.go`
  - `backend/internal/grappadcim/plenums.go`
  - `backend/internal/grappadcim/cables.go`
  - `backend/internal/grappadcim/fibers.go`
  - `backend/internal/grappadcim/xcon.go`
  - `backend/internal/grappadcim/helpers.go`
  - `backend/internal/grappadcim/dependencies.go`
  - `apps/grappa-dcim/src/api/client.ts`
  - `apps/grappa-dcim/src/api/queries.ts`
  - `apps/grappa-dcim/src/api/types.ts`
  - `apps/grappa-dcim/src/routes.tsx`
  - `apps/grappa-dcim/src/features/cabling/CablingPages.tsx`
  - `apps/grappa-dcim/src/features/cabling/cabling.module.css`
  - `apps/grappa-dcim/src/features/xcon/XconPages.tsx`
  - `apps/grappa-dcim/src/features/facilities/workspace.module.css`
  - `apps/grappa-dcim/src/App.tsx`
  - `apps/grappa-dcim/src/styles/global.css`
  - `apps/grappa-dcim/vite.config.ts`
  - `apps/grappa-dcim/package.json`

## Remediation Closure

- PASS. The previous blocking finding is closed.
- Expected behavior: cable delete must be blocked unless every fiber belonging to the cable is free and unassigned, including dependencies represented through `ports.cable_fiber_id`, `ports.fo_in_id`, and `ports.fo_out_id`.
- Actual rerun result: `backend/internal/grappadcim/cables.go` now locks all fibers for the cable, rejects non-`Libera` fibers and `left_port_id`/`right_port_id` assignments, then checks and locks `ports` rows where `cable_fiber_id IN (...) OR fo_in_id IN (...) OR fo_out_id IN (...)` before deleting fibers or the cable.
- Evidence: `docs/grappa/grappa_ports.json` confirms `fo_in_id`, `fo_out_id`, and `cable_fiber_id`; `backend/internal/grappadcim/cables.go` now includes all three fields in the delete dependency guard and still returns the existing `409 cable_fibers_assigned` path when references exist.
- Contract preserved: the destructive double-confirmation body requirement remains unchanged through `decodeDestructiveBody`.

## Product Behavior Gate

- PASS. The implementation report exists and records files changed, implemented behaviors, endpoint/route lists, matrix initialization behavior, fiber assignment transaction behavior, xcon tab semantics, map-only validation, commands, browser skip reason, deviations, unresolved validations, and recommended tests not added.
- PASS. Plenum create inserts only into `plenums`; it does not implicitly create `pl_slots`.
- PASS. Matrix initialization is explicit and transactional: it locks the plenum row, locks existing `pl_slots`, and inserts only missing cable 1/2 and number 1..12 termination rows.
- PASS. Missing matrix slots render as incomplete/missing cells rather than free fibers.
- PASS. Cable create is transactional and generates `fibers` rows numbered `1..N` with status `Libera`.
- PASS. Cable delete is now dependency-safe for `fibers.left_port_id`, `fibers.right_port_id`, `ports.cable_fiber_id`, `ports.fo_in_id`, and `ports.fo_out_id`.
- PASS. Fiber assignment is transactional, locks the fiber and affected ports, clears previous `ports.cable_fiber_id` links, sets new links, updates fiber status, and returns `409 fiber_assignment_conflict` for target-port conflicts.
- PASS. Xcon active/ceased semantics match the plan: active uses `LOWER(TRIM(x.stato)) <> 'cessata'`, ceased uses `LOWER(TRIM(x.stato)) = 'cessata'`, so `annullato` remains in the active query.
- PASS. Xcon writes mutate only `xcon`, and hop replacement mutates only `xcon_hop`; no xcon write path mutates inventory, port, cable, fiber, plenum, rack, or equipment tables.
- PASS. `crossconnects` is read only for the map-only count; no mutation of `crossconnects` was found.

## Repo and Runtime Gate

- PASS. Required frontend routes are registered:
  - `/plenum`
  - `/plenum/:plenumId`
  - `/cavi-fibre`
  - `/cavi-fibre/:cableId`
  - `/cross-connect`
  - `/cross-connect/:xconId`
- PASS. Required backend endpoints are registered under `/grappa-dcim/v1/...`, and the frontend uses the established `/api` client prefix.
- PASS. Frontend build wiring remains repo-fit: package name `mrsmith-grappa-dcim`, Vite build base `/apps/grappa-dcim/`, Vite dev port `5191`, and `/api` plus `/config` proxies.
- PASS. Static mini-app design wiring remains aligned with `docs/UI-UX.md`: `apps/grappa-dcim/src/styles/global.css` imports the clean theme and uses the approved mini-app background.
- PASS. No automated test files were added without approval. `rg --files apps/grappa-dcim backend/internal/grappadcim | rg '(_test\.go|\.test\.|\.spec\.)'` returned no matches.

## Data and Auth Gate

- PASS. Backend cabling and xcon write routes are protected by `RequireOperativo`; read routes use the Grappa DCIM Viewer/Operativo access role gate.
- PASS. Frontend mutation controls for plenum creation/edit/delete, matrix initialization, cable creation/edit/delete, fiber assignment, xcon creation/edit, and hop editing render only when `meta.canOperate` is true.
- PASS. Destructive plenum and cable deletes still require backend double confirmation, and the frontend sends `{ confirmPrimary: true, confirmSecondary: true }`.
- PASS. The remediation stayed backend-only and did not widen frontend permissions or bypass the authenticated transport used for destructive deletes.
- PASS. Xcon hop replacement locks the parent `xcon` row, deletes old hops, and inserts the submitted ordered hop set in one transaction.

## UI Review Gate

- Status: approved by code-first post-implementation review.
- Evidence package: approved `data_workspace` plan, explicit plenum matrix and cross-connect exceptions, comparable repo screens cited by the plan, implementation files, route scope, UI blocking gates, and source docs were available.
- PASS. Plenum, cable/fiber, and cross-connect screens use compact workspace headers, toolbars, tables, detail panels, modals, tabs/segmented controls, and a functional matrix surface.
- PASS. No launcher, hero, marketing dashboard, ornamental KPI row, Matrix portal styling, or decorative shell drift was found in the reviewed cabling/xcon screens.
- PASS. Matrix counts are limited to approved real local context: free cells, assigned cells, incomplete configuration, and map-only references.
- PASS. User-facing copy is operational Italian and business-facing. The reviewed UI does not expose raw HTTP status text, raw backend handler language, SQL, framework copy, or source-of-truth explanations.
- PASS. Cross-connect UI exposes `Attivi` and `Cessati` tabs and keeps `annullato` as a status option outside the ceased-only query.
- Residual UI verification gap: screenshots were not available, so populated matrix density, incomplete matrix rendering, fiber conflict rendering, destructive modals, active/ceased xcon tabs, and narrow viewport behavior were reviewed from code only.

## Verification Commands Run

- `gofmt -l backend/internal/grappadcim`
  - PASS. No files listed.
- `go build ./cmd/server` from `backend`
  - PASS. Backend compiled successfully with no output.
- `pnpm --filter mrsmith-grappa-dcim build`
  - PASS. TypeScript compiled and Vite production build completed successfully.
- `rg --files apps/grappa-dcim backend/internal/grappadcim | rg '(_test\.go|\.test\.|\.spec\.)'`
  - PASS. No automated test files found; command exited 1 because there were no matches.
- Existing dev server checks:
  - `lsof -nP -iTCP:5191 -sTCP:LISTEN` returned no listener.
  - `lsof -nP -iTCP:8080 -sTCP:LISTEN` returned no listener.
  - `lsof -nP -iTCP:5173 -sTCP:LISTEN` returned no listener.
- Focused code/schema checks:
  - Confirmed cabling/xcon route registration.
  - Confirmed `RequireOperativo` write registration.
  - Confirmed transactional cable create/delete, matrix initialization, fiber assignment, and xcon-hop replacement paths.
  - Confirmed xcon write isolation from inventory tables.
  - Confirmed `crossconnects` is map-count read only.
  - Confirmed the remediated cable delete guard includes `ports.cable_fiber_id`, `ports.fo_in_id`, and `ports.fo_out_id`.

## Manual and Browser Checks

- Browser checks were not run.
- Reason: no suitable Grappa DCIM Vite dev server or backend server was already listening on `5191`, `8080`, or `5173`, and the local project instruction requires reusing an existing suitable server before Playwright/browser checks rather than starting a second server for this QA gate.
- Populated plenum matrix, incomplete matrix, fiber assignment conflict state, active and ceased xcon tabs, destructive confirmations, and mobile/narrow matrix behavior were reviewed from implementation code only.
- Live Grappa DB/API behavior was not exercised.

## Residual Risks

- SQL behavior against live Grappa data remains unproven, including mixed legacy cabling data using `ports.fo_in_id` and `ports.fo_out_id`.
- Browser visual QA remains outstanding for populated, incomplete, conflict, destructive-confirm, active/ceased, and narrow viewport states.
- Exact legacy defaults for plenum, cable, and xcon statuses remain source-data assumptions until validated against production data.
- `crossconnects` to `xcon` reconciliation remains schema-only; implementation exposes only a map-only count.
- No transaction rollback tests were added, consistent with the no-tests-without-approval rule.
