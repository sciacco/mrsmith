# Verify standard service/template/billing logic

## Objective

Verify whether the migrated standard-quote configuration flow factually matches the Appsmith reads and conditional logic for services, templates, and billing locks.

## Appsmith source reference

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta/queries/get_product_category/get_product_category.txt`
- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta/jsobjects/TypeDocument/TypeDocument.js`
- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta/jsobjects/Service/Service.js`
- Supporting audit: `apps/quotes/APPSMITH-AUDIT.md` section `2.4 Nuova Proposta`

## Migrated implementation reference

- `apps/quotes/src/api/queries.ts` → `useCategories`, `useTemplates`
- `apps/quotes/src/pages/QuoteCreatePage.tsx`
- `apps/quotes/src/components/TypeSelector.tsx`

## Verification procedure

1. Record the Appsmith category query and the TypeDocument/Service JS rules that affect template choices and billing.
2. Inspect the migrated create page and determine whether services are loaded, whether categories are read, and how templates are filtered.
3. Inspect whether the migrated page enforces any document-type or COLOCATION billing lock in the frontend.
4. Compare the Appsmith data reads and conditional execution logic with the migrated implementation.
5. Record any missing reads or missing trigger logic explicitly.

## Expected output

Write the verification result to `apps/quotes/check/out_08.md`.
