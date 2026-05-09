# Facilities Layout Fix 1 Report

## Files Changed

- `backend/internal/grappadcim/racks.go`
- `apps/grappa-dcim/docs/facilities-layout-fix-1-report.md`

## Fixes Implemented

- Rack create and move now validate a target position through `positions -> islets -> datacenter` inside the same transaction before occupancy is updated.
- Rack create and move now require the target position's islet datacenter to match the requested datacenter.
- Rack create and move now reject a supplied `isletId` that does not match `positions.islets_id`.
- Rack create and move now derive the persisted `racks.islet_id` from the locked target position when a position is assigned.
- Rack create and move now reject incompatible position statuses before occupancy is updated while preserving the existing Full/Half conflict rules. Full racks still reject any other active occupant; Half racks still reject Full occupants and duplicate A/B vertical positions.
- Rack hard delete dependency checks now include power history through `rack_power_readings JOIN rack_sockets`, returning the existing controlled `409 DependencySummary` shape when readings exist.
- Rack cease remains the lifecycle path that updates rack sockets to `Spento`; the power-history dependency is applied only to hard delete.
- Rack unit updates now run rack metadata update and generated-unit reconciliation in one transaction.
- Rack unit decreases are blocked with a validation error because dependency-safe shrink behavior is not implemented in this slice.
- Rack unit reconciliation errors are returned. The transaction verifies generated unit rows match the requested sequence before reporting success.

## Commands Run

- `gofmt -w backend/internal/grappadcim`
  - Completed with no output.
- `go build ./cmd/server` from `backend`
  - First run failed with `internal/grappadcim/racks.go:282:10: no new variables on left side of :=`.
  - Fixed the transaction-scoped assignment and reran the command.
  - Final run passed with no output.
- `pnpm --filter mrsmith-grappa-dcim build`
  - Passed. TypeScript compiled and Vite built `dist/index.html`, CSS, and JS assets successfully.

## Unresolved Questions

- None for the listed remediation findings.

## Deviations From Remediation Instructions

- No deviations.
- No automated tests were added.
