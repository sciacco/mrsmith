# Verify detail quote save flow

## Objective

Verify whether the migrated detail-page save flow factually matches the Appsmith `salvaOfferta()` update behavior for quote header, notes, and contacts.

## Appsmith source reference

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Dettaglio/jsobjects/mainForm/mainForm.js` (`salvaOfferta`)
- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Dettaglio/queries/upd_quote/metadata.json`
- Supporting audit: `apps/quotes/APPSMITH-AUDIT.md` section `2.3 Dettaglio`

## Migrated implementation reference

- `apps/quotes/src/pages/QuoteDetailPage.tsx` → `handleSave`
- `apps/quotes/src/api/queries.ts` → `useUpdateQuote`
- `apps/quotes/src/components/HeaderTab.tsx`
- `apps/quotes/src/components/NotesTab.tsx`
- `apps/quotes/src/components/ContactsTab.tsx`

## Verification procedure

1. Read Appsmith `salvaOfferta()` and record which fields it assembles and how it adjusts template/services for IaaS templates.
2. Inspect migrated `handleSave` and the data object it sends to `useUpdateQuote`.
3. Inspect the editable fields exposed by the migrated header, notes, and contacts tabs.
4. Compare the Appsmith update payload assembly and trigger conditions with the migrated implementation.
5. If the backend semantics behind `PUT /quotes/v1/quotes/:id` are not present in `apps/quotes`, state that explicitly.

## Expected output

Write the verification result to `apps/quotes/check/out_13.md`.
