# Reports — Application Specification

## Summary

| Field | Value |
|---|---|
| Application name | Reports |
| Audit source | `apps/reports/APPSMITH-AUDIT.md` |
| Spec status | Complete (V1 scope) |
| Migration strategy | 1:1 from Appsmith, coexistence period |
| Phase B UX report | `apps/reports/reports-migspec-phaseB-ux.md` |
| Phase C logic placement | `apps/reports/reports-migspec-phaseC.md` |
| Phase D data flow | `apps/reports/reports-migspec-phaseD.md` |

### Scope boundaries

- **In scope V1**: 7 report pages (Ordini, Accessi attivi, Attivazioni in corso, Rinnovi in arrivo, Anomalie MOR Tab 1, Accounting TIMOO, AOV) + Home hub
- **Out of scope V1**: AI analysis (Anomalie MOR Tab 2, OpenRouter), AOV query consolidation, AOV area bug fix
- **Deferred items tracked in**: `docs/TODO.md` (3 entries under "Reports App")

---

## Current-State Evidence

### Source pages/views

| # | Appsmith page | New app route | Pattern |
|---|---|---|---|
| 1 | Home | `/` | Card hub (report links with icons) |
| 2 | Ordini | `/ordini` | Filter → Preview → Export XLSX |
| 3 | Accessi attivi | `/accessi-attivi` | Filter → Preview → Export XLSX |
| 4 | Attivazioni in corso | `/attivazioni-in-corso` | Master-detail (auto-load, no filters) |
| 5 | Rinnovi in arrivo | `/rinnovi-in-arrivo` | Master-detail with filters |
| 6 | Anomalie MOR...tacci | `/anomalie-mor` | Auto-load enriched table (V1: Tab 1 only) |
| 7 | Accounting TIMOO daily | `/accounting-timoo` | Auto-load single table |
| 8 | AOV | `/aov` | Filter → Stat cards + Tabbed tables → Export XLSX |

### Source entities (flat, as in current views)

Order, Customer, Access Line, Bandwidth Profile (derived attribute), Billing Customer (Grappa bridge), Phone Billing Record, Sales Rep (HubSpot), TIMOO Tenant.

No entity separation or abstraction in V1 — all queries use the same flat views as Appsmith.

### Source integrations and datasources

| Datasource | Type | V1 status |
|---|---|---|
| mistra (PostgreSQL) | Primary DB | Active — 6/7 pages |
| grappa (MySQL) | Secondary DB | Active — Anomalie MOR only |
| anisetta (PostgreSQL) | Tertiary DB | Active — Accounting TIMOO only |
| carbone.io (REST API) | XLSX rendering | Active — same template IDs as constants |
| TIMOO API (REST) | Tenant names | **Eliminated** — `as7_tenants.name` now in DB |
| openrouter (REST API) | AI analysis | **Deferred** — Phase 2 |

### Known audit gaps

- MB1: AOV `get_report_data_area` inconsistency — replicated as-is, tracked in TODO
- MB2: DB view DDLs — resolved, present in `docs/mistradb/mistra_loader.json`
- MB3: Carbone.io templates — preserved, same IDs as constants
- MB6: Missing pages in git export — resolved, single-file JSON covers all 8

---

## Entity Catalog

### Entity: Order (flat, includes line items)

- **Purpose**: Central entity across Ordini, Attivazioni, Rinnovi, AOV pages
- **Source views**: `loader.v_ordini_ric_spot`, `loader.v_ordini_sintesi`, `loader.v_ordini_ricorrenti`, `loader.v_ordini_ricorrenti_conrinnovo`
- **Operations**: read (list filtered, detail, aggregated), export XLSX
- **Fields**: `nome_testata_ordine`, `tipo_ordine` (N/A/R/C), `stato_ordine`, `stato_riga`, `descrizione_long`, `quantita`, `canone` (MRC), `setup` (NRC), `serialnumber`, `data_ordine`, `data_conferma`, `data_documento`, `data_attivazione`, `data_cessazione`, `data_ultima_fatt`, `durata_servizio`, `durata_rinnovo`, `tacito_rinnovo`, `sost_ord`, `sostituito_da`, `progressivo_riga`, `note_legali`, `metodo_pagamento`, `codice_prodotto`, `codice_ordine`, `numero_azienda`
- **Relationships**: belongs to Customer (`numero_azienda`), has substitution chain (`sost_ord`/`sostituito_da`), has order history (`get_reverse_order_history_path`)
- **Notes**: 4 different views used by different pages. Order and Order Line are treated as flat (no separation).

