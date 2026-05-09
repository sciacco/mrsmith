# Grappa DCIM Fiber Topology and Artifacts Implementation Plan

## Slice Contract

- Slice: `fiber-topology-artifacts`
- Purpose: implement fiber ring topology, KML metadata/files, and shared artifact behavior needed by maps/media.
- Approved source: `apps/grappa-dcim/docs/grappa-dcim-spec.md`.
- Depends on: `foundation`; may consume selectors from `facilities-layout`.
- In-scope source surfaces:
  - `anelli-fibra`
  - `mappa_tracciati_anelli`
  - artifact behavior for KML and approved map/media references
- Primary write ownership:
  - `backend/internal/grappadcim/rings*.go`
  - `backend/internal/grappadcim/topology*.go`
  - `backend/internal/grappadcim/artifacts*.go`
  - `apps/grappa-dcim/src/features/rings/*`
  - `apps/grappa-dcim/src/features/artifacts/*`
- Required schema evidence for implementation validation:
  - `docs/grappa/grappa_anelli_fibra.json`
  - `docs/grappa/grappa_nodi.json`
  - `docs/grappa/grappa_archi.json`
  - `docs/grappa/grappa_archi_tratta.json`
  - `docs/grappa/grappa_mappa_tracciati_anelli.json`
  - `docs/grappa/grappa_media.json`

## Comparable Apps Audit

- Reference 1: `apps/manutenzioni/src/pages/MaintenanceDetailPage.tsx`.
- Reference 2: `apps/energia-dc/src/pages/SituazioneRackPage.tsx`.
- Reused patterns:
  - detail shell with tabs for topology, KML metadata, and route details.
  - compact selector/filter workspace from `energia-dc`.
  - explicit empty/error panels from existing mini-apps.
  - auth-capable API transport for downloads/uploads rather than plain unauthenticated links.
- Rejected patterns:
  - map visuals as decorative background.
  - report-style KPI summary cards.
  - Hive sync controls, polling status, or V2-only report links.

## Archetype Choice

- Selected archetype: `data_workspace`.
- Why it fits: fiber rings combine CRUD, generated topology children, route detail, map/KML artifacts, and lifecycle/delete policy.
- Required states:
  - ring list loading, empty, filtered-empty, error
  - ring detail with generated nodes/arcs
  - create topology preview/confirmation
  - node-count increase flow
  - node-count decrease blocked
  - hard delete blocked by meaningful data/artifacts
  - KML file unavailable/preserved historical metadata

## User Copy Rules

- Allowed copy style: business-user-only Italian labels.
- Approved user labels:
  - `Anelli fibra`
  - `Topologia`
  - `Nodi`
  - `Tratte`
  - `KML`
  - `Aumenta nodi`
  - `Cessa anello`
  - `Mappa`
- Forbidden copy risks:
  - do not mention PHP map generation.
  - do not expose source filenames or filesystem mechanics.
  - do not show Hive upload/sync controls in V1.
  - do not surface CWDM or TIM GEA report entry points.
- Metrics allowed: only real topology counts such as nodes, arcs, routes, and KML artifacts for the selected ring.

## Repo-Fit

- Route/base path: under `/apps/grappa-dcim/`.
- Planned frontend routes:
  - `/anelli-fibra`
  - `/anelli-fibra/:ringId`
- API prefix:
  - `/api/grappa-dcim/v1/fiber-rings/...`
  - `/api/grappa-dcim/v1/artifacts/...`
- Access role:
  - Viewer can read topology and approved artifact metadata/downloads.
  - Operativo can create/update rings, increase node count, update route details, upload/update KML metadata/files, cease, and hard-delete only when allowed.
- Dev port / proxy notes: inherited from foundation.
- Static hosting / deployment notes: inherited from foundation.
- Auth transport:
  - downloads/uploads must use authenticated API client or fetch with bearer token.
  - do not use plain links for protected artifacts unless the backend exposes a short-lived authenticated transfer pattern.

## Domain Rules

- Fiber rings:
  - create is atomic and generates N nodes plus N circular arcs.
  - default `stato` on create is `Attivo`.
  - distance and attenuation default to `0` and remain manually editable.
  - node count can increase but cannot decrease.
  - increasing node count must generate the additional nodes/arcs consistently with circular topology semantics.
  - hard delete only for rings without meaningful operational data, KML, routes, coordinates, or references.
  - otherwise use `stato=Cessato`.
