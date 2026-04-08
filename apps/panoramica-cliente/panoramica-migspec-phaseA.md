# Phase A: Entity-Operation Model — Panoramica Cliente

## Scope

**Pages in scope:** 7 (Dashboard excluded — WIP, deferred to `docs/TODO.md`)

| # | Page | Status |
|---|------|--------|
| 1 | Dashboard | **EXCLUDED** — WIP, not migrated now |
| 2 | Ordini ricorrenti | Migrate as-is (separate page) |
| 3 | Ordini Ricorrenti e Spot | Migrate as-is (separate page) |
| 4 | Fatture | Migrate |
| 5 | Accessi | Migrate |
| 6 | IaaS Pay Per Use | Migrate |
| 7 | Timoo tenants | Migrate (DB path only, no TIMOO REST API) |
| 8 | Licenze Windows su Cloudstack | Migrate as-is (separate page — riepilogo complessivo, non per cliente) |

**Decisions already made:**
- TIMOO REST API: **excluded** — too slow, replaced by Anisetta DB queries. Not documented further.
- Navigation: `TabNavGroup` variant as per `listini-e-sconti` SPEC. Groups: **Ordini**, **Fatture**, **Servizi** (Accessi + IaaS PPU + Timoo + Licenze Windows).
- Ordini pages: **kept separate** (no merge). Both use **Master-Detail Drawer** pattern (flat table + slide-over panel).
- Ordini: customer selection is **always required** on both pages (no "Tutti i clienti" option — removed).
- Licenze Windows: **kept separate** from IaaS PPU — different scope (complessivo vs per-account).
- Keycloak role: `app_panoramica_access`.
- "Righe espanse" checkbox: **dropped** (was Appsmith workaround).
- `KlajdiandCo` tenant: **excluded** via `WHERE name != 'KlajdiandCo'` in tenant query.

---

## Extracted Entities

### Entity 1: Customer (cross-database, read-only)

**Purpose:** Central lookup entity. Every page has a customer selector. Never written by this app.

**Source tables:**
| System | Table | Key column | Used by |
|--------|-------|------------|---------|
| Mistra PG | `loader.erp_clienti_con_fatture` | `numero_azienda` (= ERP ID) | Fatture (customer dropdown) |
| Mistra PG | `loader.erp_anagrafiche_clienti` | `numero_azienda` | Ordini ricorrenti, Ordini R&S (customer dropdown, dismissed filter) |
| Mistra PG | `loader.v_ordini_ricorrenti` | `numero_azienda` | Ordini ricorrenti, Ordini R&S (join for customer+order existence) |
| Mistra PG | `loader.grappa_cli_fatturazione` | `id` (Grappa ID), `codice_aggancio_gest` (ERP ID) | Accessi (client selector + join) |
| Grappa MySQL | `cli_fatturazione` | `id` (Grappa ID), `codice_aggancio_gest` (ERP ID) | IaaS PPU (account join) |

**Identity mapping:** Same as documented in `docs/IMPLEMENTATION-KNOWLEDGE.md`:
- Alyante ERP ID = Mistra `customers.customer.id` = `loader.erp_*.numero_azienda`
- Grappa `cli_fatturazione.codice_aggancio_gest` = ERP ID
- Grappa `cli_fatturazione.id` = internal Grappa ID (different!)

**Operations and original queries:**

#### `listCustomersWithInvoices` — used by: Fatture

```sql
-- Original Appsmith query: get_clienti_con_fatture
-- Datasource: mistra (postgres)
-- executeOnLoad: true
select * from loader.erp_clienti_con_fatture
order by ragione_sociale
```

#### `listCustomersWithOrders` (variant A) — used by: Ordini ricorrenti

```sql
-- Original Appsmith query: get_aziende_con_ordini
-- Datasource: mistra (postgres)
-- executeOnLoad: true
select distinct odv.numero_azienda, odv.ragione_sociale
from loader.v_ordini_ricorrenti  as odv
JOIN loader.erp_anagrafiche_clienti AS cli ON cli.numero_azienda =odv.numero_azienda AND (cli.data_dismissione >= NOW() OR cli.data_dismissione='0001-01-01 00:00:00' OR cli.data_dismissione IS NULL)
UNION all
SELECT -1, 'TUTTI I CLIENTI'
order by ragione_sociale
```

#### `listCustomersWithOrders` (variant B) — used by: Ordini Ricorrenti e Spot

