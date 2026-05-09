# Fiber Topology Artifacts Remediation 1

## Status

- Owning slice: `fiber-topology-artifacts`
- Source QA: `apps/grappa-dcim/docs/fiber-topology-artifacts-qa.md`
- Required fix report: `apps/grappa-dcim/docs/fiber-topology-artifacts-fix-1-report.md`

## Blocking Findings

`apps/grappa-dcim/docs/fiber-topology-artifacts-qa.md` reports `Status: FAIL` with five blockers:

1. KML history can be orphaned by ring rename because `mappa_tracciati_anelli` rows are associated by mutable ring name.
2. KML upload stores process-temp absolute paths in Grappa metadata; this is not a durable repo-fit artifact contract.
3. The UI lacks the explicit approved `Aumenta nodi` workflow.
4. The ring list UI lacks customer filtering.
5. Viewer users cannot inspect node/arc topology details because inspection is tied to Operativo write permission.

## Required Remediation

### Stable KML Association Across Rename

- Keep KML metadata/history associated when a ring name changes.
- Because `mappa_tracciati_anelli` has no `id_anello`, update rows already considered associated with the old name in the same transaction as the ring rename:
  - `nome_anello = oldName`
  - or rows matched by the existing legacy association rule.
- Ensure list/count/delete dependency checks continue to use a single consistent association rule after rename.
- Do not drop historical KML metadata.

### Durable Artifact Storage Contract

- Do not store `os.TempDir()` paths in Grappa metadata.
- Add an explicit storage contract before enabling KML upload:
  - a configured artifact root such as `GRAPPA_DCIM_ARTIFACT_ROOT`
  - pass it into `grappadcim.Deps`
  - store relative artifact keys in `anelli_fibra.kml_file_path` and `mappa_tracciati_anelli.kml`
  - resolve relative keys against the configured root for availability/download
  - keep legacy absolute paths read-only and downloadable only when available
- If `GRAPPA_DCIM_ARTIFACT_ROOT` is not configured, upload must return a clear unsupported/not-configured response and must not persist metadata.
- Document the env var in `backend/.env.example`. Update deployment wiring if needed, but do not invent an object store or migration process beyond this slice.

### Explicit Node-Increase UI

- Add a dedicated Operativo action labeled `Aumenta nodi`.
- Collect a target node count greater than the current count.
- Show an explicit confirmation that additional topology rows will be generated.
- Call `POST /grappa-dcim/v1/fiber-rings/{id}/increase-nodes`.
- Keep node-count decrease blocked in both frontend and backend.

### Customer Filter UI

- Add a customer filter in the ring list workspace.
- Use an existing lookup/customer selector if one exists in the app. If no customer lookup is available, use a numeric customer-id field.
- Pass `customerId` to `useFiberRings`.
- Preserve unknown/legacy customer IDs in display.

### Viewer Topology Inspection

- Decouple topology inspection from mutation.
- Viewer users must be able to open node and arc detail panels.
- Hide or disable save/edit controls for Viewer users; Operativo users retain mutation controls.

## Verification Required

- Run `gofmt` on changed Go files.
- Run `gofmt -l backend/internal/grappadcim`.
- Run `go build ./cmd/server` from `backend`.
- Run `pnpm --filter mrsmith-grappa-dcim build`.
- Confirm no automated test files were added.
- Check whether a suitable Grappa DCIM frontend/backend server is already running before browser checks. If none is running, record the gap.

## Reporting Required

Write `apps/grappa-dcim/docs/fiber-topology-artifacts-fix-1-report.md` with:

- files changed
- behavior implemented
- contracts preserved
- artifact storage contract and disabled/unconfigured behavior
- commands run and outputs summarized
- manual/browser checks and skipped-check reasons
- unresolved questions
- deviations from this remediation
