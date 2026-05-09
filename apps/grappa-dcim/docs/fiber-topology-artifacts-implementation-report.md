# Fiber Topology Artifacts Implementation Report

## Files Changed

- `backend/internal/grappadcim/handler.go`
- `backend/internal/grappadcim/rings_types.go`
- `backend/internal/grappadcim/rings.go`
- `backend/internal/grappadcim/topology.go`
- `backend/internal/grappadcim/artifacts.go`
- `apps/grappa-dcim/src/api/types.ts`
- `apps/grappa-dcim/src/api/queries.ts`
- `apps/grappa-dcim/src/routes.tsx`
- `apps/grappa-dcim/src/features/rings/RingPages.tsx`
- `apps/grappa-dcim/src/features/rings/rings.module.css`
- `apps/grappa-dcim/docs/fiber-topology-artifacts-implementation-report.md`

## Behavior Implemented

- Added the `Anelli fibra` data workspace at `/anelli-fibra` and `/anelli-fibra/:ringId`.
- Implemented ring list/search/filter by text, status, and customer.
- Implemented ring detail with `Riepilogo`, `Topologia`, `Tratte`, `KML`, and `Storico` tabs.
- Implemented create/update for `anelli_fibra`.
- Implemented atomic ring create with generated `n_nodi` node rows and `n_nodi` circular arc rows.
- Implemented node-count increase through both PATCH and the explicit increase endpoint.
- Blocked node-count decrease in backend and frontend.
- Implemented cease and dependency-gated hard delete with double confirmation.
- Implemented topology node inspection/edit and arc inspection/edit.
- Implemented route detail replacement for one selected arc without touching other arc route details.
- Implemented KML metadata listing with available/unavailable file states.
- Implemented protected KML upload and protected artifact download through authenticated API transport.

## Endpoints

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

## Routes

- `/anelli-fibra`
- `/anelli-fibra/:ringId`

## Topology Contract

- Create runs in one transaction.
- New rings default to `stato = Attivo` unless an explicit status is supplied.
- New nodes use sequential `identificativo` values and `posizione = n * 100`.
- New arcs connect each node to the next, with the final node connected back to the first.
- New arc `distanza` and `attenuazione` are stored as `0`.
- Increasing node count adds new nodes and preserves circular topology.
- If increasing would have to rewrite a closing arc that already has route/reference/distance/attenuation/release data, the backend blocks the operation.
- Decreasing node count returns `fiber_ring_node_decrease_blocked`.

## KML and Artifact Transport

- KML metadata is returned without exposing raw storage paths.
- Historical files that cannot be opened are shown as unavailable artifacts.
- Upload uses authenticated `POST /fiber-rings/{id}/kml` with multipart form data.
- Download uses authenticated fetch against `GET /artifacts/{artifactId}/download`; the UI does not use plain unauthenticated anchor navigation for protected downloads.

## Blocked V2 Features

- No CWDM UI/API was added.
- No TIM GEA or kit report UI/API was added.
- No Hive sync/upload control was added.
- No polling or alerting behavior was added.
- No first-class `cassetti_ottici` workflow was added.

## Contracts Preserved

- Viewer/Operativo split is preserved: reads are under the existing read role gate; mutations and artifact upload are under `RequireOperativo`.
- Destructive cease/delete flows require the existing double-confirmation body.
- The UI keeps the approved `data_workspace` shape with compact Italian operational copy.
- No launcher, hero, marketing dashboard, fake KPI cards, or V2 links were introduced.
- No automated tests were added.

## UI Review

- Phase: post-implementation code-first review.
- Status: approved.
- Evidence package: approved implementation plan, `data_workspace` archetype, explicit exceptions, comparable screens from the plan, route/component scope, and implementation files.
- Residual visual gap: screenshots were not captured because no suitable Grappa DCIM frontend/backend server was already running.

## Commands Run

- `gofmt -w backend/internal/grappadcim/handler.go backend/internal/grappadcim/rings_types.go backend/internal/grappadcim/rings.go backend/internal/grappadcim/topology.go backend/internal/grappadcim/artifacts.go`
  - Completed.
- `gofmt -w backend/internal/grappadcim/rings.go`
  - Completed after a SQL alias cleanup.
- `gofmt -l backend/internal/grappadcim`
  - PASS. No files listed.
- `go build ./cmd/server` from `backend`
  - PASS. Backend compiled successfully.
- `pnpm --filter mrsmith-grappa-dcim build`
  - PASS. TypeScript and Vite production build completed successfully.
- `rg --files apps/grappa-dcim backend/internal/grappadcim | rg '(_test\.go|\.test\.|\.spec\.)'`
  - PASS. No automated test files found; command exited 1 because there were no matches.
- `lsof -nP -iTCP:5191 -sTCP:LISTEN`
  - No listener found.
- `lsof -nP -iTCP:8080 -sTCP:LISTEN`
  - No listener found.
- `lsof -nP -iTCP:5173 -sTCP:LISTEN`
  - No listener found.
- Focused UI copy/V2 leakage checks with `rg`
  - PASS. No implemented ring UI matches for launcher/hero/dashboard/V2/raw technical copy terms.

## Manual and Browser Checks

- Browser checks were not run.
- Reason: no suitable Grappa DCIM Vite dev server or backend server was already listening on `5191`, `8080`, or `5173`, and the run contract says to reuse an existing suitable server rather than start a second server only for this run.
- Populated ring list/detail, generated topology, KML states, node decrease blocked state, destructive confirmations, and narrow topology behavior were reviewed from implementation code only.
- Live Grappa DB/API behavior was not exercised.

## Unresolved Artifact Storage Questions

- The slice does not define a production artifact storage root, retention policy, or object-store integration.
- Uploaded KML files are stored under the process temp directory and persisted in Grappa metadata, which is suitable for protected transport wiring but not final durable production storage.
- Existing legacy KML paths are preserved as metadata and are downloadable only when the backend process can open the referenced file.
- Historical unavailable-file behavior is implemented, but final path migration/normalization remains a deployment/data-migration question.

## Deviations From Plan

- No functional V2 deviations.
- Artifact upload uses local temporary storage because no durable artifact-storage contract is present in the approved run scope.
- The ring-level `anelli_fibra.kml_file_path` entry is exposed through a synthetic protected artifact ID so the UI can use the same authenticated download flow as `mappa_tracciati_anelli` rows without exposing storage paths.
