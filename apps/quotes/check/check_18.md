# Verify quote-row delete flow

## Objective

Verify whether the migrated kit-row deletion action factually matches the original Appsmith row-delete behavior.

## Appsmith source reference

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Dettaglio/queries/del_quote_row/del_quote_row.txt`
- Supporting audit: `apps/quotes/APPSMITH-AUDIT.md` section `2.3 Dettaglio`

## Migrated implementation reference

- `apps/quotes/src/api/queries.ts` → `useDeleteRow`
- `apps/quotes/src/components/KitsTab.tsx`
- `apps/quotes/src/components/KitAccordion.tsx`

## Verification procedure

1. Read the Appsmith row-delete SQL and note its target and parameter source.
2. Inspect the migrated `useDeleteRow` mutation and record its request path.
3. Inspect `KitsTab.tsx` and `KitAccordion.tsx` to determine how the delete action is exposed and confirmed.
4. Compare the Appsmith delete flow with the migrated one.
5. Limit conclusions to observable repository evidence.

## Expected output

Write the verification result to `apps/quotes/check/out_18.md`.
