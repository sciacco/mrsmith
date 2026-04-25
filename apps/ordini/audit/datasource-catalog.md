# Datasource & query catalog — Ordini

Legend — **Target layer** recommendation for the rewrite:
- **BE** = server-side (Go backend) endpoint.
- **FE-derived** = client-side computation from other data; no network call needed.
- **DROP** = dead code or orphan, do not port.

---

## Datasources

### `Alyante` — Microsoft SQL Server
- Host `172.16.1.16:1433`, READ_WRITE, SSL `NO_VERIFY`.
- Only one read query (`erp_anagrafiche_cli`). Write-mode was declared but never used by this app.
- **Rewrite recommendation:** the Go backend must fetch customer anagraphics on the server side. Prefer routing through the canonical Mistra customer entity (see `docs/IMPLEMENTATION-KNOWLEDGE.md`: Alyante ID `NUMERO_AZIENDA` = Mistra `customers.customer.id` = Grappa `cli_fatturazione.codice_aggancio_gest`).

### `db-mistra` — PostgreSQL
- Host `10.129.32.20` (default port 5432), READ_WRITE, SSL DEFAULT.
- Used only by `Ordini semplificati` (unfinished page).
- **Rewrite recommendation:** unless the HubSpot-based order-creation flow is carried forward, this datasource is out of scope. Otherwise the Go backend should read from the same `loader.hubs_*` tables and expose `/ordini/ref/potentials` and `/ordini/ref/payment-terms`.

### `vodka` — MySQL
- Host `10.129.32.7:3306`, READ_WRITE, SSL DEFAULT.
- Primary datastore for this app: `orders`, `orders_rows`.
- **Rewrite recommendation:** the Go backend owns all orders mutations. No direct connection from the React app. Parameterized queries required (today everything is string-interpolated).

### `GW internal CDLAN` — REST
- Base URL `https://gw-int.cdlan.net`, no default headers/auth in the export.
- The gateway is already the sanctioned bridge between apps and ERP / PDF / Arxivar. Most of its endpoints are under `/orders/v1/…`.
- **Rewrite recommendation:** call the GW from the Go backend, not from the browser. Keep the endpoints; move credentials server-side; add timeouts and retry policy; log request IDs.

---

## Queries

### Home page

#### `Select_Orders_Table` — vodka, onLoad
- **Purpose:** feed the main orders table with display-ready fields.
- **Inputs:** none.
- **Outputs:** `id`, `System ODV`, `Tipo di documento`, `Codice ordine`, `Numero proposta`, `Anno documento`, `Sostituisce ordini (Num/Anno)`, `Ragione sociale`, `Data proposta`, `Tipo di servizi`, `Tipo di proposta`, `Data conferma`, `Stato`, `Lingua`, `cdlan_evaso`, `Dal CP?`.
- **Dependencies:** none. Returns `ORDER BY id DESC`, no pagination.
- **SQL** (verbatim):
  ```sql
  SELECT 
  id,
  cdlan_systemodv AS "System ODV",
  cdlan_tipodoc AS "Tipo di documento",
  concat(cdlan_ndoc,'/',cdlan_anno) as "Codice ordine",
  cdlan_ndoc AS "Numero proposta",
  cdlan_anno AS "Anno documento",
  cdlan_sost_ord AS "Sostituisce ordini (Num/Anno)",
  cdlan_cliente AS "Ragione sociale",
  cdlan_datadoc AS "Data proposta",
  IF(is_colo != 0, is_colo, service_type) AS "Tipo di servizi",
  CASE cdlan_tipo_ord
      WHEN "A" THEN "Sostituzione"
      WHEN "N" THEN "Nuovo"
        WHEN "R" THEN "Rinnovo"
        ELSE NULL
  END AS "Tipo di proposta",
  cdlan_dataconferma AS "Data conferma",
  cdlan_stato AS "Stato",
  profile_lang AS "Lingua",
  cdlan_evaso,
  IF(from_cp != 0, "Sì", "No") AS "Dal CP?"
  FROM orders ORDER BY id DESC;
  ```
