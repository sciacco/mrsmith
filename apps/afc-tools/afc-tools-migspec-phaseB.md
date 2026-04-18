# AFC Tools — Migration Spec Phase B: UX Pattern Map

**Scope**: classify each current Appsmith page by interaction pattern, map to the portal mini-app shell, and lock the fact-set needed for Phase C (logic placement). Decisions from Phase A are assumed.

---

## B.1 App shell and navigation

Convention (inferred from `reports`, `energia-dc`, `listini-e-sconti`, `compliance`, `quotes`): React Router + `AppShell` from `@mrsmith/ui`, with `TabNav` (flat) or `TabNavGroup` (grouped).

AFC Tools has 8 pages that split cleanly into three operational domains — the pattern matches `reports` (which uses `TabNavGroup`). Proposed grouping:

- **Billing & fatturazione**
  - Transazioni WHMCS
  - Fatture Prometeus
  - Nuovi articoli da inserire
  - Report DDT per cespiti
- **Ordini & XConnect**
  - Ordini Sales (list)
  - Dettaglio ordini (drill-down, **not a tab** — reached via row click, route `/ordini-sales/:id`)
  - Report XConnect e Remote Hands
- **Energia Colo**
  - Consumi variabili Energia Colo

Route table:

| Path | View | In tab nav |
|---|---|---|
| `/transazioni-whmcs` | Transazioni WHMCS | ✓ |
| `/fatture-prometeus` | Fatture Prometeus | ✓ |
| `/nuovi-articoli` | Nuovi articoli da inserire | ✓ |
| `/report-ddt-cespiti` | Report DDT per cespiti | ✓ |
| `/ordini-sales` | Ordini Sales | ✓ |
| `/ordini-sales/:id` | Dettaglio ordini | — (drill-down) |
| `/report-xconnect-rh` | Report XConnect e Remote Hands | ✓ |
| `/consumi-energia-colo` | Consumi variabili Energia Colo | ✓ |

Appsmith currently drills down to Dettaglio ordini via `?id=`. The portal convention is path-parameter (`:id`); same contract, idiomatic routing.

---

## B.2 Per-view classification

Pattern vocabulary used:
- **R1**: *list-with-date-range* — two date pickers + table + export action.
- **R2**: *read-only dump* — single table bound to an on-load query.
- **R3**: *mixed-tab canvas* — a view with two disparate sub-flows behind tabs.
- **R4**: *year-filtered report* — single input + Cerca + dual table (pivot + detail).
- **R5**: *list → detail* — paginable list with per-row navigation to a detail page.
- **R6**: *read-only detail* — header + rows view keyed by URL id.

---

### B.2.1 Transazioni WHMCS — **R1**
- **User intent**: "voglio vedere le transazioni WHMCS (pagamenti + rimborsi) in un intervallo di date ed esportarle in XLSX".
- **Interaction pattern**: date-range filter + table + export.
- **UI sections**:
  1. Filter bar: `i_dal` (DATE_PICKER, default `moment().subtract(15,'days')`), `i_al` (DATE_PICKER, default today), `Button1` Cerca (primary), `Button2` Esporta (secondary).
  2. Results table `tbl_transactions` — 14 columns.
- **Entry**: tab click → blank state until Cerca is pressed (no auto-load).
- **Exit**: Esporta → opens carbone-rendered XLSX URL in a new tab.
- **Preserved behaviors**: auto-load is **off**; filter defaults are kept; Esporta re-runs the query before exporting.
- **Changes from 1:1**: none.

### B.2.2 Fatture Prometeus — **R2**
- **User intent**: audit delle ultime 2000 righe fattura spedite da WHMCS ad Alyante.
- **Interaction pattern**: single read-only table, on-load query.
- **UI sections**: `tbl_fatture_prometeus` table (31 cols, see Appendix SQL).
- **Entry**: tab click → auto-load.
- **Exit**: none.
- **Preserved behaviors**: `ORDER BY id DESC LIMIT 2000`, no filter.
- **Changes from 1:1**: none.

