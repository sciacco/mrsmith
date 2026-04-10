# Task reference

`check_04.md`

## Verification target

Owner-list loading.

## Appsmith factual source

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta/queries/get_owners/get_owners.txt`
- `apps/quotes/APPSMITH-AUDIT.md` sections `2.3 Dettaglio` and `2.4 Nuova Proposta`

## Migrated implementation evidence

- `apps/quotes/src/api/queries.ts` → `useOwners`
- `apps/quotes/src/pages/QuoteCreatePage.tsx`
- `apps/quotes/src/components/HeaderTab.tsx`
- `apps/quotes/src/components/FilterBar.tsx`

## Comparison

Both implementations read owners for create/detail flows, and the migrated app also uses the owner list to resolve the "Le mie proposte" preset. Appsmith exposes the exact SQL `select * from loader.hubs_owner where archived = FALSE`; the migrated frontend only calls `/quotes/v1/owners`, so the exact filter and query shape cannot be confirmed from `apps/quotes`.

## Outcome

`PARTIAL MATCH`

## Differences

- Appsmith query explicitly filters `archived = FALSE`.
- The migrated frontend adds an extra consumer in `FilterBar.tsx` to match owner email to the authenticated user.
- The backend implementation of `/quotes/v1/owners` is not available inside `apps/quotes`.

## Evidence

- Appsmith SQL: `select * from loader.hubs_owner where archived = FALSE`
- Migrated hook: `api.get<Owner[]>('/quotes/v1/owners')`
- Migrated consumers render owner names from `firstname`, `lastname`, and use `email` for the preset match.

## Notes

- The read exists, but exact SQL parity is not provable from the migrated frontend alone.
