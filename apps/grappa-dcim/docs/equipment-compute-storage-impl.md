# Grappa DCIM Equipment, Compute, Storage, and Cameras Implementation Plan

## Slice Contract

- Slice: `equipment-compute-storage`
- Purpose: implement device inventory, server details, storage allocations, and camera inventory.
- Approved source: `apps/grappa-dcim/docs/grappa-dcim-spec.md`.
- Depends on: `foundation` and enough `facilities-layout` API surface for rack/datacenter selectors.
- In-scope source surfaces:
  - `apparato`
  - `server`
  - `storage`
  - `dcimadmin-cam`
- Primary write ownership:
  - `backend/internal/grappadcim/equipment*.go`
  - `backend/internal/grappadcim/servers*.go`
  - `backend/internal/grappadcim/storage*.go`
  - `backend/internal/grappadcim/cameras*.go`
  - `backend/internal/grappadcim/credentials*.go`
  - `apps/grappa-dcim/src/features/equipment/*`
  - `apps/grappa-dcim/src/features/servers/*`
  - `apps/grappa-dcim/src/features/storage/*`
  - `apps/grappa-dcim/src/features/cameras/*`
- Required schema evidence for implementation validation:
  - `docs/grappa/grappa_apparato.json`
  - `docs/grappa/grappa_nic.json`
  - `docs/grappa/grappa_server.json`
  - `docs/grappa/grappa_server_schede.json`
  - `docs/grappa/grappa_server_applicazioni.json`
  - `docs/grappa/grappa_server_servizi.json`
  - `docs/grappa/grappa_server_porte.json`
  - `docs/grappa/grappa_storage.json`
  - `docs/grappa/grappa_cli_contatti_escalation.json`
  - `docs/grappa/grappa_cams.json`

## Comparable Apps Audit

- Reference 1: `apps/manutenzioni/src/pages/MaintenanceDetailPage.tsx`.
- Reference 2: `apps/fornitori/src/views.tsx`.
- Additional reference: `apps/rda/src/pages/PoDetailPage.tsx`.
- Reused patterns:
  - detail shell with tabs for aggregate records.
  - read/edit panels that keep the primary data visible while edits happen in modal/drawer forms.
  - role-based action visibility and guarded mutations.
  - compact tables with row actions and empty/error states.
- Rejected patterns:
  - large dashboard tiles for device totals.
  - technical labels such as "encrypted field", "child table", or "side effect" in the UI.
  - automatic background polling or alerting around monitoring fields.

## Archetype Choice

- Selected archetype: `master_detail_crud`.
- Why it fits: apparati, servers, storage rows, and cameras are registries with create/edit/detail flows. Server details add tabs, but the primary task remains entity management.
- Required states:
  - list loading, empty, filtered-empty, error
  - detail loading, not found, read-only closed storage
  - credential hidden state for Viewer
  - credential editable state for Operativo
  - archive confirmation and blocked delete
  - mutation conflict or stale detail reload

## User Copy Rules

- Allowed copy style: direct operational Italian copy.
- Approved user labels:
  - `Apparati`
  - `Server`
  - `Storage`
  - `Telecamere`
  - `Credenziali`
  - `Archivia storage`
  - `Cessa apparato`
  - `Porte`
  - `Schede`
  - `Applicazioni`
  - `Servizi`
- Forbidden copy risks:
  - do not expose `k_crypt`, plaintext, database column names, or encryption implementation to users.
  - do not say a save "syncs apparato" in UI copy; say the linked asset will be updated only if a user-facing warning is needed.
  - do not show `id.asc`, handler names, or legacy route names.
- Metrics allowed: only row counts in current tabs such as number of NICs, server ports, applications, or storage allocations.

## Repo-Fit

- Route/base path: under `/apps/grappa-dcim/`.
- Planned frontend routes:
  - `/apparati`
  - `/apparati/:apparatoId`
  - `/server`
  - `/server/:serverId`
  - `/storage`
  - `/storage/:storageId`
  - `/telecamere`
- API prefix:
  - `/api/grappa-dcim/v1/equipment/...`
  - `/api/grappa-dcim/v1/servers/...`
  - `/api/grappa-dcim/v1/storage/...`
  - `/api/grappa-dcim/v1/cameras/...`
- Access role:
  - Viewer can read approved non-secret inventory and non-sensitive server details.
  - Viewer cannot view credential values or call credential endpoints.
  - Operativo can write approved records and view/update server credentials.
- Dev port / proxy notes: inherited from foundation.
- Static hosting / deployment notes: inherited from foundation.

## Domain Rules

- Apparati:
  - list, view, create, update, cease.
  - device `type` values remain legacy/free-text with DB-derived picklist.
  - unknown stored type values must remain visible and round-trip safe.
  - create with ports generates sequential `nic` rows.
  - only proven types get side effects.
  - no automatic NIC regeneration on update.
  - cessation cascades NICs.
  - monitoring fields are preserved but do not trigger polling or alerts in V1.
- Servers:
  - support physical and virtual server create/update paths.
  - physical server syncs selected customer/order/serial fields to linked `apparato`.
  - preserve aggregate child views for `server_schede`, `server_applicazioni`, `server_servizi`, and `server_porte`.
  - credentials:
    - Operativo can view/update sensitive credential fields.
    - Viewer cannot receive sensitive credential values from API responses.
    - proven encrypted fields must remain compatible with legacy `k_crypt`.
    - `pwd_utenza_cliente` behavior must be validated before write support; treat it as sensitive even while validation is open.
    - omitted credential field means unchanged.
    - explicit empty value means clear only where the approved spec allows it.
    - never log credential values.
