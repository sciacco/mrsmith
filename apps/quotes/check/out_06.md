# Task reference

`check_06.md`

## Verification target

Customer-specific ERP default-payment lookup and fallback.

## Appsmith factual source

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta/jsobjects/utils/utils.js` (`metodoPagDefault`)
- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta/queries/get_pagamento_anagrCli/get_pagamento_anagrCli.txt`
- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta IaaS/jsobjects/creazioneProposta/creazioneProposta.js` (`metodoPagDefault`)

## Migrated implementation evidence

- `apps/quotes/src/api/queries.ts` → `useCustomerPayment`
- Usage search in `apps/quotes/src`
- `apps/quotes/src/pages/QuoteCreatePage.tsx`

## Comparison

Appsmith actively looks up the ERP payment code for the selected customer and falls back to `402` when no value is available. The migrated app defines a `useCustomerPayment` hook but does not call it anywhere in `apps/quotes/src`, so the documented customer-specific enrichment does not run in the current migrated frontend.

## Outcome

`MISMATCH`

## Differences

- Appsmith triggers the ERP lookup from the create flows.
- The migrated frontend has no consumer for `useCustomerPayment`.
- The migrated create page uses a static initial state `payment_method: '402'` instead of a customer-driven lookup.

## Evidence

- Appsmith JS sets `met_pag_num = '402'`, runs `get_pagamento_anagrCli`, then reuses the returned `CODICE_PAGAMENTO` when present.
- Migrated hook exists in `queries.ts`, but `rg` usage search shows no call sites outside the hook definition.
- `QuoteCreatePage.tsx` initializes `payment_method: '402'`.

## Notes

- This is a direct feature drop in the migrated frontend.
