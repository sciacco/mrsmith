# Verify grouped-product read flow

## Objective

Verify whether the migrated product-group read factually matches the original Appsmith query used to load grouped products for a selected quote row.

## Appsmith source reference

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Dettaglio/queries/get_quote_products_grouped/get_quote_products_grouped.txt`
- Supporting audit: `apps/quotes/APPSMITH-AUDIT.md` section `2.3 Dettaglio`

## Migrated implementation reference

- `apps/quotes/src/api/queries.ts` → `useRowProducts`
- `apps/quotes/src/components/KitAccordion.tsx`
- `apps/quotes/src/components/ProductGroupRadio.tsx`
- `apps/quotes/src/api/types.ts` → `ProductVariant`

## Verification procedure

1. Read the Appsmith grouped-products SQL and note its source view, lateral JSON extraction, and parameter source.
2. Inspect the migrated `useRowProducts` hook and record the request path and enable condition.
3. Inspect `KitAccordion.tsx` and `ProductGroupRadio.tsx` to determine when the migrated read is triggered and how the response is grouped.
4. Compare the Appsmith grouped-product read flow with the migrated one.
5. If the backend grouping source cannot be inspected in `apps/quotes`, state that explicitly.

## Expected output

Write the verification result to `apps/quotes/check/out_20.md`.
