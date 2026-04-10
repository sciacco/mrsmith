# Verify quote-row add flow

## Objective

Verify whether the migrated add-kit action factually matches the Appsmith quote-row insert behavior.

## Appsmith source reference

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Dettaglio/queries/ins_quote_rows/ins_quote_rows.txt`
- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta/queries/ins_quote_rows/ins_quote_rows.txt`
- Supporting audit: `apps/quotes/APPSMITH-AUDIT.md` sections `2.3 Dettaglio` and `2.4 Nuova Proposta`

## Migrated implementation reference

- `apps/quotes/src/api/queries.ts` → `useAddRow`
- `apps/quotes/src/components/KitsTab.tsx`
- `apps/quotes/src/components/KitPickerModal.tsx`

## Verification procedure

1. Read the Appsmith `ins_quote_rows` statements and note their input parameters.
2. Inspect the migrated `useAddRow` mutation and record its request path and request body.
3. Inspect `KitsTab.tsx` and `KitPickerModal.tsx` to determine how the add-row mutation is triggered.
4. Compare the Appsmith insert behavior with the migrated action flow.
5. If only the frontend request can be observed, limit conclusions to that scope.

## Expected output

Write the verification result to `apps/quotes/check/out_17.md`.
