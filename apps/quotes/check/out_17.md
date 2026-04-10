# Task reference

`check_17.md`

## Verification target

Adding a kit row to an existing quote.

## Appsmith factual source

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Dettaglio/queries/ins_quote_rows/ins_quote_rows.txt`
- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta/queries/ins_quote_rows/ins_quote_rows.txt`

## Migrated implementation evidence

- `apps/quotes/src/api/queries.ts` → `useAddRow`
- `apps/quotes/src/components/KitsTab.tsx`
- `apps/quotes/src/components/KitPickerModal.tsx`

## Comparison

Both implementations add a quote row from a selected kit and refresh the visible row list afterward. Appsmith inserts directly into `quotes.quote_rows`; the migrated frontend posts `{ kit_id }` to `/quotes/v1/quotes/:id/rows` and invalidates the row query on success.

## Outcome

`PARTIAL MATCH`

## Differences

- Appsmith writes directly to the table from the frontend.
- The migrated frontend delegates the insert to a backend endpoint.
- The exact insert statement executed by the backend is not visible in `apps/quotes`.

## Evidence

- Appsmith detail SQL: `insert into quotes.quote_rows (quote_id, kit_id) values (...)`
- Migrated mutation: `api.post<QuoteRow>(\`/quotes/v1/quotes/\${quoteId}/rows\`, { kit_id: kitId })`
- `KitsTab.tsx` triggers `addRow.mutate({ quoteId, kitId })`.

## Notes

- The observable add-row flow is present even though the backend insert is opaque from the frontend.
