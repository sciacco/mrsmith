# Task reference

`check_09.md`

## Verification target

Standard quote creation write flow.

## Appsmith factual source

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta/jsobjects/utils/utils.js` (`salvaOfferta`)
- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta/queries/new_quote_number/new_quote_number.txt`
- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta/queries/ins_quote/ins_quote.txt`
- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta/queries/ins_quote_rows/ins_quote_rows.txt`

## Migrated implementation evidence

- `apps/quotes/src/pages/QuoteCreatePage.tsx` → `handleCreate`
- `apps/quotes/src/api/queries.ts` → `useCreateQuote`

## Comparison

Appsmith creates a quote by generating a document number, creating a HubSpot quote, inserting the DB quote, inserting one row per selected kit, storing the new quote id, and navigating to detail. The migrated create flow performs one POST to `/quotes/v1/quotes` with a frontend-assembled payload and then navigates to `/quotes/:id`; it does not generate the quote number in the frontend, does not create the HubSpot quote in the frontend, and does not insert quote rows from a user-selected kit list.

## Outcome

`MISMATCH`

## Differences

- Appsmith runs multiple writes and side effects before navigation.
- The migrated frontend performs a single backend request.
- The migrated create UI does not collect kits in step 3; `kit_ids` remains part of state but no control populates it.

## Evidence

- Appsmith JS: `await new_quote_number.run(); ... await new_hs_quote.run(...); ... await ins_quote.run({dati: updRecord}); ... await ins_quote_rows.run(...)`
- Migrated `handleCreate`: `createQuote.mutateAsync({ ... status: 'DRAFT', kit_ids: state.kit_ids })`
- Migrated step 2 and step 3 text states that kits will be configured after creation.

## Notes

- This result compares factual behavior, not whether the newer flow is preferable.
