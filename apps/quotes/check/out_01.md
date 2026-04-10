# Task reference

`check_01.md`

## Verification target

Main quote-list retrieval for the list page.

## Appsmith factual source

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Elenco Proposte/queries/get_quotes/get_quotes.txt`
- `apps/quotes/APPSMITH-AUDIT.md` documents the same query under `2.2 Elenco Proposte`

## Migrated implementation evidence

- `apps/quotes/src/api/queries.ts` → `useQuotes`
- `apps/quotes/src/pages/QuoteListPage.tsx`
- `apps/quotes/src/components/QuoteTable.tsx`

## Comparison

Both implementations load the quote list when opening the main list view and render quote number, date, customer, deal, owner, and status in the UI. Appsmith uses a concrete SQL query with joins, `order by q.quote_number desc`, and `limit 2000`, while the migrated app sends a frontend request to `/quotes/v1/quotes` with `page`, `status`, `owner`, `q`, `date_from`, `date_to`, `sort`, and `dir` query parameters and leaves the backend query shape outside `apps/quotes`.

## Outcome

`PARTIAL MATCH`

## Differences

- Appsmith uses one fixed SQL query with no user-facing filters and a hard `limit 2000`.
- The migrated app adds pagination, filters, and sortable request parameters from the frontend.
- The exact SQL fields, joins, ordering defaults, and limit behind `/quotes/v1/quotes` are not present in `apps/quotes`.

## Evidence

- Appsmith query: `select q.id, q.quote_number, q.document_date, ... order by q.quote_number desc limit 2000;`
- Migrated hook: `api.get<QuoteListResponse>(\`/quotes/v1/quotes\${qs ? '?' + qs : ''}\`)`
- Migrated page builds params from URL search state and passes them to `useQuotes`.

## Notes

- This result is limited to the migrated frontend in `apps/quotes`; backend SQL parity cannot be proven there.
