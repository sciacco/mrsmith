# Application Specification — Panoramica Cliente

## Summary

- **Application name:** panoramica-cliente
- **Audit source:** Appsmith Git export (`panoramica-cliente-main.zip`)
- **Spec status:** Complete — all questions resolved
- **Datasources:** Mistra (PostgreSQL, `loader` schema), Grappa (MySQL), Anisetta (PostgreSQL)
- **Pages:** 7 (Dashboard excluded — WIP, deferred to `docs/TODO.md`)
- **Nature:** Entirely read-only — no writes to any database or external system
- **Deployment:** All pages developed and deployed together as a complete app
- **Coexistence:** Runs alongside Appsmith during transition; both access same databases

---

## Entity Catalog

### Entity: Customer (cross-database, read-only)

- **Purpose:** Lookup entity for all pages. Never written by this app.
- **Identity:** Mistra `loader.erp_*.numero_azienda` = Alyante ERP ID. Grappa `cli_fatturazione.id` = internal Grappa ID. Bridge: `cli_fatturazione.codice_aggancio_gest` = ERP ID. See `docs/IMPLEMENTATION-KNOWLEDGE.md`.
- **Operations:**

| Endpoint | DB | Filter | Used by |
|----------|----|--------|---------|
| `GET .../customers/with-invoices` | Mistra | ORDER BY ragione_sociale | Fatture |
| `GET .../customers/with-orders` (variant A) | Mistra | Active (dismissed filter with IS NULL), no "Tutti i clienti" | Ordini ricorrenti |
| `GET .../customers/with-orders` (variant B) | Mistra | Active (dismissed filter without IS NULL), no "Tutti i clienti" | Ordini R&S |
| `GET .../customers/with-access-lines` | Mistra | Active Grappa clients with access lines | Accessi |

**Original queries:**

```sql
-- get_clienti_con_fatture (Fatture page)
-- Datasource: mistra (postgres), executeOnLoad: true
select * from loader.erp_clienti_con_fatture
order by ragione_sociale
```

```sql
-- get_aziende_con_ordini (Ordini ricorrenti page)
-- Datasource: mistra (postgres), executeOnLoad: true
-- NOTE: "TUTTI I CLIENTI" row REMOVED in migration — customer always required
select distinct odv.numero_azienda, odv.ragione_sociale
from loader.v_ordini_ricorrenti as odv
JOIN loader.erp_anagrafiche_clienti AS cli ON cli.numero_azienda = odv.numero_azienda
  AND (cli.data_dismissione >= NOW() OR cli.data_dismissione='0001-01-01 00:00:00' OR cli.data_dismissione IS NULL)
order by ragione_sociale
```

```sql
-- GET_aziendeConOrdini (Ordini R&S page)
-- Datasource: mistra (postgres), executeOnLoad: true
-- NOTE: no IS NULL check (different from variant A); "TUTTI I CLIENTI" row REMOVED
select distinct odv.numero_azienda, odv.ragione_sociale
from loader.v_ordini_ricorrenti as odv
JOIN loader.erp_anagrafiche_clienti AS cli ON cli.numero_azienda = odv.numero_azienda
  AND (cli.data_dismissione >= NOW() OR cli.data_dismissione='0001-01-01 00:00:00')
order by ragione_sociale
```

```sql
-- get_clients_accessi (Accessi page)
-- Datasource: mistra (postgres), executeOnLoad: true
select distinct cf.id, cf.intestazione
from loader.grappa_foglio_linee fl join loader.grappa_cli_fatturazione cf on fl.id_anagrafica = cf.id
where cf.codice_aggancio_gest is not null and cf.stato = 'attivo'
order by cf.intestazione
```

---

### Entity: Invoice / Credit Note (read-only)

- **Purpose:** Browse invoice/credit note line items for a customer within a time period.
- **Operations:** `listInvoiceLines(cliente, mesi)`
- **Endpoint:** `GET .../invoices?cliente=X&mesi=N` (mesi=null → no date filter)

**Original query:**

