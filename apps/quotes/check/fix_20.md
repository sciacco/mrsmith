# Fix 20 — Move grouped product loading back to the backend contract

## Outcome

`issue_20.md` is addressed.

## What Changed

- `backend/internal/quotes/handler_rows.go` no longer returns a flat product list for a quote row.
- The endpoint now reads `quotes.v_quote_rows_products` and returns grouped product data directly:
  - `group_name`
  - `quote_row_id`
  - `products`
  - `count`
  - `required`
  - `main_product`
  - `position`
  - `included_product`
- `apps/quotes/src/api/queries.ts` and `apps/quotes/src/api/types.ts` now model the grouped response explicitly.
- `apps/quotes/src/components/ProductGroupRadio.tsx` now consumes those grouped structures directly instead of rebuilding grouping on the client.

## Files

- `backend/internal/quotes/handler_rows.go`
- `apps/quotes/src/api/types.ts`
- `apps/quotes/src/api/queries.ts`
- `apps/quotes/src/components/ProductGroupRadio.tsx`
- `apps/quotes/src/components/KitAccordion.tsx`

## Validation

- `pnpm --filter mrsmith-quotes exec tsc --noEmit`
- `go build ./...`
