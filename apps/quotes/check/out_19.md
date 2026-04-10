# Task reference

`check_19.md`

## Verification target

Quote-row position updates.

## Appsmith factual source

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Dettaglio/queries/upd_quote_row_position/upd_quote_row_position.txt`
- `apps/quotes/APPSMITH-AUDIT.md` section `2.3 Dettaglio`

## Migrated implementation evidence

- `apps/quotes/src/api/queries.ts` → `useUpdateRowPosition`
- `apps/quotes/src/components/KitsTab.tsx`

## Comparison

Appsmith updates one row position directly from an inline edited value. The migrated frontend uses drag-and-drop, finds the dragged and target rows, then sends two separate position updates: one for the dragged row and one for the target row, swapping their positions.

## Outcome

`MISMATCH`

## Differences

- Appsmith performs one update for one edited row.
- The migrated frontend performs two writes per drop operation.
- The trigger model changes from inline numeric edit to drag-and-drop swap semantics.

## Evidence

- Appsmith SQL: `update quotes.quote_rows set position= {{tbl_quote_rows.updatedRow.position}} where id = {{tbl_quote_rows.updatedRow.id}}`
- Migrated `KitsTab.tsx`:
  - `updatePosition.mutate({ quoteId, rowId: dragId, position: targetRow.position })`
  - `updatePosition.mutate({ quoteId, rowId: targetId, position: dragRow.position })`

## Notes

- This is a factual change in write behavior, not just a UI restyling.
