# Grappa DCIM Cabling, Fiber Assignment, and Cross Connect Implementation Plan

## Slice Contract

- Slice: `cabling-crossconnects`
- Purpose: implement plenum/cable/fiber/port workflows and cross-connect path documentation.
- Approved source: `apps/grappa-dcim/docs/grappa-dcim-spec.md`.
- Depends on: `foundation` and enough `facilities-layout` data for datacenter/MMR/rack selectors.
- In-scope source surfaces:
  - `plenums`
  - `dcimadmin-cable`
  - `xcon`
  - related port workflows under Sala/Cage and MMR context
- Primary write ownership:
  - `backend/internal/grappadcim/plenums*.go`
  - `backend/internal/grappadcim/cables*.go`
  - `backend/internal/grappadcim/fibers*.go`
  - `backend/internal/grappadcim/ports*.go`
  - `backend/internal/grappadcim/xcon*.go`
  - `apps/grappa-dcim/src/features/cabling/*`
  - `apps/grappa-dcim/src/features/xcon/*`
- Required schema evidence for implementation validation:
  - `docs/grappa/grappa_plenums.json`
  - `docs/grappa/grappa_pl_slots.json`
  - `docs/grappa/grappa_slots.json`
  - `docs/grappa/grappa_ports.json`
  - `docs/grappa/grappa_cables.json`
  - `docs/grappa/grappa_fibers.json`
  - `docs/grappa/grappa_xcon.json`
  - `docs/grappa/grappa_xcon_hop.json` if present in schema index
  - `docs/grappa/grappa_crossconnects.json` for map-only validation

## Comparable Apps Audit

- Reference 1: `apps/energia-dc/src/pages/SituazioneRackPage.tsx`.
- Reference 2: `apps/manutenzioni/src/pages/MaintenanceDetailPage.tsx`.
- Additional reference: `apps/rda/src/components/POWorkspacePanels.tsx`, `apps/rda/src/pages/PoDetailPage.tsx`.
- Reused patterns:
  - selector-driven workspace from `energia-dc`.
  - tabbed detail and action bars from `manutenzioni`.
  - readiness/error panels from `rda` for blocked operations.
  - compact tables with side panels for path/hop editing.
- Rejected patterns:
  - chart-driven visuals for fiber capacity. The plenum matrix is a working map, not a metric chart.
  - cards that summarize counts already visible in the fiber matrix or table.

## Archetype Choice

- Selected archetype: `data_workspace`.
- Why it fits: users coordinate a visual plenum matrix, cable/fiber inventory, port assignment, and xcon path details in one operational workspace.
- Required states:
  - uninitialized plenum matrix
  - initialized matrix with free/linked/used/xcon cells
  - fiber assignment conflict
  - cable delete blocked by assigned fibers
  - xcon active and ceased tabs
  - ordered hop empty state
  - save conflict requiring reload

## User Copy Rules

- Allowed copy style: operational Italian labels focused on cabling and customer circuits.
- Approved user labels:
  - `Plenum`
  - `Matrice fibre`
  - `Inizializza matrice`
  - `Cavi`
  - `Fibre`
  - `Assegna fibra`
  - `Cross connect`
  - `Attivi`
  - `Cessati`
  - `Ticket Esteso`
  - `Codice Ordine`
  - `Serial Number`
- Forbidden copy risks:
  - do not use raw table names in UI.
  - do not explain that `xcon` is the source of truth.
  - do not rename `CDL-X*` product codes into invented business descriptions.
  - do not call `annullato` ceased unless the source query says so.
- Metrics allowed: only real context counts such as visible fibers, free cells, assigned cells, and active/ceased xcon row counts within the current filter.

## Repo-Fit

- Route/base path: under `/apps/grappa-dcim/`.
- Planned frontend routes:
  - `/plenum`
  - `/plenum/:plenumId`
  - `/cavi-fibre`
  - `/cavi-fibre/:cableId`
  - `/cross-connect`
  - `/cross-connect/:xconId`
- API prefix:
  - `/api/grappa-dcim/v1/plenums/...`
  - `/api/grappa-dcim/v1/cables/...`
  - `/api/grappa-dcim/v1/fibers/...`
  - `/api/grappa-dcim/v1/ports/...`
  - `/api/grappa-dcim/v1/xcon/...`
- Access role:
  - Viewer can read matrices, cable/fiber inventory, port states, and xcon details.
  - Operativo can initialize matrices, create/update/delete unused cables, assign fibers, update ports, and create/update xcon/hops.
- Dev port / proxy notes: inherited from foundation.
- Static hosting / deployment notes: inherited from foundation.

## Domain Rules

- Plenums:
  - CRUD plus visual map.
  - visual capacity is 288 fiber cells: 2 cables x 12 termination points x 12 fibers.
  - create only inserts `plenums`; it does not implicitly create `pl_slots`.
  - explicit initialize action creates missing `pl_slots` for cable 1/2 and `num=1..12`.
  - missing slots render as incomplete configuration, not free fibers.
  - delete blocked when ports are linked.
- Ports:
  - preserve statuses `Empty`, `Linked`, `Used`, `Xcon` and tolerated legacy values.
  - port operations must be backend-owned and transaction-safe.
