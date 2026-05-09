# Grappa DCIM Facilities Layout QA - Overall Remediation 1 Rerun

Status: PASS

## Review Scope

- QA phase: post-implementation rerun for `apps/grappa-dcim/docs/overall-remediation-1.md`.
- Owning slice: `facilities-layout`.
- Gate type: code-first product/data/UI review, plus local build verification.
- Result: the previous overall QA blockers are closed.

## Source Docs Checked

- `apps/grappa-dcim/docs/facilities-layout-run.md`
- `apps/grappa-dcim/docs/facilities-layout-impl.md`
- `apps/grappa-dcim/docs/facilities-layout-qa.md`
- `apps/grappa-dcim/docs/overall-qa.md`
- `apps/grappa-dcim/docs/overall-remediation-1.md`
- `apps/grappa-dcim/docs/facilities-layout-overall-fix-1-report.md`
- `apps/grappa-dcim/docs/grappa-dcim-spec.md`
- `docs/UI-UX.md`
- `docs/IMPLEMENTATION-PLANNING.md`
- `docs/IMPLEMENTATION-KNOWLEDGE.md`
- `.agents/skills/portal-miniapp-ui-review/SKILL.md`
- `.agents/skills/portal-miniapp-ui-review/references/evidence-checklist.md`
- `.agents/skills/portal-miniapp-ui-review/references/blocking-gates.md`

## Files Reviewed

- `apps/grappa-dcim/src/api/client.ts`
- `apps/grappa-dcim/src/api/queries.ts`
- `apps/grappa-dcim/src/api/types.ts`
- `apps/grappa-dcim/src/features/facilities/FacilitiesPages.tsx`
- `apps/grappa-dcim/src/features/facilities/workspace.module.css`
- `apps/grappa-dcim/src/features/racks/RackPages.tsx`
- `apps/grappa-dcim/src/App.tsx`
- `apps/grappa-dcim/src/routes.tsx`
- Backend route/type/query contract references:
  - `backend/internal/grappadcim/handler.go`
  - `backend/internal/grappadcim/helpers.go`
  - `backend/internal/grappadcim/facilities.go`
  - `backend/internal/grappadcim/facilities_datacenters.go`
  - `backend/internal/grappadcim/layout.go`
  - `backend/internal/grappadcim/layout_types.go`
  - `backend/internal/grappadcim/racks.go`
  - `backend/internal/grappadcim/racks_types.go`
  - `backend/internal/grappadcim/racks_units_media.go`
  - `backend/internal/grappadcim/power.go`
  - `backend/internal/grappadcim/power_types.go`

## Findings

No blocking findings remain.

## Previous Overall QA Blockers

### Facilities/Layout Hard Deletes

PASS. Operativo-only hard-delete paths are now exposed for buildings, sale/MMR, islets, positions, racks, and rack sockets.

- Frontend mutations exist for `deleteBuilding`, `deleteDatacenter`, `deleteIslet`, `deletePosition`, `deleteRack`, and `deleteRackSocket` in `apps/grappa-dcim/src/api/queries.ts`.
- DELETE-with-body transport sends authenticated `DELETE /api/grappa-dcim/v1/...` requests with JSON payload support in `apps/grappa-dcim/src/api/queries.ts`.
- Visible delete controls are rendered only when `meta.canOperate` is true in `FacilitiesPages.tsx` and `RackPages.tsx`.
- Backend routes remain Operativo-protected through `RequireOperativo` in `backend/internal/grappadcim/handler.go`.
- Backend destructive handlers still require `confirmPrimary` and `confirmSecondary` through `decodeDestructiveBody` in `backend/internal/grappadcim/helpers.go`.

### Isole E Posizioni CRUD

PASS. `/isole-posizioni` now includes compact Operativo-only controls for islet create/edit/delete and position edit/delete, while retaining batch position creation.

- Islet create/edit/delete UI and mutations are present in `FacilitiesPages.tsx` and `queries.ts`.
- Position edit/delete UI and mutations are present in `FacilitiesPages.tsx` and `queries.ts`.
- Viewer users keep read-only access because all mutation controls are gated by `canOperate`.
- The layout remains a compact data workspace with selectors, tables, position grid, and modals rather than a hero/dashboard shell.