- Storage:
  - list, view, create, update, archive.
  - archive is the preferred closure and sets `status='Chiuso'` plus `closed_at`.
  - closed rows are read-only except for allowed reopen/edit behavior if later approved; V1 default is read-only.
  - hard delete only when no known operational/billing dependencies exist.
  - no automatic billing side effect in V1.
  - verify most common active `storage.status` before choosing default for new records.
- Cameras:
  - list, create, update only.
  - no delete in V1 unless the human expert later approves it.
  - required app-level fields: `code`, `model`, `brand`, `position`.
  - optional `ipaddr`, `status`, and `serial`.
  - no uniqueness enforcement for `code`, `ipaddr`, or `serial`.
  - validate `ipaddr` as an IP when provided.

## API Contract

- Apparati:
  - `GET /grappa-dcim/v1/equipment`
  - `POST /grappa-dcim/v1/equipment`
  - `GET /grappa-dcim/v1/equipment/{id}`
  - `PATCH /grappa-dcim/v1/equipment/{id}`
  - `POST /grappa-dcim/v1/equipment/{id}/cease`
  - `GET /grappa-dcim/v1/equipment/{id}/nics`
  - `GET /grappa-dcim/v1/equipment/type-options`
- Servers:
  - `GET /grappa-dcim/v1/servers`
  - `POST /grappa-dcim/v1/servers`
  - `GET /grappa-dcim/v1/servers/{id}`
  - `PATCH /grappa-dcim/v1/servers/{id}`
  - `GET /grappa-dcim/v1/servers/{id}/children`
  - `GET /grappa-dcim/v1/servers/{id}/credentials`
  - `PATCH /grappa-dcim/v1/servers/{id}/credentials`
- Storage:
  - `GET /grappa-dcim/v1/storage`
  - `POST /grappa-dcim/v1/storage`
  - `GET /grappa-dcim/v1/storage/{id}`
  - `PATCH /grappa-dcim/v1/storage/{id}`
  - `POST /grappa-dcim/v1/storage/{id}/archive`
  - `DELETE /grappa-dcim/v1/storage/{id}`
- Cameras:
  - `GET /grappa-dcim/v1/cameras`
  - `POST /grappa-dcim/v1/cameras`
  - `GET /grappa-dcim/v1/cameras/{id}`
  - `PATCH /grappa-dcim/v1/cameras/{id}`

## Frontend Plan

- Use registry pages for the four main surfaces with search/filter toolbar and detail routes.
- Apparato detail tabs:
  - `Riepilogo`
  - `NIC`
  - `Rack`
  - `Storico`
- Server detail tabs:
  - `Riepilogo`
  - `Hardware`
  - `Accessi`
  - `Schede`
  - `Applicazioni`
  - `Servizi`
  - `Porte`
- Storage detail:
  - show archive state clearly.
  - disable editing for `Chiuso` rows.
- Camera registry:
  - simple table with create/edit modal.
  - no delete action.
- Credential UI:
  - hidden/locked state for Viewer.
  - explicit reveal and edit actions for Operativo.
  - do not show placeholder values that imply a password has been loaded when it has not.

## Backend Plan

- Use separate response DTOs for read-safe detail and Operativo credential detail.
- Centralize credential encryption/decryption compatibility so field handling cannot drift across handlers.
- Use transactions for apparato create with NIC generation, apparato cease with NIC cascade, physical server update with apparato sync, and storage archive/delete.
- Validate IP addresses for cameras in backend as well as frontend.
- Preserve legacy text encodings and nullable fields safely.
- Use operation-specific logs and never log sensitive credential values.

## Verification

- UI review checks:
  - apparati list and detail with NIC tab.
  - server detail as Viewer with credentials hidden.
  - server detail as Operativo with credential action available.
  - storage closed row read-only state.
  - camera create/update form and validation.
  - mobile detail tabs without overflow.
- Runtime / auth checks:
  - Viewer cannot call server credential endpoints.
  - Viewer cannot call mutations.
  - Operativo can update allowed records.
  - stale detail reload after archive/cease works.
- Tests:
  - No tests are authorized by this planning artifact.
  - The implementer should ask the human expert before adding tests for credential role boundaries, encryption compatibility, apparato NIC generation, storage archive, and camera IP validation because they protect non-trivial business and security behavior.
- Manual validation:
  - verify physical server sync fields against live/source behavior.
  - validate `pwd_utenza_cliente` storage/encryption before enabling writes.
  - verify active storage status default from production data before create default is final.

## Agent Deliverables

- Implementation report: `apps/grappa-dcim/docs/equipment-compute-storage-implementation-report.md`.
- QA report: `apps/grappa-dcim/docs/equipment-compute-storage-qa.md`.
- Report must include:
  - fields hidden from Viewer
  - credential compatibility validation outcome
  - generated NIC behavior
  - storage archive behavior
  - camera validation behavior

## Exceptions

- Server detail uses a tabbed aggregate inside a `master_detail_crud` slice because the source server entity has multiple child detail views. The exception is justified by user need to inspect one server without leaving the server workspace.
- Credential reveal/edit controls are an exception to simple CRUD because the spec requires Operativo access while protecting Viewer reads.
