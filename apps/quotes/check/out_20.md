# Task reference

`check_20.md`

## Verification target

Grouped product loading for a selected quote row.

## Appsmith factual source

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Dettaglio/queries/get_quote_products_grouped/get_quote_products_grouped.txt`
- `apps/quotes/APPSMITH-AUDIT.md` section `2.3 Dettaglio`

## Migrated implementation evidence

- `apps/quotes/src/api/queries.ts` → `useRowProducts`
- `apps/quotes/src/components/KitAccordion.tsx`
- `apps/quotes/src/components/ProductGroupRadio.tsx`

## Comparison

Both implementations load product data for a selected quote row and then present it grouped by `group_name`. Appsmith performs the grouping and included-item extraction in SQL using `quotes.v_quote_rows_products` plus a lateral JSON expansion; the migrated frontend requests `/quotes/v1/quotes/:quoteId/rows/:rowId/products` and groups the returned flat `ProductVariant[]` client-side.

## Outcome

`PARTIAL MATCH`

## Differences

- Appsmith SQL already returns grouped row data with included-item helper fields.
- The migrated frontend expects a flat product list and rebuilds groups in `ProductGroupRadio.tsx`.
- The backend data source behind the migrated endpoint is not visible in `apps/quotes`.

## Evidence

- Appsmith SQL starts with `select vqrp.*, obj->>'id' as inc_id, ... from quotes.v_quote_rows_products vqrp left join lateral ...`
- Migrated hook: `api.get<ProductVariant[]>(\`/quotes/v1/quotes/\${quoteId}/rows/\${rowId}/products\`)`
- Migrated grouping: `const map = new Map<string, ProductVariant[]>(); ... map.get(p.group_name)`

## Notes

- The same read purpose exists, but the response shape and grouping boundary differ.