- **Target layer:** **BE** — `GET /api/ordini` returning paginated rows. Drop the embedded label mapping (`Tipo di proposta`, `Tipo di documento`, `Dal CP?`) and move it to the frontend formatter or a shared enum map.

#### `Dettaglio_ordine_vero` (Home instance) — vodka, onLoad
- **Purpose:** back the legacy Home modal with the full order record.
- **SQL:** `SELECT * FROM orders WHERE id = {{Lista_ordini.triggeredRow.id}};`
- **Target layer:** **DROP** (legacy modal is not reachable).

#### `Lista_righe_d_ordine` (Home instance) — vodka, onLoad
- **Purpose:** rows for the legacy Home modal.
- **SQL:**
  ```sql
  SELECT 
  cdlan_systemodv_row AS 'System ODV Riga',
  IF(cdlan_codice_kit != '', CONCAT(cdlan_codice_kit, '-', index_kit), '') AS 'Codice articolo bundle',
  cdlan_codart AS 'Codice articolo',
  cdlan_descart AS 'Descrizione articolo',
  cdlan_prezzo AS 'Canone',
  cdlan_prezzo_attivazione AS 'Prezzo attivazione',
  cdlan_qta AS 'Quantità',
  cdlan_prezzo_cessazione AS 'Prezzo cessazione',
  cdlan_ragg_fatturazione AS 'Codice raggruppamento fatturazione',
  cdlan_data_attivazione AS 'Data attivazione',
  cdlan_serialnumber AS 'Numero seriale'
  FROM orders_rows WHERE orders_id = {{Lista_ordini.triggeredRow.id }};
  ```
- **Target layer:** **DROP** (same reason).

#### `Lista_righe_d_ordine_info_tecn` — vodka, onLoad(false) but invoked from a binding
- **SQL:** `SELECT cdlan_systemodv_row AS 'System ODV Riga', cdlan_codart AS 'Codice articolo', cdlan_descart AS 'Descrizione articolo', note_tecnici AS 'Note tecnici', data_annullamento AS 'Data Annullamento' FROM orders_rows WHERE orders_id = {{Lista_ordini.triggeredRow.id}};`
- **Target layer:** **DROP**.

#### `Dettaglio_riga_d_ordine` — vodka
- **SQL:** `SELECT * FROM orders_rows WHERE id = {{Lista_righe_d_ordine. }};` — **broken binding** (trailing `.`).
- **Target layer:** **DROP**.

#### `Select_orders1`, `Total_record_orders1` — vodka
- **Purpose:** server-side paginated + search + sort variant of the list (count + page query).
- **SQL (Total):** `SELECT COUNT(*) FROM orders WHERE cdlan_ndoc LIKE '%{{Lista_ordini.searchText}}%';`
- **SQL (Select):** `SELECT * FROM orders WHERE cdlan_ndoc LIKE '%{{Lista_ordini.searchText}}%' ORDER BY {{Lista_ordini.sortOrder.column || 'id'}} {{Lista_ordini.sortOrder.order !== "desc" ? "" : "DESC"}} LIMIT {{Lista_ordini.pageSize}} OFFSET {{Lista_ordini.pageOffset}}`
- **Target layer:** **BE** — if server-side pagination is desired in the rewrite. Today the table has `totalRecordsCount: 0` and the queries are unused. **SQL injection risks** in `searchText` and raw `sortOrder.column`.

#### `Insert_orders1` — vodka
- **Purpose:** full-row INSERT built from `Lista_ordini.newRow` (50 columns). Not reachable from any widget today.
- **Target layer:** **DROP** unless order creation is brought back in-app. Even then, do not port the string-concatenated SQL — use the Go backend with a DTO.

