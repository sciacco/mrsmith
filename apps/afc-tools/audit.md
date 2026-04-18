# AFC Tools — Appsmith Audit

Source: `apps/afc-tools/AFC-Tools.json.gz` (Appsmith export, artifact version current as of export).
Scope: reverse-engineering inventory to feed `appsmith-migration-spec`. No target code is produced here.

## 1. Application inventory

| Property | Value |
|---|---|
| Application name | AFC Tools |
| Slug | `afc-tools` |
| Pages | 8 |
| Datasources | 6 |
| Actions (queries/APIs) | 21 |
| JSObjects (action collections) | 5 |
| Custom JS libs | 0 |

### Pages

| # | Name | Slug | Purpose (inferred) |
|---|---|---|---|
| 1 | Transazioni whmcs | `transazioni-whmcs` | Browse/export WHMCS (Prometeus) billing transactions between two dates, export to XLSX via carbone.io. |
| 2 | Fatture Prometeus | `fatture-prometeus` | Read-only list of the last 2000 invoice rows from WHMCS "rigaaliante" feed to Alyante. |
| 3 | Nuovi articoli da inserire | `nuovi-articoli-da-inserire` | List of Mistra products not yet present in Alyante ERP master data — candidates to be created. |
| 4 | Report XConnect e Remote Hands | `report-xconnect-e-remote-hands` | Two tabs: download RH ticket PDF by ticket number, and list XConnect EVASO orders with "Download PDF" per row. |
| 5 | Consumi variabili Energia Colo | `consumi-variabili-energia-colo` | Year-filtered view of colocation energy consumption: monthly pivot table + per-period detail. |
| 6 | Ordini Sales | `ordini-sales` | List of active/sent sales orders from Vodka/daiquiri (MySQL); navigates to Dettaglio ordini. |
| 7 | Dettaglio ordini | `dettaglio-ordini` | Per-order detail + rows (reached via `?id=`). |
| 8 | Report DDT per cespiti | `report-ddt-per-cespiti` | Read-only dump of Alyante `Tsmi_DDT_Verifica_Cespiti` (DDT/cespiti verification). |

### Datasources

| Name | Plugin | Purpose | Notes |
|---|---|---|---|
| Alyante | `mssql-plugin` | ERP Sistemi (MSSQL at `172.16.1.16:1433`) | Read-only custom views (`Tsmi_*`). |
| Vodka / daiquiri | `mysql-plugin` | Sales/orders DB (CRM-side) | Used only by Ordini Sales + Dettaglio ordini. |
| grappa | `mysql-plugin` | Billing + colocation consumption DB | Used by Consumi variabili Energia Colo (`importi_corrente_colocation`, `cli_fatturazione`). |
| mistra | `postgres-plugin` | Mistra products + orders catalog (PostgreSQL) | Used by "Nuovi articoli" and "All orders XConnect". |
| whmcs_prom | `mysql-plugin` | WHMCS (Prometeus) billing + invoices feed | Used by Transazioni and Fatture Prometeus. |
| carbone.io | `restapi-plugin` | External document rendering service (XLSX/PDF templates) | Hard-coded template id in JSObject. |
| DEFAULT_REST_DATASOURCE | `restapi-plugin` | `https://gw-int.cdlan.net` — internal API gateway | Already the correct target for the Mistra NG Internal API. Only used by two actions. |

### JSObjects

| Name | Page | Role |
|---|---|---|
| utils | Transazioni whmcs | Holds carbone.io template id and orchestrates export (`runReport`). |
| TicketTools | Report XConnect/RH | `downloadTicketPDF`: validates inputs, calls REST, converts body to Blob, triggers browser download. |
| OrderTools | Report XConnect/RH | `download(orderId)`: same pattern as TicketTools but for order PDFs. |
| JSObject1 | Consumi Energia Colo | Empty placeholder (dead code). |
| JSObject2 | Consumi Energia Colo | `printCurrentYear()` — used only as placeholder text for the year input. |

### Global navigation

