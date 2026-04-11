# Fix 18 — Keep delete-row semantics correct while hardening the migrated confirmation UX

## Outcome

`issue_18.md` is addressed.

## What Changed

- `apps/quotes/src/components/KitsTab.tsx` now tracks the row currently being deleted and prevents duplicate delete requests for that row.
- `apps/quotes/src/components/KitAccordion.tsx` now scopes the inline confirmation timer safely:
  - cleans up the timer on unmount
  - clears it when the delete is confirmed
  - disables the delete button while the mutation is in flight
- Backend delete-by-row-id ownership checks were already correct and were left intact.

## Files

- `apps/quotes/src/components/KitsTab.tsx`
- `apps/quotes/src/components/KitAccordion.tsx`

## Validation

- `pnpm --filter mrsmith-quotes exec tsc --noEmit`
