# Verify ERP default-payment enrichment

## Objective

Verify whether the migrated create flow factually preserves the Appsmith customer-specific default-payment lookup and fallback behavior.

## Appsmith source reference

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta/jsobjects/utils/utils.js` (`metodoPagDefault`)
- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta/queries/get_pagamento_anagrCli/get_pagamento_anagrCli.txt`
- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta IaaS/jsobjects/creazioneProposta/creazioneProposta.js` (`metodoPagDefault`)
- Supporting audit: `apps/quotes/APPSMITH-AUDIT.md` sections `2.4 Nuova Proposta` and `2.6 Nuova Proposta IaaS`

## Migrated implementation reference

- `apps/quotes/src/api/queries.ts` → `useCustomerPayment`
- `apps/quotes/src/pages/QuoteCreatePage.tsx`
- `apps/quotes/src/components/HeaderTab.tsx`
- Usage search in `apps/quotes/src`

## Verification procedure

1. Read the Appsmith ERP-payment query and the JS functions that call it, including the fallback to `402`.
2. Inspect the migrated `useCustomerPayment` hook and record the request path and enable condition.
3. Search `apps/quotes/src` for actual consumers of `useCustomerPayment`.
4. Compare the Appsmith trigger conditions and fallback behavior with the migrated implementation that is actually wired.
5. If the migrated hook exists but is unused, record that explicitly.

## Expected output

Write the verification result to `apps/quotes/check/out_06.md`.
