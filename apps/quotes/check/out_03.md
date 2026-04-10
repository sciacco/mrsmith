# Task reference

`check_03.md`

## Verification target

Deal-list loading for standard quote creation.

## Appsmith factual source

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta/queries/get_potentials/get_potentials.txt`
- `apps/quotes/APPSMITH-AUDIT.md` section `2.4 Nuova Proposta`

## Migrated implementation evidence

- `apps/quotes/src/api/queries.ts` → `useDeals`
- `apps/quotes/src/pages/QuoteCreatePage.tsx`
- `apps/quotes/src/api/types.ts` → `Deal`

## Comparison

Both implementations load a deal list into the create flow and allow the user to choose one before proceeding. Appsmith uses a concrete SQL query with hardcoded pipeline/stage filters and `codice <> ''`; the migrated app calls `/quotes/v1/deals` and then filters the returned rows client-side by `name` and `company_name`.

## Outcome

`PARTIAL MATCH`

## Differences

- Appsmith filtering rules are explicit in SQL.
- The migrated frontend does not encode those SQL filters; it only performs local text filtering after the endpoint returns.
- The exact backend behavior of `/quotes/v1/deals` is not present in `apps/quotes`.

## Evidence

- Appsmith SQL contains `where ((d.pipeline ='255768766' ... ) or (d.pipeline = '255768768' ...)) and d.codice <> ''`.
- Migrated hook: `api.get<Deal[]>('/quotes/v1/deals')`
- Migrated page filters deals with `d.name.toLowerCase().includes(q)` or `d.company_name?.toLowerCase().includes(q)`.

## Notes

- Frontend parity is only partial because the hardcoded Appsmith SQL filters cannot be verified from `apps/quotes/src`.
