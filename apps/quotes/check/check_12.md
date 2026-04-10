# Verify HubSpot status read flow

## Objective

Verify whether the migrated detail page factually matches the Appsmith HubSpot-status read and response handling.

## Appsmith source reference

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Dettaglio/queries/hs_get_quote_status/metadata.json`
- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Dettaglio/jsobjects/mainForm/mainForm.js` (`mandaSuHubspot`)
- Supporting audit: `apps/quotes/APPSMITH-AUDIT.md` section `2.3 Dettaglio`

## Migrated implementation reference

- `apps/quotes/src/api/queries.ts` → `useHSStatus`
- `apps/quotes/src/api/types.ts` → `HSStatus`
- `apps/quotes/src/pages/QuoteDetailPage.tsx`

## Verification procedure

1. Read the Appsmith HubSpot-status request metadata and record the requested object path and properties.
2. Inspect the migrated `useHSStatus` hook and record the request path.
3. Inspect `HSStatus` and `QuoteDetailPage.tsx` to determine which response fields the migrated frontend expects and how it uses them.
4. Compare the Appsmith read contract and response handling with the migrated implementation.
5. Record any field omissions or changed handling explicitly.

## Expected output

Write the verification result to `apps/quotes/check/out_12.md`.
