# Fix 12 — Expand HubSpot status contract and fix the detail-page link target

## Outcome

`issue_12.md` is addressed.

## What Changed

- Added `hubspot.Client.GetQuoteStatus(...)` in `backend/internal/platform/hubspot/quotes.go` to fetch the Appsmith-relevant HubSpot properties.
- `backend/internal/quotes/handler_quotes.go` now returns a richer `/hs-status` response:
  - `hs_quote_id`
  - local `status`
  - remote `hs_status`
  - `quote_url`
  - `pdf_url`
  - `sign_status`
- The detail page now uses `quote_url` for `Apri su HS`, matching Appsmith semantics.
- Added a separate `PDF HS` link when HubSpot exposes a distinct PDF download URL.

## Files

- `backend/internal/platform/hubspot/quotes.go`
- `backend/internal/quotes/handler_quotes.go`
- `apps/quotes/src/api/types.ts`
- `apps/quotes/src/pages/QuoteDetailPage.tsx`

## Validation

- `pnpm --filter mrsmith-quotes exec tsc --noEmit`
- `go build ./...`