```sql
-- get_fatture (Fatture page)
-- Datasource: mistra (postgres), executeOnLoad: false, preparedStatement: false
select CASE WHEN rn = 1 THEN doc || ' ' || num_documento || CHR(13) || CHR(10) ||to_char(data_documento, '(YYYY-MM-DD)') ELSE NULL END AS documento,
       descrizione_riga, qta, prezzo_unitario, prezzo_totale_netto, codice_articolo,
       data_documento, num_documento, id_cliente, progressivo_riga, serialnumber,
       riferimento_ordine_cliente, condizione_pagamento, scadenza, desc_conto_ricavo,
       gruppo, sottogruppo, rn
from loader.v_erp_fatture_nc
WHERE id_cliente = :cliente and (data_documento >= current_date - interval ':mesi months')
order by anno_documento desc, mese_documento desc, tipo_documento, num_documento, rn
```

- **Fields (visible):** documento, descrizione_riga, qta, prezzo_unitario (EUR), prezzo_totale_netto (EUR), codice_articolo, serialnumber, riferimento_ordine_cliente, condizione_pagamento, scadenza (DD-MM-YYYY), desc_conto_ricavo, gruppo, sottogruppo
- **Fields (hidden):** data_documento, num_documento, id_cliente, progressivo_riga, rn
- **Business rules:**
  - `segno` column: +1 invoice, -1 credit note (used in aggregations, not selected in this query)
  - Visual grouping: `documento` computed only on `rn = 1` (backend SQL, as-is)
  - Period: backend accepts `null` as "no date filter" (replaces 2000-month hack)

---

### Entity: Order — Summary (read-only)

- **Purpose:** Browse recurring orders with customer/status filters and order history chain.
- **Operations:** `listOrderStatuses`, `listOrdersSummary(cliente, stati[])`
- **Endpoints:** `GET .../order-statuses`, `GET .../orders/summary?cliente=X&stati=...`

**Original queries:**

```sql
-- get_stati_ordine (Ordini ricorrenti page)
-- Datasource: mistra (postgres), executeOnLoad: true
select distinct stato_ordine
from loader.v_ordini_ricorrenti
order by stato_ordine
```

```sql
-- get_ordini_ricorrenti (Ordini ricorrenti page)
-- Datasource: mistra (postgres), executeOnLoad: false, preparedStatement: false, timeout: 20000ms
-- NOTE: in migration, customer is always required (no -1/"all clients" option)
SELECT stato, numero_ordine, descrizione_long, quantita, nrc, mrc, totale_mrc,
       stato_ordine, nome_testata_ordine, rn, numero_azienda, data_documento,
       stato_riga, data_ultima_fatt, serialnumber,
       metodo_pagamento, durata_servizio, durata_rinnovo, data_cessazione,
       data_attivazione, note_legali, sost_ord, sostituito_da,
       loader.get_reverse_order_history_path(nome_testata_ordine) as storico
from loader.v_ordini_sintesi
where numero_azienda = :cliente
and stato_ordine in (:stati)
order by data_documento, nome_testata_ordine, rn
```

---

### Entity: Order — Detail (read-only)

- **Purpose:** Full-detail order viewer — recurring and spot orders with lifecycle, referents, product families, computed `stato_riga`.
- **Operations:** `listOrderStatuses` (shared), `listOrdersDetail(cliente, stati[])`
- **Endpoints:** `GET .../order-statuses` (shared), `GET .../orders/detail?cliente=X&stati=...`

**Original query:**