```sql
-- Original Appsmith query: GET_aziendeConOrdini
-- Datasource: mistra (postgres)
-- executeOnLoad: true
-- NOTE: subtle difference from variant A — no "OR cli.data_dismissione IS NULL" check
select distinct odv.numero_azienda, odv.ragione_sociale
from loader.v_ordini_ricorrenti  as odv
JOIN loader.erp_anagrafiche_clienti AS cli ON cli.numero_azienda =odv.numero_azienda AND (cli.data_dismissione >= NOW() OR cli.data_dismissione='0001-01-01 00:00:00')
UNION all
SELECT -1, 'TUTTI I CLIENTI'
order by ragione_sociale;
```

#### `listCustomersWithAccessLines` — used by: Accessi

```sql
-- Original Appsmith query: get_clients_accessi
-- Datasource: mistra (postgres)
-- executeOnLoad: true
select distinct cf.id, cf.intestazione
from loader.grappa_foglio_linee fl join loader.grappa_cli_fatturazione cf on fl.id_anagrafica = cf.id
where cf.codice_aggancio_gest is not null and cf.stato = 'attivo'
order by cf.intestazione
;
```

**Fields (union across variants):**

| Field | Type | Source | Notes |
|-------|------|--------|-------|
| numero_azienda / id | int | Various | Customer ID (ERP or Grappa depending on context) |
| ragione_sociale / intestazione | string | Various | Display name |
| data_dismissione | timestamp | `erp_anagrafiche_clienti` | Dismissed date; sentinel `0001-01-01` or NULL = active |
| stato | string | `grappa_cli_fatturazione` | Filter: 'attivo' |
| codice_aggancio_gest | int | `grappa_cli_fatturazione` | ERP ID bridge |

**Business rules:**
- Dismissed customer exclusion (variant A): `data_dismissione >= NOW() OR data_dismissione = '0001-01-01' OR IS NULL`
- Dismissed customer exclusion (variant B): `data_dismissione >= NOW() OR data_dismissione = '0001-01-01'` (no IS NULL)
- "TUTTI I CLIENTI" synthetic option (value -1) for both Ordini pages
- Accessi client list uses Grappa internal IDs, not ERP IDs

**Resolved:** The two Ordini pages have slightly different dismissal filters — each is migrated as-is.

---

### Entity 2: Invoice / Credit Note (read-only)

**Purpose:** Browse invoice and credit note line items for a customer within a time period.

**Source table:** `loader.v_erp_fatture_nc` (Mistra PG view)

**Operations and original queries:**

#### `listInvoiceLines` — used by: Fatture

```sql
-- Original Appsmith query: get_fatture
-- Datasource: mistra (postgres)
-- executeOnLoad: false (triggered by customer/period change)
-- preparedStatement: false (SQL injection risk — will be parameterized in backend)
select  CASE WHEN rn = 1 THEN doc || ' ' || num_documento || CHR(13) || CHR(10) ||to_char( data_documento, '(YYYY-MM-DD)')  ELSE NULL END AS documento,descrizione_riga,
        qta, prezzo_unitario, prezzo_totale_netto, codice_articolo,
        data_documento, num_documento, id_cliente, progressivo_riga, serialnumber, riferimento_ordine_cliente, condizione_pagamento, scadenza, desc_conto_ricavo, gruppo, sottogruppo, rn
from loader.v_erp_fatture_nc
WHERE id_cliente = {{s_f_clienti.selectedOptionValue||-1}} and ( data_documento >= current_date - interval '{{cs_periodo.value}} months')
order by anno_documento desc , mese_documento desc, tipo_documento, num_documento, rn
```

**Fields:**

| Field | Type | Visible | Notes |
|-------|------|---------|-------|
| documento | string (computed) | Yes | `doc + num_documento + data_documento` on first row only (rn=1) |
| descrizione_riga | string | Yes | Line description |
| qta | number | Yes | Quantity |
| prezzo_unitario | decimal (EUR) | Yes | Unit price |
| prezzo_totale_netto | decimal (EUR) | Yes | Net line total |
| codice_articolo | string | Yes | Product code |
| serialnumber | string | Yes | Serial number |
| riferimento_ordine_cliente | string | Yes | Customer order reference |
| condizione_pagamento | string | Yes | Payment terms |
| scadenza | date | Yes | Due date (DD-MM-YYYY) |
| desc_conto_ricavo | string | Yes | Revenue account |
| gruppo | string | Yes | Product group |
| sottogruppo | string | Yes | Product subgroup |
| data_documento | date | Hidden | Document date |
| num_documento | int | Hidden | Document number |
| id_cliente | int | Hidden | Customer ID |
| progressivo_riga | int | Hidden | Line sequence |
| rn | int | Hidden | Row number within document (for grouping) |
| segno | int | Not selected | +1 (invoice) / -1 (credit note) — used in aggregations |

