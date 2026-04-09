# Appsmith Audit: Reports

**Source**: `apps/reports/Reports.json` (single-file export) + `apps/reports/reports-main.zip` (git-style export, partial: AOV + Accessi attivi only)
**Audit date**: 2026-04-09

---

## 1. Application Inventory

| Field | Value |
|---|---|
| App name | Reports |
| Layout | Sidebar navigation, fixed positioning, light theme |
| Pages | 8 |
| Datasources | 6 |
| Actions (queries + API calls) | 39 |
| JSObject collections | 6 |
| Custom JS libraries | 0 |

### Pages

| Page | Slug | Default | Purpose |
|---|---|---|---|
| Home | home | Yes (app.json) | Splash/landing image only |
| Ordini | ordini | No | Order detail report with XLSX export |
| Accessi attivi | accessi-attivi | No | Active access lines report with XLSX export |
| Attivazioni in corso | attivazioni-in-corso | No | Confirmed orders pending activation (master-detail) |
| Rinnovi in arrivo | rinnovi-in-arrivo | No | Upcoming contract renewals (master-detail) |
| Anomalie MOR...tacci | anomalie-mor-tacci | No | MOR billing anomaly detection with AI analysis |
| Accounting TIMOO daily | accounting-timoo-daily | No | TIMOO tenant daily user/SE accounting |
| AOV | aov | No | Annual Order Value multi-view analysis with XLSX export |

### Datasources

| Name | Plugin | Type | Used by pages |
|---|---|---|---|
| mistra | postgres-plugin | PostgreSQL | Ordini, Accessi attivi, Attivazioni in corso, Rinnovi in arrivo, Anomalie MOR...tacci, AOV |
| grappa | mysql-plugin | MySQL | Anomalie MOR...tacci |
| anisetta | postgres-plugin | PostgreSQL | Accounting TIMOO daily |
| carbone.io | restapi-plugin | REST API | Ordini, Accessi attivi, AOV |
| openrouter | restapi-plugin | REST API | Anomalie MOR...tacci |
| TIMOO API | restapi-plugin | REST API | Accounting TIMOO daily |

### JSObject Collections

| Page | Name | Methods |
|---|---|---|
| Ordini | utils | `getURL()`, `runReport()` |
| Accessi attivi | utils | `getURL()`, `runReport()` |
| Anomalie MOR...tacci | utils | `collega_ordini()`, `analizza()`, `ai_request()`, `abilita_controlli_ai()` |
| Anomalie MOR...tacci | _$js_openrouter1$_js_openrouter | (empty - vestigial) |
| Accounting TIMOO daily | utils | `generaTenantIdList()`, `listaTenants()`, `reportData()` |
| AOV | utils | `getURL()`, `runReport()` |

### Global Notes

- The app is a collection of **read-only reporting tools** with no write operations to any database.
- Three pages (Ordini, Accessi attivi, AOV) share an identical XLSX export pattern via Carbone.io.
- The Home page is a static image with no functionality.
- Navigation is sidebar-driven; no explicit cross-page navigation links in code.

---

## 2. Page Audits

### 2.1 Home

**Purpose**: Static landing page.

**Widgets**:
- `Image1` (IMAGE_WIDGET) — splash image

**Queries**: None
**JSObjects**: None
**Event flow**: None
**Hidden logic**: None
**Migration notes**: Trivial — just a branded landing page.

---

### 2.2 Ordini

**Purpose**: Generate a filterable order detail report and export to XLSX via Carbone.io.

**Widgets**:

| Widget | Type | Role |
|---|---|---|
| `Container1` | CONTAINER_WIDGET | Layout wrapper |
| `i_from` | DATE_PICKER_WIDGET2 | Start date filter (default: `moment().subtract(1,'month')`) |
| `i_to` | DATE_PICKER_WIDGET2 | End date filter (default: `moment().subtract(1,'days')`) |
| `ms_o_stati` | MULTI_SELECT_WIDGET_V2 | Order status filter (source: `get_stati_ordine.data`) |
| `Text1` | TEXT_WIDGET | Label: "Genera xlsx con gli ordini nel periodo indicato" |
| `Button1` | BUTTON_WIDGET | onClick: `utils.runReport()` |

**Queries**:

| Query | Datasource | executeOnLoad | Purpose |
|---|---|---|---|
| `get_stati_ordine` | mistra | Yes | Fetch distinct `stato_ordine` values from `loader.v_ordini_ric_spot` |
| `get_report_data` | mistra | No | Fetch order detail rows with MRC/NRC calculations, filtered by status + date range |
| `render_template` | carbone.io | No | Send data to Carbone.io API for XLSX rendering |

**Event flow**:
1. Page load → `get_stati_ordine` runs → populates `ms_o_stati` dropdown
2. User selects date range + order statuses
3. User clicks Button1 → `utils.runReport()`:
   - Runs `get_report_data` query
   - Stores results in `utils.dati`
   - Sets `utils.reportName` = `"report_ordini_dal_" + i_from.formattedDate + "_al_" + i_to.formattedDate`
   - Runs `render_template` (POSTs to Carbone.io with template ID + data)
   - Gets render URL via `utils.getURL()` → opens in new window

