# Equipment Compute Storage Run Contract

## Status

- Iteration: 1
- Dependency status: foundation and facilities-layout accepted with `Status: PASS` QA reports.
- Allowed write scope:
  - `backend/internal/grappadcim/equipment*.go`
  - `backend/internal/grappadcim/servers*.go`
  - `backend/internal/grappadcim/storage*.go`
  - `backend/internal/grappadcim/cameras*.go`
  - `backend/internal/grappadcim/credentials*.go`
  - narrowly scoped additions to `backend/internal/grappadcim/handler.go`, `helpers.go`, `types.go`, and shared dependency helpers needed to register and share this slice
  - `apps/grappa-dcim/src/features/equipment/*`
  - `apps/grappa-dcim/src/features/servers/*`
  - `apps/grappa-dcim/src/features/storage/*`
  - `apps/grappa-dcim/src/features/cameras/*`
  - narrowly scoped additions to `apps/grappa-dcim/src/routes.tsx`, API query/type files, and shared app-local styles/components needed by this slice
  - `apps/grappa-dcim/docs/equipment-compute-storage-implementation-report.md`
- Disallowed write scope:
  - Facilities/rack behavior except selector consumption.
  - Cabling, xcon, fiber-ring, topology, artifact, CWDM, TIM GEA, Hive sync, polling, alerting, or first-class `cassetti_ottici` implementation.
  - Repo-wide wiring already completed by foundation unless a build break caused by this slice requires a minimal fix.
  - Automated tests unless the human explicitly approves them.

## Required Reading

- `apps/grappa-dcim/docs/grappa-dcim-spec.md`
- `apps/grappa-dcim/docs/foundation-impl.md`
- `apps/grappa-dcim/docs/foundation-qa.md`
- `apps/grappa-dcim/docs/facilities-layout-impl.md`
- `apps/grappa-dcim/docs/facilities-layout-qa.md`
- `apps/grappa-dcim/docs/equipment-compute-storage-impl.md`
- `apps/grappa-dcim/docs/planning-ui-review.md`
- `docs/UI-UX.md`
- `docs/IMPLEMENTATION-PLANNING.md`
- `docs/IMPLEMENTATION-KNOWLEDGE.md`
- `docs/grappa/GRAPPA.md`
- Schema evidence:
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

## Implementation Target

- Implement the equipment, compute, storage, and cameras slice from `apps/grappa-dcim/docs/equipment-compute-storage-impl.md`.
- Replace foundation stubs for `Apparati`, `Server`, `Storage`, and `Telecamere` with working registry/detail workspaces.
- Implement backend endpoints listed in the equipment plan where source evidence is sufficient.
- Preserve Viewer as read-only and non-secret. Viewer must not receive credential values or call credential endpoints.
- Operativo may mutate approved records and may view/update validated credential fields.
- Treat `pwd_utenza_cliente` as sensitive. Do not enable writes for it unless the implementation report documents concrete validation of storage/encryption behavior. If not validated, read it only through the Operativo credential endpoint when safe, and record the write limitation.
- Never log credential values.
- Camera V1 has no delete action or endpoint.
- Storage rows with `status='Chiuso'` must render read-only and archive must set `status='Chiuso'` plus `closed_at`.
- Apparato create with ports must generate `nic` rows transactionally. Apparato updates must not regenerate NICs.
- Do not add tests; record recommended tests in the implementation report.

## Verification Required

- Commands:
  - `pnpm --filter mrsmith-grappa-dcim build`
  - `go build ./cmd/server` from `backend`
- Manual/browser checks:
  - If a suitable dev server is already running, reuse it; otherwise do not start a second server.
  - Record whether populated/empty/error/credential states were checked in browser. If not checked, explain why.
- UI review states:
  - apparati list and detail with NIC tab
  - server detail as Viewer with credentials hidden
  - server detail as Operativo with credential action available
  - storage closed row read-only state
  - camera create/update form and IP validation
  - mobile detail tabs without overflow, checked by browser when feasible or recorded as a residual risk

## Reporting Required

Write `apps/grappa-dcim/docs/equipment-compute-storage-implementation-report.md` with:
- files changed
- behavior implemented
- endpoint list implemented
- route list implemented
- fields hidden from Viewer
- credential compatibility validation outcome
- generated NIC behavior
- storage archive behavior
- camera validation behavior
- contracts preserved
- commands run and outputs summarized
- manual/browser checks run or skipped with reason
- unresolved source validations
- deviations from plan