### Entity: Customer

- **Purpose**: Supporting entity — always joined for `ragione_sociale`
- **Source**: `loader.erp_anagrafiche_clienti`
- **Operations**: read (join only)
- **Fields**: `ragione_sociale`, `numero_azienda`
- **Relationships**: has many Orders

### Entity: Billing Customer (Grappa bridge)

- **Purpose**: Bridge between Grappa and Mistra customer identifiers
- **Source**: `grappa.cli_fatturazione` (MySQL direct), `loader.grappa_cli_fatturazione` (Mistra copy)
- **Operations**: read (join only)
- **Fields**: `id`, `intestazione`, `codice_aggancio_gest`
- **Key mapping**: `codice_aggancio_gest` = `numero_azienda` (ERP)

### Entity: Access Line

- **Purpose**: Primary entity for Accessi attivi page
- **Source**: `loader.grappa_foglio_linee`
- **Operations**: read (list filtered by type/status)
- **Fields**: `tipo_conn`, `fornitore`, `provincia`, `comune`, `cogn_rsoc_intest_linea`, `stato`, `id`, `codice_ordine`, `serialnumber`, `id_anagrafica`, `id_profilo`
- **Derived attribute**: bandwidth classification (CONDIVISA/DEDICATA) from `grappa_profili`
- **Relationships**: belongs to Billing Customer, has Bandwidth Profile, matched to latest Order via serialnumber

### Entity: Phone Billing Record

- **Purpose**: Primary entity for Anomalie MOR page
- **Source**: `grappa.importi_telefonici`, `grappa.conti_telefonici`
- **Operations**: read (latest period), cross-reference with ERP orders (server-side)
- **Fields**: `conto`, `lastname`, `firstname`, `is_da_fatturare`, `codice_ordine`, `serialnumber`, `periodo_inizio`, `importo`, `stato`, `tipologia`, `id_cliente`
- **Enrichment**: `ordine_presente` (SI/NO), `numero_ordine_corretto` (SI/NO) — computed server-side by matching `serialnumber`

### Entity: Sales Rep (HubSpot)

- **Purpose**: Lookup for AOV page
- **Source**: `loader.hubs_deal`, `loader.hubs_owner`
- **Operations**: read (join in AOV queries)
- **Fields**: `first_name`, `last_name`
- **Business rule**: order code normalization `/`→`-`, fallback to `'CP'`

### Entity: TIMOO Tenant

- **Purpose**: Primary entity for Accounting TIMOO page
- **Source**: `anisetta.as7_tenants` (includes `name` column), `anisetta.as7_pbx_accounting`
- **Operations**: read (daily stats aggregated with tenant name)
- **Fields**: `id`, `customer_id`, `as7_tenant_id`, `name` (varchar 255, nullable)
- **V1 simplification**: TIMOO API call eliminated — `name` now available in DB

---

## View Specifications

### View: Home

- **User intent**: Overview of available reports, quick navigation
- **Interaction pattern**: Card hub
- **Layout**: `TabNavGroup` in AppShell header (grouped navigation) + card grid on Home page
- **Navigation groups**:

| Group | Pages |
|---|---|
| Commerciale | Ordini, AOV |
| Rete | Accessi attivi, Attivazioni in corso |
| Contratti | Rinnovi in arrivo |
| Operativo | Anomalie MOR, Accounting TIMOO |

- **Card content**: Icon (from Icon system) + report title + one-line description
- **Design**: `--color-bg-elevated`, `--shadow-sm`→`--shadow-md` on hover, `--radius-lg`, `sectionEnter` animation with stagger
- **Entry point**: App launch (default route `/`)
- **Exit points**: Click card or TabNavGroup → report page

### View: Ordini

