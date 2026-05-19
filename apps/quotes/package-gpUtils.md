# Appsmith package `Ordini gestione portale` — audit & port checklist

**Source artifact:** `artifacts/Ordini-gestione-portale.json` (`artifactJsonType: PACKAGE`, schema server 11.0 / client 2.0).
**Internal name:** `Ordini gestione portale` (icon: package, color: `#9747FF1A`).
**Domain:** quotes — produces orders in `vodka.orders` from `quotes.quote`.
**Go port:** `backend/internal/quotes/order_conversion.go` (1004 lines). Endpoints `GET/POST /api/quotes/:id/convert-order`.

This document is the reference for whoever maintains the quote→order converter. It records the original Appsmith logic, maps each rule to its Go equivalent, and tracks outstanding work where the port diverges or replicates a defect.

---

## Inventory

- **1 Module:** `gpUtils` with public inputs `newOrderFromQuote(quoteId)` and `rowsFromQuote(quoteId, orderId, language)`.
- **1 JSObject:** `gpUtils` with static enum `mapTipoDocumento = {NUOVO:"N", SOSTITUZIONE:"A", RINNOVO:"R"}`.
- **2 datasources:** `db-mistra` (PostgreSQL, 7 queries), `vodka` (MySQL, 2 queries).
- **9 SQL/REST actions:**
  - Mistra read: `get_quote_by_id`, `get_quote_rows`, `get_next_system_odv`, `get_next_serial_number`, `get_product_category`.
  - Mistra write: `ins_legacy_order`.
  - Vodka write: `ins_order`, `ins_order_row`.
  - Orphan: `get_new_order_id` (defined but never called — the flow uses `LAST_INSERT_ID()` returned by `ins_order`).

---

## Tables and sequences consumed (Mistra Postgres)

Discovered by reading the package. All live in `db-mistra` (`10.129.32.20`).

| Object | Kind | Role |
|---|---|---|
| `quotes.quote` | table | Header of the proposta. 35+ columns including `deal_number`, `document_type`, `proposal_type`, `services` (JSON), `bill_months`, `nrc_charge_time`, `payment_method`, `trial`, `notes`, `lingua`, all the `rif_*` referents. |
| `quotes.quote_rows` | table | Bundle/row container (`bundle_prefix_row`, `kit_id`, `internal_name`, `position`). |
| `quotes.quote_rows_products` | table | Article rows (`product_code`, `nrc`, `mrc`, `quantity`, `extended_description`, `main_product`, `included`, `position`). |
| `quotes.template` | table | `template_id`, `description` — used to derive `is_colo`. |
| `loader.hubs_company` | table | HubSpot company mirror with `numero_azienda` (= Alyante NUMERO_AZIENDA), `partita_iva`, `codice_fiscale`, `address`, `city`, `zip`, `provincia_di_fatturazione`, `lingua`. |
| `loader.hubs_owner` | table | HubSpot owner (`first_name`, `last_name`). |
| `products.product_category` | table | Service-category lookup (id → name). |
| `products.product` | table | Product master (`code`, `translation_uuid`). |
| `common.translation` | table | i18n catalog (`translation_uuid`, `language`, `short`). |
| `orders.legacy_orders(quote_id, vodka_id, jdata)` | table | Audit trail proposta↔vodka order. Single writer in monorepo: `backend/internal/quotes/order_conversion.go:insertLegacyOrder`. Read by Ordini for traceability. |
| `orders.system_odv_alyante` | sequence | Allocates `cdlan_systemodv` (header) and `cdlan_systemodv_row` (each row, N+1 nextval calls per order). |
| `orders.serial_number` | sequence | Default `cdlan_serialnumber` for each row. |

---

## Derivation rules — source vs Go port

For each rule the Appsmith source is in `gpUtils.newOrderFromQuote` / `rowsFromQuote` (see body inside the package JSON). The Go column is the function in `order_conversion.go` that implements it.

