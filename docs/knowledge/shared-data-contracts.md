# Shared Data Contracts

Column-level quirks of the shared databases (loader, vodka, Grappa, Mistra common) that bind every consumer, present and future.
Part of the Implementation Knowledge Handbook — see [docs/IMPLEMENTATION-KNOWLEDGE.md](../IMPLEMENTATION-KNOWLEDGE.md) for the index, entry format, and placement rules.

### Grappa DATE Columns Serialize as RFC3339 When Scanned to String

- Context: any Go backend query against Grappa (MySQL) that selects a `DATE`/`DATETIME` column and scans it into a `string` (or returns it in JSON).
- Discovery: the MySQL DSN runs with `parseTime`, so `DATE` columns come back as `time.Time`; `database/sql` then formats `time.Time` → `string` as RFC3339 (e.g. `2025-12-22T00:00:00Z`), not `2025-12-22`. This silently breaks YYYY-MM-DD validation and date formatting on the client and caused 400 `invalid_from_parameter` on the IaaS drill-down.
- Practical rule: when a Go handler must return a clean date string for a `DATE`/`DATETIME` value, wrap the expression in `DATE_FORMAT(col, '%Y-%m-%d')` (or cast to CHAR) in SQL. Do not rely on scanning a raw DATE into a string. Frontend date helpers that consume these values should also tolerate an optional time/timezone suffix (take the first 10 chars).
- Evidence: `backend/internal/panoramica/handler_iaas.go` `bucketExpression` (DATE_FORMAT-wrapped after the fix) and regression test `TestBucketExpressionValidation`; `apps/panoramica-cliente/src/hooks/useChargeDrill.ts` `parseDate`.
- Used by: `apps/panoramica-cliente` IaaS Pay Per Use.
- Open questions: none.

### Panoramica Orders Summary Text Columns Can Be NULL

- Context: `GET /api/panoramica/v1/orders/summary` backed by `loader.v_ordini_sintesi`.
- Discovery: production data can return `NULL` for multiple `loader.v_ordini_sintesi` text fields used by the summary endpoint, including `stato` and `numero_ordine`, even though the original backend/frontend contract modeled them as required strings.
- Practical rule: scan summary text columns with `sql.NullString` in backend handlers and normalize them deliberately before JSON encoding; do not scan those columns directly into Go `string` fields.
- Evidence: backend failures `list_orders_summary_scan` on 2026-04-09 for `stato` and `numero_ordine` (`converting NULL to string is unsupported`), fixed in `backend/internal/panoramica/handler_orders.go`.
- Used by: nothing anymore — the summary endpoint and the Ordini Ricorrenti (OLD) page were removed on 2026-06-10; the general rule (scan loader text columns with `sql.NullString`) still applies to all loader-backed handlers.
- Open questions: none.

### Loader `quantita` Must Be Treated as Decimal (Nullable) Across Reports and Panoramica

- Context: report/order endpoints reading `quantita` from loader views such as `v_ordini_ric_spot`, `v_ordini_sintesi`, and `v_ordini_ricorrenti_conrinnovo`.
- Discovery: `quantita` is defined as `double precision` in Mistra loader view contracts and can be fractional (for example `7.5`) and nullable. Scanning into Go `int`/`sql.NullInt64` causes runtime failures (`Scan error ... converting driver.Value type float64 ... to int`) and/or truncation risk.
- Practical rule: for loader-backed APIs, scan `quantita` with `sql.NullFloat64` and expose it as nullable decimal in JSON/TS contracts (`*float64` in Go responses, `number | null` in TS). Do not cast/round to int unless an explicit business rule requires integer quantities.
- Evidence: `docs/mistradb/mistra_loader.json` (`quantita` column type `double precision` on loader views), backend failure on `POST /api/reports/v1/orders/preview` on 2026-04-14 with value `7.5`, and follow-up fixes in reports/panoramica handlers.
- Used by: `apps/reports` (`orders`, `active-lines`, `pending-activations`, `upcoming-renewals`) and `apps/panoramica-cliente` (`orders/summary`, `orders/detail`).
- Open questions: none.

### Loader `tipo_documento` Is Fixed-Width Padded; `v_ordini_ric_spot` Is The Canonical Recurring+Spot Source

