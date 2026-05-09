# Equipment Compute Storage Remediation 1

## Status

- Owning slice: `equipment-compute-storage`
- Source QA: `apps/grappa-dcim/docs/equipment-compute-storage-qa.md`
- QA status: `Status: FAIL`
- Fix report required: `apps/grappa-dcim/docs/equipment-compute-storage-fix-1-report.md`
- Allowed write scope:
  - `backend/internal/grappadcim/storage*.go`
  - `backend/internal/grappadcim/servers*.go`
  - `backend/internal/grappadcim/dependencies.go` only if shared dependency helper changes are needed
  - `apps/grappa-dcim/docs/equipment-compute-storage-fix-1-report.md`
- Disallowed write scope:
  - Frontend changes unless needed to compile against backend contract corrections.
  - Credential/password behavior changes, tests, new features, repo wiring, or other slice files.

## Required Reading

- `apps/grappa-dcim/docs/equipment-compute-storage-run.md`
- `apps/grappa-dcim/docs/equipment-compute-storage-implementation-report.md`
- `apps/grappa-dcim/docs/equipment-compute-storage-qa.md`
- `apps/grappa-dcim/docs/equipment-compute-storage-impl.md`
- `apps/grappa-dcim/docs/grappa-dcim-spec.md`
- Schema evidence:
  - `docs/grappa/grappa_storage.json`
  - `docs/grappa/grappa_server.json`
  - `docs/grappa/grappa_apparato.json`

## Findings To Fix

1. High - Storage can be closed outside the archive path, bypassing `closed_at`.
   - QA evidence: `handleCreateStorage` accepts caller-provided `status`, including `Chiuso`, without setting `closed_at`; `storagePatch` accepts `status`, and `handleUpdateStorage` applies it to open rows without setting `closed_at`.
   - Required correction:
     - Reject `status='Chiuso'` in storage create and storage update, or route close transitions through the archive handler.
     - Keep archive as the supported closure path that always sets `status='Chiuso'` and `closed_at=COALESCE(closed_at, NOW())`.
     - Keep reopen/edit unsupported unless explicitly approved later.

2. High - Physical server update sync depends on clients resending unchanged PATCH fields.
   - QA evidence: `handleUpdateServer` calls `syncPhysicalServerEquipmentTx` with only fields present in the PATCH body. If an already-physical server PATCH changes `customerId`, `orderCode`, or `serialNumber` without resending `kind` and `equipmentId`, the linked apparato sync is skipped.
   - Required correction:
     - Inside the update transaction, when sync-relevant fields are present, load the effective post-update `tipologia`, `apparato_id`, `id_anagrafica`, `codice_ordine`, and `serialnumber`.
     - Sync `id_anagrafica`, `codice_ordine`, and `serialnumber` to linked `apparato` for physical servers.
     - Do not rely on clients resending unchanged fields.

3. Medium - Storage hard delete is registered without dependency validation or transaction handling.
   - QA evidence: `DELETE /grappa-dcim/v1/storage/{id}` performs direct `DELETE FROM storage WHERE id = ?` after double confirmation only.
   - Required correction:
     - Either remove/defer the delete endpoint for V1, or implement a transactional delete path with known dependency validation.
     - Given current unresolved source evidence, prefer removing/defering the registered delete endpoint so V1 exposes archive but not unsafe hard delete.
     - If retaining the endpoint, it must return controlled dependency summaries and run in a transaction.

## Verification Required

- `gofmt -w backend/internal/grappadcim`
- `go build ./cmd/server` from `backend`
- `pnpm --filter mrsmith-grappa-dcim build`

## Reporting Required

Write `apps/grappa-dcim/docs/equipment-compute-storage-fix-1-report.md` with:
- files changed
- fixes implemented
- commands run and summarized outputs
- unresolved questions
- deviations from remediation instructions