**Hidden logic**:
- **Business rule**: Date filter uses `data_ordine` (order date), not `data_conferma` (unlike AOV which uses confirmation date with fallback logic).
- **Business rule**: The query joins `loader.v_ordini_ric_spot` with `loader.erp_anagrafiche_clienti` on `numero_azienda` to get `ragione_sociale`.
- **Business rule**: MRC/NRC computation: `totale_mrc = round(quantita * canone, 2)`.
- **Hardcoded template ID**: `d18b310491b0c8d2518841b4e09cc18d8b91c5a59ae5a55c37924fcb169de166` — Carbone.io template for order XLSX.
- **Carbone.io URL construction**: `https://render.carbone.io/render/{renderId}` — hardcoded base URL in JSObject.

**Candidate domain entities**: Order, Customer (ragione_sociale), Order Status.

**SQL** (get_report_data):
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
where stato_ordine in ({{ms_o_stati.selectedOptionValues.map(i => "'" + i + "'").join()}})
and data_ordine BETWEEN '{{i_from.selectedDate}}' and '{{i_to.selectedDate}}'
order by eac.ragione_sociale, data_documento, nome_testata_ordine, progressivo_riga;
```

---

### 2.3 Accessi attivi

**Purpose**: Report of active network access lines with XLSX export.

**Widgets**:

| Widget | Type | Role |
|---|---|---|
| `Container1` | CONTAINER_WIDGET | Layout wrapper |
| `ms_tipo_conn` | MULTI_SELECT_WIDGET_V2 | Connection type filter (source: `get_tipo_conn.data`, transformed to `{label, value}`) |
| `ms_stato` | MULTI_SELECT_WIDGET_V2 | Line status filter (hardcoded options: Attiva, Cessata, da attivare, in attivazione, KO; default: `["Attiva"]`) |
| `Button1` | BUTTON_WIDGET | onClick: `utils.runReport()` |
| `Text1` | TEXT_WIDGET | Label: "Genera xlsx con accessi attivi" |

**Queries**:

| Query | Datasource | executeOnLoad | Purpose |
|---|---|---|---|
| `get_tipo_conn` | mistra | Yes | Fetch distinct `tipo_conn` from `loader.grappa_foglio_linee` |
| `get_accessi` | mistra | No | Fetch active access line details with order/profile joins |
| `render_template` | carbone.io | No | XLSX rendering via Carbone.io |

**Event flow**:
1. Page load → `get_tipo_conn` runs → populates `ms_tipo_conn`
2. User selects connection types + line statuses
3. Button1 → `utils.runReport()`:
   - Runs `get_accessi`
   - Stores data in `utils.dati`
   - Runs `render_template` with template `a482f92419a0c17bb9bfae00c64d251c6a527f95c67993d86bf2d11d9e2e7a9e`
   - Opens Carbone.io download URL in new window

**Hidden logic**:
- **Business rule**: Complex join chain: `grappa_foglio_linee` → `grappa_cli_fatturazione` (customer) → `grappa_profili` (bandwidth profile) → `v_ordini_ricorrenti` (latest order per serial number via `ROW_NUMBER() OVER(PARTITION BY serialnumber ORDER BY data_documento DESC, progressivo_riga)`).
- **Business rule**: Bandwidth classification: `CASE WHEN p.banda_up <> p.banda_down THEN 'CONDIVISA' ELSE 'DEDICATA' END AS macro`.
- **Business rule**: Latest order matching uses serial number + customer ID (`r.numero_azienda = cf.codice_aggancio_gest`), picking only `rn = 1`.
- **Cross-database reference**: Uses Grappa tables (`grappa_foglio_linee`, `grappa_cli_fatturazione`, `grappa_profili`) loaded into the Mistra `loader` schema.
- **Different Carbone template**: Uses template ID `a482f92419a0c17bb9bfae00c64d251c6a527f95c67993d86bf2d11d9e2e7a9e` (distinct from Ordini/AOV).
- **Hardcoded report name**: `"report_accessi_attivi"` (unlike Ordini/AOV which use dynamic date-based names).

**SQL** (get_accessi):
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
WHERE fl.stato in ({{ms_stato.selectedOptionValues.map(i => "'" + i + "'").join()}})
    and fl.tipo_conn in ({{ms_tipo_conn.selectedOptionValues.map(i => "'" + i + "'").join()}})
order by cf.intestazione, tipo_conn, fl.fornitore, provincia, comune, p.tipo, p.profilo_commerciale;
```

---

### 2.4 Attivazioni in corso

**Purpose**: Master-detail view of confirmed orders with rows pending activation.

**Widgets**:

| Widget | Type | Role |
|---|---|---|
| `Text1` | TEXT_WIDGET | Label: "Elenco ordini in stato confermato con righe da attivare" |
| `tbl_ordini` | TABLE_WIDGET_V2 | Master table — confirmed orders (source: `get_confirmed_orders.data`); onRowSelected → `get_rows.run()` |
| `tbl_righe` | TABLE_WIDGET_V2 | Detail table — order line items (source: `get_rows.data`) |

**Queries**:

