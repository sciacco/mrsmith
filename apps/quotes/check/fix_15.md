# Fix 15 — Preserve Appsmith row ordering semantics on load and after reorder

## Outcome

`issue_15.md` is addressed.

## What Changed

- `/quotes/v1/quotes/{id}/rows` still returns rows ordered by `position`, but the response now also exposes the Appsmith-style helper aliases `hs_mrc` and `hs_nrc`.
- `backend/internal/quotes/handler_rows.go` now renumbers row positions deterministically when a move is persisted, so reloads stay stable and position-based.
- `apps/quotes/src/api/types.ts` now models the helper aliases on `QuoteRow`.

## Files

- `backend/internal/quotes/handler_rows.go`
- `apps/quotes/src/api/types.ts`

## Validation

- `pnpm --filter mrsmith-quotes exec tsc --noEmit`
- `go build ./...`