| # | Rule | Source (package) | Go port | Status |
|---|---|---|---|---|
| 1 | `cdlan_systemodv` (header) and `cdlan_systemodv_row` (per row) from `nextval('orders.system_odv_alyante')` | `get_next_system_odv` called once per header + once per row | `insertVodkaOrder` + `buildVodkaOrderRows` (lines ~555, ~697) | ✅ preserved; **Q-new-4** N+1 call still present |
| 2 | `cdlan_serialnumber` default from `nextval('orders.serial_number')` | `get_next_serial_number` per row | line ~701 | ✅ preserved; N+1 |
| 3 | `cdlan_tacito_rin` defaults to `"1"`, forced to `"0"` if `document_type == "TSC-ORDINE"` | `if(quoteData.document_type=="TSC-ORDINE") orderData.cdlan_tacito_rin = "0"` | line 575 (`if documentType == "TSC-ORDINE"`) | ✅ preserved |
| 4 | `profile_lang` derivation | `quote.lingua == "ITA" ? "it" : "en"` (default `en`) | `normalizeLegacyQuoteLanguage`: `EN|ENG|ING → "en"`, else `"it"` (default `it`) | ⚠️ **drift** — Go defaults to `it`, package defaults to `en`. Divergent for NULL/empty/`FRA`. |
| 5 | `is_colo` derivation | `template_description.startsWith("COLO") → "Colocation variabile"`, else `"0"` | `legacyIsColo`: `strings.HasPrefix(strings.TrimSpace(td), "COLO")` | ✅ preserved (Go adds `TrimSpace` — slightly more robust) |
| 6 | `service_type` build | `JSON.parse(quote.services).map(id → category.name).join(", ")` | `parseServiceCategoryIDs` + `serviceNamesForLegacy` + `categoryNamesByID` | ✅ preserved |
| 7 | `cdlan_descart` shape | `translations.find(t→t.language==lang).short + (extended_description ? "\r\n" + extended : "")` | `legacyRowDescription` + `translationShort` | ✅ preserved |
| 8 | `cdlan_prezzo` storage format | `(row.mrc + '').replace(".", ",")` → Italian-locale string `"1234,56"` | `formatLegacyCommaDecimal(value) = strings.ReplaceAll(formatLegacyPlainDecimal(value), ".", ",")` | ✅ preserved |
| 9 | `cdlan_prezzo_attivazione` storage format | `row.nrc` raw (no comma conversion) — asymmetric vs `cdlan_prezzo` | `formatLegacyPlainDecimal` | ✅ preserved as-is (asymmetry intentional or latent — see Q-new-8) |
| 10 | `profile_pv` truncation | `provincia_di_fatturazione.slice(0, 2)` | `provincePrefix` (returns `nil` for empty string instead of `""`) | ✅ preserved (slightly safer) |
| 11 | `cdlan_note` concat | `trial == null ? notes : trial + notes` (no separator) | `quoteOrderNote(trial, notes)` | ✅ preserved |
| 12 | `cdlan_ragg_fatturazione = "A"` for every row | hardcoded in `rowData` | hardcoded in `buildVodkaOrderRow` | ✅ preserved |
| 13 | `cdlan_prezzo_cessazione = "0"` for every row | hardcoded | hardcoded | ✅ preserved |
| 14 | `cdlan_dataconferma = NULL` at creation | hardcoded null | hardcoded | ✅ preserved |
| 15 | `cdlan_evaso = 0`, `cdlan_chiuso = 0`, `confirm_data_attivazione = 0` at creation | hardcoded | hardcoded | ✅ preserved |
| 16 | `cdlan_valuta = "EURO"` | hardcoded | hardcoded | ✅ preserved |
| 17 | `data_decorrenza = ""` (empty string, not NULL) | hardcoded `""` | hardcoded | ✅ preserved |
| 18 | `mapTipoDocumento`: `NUOVO→N` `SOSTITUZIONE→A` `RINNOVO→R` | JSObject static map | `mapProposalTypeToLegacyOrderType` (with `ToUpper(TrimSpace())` normalization) | ✅ preserved + tolerated |

---

## Bug findings (Q-new-1 … Q-new-9)