- No global layout/nav — each page is independent.
- The only programmatic navigation is `Ordini Sales → Dettaglio ordini?id={id}` (row icon button) and a `Torna alla lista ordini` back button.
- `Transazioni whmcs` triggers `navigateTo(url, {}, 'NEW_WINDOW')` to open the carbone.io renderer URL in a new tab.

---

## 2. Per-page audit

### 2.1 Transazioni whmcs

- **Purpose**: User picks a date range; app lists WHMCS payment/refund transactions and can export them to XLSX via carbone.io.
- **Widgets**:
  - `i_dal` DATE_PICKER — default: `moment().subtract(15,'days')`.
  - `i_al` DATE_PICKER — default: `Date()` (today).
  - `Button1` (Cerca) — `onClick: getTransactions.run()`.
  - `Button2` (Esporta) — `onClick: utils.runReport()`.
  - `tbl_transactions` TABLE — `tableData: {{getTransactions.data}}`, columns: cliente, fattura, invoiceid, userid, payment_method, date, description, amountin, fees, amountout, rate, transid, refundid, accountsid.
- **On load**: none (user must press Cerca).
- **Event flow**:
  - Cerca → `getTransactions` query (identical body to orphaned `transazioni_whmcs`).
  - Esporta → `utils.runReport()`:
    1. `getTransactions.run()`
    2. Store result in `utils.dati`, build `utils.reportName = "transazioni_whmcs_dal_{formatted}_al_{formatted}"`.
    3. POST `render_template` to carbone.io with body `{convertTo: "xlsx", reportName, data: { righe: dati }}` and hard-coded `templateId`.
    4. Compute `https://render.carbone.io/render/{renderId}` and `navigateTo(url, {}, 'NEW_WINDOW')`.
- **Hidden logic / constraints embedded in query**:
  - Hard-coded floor `date > 20230120` inside SQL — business data before Jan 20 2023 is silently excluded.
  - Filter `(fattura <> '' AND invoiceid > 0) OR refundid > 0` — only transactions tied to an invoice OR a refund are shown.
  - Date values are interpolated from `selectedDate` into an integer comparison (`BETWEEN {{i_dal.selectedDate}} AND {{i_al.selectedDate}}`) against a numeric `date` column — works only because the column stores a `YYYYMMDD` integer and Appsmith emits dates as strings. Fragile.
- **Candidate domain entities**: `Transaction` (payment/refund) linked to `Invoice`, `User`, `Account`.
- **Migration notes**:
  - `transazioni_whmcs` action is an orphan (no widget references it); `getTransactions` is the live copy. Drop `transazioni_whmcs` on migration.
  - `utils.templateId` is a carbone.io artifact — becomes a backend config value.
  - XLSX generation should move server-side: backend queries WHMCS, calls carbone.io, streams XLSX to the browser. Avoids exposing the template id and the navigateTo-to-external-URL pattern.

### 2.2 Fatture Prometeus

- **Purpose**: Read-only audit of the last 2000 invoice lines produced by WHMCS for Alyante import.
- **Widgets**: `tbl_fatture_prometeus` bound to `righealiante.data` (31 columns).
- **On load**: `righealiante`.
- **Event flow**: page load → query → render.
- **Hidden logic**:
  - Hard-coded `LIMIT 2000`, `ORDER BY id DESC`. No user-controlled filter.
- **Candidate domain entities**: `InvoiceLine` feed (WHMCS→Alyante).
- **Migration notes**:
  - Needs pagination / date filter; current 2000-row cap is a silent business limit.

### 2.3 Nuovi articoli da inserire

- **Purpose**: Show Mistra products (`products.product`) that are flagged for ERP sync but not present in `loader.erp_anagrafica_articoli_vendita` (i.e., not yet created in Alyante).
- **Widgets**: `tbl_lista_articoli` bound to `articoli_non_in_alyante.data`, columns: code, categoria, descrizione_it, descrizione_en, nrc, mrc.
- **On load**: `articoli_non_in_alyante`.
- **Hidden logic / business rules (in SQL)**:
  - `erp_sync = true` filter — only products flagged for sync are eligible.
  - `RIGHT JOIN` pattern plus `WHERE a.cod_articolo IS NULL` — classic "anti-join" to find missing rows. This rule ("article is missing if absent from `erp_anagrafica_articoli_vendita`") is a genuine business rule embedded in SQL.
  - Translation aggregation uses `MAX(CASE WHEN language='it'…)` to pivot IT/EN descriptions.
