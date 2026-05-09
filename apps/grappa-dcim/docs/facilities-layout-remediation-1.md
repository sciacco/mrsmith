# Facilities Layout Remediation 1

## Status

- Owning slice: `facilities-layout`
- Source QA: `apps/grappa-dcim/docs/facilities-layout-qa.md`
- QA status: `Status: FAIL`
- Fix report required: `apps/grappa-dcim/docs/facilities-layout-fix-1-report.md`
- Allowed write scope:
  - `backend/internal/grappadcim/racks*.go`
  - `backend/internal/grappadcim/dependencies.go` only if needed for dependency helpers
  - `backend/internal/grappadcim/*types.go` only if needed for existing rack response/request semantics
  - `apps/grappa-dcim/docs/facilities-layout-fix-1-report.md`
- Disallowed write scope:
  - Frontend UI changes unless needed to compile against backend request/response shape changes.
  - New features, tests, schema docs, repo wiring, or other slice files.

## Required Reading

- `apps/grappa-dcim/docs/facilities-layout-run.md`
- `apps/grappa-dcim/docs/facilities-layout-implementation-report.md`
- `apps/grappa-dcim/docs/facilities-layout-qa.md`
- `apps/grappa-dcim/docs/facilities-layout-impl.md`
- `apps/grappa-dcim/docs/grappa-dcim-spec.md`
- Schema evidence:
  - `docs/grappa/grappa_positions.json`
  - `docs/grappa/grappa_islets.json`
  - `docs/grappa/grappa_racks.json`
  - `docs/grappa/grappa_units.json`
  - `docs/grappa/grappa_rack_sockets.json`
  - `docs/grappa/grappa_rack_power_readings.json`

## Findings To Fix

1. High - Rack create/move can bind a rack to a position from another sala.
   - QA evidence: `backend/internal/grappadcim/racks.go` checks only `SELECT id FROM positions WHERE id = ? FOR UPDATE` in `ensureRackPositionAvailableTx`, then writes `racks.id_datacenter`, `racks.positions_id`, and `racks.islet_id` from independent request fields.
   - Required correction:
     - Validate target position ownership inside the same transaction.
     - Query `positions p JOIN islets i ON i.id = p.islets_id`.
     - Require `i.datacenter_id = requestedDatacenterId`.
     - Require any supplied `isletId` to match `p.islets_id`.
     - Reject incompatible `p.status` values before updating occupancy.
     - Preserve existing Full/Half rack conflict checks.

2. High - Rack hard delete lacks the socket power-history dependency check.
   - QA evidence: `rackDependencies` checks apparati, servers, optical cassettes, and non-empty ports, but does not check `rack_power_readings` through `rack_sockets`; delete branch then executes `DELETE FROM rack_sockets WHERE rack_id = ?`.
   - Required correction:
     - Add a rack delete dependency check:
       `SELECT COUNT(*) FROM rack_power_readings rpr JOIN rack_sockets rs ON rs.id = rpr.rack_socket_id WHERE rs.rack_id = ?`
     - Return a controlled `409 DependencySummary` when readings exist.
     - Keep rack cease as the safe lifecycle path that sets sockets to `Spento`.

3. Medium - Rack unit reconciliation can report success after a failed unit update.
   - QA evidence: `handleUpdateRack` updates `racks.unit`, ignores `reconcileRackUnits`, and the helper only inserts missing units.
   - Required correction:
     - Make rack metadata update and unit reconciliation transactional.
     - Return reconciliation errors instead of ignoring them.
     - Define shrink behavior conservatively. If decreasing units is not safely dependency-checked in this slice, block decrease with a business-facing validation error instead of silently leaving generated units inconsistent.

## Verification Required

- `gofmt -w backend/internal/grappadcim`
- `go build ./cmd/server` from `backend`
- `pnpm --filter mrsmith-grappa-dcim build`

## Reporting Required

Write `apps/grappa-dcim/docs/facilities-layout-fix-1-report.md` with:
- files changed
- fixes implemented
- commands run and summarized outputs
- unresolved questions
- deviations from remediation instructions
