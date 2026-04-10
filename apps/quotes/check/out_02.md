# Task reference

`check_02.md`

## Verification target

Quote deletion from the list page.

## Appsmith factual source

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Elenco Proposte/jsobjects/utils/utils.js`
- `apps/quotes/APPSMITH-AUDIT.md` section `2.2 Elenco Proposte`

## Migrated implementation evidence

- `apps/quotes/src/components/QuoteTable.tsx`
- `apps/quotes/src/components/KebabMenu.tsx`
- `apps/quotes/src/api/queries.ts` → `useDeleteQuote`

## Comparison

Appsmith exposes a working delete flow: role check, optional HubSpot quote delete, DB delete, then `get_quotes.run()`. The migrated app exposes a visual `Elimina` menu item only when the user has `app_quotes_delete`, but `QuoteTable` does not pass any `onDelete` handler to `KebabMenu`, so the UI action does not invoke `useDeleteQuote` at all.

## Outcome

`MISMATCH`

## Differences

- Appsmith delete is wired and executes a documented request sequence.
- The migrated frontend defines `useDeleteQuote` but never connects it to the visible delete menu item.
- Appsmith conditionally deletes the HubSpot quote before the DB quote; the migrated frontend shows only one backend delete endpoint and never calls it from the menu.

## Evidence

- Appsmith JS: `if (tbl_quote.selectedRow.hs_quote_id > 0) await Cancella_HS_Quote.run(...); await Cancella_Offerta.run(...); await get_quotes.run();`
- Migrated `QuoteTable`: `<KebabMenu quoteId={q.id} canDelete={canDelete} />`
- Migrated `KebabMenu`: delete button calls `onDelete?.(); close();`
- No caller passes `onDelete` from `QuoteTable`.

## Notes

- This is a direct UI wiring mismatch, not just an unverifiable backend difference.