- **Candidate domain entities**: `Product`, `ProductCategory`, `Translation`, ERP sync status.
- **Migration notes**: SQL joins cross `loader`, `products`, `common` schemas — becomes a backend endpoint on Mistra PG; do not expose the join logic to the UI.

### 2.4 Report XConnect e Remote Hands

- **Purpose**: Two tabs:
  1. Download the PDF of a Remote Hands ticket by ticket number.
  2. List of EVASO XConnect orders with a per-row "Download PDF" button.
- **Widgets**:
  - Tab 1 (Canvas1): `numeroTicket` input, `Seleziona_lingua` select (static `it`/`en` options), `Scarica_PDF_button` (onClick `TicketTools.downloadTicketPDF`), `Text1` instructional copy, `Divider1`.
  - Tab 2 (Canvas2): `Table1` bound to `All_orders_xcon.data`, columns id_ordine, codice_ordine, cliente, data_creazione, customColumn1 (button "Scarica PDF...") → `OrderTools.download(Table1.triggeredRow.id_ordine)`.
- **On load**: `All_orders_xcon` (Mistra PG query joining `loader.hubs_deal`, `loader.cp_ordini`, `orders.order`, `customers.customer`, `orders.order_state`, filtered `kit_category='XCONNECT' AND state='EVASO'`).
- **Event flow**:
  - `downloadTicketPDF`:
    1. Validate `ticketId` + `language`, else `showAlert`.
    2. Call `DownloadTicketPDF` (`GET /tickets/v1/pdf/{ticketId}?ticket_type=RemoteHands&lang={language}` on `gw-int.cdlan.net`).
    3. Detect base64 vs raw bytes (heuristic: starts with `%PDF` vs regex `/^[A-Za-z0-9+/]+={0,2}$/`).
    4. Build `Blob`, `URL.createObjectURL`, call Appsmith helper `download(url, filename, mime)`.
  - `download(orderId)`: same pattern against `GET /orders/v1/order/pdf/{orderId}`, with a 404 specialization (`"Il PDF non è ancora pronto."`).
- **Hidden logic**:
  - Base64-detection heuristic: `payload.startsWith("%PDF")` *or* regex on the first 80 chars — a pragmatic workaround for Appsmith wrapping binary payloads as strings. On migration this whole branch disappears: in a native fetch the body can be consumed as a `Blob` directly.
  - `ticket_type` is pinned to `RemoteHands` (XConnect tickets are not supported by this flow despite the page title).
- **Candidate domain entities**: `Ticket` (RH), `Order` (XConnect), `Customer`.
- **Migration notes**:
  - `DownloadOrderPDF` / `DownloadTicketPDF` are already on the internal gateway and can be proxied unchanged.
  - `All_orders_xcon` needs to be a backend endpoint hitting Mistra NG (the same system the gateway fronts) — don't replicate direct PG access from the client.

### 2.5 Consumi variabili Energia Colo

- **Purpose**: Given a year, show monthly colocation energy consumption by customer (pivoted) plus per-period detail.
- **Widgets**:
  - `TXT_anno` input — placeholder `{{JSObject2.printCurrentYear()}}` (NB: placeholder only, not a default).
  - `BTN_ricerca` (Cerca) — `onClick: Q_select_consumi_colo_filter.run(); Q_select_consumi_colo.run()`.
  - `TBL_ConsumiColo` (pivot) bound to `Q_select_consumi_colo_filter.data`.
  - `TBL_ConsumiColoDetail` bound to `Q_select_consumi_colo.data`.
