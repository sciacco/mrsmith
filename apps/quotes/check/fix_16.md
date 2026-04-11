# Fix 16 — Make kit eligibility explicit in both backend and frontend picker flows

## Outcome

`issue_16.md` is addressed.

## What Changed

- `backend/internal/quotes/handler_reference.go` now returns explicit kit eligibility flags:
  - `is_active`
  - `ecommerce`
  - `quotable`
- `apps/quotes/src/api/queries.ts` adds a frontend safeguard that filters kits again to the Appsmith-eligible set even if the backend contract changes later.
- `apps/quotes/src/components/KitPickerModal.tsx` now:
  - searches by category as well as kit name
  - shows an explicit empty state when no eligible kits remain

## Files

- `backend/internal/quotes/handler_reference.go`
- `apps/quotes/src/api/types.ts`
- `apps/quotes/src/api/queries.ts`
- `apps/quotes/src/components/KitPickerModal.tsx`

## Validation

- `pnpm --filter mrsmith-quotes exec tsc --noEmit`
- `go build ./...`
