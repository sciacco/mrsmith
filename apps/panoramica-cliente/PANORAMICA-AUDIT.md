# Panoramica Cliente - Appsmith Audit

## Application Inventory

| Field | Value |
|-------|-------|
| **App name** | panoramica-cliente |
| **Source type** | Appsmith export (git repo format) |
| **Layout** | Sidebar navigation, FIXED positioning, LARGE max width |
| **Pages** | 8 |
| **Datasources** | 4 |
| **JSObjects** | 5 (across 3 pages) |
| **Theme** | Light color style, sidebar nav |

### Pages

| # | Page | Default | Purpose |
|---|------|---------|---------|
| 1 | Dashboard | Yes | Revenue charts per client: revenue by account, historical billing (K EUR), active services |
| 2 | Ordini ricorrenti | No | Recurring orders with order history path tracking |
| 3 | Ordini Ricorrenti e Spot | No | Combined recurring + spot orders, full order detail (newer/more complete version) |
| 4 | Fatture | No | Invoice line-item browser with period slider |
| 5 | Accessi | No | Connectivity access lines (fiber, DSL, etc.) with status/type filters |
| 6 | IaaS Pay Per Use | No | Cloudstack IaaS consumption: daily/monthly charges, charge breakdown by type |
| 7 | Timoo tenants | No | Timoo PBX tenant management and PBX statistics |
| 8 | Licenze Windows su Cloudstack | No | Windows Server license count chart (last 14 days) |

### Datasources

| Name | Plugin | Type | Used by pages |
|------|--------|------|---------------|
| **mistra** | postgres-plugin | PostgreSQL | Dashboard, Fatture, Accessi, Ordini ricorrenti, Ordini Ric. e Spot |
| **grappa** | mysql-plugin | MySQL | Dashboard (chart_servizi_attivi), IaaS Pay Per Use, Licenze Windows |
| **anisetta** | postgres-plugin | PostgreSQL | Timoo tenants |
| **TIMOO API** | restapi-plugin | REST API | Timoo tenants |

### JSObjects

| Page | JSObject | Methods | Purpose |
|------|----------|---------|---------|
| Dashboard | `utils` | `generaTutto()`, `myFun1()` | Date variables, trigger all chart regeneration |
| Dashboard | `echart_ricavi` | `Rigenera()` | Pie chart: revenue by account (conto ricavo) |
| Dashboard | `echart_fatturato` | `Rigenera()` | Bar chart: historical billing in K EUR |
| Dashboard | `echart_servizi` | `Rigenera()` | Bar chart: active services count |
| IaaS PPU | `utils` | `aggiornaSerie()` | Transform charge-by-type data into chart series |
| Timoo | `utils` | `listaTenants()`, `pbxStats()`, `generaTenantIdList()` | Build tenant list URL, aggregate PBX statistics |

---

## Page Audits

### 1. Dashboard (default page)

**Purpose:** Multi-chart customer overview showing revenue breakdown, historical billing, and active services for a selected customer.

**Widgets:**
| Widget | Type | Role |
|--------|------|------|
| `s_f_clienti` | SELECT | Customer selector (searchable) |
| `d_dal` | DATE_PICKER | "From" date filter (default: Jan 1 of previous year) |
| `d_al` | DATE_PICKER | "To" date filter (default: hardcoded Feb 26, 2024) |
| `Button1` | BUTTON | "GO" - triggers `utils.generaTutto()` |
| `Chart1` | CHART (ECharts) | Revenue by conto ricavo (pie/donut chart) |
| `Chart2` | CHART (ECharts) | Historical billing in K EUR (horizontal bar) |
| `Chart3` | CHART (ECharts) | Active services (bar chart, log scale) |

