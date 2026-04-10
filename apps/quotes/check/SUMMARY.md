# Quotes Migration Verification Summary

## 1. Global Overview

- Total checks: 21
- MATCH: 0
- PARTIAL MATCH: 12
- MISMATCH: 9
- NOT VERIFIABLE: 0

Overall migration reliability is limited. The migrated app covers many of the same entry points, but no check achieved full parity, core create/update/delete flows contain direct mismatches, and many read-path checks remain only partially verified because the backend behavior is not observable from `apps/quotes`.

## 2. Issue List

| ID | Source | Verification target | Outcome | Severity | Importance | Short description of the issue | Difference summary | Confidence |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| out_01 | [out_01.md](./out_01.md) | Main quote-list retrieval for the list page | PARTIAL MATCH | 3 | 5 | Quote listing exists, but Appsmith's fixed SQL semantics are not proven in the migrated flow. | Appsmith uses one SQL query with explicit ordering and `limit 2000`; the migrated app calls `/quotes/v1/quotes` with pagination, filters, and sort parameters. | MEDIUM |
| out_02 | [out_02.md](./out_02.md) | Quote deletion from the list page | MISMATCH | 3 | 4 | The visible delete action is not wired to any mutation. | Appsmith runs a documented delete sequence and refresh; the migrated menu renders `Elimina` but `QuoteTable` does not pass `onDelete` to `KebabMenu`. | HIGH |
| out_03 | [out_03.md](./out_03.md) | Deal-list loading for standard quote creation | PARTIAL MATCH | 3 | 5 | Deal loading exists, but the Appsmith filtering rules are not preserved in the migrated frontend evidence. | Appsmith hardcodes pipeline/stage and `codice <> ''` filters in SQL; the migrated app calls `/quotes/v1/deals` and only text-filters the returned list. | MEDIUM |
| out_04 | [out_04.md](./out_04.md) | Owner-list loading | PARTIAL MATCH | 2 | 4 | Owner loading exists, but archived-owner filtering parity is unproven. | Appsmith explicitly filters `archived = FALSE`; the migrated app calls `/quotes/v1/owners` and also adds email matching for "Le mie proposte". | MEDIUM |
| out_05 | [out_05.md](./out_05.md) | Payment-method list loading | PARTIAL MATCH | 2 | 4 | Payment methods are loaded, but selectable/order rules are not verifiable from the migrated frontend. | Appsmith SQL applies `selezionabile is true` and sorts by description; the migrated app consumes a normalized `/quotes/v1/payment-methods` response. | MEDIUM |
| out_06 | [out_06.md](./out_06.md) | Customer-specific ERP default-payment lookup and fallback | MISMATCH | 4 | 4 | Customer-driven payment enrichment is dropped from the create flow. | Appsmith looks up the ERP payment code and falls back to `402`; the migrated app leaves `useCustomerPayment` unused and initializes `payment_method` statically to `402`. | HIGH |
| out_07 | [out_07.md](./out_07.md) | Replacement-order loading for `SOSTITUZIONE` | MISMATCH | 3 | 3 | Replacement-order selection is replaced by free text. | Appsmith loads Alyante orders; the migrated app defines `useCustomerOrders` but does not use it and exposes `replace_orders` as plain text. | HIGH |
| out_08 | [out_08.md](./out_08.md) | Standard create-flow reads and frontend logic for services, templates, and billing locks | MISMATCH | 4 | 5 | The migrated standard create flow omits documented Appsmith read logic and conditional rules. | Appsmith loads service categories, constrains templates by service/document rules, and locks billing for COLOCATION; the migrated page only loads templates by type and has no equivalent service or billing-lock logic. | HIGH |
| out_09 | [out_09.md](./out_09.md) | Standard quote creation write flow | MISMATCH | 5 | 5 | Core creation orchestration is materially different and incomplete from the documented Appsmith behavior. | Appsmith generates a quote number, creates HubSpot data, inserts the quote, inserts kit rows, and navigates; the migrated app performs one POST and does not populate kits during creation. | HIGH |
| out_10 | [out_10.md](./out_10.md) | IaaS quote creation flow | MISMATCH | 4 | 4 | IaaS-specific creation rules are not implemented in the migrated frontend. | Appsmith applies language-filtered templates, template-to-kit/services mappings, fixed term behavior, and trial text generation; the migrated app reuses the generic create page and only switches `quoteType`. | HIGH |
| out_11 | [out_11.md](./out_11.md) | Quote detail header loading | PARTIAL MATCH | 2 | 4 | Detail loading exists, but field-level projection parity is not proven. | Appsmith exposes exact SQL and transforms `replace_orders`; the migrated app reads `/quotes/v1/quotes/:id` and consumes the returned object directly. | MEDIUM |
| out_12 | [out_12.md](./out_12.md) | HubSpot status read and response handling on the detail page | MISMATCH | 4 | 4 | The migrated HubSpot status contract is materially reduced and handled differently. | Appsmith reads a broad HubSpot property set including quote/signature links and sign status; the migrated app models only `{ hs_quote_id, status, pdf_url }` and uses `pdf_url` for "Apri su HS". | HIGH |
| out_13 | [out_13.md](./out_13.md) | Detail-page quote save flow | PARTIAL MATCH | 3 | 5 | Save exists, but payload assembly and IaaS-specific rewrite behavior are not equivalent. | Appsmith rebuilds `updRecord` field-by-field and rewrites template/services for some IaaS cases; the migrated app submits `localQuote` and keeps `services` and `template` read-only. | MEDIUM |
| out_14 | [out_14.md](./out_14.md) | Publish orchestration and client-side prechecks | PARTIAL MATCH | 3 | 5 | Publish exists, but most documented prechecks and orchestration moved out of the frontend. | Appsmith saves, checks HubSpot ids/signature state, validates required products, and performs multiple publish-side effects; the migrated app blocks only on dirty state and posts once to `/publish`. | MEDIUM |
| out_15 | [out_15.md](./out_15.md) | Quote-row list loading on the detail page | PARTIAL MATCH | 2 | 4 | Quote-row loading exists, but ordering/helpers are not proven equivalent. | Appsmith exposes explicit SQL ordered by `position`; the migrated app consumes `/quotes/v1/quotes/:id/rows` as returned. | MEDIUM |
| out_16 | [out_16.md](./out_16.md) | Kit-picker source loading | PARTIAL MATCH | 2 | 4 | Kit loading exists, but exact filter parity is not visible in the migrated frontend. | Appsmith uses SQL variants that exclude inactive/ecommerce kits; the migrated app calls `/quotes/v1/kits` and expects richer presentation fields. | MEDIUM |
| out_17 | [out_17.md](./out_17.md) | Adding a kit row to an existing quote | PARTIAL MATCH | 2 | 4 | Add-row behavior is present, but backend write parity is not directly observable. | Appsmith inserts directly into `quotes.quote_rows`; the migrated app posts `{ kit_id }` to a backend endpoint and refreshes via query invalidation. | MEDIUM |
| out_18 | [out_18.md](./out_18.md) | Deleting a kit row from a quote | PARTIAL MATCH | 2 | 3 | Delete-row behavior exists with a different interaction model. | Appsmith uses a direct SQL delete; the migrated app uses a routed DELETE endpoint and a 3-second inline confirm interaction. | HIGH |
| out_19 | [out_19.md](./out_19.md) | Quote-row position updates | MISMATCH | 3 | 3 | Row reordering semantics changed from single-row numeric edit to two-write swap behavior. | Appsmith updates one row position directly; the migrated app uses drag-and-drop and sends two position updates per drop. | HIGH |
| out_20 | [out_20.md](./out_20.md) | Grouped product loading for a selected quote row | PARTIAL MATCH | 2 | 4 | Product loading exists, but grouping responsibility and response shape differ. | Appsmith returns grouped SQL rows with included-item helper fields; the migrated app fetches a flat product list and groups it client-side. | MEDIUM |
| out_21 | [out_21.md](./out_21.md) | Per-product update flow for quote-row products | MISMATCH | 4 | 5 | Product updates send a reduced payload and omit documented Appsmith adjustments. | Appsmith updates name, prices, quantity, description, and included state with spot-quote adjustments; the migrated app only sends `included`/`quantity` updates and has no explicit `included: false` path. | HIGH |