| Query | Datasource | executeOnLoad | Purpose |
|---|---|---|---|
| `get_confirmed_orders` | mistra | Yes | Fetch distinct confirmed orders with `stato_riga = 'Da attivare'` |
| `get_rows` | mistra | Yes | Fetch line items for selected order (`tbl_ordini.selectedRow.numero_ordine`) |

**Event flow**:
1. Page load → `get_confirmed_orders` runs → populates master table
2. User selects a row → `get_rows` runs → populates detail table with line items for that order

**Hidden logic**:
- **Business rule**: Filters `stato_ordine = 'Confermato'` AND `stato_riga = 'Da attivare'`.
- **Business rule**: Uses `loader.get_reverse_order_history_path(nome_testata_ordine)` — a **database function** — to get order history chain (`storico` column). This is server-side logic embedded in the DB.
- **Business rule**: Master query uses `loader.v_ordini_sintesi` (different view from `v_ordini_ric_spot` used elsewhere).
- **No export**: This is a view-only page with no XLSX export.

**SQL** (get_confirmed_orders):
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

**SQL** (get_rows):
```sql
SELECT descrizione_long, quantita, nrc, mrc, totale_mrc,
       stato_riga, serialnumber, note_legali
from loader.v_ordini_sintesi os join loader.erp_anagrafiche_clienti eac on os.numero_azienda = eac.numero_azienda
where os.nome_testata_ordine = {{tbl_ordini.selectedRow.numero_ordine || ''}}
  and stato_riga in ('Da attivare')
order by eac.ragione_sociale, data_documento, nome_testata_ordine;
```

---

### 2.5 Rinnovi in arrivo

**Purpose**: Upcoming contract renewals within a configurable time window, with master-detail view.

**Widgets**:

| Widget | Type | Role |
|---|---|---|
| `i_mrc` | INPUT_WIDGET_V2 | Minimum MRC filter (default: `11`, label: "MRC minimo") |
| `ns_mesi` | NUMBER_SLIDER_WIDGET | Months ahead to check (default: `4`, range: 1–12, label: "Rinnovi entro") |
| `Button1` | BUTTON_WIDGET | onClick: runs `get_aggregato_scadenze` then `get_rows` |
| `Table1` | TABLE_WIDGET_V2 | Master — aggregated customer renewal summary; onRowSelected → `get_rows.run()` |
| `Table2` | TABLE_WIDGET_V2 | Detail — individual service renewal rows for selected customer |

**Queries**:

| Query | Datasource | executeOnLoad | Purpose |
|---|---|---|---|
| `get_aggregato_scadenze` | mistra | Yes | Aggregate renewal dates by customer within time window |
| `get_rows` | mistra | Yes | Detail rows for selected customer (`Table1.selectedRow.numero_azienda`) |

**Event flow**:
1. Page load → both queries run → master table populated
2. User adjusts slider (months) and MRC threshold
3. Button1 → `get_aggregato_scadenze.run().then(() => { get_rows.run(); })`
4. User selects customer row → `get_rows.run()`

**Hidden logic**:
- **Business rule**: Renewal window = `current_date - 15 days` to `current_date + N months` — the 15-day lookback catches recently-passed renewals.
- **Business rule**: Only includes services where `durata_rinnovo > 3 OR tacito_rinnovo = 0` — filters out short auto-renewals, highlights manual renewals and significant contract terms.
- **Business rule**: Filters `stato_ordine = 'Evaso'`, `stato_riga = 'Attiva'`, `data_cessazione IS NULL`.
- **Business rule**: Uses `loader.v_ordini_ricorrenti_conrinnovo` — a view that includes a computed `prossimo_rinnovo` column.
- **Business rule**: `senza_tacito_rinnovo` flag = `sum(tacito_rinnovo) < count(0)` — TRUE if any service in the group lacks auto-renewal.
- **Business rule**: MRC threshold filter (`mrc >= {{i_mrc.text}}`) to exclude low-value services.

**SQL** (get_aggregato_scadenze):
```sql
select ragione_sociale, min(prossimo_rinnovo) as rinnovi_dal, max(prossimo_rinnovo) as rinnovi_al,
       count(distinct nome_testata_ordine) as numero_ordini, count(0) as servizi_attivi,
       count(distinct nome_testata_ordine) || ' / ' || count(0) as ordini_servizi,
       sum(tacito_rinnovo) < count(0) as senza_tacito_rinnovo, sum(mrc) as canoni, numero_azienda
from loader.v_ordini_ricorrenti_conrinnovo os
where (durata_rinnovo > 3 or tacito_rinnovo=0)
  and stato_ordine in ('Evaso') and (data_cessazione is null) and stato_riga in ('Attiva')
  and prossimo_rinnovo BETWEEN current_date - INTERVAL '15 days' and current_date + ({{ns_mesi.value}} ||' months')::interval
  and mrc >= {{i_mrc.text}}
group by ragione_sociale, numero_azienda
order by 2;
```

