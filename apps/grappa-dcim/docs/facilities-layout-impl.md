# Grappa DCIM Facilities and Rack Layout Implementation Plan

## Slice Contract

- Slice: `facilities-layout`
- Purpose: implement the physical hierarchy and rack layout workspace.
- Approved source: `apps/grappa-dcim/docs/grappa-dcim-spec.md`.
- Depends on: `apps/grappa-dcim/docs/foundation-impl.md` accepted.
- In-scope source surfaces:
  - `dc-build`
  - `datacenter-sala-cage`
  - `datacenter-mmr`
  - `racks`
  - `rack-sockets`
  - `islets`
  - `positions`
- Primary write ownership:
  - `backend/internal/grappadcim/facilities*.go`
  - `backend/internal/grappadcim/racks*.go`
  - `backend/internal/grappadcim/power*.go`
  - `apps/grappa-dcim/src/features/facilities/*`
  - `apps/grappa-dcim/src/features/racks/*`
  - route/nav additions for this slice only
- Required schema evidence for implementation validation:
  - `docs/grappa/grappa_dc_build.json`
  - `docs/grappa/grappa_datacenter.json`
  - `docs/grappa/grappa_islets.json`
  - `docs/grappa/grappa_positions.json`
  - `docs/grappa/grappa_racks.json`
  - `docs/grappa/grappa_units.json` if present in the schema index
  - `docs/grappa/grappa_media.json`
  - `docs/grappa/grappa_rack_sockets.json`
  - `docs/grappa/grappa_rack_power_readings.json`
  - `docs/grappa/grappa_rack_power_daily_summary.json`

## Comparable Apps Audit

- Reference 1: `apps/energia-dc/src/pages/SituazioneRackPage.tsx`, `apps/energia-dc/src/pages/shared.module.css`.
- Reference 2: `apps/manutenzioni/src/pages/MaintenanceListPage.tsx`, `apps/manutenzioni/src/pages/MaintenanceDetailPage.tsx`.
- Reused patterns:
  - cascading selectors for customer/building/room/rack paths from `energia-dc`.
  - rack detail cards, socket panels, and history table composition from `energia-dc`.
  - filter bars, active filter chips, table rows, empty/error states, and detail tabs from `manutenzioni`.
  - no full-width hero, no ornamental summary band.
- Rejected patterns:
  - `energia-dc` chart-first view as a default for registries. Grappa DCIM facilities are management records first; charts appear only in rack power history context.
  - `manutenzioni` radar lane concept. It is specific to scheduled windows and must not be reused as fake rack health metrics.

## Archetype Choice

- Selected archetype: `data_workspace`.
- Why it fits: the slice coordinates hierarchy selectors, registries, physical maps, rack detail, sockets, media, and lifecycle actions.
- Required states:
  - list loading, empty, filtered-empty, error
  - detail loading, not found, stale/deleted, dependency-blocked action
  - destructive confirm and dependency failure
  - rack move conflict
  - position batch blocked because positions already exist
  - map incomplete configuration

## User Copy Rules

- Allowed copy style: business-user-only Italian labels for facilities, rooms, rack positions, and actions.
- Approved user labels:
  - `Edifici`
  - `Sale e MMR`
  - `Rack`
  - `Isole e posizioni`
  - `Sposta rack`
  - `Cessa`
  - `Elimina`
  - `Socket rack`
  - `Storico potenza`
- Forbidden copy risks:
  - do not call half-rack `A`/`B` values "lato"; use `posizione alta` and `posizione bassa`.
  - do not show source table names such as `dc_build`, `datacenter`, `rack_sockets`.
  - do not expose "generated units" or "cascade" language in user-facing text.
- Metrics allowed: only real local counts such as visible racks, sockets in selected rack, power readings in current filter, or positions in a selected islet.

## Repo-Fit

- Route/base path: under `/apps/grappa-dcim/`.
- Planned frontend routes:
  - `/edifici`
  - `/sale-mmr`
  - `/sale-mmr/:datacenterId`
  - `/rack`
  - `/rack/:rackId`
  - `/rack/:rackId/potenza`
  - `/isole-posizioni`
- API prefix:
  - `/api/grappa-dcim/v1/facilities/...`
  - `/api/grappa-dcim/v1/racks/...`
  - `/api/grappa-dcim/v1/layout/...`
- Access role:
  - Viewer can read non-secret data and maps.
  - Operativo can create/update, move racks, initialize batch positions, cease/archive where allowed, and hard-delete only when dependency checks pass.
- Dev port / proxy notes: use foundation Vite port `5191` with `/api` and `/config` proxy.
- Static hosting / deployment notes: inherited from foundation.

## Domain Rules

- Buildings:
  - list, view, create, update, delete.
  - `status` values preserve source values such as `Attivo` and `Cessato`.
  - transition to `Cessato` and hard delete are blocked while active datacenters/MMRs, racks, apparati, services, or Customer Portal-exposed references depend on the building.
  - if cessation is allowed, set `ceased_at`/equivalent source timestamp only when empty.
  - `portale_clienti=1` means Customer Portal exposure and must remain visible.