- **User intent**: Generate filtered order detail XLSX report
- **Interaction pattern**: Filter → Preview (summary + detail when rows > 0) → Export
- **Filters**:
  - Date range: `i_from` (default: 1 month ago), `i_to` (default: yesterday)
  - Order status: multi-select (options from `GET /api/reports/order-statuses`)
- **Phase 1 — Summary panel** (after "Anteprima" click):
  - Order count, total MRC, total NRC
  - Breakdown by `stato_ordine` (chip/badge)
  - Effective date range (first/last order)
- **Phase 2 — Detail table**:
  - Auto-shown when preview returns rows
  - First 100 rows with truncation indicator
- **Actions**: "Anteprima" (secondary button), "Esporta XLSX" (primary button)
- **Data source**: `POST /api/reports/orders/preview` → `POST /api/reports/orders/export`
- **Appsmith SQL**: `get_report_data` (Ordini) — verbatim in backend
- **Notes**: Date filter uses `data_ordine`, not `data_conferma` (differs from AOV)

### View: Accessi attivi

- **User intent**: Generate filtered active access lines XLSX report
- **Interaction pattern**: Filter → Preview (summary + detail when rows > 0) → Export
- **Filters**:
  - Connection type: multi-select (options from `GET /api/reports/connection-types`)
  - Line status: multi-select (hardcoded options: Attiva, Cessata, da attivare, in attivazione, KO; default: `["Attiva"]`)
- **Phase 1 — Summary panel**:
  - Line count
  - Breakdown by `tipo_conn`
  - Breakdown by `stato`
- **Phase 2 — Detail table**:
  - Auto-shown when preview returns rows
  - First 100 rows
- **Actions**: "Anteprima", "Esporta XLSX"
- **Data source**: `POST /api/reports/active-lines/preview` → `POST /api/reports/active-lines/export`
- **Appsmith SQL**: `get_accessi` — verbatim in backend
- **Notes**: Report name is static (`report_accessi_attivi`), different Carbone template

### View: Attivazioni in corso

- **User intent**: Monitor confirmed orders with rows pending activation
- **Interaction pattern**: Master-detail, auto-load, no filters, no export
- **Master table**: Confirmed orders (`stato_ordine = 'Confermato'`, `stato_riga = 'Da attivare'`)
  - Columns: `ragione_sociale`, `numero_ordine`, `data_documento`, `durata_servizio`, `durata_rinnovo`, `sost_ord`, `sostituito_da`, `storico`
  - `storico` computed by DB function `get_reverse_order_history_path()`
- **Detail table** (on row select): Order line items for selected order
  - Columns: `descrizione_long`, `quantita`, `nrc`, `mrc`, `totale_mrc`, `stato_riga`, `serialnumber`, `note_legali`
- **Data source**: `GET /api/reports/pending-activations` → `GET /api/reports/pending-activations/:orderNumber/rows`
- **Appsmith SQL**: `get_confirmed_orders`, `get_rows` (Attivazioni) — verbatim in backend

### View: Rinnovi in arrivo

- **User intent**: Review upcoming contract renewals within configurable window
- **Interaction pattern**: Master-detail with filters
- **Filters**:
  - Minimum MRC: input (default: `11`)
  - Months ahead: slider 1–12 (default: `4`)
  - "Esegui" button to refresh
- **Master table**: Customer renewal aggregates
  - Columns: `ragione_sociale`, `rinnovi_dal`, `rinnovi_al`, `ordini_servizi`, `senza_tacito_rinnovo`, `canoni`
- **Detail table** (on row select): Individual service renewal rows
  - Columns: `nome_testata_ordine`, `stato_ordine`, `descrizione_long`, `quantita`, `nrc`, `mrc`, `stato_riga`, `serialnumber`, `note_legali`, `data_attivazione`, `durata`, `prossimo_rinnovo`, `sost_ord`, `sostituito_da`, `tacito_rinnovo`
- **Business rules in query**:
  - Renewal window: `current_date - 15 days` to `current_date + N months` (BR6)
  - Filter: `durata_rinnovo > 3 OR tacito_rinnovo = 0` (BR7)
  - `senza_tacito_rinnovo` = `sum(tacito_rinnovo) < count(0)` (BR7)
  - Only `stato_ordine = 'Evaso'`, `stato_riga = 'Attiva'`, `data_cessazione IS NULL`
