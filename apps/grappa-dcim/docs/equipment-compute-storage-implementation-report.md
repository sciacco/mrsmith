# Equipment Compute Storage Implementation Report

## Files Changed

- Backend:
  - `backend/internal/grappadcim/handler.go`
  - `backend/internal/grappadcim/equipment.go`
  - `backend/internal/grappadcim/equipment_types.go`
  - `backend/internal/grappadcim/servers.go`
  - `backend/internal/grappadcim/servers_types.go`
  - `backend/internal/grappadcim/credentials.go`
  - `backend/internal/grappadcim/storage.go`
  - `backend/internal/grappadcim/storage_types.go`
  - `backend/internal/grappadcim/cameras.go`
  - `backend/internal/grappadcim/cameras_types.go`
- Frontend:
  - `apps/grappa-dcim/src/api/queries.ts`
  - `apps/grappa-dcim/src/api/types.ts`
  - `apps/grappa-dcim/src/routes.tsx`
  - `apps/grappa-dcim/src/features/equipment/EquipmentPages.tsx`
  - `apps/grappa-dcim/src/features/equipment/assetPageUtils.tsx`
  - `apps/grappa-dcim/src/features/servers/ServerPages.tsx`
  - `apps/grappa-dcim/src/features/storage/StoragePages.tsx`
  - `apps/grappa-dcim/src/features/cameras/CameraPages.tsx`

## Behavior Implemented

- Replaced the `Apparati`, `Server`, `Storage`, and `Telecamere` stubs with registry/detail workspaces.
- Added read/write backend handlers for apparati, servers, storage, and cameras.
- Added apparato detail tabs for `Riepilogo`, `NIC`, `Rack`, and `Storico`.
- Added server detail tabs for `Riepilogo`, `Hardware`, `Accessi`, `Schede`, `Applicazioni`, `Servizi`, and `Porte`.
- Added storage detail with closed/read-only state and archive action.
- Added camera registry and create/update modal with frontend and backend IP validation.
- Kept mutation controls hidden unless `meta.canOperate` is true.

## Endpoint List Implemented

- `GET /grappa-dcim/v1/equipment`
- `POST /grappa-dcim/v1/equipment`
- `GET /grappa-dcim/v1/equipment/type-options`
- `GET /grappa-dcim/v1/equipment/{id}`
- `PATCH /grappa-dcim/v1/equipment/{id}`
- `POST /grappa-dcim/v1/equipment/{id}/cease`
- `GET /grappa-dcim/v1/equipment/{id}/nics`
- `GET /grappa-dcim/v1/servers`
- `POST /grappa-dcim/v1/servers`
- `GET /grappa-dcim/v1/servers/{id}`
- `PATCH /grappa-dcim/v1/servers/{id}`
- `GET /grappa-dcim/v1/servers/{id}/children`
- `GET /grappa-dcim/v1/servers/{id}/credentials`
- `PATCH /grappa-dcim/v1/servers/{id}/credentials`
- `GET /grappa-dcim/v1/storage`
- `POST /grappa-dcim/v1/storage`
- `GET /grappa-dcim/v1/storage/{id}`
- `PATCH /grappa-dcim/v1/storage/{id}`
- `POST /grappa-dcim/v1/storage/{id}/archive`
- `DELETE /grappa-dcim/v1/storage/{id}`
- `GET /grappa-dcim/v1/cameras`
- `POST /grappa-dcim/v1/cameras`
- `GET /grappa-dcim/v1/cameras/{id}`
- `PATCH /grappa-dcim/v1/cameras/{id}`

Credential endpoints are registered behind Operativo-only protection. Viewer cannot call them through the frontend because the credentials query is disabled unless `canViewCredentials` is true.

## Route List Implemented

- `/apparati`
- `/apparati/:apparatoId`
- `/server`
- `/server/:serverId`
- `/storage`
- `/storage/:storageId`
- `/telecamere`

## Fields Hidden From Viewer

- Viewer does not receive values from `GET /servers/{id}/credentials`.
- The read-safe server list/detail DTO excludes credential password columns:
  - `pwd_ilo`
  - `root_administrator_password`
  - `pwd_utenza_cliente`
  - `pwd_utenza_cdlan`
- The Viewer UI shows a locked `Credenziali` state and does not call credential endpoints.

## Credential Compatibility Validation Outcome

- Source docs validate encryption for `pwd_ilo`, `root_administrator_password`, and `pwd_utenza_cdlan` with legacy `k_crypt`.
- This repo does not expose a `k_crypt` key/config contract or a compatible encrypt/decrypt helper.
- Password value reveal and password writes were therefore not enabled.
- The Operativo credential endpoint returns only non-password access fields plus stored/not-stored flags for password-bearing fields.
- `pwd_utenza_cliente` was treated as sensitive. It is not returned as a value and is not writable. Only its stored/not-stored state is exposed to Operativo.

## Generated NIC Behavior

