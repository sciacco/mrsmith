# Verify standard quote creation flow

## Objective

Verify whether the migrated standard-quote creation flow factually matches the original Appsmith write sequence and payload assembly.

## Appsmith source reference

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta/jsobjects/utils/utils.js` (`salvaOfferta`)
- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta/queries/new_quote_number/new_quote_number.txt`
- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta/queries/ins_quote/ins_quote.txt`
- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta/queries/ins_quote_rows/ins_quote_rows.txt`
- Supporting audit: `apps/quotes/APPSMITH-AUDIT.md` section `2.4 Nuova Proposta`

## Migrated implementation reference

- `apps/quotes/src/pages/QuoteCreatePage.tsx` → `handleCreate`
- `apps/quotes/src/api/queries.ts` → `useCreateQuote`
- `apps/quotes/src/api/types.ts`

## Verification procedure

1. Read Appsmith `salvaOfferta()` and record the write sequence: quote number generation, HubSpot quote creation, DB insert, kit-row inserts, and navigation.
2. Inspect migrated `handleCreate` and record the payload posted to `/quotes/v1/quotes`.
3. Verify whether the migrated create page collects kits, generates quote numbers, or performs any pre-insert side effects in the frontend.
4. Compare the Appsmith write sequence and payload assembly with the migrated implementation.
5. Treat only observable code in `apps/quotes` as evidence.

## Expected output

Write the verification result to `apps/quotes/check/out_09.md`.