- **On load**: both queries — with an empty `TXT_anno` the SQL interpolates `year(i.start_period)=''`, which MySQL will evaluate as false; effective result: empty tables until the user searches. Likely unintentional.
- **Event flow**: Cerca runs both queries in parallel (no `await`, no error handling).
- **Hidden logic / business rules**:
  - `IF(i.ampere > 0, i.ampere, i.Kw)` — consumption displayed is "Ampere if positive, else Kw". This is a *business rule*, not presentation.
  - Pivot SQL produces monthly columns `Gennaio…Dicembre` as sums — defines the 12-month report shape server-side.
  - `year('{{TXT_anno.text}}')` — year is interpolated as a string into a SQL function; not parameterized.
- **Open question**: `tipo_variabile`, `coefficiente`, `fisso_cu`, `eccedenti`, `importo_eccedenti` are shown in the detail but never explained — business meaning needs user confirmation.
- **Migration notes**:
  - The IF(ampere, kw) pivot is a clear backend aggregation endpoint; year becomes a typed query param.
  - `JSObject1`/`myFun1`/`myFun2` are empty placeholders — delete.

### 2.6 Ordini Sales

- **Purpose**: Active/sent sales orders list from Vodka/daiquiri; click icon to drill down.
- **Widgets**:
  - `Lista_ordini` bound to `Select_Orders_Table.data`. A derived column `customColumn1` (icon button) does `navigateTo('Dettaglio ordini', {id: triggeredRow.id})`.
  - `Dettaglio_ordine` modal with placeholder title/close/confirm buttons — **declared but never opened from any event**; it is unused UI.
- **On load**: `Select_Orders_Table`.
- **Hidden logic / business rules (in SQL)**:
  - `WHERE cdlan_stato IN ('ATTIVO','INVIATO')` — only two order states surfaced.
  - `IF(is_colo != 0, is_colo, service_type)` — "Tipo di servizi" is the colocation value when set, else the generic service type.
  - `cdlan_tipo_ord` code mapping: A→Sostituzione, N→Nuovo, R→Rinnovo (SQL-side CASE).
  - `from_cp != 0` → "Sì"/"No" (came from CP? — Commercial Platform, presumably).
- **Migration notes**:
  - Unused `Dettaglio_ordine` modal should be removed from scope.
  - Navigation via query string `?id=` is fine in the rewrite; keep the same contract.

### 2.7 Dettaglio ordini

- **Purpose**: Header + rows detail of one order (id from URL querystring).
- **Widgets**:
  - 20+ TEXT_WIDGETs rendering header fields with HTML `<b>…</b>` bindings against `Order.data[0].*`.
  - `TBL_OrderRows` bound to `RigheOrdine.data`.
  - `TornaIndietro` button → `navigateTo('Ordini Sales')`.
- **On load**: `RigheOrdine`, `Order` (both depend on `appsmith.URL.queryParams.id`).
- **Hidden logic / business rules** (critical — most UI ternaries are actually business mappings):
  - `cdlan_tipodoc`: `TSC-ORDINE-RIC` → "Ordine ricorrente", else "Ordine Spot".
  - `cdlan_tipo_ord` mapping A/N/R (as in Ordini Sales).
  - `cdlan_dur_rin` mapping: 1→Mensile, 2→Bimestrale, 3→Trimestrale, **4→Quadrimestrale**, 6→Semestrale, 12→Annuale.
  - `cdlan_tacito_rin`: 1→"Sì", else "No".
  - `cdlan_int_fatturazione` (in `Order` SQL): 1→Mensile, 2→Bimestrale, 3→Trimestrale, **5→Quadrimestrale**, 6→Semestrale, else→Annuale.
  - `cdlan_int_fatturazione_att`: 1→"All'ordine", else "All'attivazione della Soluzione/Consegna".
  - `cdlan_cod_termini_pag`: extensive mapping (301, 303, 304, 311–318, 400–409) to human-readable payment terms; embedded inside a single text widget.
  - `RigheOrdine` SQL: `IF(cdlan_codice_kit != '', CONCAT(cdlan_codice_kit, '-', index_kit), '')` — kit bundle code composition rule.