#### `Update_orders1` — vodka
- **Purpose:** full-row UPDATE from `Lista_ordini.updatedRow`. Not reachable from any widget today.
- **Target layer:** **DROP** for the same reason.

#### `Query1` — vodka
- **SQL:** `SELECT * FROM orders ORDER BY cdlan_systemodv DESC;`
- **Target layer:** **DROP** (orphan).

---

### Ordini semplificati page

#### `get_potentials` — db-mistra, onLoad
- **Purpose:** list HubSpot deals at specific pipeline stages (candidate orders).
- **SQL:**
  ```sql
  select d.codice, d.name as deal_name, p.label as pipeline, ds.label as stage, c.name as company_name, o.email as owner , d.id
  from loader.hubs_deal d
  left join loader.hubs_company c on d.company_id = c.id
  left join loader.hubs_pipeline p on d.pipeline = p.id
  left join loader.hubs_stages ds on d.dealstage = ds.id
  left join loader.hubs_owner o on d.hubspot_owner_id = o.id
  where ((d.pipeline ='255768766' and ds.display_order between 3 and 8) or (d.pipeline = '255768768' and ds.display_order between 3 and 8)) and d.codice <> ''
  order by id desc;
  ```
- **Target layer:** **BE** (or **DROP**). If the HubSpot-to-order flow is kept, this is an `/ordini/ref/potentials` endpoint. Pipeline IDs `255768766` / `255768768` are magic constants — move them to config.

#### `get_payment_methods` — db-mistra
- **SQL:** `SELECT cod_pagamento, desc_pagamento FROM loader.erp_metodi_pagamento WHERE selezionabile is true ORDER BY desc_pagamento;`
- **Target layer:** **BE** — `/ordini/ref/payment-terms`. Payment terms are currently hard-coded in the `Form ordine` dropdown; a backend endpoint consolidates the two sources.

---

### Form ordine page

#### `Dettaglio_ordine_vero` (Form ordine instance) — vodka, onLoad
- **SQL:** `SELECT * FROM orders WHERE id = {{ appsmith.URL.queryParams.id }};`
- **Target layer:** **BE** — `GET /api/ordini/:id` returning the full record. Page itself is likely not ported; the query is the only useful artifact.

#### `Lista_righe_d_ordine` (Form ordine instance) — vodka, onLoad
- **SQL:** same body as the Home instance, but keyed off `appsmith.URL.queryParams.id`.
- **Target layer:** **BE** — `GET /api/ordini/:id/rows`.

---

### Dettaglio ordine page — reads

#### `Order` — vodka, onLoad
- **Purpose:** load the full order with CASE-computed label fields.
- **SQL:**
  ```sql
  SELECT id, cdlan_systemodv, cdlan_tipodoc, cdlan_ndoc, cdlan_datadoc, cdlan_cliente,
         cdlan_commerciale, cdlan_cod_termini_pag, cdlan_note, cdlan_tipo_ord,
         cdlan_dur_rin, cdlan_tacito_rin, cdlan_sost_ord, cdlan_tempi_ril,
         cdlan_durata_servizio, cdlan_dataconferma, cdlan_rif_ordcli,
         cdlan_rif_tech_nom, cdlan_rif_tech_tel, cdlan_rif_tech_email,
         cdlan_rif_altro_tech_nom, cdlan_rif_altro_tech_tel, cdlan_rif_altro_tech_email,
         cdlan_rif_adm_nom, cdlan_rif_adm_tech_tel, cdlan_rif_adm_tech_email,
         CASE cdlan_int_fatturazione
              WHEN '1' THEN 'Mensile'
              WHEN '2' THEN 'Bimestrale'
              WHEN '3' THEN 'Trimestrale'
              WHEN '5' THEN 'Quadrimestrale'
              WHEN '6' THEN 'Semestrale'
              ELSE 'Annuale'
         END AS cdlan_int_fatturazione_desc,
         cdlan_int_fatturazione,
         CASE cdlan_int_fatturazione_att
              WHEN '1' THEN 'All\'ordine'
              ELSE 'All\'attivazione della Soluzione/Consegna'
         END AS cdlan_int_fatturazione_att_desc,
         cdlan_int_fatturazione_att,
         cdlan_stato, cdlan_evaso, cdlan_chiuso, cdlan_anno, cdlan_valuta,
         written_by, profile_iva, profile_cf, profile_address, profile_city,
         profile_cap, profile_pv, profile_sdi, profile_lang, cdlan_cliente_id,
         service_type, data_decorrenza, cdlan_tacito_rin_in_pdf, is_colo,
         origin_cod_termini_pag, is_arxivar, from_cp, arx_doc_number
  FROM orders WHERE id = {{ appsmith.URL.queryParams.id }} LIMIT 1;
  ```
