# Verify payment-method list loading

## Objective

Verify whether the migrated payment-method read factually matches the original Appsmith payment-method query used in creation and detail flows.

## Appsmith source reference

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Dettaglio/queries/get_payment_method/get_payment_method.txt`
- Supporting audit: `apps/quotes/APPSMITH-AUDIT.md` sections `2.3 Dettaglio` and `2.4 Nuova Proposta`

## Migrated implementation reference

- `apps/quotes/src/api/queries.ts` → `usePaymentMethods`
- `apps/quotes/src/pages/QuoteCreatePage.tsx`
- `apps/quotes/src/components/HeaderTab.tsx`
- `apps/quotes/src/api/types.ts` → `PaymentMethod`

## Verification procedure

1. Read the Appsmith payment-method SQL and note its selected fields, filter, and ordering.
2. Inspect the migrated `usePaymentMethods` hook and record its request path.
3. Inspect the create and detail consumers to determine the expected response shape in the migrated app.
4. Compare the Appsmith query contract with the observable migrated request and response handling.
5. If the exact backend SQL behind `/quotes/v1/payment-methods` cannot be observed in `apps/quotes`, state that explicitly.

## Expected output

Write the verification result to `apps/quotes/check/out_05.md`.
