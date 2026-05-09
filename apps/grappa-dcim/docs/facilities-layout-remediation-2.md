# Facilities Layout Remediation 2

## Status

- Owning slice: `facilities-layout`
- Source QA: `apps/grappa-dcim/docs/facilities-layout-qa.md`
- QA status: `Status: FAIL`
- Fix report required: `apps/grappa-dcim/docs/facilities-layout-fix-2-report.md`
- Allowed write scope:
  - `apps/grappa-dcim/src/api/queries.ts`
  - `apps/grappa-dcim/src/api/types.ts` only if a narrow patch DTO type is needed
  - `apps/grappa-dcim/docs/facilities-layout-fix-2-report.md`
- Disallowed write scope:
  - Backend changes, UI layout changes, tests, new features, or other slice files.

## Finding To Fix

1. High - Edit flows send unsupported PATCH payload fields and are rejected before update.
   - QA evidence: `apps/grappa-dcim/src/api/queries.ts` sends the whole form object to `api.patch` for buildings, datacenters, and racks, including `id`. Rack edit also sends create/move-only fields such as `datacenterId`, `type`, `position`, `positionId`, `isletId`, and `socketCount`.
   - Backend evidence: `decodeJSONBody` uses `DisallowUnknownFields`; `BuildingPatch`, `DatacenterPatch`, and `RackPatch` do not accept those fields.
   - Required correction:
     - Strip `id` from every PATCH body before calling `api.patch`.
     - For rack edit, construct an explicit patch body containing only fields supported by `RackPatch`.
     - Keep rack placement changes on the explicit move endpoint.

## Verification Required

- `pnpm --filter mrsmith-grappa-dcim build`
- `go build ./cmd/server` from `backend`

## Reporting Required

Write `apps/grappa-dcim/docs/facilities-layout-fix-2-report.md` with:
- files changed
- fix implemented
- commands run and summarized outputs
- unresolved questions
- deviations from remediation instructions
