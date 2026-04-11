# Fix 10 — Restore IaaS-specific quote creation behavior

## Outcome

`issue_10.md` is addressed.

## What Changed

- `apps/quotes/src/pages/QuoteCreatePage.tsx` now branches the wizard into a real IaaS flow instead of treating IaaS as a generic type toggle.
- Added an IaaS language selector (`ITA` / `ENG`) and passed `lang` into `useTemplates(...)`, so template options are filtered by language.
- Added deterministic template handling through `apps/quotes/src/utils/quoteRules.ts`:
  - template -> kit mapping
  - template -> services mapping (`[12]`, `[13]`, `[14]`, `[15]`)
  - fixed IaaS terms (`initial_term_months = 1`, `next_term_months = 1`, `bill_months = 1`)
- The wizard now derives exactly one initial kit row for IaaS by forcing `kit_ids` to the selected template’s mapped kit.
- Added Appsmith-style IaaS trial text generation from the 0-200 slider and included the generated `trial` text in the create payload.
- Step 3 now shows a read-only derived IaaS kit card instead of the generic multi-kit picker.

## Files

- `apps/quotes/src/pages/QuoteCreatePage.tsx`
- `apps/quotes/src/pages/QuoteCreatePage.module.css`
- `apps/quotes/src/utils/quoteRules.ts`

## Validation

- `pnpm --filter mrsmith-quotes exec tsc --noEmit`
- `go build ./...`
