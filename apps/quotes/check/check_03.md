# Verify standard-wizard deal loading

## Objective

Verify whether the migrated create wizard factually matches the original Appsmith deal-list read used to select a quote deal.

## Appsmith source reference

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta/queries/get_potentials/get_potentials.txt`
- Supporting audit: `apps/quotes/APPSMITH-AUDIT.md` section `2.4 Nuova Proposta`

## Migrated implementation reference

- `apps/quotes/src/api/queries.ts` → `useDeals`
- `apps/quotes/src/pages/QuoteCreatePage.tsx`
- `apps/quotes/src/components/DealCard.tsx`

## Verification procedure

1. Read the Appsmith `get_potentials` SQL and note its pipeline/stage filters, required fields, and ordering.
2. Inspect the migrated `useDeals` hook and record the frontend request path.
3. Inspect `QuoteCreatePage.tsx` to determine how the returned deal data is filtered and consumed.
4. Compare the Appsmith query contract with the observable migrated request and client-side handling.
5. If Appsmith SQL filtering cannot be verified from `apps/quotes/src`, state that explicitly.

## Expected output

Write the verification result to `apps/quotes/check/out_03.md`.
