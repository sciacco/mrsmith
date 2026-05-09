# Equipment Compute Storage QA

Status: PASS

## Source Docs Checked

- `apps/grappa-dcim/docs/equipment-compute-storage-run.md`
- `apps/grappa-dcim/docs/equipment-compute-storage-implementation-report.md`
- `apps/grappa-dcim/docs/equipment-compute-storage-qa.md`
- `apps/grappa-dcim/docs/equipment-compute-storage-remediation-1.md`
- `apps/grappa-dcim/docs/equipment-compute-storage-fix-1-report.md`
- `apps/grappa-dcim/docs/equipment-compute-storage-impl.md`
- `apps/grappa-dcim/docs/grappa-dcim-spec.md`
- `apps/grappa-dcim/docs/planning-ui-review.md`
- `docs/UI-UX.md`
- `docs/IMPLEMENTATION-PLANNING.md`
- `docs/IMPLEMENTATION-KNOWLEDGE.md`
- `docs/grappa/GRAPPA.md`
- `.agents/skills/portal-miniapp-ui-review/SKILL.md`
- `.agents/skills/portal-miniapp-ui-review/references/blocking-gates.md`
- `.agents/skills/portal-miniapp-ui-review/references/evidence-checklist.md`
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

## Changed Files Inspected

- `backend/internal/grappadcim/handler.go`
- `backend/internal/grappadcim/helpers.go`
- `backend/internal/grappadcim/types.go`
- `backend/internal/grappadcim/equipment.go`
- `backend/internal/grappadcim/equipment_types.go`
- `backend/internal/grappadcim/servers.go`
- `backend/internal/grappadcim/servers_types.go`
- `backend/internal/grappadcim/credentials.go`
- `backend/internal/grappadcim/storage.go`
- `backend/internal/grappadcim/storage_types.go`
- `backend/internal/grappadcim/cameras.go`
- `backend/internal/grappadcim/cameras_types.go`
- `apps/grappa-dcim/src/api/queries.ts`
- `apps/grappa-dcim/src/api/types.ts`
- `apps/grappa-dcim/src/routes.tsx`
- `apps/grappa-dcim/src/features/equipment/EquipmentPages.tsx`
- `apps/grappa-dcim/src/features/equipment/assetPageUtils.tsx`
- `apps/grappa-dcim/src/features/servers/ServerPages.tsx`
- `apps/grappa-dcim/src/features/storage/StoragePages.tsx`
- `apps/grappa-dcim/src/features/cameras/CameraPages.tsx`

Current note: the Grappa DCIM app and `backend/internal/grappadcim/` package remain untracked in this worktree, so `git diff --name-only` does not report the slice implementation files. The files above were inspected directly from the untracked tree.

## Product Behavior Findings

- PASS. No blocking product behavior findings remain.
- PASS. Remediation finding 1 is resolved: storage create and update reject `status='Chiuso'` with `storage_close_requires_archive`, while `handleArchiveStorage` remains the only close path and sets `status='Chiuso'` plus `closed_at=COALESCE(closed_at, NOW())`.
  - Evidence: `backend/internal/grappadcim/storage.go:84`, `backend/internal/grappadcim/storage.go:170`, `backend/internal/grappadcim/storage.go:232`, `backend/internal/grappadcim/storage.go:273`.
- PASS. Remediation finding 2 is resolved: server PATCH detects sync-relevant fields, reloads effective post-update `tipologia`, `apparato_id`, `id_anagrafica`, `codice_ordine`, and `serialnumber` inside the transaction, then syncs the linked apparato only for physical servers.
  - Evidence: `backend/internal/grappadcim/servers.go:149`, `backend/internal/grappadcim/servers.go:160`, `backend/internal/grappadcim/servers.go:387`, `backend/internal/grappadcim/servers.go:391`, `backend/internal/grappadcim/servers.go:400`.
- PASS. Remediation finding 3 is resolved for V1 safety: the storage delete route is still registered, but `handleDeleteStorage` now returns `501 storage_delete_deferred` and no longer executes `DELETE FROM storage`.
  - Evidence: `backend/internal/grappadcim/handler.go:116`, `backend/internal/grappadcim/storage.go:183`; focused search found no `DELETE FROM storage` match.
- PASS. Apparato create still generates NIC rows in the create transaction, and apparato update does not regenerate NIC rows.
- PASS. Camera create/update keeps app-level required fields and frontend/backend IP validation; no camera delete endpoint or UI action was found.
- PASS. Storage closed rows still return `readOnly: true` and the UI hides edit/archive controls when `readOnly` is true.

## Repo/Runtime Findings

- PASS. Required implementation and fix reports exist and summarize files changed, behavior, endpoints/routes, remediation actions, verification commands, unresolved questions, and deviations.
- PASS. Route/base/API/static wiring remains consistent with prior foundation QA: Vite base `/apps/grappa-dcim/`, browser API calls under `/api/grappa-dcim/v1/...`, backend mux routes under `/grappa-dcim/v1/...`, and Vite port `5191`.
- PASS. Equipment/server/storage/camera routes are registered in `apps/grappa-dcim/src/routes.tsx`; backend routes are registered in `backend/internal/grappadcim/handler.go`.
- PASS. No automated tests were added without approval.