**Queries:**
| Query | DB | On Load | Purpose |
|-------|-----|---------|---------|
| `get_clienti_con_fatture` | mistra | Yes | Populate customer dropdown from `loader.erp_clienti_con_fatture` |
| `chart_conto_ricavi` | mistra | No | Revenue grouped by `desc_conto_ricavo` within date range |
| `chart_fatturato` | mistra | No | Yearly billing total (sum * segno) from `loader.v_erp_fatture_nc` |
| `chart_servizi_attivi` | grappa | No | Active services from `v_analytics_servizi_attivi_aggregati` |

**Event flow:**
1. Page loads -> `get_clienti_con_fatture` runs -> populates `s_f_clienti`
2. User selects customer, picks date range, clicks "GO"
3. Button triggers `utils.generaTutto()` which calls all three `Rigenera()` methods
4. Each `Rigenera()` runs its SQL query, then maps results into ECharts option data

**Hidden logic / business rules:**
- **Cross-database join:** `chart_servizi_attivi` uses `codice_aggancio_gest` to map Mistra customer ID to Grappa `cli_fatturazione.id` (via subselect)
- **Revenue sign handling:** `prezzo_totale_netto * segno` — the `segno` column carries +1/-1 to handle invoices vs credit notes
- **Division by 1000:** `chart_fatturato` divides by 1000 for K EUR display
- **Default date bug:** `d_al` has a hardcoded default `2024-02-26T06:39:40.581Z` instead of dynamic "today"
- **ECharts `originale` variable:** Dead code left from template, never used

**Migration notes:**
- Three separate ECharts configurations managed via JSObject state mutation pattern (`this.option.series[0].data = ...`) — in React, these become chart component props
- Customer selector populated from a materialized/staging view (`loader.erp_clienti_con_fatture`), not the canonical customer table

---

### 2. Ordini ricorrenti

**Purpose:** Browse recurring orders with customer/status filters and order history chain.

**Widgets:**
| Widget | Type | Role |
|--------|------|------|
| `s_o_cliente` | SELECT | Customer selector (includes "TUTTI I CLIENTI" option with value -1) |
| `ms_o_stati` | MULTI_SELECT | Order status filter (default: "Evaso", "Confermato") |
| `Checkbox1` | CHECKBOX | "Righe espanse" — unclear effect (no `onCheckChange` handler) |
| `Button1` | BUTTON | "Cerca" — runs `get_ordini_ricorrenti` |
| `Table1` | TABLE_V2 | Results table |

**Queries:**
| Query | DB | On Load | Purpose |
|-------|-----|---------|---------|
| `get_aziende_con_ordini` | mistra | Yes | Active customers with recurring orders (excludes dismissed) |
| `get_stati_ordine` | mistra | Yes | Distinct order statuses for filter |
| `get_ordini_ricorrenti` | mistra | No | Main order data from `loader.v_ordini_sintesi` |

**Hidden logic:**
- **"All clients" pattern:** Query uses `s_o_cliente.selectedOptionValue === '' || ... == -1 ? 'true' : " numero_azienda = '..."` — builds dynamic SQL WHERE clause client-side
- **SQL injection risk:** The query uses `pluginSpecifiedTemplates: [{value: false}]` (prepared statements OFF) with string concatenation
- **Order history chain:** Calls `loader.get_reverse_order_history_path(nome_testata_ordine)` — a PostgreSQL function that traces order substitution chains
- **Dismissed customer exclusion:** Filters by `data_dismissione >= NOW() OR data_dismissione='0001-01-01' OR IS NULL`
- **Checkbox1 "Righe espanse":** Has no bound action — likely meant to toggle row expansion or table column visibility, but the handler is empty (`{{}}`)

**Migration notes:**
- The `v_ordini_sintesi` view and `get_reverse_order_history_path()` function are DB-side abstractions that should be preserved
- Extended timeout (20s) suggests heavy query

---

### 3. Ordini Ricorrenti e Spot

**Purpose:** More detailed order viewer combining recurring and spot orders. Shows the full order lifecycle including referents, payment terms, product families, and computed `stato_riga`.