**SQL** (get_rows):
```sql
SELECT nome_testata_ordine, stato_ordine, descrizione_long, quantita,
       setup as nrc, canone as mrc, stato_riga, serialnumber, note_legali,
       data_attivazione, durata_servizio, durata_rinnovo,
       durata_servizio || ' / ' || durata_rinnovo as durata,
       prossimo_rinnovo, sost_ord, sostituito_da, tacito_rinnovo
from loader.v_ordini_ricorrenti_conrinnovo
where (durata_rinnovo > 3 or tacito_rinnovo=0)
  and stato_ordine in ('Evaso') and (data_cessazione is null) and stato_riga in ('Attiva')
  and prossimo_rinnovo BETWEEN current_date - INTERVAL '15 days' and current_date + ({{ns_mesi.value}} ||' months')::interval
  and mrc >= {{i_mrc.text}}
  and numero_azienda = {{Table1.selectedRow.numero_azienda}}
order by prossimo_rinnovo, nome_testata_ordine;
```

---

### 2.6 Anomalie MOR...tacci

**Purpose**: Detect billing anomalies in telephone charge data (MOR system), cross-reference with ERP orders, and optionally run AI-based analysis via OpenRouter.

**Widgets**:

| Widget | Type | Role |
|---|---|---|
| `Tabs1` | TABS_WIDGET | Two-tab layout |
| Tab 1: `Table1` | TABLE_WIDGET_V2 | Displays enriched billing records (source: `utils.conti`) |
| Tab 2: `Text1` | TEXT_WIDGET | AI analysis output (source: `utils.analisi`) |
| Tab 2: `Button1` | BUTTON_WIDGET | onClick: `utils.analizza()` |
| Tab 2: `sl_modello` | SELECT_WIDGET | AI model selector (source: `utils.modelli`); **visibility gated** by `utils.abilita_controlli_ai()` |
| Tab 2: `Text2` | TEXT_WIDGET | Usage stats display (prompt/completion/total tokens); **visibility gated** |

**Queries**:

| Query | Datasource | executeOnLoad | Purpose |
|---|---|---|---|
| `get_ultimi_importi_tel` | grappa (MySQL) | Yes | Fetch latest billing period telephone charges with customer details |
| `check_ordine_voce` | mistra | Yes | Fetch active voice service order rows (`codice_prodotto = 'CDL-TVOCE'`) |
| `xcall` | openrouter | No | Proxy for OpenRouter API calls (body: `this.params.richiesta`) |
| `collega_ordini` | JS (UNUSED_DATASOURCE) | Yes | JSObject method — cross-references billing data with ERP orders |
| `analizza` | JS (UNUSED_DATASOURCE) | No | JSObject method — triggers AI analysis |
| `abilita_controlli_ai` | JS (UNUSED_DATASOURCE) | No | JSObject method — gates AI features to user `sciacco` only |

**Event flow**:
1. Page load → `check_ordine_voce` + `get_ultimi_importi_tel` run → `collega_ordini()` enriches and cross-references data → populates `utils.conti` → Tab 1 table
2. User switches to Tab 2 (AI analysis):
   - `sl_modello` and `Button1` only visible if `utils.abilita_controlli_ai()` returns true (user email contains "sciacco")
   - User selects AI model from dropdown
   - Button1 → `utils.analizza()`:
     - Calls `collega_ordini()` to get enriched data
     - Constructs prompt from `task_prompt` + JSON data
     - Calls `utils.ai_request()` which POSTs to OpenRouter via `xcall`
     - Stores response in `utils.analisi` (rendered as HTML in Text1)
     - Stores token usage in `utils.usage`

**Hidden logic**:
- **CRITICAL BUSINESS RULE** — `collega_ordini()` cross-referencing logic:
  ```javascript
  get_ultimi_importi_tel.data.map(conto => {
      const ordine_presente = check_ordine_voce.data.find(riga => riga.serialnumber === sn);
      return {
          ...conto,
          ordine_presente: ordine_presente?.codice_prodotto ? 'SI' : 'NO',
          numero_ordine_corretto: ordine_presente?.nome_testata_ordine === co ? 'SI' : 'NO'
      }
  })
  ```
  Matches billing records to ERP orders by **serial number** and checks if the order code matches.
- **Access control**: `abilita_controlli_ai()` = `appsmith.user.email.toLowerCase().includes('sciacco')` — **user-level feature gate** hardcoded to a single user.
- **AI prompt engineering**: Detailed Italian-language validation rules embedded as a JSObject variable (`task_prompt`). Defines 6 anomaly types with corrective actions.
- **OpenRouter integration**: Generic `ai_request()` method constructs `{model, messages, max_tokens, temperature}` payload. Default model: `google/gemini-2.5-flash-lite-preview-06-17`.
- **Model selector**: 11 models available (GPT-4.1 Nano, Gemini 2.5 Flash/Pro, Gemma 2 9B, DeepSeek R1, Mistral Small, Llama 3.2/3.3, Mercury).
- **Cross-database query**: `get_ultimi_importi_tel` hits Grappa MySQL; `check_ordine_voce` hits Mistra PostgreSQL. The join happens client-side in JS.
- **Vestigial JSObject**: `_$js_openrouter1$_js_openrouter` has an empty body — likely a leftover from earlier OpenRouter integration attempt.