- **Data source**: `GET /api/reports/upcoming-renewals?months=N&minMrc=N` → `GET /api/reports/upcoming-renewals/:customerId/rows?months=N&minMrc=N`
- **Appsmith SQL**: `get_aggregato_scadenze`, `get_rows` (Rinnovi) — verbatim in backend

### View: Anomalie MOR (V1 — Tab 1 only)

- **User intent**: Identify telephone billing anomalies by cross-referencing with ERP orders
- **Interaction pattern**: Auto-load enriched table, no filters, no export
- **Table**: Enriched billing records with flags
  - All fields from `importi_telefonici` + `conti_telefonici` + `cli_fatturazione.intestazione`
  - Enrichment: `ordine_presente` (SI/NO), `numero_ordine_corretto` (SI/NO) — matching by `serialnumber`
- **Cross-DB join**: Backend queries Grappa MySQL (latest billing period) + Mistra PostgreSQL (active voice orders `codice_prodotto = 'CDL-TVOCE'`), cross-references server-side
- **Data source**: `GET /api/reports/mor-anomalies`
- **Appsmith SQL**: `get_ultimi_importi_tel` (Grappa), `check_ordine_voce` (Mistra) — verbatim in backend
- **Appsmith JS**: `collega_ordini()` cross-reference logic — moved to backend Go
- **V1 exclusion**: Tab 2 (AI analysis, model selector, OpenRouter integration) deferred to Phase 2

### View: Accounting TIMOO daily

- **User intent**: View daily user and service extension accounting per TIMOO tenant
- **Interaction pattern**: Auto-load single table, no filters, no export
- **Table**: Daily stats with tenant name
  - Columns: `tenant_id`, `name` (tenant), `day`, `users`, `service_extensions`
- **Business rules in query**:
  - Date window: last 3 full months (BR6-like)
  - Stats aggregation: `MAX(users)` and `MAX(service_extensions)` per PBX per day, summed per tenant per day
  - Tenant test exclusion: `WHERE name != 'KlajdiandCo'` (BR11)
- **V1 simplification**: Single SQL query with JOIN `as7_tenants` replaces the Appsmith chain (Anisetta→TIMOO API→Anisetta→client-side merge)
- **Data source**: `GET /api/reports/timoo/daily-stats`
- **Appsmith SQL**: `getUsersSEbyDay`, `getAnisettaTenants` — **simplified** to single JOIN query in V1

### View: AOV (Annual Order Value)

- **User intent**: Multi-view AOV analysis with XLSX export
- **Interaction pattern**: Filter → Stat cards + Tabbed tables → Export
- **Filters**:
  - Date range: `i_from` (default: 1 month ago), `i_to` (default: yesterday)
  - Order status: multi-select (options from `GET /api/reports/order-statuses`)
- **Stat cards panel** (always visible after data load):
  - AOV total (EUR), count by-type, count by-category, count by-sales
  - Clickable — activates corresponding tab
  - Design: `--color-bg-elevated`, `--shadow-xs`, `--radius-md`; value `1.75rem` weight 700; label `0.75rem` uppercase
- **Tabbed tables** (controlled `TabNav`, not routing):
  - Tab 1 "Per tipo": AOV by order type + year/month
  - Tab 2 "Per categoria": AOV by product category + year/month
  - Tab 3 "Per commerciale": AOV by sales rep + order type
  - Tab 4 "Dettaglio": Full order-level detail
- **Actions**: "Esegui" (refresh all), "Esporta XLSX" (primary button)
- **Business rules in queries** (all 4 queries, verbatim):
  - AOV: `MRC_new * 12 + NRC` with substitution delta and TSC-ORDINE swap (BR1)
  - Date fallback: `data_conferma`, sentinel `0001-01-01` → `data_documento` (BR2)
  - Type mapping: N→NUOVO, A→SOST, R→RINNOVO, C→CESSAZIONE (BR3)
  - Sales rep: HubSpot join with `/`→`-` normalization, default `'CP'` (BR4)
  - **Known inconsistency**: `get_report_data_area` does NOT subtract old MRC for substitutions (tracked in TODO)