- **Bugs observed** (to preserve as "fix on migration" notes):
  1. `cdlan_cod_termini_pag` mapping contains `Order.data[0].Order == 400 ? 'SDD FM'` — typo: should read `cdlan_cod_termini_pag == 400`. Value 400 is currently unreachable.
  2. Quadrimestrale maps to **4** on `cdlan_dur_rin` (widget) but **5** on `cdlan_int_fatturazione` (SQL CASE). Either the two fields genuinely use different codings, or one side is wrong — confirm with business.
  3. `cdlan_note == ''` and `data_decorrenza == ''` null-checks use `== ''` instead of `null/undefined` — might render "Nessun valore" incorrectly depending on DB NULLs vs empty strings.
- **Candidate domain entities**: `Order`, `OrderRow`, `OrderKit`, `PaymentTerm`, `BillingFrequency`.
- **Migration notes**:
  - All the ternary mappings must be extracted into backend lookup tables or a shared enum translation module; the same codes recur in Ordini Sales.
  - HTML-in-text bindings (`<b>…</b>`) are presentation — replace with proper typography.

### 2.8 Report DDT per cespiti

- **Purpose**: Full dump of Alyante MSSQL view `Tsmi_DDT_Verifica_Cespiti` (cespiti/DDT verification).
- **Widgets**: `TBL_DDTCespiti` bound to `ListaDdtVerificaCespiti.data`, 12 columns including `Seriali`, `Importo_unitario`, etc.
- **On load**: `ListaDdtVerificaCespiti`.
- **Hidden logic**:
  - Query is literally `SELECT * FROM Tsmi_DDT_Verifica_Cespiti` followed by the default Appsmith placeholder comment — **no filter, no limit**. On a large fleet, this will return the full table every load. Risk of timeouts.
- **Migration notes**:
  - Needs pagination + filter; this is the weakest page performance-wise.

---

## 3. Datasource & query catalog

| Query | Datasource | R/W | Page | Purpose | Inputs | Orphan? | Rewrite target |
|---|---|---|---|---|---|---|---|
| `transazioni_whmcs` | whmcs_prom (MySQL) | R | Transazioni whmcs | Duplicate of `getTransactions` | `i_dal`, `i_al` | **Yes** (no widget reference) | Delete. |
| `getTransactions` | whmcs_prom | R | Transazioni whmcs | Return WHMCS transactions in date range | `i_dal.selectedDate`, `i_al.selectedDate` | No | Backend endpoint `GET /afc/whmcs/transactions?from=&to=`. |
| `render_template` | carbone.io (REST) | Write (ext.) | Transazioni whmcs | POST `/render/{templateId}` to generate XLSX | `utils.dati`, `utils.reportName`, `utils.templateId` | No | Backend proxies carbone.io; streams XLSX to client. |
| `getURL` | n/a (JS placeholder) | — | Transazioni whmcs | Construct `render.carbone.io/render/{renderId}` | `render_template.data.data.renderId` | Superseded by `utils.getURL()` | Remove; backend handles the full flow. |
| `runReport` | n/a (JS placeholder) | — | Transazioni whmcs | Duplicate of `utils.runReport` | — | **Yes** (JSObject `utils.runReport` is the live copy) | Delete. |
| `righealiante` | whmcs_prom | R | Fatture Prometeus | Last 2000 WHMCS→Alyante invoice lines | — | No | Backend endpoint with pagination + date filter. |
| `articoli_non_in_alyante` | mistra (PG) | R | Nuovi articoli | Products flagged for ERP sync but missing in Alyante | — | No | Backend endpoint on Mistra. |
| `downloadTicketPDF` | n/a (JS placeholder) | — | Report XConnect/RH | Dup of `TicketTools.downloadTicketPDF` | — | **Yes** | Delete. |
| `DownloadOrderPDF` | DEFAULT_REST (`gw-int.cdlan.net`) | R | Report XConnect/RH | GET `/orders/v1/order/pdf/{orderId}` | `params.orderId` | No | Proxy through new backend (preserves auth). |
| `DownloadTicketPDF` | DEFAULT_REST | R | Report XConnect/RH | GET `/tickets/v1/pdf/{ticketId}?ticket_type=RemoteHands&lang=` | `numeroTicket.text`, `Seleziona_lingua.selectedOptionValue` | No | Proxy via backend; drop `ticket_type` hard-pin if other types become needed. |
| `All_orders_xcon` | mistra (PG) | R | Report XConnect/RH | List EVASO XConnect orders | — | No | Backend endpoint on Mistra NG. |
| `download` | n/a (JS placeholder) | — | Report XConnect/RH | Dup of `OrderTools.download` | — | **Yes** | Delete. |
| `Q_select_consumi_colo_filter` | grappa (MySQL) | R | Consumi Energia Colo | Monthly pivot of colocation consumption | `TXT_anno.text` | No | Backend aggregation endpoint. |
| `Q_select_consumi_colo` | grappa | R | Consumi Energia Colo | Raw per-period rows | `TXT_anno.text` | No | Same, detail variant. |
| `myFun1`/`myFun2`/`printCurrentYear` | n/a (JS) | — | Consumi Energia Colo | Placeholders; only `printCurrentYear` is consumed (input placeholder) | — | Mostly yes | Replace `printCurrentYear` with a literal / default value; drop the rest. |
| `Select_Orders_Table` | Vodka / daiquiri (MySQL) | R | Ordini Sales | Active/sent sales orders | — | No | Backend endpoint with pagination, optional filter. |
| `RigheOrdine` | Vodka / daiquiri | R | Dettaglio ordini | Rows for a given order | `URL.queryParams.id` | No | Backend endpoint `GET /afc/orders/{id}/rows`. |
| `Order` | Vodka / daiquiri | R | Dettaglio ordini | Header of a given order | `URL.queryParams.id` | No | Backend endpoint `GET /afc/orders/{id}`. |
| `ListaDdtVerificaCespiti` | Alyante (MSSQL) | R | Report DDT cespiti | `SELECT * FROM Tsmi_DDT_Verifica_Cespiti` | — | No | Backend endpoint with pagination/filters. |