**Widgets:**
| Widget | Type | Role |
|--------|------|------|
| `Text1` | TEXT | Greeting: "Hello {{appsmith.user.name}}" |
| `s_clienti` | SELECT | Customer selector |
| `ms_stati` | MULTI_SELECT | Order status filter (default: "Evaso", "Confermato") |
| `Button1` | BUTTON | "GO" — runs `GET_ordini_Ric_Spot` |
| `Table1` | TABLE_V2 | Full detail order table (2345 lines of column config) |

**Queries:**
| Query | DB | On Load | Purpose |
|-------|-----|---------|---------|
| `GET_aziendeConOrdini` | mistra | Yes | Active customers (slightly different dismissal filter — no IS NULL check) |
| `GET_StatiOrdine` | mistra | Yes | Distinct order statuses |
| `GET_ordini_Ric_Spot` | mistra | No | Massive join across `erp_ordini`, `erp_righe_ordini`, `erp_anagrafiche_clienti`, `erp_anagrafica_articoli_vendita` |
| `get_ordini_ricorrenti` | mistra | No | Same as page 2 but scoped to selected client (no "ALL" option). Appears unused — possibly vestigial |

**Hidden logic / business rules:**
- **`stato_riga` computation (CRITICAL BUSINESS RULE):**
  ```
  Cessato order -> 'Cessata'
  Bloccato order -> 'Bloccata'
  Confermato + year(data_attivazione)=1 -> 'Da attivare'
  Confermato + year(data_attivazione)>1 -> 'Attiva'
  annullato=1 -> 'Annullata'
  year(data_cessazione)=1 -> 'Attiva'
  data_cessazione <= now() -> 'Cessata'
  data_cessazione > now() -> 'Cessazione richiesta'
  else -> 'Unknown'
  ```
  This is a key business rule embedded in SQL. Year=1 is a sentinel for "not set" (0001-01-01).
- **`data_ordine` computation:** Uses `MAX(data_conferma, data_documento)` as the effective order date
- **`descrizione_long` concatenation:** Combines `descrizione_prodotto` + `descrizione_estesa` with CR/LF only when they differ
- **`ORDINE` column:** Shows order name only on `progressivo_riga = 1` (first row), NULL otherwise — visual grouping trick
- **Product exclusion:** Filters out `codice_prodotto = 'CDL-AUTO'`
- **`intestazione_ordine`:** Computed display string: "ORD-NAME del YYYY-MM-DD (STATUS)"
- **NULLIF sentinel dates:** All `0001-01-01` dates converted to NULL for display

**Migration notes:**
- This page duplicates much of "Ordini ricorrenti" with more detail — likely the newer version
- The `stato_riga` CASE logic and `intestazione_ordine` formatting MUST be preserved in backend
- `GET_aziendeConOrdini` has a subtle difference from page 2: no `OR data_dismissione IS NULL` check — potential bug or intentional

---

### 4. Fatture

**Purpose:** Invoice line-item browser for a selected customer within a configurable time period.

**Widgets:**
| Widget | Type | Role |
|--------|------|------|
| `s_f_clienti` | SELECT | Customer selector (triggers `get_fatture` on change) |
| `cs_periodo` | CATEGORY_SLIDER | Period filter: 6, 12, 24, 36 months or "all" (2000 months). Triggers `get_fatture` on change |
| `Table1` | TABLE_V2 | Invoice detail table with 18 columns |

**Queries:**
| Query | DB | On Load | Purpose |
|-------|-----|---------|---------|
| `get_clienti_con_fatture` | mistra | Yes | Same as Dashboard |
| `get_fatture` | mistra | No | Invoice lines from `loader.v_erp_fatture_nc` |

