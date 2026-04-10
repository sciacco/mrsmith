# Verify quote-list retrieval

## Objective

Verify whether the migrated quote-list read in `apps/quotes` factually matches the original Appsmith `get_quotes` data interaction for loading the main quote table.

## Appsmith source reference

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Elenco Proposte/queries/get_quotes/get_quotes.txt`
- Supporting audit: `apps/quotes/APPSMITH-AUDIT.md` section `2.2 Elenco Proposte`

## Migrated implementation reference

- `apps/quotes/src/api/queries.ts` → `useQuotes`
- `apps/quotes/src/pages/QuoteListPage.tsx`
- `apps/quotes/src/components/QuoteTable.tsx`

## Verification procedure

1. Read the Appsmith `get_quotes` SQL and record its selected fields, joins, default ordering, and limit.
2. Inspect the migrated `useQuotes` hook and identify the request path and query parameters sent by the frontend.
3. Inspect `QuoteListPage.tsx` and `QuoteTable.tsx` to confirm which returned fields the migrated UI consumes.
4. Compare the documented Appsmith query shape with the observable migrated request and response handling.
5. Conclude only from repository evidence in `apps/quotes`; if the backend query shape cannot be observed there, state that explicitly.

## Expected output

Write the verification result to `apps/quotes/check/out_01.md`.