```sql
-- GET_ordini_Ric_Spot (Ordini R&S page)
-- Datasource: mistra (postgres), executeOnLoad: false, preparedStatement: false
SELECT c.ragione_sociale,
    CASE WHEN o.data_conferma > o.data_documento THEN o.data_conferma ELSE o.data_documento END AS data_ordine,
    o.nome_testata_ordine, o.cliente, o.numero_azienda, o.id_gamma, o.commerciale,
    o.data_documento, o.data_conferma, o.stato_ordine, o.tipo_ordine, o.tipo_documento,
    o.sost_ord, o.riferimento_odv_cliente, o.durata_servizio, o.tacito_rinnovo,
    o.durata_rinnovo, o.tempi_rilascio, o.metodo_pagamento, o.note_legali,
    o.referente_amm_nome, o.referente_amm_mail, o.referente_amm_tel,
    o.referente_tech_nome, o.referente_tech_mail, o.referente_tech_tel,
    o.referente_altro_nome, o.referente_altro_mail, o.referente_altro_tel,
    o.data_creazione, o.data_variazione, o.sostituito_da,
    r.quantita, r.codice_kit, r.codice_prodotto, r.descrizione_prodotto, r.descrizione_estesa,
    r.serialnumber, r.setup, r.canone, r.valuta, r.costo_cessazione,
    NULLIF(r.data_attivazione, '0001-01-01 00:00:00'::timestamp) AS data_attivazione,
    NULLIF(r.data_disdetta, '0001-01-01 00:00:00'::timestamp) AS data_disdetta,
    NULLIF(r.data_cessazione, '0001-01-01 00:00:00'::timestamp) AS data_cessazione,
    r.raggruppamento_fatturazione, r.intervallo_fatt_attivazione, r.intervallo_fatt_canone,
    NULLIF(r.data_ultima_fatt, '0001-01-01 00:00:00'::timestamp) AS data_ultima_fatt,
    NULLIF(r.data_fine_fatt, '0001-01-01 00:00:00'::timestamp) AS data_fine_fatt,
    r.system_odv_row, r.id_gamma_testata, r.progressivo_riga,
    CASE WHEN r.progressivo_riga = 1 THEN o.nome_testata_ordine ELSE NULL END AS ORDINE,
    r.annullato,
    NULLIF(r.data_scadenza_ordine, '0001-01-01 00:00:00'::timestamp) AS data_scadenza_ordine,
    r.quantita * r.canone AS mrc,
    p.famiglia, p.sotto_famiglia, p.desc_conto_ricavo AS conto_ricavo,
    CASE
        WHEN o.stato_ordine = 'Cessato' THEN 'Cessata'
        WHEN o.stato_ordine = 'Bloccato' THEN 'Bloccata'
        WHEN o.stato_ordine = 'Confermato' AND date_part('year', r.data_attivazione) = 1 THEN 'Da attivare'
        WHEN o.stato_ordine = 'Confermato' AND date_part('year', r.data_attivazione) > 1 THEN 'Attiva'
        WHEN r.annullato = 1 THEN 'Annullata'
        WHEN date_part('year', r.data_cessazione) = 1 THEN 'Attiva'
        WHEN r.data_cessazione >= '0001-01-01'::timestamp AND r.data_cessazione <= now() THEN 'Cessata'
        WHEN r.data_cessazione > now() THEN 'Cessazione richiesta'
        ELSE 'Unknown'
    END AS stato_riga,
    o.nome_testata_ordine || ' del ' || to_char(o.data_documento, 'YYYY-MM-DD') || ' (' || o.stato_ordine || ')' AS intestazione_ordine,
    CASE
        WHEN r.descrizione_prodotto = r.descrizione_estesa OR r.descrizione_estesa IS NULL OR r.descrizione_estesa = '' THEN r.descrizione_prodotto
        ELSE r.descrizione_prodotto || chr(13) || chr(10) || r.descrizione_estesa
    END AS descrizione_long
FROM loader.erp_ordini o
  JOIN loader.erp_righe_ordini r ON o.id_gamma = r.id_gamma_testata
  JOIN loader.erp_anagrafiche_clienti c ON o.numero_azienda = c.numero_azienda
  LEFT JOIN loader.erp_anagrafica_articoli_vendita p ON r.codice_prodotto = btrim(p.cod_articolo)
WHERE c.numero_azienda = :cliente
  AND o.stato_ordine IN (:stati)
  AND r.codice_prodotto <> 'CDL-AUTO'
ORDER BY o.nome_testata_ordine, o.data_documento DESC
```

- **Critical business rule — `stato_riga`:** 8-way CASE (see query above). Year=1 is sentinel for `0001-01-01` = "not set". Must be preserved exactly in backend SQL.
- **Other business rules:** CDL-AUTO exclusion, sentinel date → NULL via NULLIF, `data_ordine` = MAX(conferma, documento), `mrc` = quantita * canone

---

### Entity: AccessLine (read-only)

