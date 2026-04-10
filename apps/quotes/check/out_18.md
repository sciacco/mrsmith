# Task reference

`check_18.md`

## Verification target

Deleting a kit row from a quote.

## Appsmith factual source

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Dettaglio/queries/del_quote_row/del_quote_row.txt`
- `apps/quotes/APPSMITH-AUDIT.md` section `2.3 Dettaglio`

## Migrated implementation evidence

- `apps/quotes/src/api/queries.ts` → `useDeleteRow`
- `apps/quotes/src/components/KitsTab.tsx`
- `apps/quotes/src/components/KitAccordion.tsx`

## Comparison

Both implementations expose a row deletion action in the detail page and remove the row by id. Appsmith issues a direct SQL delete for the selected row; the migrated frontend calls a backend DELETE endpoint with both quote id and row id and refreshes the row list through query invalidation.

## Outcome

`PARTIAL MATCH`

## Differences

- Appsmith uses a direct table delete from the frontend.
- The migrated app uses a routed backend endpoint.
- The migrated UI uses a 3-second inline confirm state instead of Appsmith's separate confirmation model.

## Evidence

- Appsmith SQL: `DELETE from quotes.quote_rows where id = {{tbl_quote_rows.selectedRow.id}}`
- Migrated mutation: `api.delete(\`/quotes/v1/quotes/\${quoteId}/rows/\${rowId}\`)`
- `KitAccordion.tsx` requires a second click after showing `Conferma?`.

## Notes

- The delete action exists and is wired in the migrated frontend.
