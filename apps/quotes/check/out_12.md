# Task reference

`check_12.md`

## Verification target

HubSpot status read and response handling on the detail page.

## Appsmith factual source

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Dettaglio/queries/hs_get_quote_status/metadata.json`
- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Dettaglio/jsobjects/mainForm/mainForm.js` (`mandaSuHubspot`)

## Migrated implementation evidence

- `apps/quotes/src/api/queries.ts` → `useHSStatus`
- `apps/quotes/src/api/types.ts` → `HSStatus`
- `apps/quotes/src/pages/QuoteDetailPage.tsx`

## Comparison

Appsmith reads the HubSpot quote object directly and requests a large property set including `hs_status`, `hs_pdf_download_link`, `hs_quote_link`, and multiple e-signature fields. The migrated frontend reads a reduced internal endpoint `/quotes/v1/quotes/:id/hs-status`, expects only `{ hs_quote_id, status, pdf_url }`, and uses `pdf_url` for the visible "Apri su HS" link.

## Outcome

`MISMATCH`

## Differences

- Appsmith reads many HubSpot properties; the migrated frontend models only three fields.
- Appsmith distinguishes `hs_quote_link` and `hs_pdf_download_link`; the migrated page uses `pdf_url` for the "Apri su HS" action.
- Appsmith publish logic uses `hs_sign_status`; the migrated `HSStatus` type has no equivalent field.

## Evidence

- Appsmith metadata requests properties including `hs_status,hs_pdf_download_link,hs_quote_link,...,hs_sign_status`.
- Migrated type: `interface HSStatus { hs_quote_id: number | null; status: QuoteStatus; pdf_url: string | null; }`
- Migrated page renders `<a ... href={hsStatus.pdf_url ?? '#'}>Apri su HS</a>`.

## Notes

- The mismatch is in both read contract and response handling.
