# Fix 21 — Expand product update payloads and restore Appsmith-side product rules

## Outcome

`issue_21.md` is addressed.

## What Changed

- Added `buildProductUpdatePayload(...)` in `apps/quotes/src/utils/quoteRules.ts`.
- `apps/quotes/src/components/ProductGroupRadio.tsx` now sends full product updates instead of partial patches:
  - `id`
  - `product_name`
  - `nrc`
  - `mrc`
  - `quantity`
  - `extended_description`
  - `included`
- Added an explicit `Non incluso` path for optional groups, so deselection is now a first-class frontend action.
- The frontend now mirrors the Appsmith business rules before submit:
  - spot quotes force `mrc = 0`
  - included rows with `quantity <= 0` are corrected to `1`
- The backend still enforces the same rules, so the final persisted outcome is protected on both sides.

## Files

- `apps/quotes/src/utils/quoteRules.ts`
- `apps/quotes/src/components/ProductGroupRadio.tsx`
- `apps/quotes/src/components/KitAccordion.tsx`
- `apps/quotes/src/components/KitsTab.tsx`

## Validation

- `pnpm --filter mrsmith-quotes exec tsc --noEmit`
- `go build ./...`
