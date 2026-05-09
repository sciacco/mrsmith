# Fiber Topology Artifacts Run Contract

## Status

- Iteration: 1
- Dependency status:
  - Planning UI review: PASS (`apps/grappa-dcim/docs/planning-ui-review.md`)
  - `foundation`: PASS (`apps/grappa-dcim/docs/foundation-qa.md`)
  - `facilities-layout`: PASS (`apps/grappa-dcim/docs/facilities-layout-qa.md`)
  - `equipment-compute-storage`: PASS (`apps/grappa-dcim/docs/equipment-compute-storage-qa.md`)
  - `cabling-crossconnects`: PASS (`apps/grappa-dcim/docs/cabling-crossconnects-qa.md`)
- Allowed write scope:
  - Backend:
    - `backend/internal/grappadcim/handler.go`
    - `backend/internal/grappadcim/cabling_types.go` or new ring/topology/artifact type files if needed
    - `backend/internal/grappadcim/rings*.go`
    - `backend/internal/grappadcim/topology*.go`
    - `backend/internal/grappadcim/artifacts*.go`
  - Frontend:
    - `apps/grappa-dcim/src/api/types.ts`
    - `apps/grappa-dcim/src/api/queries.ts`
    - `apps/grappa-dcim/src/routes.tsx`
    - `apps/grappa-dcim/src/features/rings/*`
    - `apps/grappa-dcim/src/features/artifacts/*`
    - shared feature CSS only when needed for this slice
  - Documentation:
    - `apps/grappa-dcim/docs/fiber-topology-artifacts-implementation-report.md`
- Disallowed write scope:
  - Completed slice behavior outside direct integration needs.
  - CWDM, TIM GEA, Hive sync, polling, alerting, and first-class `cassetti_ottici`.
  - Automated tests unless explicitly approved by the human.
  - Changes to role names, app base path, or V1 scope.

## Required Reading

- `apps/grappa-dcim/docs/grappa-dcim-spec.md`
- `apps/grappa-dcim/docs/fiber-topology-artifacts-impl.md`
- `apps/grappa-dcim/docs/foundation-qa.md`
- `apps/grappa-dcim/docs/facilities-layout-qa.md`
- `apps/grappa-dcim/docs/equipment-compute-storage-qa.md`
- `apps/grappa-dcim/docs/cabling-crossconnects-qa.md`
- `docs/UI-UX.md`
- `docs/IMPLEMENTATION-PLANNING.md`
- `docs/IMPLEMENTATION-KNOWLEDGE.md`
- `docs/grappa/GRAPPA.md`
- `docs/grappa/grappa_anelli_fibra.json`
- `docs/grappa/grappa_nodi.json`
- `docs/grappa/grappa_archi.json`
- `docs/grappa/grappa_archi_tratta.json`
- `docs/grappa/grappa_mappa_tracciati_anelli.json`
- `docs/grappa/grappa_media.json`

## Implementation Target

- Implement the approved fiber ring workspace for:
  - list/search/filter
  - detail view
  - create/update
  - atomic topology generation
  - node-count increase
  - decrease blocked
  - cease
  - dependency-gated hard delete
  - topology node/arc inspection and edits
  - route detail preservation/update
  - KML metadata/file state and protected artifact download/upload behavior
- Backend API contract:
  - `GET /grappa-dcim/v1/fiber-rings`
  - `POST /grappa-dcim/v1/fiber-rings`
  - `GET /grappa-dcim/v1/fiber-rings/{id}`
  - `PATCH /grappa-dcim/v1/fiber-rings/{id}`
  - `POST /grappa-dcim/v1/fiber-rings/{id}/increase-nodes`
  - `POST /grappa-dcim/v1/fiber-rings/{id}/cease`
  - `DELETE /grappa-dcim/v1/fiber-rings/{id}`
  - `GET /grappa-dcim/v1/fiber-rings/{id}/topology`
  - `PATCH /grappa-dcim/v1/fiber-rings/{id}/nodes/{nodeId}`
  - `PATCH /grappa-dcim/v1/fiber-rings/{id}/arcs/{arcId}`
  - `PUT /grappa-dcim/v1/fiber-rings/{id}/routes`
  - `GET /grappa-dcim/v1/fiber-rings/{id}/kml`
  - `POST /grappa-dcim/v1/fiber-rings/{id}/kml`
  - `GET /grappa-dcim/v1/artifacts/{artifactId}/download`
- Frontend route contract:
  - `/anelli-fibra`
  - `/anelli-fibra/:ringId`
- Data contracts:
  - Create must be atomic and generate `n_nodi` rows plus `n_nodi` circular `archi` rows.
  - Create default `stato` is `Attivo`.
  - New nodes use sequential `identificativo` values and `posizione = n * 100`.
  - New arcs connect each node to the next and the last node back to the first.
  - New arc `distanza` and `attenuazione` default to `0`.
  - Increasing node count is allowed and must preserve circular topology semantics.
  - Decreasing node count is blocked.
  - Hard delete is allowed only when the ring has no meaningful operational data, KML, route details, coordinates, or references; otherwise use `stato=Cessato`.
  - KML metadata and unavailable historical files must be represented without exposing raw filesystem mechanics.
  - Protected downloads/uploads must use authenticated API transport.

## Verification Required

- Commands:
  - `gofmt -w` on changed Go files.
  - `gofmt -l backend/internal/grappadcim`
  - `go build ./cmd/server` from `backend`
  - `pnpm --filter mrsmith-grappa-dcim build`
  - `rg --files apps/grappa-dcim backend/internal/grappadcim | rg '(_test\.go|\.test\.|\.spec\.)'`
- Manual/browser checks:
  - Before browser checks, verify whether a suitable Grappa DCIM frontend/backend server is already running and reuse it.
  - If no suitable server is already running, record the browser check gap; do not start a second server only for this run.
  - Review populated ring list/detail, generated topology, KML available/unavailable states, node decrease blocked state, destructive confirmations, and narrow topology behavior.
- UI review states:
  - Preserve approved `data_workspace` archetype.
  - Use compact operational Italian workspace copy.
  - No launcher, hero, marketing dashboard, decorative map background, fake KPI cards, raw table names, SQL/backend copy, or V2 feature links.

## Reporting Required

Write `apps/grappa-dcim/docs/fiber-topology-artifacts-implementation-report.md` with:

- files changed
- behavior implemented
- endpoint and route lists
- topology generation contract implemented
- KML/artifact auth transport
- blocked V2 features confirmed absent
- contracts preserved
- commands run and outputs summarized
- manual/browser checks and any skipped-check reasons
- unresolved artifact storage questions
- deviations from plan
