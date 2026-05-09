# Fiber Topology Artifacts Fix 1 Report

## Files Changed

- `backend/internal/grappadcim/rings.go`
- `backend/internal/grappadcim/artifacts.go`
- `backend/internal/grappadcim/handler.go`
- `backend/internal/platform/config/config.go`
- `backend/cmd/server/main.go`
- `backend/.env.example`
- `deploy/k8s/configmap.yaml`
- `deploy/k8s/deployment.yaml`
- `deploy/k8s/grappa-dcim-artifacts-pvc.yaml`
- `apps/grappa-dcim/src/features/rings/RingPages.tsx`
- `apps/grappa-dcim/src/features/rings/rings.module.css`
- `apps/grappa-dcim/docs/fiber-topology-artifacts-fix-1-report.md`

## Behavior Implemented

- Ring rename now updates already-associated `mappa_tracciati_anelli` rows in the same transaction, setting `nome_anello` to the new ring name for rows matched by the legacy association rule.
- KML list/count/delete checks use the same association rule through shared helpers.
- KML upload no longer writes to `os.TempDir()` or stores absolute temp paths.
- KML upload stores relative artifact keys under `kml/ring-{id}/...` when `GRAPPA_DCIM_ARTIFACT_ROOT` is configured.
- Historical absolute KML paths remain read-only and downloadable when the file still exists.
- Relative KML keys resolve only under the configured artifact root.
- Upload returns `503 grappa_dcim_artifact_root_not_configured` before parsing or persisting metadata when the artifact root is not configured.
- The ring workspace now has a dedicated Operativo action labeled `Aumenta nodi`, validates that the target count is greater than the current count, confirms topology row generation, and calls `POST /grappa-dcim/v1/fiber-rings/{id}/increase-nodes`.
- The generic edit modal keeps node count read-only for existing rings and directs operators to `Aumenta nodi`.
- The ring list now includes a numeric customer filter and passes `customerId` to `useFiberRings`.
- Viewer users can open node and arc inspection modals. Save/edit controls remain available only to Operativo users.

## Contracts Preserved

- Existing route and auth contracts are unchanged.
- Protected KML upload/download continues to use authenticated API transport.
- Node-count decrease remains blocked.
- Hard delete remains dependency-gated and continues to block when ring KML metadata or file references exist.
- Historical KML metadata is preserved during rename.
- No automated tests were added.

## Artifact Storage Contract

- New env var: `GRAPPA_DCIM_ARTIFACT_ROOT`.
- Empty value means uploads are disabled with a clear 503 response.
- Configured value must point to durable local/shared storage managed by the deployment environment.
- New uploads persist files below that root and store only relative keys in Grappa metadata.
- Legacy absolute paths are treated as read-only historical references.
- Kubernetes wiring sets `GRAPPA_DCIM_ARTIFACT_ROOT=/var/lib/mrsmith/grappa-dcim-artifacts`, mounts a `ReadWriteMany` PVC at that path, and sets `fsGroup: 65532` for the distroless nonroot runtime.

## Verification Commands

- `gofmt -w backend/internal/grappadcim/rings.go backend/internal/grappadcim/artifacts.go backend/internal/grappadcim/handler.go backend/internal/platform/config/config.go backend/cmd/server/main.go`
  - PASS.
- `gofmt -l backend/internal/grappadcim`
  - PASS. No files listed.
- `go build ./cmd/server` from `backend`
  - PASS. No output.
- `pnpm --filter mrsmith-grappa-dcim build`
  - PASS. TypeScript build and Vite production build completed.
- `rg --files apps/grappa-dcim backend/internal/grappadcim | rg '(_test\.go|\.test\.|\.spec\.)'`
  - PASS for the no-tests rule. No matches; command exited 1 because no automated test files were present.

## Manual Browser Checks

- Checked existing listeners before browser verification:
  - `lsof -nP -iTCP:5191 -sTCP:LISTEN`
  - `lsof -nP -iTCP:8080 -sTCP:LISTEN`
  - `lsof -nP -iTCP:5173 -sTCP:LISTEN`
- No suitable frontend/backend server was already running on the checked ports.
- Browser checks were skipped per the run contract instead of starting a second server.

## UI Review

- Status: approved by code-first post-gate review.
- Evidence: approved `data_workspace` archetype in `apps/grappa-dcim/docs/fiber-topology-artifacts-impl.md`; comparable screens `apps/manutenzioni/src/pages/MaintenanceDetailPage.tsx` and `apps/energia-dc/src/pages/SituazioneRackPage.tsx`; implementation file `apps/grappa-dcim/src/features/rings/RingPages.tsx`.
- Findings: none.
- Residual risk: screenshots were not captured because no suitable existing Grappa DCIM frontend/backend server was running.

## Unresolved Questions

- The production cluster must provide a storage class that supports `ReadWriteMany` for the Grappa DCIM artifact PVC, or operators must bind the claim to an equivalent shared volume.
- No migration was added for legacy absolute KML paths; they remain read-only historical references as required.

## Deviations

- None.