**SQL** (get_ultimi_importi_tel — Grappa/MySQL):
```sql
select it.conto, lastname, firstname, is_da_fatturare, codice_ordine, serialnumber,
       it.periodo_inizio, it.importo, it.stato, it.tipologia, ct.id_cliente, ac.intestazione
from importi_telefonici it
         left join conti_telefonici ct on ct.conto = it.conto
         left join grappa.cli_fatturazione ac on ct.id_cliente = ac.id
where periodo_inizio = (select periodo_inizio from importi_telefonici order by id desc limit 1);
```

**SQL** (check_ordine_voce — Mistra/PostgreSQL):
```sql
SELECT *
from loader.erp_righe_ordini ero
where codice_prodotto = 'CDL-TVOCE' and data_cessazione = '0001-01-01 00:00:00.000000'
order by cliente;
```

---

### 2.7 Accounting TIMOO daily

**Purpose**: Daily user and service extension accounting per TIMOO tenant, enriched with tenant names from the TIMOO API.

**Widgets**:

| Widget | Type | Role |
|---|---|---|
| `Table1` | TABLE_WIDGET_V2 | Accounting data table (source: `utils.accountingData`) |
| `Text1` | TEXT_WIDGET | Label: "Elenco consistenze giornaliere utenti e service extension per tenant TIMOO" |

**Queries**:

| Query | Datasource | executeOnLoad | Purpose |
|---|---|---|---|
| `getAnisettaTenants` | anisetta | Yes | Fetch all TIMOO tenant records from `as7_tenants` |
| `getTimooAPI` | TIMOO API | Yes | Fetch tenant org units from TIMOO API (dynamic URL) |
| `getUsersSEbyDay` | anisetta | No | Fetch daily user/SE stats for last 3 months |
| `reportData` | JS (UNUSED_DATASOURCE) | Yes | JSObject method — orchestrates data assembly |

**Event flow**:
1. Page load → `getAnisettaTenants` runs → `reportData()` auto-fires:
   - Calls `listaTenants()` → `generaTenantIdList()` builds TIMOO API URL with tenant IDs
   - Calls `getTimooAPI` with the constructed URL → gets tenant names
   - Calls `getUsersSEbyDay` → gets daily stats
   - Merges tenant names into stats data → stores in `utils.accountingData`
2. Table displays merged data

**Hidden logic**:
- **API orchestration**: Multi-step data assembly: Anisetta DB → TIMOO API → Anisetta DB → client-side merge.
- **Business rule**: URL construction pattern: `/orgUnits?where=type.eq('tenant').and(id.in({ids})).and(name.ne('KlajdiandCo'))` — filters out a specific test tenant (`KlajdiandCo`).
- **Business rule**: Date window: last 3 full months (`DATE_TRUNC('month', CURRENT_DATE - INTERVAL '3 month')` to `CURRENT_DATE`).
- **Business rule**: Stats aggregation: `MAX(users)` and `MAX(service_extensions)` per PBX per day, then summed across PBXs per tenant per day.
- **Fragile binding**: `getTimooAPI` uses a dynamic path (`{{this.params.URL}}`) passed from JSObject.

**SQL** (getUsersSEbyDay):
```sql
select accounting.as7_tenant_id as tenant_id, giorno as day,
       sum(users) as users, sum(service_extensions) as service_extensions
from (
    SELECT as7_tenant_id, pbx_id,
           DATE(data) AS giorno,
           MAX(users) AS users,
           MAX(service_extensions) AS service_extensions
    FROM as7_pbx_accounting
    where data >= DATE_TRUNC('month', CURRENT_DATE - INTERVAL '3 month')
      AND data < CURRENT_DATE
    GROUP BY as7_tenant_id, pbx_id, DATE(data)
    order by giorno
) accounting
group by accounting.as7_tenant_id, giorno
order by giorno desc, as7_tenant_id;
```

**SQL** (getAnisettaTenants):
```sql
SELECT * FROM public."as7_tenants";
```

---

### 2.8 AOV (Annual Order Value)

**Purpose**: Multi-view AOV analysis: by order type, by product category, by sales rep, and full detail. With XLSX export.

**Widgets**:

| Widget | Type | Role |
|---|---|---|
| `CNT_search` | CONTAINER_WIDGET | Filter bar |
| `i_from` | DATE_PICKER_WIDGET2 | Start date (default: `moment().subtract(1,'month')`) |
| `i_to` | DATE_PICKER_WIDGET2 | End date (default: `moment().subtract(1,'days')`) |
| `ms_o_stati` | MULTI_SELECT_WIDGET_V2 | Order status filter (source: `get_stati_ordine.data`) |
| `Button1` | BUTTON_WIDGET | onClick: runs all 4 data queries in parallel |
| `CNT_AOV` | CONTAINER_WIDGET | General AOV report section |
| `TBL_aov` | TABLE_WIDGET_V2 | AOV by order type + year/month (source: `get_report_data_tipo_ord.data`) |
| `CNT_article_area` | CONTAINER_WIDGET | Category report section |
| `TBL_area` | TABLE_WIDGET_V2 | AOV by product category + year/month (source: `get_report_data_area.data`) |
| `CNT_sales` | CONTAINER_WIDGET | Sales rep report section |
| `TBL_sales` | TABLE_WIDGET_V2 | AOV by sales rep + order type (source: `get_report_data_sales.data`) |
| `CNT_dati` | CONTAINER_WIDGET | Detail data section |
| `Table1` | TABLE_WIDGET_V2 | Full order detail (source: `get_report_data.data`) |