**Business rules:**
- `segno` column determines invoice (+1) vs credit note (-1) — used for sum calculations
- Visual grouping: document header shown only on `rn = 1`
- Period filter: `data_documento >= current_date - interval N months`; "all" = 2000 months
- Sort: `anno_documento DESC, mese_documento DESC, tipo_documento, num_documento, rn`

**Questions for expert:**
2. The "all" option currently uses 2000 months as a hack. Should the backend treat a special value (e.g., 0 or null) as "no date filter"?

---

### Entity 3: Order — Summary view (read-only)

**Purpose:** Browse recurring orders with customer/status filters and order history chain. Summary-level view with ~24 columns.

**Source tables:**
- `loader.v_ordini_sintesi` (Mistra PG view)
- `loader.get_reverse_order_history_path()` (PG function)

**Operations and original queries:**

#### `listOrderStatuses` — used by: Ordini ricorrenti

```sql
-- Original Appsmith query: get_stati_ordine
-- Datasource: mistra (postgres)
-- executeOnLoad: true
select distinct stato_ordine
from loader.v_ordini_ricorrenti
order by stato_ordine
```

#### `listOrdersSummary` — used by: Ordini ricorrenti

```sql
-- Original Appsmith query: get_ordini_ricorrenti
-- Datasource: mistra (postgres)
-- executeOnLoad: false (triggered by "Cerca" button)
-- preparedStatement: false
-- timeout: 20000ms
SELECT stato,
       numero_ordine,
       descrizione_long,
       quantita,
       nrc,
       mrc,
       totale_mrc,
       stato_ordine,
       nome_testata_ordine,
       rn,
       numero_azienda,
       data_documento,
       stato_riga,
       data_ultima_fatt,
       serialnumber,
       metodo_pagamento,durata_servizio, durata_rinnovo, data_cessazione, data_attivazione, note_legali, sost_ord, sostituito_da, loader.get_reverse_order_history_path( nome_testata_ordine) as storico
from loader.v_ordini_sintesi
where {{s_o_cliente.selectedOptionValue === '' || s_o_cliente.selectedOptionValue == -1 ? 'true' : " numero_azienda = '"+s_o_cliente.selectedOptionValue+"'"  }}
and stato_ordine in ({{ms_o_stati.selectedOptionValues.map(i => "'" + i + "'").join()}})
order by data_documento, nome_testata_ordine, rn
```

**Fields:**

| Field | Type | Notes |
|-------|------|-------|
| stato | string | Row status |
| numero_ordine | string | Order number |
| descrizione_long | string | Computed: product + extended description |
| quantita | int | Quantity |
| nrc | decimal | Non-recurring charge |
| mrc | decimal | Monthly recurring charge |
| totale_mrc | decimal | Total MRC (qty * mrc) |
| stato_ordine | string | Order-level status |
| nome_testata_ordine | string | Order header name |
| rn | int | Row number within order |
| numero_azienda | int | Customer ERP ID |
| data_documento | date | Order date |
| stato_riga | string | Computed row status |
| data_ultima_fatt | date | Last invoice date |
| serialnumber | string | Serial number |
| metodo_pagamento | string | Payment method |
| durata_servizio | string | Service duration |
| durata_rinnovo | string | Renewal duration |
| data_cessazione | timestamp | Cessation date |
| data_attivazione | timestamp | Activation date |
| note_legali | text | Legal notes |
| sost_ord | string | Substitute order ref |
| sostituito_da | string | Substituted by |
| storico | string | Computed: order history chain path |

**Business rules:**
- "All clients" pattern: `selectedOptionValue == -1` → no client filter (`WHERE true`)
- Dynamic SQL WHERE (SQL injection risk — parameterize in backend)
- Order history chain: `loader.get_reverse_order_history_path(nome_testata_ordine)`
- `stato_riga` and `descrizione_long` are computed by the `v_ordini_sintesi` view

**Resolved:**
- `get_reverse_order_history_path()` stays as-is in the query.
- "Righe espanse" checkbox: **dropped** (Appsmith workaround, non serve).

---

### Entity 4: Order — Detail view (read-only)

**Purpose:** Full-detail order viewer combining recurring and spot orders, with order lifecycle, referents, product families, and computed `stato_riga`. This is a separate page from Entity 3.

**Source tables:**
- `loader.erp_ordini` + `loader.erp_righe_ordini` + `loader.erp_anagrafiche_clienti` + `loader.erp_anagrafica_articoli_vendita`

**Operations and original queries:**

#### `listOrderStatuses` — used by: Ordini Ricorrenti e Spot