### Rack Socket CRUD

PASS. Rack detail now exposes socket create/edit/delete on the `Socket` tab for Operativo users.

- Query/type layer includes `RackSocketInput`, `saveRackSocket`, and `deleteRackSocket`.
- UI includes `Nuovo socket`, `Modifica`, and `Elimina` actions gated by `canOperate`.
- Socket delete uses the same two-checkbox confirmation modal and sends the double-confirmation payload.
- Backend socket delete dependency checks for historical power readings remain authoritative.

### Rack Media Replace

PASS. Rack detail now exposes media replacement on the `Media` tab for Operativo users.

- Query/type layer includes `RackMediaWrite`, `RackMediaInput`, and `replaceRackMedia`.
- UI includes `Sostituisci media` and row-level replace actions gated by `canOperate`.
- The frontend sends the existing JSON contract `{ items: [{ unitId, side, path }] }` to `PUT /grappa-dcim/v1/racks/{id}/media`.
- Backend media replacement validates that the unit belongs to the selected rack before upsert/delete.

### Double Confirmation Payload

PASS. Destructive UI actions require two modal checkboxes before calling the mutation and send `{ confirmPrimary: true, confirmSecondary: true }`.

- Facilities destructive body: `apps/grappa-dcim/src/features/facilities/FacilitiesPages.tsx`.
- Rack destructive body: `apps/grappa-dcim/src/features/racks/RackPages.tsx`.
- Payload type: `apps/grappa-dcim/src/api/types.ts`.
- Backend enforcement: `backend/internal/grappadcim/helpers.go`.

### Viewer Read-Only Behavior

PASS. Viewer behavior remains read-only for the reviewed facilities/layout/rack remediation paths.

- Create, edit, move, cease, delete, socket, and media mutation controls are rendered only when `meta.canOperate` is true.
- Backend mutation routes remain protected by `RequireOperativo`; read routes remain available to the Grappa DCIM access roles.

### Compact Workspace UI

PASS. The reviewed UI keeps the approved `data_workspace` archetype.

- Facilities and rack pages use compact headers, toolbars, tables, grids, tabs, and modals.
- No launcher, hero, decorative dashboard shell, or fake KPI band was introduced.
- Responsive CSS stacks split panes, forms, detail grids, and socket rows at narrow widths.
- User-facing copy remains business-facing Italian. Search found no user-facing source table names, raw HTTP labels, auth labels, or framework copy in the reviewed facilities/rack UI.

### No Tests Added

PASS. No automated tests were added.

- `rg --files apps/grappa-dcim backend/internal/grappadcim | rg '(_test\.go|\.test\.|\.spec\.)'` returned no matches.

## Verification Commands

- `pnpm --filter mrsmith-grappa-dcim build`
  - PASS. TypeScript compiled and Vite built the Grappa DCIM app successfully.
- `go build ./cmd/server` from `backend`
  - PASS. Backend build completed successfully.
- `rg --files apps/grappa-dcim backend/internal/grappadcim | rg '(_test\.go|\.test\.|\.spec\.)'`
  - PASS. No matching automated test files found.
- `lsof -nP -iTCP:5191 -sTCP:LISTEN`
  - No listener found.
- `lsof -nP -iTCP:8080 -sTCP:LISTEN`
  - No listener found.
- `lsof -nP -iTCP:5173 -sTCP:LISTEN`
  - No listener found.

## Manual And Browser Checks

Browser checks were not run. No suitable Grappa DCIM frontend/backend server was already listening on ports `5191`, `8080`, or `5173`, and the repo instruction says to reuse a suitable existing server before Playwright/browser checks rather than starting a second server.

The following states were reviewed from code only:

- Operativo hard-delete controls and double-confirm modals.
- Viewer hidden mutation controls.
- `/isole-posizioni` islet/position CRUD controls.
- `/rack/:rackId` socket CRUD controls.
- `/rack/:rackId` media replacement controls.
- Compact desktop and narrow responsive layout structure.

## Residual Risks

- Live Grappa DB behavior for dependency-blocked deletes, socket mutations, and media replacement was not exercised.
- Browser screenshot evidence for populated, empty, destructive-confirm, and narrow viewport states remains outstanding.
- These are verification gaps only; no known blocker remains from overall remediation 1.