- Apparato create runs in a transaction.
- When `portCount > 0`, the backend inserts sequential `nic` rows in the same transaction.
- Generated NIC rows use `PortName + index` or `Porta + index` when no port name is supplied.
- Generated NIC rows preserve customer, type, layer, and status from the apparato create payload where provided.
- Apparato update does not regenerate NIC rows.
- Apparato cease sets `apparato.stato='Cessato'`, preserves existing `data_cessazione` when already present, and cascades `nic.stato='Cessato'`.

## Storage Archive Behavior

- Storage archive requires the double-confirmation request body.
- Archive sets `status='Chiuso'` and `closed_at=COALESCE(closed_at, NOW())`.
- Storage rows with `status='Chiuso'` return `readOnly: true`.
- Closed storage rows do not render edit/archive actions in the UI.
- Backend PATCH rejects closed rows with `storage_closed_read_only`.
- The create/edit UI does not offer `Chiuso`; closure goes through archive.

## Camera Validation Behavior

- Camera create requires `code`, `model`, `brand`, and `position`.
- Optional `ipaddr` is validated in both frontend and backend.
- No uniqueness checks were added for `code`, `ipaddr`, or `serial`.
- No camera delete endpoint or UI action was implemented.

## Contracts Preserved

- Viewer is read-only and non-secret.
- Operativo-only routes guard all mutations and server credential endpoints.
- Credential values are never logged by the new handlers.
- No cabling/xcon, fiber topology/artifacts, CWDM, TIM GEA, Hive sync, polling, alerting, or first-class `cassetti_ottici` UI was implemented.
- Camera V1 has no delete action or endpoint.
- Unknown/free-text apparato type values remain visible and editable through the DB-derived type datalist.
- Physical server create/update attempts to sync selected customer/order/serial fields to the linked apparato when the request identifies a physical server and linked apparato.

## Commands Run And Summarized Outputs

- Required/source reading:
  - Read the slice run contract, implementation plan, foundation/facilities reports, planning UI review, UI/UX docs, implementation planning/knowledge docs, and Grappa schema index.
  - Read schema evidence with `jq` for `apparato`, `nic`, `server`, server child tables, `storage`, `cli_contatti_escalation`, and `cams`.
- `gofmt -w backend/internal/grappadcim/equipment*.go backend/internal/grappadcim/servers*.go backend/internal/grappadcim/storage*.go backend/internal/grappadcim/cameras*.go backend/internal/grappadcim/credentials*.go backend/internal/grappadcim/handler.go`
  - Completed with no output.
- `pnpm --filter mrsmith-grappa-dcim build`
  - First run failed on one unused import.
  - Final run passed: TypeScript built and Vite produced `dist/index.html`, CSS, and JS assets.
- `go build ./cmd/server` from `backend`
  - Passed with no output.
- `rg --files apps/grappa-dcim backend/internal/grappadcim | rg '(_test\.go|\.test\.|\.spec\.)'`
  - No matches; no automated tests were added.
- `rg "DELETE /grappa-dcim/v1/cameras|handleDeleteCamera|deleteCamera|Elimina" ...`
  - No matches in camera/API/backend slice files.
- `lsof -nP -iTCP:5191 -sTCP:LISTEN`, `lsof -nP -iTCP:8080 -sTCP:LISTEN`, `lsof -nP -iTCP:5173 -sTCP:LISTEN`
  - No listeners found.

## Manual/Browser Checks

- Browser checks were not run because no suitable Grappa DCIM Vite/backend dev server was already listening on `5191`, `8080`, or `5173`, and local instructions require reusing a suitable server before browser checks.
- Code-level checks covered:
  - apparati list/detail and NIC tab wiring
  - Viewer credential-hidden state
  - Operativo credential query/action gating
  - storage closed read-only state
  - camera create/update form and IP validation
  - tab rows use horizontal overflow from existing workspace styles for narrow layouts

## Unresolved Source Validations

- Live SQL behavior against a populated Grappa database was not exercised.
- Legacy `k_crypt` encryption/decryption compatibility is unresolved because no key/config/helper contract exists in this repo.
- `pwd_utenza_cliente` storage/encryption remains unvalidated; writes are intentionally disabled.
- Most common production `storage.status` value was not sampled; V1 create currently defaults to `Attivo`.
- Storage hard-delete dependency checks remain conservative only by double confirmation; no additional source evidence for operational/billing dependency tables was validated in this slice.
- Exact legacy NIC label format was not validated beyond sequential generation from available `numero_porte`, `nome_porte`, `tipo_porte`, and `layer_porte` evidence.

## Deviations From Plan

- Password value reveal/update was not implemented because credential compatibility could not be concretely validated.
- `pwd_utenza_cliente` write support was not implemented.
- Server credential UI edits only non-password access fields.
- Browser/mobile screenshots were not captured because no existing dev server was available.
- No automated tests were added, per the run contract.