**Hidden logic:**
- **Document grouping:** `CASE WHEN rn = 1 THEN doc || ' ' || num_documento || CHR(13) || CHR(10) || to_char(data_documento, '(YYYY-MM-DD)') ELSE NULL END AS documento` — shows document header only on first row
- **Period as interval:** `data_documento >= current_date - interval '{{cs_periodo.value}} months'` — "all" uses value 2000
- **Prepared statements OFF** (`pluginSpecifiedTemplates: [{value: false}]`) — SQL injection risk for the interval parameter
- **Currency formatting:** `prezzo_unitario` and `prezzo_totale_netto` display as EUR with 2 decimals
- **Hidden columns:** `data_documento`, `num_documento`, `id_cliente`, `progressivo_riga`, `rn` are `isVisible: false`
- **Auto-refresh on selection:** Both `s_f_clienti.onOptionChange` and `cs_periodo.onChange` trigger `get_fatture.run()` — no explicit "GO" button needed

**Migration notes:**
- The "2000 months" hack for "all" should become a proper "no date filter" option
- `rn` (row number) used for visual grouping is a presentation concern that should stay in frontend

---

### 5. Accessi

**Purpose:** Browse connectivity access lines (fiber, DSL, etc.) with multi-select filters for client, line status, and connection type.

**Widgets:**
| Widget | Type | Role |
|--------|------|------|
| `ms_clienti` | MULTI_SELECT | Multi-client selector (from Grappa `cli_fatturazione`) |
| `ms_stato` | MULTI_SELECT | Line status filter (hardcoded: Attiva, Cessata, da attivare, in attivazione, KO). Default: "Attiva" |
| `ms_tipo_conn` | MULTI_SELECT | Connection type filter (dynamic from DB, all selected by default) |
| `IconButton1` | ICON_BUTTON | Confirm/GO — runs `get_accessi_cliente` |
| `Table1` | TABLE_V2 | Access lines table with 16 columns |

**Queries:**
| Query | DB | On Load | Purpose |
|-------|-----|---------|---------|
| `get_clients_accessi` | mistra | Yes | Active clients with access lines (from Grappa tables replicated to Mistra `loader.`) |
| `get_tipo_conn` | mistra | Yes | Distinct connection types from `loader.grappa_foglio_linee` |
| `get_accessi_cliente` | mistra | Yes | Complex join: `grappa_foglio_linee` + `grappa_cli_fatturazione` + `grappa_profili` + `v_ordini_ricorrenti` |

**Hidden logic:**
- **Cross-system join in SQL:** The main query joins Grappa-origin tables (`loader.grappa_foglio_linee`, `loader.grappa_cli_fatturazione`, `loader.grappa_profili`) with ERP-origin data (`loader.v_ordini_ricorrenti`) via `serialnumber` and `codice_aggancio_gest`
- **Latest order row:** `ROW_NUMBER() OVER(PARTITION BY serialnumber ORDER BY data_documento DESC, progressivo_riga) AS rn` with `rn = 1` filter — gets the most recent order for each serial
- **SQL injection risk:** Multi-select values concatenated with `.map(i => "'" + i + "'").join()` and prepared statements OFF
- **Customer ID mapping:** `cf.codice_aggancio_gest AS id_anagrafica` — exposed as "Id Alyante" in table
- **Status hardcoded list:** `ms_stato` uses a static JSON array, not fetched from DB — may drift from actual data
- **Runs on page load:** `get_accessi_cliente` has `executeOnLoad: true` but depends on `ms_clienti` which defaults to `[]` — may return no results on first load

**Migration notes:**
- This page uses `loader.grappa_*` tables — data replicated from Grappa MySQL to Mistra PostgreSQL. Determine whether the rewrite should query Grappa directly or keep using the loader copies
- The `v_ordini_ricorrenti` LEFT JOIN adds order context to each access line — this is a valuable cross-domain join

---

### 6. IaaS Pay Per Use

**Purpose:** Cloudstack IaaS consumption dashboard with daily/monthly charge views and charge-type breakdown.

**Widgets:**
| Widget | Type | Role |
|--------|------|------|
| `tbl_accounts` | TABLE_V2 | Account selector table (from `cdl_accounts` + `cli_fatturazione`) |
| `Tabs1` | TABS | Contains daily/monthly views |
| `tbl_giornalieri` | TABLE_V2 | Daily charges table (inside Tabs1) |
| `chart_giorno` | CHART | Daily charge breakdown by type (pie chart from `utils.daySeries`) |
| `chart_mensili` | CHART | Monthly charges (bar chart from `get_monthly_charges`) |

