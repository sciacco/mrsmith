# Task reference

`check_08.md`

## Verification target

Standard create-flow reads and frontend logic for services, templates, and billing locks.

## Appsmith factual source

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta/queries/get_product_category/get_product_category.txt`
- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta/jsobjects/TypeDocument/TypeDocument.js`
- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta/jsobjects/Service/Service.js`

## Migrated implementation evidence

- `apps/quotes/src/api/queries.ts` → `useCategories`, `useTemplates`
- `apps/quotes/src/pages/QuoteCreatePage.tsx`
- `apps/quotes/src/components/TypeSelector.tsx`

## Comparison

Appsmith reads service categories, lets the user choose services, derives allowed templates from document type, and forces trimestral billing when COLOCATION is selected for recurring documents. The migrated create page does not load or render service categories at all, filters templates only by `type`, and has no COLOCATION-driven billing lock in the frontend.

## Outcome

`MISMATCH`

## Differences

- Appsmith reads `products.product_category`; the migrated create page never calls `useCategories`.
- Appsmith template choices depend on document type and service selection; the migrated page only calls `useTemplates({ type: state.quoteType })`.
- Appsmith `ServiceChange()` disables and sets billing for COLOCATION; the migrated page has no equivalent trigger logic.

## Evidence

- Appsmith SQL: `SELECT * FROM products.product_category WHERE id NOT IN(12, 13) order by name;`
- Appsmith JS: `if(servizio_colo == true && type_document== "TSC-ORDINE-RIC"){ sl_fatturazione_canoni.setSelectedOption(3); sl_fatturazione_canoni.setDisabled(true); }`
- Migrated `QuoteCreatePage.tsx` imports `useTemplates` but not `useCategories`; no services UI is rendered.

## Notes

- This is a concrete mismatch in both data reads and conditional execution logic.
