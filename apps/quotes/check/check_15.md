# Verify quote-row list loading

## Objective

Verify whether the migrated quote-row read factually matches the Appsmith query used to load kit rows on the detail page.

## Appsmith source reference

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Dettaglio/queries/get_quote_rows/get_quote_rows.txt`
- Supporting audit: `apps/quotes/APPSMITH-AUDIT.md` section `2.3 Dettaglio`

## Migrated implementation reference

- `apps/quotes/src/api/queries.ts` → `useQuoteRows`
- `apps/quotes/src/components/KitsTab.tsx`
- `apps/quotes/src/api/types.ts` → `QuoteRow`

## Verification procedure

1. Read the Appsmith `get_quote_rows` SQL and note the selected fields and ordering.
2. Inspect the migrated `useQuoteRows` hook and record the request path.
3. Inspect `KitsTab.tsx` and identify how the row response is consumed.
4. Compare the Appsmith query contract with the observable migrated request and response handling.
5. If the exact backend SQL behind `/quotes/v1/quotes/:id/rows` is not present in `apps/quotes`, state that explicitly.

## Expected output

Write the verification result to `apps/quotes/check/out_15.md`.