- **Purpose:** Browse connectivity access lines with multi-client, status, and connection type filters.
- **Operations:** `listConnectionTypes`, `listAccessLines(clienti[], stati[], tipi_conn[])`
- **Endpoints:** `GET .../connection-types`, `GET .../access-lines?clienti=...&stati=...&tipi=...`

**Original queries:**

```sql
-- get_tipo_conn (Accessi page)
-- Datasource: mistra (postgres), executeOnLoad: true
select distinct tipo_conn from loader.grappa_foglio_linee order by tipo_conn;
```

```sql
-- get_accessi_cliente (Accessi page)
-- Datasource: mistra (postgres), executeOnLoad: true, preparedStatement: false
SELECT
    tipo_conn, fl.fornitore, provincia, comune, p.tipo, p.profilo_commerciale,
    cogn_rsoc_intest_linea AS intestatario, r.nome_testata_ordine ordine,
    r.data_ultima_fatt fatturato_fino_al, r.stato_riga, r.stato_ordine,
    fl.stato, fl.id, codice_ordine, fl.serialnumber, cf.codice_aggancio_gest AS id_anagrafica
FROM loader.grappa_foglio_linee fl
  JOIN loader.grappa_cli_fatturazione cf ON fl.id_anagrafica = cf.id
  LEFT JOIN loader.grappa_profili p ON fl.id_profilo = p.id
  LEFT JOIN (
    SELECT *, ROW_NUMBER() OVER(PARTITION BY serialnumber ORDER BY data_documento DESC, progressivo_riga) AS rn
    FROM loader.v_ordini_ricorrenti
  ) r ON fl.serialnumber = r.serialnumber AND r.numero_azienda = cf.codice_aggancio_gest AND r.rn = 1
WHERE fl.id_anagrafica IN (:clienti)
  AND fl.stato IN (:stati)
  AND fl.tipo_conn IN (:tipi_conn)
ORDER BY tipo_conn, fl.fornitore, provincia, comune, p.tipo, p.profilo_commerciale
```

- **Status values (hardcoded):** Attiva, Cessata, da attivare, in attivazione, KO. Default: "Attiva".
- **Data source:** Mistra `loader.grappa_*` copies (not Grappa directly).

---

### Entity: IaaSAccount (read-only)

- **Purpose:** Cloudstack billing accounts with consumption time series and category breakdown over a selectable period/aggregation.
- **Operations:** `listAccounts`, `listCharges(domain, from, to, group)`, `getChargesByCategory(domain, from, to)`, `listWindowsLicenses`

**Original queries:**

```sql
-- get_cdl_accounts (IaaS PPU page)
-- Datasource: grappa (mysql), executeOnLoad: true
select c.intestazione, a.credito, domainuuid as cloudstack_domain, id_cli_fatturazione,
       abbreviazione, codice_ordine, serialnumber, data_attivazione
from cdl_accounts a
JOIN cli_fatturazione c on a.id_cli_fatturazione = c.id
WHERE id_cli_fatturazione > 0 and attivo = 1 and fatturazione = 1
  and c.codice_aggancio_gest not in (385,485)
order by intestazione
```

```sql
-- get_charges (IaaS PPU page) — aggregated time series
-- Datasource: grappa (mysql). group ∈ {daily, weekly, monthly, quarterly, yearly}.
-- usage_type 9999 (Credit) is excluded from totals.
SELECT <bucket_expr> AS bucket,
    CAST(SUM(CASE WHEN usage_type = 9999 THEN 0 ELSE usage_charge END) AS DECIMAL(12,2)) AS total_importo
FROM cdl_charges
WHERE domainid = :domain AND charge_day BETWEEN :from AND :to
GROUP BY bucket ORDER BY bucket ASC
-- bucket_expr:
--   daily     = charge_day
--   weekly    = DATE(DATE_SUB(charge_day, INTERVAL WEEKDAY(charge_day) DAY))
--   monthly   = DATE_FORMAT(charge_day, '%Y-%m-01')
--   quarterly = DATE(CONCAT(YEAR(charge_day), '-', LPAD((QUARTER(charge_day)-1)*3+1, 2, '0'), '-01'))
--   yearly    = DATE_FORMAT(charge_day, '%Y-01-01')
```