### B.2.3 Nuovi articoli da inserire — **R2**
- **User intent**: lista degli articoli Mistra con `erp_sync=true` che non esistono ancora in Alyante.
- **UI sections**: `tbl_lista_articoli` table (6 cols: `code`, `categoria`, `descrizione_it`, `descrizione_en`, `nrc`, `mrc`).
- **Entry**: tab click → auto-load.
- **Changes from 1:1**: none.

### B.2.4 Report XConnect e Remote Hands — **R3**
- **User intent**: due flussi distinti accomunati solo dal download di un PDF.
  - **Tab 1**: scarica PDF di un ticket Remote Hands per numero.
  - **Tab 2**: lista ordini XConnect EVASO con download PDF per riga.
- **UI sections**:
  - Tab 1 (Canvas1): `numeroTicket` input (required), `Seleziona_lingua` select (`it` / `en`), `Scarica_PDF_button`, `Text1` istruzioni, `Divider1`.
  - Tab 2 (Canvas2): `Table1` bound to `All_orders_xcon.data`, 4 cols + "Scarica PDF" button column.
- **Entry**: tab click → Tab 2 auto-loads the orders list; Tab 1 is empty until the user types.
- **Exit**: PDF download (browser `download()`).
- **Preserved behaviors**:
  - `ticket_type=RemoteHands` hard-pinned (XConnect tickets not supported here despite the page title).
  - On 404 for order PDF → toast "Il PDF non è ancora pronto.".
  - Tab structure preserved (two tabs in a single view).
- **Changes from 1:1**: the client-side base64-vs-raw-bytes heuristic disappears — in a native `fetch` the body is consumed as a `Blob` directly. This is *implementation-equivalent*, not a user-visible change: the end-user still sees the same downloaded PDF.

### B.2.5 Consumi variabili Energia Colo — **R4**
- **User intent**: dato un anno, mostrare i consumi mensili di colocation (pivot per cliente) e i dettagli per periodo.
- **UI sections**:
  1. Filter bar: `TXT_anno` input (default = anno corrente — deviation 1:1d), `BTN_ricerca` Cerca.
  2. Pivot table `TBL_ConsumiColo` — columns: `customer`, `Gennaio…Dicembre`.
  3. Detail table `TBL_ConsumiColoDetail` — columns: `customer`, `start_period`, `end_period`, `consumo`, `amount`, `pun`, `coefficiente`, `fisso_cu`, `eccedenti`, `importo_eccedenti`, `tipo_variabile` (verbatim from `Q_select_consumi_colo`).
- **Entry**: tab click → both queries auto-run with the current-year default (deviation 1:1d). Pressing Cerca re-runs both in parallel.
- **Preserved behaviors**: consumption metric `IF(ampere>0, ampere, Kw)`; pivot shape (IT month labels); no ORDER BY on detail.
- **Changes from 1:1**: default year populated at page load (decision A.5.1d).

### B.2.6 Ordini Sales — **R5**
- **User intent**: lista ordini Vodka con stato `ATTIVO` o `INVIATO`; icona per drill-down su dettaglio.
- **UI sections**: `Lista_ordini` table (12 cols, see Appendix SQL) + icon-button column → `/ordini-sales/:id`.
- **Entry**: tab click → auto-load.
- **Exit**: row click → Dettaglio ordini.
- **Preserved behaviors**: state filter, SQL-side label mappings (Tipo di ordine / Tipo di servizi / Dal CP?).
- **Dropped**: `Dettaglio_ordine` modal widget (dead UI, decision A.5.2).
- **Changes from 1:1**: none observable to the end user.

### B.2.7 Dettaglio ordini — **R6**
- **User intent**: header + rows di un ordine, letto dal querystring.
- **UI sections**:
  1. Header: 20+ text widgets in a grid — see Appendix "Dettaglio ordini widget bindings" for the verbatim layout. Fields map to `Order.data[0].*`; many use ternaries for code→label translation.
  2. Rows table `TBL_OrderRows` bound to `RigheOrdine.data` (14 cols, see Appendix SQL).
  3. `TornaIndietro` button → back to `/ordini-sales`.
