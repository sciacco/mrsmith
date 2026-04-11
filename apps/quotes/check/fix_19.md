# Fix 19 — Replace two-row swap writes with a single definitive move

## Outcome

`issue_19.md` is addressed.

## What Changed

- `apps/quotes/src/components/KitsTab.tsx` no longer sends two writes on drop. It now sends one position update for the moved row.
- `backend/internal/quotes/handler_rows.go` translates that single target position into a deterministic full reorder:
  - loads current order
  - moves the selected row once
  - renumbers positions contiguously in a transaction
- This removes the raw swap behavior and makes persisted order match the intended moved-row result after reload.

## Files

- `apps/quotes/src/components/KitsTab.tsx`
- `backend/internal/quotes/handler_rows.go`

## Validation

- `pnpm --filter mrsmith-quotes exec tsc --noEmit`
- `go build ./...`