```sql
-- get_charges_by_category (IaaS PPU page) — macro-category composition over a period
-- Datasource: grappa (mysql). total = SUM of returned categories (computed in Go).
-- usage_type 9999 (Credit) is excluded.
SELECT
  CASE
    WHEN usage_type = 2 THEN 'VM'
    WHEN usage_type IN (6,7,8,9) THEN 'Storage'
    WHEN usage_type = 9998 THEN 'Licenze Windows'
    ELSE 'Altro'
  END AS category,
  CAST(SUM(usage_charge) AS DECIMAL(12,2)) AS amount
FROM cdl_charges
WHERE domainid = :domain AND charge_day BETWEEN :from AND :to
  AND (usage_type IS NULL OR usage_type != 9999)
GROUP BY category
ORDER BY FIELD(category, 'VM', 'Storage', 'Licenze Windows', 'Altro')
```

```sql
-- get_licenses_by_day (Licenze Windows page)
-- Datasource: grappa (mysql), executeOnLoad: true
select charge_day as x, count(0) as y
from cdl_charges where charge_day >= CURDATE() - INTERVAL 14 DAY and usage_type = 9998
group by charge_day order by charge_day desc
```

- **Usage type codes:** 1=RunningVM, 2=AllocatedVM, 3=IP, 6=Volume, 7=Template, 8=ISO, 9=Snapshot, 26=VolumeSecondary, 27=VmSnapshotOnPrimary, 9998=WindowsLicense, 9999=Credit
- **Macro-category mapping (fixed backend-side):** VM=2; Storage=6,7,8,9; Licenze Windows=9998; Altro=1,3,26,27,NULL,unknown. `usage_type 9999` (Credit) is excluded from both series totals and category composition, so the four categories sum exactly to the "Totale periodo".
- **Exclusions:** `codice_aggancio_gest NOT IN (385, 485)` — as-is, not centralized

---

### Entity: TimooTenant (read-only)

- **Purpose:** View Timoo PBX tenants and PBX instance statistics.
- **Operations:** `listTenants`, `getPbxStatsByTenant(tenant_id)`
- **Endpoints:** `GET .../timoo/tenants`, `GET .../timoo/pbx-stats?tenant=X`

**Original queries:**

```sql
-- getAnisettaTenants (Timoo page)
-- Datasource: anisetta (postgres), executeOnLoad: true
-- NOTE: in migration, add WHERE name != 'KlajdiandCo' (test tenant exclusion)
SELECT * FROM public."as7_tenants"
WHERE name != 'KlajdiandCo'
```

```sql
-- getPbxByTenandId (Timoo page)
-- Datasource: anisetta (postgres), executeOnLoad: false
select as7_tenant_id, pbx_id, pbx_name, MAX(users) as users, MAX(service_extensions) as service_extensions
from public.as7_pbx_accounting apb
where as7_tenant_id = :tenant_id
  and to_char(data, 'YYYY-MM-DD') = (select to_char(data, 'YYYY-MM-DD') from public.as7_pbx_accounting order by id desc limit 1)
GROUP BY as7_tenant_id, pbx_id, pbx_name
order by pbx_name
```

- **PBX stats response:** Backend returns rows + computed totals (`totalUsers`, `totalSE`, per-row `totale = users + service_extensions`)
- **TIMOO REST API:** Excluded (too slow, replaced by Anisetta DB queries)
- **Tenants and customers:** Independent entities, no relationship

---

## View Specifications

### Navigation

Grouped horizontal tabs (`TabNavGroup` variant, same as `listini-e-sconti`). Mobile: hamburger + expandable sections.

| Group | Pages |
|-------|-------|
| **Ordini** | Ordini ricorrenti, Ordini Ricorrenti e Spot |
| **Fatture** | Fatture |
| **Servizi** | Accessi, IaaS Pay Per Use, Timoo tenants, Licenze Windows |

---

### View: Ordini ricorrenti

- **User intent:** Browse recurring orders in summary form per customer.
- **Interaction pattern:** Master-Detail Drawer — flat table with visual row grouping + slide-over panel (480px) on row click.

**Filter bar:** Customer selector (required, searchable) + status multi-select (default: Evaso, Confermato) + "Cerca" button.