**Queries**:

| Query | Datasource | executeOnLoad | Purpose |
|---|---|---|---|
| `get_stati_ordine` | mistra | Yes | Distinct order statuses (same query as Ordini page) |
| `get_report_data_tipo_ord` | mistra | Yes | AOV aggregated by year/month/order type |
| `get_report_data_area` | mistra | Yes | AOV aggregated by year/month/product category |
| `get_report_data_sales` | mistra | Yes | AOV aggregated by year/salesperson/order type |
| `get_report_data` | mistra | Yes | Full order-level detail with AOV computation |
| `render_template` | carbone.io | No | XLSX export via Carbone.io |

**Event flow**:
1. Page load → all 5 mistra queries run → 4 tables populated
2. User adjusts filters → clicks Button1 → all 4 data queries re-run in parallel
3. XLSX export (not wired to Button1): `utils.runReport()` — runs `get_report_data_tipo_ord`, sends to Carbone.io, opens download URL

**Hidden logic**:
- **CRITICAL BUSINESS RULE — AOV calculation** (repeated across 4 queries with slight variations):
  - `valore_aov` = `totale_mrc_new * 12 + totale_nrc`
  - For **new orders** (`tipo_ordine = 'N'`): `totale_mrc_new = MRC`, `totale_nrc = setup`
  - For **substitutions** (`tipo_ordine = 'A'`): `totale_mrc_new = new_MRC - old_MRC` (delta from replaced order)
  - For **TSC-ORDINE** documents: MRC and NRC columns are swapped (setup stored in `canone`, recurring in `setup`)
  - Old MRC lookup: subquery on `loader.v_ordini_ric_spot` matching `sost_ord` with `/`-to-`-` normalization
- **Business rule**: Date uses `data_conferma` (confirmation date) with fallback to `data_documento` when `data_conferma = '0001-01-01 00:00:00'` (sentinel value for "not confirmed").
- **Business rule**: Sales rep lookup via HubSpot data: `loader.hubs_deal` joined with `loader.hubs_owner`, matching on order code with `/`-to-`-` normalization. Falls back to `'CP'` when no match.
- **Business rule**: Product category lookup: `products.product_category` joined via `products.product.code = o.codice_prodotto` (only in `get_report_data_area`).
- **Business rule**: Order type mapping: N→NUOVO, A→SOST, R→RINNOVO, C→CESSAZIONE.
- **Duplicated SQL**: The 4 data queries share ~80% of the same SQL logic (base query, WHERE clause, CASE expressions) with different GROUP BY and SELECT variations. The AOV/MRC/NRC computation is copy-pasted with minor differences.
- **Inconsistency**: `get_report_data_area` does NOT subtract old MRC for substitutions in its `valore_aov` calculation — it computes `(quantita * canone) * 12 + (quantita * setup)` regardless of `tipo_ordine`. This means the "area" view may show different totals than the "tipo_ord" view for the same data.

---

## 3. Datasource and Query Catalog

### 3.1 mistra (PostgreSQL)

Primary database for order, customer, and access line data. All queries use the `loader` schema.

| Query | Page | Read/Write | Key Tables/Views | Parameters | Rewrite Recommendation |
|---|---|---|---|---|---|
| `get_stati_ordine` | Ordini, AOV | Read | `loader.v_ordini_ric_spot` | None | Backend API: `GET /api/reports/order-statuses` |
| `get_report_data` (Ordini) | Ordini | Read | `loader.v_ordini_ric_spot`, `loader.erp_anagrafiche_clienti` | status[], dateFrom, dateTo | Backend API: `GET /api/reports/orders` |
| `get_accessi` | Accessi attivi | Read | `loader.grappa_foglio_linee`, `loader.grappa_cli_fatturazione`, `loader.grappa_profili`, `loader.v_ordini_ricorrenti` | status[], tipoConn[] | Backend API: `GET /api/reports/active-lines` |
| `get_tipo_conn` | Accessi attivi | Read | `loader.grappa_foglio_linee` | None | Backend API: `GET /api/reports/connection-types` |
| `get_confirmed_orders` | Attivazioni in corso | Read | `loader.v_ordini_sintesi`, `loader.erp_anagrafiche_clienti` | None | Backend API: `GET /api/reports/pending-activations` |
| `get_rows` (Attivazioni) | Attivazioni in corso | Read | `loader.v_ordini_sintesi`, `loader.erp_anagrafiche_clienti` | orderNumber | Backend API: `GET /api/reports/pending-activations/{orderId}/rows` |
| `get_aggregato_scadenze` | Rinnovi in arrivo | Read | `loader.v_ordini_ricorrenti_conrinnovo` | months, minMrc | Backend API: `GET /api/reports/upcoming-renewals` |
| `get_rows` (Rinnovi) | Rinnovi in arrivo | Read | `loader.v_ordini_ricorrenti_conrinnovo` | months, minMrc, customerId | Backend API: `GET /api/reports/upcoming-renewals/{customerId}/rows` |
| `check_ordine_voce` | Anomalie MOR | Read | `loader.erp_righe_ordini` | None | Backend API: `GET /api/reports/voice-orders` |
| `get_report_data` (AOV) | AOV | Read | `loader.v_ordini_ric_spot`, `loader.erp_anagrafiche_clienti`, `loader.hubs_deal`, `loader.hubs_owner` | status[], dateFrom, dateTo | Backend API: `GET /api/reports/aov/detail` |
| `get_report_data_tipo_ord` | AOV | Read | (same as above) | status[], dateFrom, dateTo | Backend API: `GET /api/reports/aov/by-type` |
| `get_report_data_area` | AOV | Read | (same + `products.product_category`, `products.product`) | status[], dateFrom, dateTo | Backend API: `GET /api/reports/aov/by-category` |
| `get_report_data_sales` | AOV | Read | (same as get_report_data) | status[], dateFrom, dateTo | Backend API: `GET /api/reports/aov/by-sales` |

