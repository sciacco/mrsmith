# Task reference

`check_07.md`

## Verification target

Replacement-order loading for `SOSTITUZIONE`.

## Appsmith factual source

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta/queries/cli_orders/cli_orders.txt`
- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Dettaglio/queries/cli_orders/cli_orders.txt`
- `apps/quotes/APPSMITH-AUDIT.md` sections `2.3 Dettaglio`, `2.4 Nuova Proposta`, and `2.6 Nuova Proposta IaaS`

## Migrated implementation evidence

- `apps/quotes/src/api/queries.ts` → `useCustomerOrders`
- Usage search in `apps/quotes/src`
- `apps/quotes/src/pages/QuoteCreatePage.tsx`
- `apps/quotes/src/components/HeaderTab.tsx`

## Comparison

Appsmith loads an Alyante order list for replacement orders. The migrated app defines `useCustomerOrders`, but it is not used anywhere in `apps/quotes/src`; instead, `replace_orders` is captured as a plain text input in detail and a plain string field in create state.

## Outcome

`MISMATCH`

## Differences

- Appsmith has a dedicated data read for replacement orders.
- The migrated frontend does not trigger any order-list read.
- The migrated detail UI uses a free-text field for `replace_orders` instead of a loaded option set.

## Evidence

- Appsmith SQL: `select NOME_TESTATA_ORDINE from Tsmi_Ordini where STATO_ORDINE in ('Evaso','Confermato') group by NOME_TESTATA_ORDINE`
- Migrated hook exists in `queries.ts`, but usage search shows no consumers.
- `HeaderTab.tsx` renders `replace_orders` as `<input type="text" ... />`.

## Notes

- The mismatch is observable entirely inside `apps/quotes/src`.