**Main table:** Flat rows (line items). First row of each order: taller (48px), bold `nome_testata_ordine` + `numero_ordine` mono, top border. Subsequent rows: indented, tree-line connector.

| Default columns | Field |
|----------------|-------|
| Stato ordine | `stato_ordine` (colored dot) |
| Numero | `numero_ordine` (mono) |
| Ordine/Descrizione | `nome_testata_ordine` / `descrizione_long` |
| Qta | `quantita` |
| NRC | `nrc` (EUR) |
| MRC | `mrc` (EUR) |
| Totale MRC | `totale_mrc` (EUR) |
| Data | `data_documento` |
| Stato riga | `stato_riga` (badge) |
| Serial | `serialnumber` |

Hidden columns (toggleable): `data_ultima_fatt`, `metodo_pagamento`, `durata_servizio`, `durata_rinnovo`, `storico`, etc.

**Slide-over panel (480px):** Header (order name + number + status) → order metadata (label/value pairs) → selected line card → sibling lines list.

**Interactions:** Click row → panel opens. Click another row → cross-fade. Arrow keys navigate. Escape closes. CSV export on table.

---

### View: Ordini Ricorrenti e Spot

- **User intent:** Full-detail order viewer per customer with referents, product families, computed stato_riga.
- **Interaction pattern:** Master-Detail Drawer — flat table + slide-over panel (600px) with 4 tabs.

**Filter bar:** Customer selector (required) + status multi-select (default: Evaso, Confermato) + "GO" button.

**Main table:** Same visual grouping as Ordini ricorrenti. Default columns: stato_ordine, ORDINE/descrizione_long, tipo_ordine, commerciale, data_ordine, quantita, mrc, stato_riga, serialnumber, codice_prodotto.

**Slide-over panel (600px) — 4 tabs:**

| Tab | Content |
|-----|---------|
| **Testata** | Anagrafica (ragione_sociale, commerciale, tipo_ordine, tipo_documento, riferimento_odv_cliente), Condizioni (tacito_rinnovo, durata_servizio, durata_rinnovo, tempi_rilascio, metodo_pagamento, note_legali), Referenti (amm/tech/altro: nome, mail, tel), Fatturazione (raggruppamento, intervalli, date scadenza/fine), Sostituzioni (sost_ord, sostituito_da, intestazione_ordine) |
| **Riga selezionata** | Prodotto (codice, kit, descrizione, estesa, famiglia, sotto_famiglia, conto_ricavo), Importi (setup, canone, mrc, nrc, costo_cessazione, valuta), Date (attivazione, disdetta, cessazione, ultima_fatt, fine_fatt, scadenza_ordine), Stato (stato_riga badge, annullato) |
| **Tutte le righe** | Mini-table: codice_prodotto, descrizione, mrc, stato_riga. Click → switches to "Riga selezionata" |
| **Storico** | Order substitution chain as vertical timeline |

---

### View: Fatture

- **User intent:** Browse invoice line items for a customer within a time period.
- **Interaction pattern:** Filter bar → auto-refresh table. No explicit button.

**Filter bar:** Customer selector (searchable, required) + period selector (6/12/24/36/Tutti). Both trigger auto-refresh.

**Table:** Invoice lines with visual grouping (document header on `rn=1` only, computed in backend SQL). Visible: Documento, Descrizione Riga, Qta, Importo Uni (EUR), Totale Riga (EUR), Codice Articolo, Serial N, Rif Cliente, Pagamento, Scadenza, Conto Ricavo, Gruppo, Sottogruppo. CSV export enabled.

---

### View: Accessi

- **User intent:** Browse connectivity access lines with multi-client and multi-filter support.
- **Interaction pattern:** Multi-filter bar → table. Manual "Cerca" button.

**Filter bar:** Multi-select clients + multi-select status (default: Attiva, hardcoded list) + multi-select connection type (dynamic, all selected by default) + "Cerca" button (standardized label).

**Table:** 16 columns. Does NOT auto-query on page load with empty selection. CSV export enabled.

---

### View: IaaS Pay Per Use

- **User intent:** Monitor Cloudstack IaaS consumption per account over selectable periods and aggregations.
- **Interaction pattern:** Dashboard a colonne. Master (account list, auto-select first) → detail with KPI, time-series chart, category pie, and bucket table.