```sql
-- Original Appsmith query: GET_StatiOrdine
-- Datasource: mistra (postgres)
-- executeOnLoad: true
-- NOTE: identical to get_stati_ordine on the other Ordini page
select distinct stato_ordine
from loader.v_ordini_ricorrenti
order by stato_ordine
```

#### `listOrdersDetail` — used by: Ordini Ricorrenti e Spot

```sql
-- Original Appsmith query: GET_ordini_Ric_Spot
-- Datasource: mistra (postgres)
-- executeOnLoad: false (triggered by "GO" button)
-- preparedStatement: false
SELECT c.ragione_sociale,
        CASE
            WHEN o.data_conferma > o.data_documento THEN o.data_conferma
            ELSE o.data_documento
        END AS data_ordine,
    o.nome_testata_ordine,
    o.cliente,
    o.numero_azienda,
    o.id_gamma,
    o.commerciale,
    o.data_documento,
    o.data_conferma,
    o.stato_ordine,
    o.tipo_ordine,
    o.tipo_documento,
    o.sost_ord,
    o.riferimento_odv_cliente,
    o.durata_servizio,
    o.tacito_rinnovo,
    o.durata_rinnovo,
    o.tempi_rilascio,
    o.metodo_pagamento,
    o.note_legali,
    o.referente_amm_nome,
    o.referente_amm_mail,
    o.referente_amm_tel,
    o.referente_tech_nome,
    o.referente_tech_mail,
    o.referente_tech_tel,
    o.referente_altro_nome,
    o.referente_altro_mail,
    o.referente_altro_tel,
    o.data_creazione,
    o.data_variazione,
    o.sostituito_da,
    r.quantita,
    r.codice_kit,
    r.codice_prodotto,
    r.descrizione_prodotto,
    r.descrizione_estesa,
    r.serialnumber,
    r.setup,
    r.canone,
    r.valuta,
    r.costo_cessazione,
    NULLIF(r.data_attivazione, '0001-01-01 00:00:00'::timestamp without time zone) AS data_attivazione,
    NULLIF(r.data_disdetta, '0001-01-01 00:00:00'::timestamp without time zone) AS data_disdetta,
    NULLIF(r.data_cessazione, '0001-01-01 00:00:00'::timestamp without time zone) AS data_cessazione,
    r.raggruppamento_fatturazione,
    r.intervallo_fatt_attivazione,
    r.intervallo_fatt_canone,
    NULLIF(r.data_ultima_fatt, '0001-01-01 00:00:00'::timestamp without time zone) AS data_ultima_fatt,
    NULLIF(r.data_fine_fatt, '0001-01-01 00:00:00'::timestamp without time zone) AS data_fine_fatt,
    r.system_odv_row,
    r.id_gamma_testata,
    r.progressivo_riga,
    CASE
       WHEN r.progressivo_riga = 1 THEN o.nome_testata_ordine
       ELSE NULL::character varying
       END AS ORDINE,
    r.annullato,
    NULLIF(r.data_scadenza_ordine, '0001-01-01 00:00:00'::timestamp without time zone) AS data_scadenza_ordine,
    r.quantita * r.canone AS mrc,
    p.famiglia,
    p.sotto_famiglia,
    p.desc_conto_ricavo AS conto_ricavo,
        CASE
            WHEN o.stato_ordine::text = 'Cessato'::text THEN 'Cessata'::text
            WHEN o.stato_ordine::text = 'Bloccato'::text THEN 'Bloccata'::text
            WHEN o.stato_ordine::text = 'Confermato'::text AND date_part('year'::text, r.data_attivazione) = 1::double precision THEN 'Da attivare'::text
            WHEN o.stato_ordine::text = 'Confermato'::text AND date_part('year'::text, r.data_attivazione) > 1::double precision THEN 'Attiva'::text
            WHEN r.annullato = 1 THEN 'Annullata'::text
            WHEN date_part('year'::text, r.data_cessazione) = 1::double precision THEN 'Attiva'::text
            WHEN r.data_cessazione >= '0001-01-01 00:00:00'::timestamp without time zone AND r.data_cessazione <= now() THEN 'Cessata'::text
            WHEN r.data_cessazione > now() THEN 'Cessazione richiesta'::text
            ELSE 'Unknown'::text
        END AS stato_riga,
    ((((o.nome_testata_ordine::text || ' del '::text) || to_char(o.data_documento::timestamp with time zone, 'YYYY-MM-DD'::text)) || ' ('::text) || o.stato_ordine::text) || ')'::text AS intestazione_ordine,
        CASE
            WHEN r.descrizione_prodotto::text = r.descrizione_estesa::text OR r.descrizione_estesa IS NULL OR r.descrizione_estesa::text = ''::text THEN r.descrizione_prodotto::text
            ELSE ((r.descrizione_prodotto::text || chr(13)) || chr(10)) || r.descrizione_estesa::text
        END AS descrizione_long
   FROM loader.erp_ordini o
     JOIN loader.erp_righe_ordini r ON o.id_gamma::text = r.id_gamma_testata::text
     JOIN loader.erp_anagrafiche_clienti c ON o.numero_azienda = c.numero_azienda
     LEFT JOIN loader.erp_anagrafica_articoli_vendita p ON r.codice_prodotto::text = btrim(p.cod_articolo::text)
  WHERE c.numero_azienda = {{s_clienti.selectedOptionValue}} AND o.stato_ordine::text in ({{ms_stati.selectedOptionValues.map(i => "'" + i + "'").join()}}) AND  r.codice_prodotto::text <> 'CDL-AUTO'::text
ORDER BY o.nome_testata_ordine, o.data_documento DESC;
```

