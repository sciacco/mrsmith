# Equipment Compute Storage Fix 1 Report

## Files Changed

- `backend/internal/grappadcim/storage.go`
- `backend/internal/grappadcim/servers.go`
- `apps/grappa-dcim/docs/equipment-compute-storage-fix-1-report.md`

## Fixes Implemented

- Storage create now rejects caller-provided `status='Chiuso'` with `storage_close_requires_archive`.
- Storage update now rejects PATCH attempts to set `status='Chiuso'` with `storage_close_requires_archive`.
- Storage archive remains the only supported closure path and still sets `status='Chiuso'` plus `closed_at=COALESCE(closed_at, NOW())`.
- Storage rows already closed remain read-only through the existing `storage_closed_read_only` update guard.
- Physical server PATCH now detects sync-relevant updates to `kind`, `equipmentId`, `customerId`, `orderCode`, or `serialNumber`, then reloads the effective post-update `tipologia`, `apparato_id`, `id_anagrafica`, `codice_ordine`, and `serialnumber` inside the update transaction.
- Physical server PATCH syncs the linked `apparato` from those effective post-update values, so clients do not need to resend unchanged `kind` or `equipmentId`.
- Storage hard delete has been deferred safely: `handleDeleteStorage` now returns `501 storage_delete_deferred` and no longer executes `DELETE FROM storage`.

## Commands Run And Summarized Outputs

- `gofmt -w backend/internal/grappadcim`
  - Completed with no output.
- `go build ./cmd/server` from `backend`
  - Passed with no output.
- `pnpm --filter mrsmith-grappa-dcim build`
  - Passed. TypeScript built and Vite produced `dist/index.html`, CSS, and JS assets.
- `rg -n "func \\(h \\*Handler\\) handleDeleteStorage|DELETE /grappa-dcim/v1/storage|DELETE FROM storage|storage_delete_deferred|storage_close_requires_archive" backend/internal/grappadcim`
  - Confirmed the storage delete route points to a deferred handler, no `DELETE FROM storage` remains, and the new storage closure validation is present.

## Unresolved Questions

- Storage hard-delete dependency evidence remains unresolved, so hard delete is still deferred.
- Live SQL/API behavior against populated Grappa data was not exercised.

## Deviations From Remediation Instructions

- The remediation preferred removing the registered storage delete endpoint, but `handler.go` is outside the remediation write scope. The endpoint registration was therefore left in place and the storage delete handler was made non-mutating/deferred inside `storage.go`.
- No automated tests were added, per instruction.
- Credential/password behavior was not changed.
