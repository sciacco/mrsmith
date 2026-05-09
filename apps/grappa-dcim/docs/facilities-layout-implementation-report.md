# Grappa DCIM Facilities Layout Implementation Report

## Files Changed

- Backend:
  - `backend/internal/grappadcim/handler.go`
  - `backend/internal/grappadcim/helpers.go`
  - `backend/internal/grappadcim/types.go`
  - `backend/internal/grappadcim/dependencies.go`
  - `backend/internal/grappadcim/facilities.go`
  - `backend/internal/grappadcim/facilities_datacenters.go`
  - `backend/internal/grappadcim/facilities_map.go`
  - `backend/internal/grappadcim/facilities_types.go`
  - `backend/internal/grappadcim/layout.go`
  - `backend/internal/grappadcim/layout_types.go`
  - `backend/internal/grappadcim/racks.go`
  - `backend/internal/grappadcim/racks_types.go`
  - `backend/internal/grappadcim/racks_units_media.go`
  - `backend/internal/grappadcim/power.go`
  - `backend/internal/grappadcim/power_types.go`
- Frontend:
  - `apps/grappa-dcim/src/App.tsx`
  - `apps/grappa-dcim/src/routes.tsx`
  - `apps/grappa-dcim/src/api/queries.ts`
  - `apps/grappa-dcim/src/api/types.ts`
  - `apps/grappa-dcim/src/features/facilities/FacilitiesPages.tsx`
  - `apps/grappa-dcim/src/features/facilities/workspace.module.css`
  - `apps/grappa-dcim/src/features/racks/RackPages.tsx`
- Report:
  - `apps/grappa-dcim/docs/facilities-layout-implementation-report.md`

## Behavior Implemented

- Replaced the foundation stubs for `Edifici`, `Sale e MMR`, `Rack`, and `Isole e posizioni` with working clean mini-app workspaces.
- Added building registry with active/all filtering, search, create/update form, portal exposure display, capacity/count context, and cease confirmation.
- Added Sale/MMR registry with active/all and room/MMR filters, detail selection, room/MMR distinction, MMR path display, and physical map panel backed by islets, positions, and racks.
- Added layout workspace for selecting a sala, islet, position grid rendering, and batch position creation with the block-if-existing backend rule.
- Added rack registry and detail workspace with tabs for `Riepilogo`, `Unita rack`, `Socket`, `Media`, `Potenza`, and `Storico`.
- Added rack create/update, explicit rack move, and rack cease flows for Operativo users.
- Viewer remains read-only in the frontend because mutation actions render only when `meta.canOperate` is true. Backend mutation routes are protected by Operativo roles.
- Reads preserve legacy/free-text values by returning stored status/type/position/socket values without normalizing unknown values.
- Mutations validate only V1-created/edited controlled values such as building/datacenter/rack lifecycle status, rack Full/Half placement, and position batch count/type.

## Endpoint List Implemented

- Buildings:
  - `GET /grappa-dcim/v1/facilities/buildings`
  - `POST /grappa-dcim/v1/facilities/buildings`
  - `GET /grappa-dcim/v1/facilities/buildings/{id}`
  - `PATCH /grappa-dcim/v1/facilities/buildings/{id}`
  - `POST /grappa-dcim/v1/facilities/buildings/{id}/cease`
  - `DELETE /grappa-dcim/v1/facilities/buildings/{id}`
- Datacenters/MMR:
  - `GET /grappa-dcim/v1/facilities/datacenters?kind=room|mmr|all&status=active|all`
  - `POST /grappa-dcim/v1/facilities/datacenters`
  - `GET /grappa-dcim/v1/facilities/datacenters/{id}`
  - `PATCH /grappa-dcim/v1/facilities/datacenters/{id}`
  - `POST /grappa-dcim/v1/facilities/datacenters/{id}/cease`
  - `DELETE /grappa-dcim/v1/facilities/datacenters/{id}`
  - `GET /grappa-dcim/v1/facilities/datacenters/{id}/map`
  - `GET /grappa-dcim/v1/facilities/datacenters/{id}/ports`
  - `POST /grappa-dcim/v1/facilities/datacenters/{id}/ports`
- Layout:
  - `GET /grappa-dcim/v1/layout/islets?datacenterId=...`
  - `POST /grappa-dcim/v1/layout/islets`
  - `PATCH /grappa-dcim/v1/layout/islets/{id}`
  - `DELETE /grappa-dcim/v1/layout/islets/{id}`
  - `GET /grappa-dcim/v1/layout/islets/{id}/positions`
  - `POST /grappa-dcim/v1/layout/islets/{id}/positions/batch`
  - `PATCH /grappa-dcim/v1/layout/positions/{id}`
  - `DELETE /grappa-dcim/v1/layout/positions/{id}`
- Racks, media, sockets, power:
  - `GET /grappa-dcim/v1/racks`
  - `POST /grappa-dcim/v1/racks`
  - `GET /grappa-dcim/v1/racks/{id}`
  - `PATCH /grappa-dcim/v1/racks/{id}`
  - `POST /grappa-dcim/v1/racks/{id}/move`
  - `POST /grappa-dcim/v1/racks/{id}/cease`
  - `DELETE /grappa-dcim/v1/racks/{id}`
  - `GET /grappa-dcim/v1/racks/{id}/units`
  - `GET /grappa-dcim/v1/racks/{id}/media`
  - `PUT /grappa-dcim/v1/racks/{id}/media`
  - `GET /grappa-dcim/v1/racks/{id}/sockets`
  - `POST /grappa-dcim/v1/racks/{id}/sockets`
  - `PATCH /grappa-dcim/v1/rack-sockets/{socketId}`
  - `DELETE /grappa-dcim/v1/rack-sockets/{socketId}`
  - `GET /grappa-dcim/v1/racks/{id}/power-readings`
  - `GET /grappa-dcim/v1/racks/{id}/power-summary`

