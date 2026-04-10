# Verify detail header loading

## Objective

Verify whether the migrated detail page factually matches the original Appsmith read used to load quote header data.

## Appsmith source reference

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Dettaglio/queries/get_quote_by_id/get_quote_by_id.txt`
- Supporting audit: `apps/quotes/APPSMITH-AUDIT.md` section `2.3 Dettaglio`

## Migrated implementation reference

- `apps/quotes/src/api/queries.ts` → `useQuote`
- `apps/quotes/src/pages/QuoteDetailPage.tsx`
- `apps/quotes/src/components/HeaderTab.tsx`
- `apps/quotes/src/components/NotesTab.tsx`
- `apps/quotes/src/components/ContactsTab.tsx`

## Verification procedure

1. Read the Appsmith `get_quote_by_id` SQL and note the selected fields and the `replace_orders` transformation.
2. Inspect the migrated `useQuote` hook and record the request path.
3. Inspect the migrated detail page and child tabs to identify which quote fields are consumed after load.
4. Compare the Appsmith header-read contract with the observable migrated request and field handling.
5. If the frontend cannot prove the exact backend field transformation, state that explicitly.

## Expected output

Write the verification result to `apps/quotes/check/out_11.md`.
