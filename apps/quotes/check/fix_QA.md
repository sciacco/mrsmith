# Quotes Fix QA Report

Date: 2026-04-11
Scope: QA of `apps/quotes/check/fix_01.md` through `fix_21.md` against the matching `issue_*.md` files and the current codebase.

## Validation Run

- `pnpm --filter mrsmith-quotes exec tsc --noEmit` — PASS
- `cd backend && go build ./...` — PASS
- `cd backend && go test ./internal/quotes/...` — PASS

## Overall Verdict

- Ready to close: `01`, `02`, `03`, `04`, `05`, `06`, `07`, `08`, `09`, `10`, `11`, `12`, `13`, `14`, `15`, `16`, `17`, `18`, `19`, `20`, `21`
- Close with notes: none
- Keep open: none

The previously open blockers are resolved: replacement-order loading now matches the Alyante/Appsmith shape, standard-flow state no longer leaks into IaaS billing behavior, publish uses the correct payment-method schema and passes backend tests, and row reorders now refetch immediately.

## Per-Fix Verdict

| ID | Verdict | QA note |
| --- | --- | --- |
| 01 | READY TO CLOSE | `QuoteListPage` now preserves whether `page` is actually present in the URL, and `useQuotes()` no longer drops an explicit `page=1`. The untouched default load still uses the Appsmith 2000-row path, while returning from page 2 to page 1 stays on the migrated 25-row pagination contract. |
| 02 | READY TO CLOSE | Delete is wired from `QuoteTable` to `useDeleteQuote`, and the row menu now exposes the in-flight delete state instead of silently ignoring later clicks. While one delete is pending, every `Elimina` action is visibly disabled and the active row shows `Eliminazione in corso…`. |
| 03 | READY TO CLOSE | Deal eligibility remains encoded server-side, and the SQL test now also pins the Appsmith outer-parentheses structure plus `ORDER BY d.id DESC`, closing the previous regression-coverage gap. |
| 04 | READY TO CLOSE | Owner list now filters `archived = FALSE` in the backend and matches the issue requirements. |
| 05 | READY TO CLOSE | `/quotes/v1/payment-methods` now uses the correct columns and selectable-only filtering. |
| 06 | READY TO CLOSE | Customer payment lookup is wired into create flow and falls back to `402` correctly. |
| 07 | READY TO CLOSE | Replacement-order loading now uses `NOME_TESTATA_ORDINE`, filters `STATO_ORDINE IN ('Evaso', 'Confermato')`, and keeps the customer ERP bridge filter. The create wizard also clears `replace_orders` when leaving `SOSTITUZIONE` and guards the submit payload so stale hidden values are not persisted. |
| 08 | READY TO CLOSE | Standard create now requests the Appsmith-equivalent `id NOT IN (12,13)` category set, only loads categories while the quote type is `standard`, and computes the COLOCATION billing lock only for the standard recurring flow. That removes the hidden-state path that previously let standard service selection affect IaaS billing. |
| 09 | READY TO CLOSE | Kit selection still feeds `kit_ids` into create, and the remaining wizard polish gaps are resolved: kit search resets when leaving the kit step, the summary shows a loading/count fallback instead of `Nessuno` while kit metadata is still loading, and kit loading is no longer unconditional before the wizard needs it. |
| 10 | READY TO CLOSE | The IaaS flow no longer inherits the standard COLOCATION lock. Switching quote type clears standard service state, the category query is disabled outside standard mode, and `billingLocked` is gated by `quoteType === 'standard'`, so IaaS keeps its fixed `1/1/1` behavior even after a standard-flow COLOCATION selection. |
| 11 | READY TO CLOSE | Detail load/save now round-trips `replace_orders` with Appsmith-visible formatting and DB-safe semicolon serialization. |
| 12 | READY TO CLOSE | HubSpot status contract is expanded and the detail page now uses `quote_url` for `Apri su HS` with a separate PDF link. |
| 13 | READY TO CLOSE | Detail save now builds an explicit payload and reapplies the IaaS save remapping instead of sending raw local state. |
| 14 | READY TO CLOSE | The publish path now uses the loader payment-method columns `cod_pagamento` / `desc_pagamento`, and `persistQuoteForPublish()` no longer trips vet with a dynamic format string. Targeted backend tests pin both contracts, and `go test ./internal/quotes/...` passes. |
| 15 | READY TO CLOSE | Row-position updates now invalidate both `quote-rows` and `publish-precheck`, so the reordered kit list refetches immediately instead of waiting for a later refresh. |
| 16 | READY TO CLOSE | Kit eligibility is now owned by the backend contract and pinned by `TestListKitsQueryMatchesAppsmithEligibility`. The frontend consumes `/quotes/v1/kits` directly, so the previous duplicate-filter maintenance risk is removed while the picker keeps the richer migrated presentation. |
| 17 | READY TO CLOSE | Add-row flow now validates quote existence and kit eligibility before insert, then refreshes row data. |
| 18 | READY TO CLOSE | Delete-row confirmation is correctly scoped and hardened against duplicate requests. |
| 19 | READY TO CLOSE | The single definitive move remains in place, and the mutation now invalidates `quote-rows` after reorder so the UI reflects the backend-renumbered order immediately. |
| 20 | READY TO CLOSE | Product grouping now comes from the backend contract and the frontend consumes grouped data directly. |
| 21 | READY TO CLOSE | Product updates now send the richer Appsmith-equivalent payload, support explicit deselection, and preserve spot/included-row rules. |

## Remaining Notes

No remaining close-with-notes items. The previous QA notes for `02`, `03`, `09`, and `16` are resolved in the current codebase.