## Route List Implemented

- `/edifici`
- `/sale-mmr`
- `/sale-mmr/:datacenterId`
- `/rack`
- `/rack/:rackId`
- `/rack/:rackId/potenza`
- `/isole-posizioni`

## Dependency Checks Implemented

- Building cease/delete blocks on active datacenters, Customer Portal exposure, active racks, linked apparati, linked servers, and linked optical cassette dependency rows.
- Datacenter cease/delete blocks on islets, active racks, linked apparati, linked servers, linked optical cassette dependency rows, and Customer Portal exposure.
- Islet delete blocks on occupied child positions and linked active racks.
- Position delete blocks on occupied state and linked active racks.
- Rack move locks and checks the target position; Full racks reject any occupant, Half racks reject Full occupants and same A/B vertical position conflicts.
- Rack cease/delete blocks on linked apparati, linked servers, optical cassette dependency rows, and non-empty ports.
- Rack socket delete blocks on historical power readings.
- Position batch create blocks when the islet already has positions.
- All cease/delete handlers require the double-confirmation request shape: `confirmPrimary: true` and `confirmSecondary: true`.

## Contracts Preserved

- Viewer remains read-only; backend mutations are protected by `app_grappadcim_operativo`.
- Backend read routes remain available to Viewer/Operativo through the foundation role model.
- No automated tests were added.
- No equipment, cabling/xcon, fiber topology/artifacts, credential flows, CWDM, TIM GEA, Hive sync, polling, alerting, or first-class cassetti_ottici UI was implemented.
- `rack_sockets` is used as the authoritative socket table.
- Rack cessation sets rack sockets to `Spento`.
- Stored unknown legacy status/type/free-text values are preserved on reads.
- Half rack A/B copy uses `posizione alta` / `posizione bassa`, not `lato`.
- User-facing UI copy stays Italian and does not expose source table names or backend/source mechanics.

## Commands Run

- `gofmt -w backend/internal/grappadcim`
  - Completed with no output.
- `go build ./cmd/server` from `backend`
  - Passed with no output.
- `pnpm --filter mrsmith-grappa-dcim build`
  - First run failed on one unused frontend import in `RackPages.tsx`.
  - After removal, passed: TypeScript compiled and Vite built `dist/index.html`, CSS, and JS assets successfully.
- `lsof -nP -iTCP:5191 -sTCP:LISTEN`
  - No listener found.
- `lsof -nP -iTCP:8080 -sTCP:LISTEN`
  - No listener found.
- `lsof -nP -iTCP:5173 -sTCP:LISTEN`
  - No listener found.
- `rg` checks for forbidden/V2 feature leakage and raw technical user-facing copy.
  - Matches were limited to backend dependency checks, CSS class names, existing stubs for future routes, or domain-safe wording.

## Manual/Browser Checks

- Browser checks were not run because no suitable Grappa DCIM Vite dev server or backend server was already listening on the expected ports, and the run contract said to reuse an existing server rather than start a second one.
- Code-backed UI states reviewed:
  - populated registry structures for buildings, sale/MMR, and racks.
  - rack detail tabs with unit map, socket panel, media, power summary, and reading history.
  - empty states for no buildings, no sale/MMR, no islets, no positions, no racks, no sockets, no media, and no power data.
  - dependency-blocked destructive flow through backend `409` dependency summaries and frontend error toasts.
  - mobile/narrow behavior through CSS grid collapse under `920px`.

## Unresolved Source Validations

- Exact legacy socket-row generation count during rack create is not proven by the schema evidence. The backend supports an explicit `socketCount`; the UI defaults to zero unless Operativo supplies a count.
- Exact datacenter and rack cessation cascade breadth was not fully implemented because the required child-domain slices are out of scope. This slice blocks lifecycle actions when active dependencies exist rather than mutating equipment, server, cabling, or optical-cassette domains.
- Generated PHP map files are intentionally not reproduced. The implemented map response composes live islets, positions, and racks into a functional layout payload.
- Map-only `crossconnects` visual references remain unvalidated and are not used as V1 source of truth in this slice.
- Port create is intentionally minimal and does not implement broader cabling/fiber assignment behavior.
- No live Grappa database/browser smoke was available in this run, so SQL behavior against production-like data remains a QA validation item.

## Deviations From Plan

- Hard-delete endpoints are implemented in the backend with dependency checks and double-confirmation request shape, but the current frontend exposes cease/move/create/update controls and does not surface delete buttons. This keeps the viewer workspace conservative until live source validation confirms which hard deletes should be user-visible.
- Datacenter and rack cease actions are conservative: they block when active dependencies exist instead of cascading into out-of-scope child domains.
- Rack create supports unit generation and optional socket generation, but automatic socket count is not inferred without source evidence.
- Power history is rendered as compact tables rather than charts. This preserves real power data without introducing additional chart dependencies or visual emphasis before live data review.