**Queries (all Grappa/MySQL):**
| Query | DB | On Load | Purpose |
|-------|-----|---------|---------|
| `get_cdl_accounts` | grappa | Yes | Active billable IaaS accounts (excludes `codice_aggancio_gest` 385, 485) |
| `get_daily_charges` | grappa | Yes | Last 120 days of daily charges for selected account |
| `get_monthly_charges` | grappa | Yes | Last 12 months aggregated charges |
| `get_charges_by_type` | grappa | Yes | Charge breakdown by usage type for selected day |

**Hidden logic:**
- **Usage type codes (BUSINESS RULE):**
  - 1 = Running VM, 2 = Allocated VM, 3 = IP Charge, 6 = Volume
  - 7 = Template, 8 = ISO, 9 = Snapshot, 26 = Volume Secondary
  - 27 = VM Snapshot on Primary, 9998 = Windows licenses, 9999 = Credit
- **Credit tracking:** `utCredit` (type 9999) is separated in daily view for visibility
- **Account exclusion:** `codice_aggancio_gest not in (385, 485)` — internal/test accounts
- **`utils.aggiornaSerie()`:** Dynamically builds pie chart series from non-zero charge types — only shows types with actual charges
- **Cascading selection:** `tbl_accounts.selectedRow.cloudstack_domain` drives `get_daily_charges` and `get_monthly_charges`; `tbl_giornalieri.selectedRow.giorno` drives `get_charges_by_type`

**Migration notes:**
- All queries hit Grappa MySQL directly (not loader copies)
- The usage_type code mapping should become a backend enum/constant
- The `cdl_accounts` + `cli_fatturazione` join uses Grappa's internal IDs

---

### 7. Timoo tenants

**Purpose:** Timoo PBX management — list tenants, select one, view PBX instances and user/extension counts.

**Widgets:**
| Widget | Type | Role |
|--------|------|------|
| `sl_tenant` | SELECT | Tenant selector (from `utils.tenants`) |
| `Button1` | BUTTON | Triggers `utils.pbxStats()` |
| `Table1` | TABLE_V2 | PBX statistics table (from `utils.statistiche`) |
| `Text1` | TEXT | Shows total users/extensions |

**Queries:**
| Query | DB/API | On Load | Purpose |
|-------|--------|---------|---------|
| `getAnisettaTenants` | anisetta | Yes | All tenants from `public.as7_tenants` |
| `getPlaceholder` | TIMOO API | Yes | Generic REST proxy — path passed as parameter |
| `getPbxByTenandId` | anisetta | No | PBX accounting data grouped by tenant |
| `get_pbx_by_tenant` | TIMOO API | No | REST call to TIMOO API `/orgUnits` (used in commented-out code) |
| `getTenantsTest` | TIMOO API | No | Test query — unused |
| `varie` | TIMOO API | No | Test query for addresses — unused |

**Hidden logic:**
- **Two data sources for PBX data:** The JSObject has both a DB path (`getPbxByTenandId` from `as7_pbx_accounting`) and a REST API path (`get_pbx_by_tenant` + address counting). The REST path is commented out in favor of the DB path — the DB approach is faster but may be stale
- **Dynamic REST URL construction:** `generaTenantIdList()` builds a URL like `/orgUnits?where=type.eq('tenant').and(id.in(5,571)).and(name.ne('KlajdiandCo'))` from DB tenant IDs — using Timoo's custom query language
- **`getPlaceholder` as generic proxy:** Accepts a `URL` parameter via `this.params.URL` — effectively a pass-through to the TIMOO API. This is a reusable but fragile pattern
- **Excluded tenant:** Hardcoded `name.ne('KlajdiandCo')` in the URL — test/internal tenant
- **Statistics aggregation:** `pbxStats()` sums users and service_extensions across PBX instances
- **`executeOnLoad: true` on `utils.listaTenants`:** Automatically loads tenant list on page open