- Context: any query distinguishing recurring (`TSC-ORDINE-RIC`) from spot (`TSC-ORDINE`) orders in `loader.erp_ordini`.
- Discovery: the ERP loader pads `tipo_documento` to fixed width, so spot orders are stored as `'TSC-ORDINE    '` (14 chars, trailing spaces) in a `varchar(100)` column. Exact equality (`tipo_documento = 'TSC-ORDINE'`) silently matches zero spot rows. The view `loader.v_ordini_ric_spot` already handles this (`TRIM(BOTH FROM tipo_documento)` in its WHERE) and is the canonical recurring+spot row source (same shape as `v_ordini_ricorrenti`, also excludes `CDL-AUTO`).
- Practical rule: never compare `tipo_documento` with raw equality — use `btrim(tipo_documento)` or, better, query `loader.v_ordini_ric_spot` instead of rebuilding the join. Note the view requires at least one non-CDL-AUTO order row, so customer/status lookups built on it match exactly what an order grid built on it can show.
- Evidence: read-only probe on Mistra dev DB (2026-06-10): `length(tipo_documento) = 14` for both values; `loader.v_ordini_ric_spot` definition in `docs/mistradb/mistra_loader.json`; used by `backend/internal/reports/handler_ordini.go` and `backend/internal/panoramica/handler_orders.go`.
- Used by: `apps/reports` order endpoints, `apps/panoramica-cliente` Ordini Ricorrenti e Spot.
- Open questions: none.

### Spot Orders Carry One-Off Amounts In `canone`; MRC Must Be Reclassified As NRC

- Context: any report or app showing NRC/MRC for orders that include spot documents (`TSC-ORDINE`).
- Discovery: for spot orders the ERP stores the one-off amount in the row's `canone` field (with `setup` typically 0), so the conventional `quantita * canone AS mrc` produces a fictitious monthly recurring charge for spot rows.
- Practical rule: when `btrim(tipo_documento) = 'TSC-ORDINE'`, fold the amount into NRC (`setup + COALESCE(quantita * canone, 0)`) and emit `mrc` as NULL (UI shows an empty cell, order totals omit the MRC label). Recurring orders keep `setup` and `quantita * canone` as-is.
- Evidence: Mistra dev DB spot rows (e.g. order `OC/0001116/2026-2026`: `setup = 0`, `canone = 50`, qta 2/28/32) verified 2026-06-10; implemented in `backend/internal/panoramica/handler_orders.go` (orders/detail).
- Used by: `apps/panoramica-cliente` Ordini Ricorrenti e Spot.
- Open questions: `stato_riga` semantics (Da attivare/Attiva/Cessata) are modeled on recurring lifecycles; spot rows inherit them and may show misleading states.

### Alyante Product Translation Write Contract

- Context: server-side sync of product short descriptions from kit-products into Alyante ERP table `MG87_ARTDESC`.
- Discovery: the live Appsmith datasource query updates `MG87_DESCART` and filters with suffixed legacy column names: `MG87_DITTA_CG18`, `MG87_OPZIONE_MG5E`, `MG87_LINGUA_MG52`, `MG87_CODART_MG66`. Earlier backend assumptions using `MG87_DESCRIZIONE`, `MG87_DITTA`, `MG87_OPZIONE`, `MG87_LINGUA`, `MG87_CODART` do not match this environment.
- Practical rule: when writing product short descriptions to Alyante, use `UPDATE MG87_ARTDESC SET MG87_DESCART = ?` with `MG87_DITTA_CG18 = 1`, `MG87_OPZIONE_MG5E = '                    '`, `MG87_LINGUA_MG52 = 'ITA'/'ING'`, and `MG87_CODART_MG66 = code.padEnd(25, ' ')`.
- Evidence: verified Appsmith query `update MG87_ARTDESC set MG87_DESCART = {{this.params.descr}} where MG87_DITTA_CG18 = 1 and MG87_OPZIONE_MG5E = '                    ' and MG87_LINGUA_MG52 = {{this.params.lang}} AND MG87_CODART_MG66 = {{this.params.code}}`; backend adapter in `backend/internal/kitproducts/alyante.go`.
- Used by: `apps/kit-products` product translation sync.
- Open questions: none for this environment; if another Alyante tenant exposes different column names, verify its datasource query before generalizing.

### `common.vocabulary` Is Not Universally Read-Only for Mini-Apps

- Context: mini-apps reading or administering entries in Mistra `common.vocabulary`.
- Discovery: `kit_product_group` entries are admin-managed from `apps/kit-products`, while runtime consumers may intentionally keep reading `common.vocabulary.name`, with translations staying administrative-only for that feature slice. Vocabulary is therefore not a uniformly read-only reference table.
- Practical rule: per feature, decide and document which side owns writes (admin mini-app vs upstream system) and which field consumers actually read; do not assume read-only semantics or translated fields without checking the consuming code path.
- Evidence: `apps/kit-products/src/views/settings/ProductGroupsPage.tsx`, kit-products backend module.
- Used by: `apps/kit-products`; any future app touching `common.vocabulary`.
- Open questions: none.
