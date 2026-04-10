# Task reference

`check_13.md`

## Verification target

Detail-page quote save flow.

## Appsmith factual source

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Dettaglio/jsobjects/mainForm/mainForm.js` (`salvaOfferta`)
- `apps/quotes/APPSMITH-AUDIT.md` section `2.3 Dettaglio`

## Migrated implementation evidence

- `apps/quotes/src/pages/QuoteDetailPage.tsx` → `handleSave`
- `apps/quotes/src/api/queries.ts` → `useUpdateQuote`
- `apps/quotes/src/components/HeaderTab.tsx`
- `apps/quotes/src/components/NotesTab.tsx`
- `apps/quotes/src/components/ContactsTab.tsx`

## Comparison

Both implementations save quote changes through an explicit update action. Appsmith assembles a custom `updRecord` payload, applies template/services remapping for IaaS templates, and refreshes the quote after `upd_quote.run`. The migrated page sends the current `localQuote` object through `PUT /quotes/v1/quotes/:id`, but the editable UI exposes only a subset of the Appsmith fields and does not implement the Appsmith frontend-side IaaS remapping logic.

## Outcome

`PARTIAL MATCH`

## Differences

- Appsmith explicitly rebuilds the payload field-by-field; the migrated frontend submits the current local quote object.
- Appsmith has template/services rewrite branches for specific IaaS template ids; the migrated frontend keeps template and services read-only and has no equivalent rewrite logic.
- The exact backend semantics of `PUT /quotes/v1/quotes/:id` are not observable in `apps/quotes`.

## Evidence

- Appsmith `salvaOfferta()` builds `updRecord` with fields including `customer_id`, `deal_number`, `template`, `services`, `status`, `notes`, `payment_method`, and contact references.
- Migrated save: `updateQuote.mutateAsync({ id: quoteId, data: localQuote })`
- `HeaderTab.tsx` renders `services` and `template` as read-only inputs.

## Notes

- The explicit save interaction exists, but payload assembly is not equivalent.