- **Bug note:** the `cdlan_int_fatturazione` CASE maps `'5'` to `Quadrimestrale`, but the `Form ordine` dropdown uses `'4'` for Quadrimestrale. Either the display mapping is wrong, or the dropdown values are wrong. Verify before porting.
- **Target layer:** **BE** — `GET /api/ordini/:id`. The `*_desc` fields are presentation, move to the frontend formatter.

#### `RigheOrdine` — vodka, onLoad
- **SQL:**
  ```sql
  SELECT 
  id AS 'ID Riga',
  cdlan_systemodv_row AS 'System ODV Riga',
  IF(cdlan_codice_kit != '', CONCAT(cdlan_codice_kit, '-', index_kit), '') AS 'Codice articolo bundle',
  cdlan_codart AS 'Codice articolo',
  cdlan_descart AS 'Descrizione articolo',
  cdlan_prezzo AS 'Canone',
  cdlan_prezzo_attivazione AS 'Attivazione',
  cdlan_qta AS 'Quantità',
  cdlan_prezzo_cessazione AS 'Prezzo cessazione',
  cdlan_ragg_fatturazione AS 'Codice raggruppamento fatturazione',
  cdlan_data_attivazione AS 'Data attivazione',
  cdlan_serialnumber AS 'Numero seriale',
  confirm_data_attivazione
  FROM orders_rows WHERE orders_id = {{ appsmith.URL.queryParams.id }} ;
  ```
- Note the column alias `'Attivazione'` (Dettaglio ordine) vs `'Prezzo attivazione'` (Home/Form ordine) — SendToErp refers to both `item["Attivazione"]` in `cdlanPrezzoAttivazione` and `item["Prezzo attivazione"]` in commented-out code. Follow `'Attivazione'` on Dettaglio ordine.
- **Target layer:** **BE** — `GET /api/ordini/:id/rows`.

#### `RigheOrdineTecnici` — vodka, onLoad
- **SQL:**
  ```sql
  SELECT  
  cdlan_systemodv_row as 'ID riga',
  concat(cdlan_codice_kit,'-',index_kit) as 'codice articolo bundle',
  cdlan_codart as 'codice articolo',
  convert(note_tecnici using UTF8) as 'note tecnici',
  data_annullamento as 'data annullamento'
  FROM orders_rows WHERE orders_id = {{ appsmith.URL.queryParams.id }};
  ```
- Note: `convert(note_tecnici using UTF8)` suggests the column has a non-UTF8 collation; the rewrite must preserve the conversion.
- **Target layer:** **BE** — same endpoint as `RigheOrdine` (union the columns), or a dedicated `/technical-notes` if separation is desired.

#### `erp_anagrafiche_cli` — Alyante, onLoad
- **SQL:**
  ```sql
  SELECT NUMERO_AZIENDA, RAGIONE_SOCIALE 
  FROM Tsmi_Anagrafiche_clienti 
  where DATA_DISMISSIONE is null 
  and RAGGRUPPAMENTO_3 <> 'Ecommerce' and TIPOLOGIA_AZIENDA <> 'DIPENDENTE'
  GROUP BY NUMERO_AZIENDA, RAGIONE_SOCIALE
  ```
