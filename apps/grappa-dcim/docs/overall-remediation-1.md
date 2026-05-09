# Overall Remediation 1

## Status

- Source QA: `apps/grappa-dcim/docs/overall-qa.md`
- Overall status: FAIL
- Owning slice: `facilities-layout`
- Required fix report: `apps/grappa-dcim/docs/facilities-layout-overall-fix-1-report.md`

## Blocking Findings

`apps/grappa-dcim/docs/overall-qa.md` reports three implementation completeness gaps:

1. Facilities/layout hard-delete actions are not available to Operativo users.
2. `Isole e posizioni` CRUD is incomplete in the implemented workspace.
3. Rack socket and media mutation workflows are backend-only.

## Required Remediation

Implement the approved Operativo UI paths rather than changing V1 scope.

### Facilities Deletes

- Add frontend mutation support and Operativo-only UI actions for:
  - `DELETE /grappa-dcim/v1/facilities/buildings/{id}`
  - `DELETE /grappa-dcim/v1/facilities/datacenters/{id}`
  - `DELETE /grappa-dcim/v1/layout/islets/{id}`
  - `DELETE /grappa-dcim/v1/layout/positions/{id}`
  - `DELETE /grappa-dcim/v1/racks/{id}`
  - `DELETE /grappa-dcim/v1/rack-sockets/{socketId}`
- Use the existing two-checkbox confirmation pattern and send `{ confirmPrimary: true, confirmSecondary: true }`.
- Keep actions hidden for Viewer users.
- Preserve backend dependency checks and surface conflict errors through the existing toast/error pattern.

### Isole E Posizioni CRUD

- On `/isole-posizioni`, add compact Operativo-only controls for:
  - create islet
  - edit islet
  - delete islet
  - edit position
  - delete position
- Keep batch position creation.
- Keep Viewer access read-only.

### Rack Socket And Media Mutations

- On `/rack/:rackId`, add Operativo-only controls for:
  - create rack socket
  - edit rack socket
  - delete rack socket
  - replace rack media metadata/file payload according to the existing backend contract
- Keep read-only socket/media lists for Viewer.
- Use compact workspace UI and existing modal/action patterns.

## Required Reading

- `apps/grappa-dcim/docs/overall-qa.md`
- `apps/grappa-dcim/docs/facilities-layout-run.md`
- `apps/grappa-dcim/docs/facilities-layout-impl.md`
- `apps/grappa-dcim/docs/facilities-layout-qa.md`
- `apps/grappa-dcim/docs/grappa-dcim-spec.md`
- `docs/UI-UX.md`
- `docs/IMPLEMENTATION-PLANNING.md`
- `docs/IMPLEMENTATION-KNOWLEDGE.md`

## Allowed Write Scope

- `apps/grappa-dcim/src/api/types.ts`
- `apps/grappa-dcim/src/api/queries.ts`
- `apps/grappa-dcim/src/features/facilities/FacilitiesPages.tsx`
- `apps/grappa-dcim/src/features/facilities/workspace.module.css`
- `apps/grappa-dcim/src/features/racks/RackPages.tsx`
- `apps/grappa-dcim/docs/facilities-layout-overall-fix-1-report.md`

Do not change backend behavior unless a frontend integration contract is impossible with the current endpoints.

## Verification Required

- `pnpm --filter mrsmith-grappa-dcim build`
- `go build ./cmd/server` from `backend` only if backend files are changed.
- `gofmt -l backend/internal/grappadcim` only if backend files are changed.
- `rg --files apps/grappa-dcim backend/internal/grappadcim | rg '(_test\.go|\.test\.|\.spec\.)'`
- Browser checks only if a suitable Grappa DCIM frontend/backend server is already running.

## Reporting Required

Write `apps/grappa-dcim/docs/facilities-layout-overall-fix-1-report.md` with:

- files changed
- behavior implemented
- contracts preserved
- commands run and outputs summarized
- manual/browser checks and skipped-check reasons
- unresolved questions
- deviations from this remediation
