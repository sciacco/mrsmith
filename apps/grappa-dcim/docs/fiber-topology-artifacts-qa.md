# Fiber Topology Artifacts QA Rerun

Status: PASS

## Scope Reviewed

- Run contract: `apps/grappa-dcim/docs/fiber-topology-artifacts-run.md`
- Implementation report: `apps/grappa-dcim/docs/fiber-topology-artifacts-implementation-report.md`
- Previous QA: `apps/grappa-dcim/docs/fiber-topology-artifacts-qa.md`
- Remediation contract: `apps/grappa-dcim/docs/fiber-topology-artifacts-remediation-1.md`
- Fix report: `apps/grappa-dcim/docs/fiber-topology-artifacts-fix-1-report.md`
- Approved source spec and plan: `apps/grappa-dcim/docs/grappa-dcim-spec.md`, `apps/grappa-dcim/docs/fiber-topology-artifacts-impl.md`
- Repo/design references: `docs/UI-UX.md`, `docs/IMPLEMENTATION-PLANNING.md`, `docs/IMPLEMENTATION-KNOWLEDGE.md`
- Schema evidence: `docs/grappa/GRAPPA.md`, `docs/grappa/grappa_anelli_fibra.json`, `docs/grappa/grappa_nodi.json`, `docs/grappa/grappa_archi.json`, `docs/grappa/grappa_archi_tratta.json`, `docs/grappa/grappa_mappa_tracciati_anelli.json`, `docs/grappa/grappa_media.json`
- Implementation files changed by the original slice and remediation 1.

## Remediation 1 Closure

1. Stable KML association across rename: PASS.
   - Evidence: `backend/internal/grappadcim/rings.go` updates associated `mappa_tracciati_anelli` rows in the same ring update transaction, using the legacy association rule before changing `anelli_fibra.nome`.
   - Evidence: KML summary/list/delete checks use the shared association helpers in `backend/internal/grappadcim/rings.go` and `backend/internal/grappadcim/artifacts.go`.
   - Result: existing KML metadata remains associated after a ring rename and continues to block hard delete.

2. Durable artifact storage contract and deployment wiring: PASS.
   - Evidence: `backend/internal/platform/config/config.go`, `backend/cmd/server/main.go`, and `backend/internal/grappadcim/handler.go` wire `GRAPPA_DCIM_ARTIFACT_ROOT` into the Grappa DCIM handler.
   - Evidence: `backend/internal/grappadcim/artifacts.go` rejects upload with `503 grappa_dcim_artifact_root_not_configured` when the root is empty, stores new files under relative `kml/ring-{id}/...` keys, and resolves relative keys only below the configured root.
   - Evidence: `backend/.env.example`, `deploy/k8s/configmap.yaml`, `deploy/k8s/deployment.yaml`, and `deploy/k8s/grappa-dcim-artifacts-pvc.yaml` document and mount durable shared storage.
   - Result: the temp-directory persistence blocker is closed. Legacy absolute paths remain read-only historical references.

3. Explicit `Aumenta nodi` flow: PASS.
   - Evidence: `apps/grappa-dcim/src/features/rings/RingPages.tsx` exposes an Operativo-only `Aumenta nodi` action, validates the target count, shows confirmation copy about generated topology rows, and calls `increaseNodes`.
   - Evidence: `apps/grappa-dcim/src/api/queries.ts` calls `POST /grappa-dcim/v1/fiber-rings/{id}/increase-nodes`.
   - Result: the generic edit modal no longer serves as the only node-increase path for existing rings, and decrease remains blocked.

4. Customer filtering: PASS.
   - Evidence: `apps/grappa-dcim/src/features/rings/RingPages.tsx` includes a numeric customer filter and passes `customerId` to `useFiberRings`.
   - Evidence: `backend/internal/grappadcim/rings.go` validates and applies `customerId` as `af.id_anagrafica = ?`.
   - Result: ring list filtering by customer is implemented.