### 3.2 grappa (MySQL)

Used only by Anomalie MOR page for telephone billing data.

| Query | Page | Read/Write | Key Tables | Parameters | Rewrite Recommendation |
|---|---|---|---|---|---|
| `get_ultimi_importi_tel` | Anomalie MOR | Read | `importi_telefonici`, `conti_telefonici`, `cli_fatturazione` | None (auto-detects latest period) | Backend API: `GET /api/reports/phone-charges/latest` |

### 3.3 anisetta (PostgreSQL)

TIMOO/PBX accounting database.

| Query | Page | Read/Write | Key Tables | Parameters | Rewrite Recommendation |
|---|---|---|---|---|---|
| `getAnisettaTenants` | Accounting TIMOO | Read | `as7_tenants` | None | Backend API: `GET /api/reports/timoo/tenants` |
| `getUsersSEbyDay` | Accounting TIMOO | Read | `as7_pbx_accounting` | None (last 3 months) | Backend API: `GET /api/reports/timoo/daily-stats` |

### 3.4 carbone.io (REST API)

XLSX template rendering service.

| Query | Page | Read/Write | Endpoint | Parameters | Rewrite Recommendation |
|---|---|---|---|---|---|
| `render_template` | Ordini, Accessi attivi, AOV | Write (POST) | Carbone.io render API | templateId, reportName, data | Backend service: keep as external API call from backend, not frontend |

### 3.5 openrouter (REST API)

AI/LLM gateway for anomaly analysis.

| Query | Page | Read/Write | Endpoint | Parameters | Rewrite Recommendation |
|---|---|---|---|---|---|
| `xcall` | Anomalie MOR | Write (POST) | OpenRouter chat completions | model, messages, max_tokens, temperature | Backend service: proxy through backend API to hide API key |

### 3.6 TIMOO API (REST API)

Internal TIMOO PBX management API.

| Query | Page | Read/Write | Endpoint | Parameters | Rewrite Recommendation |
|---|---|---|---|---|---|
| `getTimooAPI` | Accounting TIMOO | Read (GET) | Dynamic URL: `/orgUnits?where=...` | URL (constructed by JSObject) | Backend API: `GET /api/reports/timoo/org-units` |

---

## 4. Findings Summary

### 4.1 Embedded Business Rules

| # | Rule | Location | Classification |
|---|---|---|---|
| BR1 | AOV calculation: `MRC_new * 12 + NRC`, with substitution delta logic and TSC-ORDINE column swap | AOV queries (4 copies) | **Business logic** — must move to backend |
| BR2 | Date fallback: use `data_conferma`, fall back to `data_documento` when sentinel `0001-01-01` | AOV queries | **Business logic** |
| BR3 | Order type mapping: N→NUOVO, A→SOST, R→RINNOVO, C→CESSAZIONE | Multiple queries | **Business logic** |
| BR4 | Sales rep lookup via HubSpot with `/`-to-`-` order code normalization, default `'CP'` | AOV get_report_data, get_report_data_sales | **Business logic** |
| BR5 | Bandwidth classification: `banda_up != banda_down` → CONDIVISA, else DEDICATA | Accessi attivi get_accessi | **Business logic** |
| BR6 | Renewal window: `current_date - 15 days` to `current_date + N months` | Rinnovi queries | **Business logic** |
| BR7 | Renewal filter: `durata_rinnovo > 3 OR tacito_rinnovo = 0` | Rinnovi queries | **Business logic** |
| BR8 | MOR anomaly detection rules (6 validation types) | Anomalie utils.task_prompt | **Business logic** (currently delegated to AI) |
| BR9 | Cross-reference billing → ERP by serial number | Anomalie utils.collega_ordini | **Business logic** |
| BR10 | AI feature access gate: email contains "sciacco" | Anomalie abilita_controlli_ai | **Access control** — must move to proper RBAC |
| BR11 | Test tenant exclusion: `name.ne('KlajdiandCo')` | Accounting TIMOO utils.generaTenantIdList | **Business logic** |
| BR12 | DB function `loader.get_reverse_order_history_path()` | Attivazioni get_confirmed_orders | **Business logic** (in database) |

### 4.2 Duplication