- **Data source**: `POST /api/reports/aov/preview` (returns 4 datasets) → `POST /api/reports/aov/export`
- **Appsmith SQL**: `get_report_data`, `get_report_data_tipo_ord`, `get_report_data_area`, `get_report_data_sales` — 4 separate verbatim queries in backend

---

## Logic Allocation

### Backend responsibilities

| Responsibility | Details |
|---|---|
| All DB queries | Verbatim from audit, parametrized with prepared statements (no string interpolation) |
| Business rules BR1–BR7, BR11, BR12 | Embedded in SQL queries, unchanged |
| Cross-DB join (BR9) | Anomalie MOR: Grappa + Mistra joined server-side in Go |
| Carbone.io orchestration | Backend calls Carbone.io API, streams XLSX to frontend |
| TIMOO tenant resolution | Single SQL JOIN (eliminated API call) |

### Frontend responsibilities

| Responsibility | Details |
|---|---|
| Filter state | Date defaults, multi-select selections, slider value |
| Master-detail selection | Row click → fetch detail endpoint |
| Client-side aggregation | Stat cards (AOV), summary panels (Ordini, Accessi) — computed on received data |
| Presentation | Tables, skeleton loading, animations, responsive layout |

### Deferred (Phase 2)

| Item | Details |
|---|---|
| AI analysis | OpenRouter proxy, prompt rules (BR8), model selector |
| AI access control | Keycloak role `app_reports_ai_access` replacing email gate (BR10) |
| AOV query consolidation | 4 queries → 1 parameterized query, post-coexistence |
| AOV area bug fix | Correct `get_report_data_area` to subtract old MRC for substitutions |

---

## Integrations and Data Flow

### External systems

| System | Purpose | V1 approach |
|---|---|---|
| mistra (PostgreSQL) | Primary data — orders, customers, access lines, renewals | Connection pool, existing in backend |
| grappa (MySQL) | Phone billing data | New connection pool |
| anisetta (PostgreSQL) | TIMOO tenant accounting | New connection pool |
| carbone.io (REST) | XLSX template rendering | Backend HTTP client, API key in env var |

### Carbone.io template IDs (constants)

| Template | ID | Used by |
|---|---|---|
| Ordini XLSX | `d18b310491b0c8d2518841b4e09cc18d8b91c5a59ae5a55c37924fcb169de166` | Ordini, AOV |
| Accessi attivi XLSX | `a482f92419a0c17bb9bfae00c64d251c6a527f95c67993d86bf2d11d9e2e7a9e` | Accessi attivi |

### Data ownership boundaries

| Data | Owned by | Accessed by |
|---|---|---|
| Orders, customers, access lines | mistra `loader` schema (views over ERP data) | Reports backend (read-only) |
| Phone billing | grappa MySQL | Reports backend (read-only) |
| TIMOO accounting | anisetta PostgreSQL | Reports backend (read-only) |
| XLSX templates | carbone.io (external) | Reports backend (POST render, GET download) |

---

## API Contract Summary

### Lookup endpoints

| Endpoint | Method | Response |
|---|---|---|
| `/api/reports/order-statuses` | GET | `string[]` — distinct `stato_ordine` values |
| `/api/reports/connection-types` | GET | `string[]` — distinct `tipo_conn` values |

### Preview endpoints (return JSON data)

| Endpoint | Method | Parameters | Response |
|---|---|---|---|
| `/api/reports/orders/preview` | POST | `{dateFrom, dateTo, statuses[]}` | JSON order rows |
| `/api/reports/active-lines/preview` | POST | `{connectionTypes[], statuses[]}` | JSON access line rows |
| `/api/reports/aov/preview` | POST | `{dateFrom, dateTo, statuses[]}` | JSON with 4 datasets: `{byType, byCategory, bySales, detail}` |

### Export endpoints (return XLSX stream)

| Endpoint | Method | Parameters | Response |
|---|---|---|---|
| `/api/reports/orders/export` | POST | `{dateFrom, dateTo, statuses[]}` | `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet` |
| `/api/reports/active-lines/export` | POST | `{connectionTypes[], statuses[]}` | XLSX stream |
| `/api/reports/aov/export` | POST | `{dateFrom, dateTo, statuses[]}` | XLSX stream |

