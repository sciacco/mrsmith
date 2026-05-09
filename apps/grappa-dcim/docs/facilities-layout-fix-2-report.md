# Facilities Layout Fix 2 Report

## Files Changed

- `apps/grappa-dcim/src/api/queries.ts`

## Fix Implemented

- Added explicit frontend PATCH payload builders for buildings, datacenters, islets, positions, and racks.
- Building and datacenter edits now keep the entity `id` in the route path only.
- Rack edits now send only fields accepted by the backend `RackPatch` contract:
  - `name`
  - `unitCount`
  - `customerId`
  - `status`
  - `magnetotermico`
  - `ampere`
  - `shared`
  - `reserved`
  - `note`
  - `orderCode`
  - `soldPower`
  - `serialNumber`
  - `committedPower`
  - `variableBilling`
- Rack placement fields remain excluded from PATCH and continue to belong to the explicit rack move endpoint.
- Layout PATCH calls were also tightened so `id`, read-only relationship fields, and map display fields are not sent to strict backend decoders.

## Commands Run

- `pnpm --filter mrsmith-grappa-dcim build`
  - Passed. TypeScript compiled and Vite built production assets.
- `go build ./cmd/server` from `backend`
  - Passed with no output.

## Unresolved Questions

- None for this remediation.

## Deviations From Remediation Instructions

- None. The fix stayed in `apps/grappa-dcim/src/api/queries.ts`.