## Data/Auth/Credential Findings

- PASS. Viewer cannot call credential endpoints through frontend query gating: `useServerCredentials` is enabled only when `meta.canViewCredentials` is true.
- PASS. Backend credential endpoints are registered behind `RequireOperativo`.
- PASS. Read-safe server list/detail DTOs do not select or return `pwd_ilo`, `root_administrator_password`, `pwd_utenza_cliente`, or `pwd_utenza_cdlan`.
- PASS. Operativo credential endpoint returns non-password access fields plus stored/not-stored flags. It does not return password values and password writes remain disabled.
- PASS. `pwd_utenza_cliente` writes remain disabled.
- PASS. Schema evidence contains the storage, server, apparato, NIC, server child, credential, and camera columns used by this slice.

## UI Findings

- Review phase: post-implementation code-first UI gate for the equipment-compute-storage slice after remediation 1.
- Evidence package: approved plan, `master_detail_crud` archetype, explicit exceptions, comparable repo screens, implementation files, routes, and UI gate rules were available. Screenshots were not available because no suitable dev server was already running.
- PASS. Apparati, Server, Storage, and Telecamere screens use compact workspace headers, toolbar filters/search, tables, detail panels, tabs/modals, and plain empty/error states. No launcher, hero, dashboard, or Matrix-style shell drift was found.
- PASS. Viewer mutation controls are hidden through `meta.canOperate`; backend still enforces Operativo on mutation routes.
- PASS. Viewer credential state is business-facing Italian copy and does not show raw backend/auth text.
- PASS. Operativo credential action is present but limited to non-password access fields.
- PASS. Closed storage rows render read-only in frontend code: row actions and detail edit/archive actions are hidden when `item.readOnly` is true.
- PASS. Camera UI exposes create/update only; no delete action was found.
- PASS. No raw user-facing `Unauthorized`, HTTP status, handler, datasource, widget, JSON, SQL/table, password column, or encryption implementation copy was found in the reviewed slice UI.
- Residual UI gap: populated/default/error/credential states and mobile tab overflow were reviewed from code only, not browser screenshots.

## Verification Commands Run

- `gofmt -l backend/internal/grappadcim`
  - PASS. No files listed.
- `go build ./cmd/server` from `backend`
  - PASS. Backend build completed successfully with no output.
- `pnpm --filter mrsmith-grappa-dcim build`
  - PASS. TypeScript and Vite production build completed successfully.
- `rg -n "DELETE FROM storage|storage_delete_deferred|storage_close_requires_archive|status = 'Chiuso'|closed_at = COALESCE|syncEffectivePhysicalServerEquipmentTx|serverPatchTouchesEquipmentSync|SELECT tipologia, apparato_id, id_anagrafica, codice_ordine, serialnumber" backend/internal/grappadcim`
  - PASS. Confirmed non-mutating storage delete deferral, storage close validation/archive behavior, and effective server sync reload.
- `rg --files apps/grappa-dcim backend/internal/grappadcim | rg '(_test\.go|\.test\.|\.spec\.)'`
  - PASS. No test files found.
- `rg -n "DELETE /grappa-dcim/v1/cameras|handleDeleteCamera|deleteCamera|api\.delete<.*cameras|/cameras/.+delete|Elimina|Cancella" apps/grappa-dcim/src backend/internal/grappadcim`
  - PASS. No camera delete endpoint/action matches found.
- `rg` checks for credential password fields, Viewer/Operativo credential gating, raw UI copy terms, storage closed state, and route/API registrations.
  - PASS.
- Schema evidence checks with `jq`/`sed` for listed Grappa tables.
  - PASS. Referenced columns used by the slice were present in the listed schema evidence.
- `lsof -nP -iTCP:5191 -sTCP:LISTEN`
  - No listener found.
- `lsof -nP -iTCP:8080 -sTCP:LISTEN`
  - No listener found.
- `lsof -nP -iTCP:5173 -sTCP:LISTEN`
  - No listener found.

## Manual/Browser Checks

- Browser checks were not run. No suitable Grappa DCIM Vite/backend dev server was already listening on `5191`, `8080`, or `5173`, and the local instruction requires reusing a suitable existing server before browser checks rather than starting another server for this gate.
- Populated, empty, error, Viewer credential-hidden, Operativo credential-action, storage closed read-only, camera validation, and mobile tab states were checked from code only.
- Live SQL/API behavior against a populated Grappa database was not exercised.

## Residual Risks

- SQL behavior against live Grappa data remains unproven.
- Browser visual QA remains outstanding for populated, empty, error, credential, destructive-confirm, and narrow viewport states.
- Exact active production default for `storage.status` remains unvalidated; implementation currently defaults to `Attivo`.
- Legacy `k_crypt` compatibility remains unresolved, but password values/writes are disabled, so this is not currently leaking secrets.
- The `storage` schema uses a composite primary key `(id, cli_fatturazione_id, apparato_id_apparato)` while routes address storage by `id` only. The implementation plan uses `{id}` routes, but this should be validated against live data uniqueness before final production signoff.
