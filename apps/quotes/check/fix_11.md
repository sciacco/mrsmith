# Fix 11 — Align detail header loading with Appsmith `replace_orders` semantics

## Outcome

`issue_11.md` is addressed.

## What Changed

- `backend/internal/quotes/handler_quotes.go` now applies the Appsmith-visible `REPLACE(replace_orders, ';', ',')` transformation in the detail query.
- `apps/quotes/src/utils/quoteRules.ts` adds:
  - `formatReplaceOrdersForDetail(...)`
  - `parseReplaceOrders(...)`
  - `normalizeReplaceOrdersForSave(...)`
- `apps/quotes/src/pages/QuoteDetailPage.tsx` normalizes detail data through `prepareQuoteForDetail(...)` before it reaches the tabs.
- `apps/quotes/src/components/HeaderTab.tsx` now parses both `;` and `,` separators safely and renders a visible comma-separated summary of the selected replacement orders.
- Save still writes the DB-compatible semicolon serialization, so the visible Appsmith formatting does not corrupt storage.

## Files

- `backend/internal/quotes/handler_quotes.go`
- `apps/quotes/src/pages/QuoteDetailPage.tsx`
- `apps/quotes/src/components/HeaderTab.tsx`
- `apps/quotes/src/utils/quoteRules.ts`

## Validation

- `pnpm --filter mrsmith-quotes exec tsc --noEmit`
- `go build ./...`
