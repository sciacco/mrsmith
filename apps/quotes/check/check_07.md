# Verify replacement-order loading

## Objective

Verify whether the migrated app factually preserves the Appsmith order-list read used for `SOSTITUZIONE` replacement orders.

## Appsmith source reference

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta/queries/cli_orders/cli_orders.txt`
- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Dettaglio/queries/cli_orders/cli_orders.txt`
- Supporting audit: `apps/quotes/APPSMITH-AUDIT.md` sections `2.3 Dettaglio`, `2.4 Nuova Proposta`, and `2.6 Nuova Proposta IaaS`

## Migrated implementation reference

- `apps/quotes/src/api/queries.ts` → `useCustomerOrders`
- `apps/quotes/src/pages/QuoteCreatePage.tsx`
- `apps/quotes/src/components/HeaderTab.tsx`
- Usage search in `apps/quotes/src`

## Verification procedure

1. Read the Appsmith `cli_orders` SQL and note what it returns.
2. Inspect the migrated `useCustomerOrders` hook and record its request path and enable condition.
3. Search `apps/quotes/src` for actual consumers of `useCustomerOrders`.
4. Inspect the migrated UI for the field that captures replacement orders.
5. Compare the Appsmith order-loading behavior with the actual migrated implementation.

## Expected output

Write the verification result to `apps/quotes/check/out_07.md`.