| # | Bug | In source? | In Go port? | Status |
|---|---|---|---|---|
| Q-new-1 | `cdlan_cliente_id` left `null` despite `customer_number` (= `loader.hubs_company.numero_azienda`) being available in the joined source | yes | yes (line 621 `CdlanClienteID: nil`) | ❌ **outstanding** — Ordini reads it; today all orders created by the converter have it null. Fix means: thread `source.CustomerNumber` into `buildVodkaOrderHeader` and assign `&customerNumber` to `CdlanClienteID`. Side effect: existing orders may need a backfill SQL. |
| Q-new-2 | No transaction across vodka inserts (`orders`, `orders_rows`) + Mistra insert (`legacy_orders`). A crash mid-loop leaves an order with partial rows and no `legacy_orders` audit row. | yes | partial — Go uses `insertVodkaOrder` for the header but row inserts are sequential and `legacy_orders` is written outside the vodka transaction | ❔ verify boundary; consider writing `legacy_orders` *first* as an idempotency sentinel, then creating the vodka order in a transaction. |
| Q-new-3 | No idempotency on `quote_id` — double click creates two orders | yes | **fixed** — `findLegacyOrder` short-circuits when a row already exists for the quote | ✅ resolved |
| Q-new-4 | N+1 sequence calls (`get_next_system_odv` per row, `get_next_serial_number` per row) | yes | yes | ❔ outstanding — could batch via `SELECT nextval(...) FROM generate_series(1, :n)` |
| Q-new-5 | SQL injection in `ins_order` / `ins_order_row` via `{{this.params.X}}` interpolation | yes | **fixed by construction** — `database/sql` parameterized queries | ✅ resolved |
| Q-new-6 | `profile_pv.slice(0,2)` silently truncates extended province names | yes | yes (`provincePrefix`) | ❔ wontfix-by-design — Ordini surfaces the value as-is |
| Q-new-7 | Race: `nextval()` allocates an ID even if the subsequent `INSERT` crashes — leaves "holes" in the sequence | yes | yes | ✅ accepted (Postgres sequence behavior is well-known) |
| Q-new-8 | Pricing format asymmetric: `cdlan_prezzo` localized, `cdlan_prezzo_attivazione` not | yes | yes | ❔ outstanding — normalize at API boundary (`decimal`) and localize in frontend |
| Q-new-9 | `cdlan_ndoc`, `cdlan_anno` written as strings (split of `deal_number`); MySQL coerces to INT on a TEXT column | yes | yes (`parseDealOrderCode` returns strings) | ❔ wontfix — coercion works in practice; documented |

---

## Outstanding work on the quotes converter

In priority order:

1. **Fix Q-new-1 (`cdlan_cliente_id` populated)** — single-line semantics fix in `buildVodkaOrderHeader`. The source row already carries `customer_number` from the `get_quote_by_id` SQL (`hc.numero_azienda as customer_number`). A backfill script may be needed for historical rows (`UPDATE vodka.orders SET cdlan_cliente_id = ? WHERE id = ?` driven by a `loader.hubs_company` join on `cdlan_cliente` RAGIONE_SOCIALE — fragile, ask PO whether to attempt).
2. **Clarify §B4 language drift** — Decide whether `it` or `en` is the correct default for missing/unrecognized `quote.lingua`. Align the port's `normalizeLegacyQuoteLanguage` with PO intent, document the chosen behavior here.
3. **Q-new-2 transactional boundary** — Verify the current handler's failure modes (read `convertQuoteToOrder` end-to-end) and decide whether to introduce an outbox-style sentinel write to `legacy_orders` upfront, or accept the existing best-effort + idempotency check.
4. **Q-new-4 batch sequence allocation** — Optimization only; safe to defer until a quote with many rows shows latency.
5. **Q-new-8 API-level pricing normalization** — Coordinate with Ordini's read path (Phase A Q4 / Phase D §3 Input contract). The legacy storage format is part of the contract today; any normalization affects all consumers.

---

## How Ordini uses this

Ordini reads `vodka.orders` and `vodka.orders_rows` produced by this converter (or by the customer portal — same shape). The invariants Ordini relies on are codified in `apps/ordini/audit/ordini-migspec-phaseD.md` §3 Input contract.

Ordini also reads `orders.legacy_orders` (Mistra Postgres, read-only, scoped to a single column projection) to surface the quote↔order back-pointer on Dettaglio ordine — see `ordini-migspec-phaseD.md` §4 Traceability.

Ordini never writes to `orders.legacy_orders`; this file's table is single-writer.

---

## Cross-references

- Quotes migration spec: `quotes-migspec-phaseA.md` … `quotes-migspec-phaseE-ux.md` (in this folder).
- Quotes implementation notes: `QUOTES-IMPL.md`, `QUOTES-SPEC.md`.
- Ordini audit (consumer view): `apps/ordini/audit/findings-summary.md`, `apps/ordini/audit/ordini-migspec-phaseD.md`, `apps/ordini/audit/ordini-migspec-phaseE-spec.md`.
- Cross-DB identifier mapping (including `loader.hubs_company.numero_azienda`): `docs/IMPLEMENTATION-KNOWLEDGE.md`.