#### Vestigial query (unused on this page)

```sql
-- Original Appsmith query: get_ordini_ricorrenti (on "Ordini Ricorrenti e Spot" page)
-- Datasource: mistra (postgres)
-- executeOnLoad: false
-- NOTE: appears unused on this page — GET_ordini_Ric_Spot is the active query.
-- Kept here for reference; should NOT be implemented for this page.
SELECT stato, numero_ordine, descrizione_long, quantita, nrc, mrc, totale_mrc,
       stato_ordine, nome_testata_ordine, rn, numero_azienda, data_documento,
       stato_riga, data_ultima_fatt, serialnumber,
       metodo_pagamento, durata_servizio, durata_rinnovo, data_cessazione,
       data_attivazione, note_legali, sost_ord, sostituito_da,
       loader.get_reverse_order_history_path(nome_testata_ordine) as storico
from loader.v_ordini_sintesi
where numero_azienda = {{s_clienti.selectedOptionValue}}
and stato_ordine in ({{ms_stati.selectedOptionValues.map(i => "'" + i + "'").join()}})
order by data_documento, nome_testata_ordine, rn
```

**Critical business rule — `stato_riga` computation (embedded in SQL above):**
```
Cessato order       → 'Cessata'
Bloccato order      → 'Bloccata'
Confermato + year(data_attivazione)=1   → 'Da attivare'
Confermato + year(data_attivazione)>1   → 'Attiva'
annullato=1         → 'Annullata'
year(data_cessazione)=1                 → 'Attiva'
data_cessazione <= now()                → 'Cessata'
data_cessazione > now()                 → 'Cessazione richiesta'
else                → 'Unknown'
```
(Year=1 is sentinel for `0001-01-01` = "not set")

**Other business rules:**
- CDL-AUTO exclusion: `codice_prodotto <> 'CDL-AUTO'`
- Sentinel dates: all `0001-01-01` → NULL via NULLIF
- `data_ordine`: MAX(data_conferma, data_documento)
- `intestazione_ordine`: `"ORD-NAME del YYYY-MM-DD (STATUS)"`
- `descrizione_long`: concat product + extended description with CR/LF only when they differ
- `ORDINE` column: order name on `progressivo_riga = 1`, NULL otherwise (visual grouping)
- `mrc` computed: `quantita * canone`

---

### Entity 5: AccessLine (read-only)

**Purpose:** Browse connectivity access lines (fiber, DSL, etc.) with status and type filters.

**Source tables (all `loader.` copies in Mistra PG):**
- `loader.grappa_foglio_linee` (main)
- `loader.grappa_cli_fatturazione` (customer join)
- `loader.grappa_profili` (profile/type info)
- `loader.v_ordini_ricorrenti` (latest order context, LEFT JOIN)

**Operations and original queries:**

#### `listConnectionTypes` — used by: Accessi

```sql
-- Original Appsmith query: get_tipo_conn
-- Datasource: mistra (postgres)
-- executeOnLoad: true
select distinct tipo_conn from loader.grappa_foglio_linee order by tipo_conn;
```

#### `listAccessLines` — used by: Accessi

