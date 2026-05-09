# Cabling Crossconnects Remediation 1

## Status

- Owning slice: `cabling-crossconnects`
- Source QA: `apps/grappa-dcim/docs/cabling-crossconnects-qa.md`
- Required fix report: `apps/grappa-dcim/docs/cabling-crossconnects-fix-1-report.md`

## Blocking Finding

`apps/grappa-dcim/docs/cabling-crossconnects-qa.md` reports:

- `Status: FAIL`
- Blocking issue: cable deletion is not dependency-safe for legacy port fiber references.
- Affected code: `backend/internal/grappadcim/cables.go`
- Expected behavior: cable delete must be blocked unless every fiber belonging to the cable is free and unassigned, including all backend dependencies represented by the source schema.
- Actual behavior: the current delete transaction checks `fibers.status`, `fibers.left_port_id`, `fibers.right_port_id`, and `ports.cable_fiber_id`, but does not check `ports.fo_in_id` or `ports.fo_out_id`.
- Schema evidence: `docs/grappa/grappa_ports.json` includes `fo_in_id` and `fo_out_id`; plenum delete dependency handling already treats those fields as linked-port dependencies.

## Required Remediation

- Update the cable delete transaction so it locks and checks `ports` rows referencing any cable fiber through:
  - `ports.cable_fiber_id`
  - `ports.fo_in_id`
  - `ports.fo_out_id`
- If any matching port row exists, return the existing `409 cable_fibers_assigned` response before deleting any `fibers` or `cables`.
- Keep the backend double-confirmation body requirement unchanged.
- Keep frontend behavior unchanged unless needed to preserve the existing user-facing flow.
- Do not add automated tests without human approval.

## Verification Required

- Run `gofmt` for any changed Go files.
- Run `go build ./cmd/server` from `backend`.
- Run `pnpm --filter mrsmith-grappa-dcim build` if any frontend files are changed.
- Confirm no automated test files were added.

## Reporting Required

Write `apps/grappa-dcim/docs/cabling-crossconnects-fix-1-report.md` with:

- files changed
- behavior implemented
- contracts preserved
- commands run and outputs summarized
- unresolved questions
- deviations from this remediation
