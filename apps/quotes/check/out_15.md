# Task reference

`check_15.md`

## Verification target

Quote-row list loading on the detail page.

## Appsmith factual source

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Dettaglio/queries/get_quote_rows/get_quote_rows.txt`
- `apps/quotes/APPSMITH-AUDIT.md` section `2.3 Dettaglio`

## Migrated implementation evidence

- `apps/quotes/src/api/queries.ts` → `useQuoteRows`
- `apps/quotes/src/components/KitsTab.tsx`
- `apps/quotes/src/api/types.ts` → `QuoteRow`

## Comparison

Both implementations load kit rows for a specific quote and use the result to render the kit list in the detail workspace. Appsmith exposes the exact SQL and orders by `position`; the migrated frontend calls `/quotes/v1/quotes/:id/rows` and renders rows in the order received.

## Outcome

`PARTIAL MATCH`

## Differences

- Appsmith query shape and ordering are explicit; the migrated frontend delegates those details to the backend endpoint.
- The migrated `QuoteRow` type omits Appsmith helper aliases `hs_mrc` and `hs_nrc`.

## Evidence

- Appsmith SQL: `SELECT id, quote_id, kit_id, internal_name, ... FROM quotes.quote_rows WHERE quote_id = ... order by position`
- Migrated hook: `api.get<QuoteRow[]>(\`/quotes/v1/quotes/\${quoteId}/rows\`)`
- `KitsTab.tsx` maps `rows` directly into `KitAccordion` components.

## Notes

- Exact SQL parity cannot be proven from `apps/quotes`, but the same read purpose is present.
