# Task reference

`check_14.md`

## Verification target

Publish orchestration and client-side prechecks.

## Appsmith factual source

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Dettaglio/jsobjects/mainForm/mainForm.js` (`mandaSuHubspot`)
- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Dettaglio/queries/check_quote_rows/check_quote_rows.txt`
- `apps/quotes/APPSMITH-AUDIT.md` section `2.3 Dettaglio`

## Migrated implementation evidence

- `apps/quotes/src/components/PublishModal.tsx`
- `apps/quotes/src/api/queries.ts` → `usePublishQuote`
- `apps/quotes/src/pages/QuoteDetailPage.tsx`

## Comparison

Both implementations expose an explicit publish action on the detail page. Appsmith performs substantial client-side orchestration before and during publish, including save, HS id checks, signed-quote checks, required-product validation, status calculation, and direct HubSpot update calls. The migrated frontend opens a modal and posts once to `/quotes/v1/quotes/:id/publish`, then renders returned step statuses.

## Outcome

`PARTIAL MATCH`

## Differences

- Appsmith contains multiple client-side prechecks and side effects; the migrated frontend delegates almost all orchestration to the backend endpoint.
- Appsmith uses `check_quote_rows` directly before publish; no equivalent frontend-side required-products query exists in the migrated app.
- The migrated page blocks publish only when the quote is dirty.

## Evidence

- Appsmith JS calls `await this.salvaOfferta();`, checks `hs_quote_id`, checks `hs_sign_status`, runs `check_quote_rows`, then executes multiple HubSpot operations.
- Migrated `QuoteDetailPage.tsx` disables publish when `isDirty`.
- Migrated `PublishModal.tsx` calls `publishQuote.mutateAsync(quoteId)` and displays returned steps.

## Notes

- Because the migrated backend publish logic is outside `apps/quotes`, this result is limited to frontend-observable behavior.
