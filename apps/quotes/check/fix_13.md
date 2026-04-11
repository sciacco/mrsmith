# Fix 13 — Use an explicit detail-save payload and reapply IaaS save remapping

## Outcome

`issue_13.md` is addressed.

## What Changed

- Added `buildQuoteSavePayload(...)` in `apps/quotes/src/utils/quoteRules.ts` so the detail page no longer sends the raw `localQuote` object back to the API.
- The save payload is now assembled field-by-field and excludes local-only joined fields like `customer_name`, `deal_name`, and `owner_name`.
- The builder reapplies the Appsmith-visible IaaS remapping before save:
  - fixed `template`
  - fixed `services`
  - fixed 1/1/1 term model
- `apps/quotes/src/pages/QuoteDetailPage.tsx` now saves through that payload builder.
- `apps/quotes/src/components/HeaderTab.tsx` also now correctly detects IaaS templates and disables the term inputs instead of leaving them editable.

## Files

- `apps/quotes/src/utils/quoteRules.ts`
- `apps/quotes/src/pages/QuoteDetailPage.tsx`
- `apps/quotes/src/components/HeaderTab.tsx`

## Validation

- `pnpm --filter mrsmith-quotes exec tsc --noEmit`
- `go build ./...`