```sql
-- Original Appsmith query: get_accessi_cliente
-- Datasource: mistra (postgres)
-- executeOnLoad: true
-- preparedStatement: false
SELECT
    tipo_conn, fl.fornitore, provincia, comune, p.tipo, p.profilo_commerciale,
    cogn_rsoc_intest_linea AS intestatario, r.nome_testata_ordine ordine, r.data_ultima_fatt fatturato_fino_al,
    r.stato_riga, r.stato_ordine,
    fl.stato, fl.id, codice_ordine, fl.serialnumber,  cf.codice_aggancio_gest AS id_anagrafica
FROM
    loader.grappa_foglio_linee fl
        JOIN loader.grappa_cli_fatturazione cf ON fl.id_anagrafica = cf.id
        LEFT JOIN loader.grappa_profili p ON fl.id_profilo = p.id
        LEFT JOIN (
        SELECT *,
               ROW_NUMBER() OVER(PARTITION BY serialnumber ORDER BY data_documento DESC , progressivo_riga) AS rn
        FROM loader.v_ordini_ricorrenti
    ) r ON fl.serialnumber = r.serialnumber AND r.numero_azienda = cf.codice_aggancio_gest AND r.rn = 1
WHERE
    fl.id_anagrafica IN  ({{ms_clienti.selectedOptionValues.map(i => "'" + i + "'").join()||-1}})
    and fl.stato in ({{ms_stato.selectedOptionValues.map(i => "'" + i + "'").join()}})
    and fl.tipo_conn in ({{ms_tipo_conn.selectedOptionValues.map(i => "'" + i + "'").join()}})
order by     tipo_conn, fl.fornitore, provincia, comune, p.tipo, p.profilo_commerciale
;
```

**Fields:**

| Field | Type | Notes |
|-------|------|-------|
| tipo_conn | string | Connection type (e.g., FTTH, FTTC, xDSL) |
| fornitore | string | Supplier |
| provincia | string | Province |
| comune | string | Municipality |
| tipo | string | Profile type |
| profilo_commerciale | string | Commercial profile |
| intestatario | string | Line holder (`cogn_rsoc_intest_linea`) |
| ordine | string | Linked order name (from latest `v_ordini_ricorrenti`) |
| fatturato_fino_al | date | Billed until date (from order) |
| stato_riga | string | Order row status |
| stato_ordine | string | Order status |
| stato | string | Line status (Attiva, Cessata, etc.) |
| id | int | Grappa line ID |
| codice_ordine | string | Order code |
| serialnumber | string | Serial number |
| id_anagrafica | int | ERP ID (via `codice_aggancio_gest`) — displayed as "Id Alyante" |

**Business rules:**
- Line status values (hardcoded in Appsmith): Attiva, Cessata, da attivare, in attivazione, KO
- Default filter: stato = "Attiva"
- Connection types: dynamic from DB (all selected by default)
- Latest order: `ROW_NUMBER() OVER(PARTITION BY serialnumber ORDER BY data_documento DESC, progressivo_riga) rn = 1`
- Customer selector uses **Grappa internal IDs** (not ERP IDs) for this page

**Resolved:**
- Line status list: **hardcoded as-is** (Attiva, Cessata, da attivare, in attivazione, KO).
- Data source: **as-is** — query Mistra `loader.grappa_*` copies, not Grappa directly.

---

### Entity 6: IaaSAccount (read-only)

**Purpose:** Cloudstack billing accounts with consumption data.

**Source tables (Grappa MySQL):**
- `cdl_accounts` (accounts)
- `cli_fatturazione` (customer join)
- `cdl_charges` (usage charges)

**Operations and original queries:**

#### `listAccounts` — used by: IaaS PPU

```sql
-- Original Appsmith query: get_cdl_accounts
-- Datasource: grappa (mysql)
-- executeOnLoad: true
select c.intestazione, a.credito, domainuuid as cloudstack_domain, id_cli_fatturazione, abbreviazione, codice_ordine, serialnumber, data_attivazione
from cdl_accounts a
JOIN cli_fatturazione c on a.id_cli_fatturazione = c.id
WHERE id_cli_fatturazione > 0 and attivo = 1 and fatturazione = 1 and c.codice_aggancio_gest not in (385,485)
order by intestazione
```

#### `listDailyCharges` — used by: IaaS PPU (daily tab)

```sql
-- Original Appsmith query: get_daily_charges
-- Datasource: grappa (mysql)
-- executeOnLoad: true
SELECT
    c.charge_day as giorno,
    c.domainid,
    CAST(SUM(CASE WHEN c.usage_type = 9999 THEN c.usage_charge ELSE 0 END) AS DECIMAL(10,2)) AS utCredit,
    CAST(SUM(c.usage_charge) AS DECIMAL(10,2)) AS total_importo
FROM
    cdl_charges c
WHERE
    c.domainid = {{tbl_accounts.selectedRow.cloudstack_domain}} AND charge_day >= date_sub(now(), interval 120 day)
GROUP BY
    c.charge_day,
    c.domainid
ORDER BY
    c.charge_day DESC;
```