- **Entry**: route `/ordini-sales/:id` → auto-load of both `Order` and `RigheOrdine`.
- **Preserved behaviors**: all ternary mappings verbatim, HTML `<b>…</b>` → bold typography.
- **Changes from 1:1**:
  - Bug fix on `cdlan_cod_termini_pag == 400` (decision A.5.1a).
  - Null-treated-as-empty on `cdlan_note` and `data_decorrenza` (decision A.5.1c).
  - `cdlan_dur_rin==4` vs `cdlan_int_fatturazione==5` divergence preserved (decision A.5.1b, TODO logged).

### B.2.8 Report DDT per cespiti — **R2**
- **User intent**: dump completo della view Alyante `Tsmi_DDT_Verifica_Cespiti`.
- **UI sections**: `TBL_DDTCespiti` table (columns per the view's `SELECT *`; see Appendix).
- **Entry**: tab click → auto-load full table.
- **Preserved behaviors**: `SELECT *`, no filter/limit (decision A.5.1e, TODO logged).

---

## B.3 Pattern-to-component mapping (for Phase C handoff)

| View | Pattern | Table pagination | Filter form | Notes |
|---|---|---|---|---|
| Transazioni WHMCS | R1 | client-side (`@mrsmith/ui` Table) | 2 date pickers | Manual trigger |
| Fatture Prometeus | R2 | client-side | — | 2000-row cap preserved |
| Nuovi articoli | R2 | client-side | — | |
| Report XConnect/RH | R3 | Tab 2 client-side | Tab 1 inputs | Two canvases → two tab panels |
| Consumi Energia Colo | R4 | client-side x2 | 1 year input | Auto-load with current year |
| Ordini Sales | R5 | client-side | — | Row action → route |
| Dettaglio ordini | R6 | client-side | — | Header grid + rows table |
| Report DDT cespiti | R2 | client-side | — | Performance TODO |

"Client-side pagination" means the backend returns the full result (respecting current SQL caps like `LIMIT 2000`) and the UI table paginates in memory — matching current Appsmith behavior. No server-side pagination is introduced.

---

## B.4 Appendix — Verbatim SQL / REST / binding quotations

These resolve Q-A1, Q-A2, Q-A4, Q-A5, Q-A6 from Phase A (exact column lists for downstream implementation).

### B.4.1 `getTransactions` (WHMCS — Transazioni WHMCS)
```sql
select cliente, fattura, invoiceid, userid, payment_method,
       date_format(date, '%Y-%m-%d') as date,
       description, amountin, fees, amountout, rate, transid, refundid, accountsid
from v_transazioni
where ((fattura <> '' and invoiceid > 0) or refundid > 0)
  and date > 20230120
  and date BETWEEN {{i_dal.selectedDate}} and {{i_al.selectedDate}}
order by date desc, fattura asc
```
Note: the bug floor `date > 20230120` and the invoice/refund filter are preserved. `date` is a `YYYYMMDD` integer column.

### B.4.2 `righealiante` (WHMCS — Fatture Prometeus) — **31 columns**
```sql
select raggruppamento, ragionesocialecliente, nomecliente, cognomecliente, partitaiva,
       codicefiscale, codiceiso, flagpersonafisica, indirizzo, numerocivico, cap, comune, provincia,
       nazione, numerodocumento, datadocumento, causale, numerolinea, quantita, descrizioneriga,
       prezzo, datainizioperiodo, datafineperiodo, modalitapagamento, ivariga, bollo,
       codiceclienteerp, tipo, invoiceid, id
from rigaaliante
order by id desc
limit 2000
```
Column count = 30 (audit said 31; the SQL projects 30 distinct names — audit rounding). Lock the field list from this SQL.

### B.4.3 `articoli_non_in_alyante` (Mistra PG — Nuovi articoli)
```sql
select p.code,
       pc.name as categoria,
       p.nrc,
       p.mrc,
       max(case when t.language = 'it' then short end) as descrizione_it,
       max(case when t.language = 'en' then short end) as descrizione_en
from loader.erp_anagrafica_articoli_vendita a
    right join products.product p on trim(a.cod_articolo) = p.code
    join products.product_category pc on pc.id = p.category_id
    left join common.translation t on p.translation_uuid = t.translation_uuid
where a.cod_articolo is null and p.erp_sync = true
group by 1, 2, 3, 4;
```

### B.4.4 `DownloadTicketPDF` (gateway — XConnect/RH Tab 1)
```
GET https://gw-int.cdlan.net/tickets/v1/pdf/{ticketId}
    ?ticket_type=RemoteHands
    &lang={it|en}
```

### B.4.5 `DownloadOrderPDF` (gateway — XConnect/RH Tab 2)
```
GET https://gw-int.cdlan.net/orders/v1/order/pdf/{orderId}
```

### B.4.6 `All_orders_xcon` (Mistra PG — XConnect/RH Tab 2)
```sql
SELECT o.id            AS id_ordine,
       hd.codice       AS codice_ordine,
       c.name          AS cliente,
       o.created_at    AS data_creazione
FROM loader.hubs_deal hd
JOIN loader.cp_ordini cpo    ON hd.id = cpo.hs_deal_id
JOIN orders.order o          ON o.order_number = cpo.order_number
JOIN customers.customer c    ON c.id = o.customer_id
JOIN orders.order_state os   ON os.id = o.state_id
WHERE o.kit_category = 'XCONNECT' AND os.name = 'EVASO'
ORDER BY o.created_at DESC;
```

### B.4.7 `Q_select_consumi_colo_filter` (grappa — Energia Colo pivot)
```sql
SELECT customer,
       sum(January)   as Gennaio,
       sum(February)  as Febbraio,
       sum(March)     as Marzo,
       sum(April)     as Aprile,
       sum(May)       as Maggio,
       sum(June)      as Giugno,
       sum(July)      as Luglio,
       sum(August)    as Agosto,
       sum(September) as Settembre,
       sum(October)   as Ottobre,
       sum(November)  as Novembre,
       sum(December)  as Dicembre
FROM (
  SELECT c.intestazione as customer,
         case when month(i.start_period)=1  then IF(i.ampere>0,i.ampere,i.Kw) else 0 end as "January",
         case when month(i.start_period)=2  then IF(i.ampere>0,i.ampere,i.Kw) else 0 end as "February",
         case when month(i.start_period)=3  then IF(i.ampere>0,i.ampere,i.Kw) else 0 end as "March",
         case when month(i.start_period)=4  then IF(i.ampere>0,i.ampere,i.Kw) else 0 end as "April",
         case when month(i.start_period)=5  then IF(i.ampere>0,i.ampere,i.Kw) else 0 end as "May",
         case when month(i.start_period)=6  then IF(i.ampere>0,i.ampere,i.Kw) else 0 end as "June",
         case when month(i.start_period)=7  then IF(i.ampere>0,i.ampere,i.Kw) else 0 end as "July",
         case when month(i.start_period)=8  then IF(i.ampere>0,i.ampere,i.Kw) else 0 end as "August",
         case when month(i.start_period)=9  then IF(i.ampere>0,i.ampere,i.Kw) else 0 end as "September",
         case when month(i.start_period)=10 then IF(i.ampere>0,i.ampere,i.Kw) else 0 end as "October",
         case when month(i.start_period)=11 then IF(i.ampere>0,i.ampere,i.Kw) else 0 end as "November",
         case when month(i.start_period)=12 then IF(i.ampere>0,i.ampere,i.Kw) else 0 end as "December"
  FROM importi_corrente_colocation as i
  JOIN cli_fatturazione as c ON c.id = i.customer_id
  WHERE year(i.start_period) = '{{TXT_anno.text}}'
  GROUP BY c.intestazione, i.start_period
) AS A2
GROUP BY customer;
```

### B.4.8 `Q_select_consumi_colo` (grappa — Energia Colo detail)
```sql
SELECT c.intestazione as customer,
       i.start_period,
       i.end_period,
       IF(i.ampere>0, i.ampere, i.Kw) as consumo,
       i.amount,
       i.pun,
       i.coefficiente,
       i.fisso_cu,
       i.eccedenti,
       i.importo_eccedenti,
       i.tipo_variabile
FROM importi_corrente_colocation as i
JOIN cli_fatturazione as c ON c.id = i.customer_id
WHERE year(i.start_period) = '{{TXT_anno.text}}';
```

### B.4.9 `Select_Orders_Table` (Vodka — Ordini Sales)
```sql
SELECT id,
       cdlan_tipodoc,
       cdlan_ndoc,
       cdlan_anno,
       concat(cdlan_ndoc, '/', cdlan_anno) as "Codice ordine",
       cdlan_sost_ord,
       cdlan_cliente,
       cdlan_datadoc,
       IF(is_colo != 0, is_colo, service_type) AS "Tipo di servizi",
       CASE cdlan_tipo_ord
           WHEN 'A' THEN 'Sostituzione'
           WHEN 'N' THEN 'Nuovo'
           WHEN 'R' THEN 'Rinnovo'
           ELSE NULL
       END AS "Tipo di ordine",
       cdlan_dataconferma,
       cdlan_stato,
       IF(from_cp != 0, 'Sì', 'No') AS "Dal CP?"
FROM orders
WHERE cdlan_stato IN ('ATTIVO', 'INVIATO')
ORDER BY cdlan_datadoc DESC;
```

### B.4.10 `Order` (Vodka — Dettaglio ordini header)
```sql
SELECT id,
       cdlan_systemodv,
       cdlan_tipodoc,
       cdlan_ndoc,
       cdlan_datadoc,
       cdlan_cliente,
       cdlan_commerciale,
       cdlan_cod_termini_pag,
       cdlan_note,
       cdlan_tipo_ord,
       cdlan_dur_rin,
       cdlan_tacito_rin,
       cdlan_sost_ord,
       cdlan_tempi_ril,
       cdlan_durata_servizio,
       cdlan_dataconferma,
       cdlan_rif_ordcli,
       cdlan_rif_tech_nom,
       cdlan_rif_tech_tel,
       cdlan_rif_tech_email,
       cdlan_rif_altro_tech_nom,
       cdlan_rif_altro_tech_tel,
       cdlan_rif_altro_tech_email,
       cdlan_rif_adm_nom,
       cdlan_rif_adm_tech_tel,
       cdlan_rif_adm_tech_email,
       CASE cdlan_int_fatturazione
           WHEN 1 THEN 'Mensile'
           WHEN 2 THEN 'Bimestrale'
           WHEN 3 THEN 'Trimestrale'
           WHEN 5 THEN 'Quadrimestrale'
           WHEN 6 THEN 'Semestrale'
           ELSE 'Annuale'
       END AS cdlan_int_fatturazione_desc,
       cdlan_int_fatturazione,
       CASE cdlan_int_fatturazione_att
           WHEN 1 THEN 'All''ordine'
           ELSE 'All''attivazione della Soluzione/Consegna'
       END AS cdlan_int_fatturazione_att_desc,
       cdlan_int_fatturazione_att,
       cdlan_stato,
       cdlan_evaso,
       cdlan_chiuso,
       cdlan_anno,
       cdlan_valuta,
       written_by,
       profile_iva,
       profile_cf,
       profile_address,
       profile_city,
       profile_cap,
       profile_pv,
       profile_sdi,
       profile_lang,
       cdlan_cliente_id,
       service_type,
       data_decorrenza,
       cdlan_tacito_rin_in_pdf,
       is_colo,
       origin_cod_termini_pag,
       is_arxivar,
       from_cp,
       arx_doc_number
FROM orders
WHERE id = {{ appsmith.URL.queryParams.id }}
LIMIT 1;
```

### B.4.11 `RigheOrdine` (Vodka — Dettaglio ordini rows)
```sql
SELECT id                                AS 'ID Riga',
       cdlan_systemodv_row               AS 'System ODV Riga',
       IF(cdlan_codice_kit != '',
          CONCAT(cdlan_codice_kit, '-', index_kit),
          '')                            AS 'Codice articolo bundle',
       cdlan_codart                      AS 'Codice articolo',
       cdlan_descart                     AS 'Descrizione articolo',
       cdlan_prezzo                      AS 'Canone',
       cdlan_prezzo_attivazione          AS 'Attivazione',
       cdlan_qta                         AS 'Quantità',
       cdlan_prezzo_cessazione           AS 'Prezzo cessazione',
       cdlan_ragg_fatturazione           AS 'Codice raggruppamento fatturazione',
       cdlan_data_attivazione            AS 'Data attivazione',
       cdlan_serialnumber                AS 'Numero seriale',
       confirm_data_attivazione,
       data_annullamento
FROM orders_rows
WHERE orders_id = {{ appsmith.URL.queryParams.id }};
```

### B.4.12 `ListaDdtVerificaCespiti` (Alyante MSSQL — Report DDT cespiti)
```sql
SELECT * FROM Tsmi_DDT_Verifica_Cespiti;
```
Resolves Q-A6: the column list is defined by the Alyante view projection — it is **not** in the Appsmith export. For the 1:1 port, the backend will expose whatever columns the view currently returns at runtime, and the frontend table will bind to them dynamically (same as Appsmith). Audit lists "12 columns incl. `Seriali`, `Importo_unitario`"; the full set is discoverable only by querying the live view.

### B.4.13 Dettaglio ordini — verbatim widget bindings (key ternaries)

Preserved 1:1 unless otherwise noted.

```text
# cdlan_tipodoc
<b>Tipo di documento: </b>{{Order.data[0].cdlan_tipodoc == 'TSC-ORDINE-RIC' ? 'Ordine ricorrente' : 'Ordine Spot'}}

# cdlan_cod_termini_pag (BUG PRESENT — to be fixed on port, decision A.5.1a)
<b>Condizioni di pagamento:</b> {{
Order.data[0].cdlan_cod_termini_pag == 301 ? 'Vista fattura' :
Order.data[0].cdlan_cod_termini_pag == 303 ? 'BB FM' :
Order.data[0].cdlan_cod_termini_pag == 304 ? 'BB Vista fattura' :
Order.data[0].cdlan_cod_termini_pag == 311 ? 'BB 30ggDF' :
Order.data[0].cdlan_cod_termini_pag == 312 ? 'BB 30ggFM' :
Order.data[0].cdlan_cod_termini_pag == 313 ? 'BB 60ggDF' :
Order.data[0].cdlan_cod_termini_pag == 314 ? 'BB 60ggFM' :
Order.data[0].cdlan_cod_termini_pag == 315 ? 'BB 90ggDF' :
Order.data[0].cdlan_cod_termini_pag == 316 ? 'BB 90ggFM' :
Order.data[0].cdlan_cod_termini_pag == 318 ? 'BB 120ggFM' :
Order.data[0].Order == 400 ? 'SDD FM' :                  ← BUG: should be cdlan_cod_termini_pag
Order.data[0].cdlan_cod_termini_pag == 402 ? 'SDD 30ggDF' :
Order.data[0].cdlan_cod_termini_pag == 403 ? 'SDD 30ggFM' :
Order.data[0].cdlan_cod_termini_pag == 404 ? 'SDD 60ggDF' :
Order.data[0].cdlan_cod_termini_pag == 405 ? 'SDD 60ggFM' :
Order.data[0].cdlan_cod_termini_pag == 406 ? 'SDD 90ggDF' :
Order.data[0].cdlan_cod_termini_pag == 407 ? 'SDD 90ggFM' :
Order.data[0].cdlan_cod_termini_pag == 409 ? 'SDD DFFM' : ''}}
```

Port target (after decision A.5.1a): single lookup table/enum with all 18 codes (301, 303, 304, 311–316, 318, 400, 402–407, 409); code 317, 308, 309, 401, 408 are *not mapped* in the current Appsmith export — preserving that gap, any value outside the list renders as the empty string `''`.

```text
# cdlan_note (preserving content, fixing null-equivalence per A.5.1c)
<b>Note Legali: </b>{{Order.data[0].cdlan_note == '' ? 'Nessuna nota legale' : Order.data[0].cdlan_note}}

# cdlan_tipo_ord
<b>Tipo di ordine:</b> {{ Order.data[0].cdlan_tipo_ord == 'N' ? 'Nuovo' :
                         Order.data[0].cdlan_tipo_ord == 'A' ? 'Sostituzione' :
                         Order.data[0].cdlan_tipo_ord == 'R' ? 'Rinnovo' : '' }}

# cdlan_dur_rin (Quadrimestrale = code 4 here — vs 5 in cdlan_int_fatturazione; decision A.5.1b preserve)
<b>Durata rinnovo:</b> {{Order.data[0].cdlan_dur_rin == 1 ? 'Mensile' :
                         Order.data[0].cdlan_dur_rin == 2 ? 'Bimestrale' :
                         Order.data[0].cdlan_dur_rin == 3 ? 'Trimestrale' :
                         Order.data[0].cdlan_dur_rin == 4 ? 'Quadrimestrale' :
                         Order.data[0].cdlan_dur_rin == 6 ? 'Semestrale' :
                         Order.data[0].cdlan_dur_rin == 12 ? 'Annuale' : ''}}

# cdlan_tacito_rin
<b>Tacito rinnovo: </b>{{Order.data[0].cdlan_tacito_rin == 1 ? 'Sì' : 'No'}}

# data_decorrenza (preserving content, fixing null-equivalence per A.5.1c)
<b>Data decorrenza: </b>{{Order.data[0].data_decorrenza == '' ? 'Nessun valore' : Order.data[0].data_decorrenza}}

# cdlan_int_fatturazione — label is pre-computed server-side (see Order SQL §B.4.10, CASE WHEN)
<b>Modalità di fatturazione canoni anticipata:</b> {{Order.data[0].cdlan_int_fatturazione_desc}}

# cdlan_int_fatturazione_att — label is pre-computed server-side
<b>Modalità di fatturazione attivazione:</b> {{Order.data[0].cdlan_int_fatturazione_att_desc}}
```

### B.4.14 Transazioni WHMCS — `utils.runReport` orchestration (carbone.io)
```js
async function () {
  await getTransactions.run();
  utils.dati = getTransactions.data;
  utils.reportName = "transazioni_whmcs_dal_" + i_dal.formattedDate + "_al_" + i_al.formattedDate;
  await render_template.run();                       // POST /render/{templateId} on carbone.io
  const url = "https://render.carbone.io/render/" + render_template.data.data.renderId;
  navigateTo(url, {}, 'NEW_WINDOW');
}
```
Port target (decision A.5.4 = 4a): backend endpoint accepts `{from, to}`, runs the equivalent of `getTransactions`, POSTs to carbone.io with the backend-owned `templateId`, returns the `renderId` to the client, client opens `https://render.carbone.io/render/{renderId}` in a new tab. The frontend never sees the `templateId`.

---

## B.5 Phase B done-check

- [x] Every view classified (R1–R6) with user intent, sections, entry/exit.
- [x] Verbatim SQL / REST / binding quotations in Appendix (resolves Q-A1..Q-A6).
- [x] Route table for the mini-app finalized.
- [x] No UX design decisions pending — all follow portal conventions (`AppShell` + `TabNavGroup`, React Router, `@mrsmith/ui` primitives).
- [x] Open items carried forward are *business TODOs* (A.5.1b, A.5.1e — logged in `docs/TODO.md`), not UX unknowns.

Ready to proceed to Phase C (Logic Placement).
