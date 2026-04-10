# Verify quote delete flow

## Objective

Verify whether the migrated delete action in `apps/quotes` factually matches the original Appsmith delete flow, including trigger wiring and request sequence.

## Appsmith source reference

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Elenco Proposte/jsobjects/utils/utils.js` (`eliminaOfferta`)
- Supporting Appsmith queries:
  - `quotes-main/pages/Elenco Proposte/queries/Cancella_HS_Quote/metadata.json`
  - `quotes-main/pages/Elenco Proposte/queries/Cancella_Offerta/Cancella_Offerta.txt`
- Supporting audit: `apps/quotes/APPSMITH-AUDIT.md` section `2.2 Elenco Proposte`

## Migrated implementation reference

- `apps/quotes/src/components/QuoteTable.tsx`
- `apps/quotes/src/components/KebabMenu.tsx`
- `apps/quotes/src/api/queries.ts` → `useDeleteQuote`

## Verification procedure

1. Inspect Appsmith `eliminaOfferta()` and record the role check, conditional HubSpot delete, DB delete, and refresh sequence.
2. Inspect the migrated `QuoteTable` and `KebabMenu` components to determine whether the delete action is visible and whether it is wired to a mutation.
3. Inspect `useDeleteQuote` to determine the migrated request path and side effect after success.
4. Compare trigger wiring and request sequence between Appsmith and the migrated implementation.
5. If the migrated mutation exists but is not invoked from the UI, record that explicitly.

## Expected output

Write the verification result to `apps/quotes/check/out_02.md`.
