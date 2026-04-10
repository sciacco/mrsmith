# Verify owner-list loading

## Objective

Verify whether the migrated owner reads factually match the original Appsmith owner-loading behavior used by create, detail, and list filters.

## Appsmith source reference

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta/queries/get_owners/get_owners.txt`
- Supporting usage documented in `apps/quotes/APPSMITH-AUDIT.md` sections `2.3 Dettaglio` and `2.4 Nuova Proposta`

## Migrated implementation reference

- `apps/quotes/src/api/queries.ts` → `useOwners`
- `apps/quotes/src/pages/QuoteCreatePage.tsx`
- `apps/quotes/src/components/HeaderTab.tsx`
- `apps/quotes/src/components/FilterBar.tsx`

## Verification procedure

1. Record the Appsmith owner query and its filter conditions.
2. Inspect the migrated `useOwners` hook and identify the request path.
3. Inspect each migrated consumer to determine what fields from the owner response are used.
4. Compare the observable migrated owner-loading flow with the Appsmith flow.
5. If the exact backend filtering implemented by `/quotes/v1/owners` is not present in `apps/quotes`, state that explicitly.

## Expected output

Write the verification result to `apps/quotes/check/out_04.md`.
