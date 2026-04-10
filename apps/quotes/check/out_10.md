# Task reference

`check_10.md`

## Verification target

IaaS quote creation flow.

## Appsmith factual source

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta IaaS/jsobjects/creazioneProposta/creazioneProposta.js`
- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta IaaS/queries/get_templates/get_templates.txt`
- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta IaaS/queries/get_templates_byid/get_templates_byid.txt`
- `apps/quotes/APPSMITH-AUDIT.md` section `2.6 Nuova Proposta IaaS`

## Migrated implementation evidence

- `apps/quotes/src/pages/QuoteCreatePage.tsx`
- `apps/quotes/src/api/queries.ts` → `useTemplates`, `useCreateQuote`
- `apps/quotes/src/components/TypeSelector.tsx`

## Comparison

Appsmith has an IaaS-specific creation flow with language-filtered templates, hardcoded template→kit and template→services mapping, fixed recurring terms, trial text generation, and insertion of exactly one derived kit row. The migrated app reuses the generic create page, switches only `quoteType`, fetches templates by `type` only, and does not implement the Appsmith IaaS-specific mappings or trial behavior in the frontend.

## Outcome

`MISMATCH`

## Differences

- No language selector or language-filtered template query is present in the migrated frontend.
- No template→kit or template→services mapping exists in the migrated frontend.
- The migrated frontend does not generate trial text and does not force the Appsmith one-month IaaS term model.
- The migrated create submission does not add the single derived kit row documented in Appsmith.

## Evidence

- Appsmith JS maps template ids such as `853027287235 -> kit 62 -> [12]`.
- Appsmith query: `WHERE lang = substr(LOWER({{ cli_lang.selectedOptionValue}}),1,2) AND (description like 'IaaS%' or description like 'VCLOUD%')`
- Migrated `QuoteCreatePage.tsx` calls `useTemplates({ type: state.quoteType })` and contains no kit/services derivation logic for IaaS.

## Notes

- This is a direct mismatch against the documented Appsmith IaaS behavior.