**Master (account list):** Searchable list with accent-bar selection; shows Intestazione + Credito + Abbreviazione. `cloudstack_domain` is the hidden key. CSV export.

**Detail:**
- **Controls:** Period select (30/90/120 days, 6/12/24 months) + Aggregation select (Giornaliera/Settimanale/Mensile/Trimestrale/Annuale). Default aggregation is period-suggested but user-overridable.
- **Drill-down (zoom + breadcrumb):** Clicking a chart bar narrows the window to that bucket and lowers aggregation by one level (stops at Giornaliera). The breadcrumb records the path and supports roll-up. Selectors lock while drilling; selecting an account resets the drill.
- **KPI row:** Totale periodo + per-category amount/percent (VM, Storage, Licenze Windows, Altro). All exclude `usage_type 9999`.
- **Time-series chart:** One bar per bucket (consumption, 9999 excluded). Clickable for drill when a finer granularity exists.
- **Composition pie:** Macro-category split over the whole selected period (localized labels).
- **Bucket table:** One row per bucket (Periodo, Totale), sortable + CSV export.

---

### View: Licenze Windows su Cloudstack

- **User intent:** Monitor Windows Server license count trend (cross-client summary).
- **Interaction pattern:** Static chart, no interaction. Auto-loads on page entry.
- **Layout:** Title "Licenze Windows Server attive su Cloudstack PPU" + bar/line chart (14 days).

---

### View: Timoo tenants

- **User intent:** View PBX instance statistics per tenant.
- **Interaction pattern:** Tenant selector → auto-load PBX stats on selection.

**Layout:** Tenant selector (searchable) → summary (total users, total SE) + PBX table (PBX Name, PBX ID, Users, Service Extensions, Totale). CSV export enabled.

---

## Logic Allocation

### Backend responsibilities

| Responsibility | Details |
|---------------|---------|
| All database queries | Parameterized SQL against Mistra, Grappa, Anisetta |
| `stato_riga` computation | 8-way CASE in SQL, preserved exactly |
| `data_ordine` computation | CASE in SQL, preserved exactly |
| Sentinel date normalization | NULLIF in SQL, preserved exactly |
| Document/order grouping | CASE expressions in SQL (as-is) |
| Charge aggregation & categorization | SQL bucket by group (daily/weekly/monthly/quarterly/yearly); fixed usage_type → category mapping (VM/Storage/Licenze Windows/Altro), 9999 excluded |
| PBX totals aggregation | Sum users/SE, return with rows |
| Period filter | Accept null as "no date filter" |
| Exclusions | codice_aggancio_gest NOT IN (385,485); KlajdiandCo WHERE filter |

### Frontend responsibilities

| Responsibility | Details |
|---------------|---------|
| Master-Detail Drawer | Slide-over panel for Ordini pages (480px / 600px with tabs) |
| Visual row grouping | First-row emphasis + tree-line connector for order tables |
| Charge pie + KPI | Build category composition from `charges-by-category`; localized labels |
| Auto-refresh | Fatture (customer/period change), Timoo (tenant change) |
| Period/aggregation controls | IaaS: period + granularity selectors; drill-down zoom with breadcrumb |
| Currency/date formatting | EUR 2 decimals, DD-MM-YYYY |
| Table features | Search, sort, pagination, column visibility toggle, CSV export |

---

## API Contract Summary

### Mistra endpoints (loader schema)

| Endpoint | Method | Original query |
|----------|--------|---------------|
| `.../customers/with-invoices` | GET | `get_clienti_con_fatture` |
| `.../customers/with-orders` | GET | `get_aziende_con_ordini` / `GET_aziendeConOrdini` |
| `.../customers/with-access-lines` | GET | `get_clients_accessi` |
| `.../order-statuses` | GET | `get_stati_ordine` / `GET_StatiOrdine` |
| `.../orders/summary` | GET | `get_ordini_ricorrenti` |
| `.../orders/detail` | GET | `GET_ordini_Ric_Spot` |
| `.../invoices` | GET | `get_fatture` |
| `.../connection-types` | GET | `get_tipo_conn` |
| `.../access-lines` | GET | `get_accessi_cliente` |