All actions except the two REST calls to `gw-int.cdlan.net` access backing databases **directly from the client** — this is the single biggest migration concern.

---

## 4. Findings summary

### 4.1 Embedded business rules (must move out of UI)

1. **Payment-terms code mapping** (`cdlan_cod_termini_pag`, 18 values) — currently one giant ternary in a text widget; buggy (value 400 unreachable). Candidate: shared enum/lookup.
2. **Order type mapping** A/N/R — duplicated in SQL (`Select_Orders_Table`) and in UI ternary (`Dettaglio ordini`).
3. **Billing frequency / renewal period mappings** — `cdlan_dur_rin` (UI) and `cdlan_int_fatturazione` (SQL) use slightly different numeric codes (4 vs 5 for Quadrimestrale); needs domain confirmation.
4. **Order state filter** `cdlan_stato IN ('ATTIVO','INVIATO')` — product decision baked into SQL.
5. **Service type coalesce** `IF(is_colo != 0, is_colo, service_type)` — colocation-first priority.
6. **WHMCS transaction filter** `((fattura <> '' AND invoiceid > 0) OR refundid > 0)` plus `date > 20230120` — business floor + "only billed/refunded" rule.
7. **"Missing in Alyante"** anti-join plus `erp_sync=true` gate.
8. **Energy consumption metric** `IF(ampere>0, ampere, kw)` — consumption is Ampere when present, else Kw.
9. **Kit bundle article code** `CONCAT(cdlan_codice_kit, '-', index_kit)` composition rule.
10. **Remote-Hands-only ticket download** — `ticket_type=RemoteHands` hard-coded in query.

### 4.2 Duplication

- `transazioni_whmcs` = `getTransactions` (same SQL, only the second is referenced).
- `getURL`, `runReport`, `downloadTicketPDF`, `download` are DB-less "actions" that duplicate the JSObject code of `utils.getURL`, `utils.runReport`, `TicketTools.downloadTicketPDF`, `OrderTools.download`. Appsmith generated them as siblings; only the JSObject versions are wired up.
- Type/frequency/state code mappings are repeated SQL-side (CASE) and UI-side (ternary).

### 4.3 Security & operational concerns

