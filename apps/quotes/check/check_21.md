# Verify product-update flow

## Objective

Verify whether the migrated per-product update action factually matches the original Appsmith product-update flow for quote-row products.

## Appsmith source reference

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Dettaglio/jsobjects/detailForm/detailForm.js` (`aggiornaRiga`)
- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Dettaglio/queries/upd_quote_row_product/upd_quote_row_product.txt`
- Supporting audit: `apps/quotes/APPSMITH-AUDIT.md` section `2.3 Dettaglio`

## Migrated implementation reference

- `apps/quotes/src/api/queries.ts` → `useUpdateProduct`
- `apps/quotes/src/components/ProductGroupRadio.tsx`

## Verification procedure

1. Read Appsmith `aggiornaRiga()` and record the payload fields it sends to `upd_quote_row_product`, including spot and quantity adjustments.
2. Inspect the migrated `useUpdateProduct` mutation and record its request path.
3. Inspect `ProductGroupRadio.tsx` to determine which frontend actions trigger product updates and which fields are sent.
4. Compare the Appsmith product-update payload and trigger logic with the migrated implementation.
5. Record any omitted fields or missing frontend-side adjustments explicitly.

## Expected output

Write the verification result to `apps/quotes/check/out_21.md`.