5. Viewer read-only topology inspection: PASS.
   - Evidence: `apps/grappa-dcim/src/features/rings/RingPages.tsx` no longer disables topology node/arc selection for Viewer users. Viewer opens read-only `Nodo fibra` and `Tratta fibra` modals, while save/edit controls remain Operativo-only.
   - Result: Viewer can inspect topology without mutation access.

## Product Behavior and Contracts

- Ring list/search/filter, ring detail, create/update, cease, dependency-gated delete, topology inspection/edit, route replacement, KML metadata, KML upload, and artifact download match the approved slice behavior at code-review level.
- Backend create is transactional and generates `n_nodi` nodes plus circular arcs with sequential identifiers, `posizione = n * 100`, and zero distance/attenuation defaults.
- Node-count decrease remains blocked in backend and frontend.
- Protected KML upload/download uses authenticated API transport rather than unauthenticated links.
- Hard delete remains gated by double confirmation and dependency checks for operational ring data, KML, route details, coordinates, node references, and arc references.
- Viewer/Operativo role separation is preserved: read routes use the existing read role gate and mutations/upload/destructive actions use `RequireOperativo`.
- Schema docs match the implementation's main table and column usage for `anelli_fibra`, `nodi`, `archi`, `archi_tratta`, `mappa_tracciati_anelli`, and `media`.

## UI Review Gate

- Phase: post-implementation code-first review.
- Status: approved.
- Evidence package: sufficient for code-first review. The approved plan declares the `data_workspace` archetype, explicit exceptions, comparable screens (`apps/manutenzioni/src/pages/MaintenanceDetailPage.tsx`, `apps/energia-dc/src/pages/SituazioneRackPage.tsx`), route scope, and implementation files.
- Gates checked: evidence, archetype, style-family, copy, metrics, shared shell, and exceptions.
- Result: no blocking UI findings remain. The screen keeps a compact mini-app workspace, business-user Italian copy, real topology counts only, and no launcher/hero/marketing-dashboard drift.
- Visual verification gap: screenshots were not captured because no suitable Grappa DCIM frontend/backend server was already listening, and the run contract says not to start a second server only for this QA rerun.

## Exclusion Checks

- No CWDM workflow was introduced.
- No TIM GEA workflow or report entry point was introduced.
- No Hive sync/upload control was introduced.
- No polling or alerting behavior was introduced.
- No first-class `cassetti_ottici` workflow was introduced. Existing references remain dependency/archive checks from completed slices.

## Verification Commands

- `gofmt -l backend/internal/grappadcim`
  - PASS. No files listed.
- `go build ./cmd/server` from `backend`
  - PASS. No output.
- `pnpm --filter mrsmith-grappa-dcim build`
  - PASS. TypeScript and Vite production build completed.
- `rg --files apps/grappa-dcim backend/internal/grappadcim | rg '(_test\.go|\.test\.|\.spec\.)'`
  - PASS for the no-tests rule. No matches; command exited 1 because no automated test files were present.
- Existing-server checks before browser verification:
  - `lsof -nP -iTCP:5191 -sTCP:LISTEN`
  - `lsof -nP -iTCP:8080 -sTCP:LISTEN`
  - `lsof -nP -iTCP:5173 -sTCP:LISTEN`
  - No suitable listener found.

## Manual Browser Checks

Not run. Per the run contract, browser checks should reuse an already running suitable Grappa DCIM frontend/backend server. No suitable listener was present on the checked Grappa/Vite/backend ports, so browser verification is recorded as a residual gap instead of starting a second server.

## Residual Risks

- Live Grappa DB behavior was not exercised in this QA rerun.
- Visual screenshot verification for populated list/detail, generated topology, KML available/unavailable states, blocked node decrease, destructive confirmations, and narrow topology layout remains pending until a suitable existing environment is available.
- Production must provide a `ReadWriteMany`-capable storage class or equivalent bound shared volume for `mrsmith-grappa-dcim-artifacts`.