### Master-detail endpoints

| Endpoint | Method | Parameters | Response |
|---|---|---|---|
| `/api/reports/pending-activations` | GET | — | JSON master rows |
| `/api/reports/pending-activations/:orderNumber/rows` | GET | — | JSON detail rows |
| `/api/reports/upcoming-renewals` | GET | `?months=N&minMrc=N` | JSON master rows |
| `/api/reports/upcoming-renewals/:customerId/rows` | GET | `?months=N&minMrc=N` | JSON detail rows |

### Auto-load endpoints

| Endpoint | Method | Parameters | Response |
|---|---|---|---|
| `/api/reports/mor-anomalies` | GET | — | JSON enriched billing records |
| `/api/reports/timoo/daily-stats` | GET | — | JSON daily stats with tenant names |

---

## Constraints and Non-Functional Requirements

### Security

- All DB queries use prepared statements — no string interpolation (fixes Appsmith SQL injection pattern)
- Carbone.io API key in backend env var only — never exposed to frontend
- Keycloak authentication required (`app_reports_access` role)
- All datasource access proxied through backend — no direct DB connections from frontend

### Coexistence

- Same databases, same views, no schema changes
- App runs alongside Appsmith during transition period
- Read-only — no write operations to any database

### Database dependencies

| View/Function | Schema | DDL location |
|---|---|---|
| `v_ordini_ric_spot` | `loader` | `docs/mistradb/mistra_loader.json` |
| `v_ordini_sintesi` | `loader` | `docs/mistradb/mistra_loader.json` |
| `v_ordini_ricorrenti` | `loader` | `docs/mistradb/mistra_loader.json` |
| `v_ordini_ricorrenti_conrinnovo` | `loader` | `docs/mistradb/mistra_loader.json` |
| `get_reverse_order_history_path()` | `loader` | `docs/mistradb/mistra_loader.json` |

---

## Verbatim SQL Reference

All queries below are copied verbatim from the Appsmith audit. The backend must use these exact queries (with prepared statement parameterization replacing `{{widget}}` interpolation) to guarantee 1:1 correspondence.

### Ordini — get_stati_ordine

```sql
SELECT DISTINCT stato_ordine FROM loader.v_ordini_ric_spot;
```

### Ordini — get_report_data

```sql
SELECT eac.ragione_sociale, o.stato_ordine,
       o.nome_testata_ordine as numero_ordine,
       o.descrizione_long,
       o.quantita,
       o.setup as nrc,
       o.canone as mrc,
       round(o.quantita::decimal * o.canone::decimal,2) as totale_mrc,
       o.numero_azienda,
       o.data_ordine as data_documento,
       o.stato_riga,
       o.data_ultima_fatt,
       o.serialnumber,
       o.metodo_pagamento,o.durata_servizio, o.durata_rinnovo, o.data_cessazione, o.data_attivazione, o.note_legali,
       o.sost_ord, o.sostituito_da, o.progressivo_riga
from loader.v_ordini_ric_spot as o join loader.erp_anagrafiche_clienti eac on o.numero_azienda = eac.numero_azienda
where stato_ordine in ($STATUSES)
and data_ordine BETWEEN $DATE_FROM and $DATE_TO
order by eac.ragione_sociale, data_documento, nome_testata_ordine, progressivo_riga;
```

### Accessi attivi — get_tipo_conn

```sql
SELECT DISTINCT tipo_conn FROM loader.grappa_foglio_linee;
```

### Accessi attivi — get_accessi

