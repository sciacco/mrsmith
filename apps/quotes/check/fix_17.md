# Fix 17 — Tighten add-row behavior around Appsmith-equivalent insertion semantics

## Outcome

`issue_17.md` is addressed.

## What Changed

- `backend/internal/quotes/handler_rows.go` now validates both sides of the add-row request before insert:
  - quote exists
  - selected kit is active, non-ecommerce, and quotable
- The add-row refetch now returns the same richer row shape used by the main rows endpoint, including HubSpot helper aliases.
- The frontend still refreshes the visible row list after a successful insert through query invalidation.

## Files

- `backend/internal/quotes/handler_rows.go`
- `apps/quotes/src/api/types.ts`

## Validation

- `pnpm --filter mrsmith-quotes exec tsc --noEmit`
- `go build ./...`
