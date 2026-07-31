# Reports

Knowledge entries specific to `apps/reports`.
Part of the Implementation Knowledge Handbook — see [docs/IMPLEMENTATION-KNOWLEDGE.md](../IMPLEMENTATION-KNOWLEDGE.md) for the index, entry format, and placement rules.

### Reports AOV Replacement MRC Matching

- Context: AOV calculations in `apps/reports` that subtract replaced-order MRC for substitution orders (`tipo_ordine = 'A'`) from `loader.v_ordini_ric_spot`.
- Discovery: Appsmith's AOV replacement lookup treats `sost_ord` as a semicolon-separated list of replaced order names, normalizes order names by replacing `/` with `-`, and counts only replaced rows whose `data_disdetta` equals the replacing order's `data_conferma`. When a substitution has no matching replaced-order rows, legacy AOV leaves old MRC, net MRC, and AOV as `NULL`; do not coalesce that case to `0`.
- Practical rule: AOV old-MRC subqueries should match `REPLACE(odv.nome_testata_ordine, '/', '-')` against `string_to_array(REPLACE(o.sost_ord, '/', '-'), ';')`, keep `odv.annullato = 0`, and require `odv.data_disdetta = o.data_conferma`. Include `o.data_disdetta` in inner grouping where existing grouped queries require it. For by-category AOV, compute economics at order level using the same `totale_mrc_new`, `totale_nrc`, and `valore_aov` semantics as detail/by-type/by-sales, then assign each order to one current-order product category using the largest current-row AOV contribution. Do not subtract replaced-order rows into their old product categories, because that creates negative category-only rows and makes category counts fan out beyond the detail order count.
- Evidence: Appsmith AOV diff `artifacts/ad4f4244a1c1b59d16b4fe0982d37faa1c639ef0.diff`; backend implementation and tests in `backend/internal/reports/handler_aov.go` and `backend/internal/reports/handler_quantita_test.go`.
- Used by: `apps/reports` AOV detail, by-type, by-category, and by-sales datasets.
- Open questions: none.

### Reports AOV CDL-CLOUD Adds Fixed NRC

- Context: AOV calculations in `apps/reports` over `loader.v_ordini_ric_spot`.
- Discovery: each sales-order row with `codice_prodotto = 'CDL-CLOUD'` contributes an extra fixed 600 euro NRC value. The amount is per matching row, not multiplied by `quantita`.
- Practical rule: include the fixed amount in both `totale_nrc` and `valore_aov` for AOV detail and aggregate views. The detail payload should expose a boolean flag so the UI can mark affected orders with a warning dot.
- Evidence: business rule provided during AOV update; backend implementation in `backend/internal/reports/handler_aov.go`.
- Used by: `apps/reports` AOV detail, by-type, by-category, and by-sales datasets.
- Open questions: none.

### Reports Carbone Export Payloads May Need Template-Specific Key Aliases

- Context: XLSX exports in `backend/internal/reports` rendered through Carbone templates.
- Discovery: Carbone export payload keys do not have to match the preview API contract exactly. `Accessi attivi` preview still exposes `stato`, but the XLSX template expects Grappa-specific aliases, so the backend now rewrites the export payload to emit `stato grappa` and `stato_grappa` instead of `stato`.
- Practical rule: when a Carbone template is already pinned to legacy field names, adapt the backend export payload in the export path only; do not widen or rename the preview API/frontend contract unless the UI actually needs the new keys too.
- Evidence: `backend/internal/reports/handler_accessi.go` `activeLinesExportRows`, `backend/internal/reports/handler_quantita_test.go`, reports template references `AccessiTemplateID = a482f92419a0c17bb9bfae00c64d251c6a527f95c67993d86bf2d11d9e2e7a9e`.
- Used by: `apps/reports` `Accessi attivi` XLSX export.
- Open questions: none.