- **Direct DB access from the browser** to Alyante (ERP, MSSQL), Vodka/daiquiri (MySQL), grappa (MySQL), mistra (PG), whmcs_prom (MySQL). SQL lives in the client bundle. Parameters are bound via Appsmith templating, so injection via `{{TXT_anno.text}}` into `year('…')` is unlikely but the pattern is fragile.
- **Carbone.io `templateId` exposed** in the exported Appsmith JSON (JSObject variable `utils.templateId`); anyone with app access can reuse it.
- **Full-table `SELECT *`** on `Tsmi_DDT_Verifica_Cespiti` on every page load → potential DB pressure / timeouts.
- **`navigateTo(carbone URL, NEW_WINDOW)`** opens the raw carbone.io render URL to the end user; the renderId flows through the browser.
- **`All_orders_xcon` joins five Mistra tables client-side**; any schema change breaks the UI silently.
- The "base64 vs bytes" heuristic is a brittle workaround for Appsmith's binary handling — irrelevant in a native frontend but flags that these endpoints return different payload shapes depending on gateway config.

### 4.4 Migration blockers / open questions

- The internal gateway (`gw-int.cdlan.net`) is already fronting two endpoints; need to confirm whether equivalent endpoints exist (or must be added) for: transazioni WHMCS, righe aliante, articoli mancanti, consumi colo, ordini sales, dettaglio ordini, DDT cespiti.
- `Dettaglio_ordine` modal in Ordini Sales is dead UI — confirm it is truly unused before removing.
- `cdlan_dur_rin`/`cdlan_int_fatturazione` code divergence for Quadrimestrale (4 vs 5) must be resolved with the business owner.
- `tipo_variabile`, `coefficiente`, `fisso_cu`, `eccedenti`, `importo_eccedenti` semantics on the Energia Colo detail page are undocumented.
- `date > 20230120` hard floor on WHMCS transactions — is this a permanent business rule or a leftover from the initial migration to the new billing system?
- Access control: Appsmith export does not include role/group bindings; the new mini-app needs its own Keycloak role (`app_afctools_access`) and per-page or per-query RBAC aligned with AFC team scope.

### 4.5 Recommended next steps

1. Run `appsmith-migration-spec` over this audit to produce the per-page PRD and API contract draft.
2. Confirm / fix the mapping bugs (payment-terms 400 typo; Quadrimestrale code divergence).
3. Inventory which Mistra NG Internal API endpoints already cover these flows; for each query in the catalog, decide reuse vs. add new endpoint.
4. Design a single ERP/billing read-only service in the Go backend that centralizes access to Alyante, Vodka/daiquiri, grappa, whmcs_prom, Mistra — the UI then only talks to `/api/afc-tools/*`.
5. Plan deprecation of carbone.io templateId exposure: backend should own the template and stream the generated XLSX.
6. Drop dead actions (`transazioni_whmcs`, `runReport` action, `getURL` action, `downloadTicketPDF` action, `download` action, `myFun1/myFun2`) from the mental model during rewrite.

---

## Classification quick-view

Every major binding/behavior, bucketed per the skill's three categories:

| Item | Bucket |
|---|---|
| Widget tables, date pickers, selects, modal shell, typography (`<b>`) | Presentation |
| On-load query orchestration (`layoutOnLoadActions`), row→detail navigation, button→query wiring, modal open/close, base64→Blob download pipeline, placeholder text defaults, multi-query parallel fan-out on Cerca | Frontend orchestration |
| `date > 20230120` floor, WHMCS invoice/refund filter, anti-join for missing articoli, IF(ampere, kw) consumption rule, CASE mappings for tipo_ord/termini_pag/int_fatturazione/dur_rin, `cdlan_stato IN ('ATTIVO','INVIATO')`, kit-code concatenation, ticket_type=RemoteHands pin, IF(is_colo,…) service-type coalesce, `erp_sync=true` + `a.cod_articolo IS NULL` eligibility | **Business logic** (move to backend) |

## Done checklist

- [x] Every page inventoried (8/8).
- [x] All 21 actions + 5 JSObjects catalogued, with orphans flagged.
- [x] Hidden logic (visibility/default/derived values/chained actions) surfaced.
- [x] Findings classified business / orchestration / presentation.
- [x] Ready to feed directly into `appsmith-migration-spec`.