```sql
SELECT cf.intestazione as ragione_sociale,
    tipo_conn, fl.fornitore, provincia, comune, p.tipo, p.profilo_commerciale,
    case when p.banda_up <> p.banda_down then 'CONDIVISA' else 'DEDICATA' end as macro,
    cogn_rsoc_intest_linea AS intestatario, r.nome_testata_ordine ordine, r.data_ultima_fatt fatturato_fino_al,
    r.stato_riga, r.stato_ordine,
    fl.stato, fl.id, codice_ordine, fl.serialnumber,  cf.codice_aggancio_gest AS id_anagrafica,
    r.quantita, r.canone
FROM
    loader.grappa_foglio_linee fl
        JOIN loader.grappa_cli_fatturazione cf ON fl.id_anagrafica = cf.id
        LEFT JOIN loader.grappa_profili p ON fl.id_profilo = p.id
        LEFT JOIN (
        SELECT *,
               ROW_NUMBER() OVER(PARTITION BY serialnumber ORDER BY data_documento DESC , progressivo_riga) AS rn
        FROM loader.v_ordini_ricorrenti
    ) r ON fl.serialnumber = r.serialnumber AND r.numero_azienda = cf.codice_aggancio_gest AND r.rn = 1
WHERE fl.stato in ($STATUSES)
    and fl.tipo_conn in ($CONNECTION_TYPES)
order by cf.intestazione, tipo_conn, fl.fornitore, provincia, comune, p.tipo, p.profilo_commerciale;
```

### Attivazioni in corso — get_confirmed_orders

```sql
SELECT distinct
       eac.ragione_sociale,
       nome_testata_ordine as numero_ordine,
       data_documento,
       durata_servizio, durata_rinnovo, sost_ord, sostituito_da,
       loader.get_reverse_order_history_path(nome_testata_ordine) as storico,
       os.numero_azienda
from loader.v_ordini_sintesi os join loader.erp_anagrafiche_clienti eac on os.numero_azienda = eac.numero_azienda
where os.stato_ordine in ('Confermato') and stato_riga in ('Da attivare')
order by eac.ragione_sociale, data_documento, nome_testata_ordine;
```

### Attivazioni in corso — get_rows

```sql
SELECT descrizione_long, quantita, nrc, mrc, totale_mrc,
       stato_riga, serialnumber, note_legali
from loader.v_ordini_sintesi os join loader.erp_anagrafiche_clienti eac on os.numero_azienda = eac.numero_azienda
where os.nome_testata_ordine = $ORDER_NUMBER
  and stato_riga in ('Da attivare')
order by eac.ragione_sociale, data_documento, nome_testata_ordine;
```

### Rinnovi in arrivo — get_aggregato_scadenze

```sql
select ragione_sociale, min(prossimo_rinnovo) as rinnovi_dal, max(prossimo_rinnovo) as rinnovi_al,
       count(distinct nome_testata_ordine) as numero_ordini, count(0) as servizi_attivi,
       count(distinct nome_testata_ordine) || ' / ' || count(0) as ordini_servizi,
       sum(tacito_rinnovo) < count(0) as senza_tacito_rinnovo, sum(mrc) as canoni, numero_azienda
from loader.v_ordini_ricorrenti_conrinnovo os
where (durata_rinnovo > 3 or tacito_rinnovo=0)
  and stato_ordine in ('Evaso') and (data_cessazione is null) and stato_riga in ('Attiva')
  and prossimo_rinnovo BETWEEN current_date - INTERVAL '15 days' and current_date + ($MONTHS || ' months')::interval
  and mrc >= $MIN_MRC
group by ragione_sociale, numero_azienda
order by 2;
```

### Rinnovi in arrivo — get_rows

```sql
SELECT nome_testata_ordine, stato_ordine, descrizione_long, quantita,
       setup as nrc, canone as mrc, stato_riga, serialnumber, note_legali,
       data_attivazione, durata_servizio, durata_rinnovo,
       durata_servizio || ' / ' || durata_rinnovo as durata,
       prossimo_rinnovo, sost_ord, sostituito_da, tacito_rinnovo
from loader.v_ordini_ricorrenti_conrinnovo
where (durata_rinnovo > 3 or tacito_rinnovo=0)
  and stato_ordine in ('Evaso') and (data_cessazione is null) and stato_riga in ('Attiva')
  and prossimo_rinnovo BETWEEN current_date - INTERVAL '15 days' and current_date + ($MONTHS || ' months')::interval
  and mrc >= $MIN_MRC
  and numero_azienda = $CUSTOMER_ID
order by prossimo_rinnovo, nome_testata_ordine;
```

### Anomalie MOR — get_ultimi_importi_tel (Grappa/MySQL)