| Issue | Details |
|---|---|
| **AOV SQL duplication** | 4 queries (`get_report_data`, `get_report_data_tipo_ord`, `get_report_data_area`, `get_report_data_sales`) share ~80% identical SQL. The AOV/MRC/NRC CASE logic is copy-pasted with subtle differences. |
| **Export pattern duplication** | `utils.getURL()` and `utils.runReport()` are nearly identical across Ordini, Accessi attivi, and AOV pages (3 copies). |
| **`get_stati_ordine` duplication** | Identical query on Ordini and AOV pages. |
| **Rinnovi WHERE clause duplication** | `get_aggregato_scadenze` and `get_rows` repeat the same complex WHERE clause. |

### 4.3 Security Concerns

| Issue | Severity | Details |
|---|---|---|
| **SQL injection risk** | HIGH | All queries interpolate widget values directly into SQL via `{{widget.value}}` without parameterization. The multi-select widgets construct IN-clause values by string concatenation: `map(i => "'" + i + "'").join()`. While Appsmith may sanitize internally, this pattern must not be replicated in a custom backend. |
| **API keys in frontend** | HIGH | OpenRouter API key and Carbone.io API key are configured as datasource credentials. In a rewrite, these must be called from the backend. |
| **Hardcoded access control** | MEDIUM | AI feature gated by email string match (`sciacco`), not by role/permission. |
| **Direct database access** | MEDIUM | Frontend queries 3 databases directly. All should be proxied through backend APIs. |

### 4.4 Migration Blockers

| # | Blocker | Details |
|---|---|---|
| MB1 | **AOV calculation inconsistency** | `get_report_data_area` computes `valore_aov` differently from the other 3 AOV queries — it does NOT subtract old MRC for substitutions. Need to verify which is correct before implementing. |
| MB2 | **Database views not in schema dumps** | Queries reference views (`v_ordini_ric_spot`, `v_ordini_sintesi`, `v_ordini_ricorrenti`, `v_ordini_ricorrenti_conrinnovo`) and a function (`get_reverse_order_history_path`) in the `loader` schema. These must be documented or their definitions obtained. |
| MB3 | **Carbone.io template IDs** | Two hardcoded template IDs. The templates themselves (XLSX layouts) are external to this export and must be preserved/migrated. |
| MB4 | **Cross-database client-side join** | Anomalie page joins Grappa MySQL data with Mistra PostgreSQL data in JavaScript. Backend must handle this cross-DB join. |
| MB5 | **TIMOO API contract** | The TIMOO API query language (`type.eq('tenant').and(...)`) is undocumented in this export. Need API spec. |
| MB6 | **6 missing pages in git export** | Only AOV and Accessi attivi were in the zip. The single-file JSON export covered all 8 pages. Verify the git export is not the primary source for future development. |

### 4.5 Fragile Bindings

| Binding | Risk |
|---|---|
| `render_template.data.data.renderId` | Deep nested path — if Carbone.io response structure changes, breaks silently |
| `tbl_ordini.selectedRow.numero_ordine` | Detail query depends on selected row state; no fallback if nothing selected |
| `Table1.selectedRow.numero_azienda` | Same issue on Rinnovi page |
| `this.params.URL` / `this.params.richiesta` | Dynamic params passed to API queries — opaque calling convention |

### 4.6 Candidate Domain Entities

- **Order** (`nome_testata_ordine`, `tipo_ordine`, `stato_ordine`, `stato_riga`, etc.)
- **Customer** (`ragione_sociale`, `numero_azienda`, `codice_aggancio_gest`)
- **Order Line** (`descrizione_long`, `quantita`, `canone`, `setup`, `serialnumber`)
- **Access Line** (`grappa_foglio_linee`: `tipo_conn`, `fornitore`, `serialnumber`, `stato`)
- **Bandwidth Profile** (`grappa_profili`: `tipo`, `profilo_commerciale`, `banda_up`, `banda_down`)
- **Sales Rep** (via HubSpot: `hubs_owner.first_name`, `hubs_owner.last_name`)
- **Phone Billing Record** (`importi_telefonici`: `conto`, `importo`, `periodo_inizio`)
- **TIMOO Tenant** (`as7_tenants`, org units from API)

### 4.7 Recommended Next Steps

1. **Clarify AOV calculation**: Resolve the `get_report_data_area` inconsistency before implementing any AOV backend.
2. **Obtain view/function definitions**: Get DDL for `loader.v_ordini_ric_spot`, `v_ordini_sintesi`, `v_ordini_ricorrenti`, `v_ordini_ricorrenti_conrinnovo`, and `get_reverse_order_history_path()`.
3. **Design backend API layer**: All 6 datasource interactions should be proxied through Go backend endpoints. The AOV queries should be consolidated into a single parameterized query or stored procedure.
4. **Extract and centralize business rules**: AOV calculation, order type mapping, date fallback logic, renewal filtering, and MOR anomaly rules should be codified in the backend, not embedded in SQL or frontend JS.
5. **Handle Carbone.io integration**: Move template rendering to backend; store template IDs in configuration.
6. **Handle OpenRouter integration**: Proxy AI calls through backend; implement proper RBAC instead of email-based gate.
7. **Run `appsmith-migration-spec`** on this audit to produce the detailed migration specification.