### Grappa endpoints

| Endpoint | Method | Original query |
|----------|--------|---------------|
| `.../iaas/accounts` | GET | `get_cdl_accounts` |
| `.../iaas/charges` | GET | `get_charges` (aggregated series: domain, from, to, group) |
| `.../iaas/charges-by-category` | GET | `get_charges_by_category` (domain, from, to) |
| `.../iaas/windows-licenses` | GET | `get_licenses_by_day` |

### Anisetta endpoints

| Endpoint | Method | Original query |
|----------|--------|---------------|
| `.../timoo/tenants` | GET | `getAnisettaTenants` |
| `.../timoo/pbx-stats` | GET | `getPbxByTenandId` + aggregation |

**Total: 15 GET endpoints, 0 write endpoints.**

---

## Constraints and Non-Functional Requirements

### Security
- All database access through backend Go API (no direct SQL from frontend)
- All queries parameterized (fixes Appsmith SQL injection risks)
- User identity from Keycloak JWT
- Keycloak role: `app_panoramica_access`

### Coexistence
- Both Appsmith and new app access same databases during transition
- Deploy all pages together as complete app
- Exclusion codes (385, 485) hardcoded to match Appsmith behavior

### Performance
- Customer/status lists loaded on page mount
- Dependent data loaded on user selection (no wasted queries)
- Orders summary query has extended timeout (20s) — heavy query
- IaaS accounts: auto-select first row on load for immediate data display

---

## Changes from Appsmith

| # | Change | Reason |
|---|--------|--------|
| 1 | "Tutti i clienti" (-1) removed from Ordini pages | Customer always required |
| 2 | "Righe espanse" checkbox dropped | Non-functional Appsmith workaround |
| 3 | `Hello {{appsmith.user.name}}` greeting dropped | Not needed |
| 4 | Vestigial `get_ordini_ricorrenti` on Ordini R&S page dropped | Unused (only `GET_ordini_Ric_Spot` active) |
| 5 | Accessi icon button → labeled "Cerca" | Standardization |
| 6 | Timoo: button → auto-load on tenant selection | UX improvement |
| 7 | Charge breakdown: flat SQL columns → typed array response | Cleaner API for chart consumption |
| 8 | Period "all" (2000 months) → null (no date filter) | Clean API |
| 9 | `KlajdiandCo` exclusion added to DB query | Was only in TIMOO REST API URL (dropped) |
| 10 | TIMOO REST API entirely dropped | Too slow, replaced by Anisetta DB queries |
| 11 | Direct DB access from browser → Go backend API | Security |
| 12 | SQL injection (unprepared statements) → parameterized queries | Security |
| 13 | Ordini pages: flat table → Master-Detail Drawer pattern | UX redesign |

---

## Acceptance Notes

### What the audit proved directly
- 7 entities across 3 databases, entirely read-only
- 14 SQL queries mapping to 16 backend API endpoints
- 1 critical business rule (`stato_riga` 8-way CASE)
- 4 SQL injection risks (unprepared statements with concatenation)
- Significant query duplication across pages (customer lists, order statuses)
- TIMOO REST API was abandoned in Appsmith itself (commented-out code, replaced by DB)

### What the expert confirmed
- Dashboard: WIP, deferred to `docs/TODO.md`
- Two Ordini pages kept separate (different scope: summary vs detail)
- Customer always required on Ordini (no "all clients")
- Navigation groups: Ordini, Fatture, Servizi
- Line status list hardcoded as-is
- Data sources: loader copies for Accessi, Grappa direct for IaaS
- Exclusion codes as-is per app, not centralized
- KlajdiandCo is a test tenant, exclude from DB query
- PBX accounting: latest snapshot only, no historical need
- Timoo tenants independent from customers
- Usage type labels: as-is English technical names
- Charge breakdown: typed array response
- Timoo: auto-load on tenant selection
- Accessi: standardize button to labeled "Cerca"
- Tables: CSV export enabled
- Invoice grouping: backend computes (as-is)
- cloudstack_domain UUID: hidden (as-is)

### What still needs validation
- Nothing — all questions resolved
