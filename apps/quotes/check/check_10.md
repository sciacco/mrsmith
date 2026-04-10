# Verify IaaS quote creation flow

## Objective

Verify whether the migrated IaaS path factually matches the original Appsmith IaaS-specific reads, mappings, and write behavior.

## Appsmith source reference

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta IaaS/jsobjects/creazioneProposta/creazioneProposta.js`
- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta IaaS/queries/get_templates/get_templates.txt`
- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta IaaS/queries/get_templates_byid/get_templates_byid.txt`
- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta IaaS/queries/ins_quote_rows/ins_quote_rows.txt`
- Supporting audit: `apps/quotes/APPSMITH-AUDIT.md` section `2.6 Nuova Proposta IaaS`

## Migrated implementation reference

- `apps/quotes/src/pages/QuoteCreatePage.tsx`
- `apps/quotes/src/api/queries.ts` → `useTemplates`, `useCreateQuote`
- `apps/quotes/src/components/TypeSelector.tsx`

## Verification procedure

1. Record the Appsmith IaaS-specific template filtering, template→kit mapping, template→services mapping, fixed term behavior, and trial-text generation.
2. Inspect the migrated create page for the `quoteType === 'iaas'` path.
3. Verify whether the migrated page reads language-specific IaaS templates, derives kit IDs or services from the chosen template, enforces fixed one-month terms, and builds trial text.
4. Compare the Appsmith IaaS write sequence with the migrated create submission.
5. Record only repository-observable behavior.

## Expected output

Write the verification result to `apps/quotes/check/out_10.md`.