**Migration notes:**
- Anisetta DB schema (`public.as7_tenants`, `public.as7_pbx_accounting`) needs documentation
- The TIMOO API datasource URL is not visible in the export — needs to be obtained from the running Appsmith instance
- The commented-out REST API approach with per-PBX address counting was replaced by DB aggregation — document which is authoritative

---

### 8. Licenze Windows su Cloudstack

**Purpose:** Simple chart showing Windows Server license count over the last 14 days.

**Widgets:**
| Widget | Type | Role |
|--------|------|------|
| `Text1` | TEXT | Page title: "Licenze Windows Server attive su Cloudstack PPU" |
| `Chart1` | CHART | Line/bar chart of daily license counts |

**Queries:**
| Query | DB | On Load | Purpose |
|-------|-----|---------|---------|
| `get_licenses_by_day` | grappa | Yes | Count of `usage_type = 9998` records per day (last 14 days) |

**Hidden logic:**
- **Usage type 9998:** Windows license charges (distinct from 9999 = credits)
- Query output aliased as `x`, `y` for direct chart consumption

**Migration notes:**
- Simplest page — single query, single chart, no user interaction
- Could be merged into the IaaS PPU page as a tab

---

## Datasource & Query Catalog

### Mistra PostgreSQL (loader schema)

| Query | Page(s) | Table/View | Read/Write | Rewrite recommendation |
|-------|---------|------------|------------|----------------------|
| `get_clienti_con_fatture` | Dashboard, Fatture | `loader.erp_clienti_con_fatture` | Read | Backend API: `/api/panoramica/clienti` |
| `chart_fatturato` | Dashboard | `loader.v_erp_fatture_nc` | Read | Backend API: `/api/panoramica/fatturato-storico?cliente=X` |
| `chart_conto_ricavi` | Dashboard | `loader.v_erp_fatture_nc` | Read | Backend API: `/api/panoramica/ricavi-per-conto?cliente=X&dal=&al=` |
| `get_fatture` | Fatture | `loader.v_erp_fatture_nc` | Read | Backend API: `/api/panoramica/fatture?cliente=X&mesi=N` |
| `get_aziende_con_ordini` | Ordini ric. | `loader.v_ordini_ricorrenti` + `loader.erp_anagrafiche_clienti` | Read | Backend API: `/api/panoramica/clienti-con-ordini` |
| `get_stati_ordine` / `GET_StatiOrdine` | Ordini (both) | `loader.v_ordini_ricorrenti` | Read | Backend API: `/api/panoramica/stati-ordine` |
| `get_ordini_ricorrenti` | Ordini ric. | `loader.v_ordini_sintesi` | Read | Backend API: `/api/panoramica/ordini?cliente=X&stati=...` |
| `GET_ordini_Ric_Spot` | Ordini R&S | `loader.erp_ordini` + `erp_righe_ordini` + `erp_anagrafiche_clienti` + `erp_anagrafica_articoli_vendita` | Read | Backend API: `/api/panoramica/ordini-dettaglio?cliente=X&stati=...` |
| `get_clients_accessi` | Accessi | `loader.grappa_foglio_linee` + `grappa_cli_fatturazione` | Read | Backend API: `/api/panoramica/clienti-accessi` |
| `get_tipo_conn` | Accessi | `loader.grappa_foglio_linee` | Read | Backend API: `/api/panoramica/tipi-connettivita` |
| `get_accessi_cliente` | Accessi | Multi-table join (see page audit) | Read | Backend API: `/api/panoramica/accessi?clienti=...&stati=...&tipi=...` |

### Grappa MySQL