```sql
select it.conto, lastname, firstname, is_da_fatturare, codice_ordine, serialnumber,
       it.periodo_inizio, it.importo, it.stato, it.tipologia, ct.id_cliente, ac.intestazione
from importi_telefonici it
         left join conti_telefonici ct on ct.conto = it.conto
         left join grappa.cli_fatturazione ac on ct.id_cliente = ac.id
where periodo_inizio = (select periodo_inizio from importi_telefonici order by id desc limit 1);
```

### Anomalie MOR — check_ordine_voce (Mistra/PostgreSQL)

```sql
SELECT *
from loader.erp_righe_ordini ero
where codice_prodotto = 'CDL-TVOCE' and data_cessazione = '0001-01-01 00:00:00.000000'
order by cliente;
```

### Anomalie MOR — collega_ordini (JS → Go logic)

Cross-reference logic to implement server-side:
```
For each billing record from get_ultimi_importi_tel:
  Find matching order row from check_ordine_voce WHERE serialnumber matches
  Set ordine_presente = 'SI' if match found with codice_prodotto, else 'NO'
  Set numero_ordine_corretto = 'SI' if match.nome_testata_ordine == billing.codice_ordine, else 'NO'
```

### Accounting TIMOO — V1 simplified query (replaces 3-step Appsmith chain)

```sql
SELECT t.as7_tenant_id as tenant_id, t.name as tenant_name,
       DATE(a.data) AS day,
       SUM(max_users) as users, SUM(max_se) as service_extensions
FROM (
    SELECT as7_tenant_id, pbx_id,
           DATE(data) AS giorno,
           MAX(users) AS max_users,
           MAX(service_extensions) AS max_se
    FROM as7_pbx_accounting
    WHERE data >= DATE_TRUNC('month', CURRENT_DATE - INTERVAL '3 month')
      AND data < CURRENT_DATE
    GROUP BY as7_tenant_id, pbx_id, DATE(data)
) a
JOIN as7_tenants t ON a.as7_tenant_id = t.as7_tenant_id
WHERE t.name IS NOT NULL AND t.name != 'KlajdiandCo'
GROUP BY t.as7_tenant_id, t.name, DATE(a.giorno)
ORDER BY day DESC, tenant_id;
```

### AOV queries

The 4 AOV queries are extensive (each 40+ lines with complex CASE expressions). They are preserved verbatim from the audit in `apps/reports/APPSMITH-AUDIT.md` §2.8. The backend must implement them as 4 separate queries — **do not consolidate**.

---

## Open Questions and Deferred Decisions

| # | Question | Status | Owner |
|---|---|---|---|
| 1 | AOV `get_report_data_area` inconsistency — is the missing MRC delta a bug or intentional? | Deferred post-coexistence | Domain expert |
| 2 | AOV query consolidation — merge 4 queries into 1? | Deferred post-coexistence | Engineering |
| 3 | AI analysis feature (Anomalie MOR Tab 2) | Deferred Phase 2 | Product |
| 4 | Carbone template management (central admin module) | Tracked in `docs/TODO.md` | Product |
| 5 | `as7_tenants.name` nullable — is KlajdiandCo the only test tenant to exclude? | Verify before implementation | Domain expert |

---

## Acceptance Notes

### What the audit proved directly

- All 7 report pages are read-only
- 39 queries across 6 datasources, all documented with verbatim SQL
- 12 business rules identified and classified
- 3 shared patterns (export, master-detail, auto-load) with clear structure
- Security concerns (SQL injection, API keys in frontend) identified and addressed in V1 design

### What the expert confirmed

- Migrazione 1:1 with Appsmith coexistence
- AOV bug replicated as-is (tracked in TODO)
- AI analysis deferred to Phase 2
- 4 AOV queries kept separate to prevent drift
- Carbone.io maintained with same template IDs as constants
- TIMOO API eliminated (tenant name now in DB)
- Navigation: TabNavGroup with logical grouping
- AOV: stat cards + tabbed tables
- Export: two-phase preview with summary

### What still needs validation

- AOV `get_report_data_area` computation correctness (post-coexistence)
- `KlajdiandCo` exclusion — confirm this is still the only test tenant
- Carbone.io template content (XLSX layouts) — verify templates still render correctly when called from backend instead of Appsmith
