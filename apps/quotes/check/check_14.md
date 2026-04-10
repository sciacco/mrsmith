# Verify publish flow orchestration

## Objective

Verify whether the migrated publish action factually matches the Appsmith publish sequence and client-side prechecks.

## Appsmith source reference

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Dettaglio/jsobjects/mainForm/mainForm.js` (`mandaSuHubspot`)
- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Dettaglio/queries/check_quote_rows/check_quote_rows.txt`
- Supporting audit: `apps/quotes/APPSMITH-AUDIT.md` section `2.3 Dettaglio`

## Migrated implementation reference

- `apps/quotes/src/components/PublishModal.tsx`
- `apps/quotes/src/api/queries.ts` → `usePublishQuote`
- `apps/quotes/src/pages/QuoteDetailPage.tsx`

## Verification procedure

1. Read Appsmith `mandaSuHubspot()` and record the observable client-side sequence and preconditions.
2. Inspect migrated `PublishModal.tsx` and `QuoteDetailPage.tsx` to determine the frontend trigger conditions before publish.
3. Inspect `usePublishQuote` to record the migrated request and response contract expected by the frontend.
4. Compare the Appsmith client-side orchestration with the migrated publish flow.
5. Distinguish clearly between logic still observable in `apps/quotes` and logic moved behind an opaque backend endpoint.

## Expected output

Write the verification result to `apps/quotes/check/out_14.md`.
