# Task reference

`check_05.md`

## Verification target

Payment-method list loading.

## Appsmith factual source

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Dettaglio/queries/get_payment_method/get_payment_method.txt`
- `apps/quotes/APPSMITH-AUDIT.md` sections `2.3 Dettaglio` and `2.4 Nuova Proposta`

## Migrated implementation evidence

- `apps/quotes/src/api/queries.ts` → `usePaymentMethods`
- `apps/quotes/src/pages/QuoteCreatePage.tsx`
- `apps/quotes/src/components/HeaderTab.tsx`
- `apps/quotes/src/api/types.ts` → `PaymentMethod`

## Comparison

Both implementations load a payment-method list for create and detail forms. Appsmith exposes the exact SQL against `loader.erp_metodi_pagamento`; the migrated app calls `/quotes/v1/payment-methods` and expects normalized fields `{ code, description }`.

## Outcome

`PARTIAL MATCH`

## Differences

- Appsmith query explicitly applies `where selezionabile is true order by desc_pagamento`.
- The migrated frontend cannot prove that the backend preserves the same filter and ordering.
- The migrated response shape is normalized to `code` and `description` rather than raw column names.

## Evidence

- Appsmith SQL: `SELECT cod_pagamento, desc_pagamento FROM loader.erp_metodi_pagamento where selezionabile is true order by desc_pagamento;`
- Migrated hook: `api.get<PaymentMethod[]>('/quotes/v1/payment-methods')`
- Migrated components render `<option key={pm.code} value={pm.code}>{pm.description}</option>`.

## Notes

- The read purpose matches, but exact SQL parity remains unverified from `apps/quotes`.