| Query | Page(s) | Table/View | Read/Write | Rewrite recommendation |
|-------|---------|------------|------------|----------------------|
| `chart_servizi_attivi` | Dashboard | `v_analytics_servizi_attivi_aggregati` + `cli_fatturazione` | Read | Backend API: `/api/panoramica/servizi-attivi?cliente=X` |
| `get_cdl_accounts` | IaaS PPU | `cdl_accounts` + `cli_fatturazione` | Read | Backend API: `/api/panoramica/iaas/accounts` |
| `get_daily_charges` | IaaS PPU | `cdl_charges` | Read | Backend API: `/api/panoramica/iaas/charges-giornalieri?domain=X` |
| `get_monthly_charges` | IaaS PPU | `cdl_charges` | Read | Backend API: `/api/panoramica/iaas/charges-mensili?domain=X` |
| `get_charges_by_type` | IaaS PPU | `cdl_charges` | Read | Backend API: `/api/panoramica/iaas/charges-per-tipo?domain=X&giorno=Y` |
| `get_licenses_by_day` | Licenze Win | `cdl_charges` | Read | Backend API: `/api/panoramica/iaas/licenze-windows` |

### Anisetta PostgreSQL

| Query | Page(s) | Table/View | Read/Write | Rewrite recommendation |
|-------|---------|------------|------------|----------------------|
| `getAnisettaTenants` | Timoo | `public.as7_tenants` | Read | Backend API: `/api/panoramica/timoo/tenants` |
| `getPbxByTenandId` | Timoo | `public.as7_pbx_accounting` | Read | Backend API: `/api/panoramica/timoo/pbx?tenant=X` |

### TIMOO REST API

| Query | Page(s) | Endpoint | Read/Write | Rewrite recommendation |
|-------|---------|----------|------------|----------------------|
| `getPlaceholder` | Timoo | Dynamic: `/{this.params.URL}` | Read | Backend proxy or direct API client |
| `get_pbx_by_tenant` | Timoo | `/orgUnits?where=...` | Read | Currently commented out — use DB approach |
| `getTenantsTest` | Timoo | `/orgUnits` (test) | Read | Remove |
| `varie` | Timoo | `/addresses` (test) | Read | Remove |

---

## Findings Summary

### Embedded Business Rules

| # | Rule | Location | Classification |
|---|------|----------|---------------|
| 1 | **Revenue sign handling:** `prezzo_totale_netto * segno` for invoice/credit note distinction | Dashboard, Fatture queries | Business logic |
| 2 | **`stato_riga` computation:** 8-way CASE determining order line status from order status, activation/cessation dates, annullato flag | `GET_ordini_Ric_Spot` query | **Critical business logic** — must be preserved exactly |
| 3 | **Dismissed customer exclusion:** `data_dismissione >= NOW() OR = '0001-01-01' OR IS NULL` | `get_aziende_con_ordini` | Business logic |
| 4 | **Sentinel date handling:** `0001-01-01 00:00:00` treated as "not set" throughout | Multiple Ordini queries | Business logic |
| 5 | **CDL-AUTO exclusion:** `codice_prodotto <> 'CDL-AUTO'` | `GET_ordini_Ric_Spot` | Business rule |
| 6 | **Internal account exclusion:** `codice_aggancio_gest not in (385, 485)` | `get_cdl_accounts` | Business rule |
| 7 | **Usage type codes:** 1-9, 26, 27, 9998, 9999 mapping to charge categories | IaaS PPU queries | Business logic |
| 8 | **Line status list:** Attiva, Cessata, da attivare, in attivazione, KO | Accessi `ms_stato` | Business rule (hardcoded) |
| 9 | **Order history chain:** `loader.get_reverse_order_history_path()` DB function | Ordini ricorrenti | Business logic (DB-side) |
| 10 | **Excluded Timoo tenant:** `name.ne('KlajdiandCo')` | Timoo JSObject | Business rule (hardcoded) |

### Duplication

