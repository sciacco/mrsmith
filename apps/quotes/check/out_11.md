# Task reference

`check_11.md`

## Verification target

Quote detail header loading.

## Appsmith factual source

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Dettaglio/queries/get_quote_by_id/get_quote_by_id.txt`
- `apps/quotes/APPSMITH-AUDIT.md` section `2.3 Dettaglio`

## Migrated implementation evidence

- `apps/quotes/src/api/queries.ts` → `useQuote`
- `apps/quotes/src/pages/QuoteDetailPage.tsx`
- `apps/quotes/src/components/HeaderTab.tsx`
- `apps/quotes/src/components/NotesTab.tsx`
- `apps/quotes/src/components/ContactsTab.tsx`

## Comparison

Both implementations load a single quote when opening the detail page and then populate header, notes, and contact UI from that record. Appsmith exposes the exact SQL and includes a `REPLACE(replace_orders,';',',')` transformation; the migrated app requests `/quotes/v1/quotes/:id` and consumes the returned object directly.

## Outcome

`PARTIAL MATCH`

## Differences

- Appsmith loads by `appsmith.store.v_offer_id`; the migrated page loads by route param `:id`.
- Appsmith applies a visible `replace_orders` transformation in SQL; the migrated frontend does not show an equivalent client-side transformation.
- The exact backend field projection behind `/quotes/v1/quotes/:id` is not present in `apps/quotes`.

## Evidence

- Appsmith SQL starts with `select id, quote_number, customer_id, ... REPLACE(replace_orders,';',',') AS replace_orders ... WHERE id = {{appsmith.store.v_offer_id || 0}}`
- Migrated hook: `api.get<Quote>(\`/quotes/v1/quotes/\${id}\`)`
- Migrated tabs consume fields such as `quote.owner`, `quote.payment_method`, `quote.notes`, and `quote.rif_*`.

## Notes

- The high-level read exists, but exact field-level parity cannot be fully proven from the frontend alone.