- Datacenters and MMR:
  - preserve `ismmr=0` Sala/Cage and `ismmr=1` MMR split.
  - `mmr_type` is a short path identifier, not an enum.
  - preserve active filters, port operations, MMR hub context, and physical map behavior.
  - generated PHP map files are not reproduced literally.
  - map-only `crossconnects` visual references must be validated without changing the source of truth for V1 `xcon`/`xcon_hop`.
- Islets and positions:
  - delete is blocked if any child position is occupied.
  - batch creation must block if positions already exist for the target islet/shape.
  - positions preserve source statuses including `free`, `occupied`, and tolerated legacy values such as `reserved`.
- Racks:
  - create generates `units`, updates the selected position, and creates rack socket rows according to source behavior.
  - explicit rack move frees the old position, occupies the new position, and rejects conflicts.
  - `Full` racks occupy a full position with `pos=F`.
  - `Half` racks use `A` high or `B` low. At most one `A` and one `B` half rack may share a physical position.
  - cessation cascades child equipment/NICs/optical cassette archive dependencies/sockets/position according to approved source contract.
- Rack sockets and power:
  - `rack_sockets` is the authoritative table name.
  - rack cessation sets sockets to `Spento`.
  - preserve OID fields and historical readings.
  - no polling, polling cadence, or alerting in V1.
- Media:
  - preserve existing referenced front/back rack unit images and media records.
  - artifact storage mechanics can differ, but user-visible history must remain accessible.

## API Contract

- Buildings:
  - `GET /grappa-dcim/v1/facilities/buildings`
  - `POST /grappa-dcim/v1/facilities/buildings`
  - `GET /grappa-dcim/v1/facilities/buildings/{id}`
  - `PATCH /grappa-dcim/v1/facilities/buildings/{id}`
  - `POST /grappa-dcim/v1/facilities/buildings/{id}/cease`
  - `DELETE /grappa-dcim/v1/facilities/buildings/{id}`
- Datacenters/MMR:
  - `GET /grappa-dcim/v1/facilities/datacenters?kind=room|mmr&status=active|all`
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
- Racks:
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

## Frontend Plan

- Build a facilities workspace with a compact header and tabs, not a dashboard.
- Use table/list plus side panel or detail route for buildings and datacenters.
- Use a physical map panel for rooms/islets/positions. The map is a working visualization with selection and occupancy state, not decoration.
- Use `SingleSelect`, `SearchInput`, `Button`, `Modal`/`Drawer`, `Skeleton`, and shared empty/error states.
- For destructive actions, show two explicit confirmations and a dependency message when blocked.
- For rack detail, use tabs:
  - `Riepilogo`
  - `Unita rack`
  - `Socket`
  - `Media`
  - `Potenza`
  - `Storico`
- Keep actions near the entity they affect. Do not put action descriptions in separate explanatory panels.

## Backend Plan

- Use explicit transactions for create/move/cease/delete flows that touch positions, racks, units, sockets, or cascaded children.
- Dependency checks are backend-owned and must run inside the mutation flow.
- Do not rely on database FKs alone; schema docs show many legacy relationships are implicit.
- Use `sql.Null*` scanners for legacy nullable text/date/numeric columns.
- Preserve unknown legacy statuses. Validate only values that V1 creates or edits.
- Emit structured logs with component `grappa-dcim` and operation labels such as `rack_move`, `building_delete_dependencies`, `position_batch_create`.

## Verification

- UI review checks:
  - populated desktop state for buildings/datacenters/racks.
  - rack detail with unit map and socket panel.
  - empty state for no positions/no racks.
  - dependency-blocked destructive confirmation.
  - mobile/narrow layout stacks without overlap.
- Runtime / auth checks:
  - Viewer sees read-only actions only.
  - Operativo sees mutation actions and destructive confirmations.
  - backend rejects mutation endpoints for Viewer.
  - deep link `/apps/grappa-dcim/rack/:rackId` works after refresh.
- Tests:
  - No tests are authorized by this planning artifact.
  - The implementer should ask the human expert before adding transaction tests for rack create/move/cease, half-rack conflict, and dependency-blocked delete because those protect business-critical rules.
- Manual validation:
  - verify active filters for datacenter and racks.
  - verify `rack_sockets` FK to readings.
  - verify exact generated `units` count and socket side effects.
  - verify position batch blocked when positions exist.
  - verify quarter position values are tolerated in reads but not offered for V1 creation/edit.

## Agent Deliverables

- Implementation report: `apps/grappa-dcim/docs/facilities-layout-implementation-report.md`.
- QA report: `apps/grappa-dcim/docs/facilities-layout-qa.md`.
- Report must include:
  - endpoint list implemented
  - route list implemented
  - dependency checks implemented
  - unresolved source validations
  - build/manual verification output

## Exceptions

- Physical maps are an approved exception to a plain CRUD layout because the source spec requires user-visible map/layout behavior. They must remain functional and restrained, not decorative.
- Rack power history may use charts because power readings are real feature data. Do not add unrelated KPI cards.