- Cables and fibers:
  - cable create generates fibers `1..N`.
  - new fibers start as `Libera`.
  - cable delete only when all fibers are free and unassigned.
  - fiber assignment clears old port links and sets new left/right port links atomically.
  - assignment rejects concurrent double assignment and reports a user-facing conflict.
- Cross Connect:
  - `xcon` plus optional ordered `xcon_hop` is the V1 source of truth.
  - writes mutate only `xcon` and `xcon_hop`.
  - active tab is `stato != 'cessata'`.
  - ceased tab is `stato='cessata'`.
  - `annullato` is terminal but appears in the active-tab query.
  - preserve raw `tipo` product selector values such as `CDL-X*`.
  - approved field labels:
    - `ticket_esteso` -> `Ticket Esteso`
    - `num_ordine` -> `Codice Ordine`
    - `riga_ordine` -> `Serial Number`
  - no inventory side effects from xcon create/update.
- Map validation:
  - map reports reference `crossconnects` for visual indicators. Validate whether those references are active, historical, or reconcilable with `xcon`/`xcon_hop` before using them in UI indicators.

## API Contract

- Plenums:
  - `GET /grappa-dcim/v1/plenums?datacenterId=...`
  - `POST /grappa-dcim/v1/plenums`
  - `GET /grappa-dcim/v1/plenums/{id}`
  - `PATCH /grappa-dcim/v1/plenums/{id}`
  - `DELETE /grappa-dcim/v1/plenums/{id}`
  - `GET /grappa-dcim/v1/plenums/{id}/matrix`
  - `POST /grappa-dcim/v1/plenums/{id}/initialize-matrix`
- Cables/fibers:
  - `GET /grappa-dcim/v1/cables`
  - `POST /grappa-dcim/v1/cables`
  - `GET /grappa-dcim/v1/cables/{id}`
  - `PATCH /grappa-dcim/v1/cables/{id}`
  - `DELETE /grappa-dcim/v1/cables/{id}`
  - `GET /grappa-dcim/v1/cables/{id}/fibers`
  - `PATCH /grappa-dcim/v1/fibers/{id}/assignment`
  - `GET /grappa-dcim/v1/ports?plenumId=...&status=...`
- Cross Connect:
  - `GET /grappa-dcim/v1/xcon?tab=active|ceased&q=...`
  - `POST /grappa-dcim/v1/xcon`
  - `GET /grappa-dcim/v1/xcon/{id}`
  - `PATCH /grappa-dcim/v1/xcon/{id}`
  - `PUT /grappa-dcim/v1/xcon/{id}/hops`
  - `GET /grappa-dcim/v1/xcon/product-options`

## Frontend Plan

- Plenum detail uses a constrained matrix component with stable cell dimensions and responsive scrolling.
- Matrix cells show domain status with accessible labels and tooltips.
- Matrix controls:
  - datacenter selector
  - initialize matrix action when configuration is incomplete
  - status filters
  - selected-cell inspector
- Cable detail shows generated fibers and assignment state.
- Fiber assignment uses a modal/drawer with left/right port selectors and conflict feedback.
- Cross Connect uses:
  - active/ceased segmented control or tabs.
  - table/list plus detail route.
  - detail tabs: `Riepilogo`, `Endpoint`, `Percorso`, `LOA/MMR`, `Storico`.
  - ordered hop editor with add/reorder/remove controls.

## Backend Plan

- Use transactions for matrix initialization, cable create with fiber generation, cable delete, fiber assignment, and xcon hop replace.
- For fiber assignment, lock or re-read affected fiber and ports in the transaction to prevent double assignment.
- Keep xcon write path isolated from inventory mutation code.
- Preserve nullable legacy fields and unknown status/code values.
- Use operation-specific sanitized error codes:
  - `plenum_has_linked_ports`
  - `plenum_matrix_incomplete`
  - `cable_fibers_assigned`
  - `fiber_assignment_conflict`
  - `invalid_xcon_status`

## Verification

- UI review checks:
  - populated plenum matrix state.
  - incomplete matrix state.
  - fiber assignment conflict state.
  - active and ceased xcon tabs.
  - mobile/narrow matrix does not overlap text or controls.
- Runtime / auth checks:
  - Viewer cannot initialize matrix or assign fibers.
  - Operativo can execute allowed mutations.
  - xcon writes do not alter inventory tables.
  - direct deep links refresh successfully.
- Tests:
  - No tests are authorized by this planning artifact.
  - The implementer should ask the human expert before adding transaction tests for matrix initialization, cable fiber generation, fiber assignment conflicts, cable delete blocking, and xcon hop replacement because they protect non-trivial data integrity.
- Manual validation:
  - validate `mmr_slots` naming against actual schema before planning any endpoint around it.
  - validate map-only `crossconnects` references before rendering visual indicators.
  - verify xcon active/ceased semantics exactly.

## Agent Deliverables

- Implementation report: `apps/grappa-dcim/docs/cabling-crossconnects-implementation-report.md`.
- QA report: `apps/grappa-dcim/docs/cabling-crossconnects-qa.md`.
- Report must include:
  - matrix initialization behavior
  - fiber assignment transaction behavior
  - xcon tab semantics
  - map-only crossconnects validation result
  - unresolved source questions

## Exceptions

- The 288-cell plenum matrix is an approved visual workspace exception. It is required by the source behavior and must remain functional, compact, and accessible.
- Cross Connect uses a workflow registry inside `data_workspace` because status tabs and ordered hops are central to the user task.
