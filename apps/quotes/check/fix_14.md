# Fix 14 — Reintroduce publish prechecks and make the save step real

## Outcome

`issue_14.md` is addressed.

## What Changed

- `backend/internal/quotes/handler_publish.go` no longer fakes the publish save step:
  - it reloads the current quote JSON
  - re-runs `quotes.upd_quote_head(...)`
  - fails the publish flow if that save step fails
- Required-product validation now uses `quotes.v_quote_rows_products`, matching the Appsmith grouped-product logic instead of the weaker old row-level check.
- Added `GET /quotes/v1/quotes/{id}/publish-precheck`.
- `apps/quotes/src/components/PublishModal.tsx` now blocks publish with explicit reasons when:
  - the quote is already signed on HubSpot (`sign_status === ESIGN_COMPLETED`)
  - required product groups are still unconfigured
- `apps/quotes/src/api/queries.ts` invalidates the new precheck whenever rows/products/header data change.

## Files

- `backend/internal/quotes/handler.go`
- `backend/internal/quotes/handler_publish.go`
- `apps/quotes/src/api/queries.ts`
- `apps/quotes/src/api/types.ts`
- `apps/quotes/src/components/PublishModal.tsx`
- `apps/quotes/src/pages/QuoteDetailPage.tsx`

## Validation

- `pnpm --filter mrsmith-quotes exec tsc --noEmit`
- `go build ./...`
