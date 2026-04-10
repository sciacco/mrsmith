# Verify quote-row reorder flow

## Objective

Verify whether the migrated quote-row reordering behavior factually matches the original Appsmith position update flow.

## Appsmith source reference

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Dettaglio/queries/upd_quote_row_position/upd_quote_row_position.txt`
- Supporting audit: `apps/quotes/APPSMITH-AUDIT.md` section `2.3 Dettaglio`

## Migrated implementation reference

- `apps/quotes/src/api/queries.ts` → `useUpdateRowPosition`
- `apps/quotes/src/components/KitsTab.tsx`

## Verification procedure

1. Read the Appsmith position-update SQL and note its trigger model and updated fields.
2. Inspect the migrated `useUpdateRowPosition` mutation and record its request path and body.
3. Inspect `KitsTab.tsx` to determine how drag-and-drop computes and sends position updates.
4. Compare the Appsmith one-row inline update flow with the migrated reorder flow.
5. Record any changed request count or changed update semantics explicitly.

## Expected output

Write the verification result to `apps/quotes/check/out_19.md`.
