# Verify kit-picker source loading

## Objective

Verify whether the migrated kit-picker read factually matches the Appsmith kit-loading behavior used for adding kits.

## Appsmith source reference

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Dettaglio/queries/get_kit_internal_names/get_kit_internal_names.txt`
- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta/queries/list_kit/list_kit.txt`
- Supporting audit: `apps/quotes/APPSMITH-AUDIT.md` sections `2.3 Dettaglio` and `2.4 Nuova Proposta`

## Migrated implementation reference

- `apps/quotes/src/api/queries.ts` → `useKits`
- `apps/quotes/src/components/KitPickerModal.tsx`
- `apps/quotes/src/api/types.ts` → `Kit`

## Verification procedure

1. Read the Appsmith kit queries and note their filters and returned fields.
2. Inspect the migrated `useKits` hook and record the request path.
3. Inspect `KitPickerModal.tsx` to determine which fields the migrated UI expects from each kit item.
4. Compare the Appsmith kit-loading behavior with the observable migrated implementation.
5. If the exact backend query shape is not present in `apps/quotes`, state that explicitly.

## Expected output

Write the verification result to `apps/quotes/check/out_16.md`.