- Topology children:
  - preserve `nodi`, `archi`, and `archi_tratta` semantics.
  - route detail updates must not silently discard existing route metadata.
- KML:
  - preserve metadata and historical files where referenced.
  - Hive upload/sync is out of V1.
  - if a historical file path cannot be resolved, UI shows an unavailable artifact state, not a generic crash.
- Exclusions:
  - CWDM is out of V1 and tracked in `docs/TODO.md`.
  - TIM GEA kit report is out of V1 and requires redesign/investigation.
  - first-class active `cassetti_ottici` workflow is out of V1; preserve only as dependency/archive data where needed by other slices.

## API Contract

- Rings:
  - `GET /grappa-dcim/v1/fiber-rings`
  - `POST /grappa-dcim/v1/fiber-rings`
  - `GET /grappa-dcim/v1/fiber-rings/{id}`
  - `PATCH /grappa-dcim/v1/fiber-rings/{id}`
  - `POST /grappa-dcim/v1/fiber-rings/{id}/increase-nodes`
  - `POST /grappa-dcim/v1/fiber-rings/{id}/cease`
  - `DELETE /grappa-dcim/v1/fiber-rings/{id}`
- Topology:
  - `GET /grappa-dcim/v1/fiber-rings/{id}/topology`
  - `PATCH /grappa-dcim/v1/fiber-rings/{id}/nodes/{nodeId}`
  - `PATCH /grappa-dcim/v1/fiber-rings/{id}/arcs/{arcId}`
  - `PUT /grappa-dcim/v1/fiber-rings/{id}/routes`
- KML/artifacts:
  - `GET /grappa-dcim/v1/fiber-rings/{id}/kml`
  - `POST /grappa-dcim/v1/fiber-rings/{id}/kml`
  - `GET /grappa-dcim/v1/artifacts/{artifactId}/download`

## Frontend Plan

- Ring list:
  - search/filter by customer, status, order code, serial, and text fields where source data supports it.
  - show active/all filter without hiding unknown legacy status values.
- Ring detail:
  - tabs: `Riepilogo`, `Topologia`, `Tratte`, `KML`, `Storico`.
  - topology panel with stable dimensions and no text overlap.
  - node/arc inspector for editable fields.
  - increase-node action guarded by confirmation and backend validation.
  - delete action visible only to Operativo and only after dependency precheck.
- Artifact panel:
  - show metadata, current availability, and authenticated download action.
  - upload/update only for Operativo.
  - preserve historical metadata even when file content is unavailable.

## Backend Plan

- Use transactions for ring create, node-count increase, route replace/update, KML metadata update, cease, and hard delete.
- Generate topology on the backend; frontend must not synthesize persisted child rows.
- Use dependency precheck helpers shared with other slices for hard delete.
- Never implement Hive sync or TIM GEA report endpoints in V1.
- Keep artifact operations auth-protected and avoid exposing raw server filesystem paths.

## Verification

- UI review checks:
  - populated ring list and ring detail.
  - topology panel with generated nodes/arcs.
  - KML available and unavailable states.
  - node decrease blocked state.
  - mobile/narrow topology view scrolls or stacks cleanly.
- Runtime / auth checks:
  - Viewer can read and download approved artifacts.
  - Viewer cannot upload/update/cease/delete.
  - Operativo can execute allowed lifecycle actions.
  - protected downloads use bearer auth.
- Tests:
  - No tests are authorized by this planning artifact.
  - The implementer should ask the human expert before adding transaction tests for ring create topology generation, node-count increase, node-count decrease rejection, protected artifact download, and delete blocking because they protect high-risk data behavior.
- Manual validation:
  - verify generated N-node circular arc count against source expectations.
  - verify KML metadata source fields and historical file availability.
  - verify no CWDM or TIM GEA links appear in V1 UI.

## Agent Deliverables

- Implementation report: `apps/grappa-dcim/docs/fiber-topology-artifacts-implementation-report.md`.
- QA report: `apps/grappa-dcim/docs/fiber-topology-artifacts-qa.md`.
- Report must include:
  - topology generation contract implemented
  - KML/artifact auth transport
  - blocked V2 features confirmed absent
  - unresolved artifact storage questions

## Exceptions

- Topology rendering is an approved functional visualization under `data_workspace`. It must support inspection and editing, not decorative presentation.
- Authenticated artifact transfer may use a backend download endpoint instead of a normal anchor link because `docs/IMPLEMENTATION-PLANNING.md` requires explicit auth strategy for protected downloads.