| # | Finding | Pages affected |
|---|---------|---------------|
| 1 | `get_clienti_con_fatture` query duplicated identically | Dashboard, Fatture |
| 2 | `get_aziende_con_ordini` / `GET_aziendeConOrdini` — near-identical with subtle IS NULL difference | Ordini ric., Ordini R&S |
| 3 | `get_stati_ordine` / `GET_StatiOrdine` — identical | Ordini ric., Ordini R&S |
| 4 | `get_ordini_ricorrenti` duplicated (once with "ALL" support, once without) | Ordini ric., Ordini R&S |
| 5 | "Ordini ricorrenti" and "Ordini Ricorrenti e Spot" are largely overlapping pages | Both Ordini pages |
| 6 | Customer select pattern (`sourceData` mapping) repeated on 5+ pages | All pages with client selector |

### Security Concerns

| # | Finding | Severity | Location |
|---|---------|----------|----------|
| 1 | **SQL injection via prepared statements OFF** — user-controlled values in `get_fatture`, `get_ordini_ricorrenti`, `GET_ordini_Ric_Spot`, `get_accessi_cliente` | High | Multiple pages |
| 2 | **Direct database access from UI** — all 4 databases queried directly from browser | High | All pages |
| 3 | **No row-level authorization** — any logged-in user can query any customer's data | Medium | All pages |
| 4 | **Hardcoded exclusions instead of RBAC** — internal accounts excluded by ID, not by role | Low | IaaS PPU |

### Migration Blockers

| # | Blocker | Impact | Resolution |
|---|---------|--------|------------|
| 1 | **TIMOO API base URL unknown** — not in export | Cannot replicate Timoo page without it | Extract from running Appsmith instance |
| 2 | **Anisetta schema undocumented** — `as7_tenants`, `as7_pbx_accounting` not in `docs/` | Cannot verify Timoo queries | Create anisetta schema dump |
| 3 | **`loader.v_ordini_sintesi` definition unknown** — referenced but not in schema docs | Need to verify column availability | Inspect view definition in Mistra DB |
| 4 | **`loader.get_reverse_order_history_path()` function** — DB-side logic needs extraction | Order history feature depends on it | Document function logic for backend reimplementation |
| 5 | **`v_analytics_servizi_attivi_aggregati` (Grappa)** — view definition unknown | Dashboard services chart | Inspect view in Grappa DB |

### Candidate Domain Entities

Based on the audit, the following domain entities emerge:

1. **Customer** (`erp_anagrafiche_clienti` / `cli_fatturazione`) — central entity, mapped across systems via `codice_aggancio_gest`
2. **Invoice/CreditNote** (`v_erp_fatture_nc`) — with line items, accounts, and sign handling
3. **RecurringOrder** (`erp_ordini` + `erp_righe_ordini` + `v_ordini_sintesi`) — with status lifecycle and substitution chains
4. **AccessLine** (`grappa_foglio_linee`) — connectivity lines with profiles and statuses
5. **IaaSAccount** (`cdl_accounts`) — Cloudstack billing accounts
6. **IaaSCharge** (`cdl_charges`) — daily usage charges by type
7. **TimooTenant** (`as7_tenants`) — PBX customer tenants
8. **PBXInstance** (`as7_pbx_accounting`) — PBX instances with user/extension counts
9. **Product** (`erp_anagrafica_articoli_vendita`) — with family, sub-family, revenue account

### Recommended Next Steps

1. **Merge duplicate pages:** Consolidate "Ordini ricorrenti" and "Ordini Ricorrenti e Spot" into one
2. **Extract business rules to backend:** All SQL queries should become Go API endpoints with proper parameterization
3. **Document missing schemas:** Anisetta, `loader` views, Grappa views
4. **Eliminate SQL injection:** All queries must use parameterized statements in the backend
5. **Add authorization:** Row-level customer access control
6. **Obtain TIMOO API details:** Base URL, auth mechanism, rate limits
7. **Resolve data freshness strategy:** Decide whether to query Grappa directly or via Mistra `loader.*` copies
8. **Hand off to `appsmith-migration-spec`** for Phase 2 specification