#### `listMonthlyCharges` — used by: IaaS PPU (monthly tab)

```sql
-- Original Appsmith query: get_monthly_charges
-- Datasource: grappa (mysql)
-- executeOnLoad: true
select date_format(charge_day,'%Y-%m') as mese, cast( sum(usage_charge) as decimal(7,2)) importo
from cdl_charges
where domainid = {{tbl_accounts.selectedRow.cloudstack_domain}}
and charge_day >= date_sub(now(), interval 365 day)
group by 1
order by 1 DESC
limit 12
```

#### `getChargeBreakdownByDay` — used by: IaaS PPU (pie chart)

```sql
-- Original Appsmith query: get_charges_by_type
-- Datasource: grappa (mysql)
-- executeOnLoad: true
SELECT
    c.charge_day,
    c.domainid,
    CAST(SUM(CASE WHEN c.usage_type = 1 THEN c.usage_charge ELSE 0 END) AS DECIMAL(10,2)) AS utRunningVM,
    CAST(SUM(CASE WHEN c.usage_type = 2 THEN c.usage_charge ELSE 0 END) AS DECIMAL(10,2)) AS utAllocatedVM,
    CAST(SUM(CASE WHEN c.usage_type = 3 THEN c.usage_charge ELSE 0 END) AS DECIMAL(10,2)) AS utIpCharge,
    CAST(SUM(CASE WHEN c.usage_type = 6 THEN c.usage_charge ELSE 0 END) AS DECIMAL(10,2)) AS utVolume,
    CAST(SUM(CASE WHEN c.usage_type = 7 THEN c.usage_charge ELSE 0 END) AS DECIMAL(10,2)) AS utTemplate,
    CAST(SUM(CASE WHEN c.usage_type = 8 THEN c.usage_charge ELSE 0 END) AS DECIMAL(10,2)) AS utISO,
    CAST(SUM(CASE WHEN c.usage_type = 9 THEN c.usage_charge ELSE 0 END) AS DECIMAL(10,2)) AS utSnapshot,
    CAST(SUM(CASE WHEN c.usage_type = 26 THEN c.usage_charge ELSE 0 END) AS DECIMAL(10,2)) AS utVolumeSecondary,
    CAST(SUM(CASE WHEN c.usage_type = 27 THEN c.usage_charge ELSE 0 END) AS DECIMAL(10,2)) AS utVmSnapshotOnPrimary,
    CAST(SUM(CASE WHEN c.usage_type = 9999 THEN c.usage_charge ELSE 0 END) AS DECIMAL(10,2)) AS utCredit,
    CAST(SUM(c.usage_charge) AS DECIMAL(10,2)) AS total_importo
FROM
    cdl_charges c
WHERE
    c.domainid = {{tbl_accounts.selectedRow.cloudstack_domain}} AND charge_day = {{tbl_giornalieri.selectedRow.giorno||''}}
GROUP BY
    c.charge_day,
    c.domainid
ORDER BY
    c.charge_day DESC;
```

#### `listWindowsLicensesByDay` — used by: Licenze Windows

```sql
-- Original Appsmith query: get_licenses_by_day
-- Datasource: grappa (mysql)
-- executeOnLoad: true
select charge_day as x, count(0) as y
from cdl_charges where charge_day >= CURDATE() - INTERVAL 14 DAY and usage_type = 9998
group by charge_day
order by charge_day desc;
```

**Business rules:**
- Account filter: `attivo = 1 AND fatturazione = 1 AND codice_aggancio_gest NOT IN (385, 485)`
- Daily charges: last 120 days
- Monthly charges: last 12 months (LIMIT 12)
- Usage type codes: 1=RunningVM, 2=AllocatedVM, 3=IP, 6=Volume, 7=Template, 8=ISO, 9=Snapshot, 26=VolumeSecondary, 27=VmSnapshotOnPrimary, 9998=WindowsLicense, 9999=Credit
- Windows license chart: last 14 days, `usage_type = 9998`
- Charge breakdown: only non-zero types shown in pie chart (frontend logic)

**Resolved:**
- Licenze Windows: **separate page** — riepilogo complessivo, scopo diverso da IaaS PPU per-account.
- Exclusion codes (385, 485): **as-is** in this app, no centralization.

---

### Entity 7: TimooTenant (read-only)

**Purpose:** View Timoo PBX tenants and their PBX instance statistics.

**Source tables (Anisetta PostgreSQL):**
- `public.as7_tenants` (tenant list)
- `public.as7_pbx_accounting` (PBX statistics)

**Schema reference:** Both tables documented in `docs/anisetta_schema.json` (public schema).

**Operations and original queries:**