- **Target layer:** **BE** — `/ordini/ref/customers`. The rewrite must persist the `NUMERO_AZIENDA` identifier (not just the display string) to enable cross-database linkage.

---

### Dettaglio ordine page — vodka writes

#### `SaveDataConfermaRifOrderCli`
- **SQL:** `UPDATE orders SET cdlan_dataconferma = '{{cdlan_dataconferma.formattedDate}}', cdlan_rif_ordcli = '{{cdlan_rif_ordcli.text}}', cdlan_cliente = '{{erp_an_cli.selectedOptionValue}}' WHERE id = '{{order_id.text}}';`
- **Target layer:** **BE** — `PATCH /api/ordini/:id` with fields `{cdlan_dataconferma, cdlan_rif_ordcli, cdlan_cliente (or customer_id)}`. Enforce BOZZA state + CustomerRelations role server-side.

#### `SaveActivationDate`
- **SQL:** `UPDATE orders_rows SET cdlan_data_attivazione = '{{this.params.cdlanDataAttivazione}}', confirm_data_attivazione = 1 WHERE cdlan_systemodv_row = '{{this.params.cdlanSystemodvRow}}';`
- **Target layer:** **BE** — `PATCH /api/ordini/:id/rows/:rowId/activate` with body `{activation_date}`. The endpoint must atomically set `confirm_data_attivazione=1` as a side-effect.

#### `UpdateOrderState`
- **SQL:** `UPDATE orders SET cdlan_stato = 'INVIATO', cdlan_evaso = 1 WHERE id = '{{this.params.OrderId}}';`
- **Target layer:** **BE** — this is an internal side-effect of the `POST /api/ordini/:id/send-to-erp` endpoint; no separate HTTP call from the client.

#### `SetOrderStateAttivo`
- **SQL:** `UPDATE orders SET cdlan_stato = 'ATTIVO' WHERE id = '{{this.params.OrderId}}';`
- **Target layer:** **BE** — internal side-effect of the activation-date endpoint; when the count of confirmed rows matches the total, the backend transitions the order to ATTIVO in the same transaction.

#### `CheckConfirmRows`
- **SQL:** `SELECT COUNT(id) as totale FROM orders_rows WHERE orders_id = '{{this.params.orderID}}' AND (confirm_data_attivazione=1 OR data_annullamento <> null OR cdlan_qta=0);`
- **Bug:** `data_annullamento <> null` is always false in SQL (must be `IS NOT NULL`). Verify before porting.
- **Target layer:** **BE** — internal helper, not exposed as an API.

#### `SaveOrderReferents`
- **SQL:** `UPDATE orders SET cdlan_rif_tech_nom/tel/email = …, cdlan_rif_altro_tech_* = …, cdlan_rif_adm_* = … WHERE id = '{{ref_order_id.text}}';`
- **Target layer:** **BE** — `PATCH /api/ordini/:id/referents`. Enforce state/role server-side.

#### `order_perso`
- **SQL:** `UPDATE orders SET cdlan_stato = 'PERSO' WHERE id = '{{order_id.text}}';`
- **Target layer:** **DROP** (button hidden). If the rewrite re-enables "ORDINE PERSO", it should be a backend endpoint; but clarify with the product owner whether this is in scope.

#### `upd_row_serNum`
- **SQL:** `UPDATE orders_rows SET cdlan_serialnumber = '{{this.params.cdlanSerialNumber}}' WHERE cdlan_systemodv_row = '{{this.params.cdlanSystemodv}}';`
- **Target layer:** **BE** — `PATCH /api/ordini/:id/rows/:rowId/serial-number` (BOZZA state guard).

#### `upd_row_note_tecnici`
- **SQL:** `UPDATE orders_rows SET note_tecnici = '{{this.params.noteTecnici}}' WHERE cdlan_systemodv_row = '{{this.params.idRiga}}';`
- **Target layer:** **BE** — `PATCH /api/ordini/:id/rows/:rowId/technical-notes`.

