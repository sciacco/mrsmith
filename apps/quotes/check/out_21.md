# Task reference

`check_21.md`

## Verification target

Per-product update flow for quote-row products.

## Appsmith factual source

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Dettaglio/jsobjects/detailForm/detailForm.js` (`aggiornaRiga`)
- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Dettaglio/queries/upd_quote_row_product/upd_quote_row_product.txt`

## Migrated implementation evidence

- `apps/quotes/src/api/queries.ts` → `useUpdateProduct`
- `apps/quotes/src/components/ProductGroupRadio.tsx`

## Comparison

Appsmith updates a selected product row by sending a payload with id, product name, nrc, mrc, quantity, extended description, and included flag, and it adjusts `mrc` to `0` for spot quotes and forces quantity `1` when an included row would otherwise be zero. The migrated frontend only sends `{ included: true, quantity: product.quantity || 1 }` when selecting a product and `{ quantity }` when editing quantity, with no frontend handling for `nrc`, `mrc`, `extended_description`, or explicit spot adjustments.

## Outcome

`MISMATCH`

## Differences

- Appsmith sends a richer update payload.
- The migrated frontend does not expose extended description editing.
- The migrated frontend does not apply the Appsmith frontend-side `mrc = 0` spot adjustment.
- The migrated frontend has no explicit path to send `included: false`; it only selects included variants and updates quantity.

## Evidence

- Appsmith `aggiornaRiga()` builds `retObject` with `id`, `product_name`, `nrc`, `mrc`, `quantity`, `extended_description`, `included`, then conditionally sets `retObject.mrc = 0` and `retObject.quantity = 1`.
- Migrated selection update: `data: { included: true, quantity: product.quantity || 1 }`
- Migrated quantity update: `data: { quantity: qty }`

## Notes

- The stored-procedure endpoint may still enforce some rules server-side, but the migrated frontend does not mirror the documented Appsmith update flow.