#### `listTenants` — used by: Timoo

```sql
-- Original Appsmith query: getAnisettaTenants
-- Datasource: anisetta (postgres)
-- executeOnLoad: true
SELECT *
FROM public."as7_tenants" ;
```

#### `getPbxStatsByTenant` — used by: Timoo

```sql
-- Original Appsmith query: getPbxByTenandId
-- Datasource: anisetta (postgres)
-- executeOnLoad: false (triggered by button)
select as7_tenant_id,pbx_id,pbx_name,MAX(users) as users,MAX(service_extensions) as service_extensions
from public.as7_pbx_accounting apb
where as7_tenant_id = {{sl_tenant.selectedOptionValue}} and to_char(data, 'YYYY-MM-DD') = (select  to_char(data, 'YYYY-MM-DD') as data from public.as7_pbx_accounting order by id desc limit 1)
GROUP BY as7_tenant_id,pbx_id,pbx_name
order by pbx_name
;
```

**Original JSObject logic (now moves to backend):**

```javascript
// Original Appsmith JSObject: utils.pbxStats()
// Classification: data aggregation → backend
// Computes per-PBX totals and grand totals
async pbxStats () {
    await getPbxByTenandId.run();
    const pbxSummaries = [];
    let totalServiceExtensions = 0;
    let totalUsers = 0;
    for (const pbx of getPbxByTenandId.data) {
        totalUsers += pbx.users;
        totalServiceExtensions += pbx.service_extensions;
        pbxSummaries.push({
            pbxName: pbx.pbx_name,
            pbxId: pbx.pbx_id,
            users: pbx.users,
            serviceExtensions: pbx.service_extensions,
            ivr: 0,
            totale: pbx.users+pbx.service_extensions
        });
    };
    this.statistiche = pbxSummaries;
    this.totalUsers = totalUsers;
    this.totalSE = totalServiceExtensions;
}
```

**Aggregated display fields (backend response):**

| Field | Type | Notes |
|-------|------|-------|
| rows[].pbx_name | string | PBX name |
| rows[].pbx_id | int | PBX instance ID |
| rows[].users | int | User count |
| rows[].service_extensions | int | Service extension count |
| rows[].totale | int | users + service_extensions |
| totalUsers | int | Sum across all PBX instances |
| totalSE | int | Sum of service_extensions |

**Business rules:**
- PBX data filtered to latest date: subquery `SELECT to_char(data, 'YYYY-MM-DD') FROM as7_pbx_accounting ORDER BY id DESC LIMIT 1`
- Grouped by `as7_tenant_id, pbx_id, pbx_name` with `MAX(users)`, `MAX(service_extensions)`
- Excluded tenant: `KlajdiandCo` — tenant di test, **va escluso** con `WHERE name != 'KlajdiandCo'` nella query tenant list
- **TIMOO REST API not used** — replaced by DB queries (per user direction)

**Resolved:**
- `KlajdiandCo`: **escludere** — aggiungere filtro WHERE alla query `getAnisettaTenants`.
- PBX accounting: **as-is** — solo ultimo snapshot.
- Tenant-Customer relationship: **nessuna** — entità indipendenti.

---

## Entity Relationships

```
Customer (ERP ID)
    ├── Invoice/CreditNote (id_cliente = ERP ID)           [Mistra loader]
    ├── Order Summary (numero_azienda = ERP ID)             [Mistra loader]
    ├── Order Detail (numero_azienda = ERP ID)              [Mistra loader]
    ├── AccessLine (via grappa_cli_fatturazione.id)          [Mistra loader / Grappa]
    └── IaaSAccount (via cli_fatturazione.id → Grappa ID)   [Grappa]

TimooTenant (standalone — no direct customer link in this app)
    └── PBXInstance (as7_tenant_id)                         [Anisetta]
```

---

## Summary of Phase A Decisions

All Phase A questions resolved:

| # | Decision |
|---|----------|
| 1 | **As-is.** Each Ordini page keeps its own dismissal filter. |
| 2 | Open (Low) — "All" period handling. |
| 3 | **As-is.** `get_reverse_order_history_path()` stays. |
| 4 | **Drop.** Checkbox was Appsmith workaround. |
| 5 | **Hardcoded as-is.** |
| 6 | **As-is.** Use Mistra `loader.grappa_*` copies. |
| 7 | **Separate pages.** Licenze Windows = riepilogo complessivo. |
| 8 | **As-is.** No centralization. |
| 9 | **Exclude `KlajdiandCo`** via WHERE in tenant query. |
| 10 | **As-is.** Latest snapshot only. |
| 11 | **Independent.** No tenant-customer relationship. |