---

### Dettaglio ordine page — GW REST

#### `GW_Kickoff`
- `GET /orders/v1/kick-off/{{OrderId}}` on `gw-int.cdlan.net`. Returns a PDF body consumed by `download()`.
- **Target layer:** **BE** — proxy endpoint `GET /api/ordini/:id/kickoff.pdf`. The client downloads from the backend, not GW.

#### `GW_ActivationForm`
- `GET /orders/v1/activation-form/{{OrderId}}`. PDF with language-aware filename.
- **Target layer:** **BE** — `GET /api/ordini/:id/activation-form.pdf?lang={it|en}`.

#### `GW_SendToErp`
- `POST /orders/v1/erp`. JSON body with ~40 fields per row (`cdlan_systemodv`, `cdlan_systemodv_row`, header fields repeated, plus per-row `cdlan_codart`, `cdlan_descart`, `cdlan_qta`, `cdlan_serialnumber`, `cdlan_prezzo`, `cdlan_prezzo_attivazione`, `cdlan_prezzo_cessazione`, `cdlan_ragg_fatturazione`, `cdlan_codice_kit`, …). Hard-codes `"cdlan_stato": "CREATO"`.
- **Target layer:** **BE** — orchestrate the whole "send to ERP" flow in a single backend call: loop rows, call GW, write `UpdateOrderState`, optionally push Arxivar file. One endpoint: `POST /api/ordini/:id/send-to-erp` with multipart body containing the Arxivar PDF. Returns a per-row outcome.

#### `GW_SetActivationDate`
- `POST /orders/v1/set-order-activation`. Body `{cdlan_systemodv, cdlan_systemodv_row, cdlan_data_attivazione}` (numeric coercion on the two IDs).
- **Target layer:** **BE** — internal call from the activation-date endpoint. Not exposed to the client.

#### `GW_SavePdfToArxivar`
- `POST /orders/v1/send-to-arxivar`. Multipart: `file`, `orderId`, `filename`, `multipart` (actually the mime type).
- **Target layer:** **BE** — internal call from the send-to-ERP endpoint.

#### `GW_GetPDFArxivarOrder`
- `GET /orders/v1/order/pdf/{{orderId}}?from=vodka`. Returns base64 or raw PDF bytes.
- **Target layer:** **BE** — `GET /api/ordini/:id/signed-pdf`. The backend normalizes the payload to `application/pdf`.

#### `DownloadOrderPDFintGW`
- `GET /orders/v1/order/pdf/{{orderId}}/generate`. Returns PDF.
- **Target layer:** **BE** — `GET /api/ordini/:id/pdf`. Same normalization rules.

#### `GW_CancelOrder`
- `POST /orders/v2/order/{{order_Id}}/cancel`.
- **Bug note:** `butt_annullato` passes the wrong parameter name (`order_number` vs `order_Id`). Verify in production before porting the payload contract.
- **Target layer:** **BE** — `POST /api/ordini/:id/cancel-request`. The backend translates to the GW call.

#### `GW_SendRequestAnnullaOdv`
- `GET /{{OrderId}}` — bare path, clearly broken.
- **Target layer:** **DROP**.

---

## JSObject pseudo-actions
Several actions are exported with `ds = UNUSED_DATASOURCE`. These are Appsmith's serialization of JSObject methods already captured in the `actionCollectionList`. They add no new logic; see `page-audit.md` Dettaglio ordine section for the authoritative JSObject bodies.

The affected entries (all on `Dettaglio ordine` unless noted):
- `run` (×3), `setState`, `saveInVodka`, `saveInErp`, `checkRows`, `SetOrderStateAttivo`, `kickOff`, `activationForm`, `download`, `GetPdfOrdineArx`, `salvaRiga`, `salvaNoteTecniche`, `myFun1`.
- `myFun1`, `myFun2` on Home and Ordini semplificati pages.