## 3. Priority Matrix

### High Priority

These issues combine high technical impact with high business relevance. They affect core quote creation, pricing/payment correctness, HubSpot behavior, or product-line editing.

- out_06: Customer-specific ERP default-payment lookup and fallback
- out_08: Standard create-flow reads and frontend logic for services, templates, and billing locks
- out_09: Standard quote creation write flow
- out_10: IaaS quote creation flow
- out_12: HubSpot status read and response handling on the detail page
- out_21: Per-product update flow for quote-row products

### Medium Priority

These issues either have moderate technical impact or affect important user flows. They should be addressed before claiming functional parity, but they are less immediately dangerous than the high-priority set.

- out_01, out_02, out_03, out_04, out_05, out_07, out_11, out_13, out_14, out_15, out_16, out_17, out_18, out_19, out_20

### Low Priority

No issues fall into the low-priority bucket. Every documented issue either touches an important workflow or has at least moderate functional impact.

## 4. Systemic Observations

- Backend opacity is the main reason for partial verification. Many migrated checks only prove that the frontend calls `/quotes/v1/*`; they do not prove SQL-level parity for filters, ordering, joins, or field projections.
- Quote creation is the most affected area. Standard create and IaaS create both have direct mismatches, and related setup reads such as deals, customer payment defaults, replacement orders, services, and templates are also incomplete or only partially verifiable.
- Detail-page business logic is thinner in the migrated frontend. Publish orchestration, HubSpot status handling, product updates, and some save behaviors move logic out of the frontend or reduce the client-side contract compared with Appsmith.
- Several Appsmith constrained selections become generic endpoints or free-text inputs in the migrated app. Observable examples include replacement orders, kit/product payloads, and some list/filter sources.
- No check reached full `MATCH`, so the current evidence supports coverage of similar screens and actions, but not parity of implementation behavior.

## 5. Final Assessment

- Overall migration risk level: HIGH
- Recommendation: Requires major rework

Justification: the evidence shows 9 direct mismatches, 12 partial matches, and 0 full matches. The mismatches are concentrated in core business flows such as quote creation, payment-method derivation, IaaS behavior, HubSpot status handling, and per-product updates. The remaining checks are mostly only partially verified because backend behavior is not visible from the migrated frontend, so the current verification set does not support treating the migration as parity-safe.
