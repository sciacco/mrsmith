# Page audits — Ordini

Legend — finding bucket: **[B]** business logic · **[O]** frontend orchestration · **[P]** presentation · **[?]** open question.

---

## Home

### Purpose
Paginated list of all orders (`vodka.orders`). One row action ("Visualizza") deep-links to the dedicated `Dettaglio ordine` page. The page also hosts a full-detail **modal** (`Dettaglio_ordine`) — it duplicates the layout of the dedicated page and appears to be superseded legacy UI that no interaction path still opens.

### Widgets (58 total; only load-bearing ones listed)
| Widget | Type | Role |
|---|---|---|
| `Titolo` | TEXT | Static heading `Lista ordini`. |
| `Lista_ordini` | TABLE_V2 | Main orders table. `tableData = {{Select_Orders_Table.data}}`. 17 columns + one `iconButton` "Visualizza" row action. |
| `Nuovo_Ordine` | ICON_BUTTON | Navigates to `Form ordine`. **Hidden + disabled** (`isVisible: false`, `isDisabled: true`). Dead UI. |
| `Dettaglio_ordine` | MODAL | Legacy detail modal with a TABS widget (Canvas2–Canvas6) binding to `Dettaglio_ordine_vero.data[0]`, `Lista_righe_d_ordine.data` and `Lista_righe_d_ordine_info_tecn` — all presentation-only. No save/submit on the modal (only an ANNULLA button with no onClick and an X close). |

### Actions / queries
| Name | onLoad | Datasource | Role |
|---|---|---|---|
| `Select_Orders_Table` | **true** | vodka | Feeds the main table. Applies display mapping (CASE for `cdlan_tipo_ord`, ternary for `is_colo/service_type`, `concat(cdlan_ndoc,'/',cdlan_anno)`). |
| `Dettaglio_ordine_vero` | true (legacy) | vodka | `SELECT * FROM orders WHERE id = {{Lista_ordini.triggeredRow.id}}`. Feeds the legacy modal. Still called after `navigateTo('Dettaglio ordine')` by the row-click handler — effectively a no-op because the new page reloads the data itself. |
| `Lista_righe_d_ordine` | true | vodka | Feeds the legacy modal rows table. |
| `Lista_righe_d_ordine_info_tecn` | false | vodka | Feeds `Table2` inside the legacy modal. Bound via `{{Lista_righe_d_ordine_info_tecn.run(Lista_ordini.triggeredRow.id)}}` directly in `Table2.tableData` — calls a `.run()` in a derived-data binding (anti-pattern). |
| `Dettaglio_riga_d_ordine` | false | vodka | `SELECT * FROM orders_rows WHERE id = {{Lista_righe_d_ordine. }}` — **broken binding** (trailing `.`). Orphan. |
| `Select_orders1`, `Total_record_orders1` | false | vodka | Server-side paginated + count variant of the list, authored against `Lista_ordini.searchText/pageSize/pageOffset/sortOrder`. Table has `totalRecordsCount: 0.0` and no visible `serverSidePaginationEnabled`, so these are defined but not actually wired — dead code. |
| `Insert_orders1`, `Update_orders1` | false | vodka | Dead code — no CRUD trigger on Home and `Lista_ordini.newRow` / `.updatedRow` are never populated (table is read-only, no editable column except the legacy modal). |
| `Query1` | false | vodka | `SELECT * FROM orders ORDER BY cdlan_systemodv DESC` — orphan. |
| JS `JSObject1` | n/a | — | Empty boilerplate. |

### Event flow
- Page load → `Select_Orders_Table` runs, `Dettaglio_ordine_vero`/`Lista_righe_d_ordine` also run (they would fail or return empty because `Lista_ordini.triggeredRow.id` is undefined until a row click, but Appsmith tolerates this).
- User clicks "Visualizza" (row iconButton):
  - `navigateTo('Dettaglio ordine', {id: Lista_ordini.triggeredRow.id}, 'SAME_WINDOW')`
  - `.then(() => Dettaglio_ordine_vero.run(Lista_ordini.triggeredRow.id))` — this runs the Home-scoped query after the page has unmounted; any data it returns has no UI to bind to. Legacy/no-op.

### Bindings & hidden logic
- **[P]** `Tipo di proposta` column: `A|N|R → Sostituzione|Nuovo|Rinnovo` (inline ternary).
- **[P]** `Tipo di documento` column: `TSC-ORDINE-RIC → "Ordine ricorrente"` else `"Ordine Spot"`.
- **[P]** `Dal CP?` column: `from_cp != 0 → "Sì"` else `"No"`.
- **[P]** `Tipo di servizi` column (in `Select_Orders_Table`): `IF(is_colo != 0, is_colo, service_type)` — the column shows the colocation code when present, otherwise the comma-separated service types.
- **[B]** Inside the legacy modal, `Button2Copy` (ANNULLA) visibility: `cdlan_evaso == 1 && cdlan_stato == 'INVIATO' && from_cp == 0` — this encodes an **annullable** order rule tied to the "created from Customer Portal" flag. Same rule is duplicated and slightly reworded on the Dettaglio ordine page.
- **[B]** In the legacy modal, `Input1.isDisabled = cdlan_stato != 'BOZZA'` and `Input1Copy.isDisabled = cdlan_stato != 'BOZZA'`: PO and data conferma are only editable in BOZZA. The same rule applies on the Dettaglio page.

### Candidate domain entities
- `Order` (→ vodka `orders`). Columns used by the list: `id`, `cdlan_systemodv`, `cdlan_tipodoc`, `cdlan_ndoc`, `cdlan_anno`, `cdlan_sost_ord`, `cdlan_cliente`, `cdlan_datadoc`, `is_colo`, `service_type`, `cdlan_tipo_ord`, `cdlan_dataconferma`, `cdlan_stato`, `profile_lang`, `cdlan_evaso`, `from_cp`.

### Open questions
- **[?]** Why are `Select_orders1` / `Total_record_orders1` defined if the table isn't server-paginated? Was server-side pagination enabled and then turned off (leaving the queries as orphans), or is this the start of a migration that never completed?
- **[?]** The legacy modal duplicates the whole detail page. Did the team intend to replace it with the dedicated page and forget to delete it, or is it used in some code path we can't see?
- **[?]** `Nuovo_Ordine` is hidden+disabled. Is order creation explicitly moved to another system (Customer Portal / ERP)? The rewrite needs to confirm whether `POST /orders` is in scope at all.

### Migration notes
- The only list behaviour that must be preserved is: `Select_Orders_Table` result + the row-action "Visualizza" deep-link to the detail view.
- Drop all dead queries and the legacy modal; do not port `Insert_orders1`, `Update_orders1`, `Query1`, `Dettaglio_riga_d_ordine`, `Select_orders1`, `Total_record_orders1`, `Lista_righe_d_ordine_info_tecn` unless the rewrite specifically adds a list-level detail preview.
- The list-side display mappings (tipo documento, tipo proposta, dal CP) are presentation only — move them to the frontend formatter, not the API.

---

## Ordini semplificati

### Purpose
Prototype page bound to a HubSpot-potentials list (`loader.hubs_*`). Appears to be an unfinished second way to create orders from a HubSpot deal. Has no submit or row-action wiring.

### Widgets (8 total)
- `Table1` TABLE_V2 — `tableData = {{get_potentials.data}}`; columns `owner`, `pipeline`, `stage`, `deal_name`, `company_name`, `codice`, `id`. No row action, no selection-driven flow.
- `ButtonGroup1` BUTTON_GROUP — default Favorite / Add / More (Delete) placeholders, **no onClick handlers anywhere**.
- `Form1` FORM — `isVisible = {{utils.globals["formVisibile"]}}` and `utils.globals.formVisibile = false`, so **the form is never displayed**. Inside the form canvas are only a `Submit` button and a `Reset` button with no onSubmit / onReset. No input widgets at all.

### Actions / queries
| Name | onLoad | Datasource | Role |
|---|---|---|---|
| `get_potentials` | **true** | db-mistra | Lists HubSpot deals at certain pipeline stages (pipelines `255768766` and `255768768`, display order between 3 and 8, non-empty `codice`). |
| `get_payment_methods` | false | db-mistra | Returns selectable ERP payment methods (`loader.erp_metodi_pagamento` where `selezionabile is true`). **No widget binds to this** in the export. |
| JS `utils` | n/a | — | Holds `globals.formVisibile` only. |

### Event flow
None wired. The page only renders the read-only table.

### Bindings & hidden logic
- **[O]** `Form1.isVisible = utils.globals["formVisibile"]` — if the rewrite recreates this flow, toggling the flag is the entry point for showing the form, but no UI currently toggles it.

### Candidate domain entities
- HubSpot deal (aliased as "potentiale") — not persisted by this app.
- ERP payment method (intended but unused here).

### Open questions
- **[?]** Is this page intended to replace `Form ordine`? Both are unfinished.
- **[?]** What was the planned flow: pick a HubSpot potential → prefill the order form → insert? That never got built.

### Migration notes
- Treat as out-of-scope for a parity migration; port only if the product owner wants the "create order from HubSpot" flow.
- The `get_potentials` SQL (pipelines + stage filters) is the only piece of domain knowledge worth preserving.

---

## Draft gp da offerta

### Purpose
Placeholder. `MainContainer` only, no child widgets and no actions.

### Migration notes
Do not port. Drop page.

---

## Form ordine

### Purpose
UI-only draft for creating/editing an order. Receives `?id=` via the URL and prefills every input via `Dettaglio_ordine_vero.data[0]`. There is no submit/save action, and the only two action buttons ("Verifica numeri d'ordine", "Aggiungi riga") have no `onClick` handlers.

### Widgets (62 total)
The DSL is organized into four containers:
1. **Dati Modulo d'Ordine** — `cdlan_ndoc` (Numero proposta*), `cdlan_anno` (Anno documento*), `cdlan_datadoc` (Data proposta*), `cdlan_sost_ord` (Sostituisce proposta), `written_by` (Redatto da), `is_colo` (select), `service_type` (multi-select), `cdlan_tipo_ord` (select A/N/R), `cdlan_potential` (HubSpot code), `Button1` (no onClick), `Text1/2/3` (static copy).
2. **Termini e condizioni fornitura** — `cdlan_tipodoc` (select TSC-ORDINE-RIC/TSC-ORDINE), `cdlan_stato` (select with only `BOZZA`), `cdlan_cod_termini_pag` (select, **hard-coded list of ~30 codes**), `cdlan_dataconferma` (date), `data_decorrenza` (date), `cdlan_dur_rin` (select 1/2/3/4/6/12), `cdlan_int_fatturazione` (select 1/2/3/4/6/12), `cdlan_int_fatturazione_att` (select 1/2), `cdlan_tempi_ril` (input), `cdlan_durata_servizio` (input), `cdlan_rif_ordcli` (input), `cdlan_note` (input).
3. **Righe d'ordine** — `Table1` bound to `Lista_righe_d_ordine.data`; `Button2` "Aggiungi riga" (no onClick).
4. **Punti di contatto del cliente** — eight INPUT widgets for referente tecnico / altro / amministrativo (name/phone/email).
5. **Dati anagrafici cliente** (Container2Copy) — `cdlan_cliente`, `profile_iva`, `profile_cf`, `profile_city`, `profile_cap`, `profile_pv`, `profile_address`, `profile_lang` (select it/en).

### Actions / queries
| Name | onLoad | Datasource | Role |
|---|---|---|---|
| `Dettaglio_ordine_vero` | **true** | vodka | `SELECT * FROM orders WHERE id = {{ appsmith.URL.queryParams.id }}`. |
| `Lista_righe_d_ordine` | **true** | vodka | `SELECT … FROM orders_rows WHERE orders_id = {{ appsmith.URL.queryParams.id }}`. |

### Event flow
None functional. The form renders prefilled inputs and static text.

### Bindings & hidden logic
- **[O]** `cdlan_sost_ord.isDisabled = cdlan_tipo_ord.selectedOptionValue == 'N'`: the "Sostituisce proposta" field is disabled when the order type is "Nuovo".
- **[O]** `service_type.isDisabled = is_colo.selectedOptionValue != 0`: when the order is a colocation/IaaS flavour, the `service_type` multi-select is locked (because the colocation flag already encodes the service).
- **[B]** `cdlan_stato` dropdown has only `BOZZA` as option — new orders can only be created in BOZZA status.
- **[B]** Hard-coded lists that are effectively business enumerations embedded in the UI: payment terms (`cdlan_cod_termini_pag` — 30+ codes incl. BB/SDD/Vista fattura variants), billing frequency (`cdlan_dur_rin`, `cdlan_int_fatturazione` with `1/2/3/4/6/12` for monthly…annual), activation billing mode (`cdlan_int_fatturazione_att` 1=All'ordine, 2=All'attivazione), colocation kind (`is_colo` with four hardcoded codes), document type (TSC-ORDINE-RIC / TSC-ORDINE), order type (A/N/R).

### Candidate domain entities
- `Order` (same table as Home), `OrderProfile` (the `profile_*` columns), `OrderContacts` (the `cdlan_rif_*` columns).
- Reference tables that should be surfaced as backend enums/API calls: payment terms, billing frequencies, service types, colocation kinds.

### Open questions
- **[?]** Was a submit flow ever built, or did the team rely entirely on the Dettaglio page for edits?
- **[?]** The payment-terms list is duplicated: here it is hard-coded, on `Ordini semplificati` it would come from `get_payment_methods`. Which is canonical?

### Migration notes
- Treat this page as **reference material for field mappings and enumerations**, not as a working screen. Its only value is listing every field that a full order creation API must accept, and revealing the default-values rules.
- When rewriting the "create order" screen, move every hard-coded dropdown list to a backend endpoint (e.g., `/ordini/ref/payment-terms`, `/ordini/ref/service-types`) so they can be maintained without a frontend deploy.

---

## Dettaglio ordine

### Purpose
The real working screen. One order per page, identified by `appsmith.URL.queryParams.id`. Manages the entire order lifecycle: BOZZA → INVIATO → ATTIVO (plus PERSO / ANNULLATO branches), per-row activation, referents editing, PDF generation, and Arxivar upload.

### Widgets (92 total)
Top-level structure:
- Header bar `Container3` with four buttons: `Visualizza_odv_arx`, `Download_kickoff`, `Genera_MA`, `Scarica_PDF_button`.
- `TornaIndietro` back-button.
- `Titolo` = `"Codice ordine: {{Order.data[0].cdlan_ndoc}}/{{Order.data[0].cdlan_anno}}"`.
- `DettaglioOrdine` TABS with six tabs:
  | Tab | Content |
  |---|---|
  | **Info** | Full order header (readonly TEXT widgets). Plus a save-state subpanel (`Container1`) with `cdlan_rif_ordcli`, `cdlan_dataconferma`, `erp_an_cli` (SINGLE_SELECT_TREE fed by Alyante), a SALVA button, and an action bar (`cont_bottoni`) with `RICHIEDI ANNULLAMENTO`, `INVIA in ERP`, `ORDINE PERSO` and the Arxivar file picker. |
  | **Azienda** | Company/profile data (readonly TEXT). Contains a hidden `profile_lang` INPUT (used by `GetPdf.activationForm` to pick the filename). |
  | **Referenti** | Six INPUTs for tecnico/altro/amministrativo contacts + `SALVA` → `SaveOrderReferents`. |
  | **Righe** | `Lista_righe` TABLE bound to `RigheOrdine.data` with editActions (save serial number) and a per-row "Modifica" iconButton that opens `ModificaRiga` modal for activation-date entry. |
  | **Informazioni dai tecnici** | `Lista_righe_tecnici` TABLE bound to `RigheOrdineTecnici.data` with editActions (save note_tecnici). |
  | **Arxivar link** | Static `Text6` + `Modal1` holding an anchor `https://arxivar.cdlan.it/#!/view/…`. |
- `ModificaRiga` MODAL — activation-date form (`cdlan_data_attivazione`, `cdlan_serialnumber`, CONFERMA, CHIUDI).
- `Modal1` — Arxivar deep-link display.
- Hidden helpers: `order_id` INPUT (holds `Order.data[0].id`), `cdlan_note_bkp` INPUT (holds `Order.data[0].cdlan_note`), `ref_order_id` INPUT (also `Order.data[0].id`), `cdlan_systemodv` / `cdlan_systemodv_row` INPUTs.

### Actions / queries (vodka + Alyante + GW)
#### Reads (onLoad)
| Name | SQL/REST | Feeds |
|---|---|---|
| `Order` | `SELECT … FROM orders WHERE id = {{appsmith.URL.queryParams.id}} LIMIT 1` (vodka). Includes CASE-to-label conversions for `cdlan_int_fatturazione` / `cdlan_int_fatturazione_att`. | Every `Order.data[0].*` binding. |
| `RigheOrdine` | `SELECT … FROM orders_rows WHERE orders_id = {{appsmith.URL.queryParams.id}}` (vodka). Returns `ID Riga`, `System ODV Riga`, `Codice articolo bundle` (computed), `Codice articolo`, `Descrizione articolo`, `Canone`, `Attivazione`, `Quantità`, `Prezzo cessazione`, `Codice raggruppamento fatturazione`, `Data attivazione`, `Numero seriale`, `confirm_data_attivazione`. | `Lista_righe.tableData`. |
| `RigheOrdineTecnici` | `SELECT … FROM orders_rows WHERE orders_id = {{appsmith.URL.queryParams.id}}` (vodka). Returns `ID riga`, `codice articolo bundle`, `codice articolo`, `note tecnici` (converted UTF8), `data annullamento`. | `Lista_righe_tecnici.tableData`. |
| `erp_anagrafiche_cli` | `SELECT NUMERO_AZIENDA, RAGIONE_SOCIALE FROM Tsmi_Anagrafiche_clienti WHERE DATA_DISMISSIONE IS NULL AND RAGGRUPPAMENTO_3 <> 'Ecommerce' AND TIPOLOGIA_AZIENDA <> 'DIPENDENTE' GROUP BY …` (Alyante). | `erp_an_cli.options`. |

#### Writes (vodka)
| Name | SQL | When triggered |
|---|---|---|
| `SaveDataConfermaRifOrderCli` | `UPDATE orders SET cdlan_dataconferma=…, cdlan_rif_ordcli=…, cdlan_cliente=erp_an_cli.selectedOptionValue WHERE id=order_id.text` | `Button3` SALVA (Info tab). |
| `SaveActivationDate` | `UPDATE orders_rows SET cdlan_data_attivazione=:date, confirm_data_attivazione=1 WHERE cdlan_systemodv_row=:row` | Inside `SetActivationDate.saveInVodka`. |
| `UpdateOrderState` | `UPDATE orders SET cdlan_stato='INVIATO', cdlan_evaso=1 WHERE id=:OrderId` | Inside `SendToErp.setState`. |
| `SetOrderStateAttivo` | `UPDATE orders SET cdlan_stato='ATTIVO' WHERE id=:OrderId` | Inside `SetActivationDate.run` when all rows are confirmed. |
| `CheckConfirmRows` | `SELECT COUNT(id) AS totale FROM orders_rows WHERE orders_id=:orderID AND (confirm_data_attivazione=1 OR data_annullamento <> null OR cdlan_qta=0)` | Inside `SetActivationDate.checkRows`. **Note:** `data_annullamento <> null` is always false in standard SQL (needs `IS NOT NULL`). Likely a silent bug. |
| `SaveOrderReferents` | `UPDATE orders SET cdlan_rif_tech_*=…, cdlan_rif_altro_tech_*=…, cdlan_rif_adm_*=… WHERE id=ref_order_id.text` | `Button6` Salva (Referenti tab). |
| `order_perso` | `UPDATE orders SET cdlan_stato='PERSO' WHERE id=order_id.text` | `butt_perso` button (but it's `isVisible: false`, so orphan). |
| `upd_row_serNum` | `UPDATE orders_rows SET cdlan_serialnumber=:serial WHERE cdlan_systemodv_row=:row` | `utili.salvaRiga` on `Lista_righe` edit save. |
| `upd_row_note_tecnici` | `UPDATE orders_rows SET note_tecnici=:note WHERE cdlan_systemodv_row=:id` | `utili.salvaNoteTecniche` on `Lista_righe_tecnici` edit save. **Note:** the JSObject passes `idRiga: f["ID riga"]` which the query then matches against `cdlan_systemodv_row`. The `RigheOrdineTecnici` SQL aliases `cdlan_systemodv_row AS 'ID riga'`, so the names line up — but the binding is fragile and a bug if anyone renames the alias. |

#### REST calls (GW internal CDLAN)
| Name | Method + Path | Body / Params |
|---|---|---|
| `GW_Kickoff` | `GET /orders/v1/kick-off/:OrderId` | path param. |
| `GW_ActivationForm` | `GET /orders/v1/activation-form/:OrderId` | path param. |
| `GW_SendToErp` | `POST /orders/v1/erp` | JSON body with 40 fields including `"cdlan_stato": "CREATO"` (hard-coded by the request template; the ERP receives `CREATO` regardless of vodka's local state). |
| `GW_SetActivationDate` | `POST /orders/v1/set-order-activation` | `{cdlan_systemodv:int, cdlan_systemodv_row:int, cdlan_data_attivazione}`. |
| `GW_SavePdfToArxivar` | `POST /orders/v1/send-to-arxivar` | multipart: `file`, `orderId`, `filename`, `multipart(mime)`. |
| `GW_GetPDFArxivarOrder` | `GET /orders/v1/order/pdf/:orderId?from=vodka` | returns base64-or-raw PDF. |
| `DownloadOrderPDFintGW` | `GET /orders/v1/order/pdf/:orderId/generate` | returns base64-or-raw PDF. |
| `GW_CancelOrder` | `POST /orders/v2/order/:order_Id/cancel` | **Bug:** `butt_annullato.onClick` passes `{order_number: this.order_id.text}`, but the path template expects `{{this.params.order_Id}}` — the parameter name mismatch means the path becomes `/orders/v2/order//cancel`. Needs verification against the live app. |
| `GW_SendRequestAnnullaOdv` | `GET /:OrderId` (path is bare) | Orphan; no visible caller except `SendRequestAnnullaOdv` JSObject which is not invoked by any widget. |

### Event flow
1. Page load: `Order`, `RigheOrdine`, `RigheOrdineTecnici`, `erp_anagrafiche_cli` all run in parallel.
2. **Edit + SALVA (Info tab)** — `Button3.onClick = SaveDataConfermaRifOrderCli.run().then(() => Order.run())`. Persists PO/data conferma/ragione sociale back into vodka and refreshes.
3. **INVIA in ERP (Info tab, cont_bottoni)** — `SendToErp.run()` then `Order.run()`. The JSObject:
   - Loops every row in `RigheOrdine.data`, calling `GW_SendToErp` for each. Each row becomes one ERP document line.
   - Collects failures into an `err` flag (1 on first error). Continues the loop despite failures.
   - If `err == 0`: calls `UpdateOrderState` (→ INVIATO, cdlan_evaso=1). If the Arxivar file picker has a file, calls `GW_SavePdfToArxivar`. Alerts success and `navigateTo('Home')`.
4. **RICHIEDI ANNULLAMENTO** — `butt_annullato.onClick = GW_CancelOrder.run({order_number: order_id.text})`. See parameter-mismatch bug above.
5. **ORDINE PERSO** — `butt_perso.onClick = order_perso.run().then(() => Order.run())`. Hidden (`isVisible: false`), inert.
6. **Upload Arxivar** — `arxivar` file picker; the actual upload happens inside `SendToErp.run` (not on file-select).
7. **Referenti tab SALVA** — `SaveOrderReferents.run()`, alert success/error.
8. **Righe tab row edit**:
   - `EditActions1.onSave = utili.salvaRiga(); storeValue('tab', 'Righe')` — saves only the `Numero seriale` value (the only column editable inline).
   - Per-row "Modifica" iconButton (only when `cdlan_stato == 'INVIATO' && user in CustomerRelations`) → opens `ModificaRiga` modal → user picks `cdlan_data_attivazione` → CONFERMA → `SetActivationDate.run()`:
     - `saveInVodka` (UPDATE `orders_rows`).
     - `saveInErp` (GW POST).
     - `checkRows` (COUNT).
     - `Promise.all([…]).then(…)` — if `totale == countAllRows`, auto-transition the order to ATTIVO via `SetOrderStateAttivo`. Always shows success alert and closes the modal.
9. **Informazioni dai tecnici tab row edit** — `EditActions1.onSave = utili.salvaNoteTecniche(); storeValue('tab', 'Informazioni dai tecnici')` — saves `note_tecnici`.
10. **PDF buttons** — `Download_kickoff` → `GetPdf.kickOff` → GW → `download(response, …)`. `Genera_MA` → `GetPdf.activationForm` (filename language-aware). `Scarica_PDF_button` → `OrderTools.download` → `DownloadOrderPDFintGW` (base64 decode and blob download). `Visualizza_odv_arx` → `GetPdfOrdineArx.GetPdfOrdineArx(Order.data[0].id)` → `GW_GetPDFArxivarOrder` (base64 decode and blob download).

### Bindings & hidden logic (classified)
#### Business rules — state machine & permissions
- **[B]** `Button3` (SALVA dati conferma) visible only if `cdlan_stato == 'BOZZA' && user ∈ CustomerRelations`.
- **[B]** `invia` (INVIA in ERP) enabled only when: `cdlan_dataconferma` set, `erp_an_cli` selected, `arxivar.files.length > 0`, `cdlan_stato == 'BOZZA'`.
- **[B]** `butt_annullato` enabled only when `cdlan_stato == 'INVIATO' && user ∈ CustomerRelations`.
- **[B]** `Download_kickoff` enabled only when `cdlan_stato == 'INVIATO' && user ∈ CustomerRelations`.
- **[B]** `Genera_MA` enabled only when `cdlan_stato ∈ {'ATTIVO','INVIATO'} && user ∈ CustomerRelations`.
- **[B]** `Scarica_PDF_button` enabled only when `arx_doc_number != null` is **false** (the binding is `isDisabled: Order.data[0].arx_doc_number != null`, so it's **enabled** when there is no Arxivar doc number — i.e., pre-Arxivar PDF generation).
- **[B]** `Visualizza_odv_arx` enabled only when `arx_doc_number != null` (inverse of above).
- **[B]** `cdlan_rif_ordcli`, `cdlan_dataconferma`, `erp_an_cli` editable only when `cdlan_stato == 'BOZZA' && user ∈ CustomerRelations`.
- **[B]** `cdlan_data_attivazione` (modal) disabled when `cdlan_stato == 'ATTIVO'` (the modal itself is only opened when state is INVIATO, so this is defence-in-depth).
- **[B]** `cdlan_serialnumber` editable only when `cdlan_stato == 'BOZZA'`.
- **[B]** `BTN_confirm_act_modal` visible when `cdlan_stato != 'ATTIVO'` and enabled when `cdlan_data_attivazione.value != null`.
- **[B]** `Lista_righe.customColumn1` (per-row Modifica) visible only when `cdlan_stato == 'INVIATO' && user ∈ CustomerRelations`.
- **[B]** `arxivar` file picker disabled when `arx_doc_number != null` or (state ∈ {'ANNULLATO','PERSO','ATTIVO'} combined with user not in CustomerRelations). **Note:** the current binding `(A != null) || (B || C || D) && !inGroup` has operator precedence issues and evaluates differently from what the comment suggests; candidate bug.
- **[B]** `Button6` (Referenti SALVA) enabled only when `cdlan_stato ∈ {BOZZA, INVIATO} && user ∈ CustomerRelations`.
- **[B]** Auto-transition to ATTIVO: the `CheckConfirmRows` COUNT vs `RigheOrdine.data.length` equality in `SetActivationDate.run`. This is the **only** path to ATTIVO in the UI.
- **[B]** Annullamento precondition used only on the Home modal (not here): `cdlan_evaso == 1 && cdlan_stato == 'INVIATO' && from_cp == 0`. Apparently, orders originating from the Customer Portal (`from_cp != 0`) cannot be cancelled from this app.

#### Orchestration rules
- **[O]** Every GW call is launched without explicit retry; failures display an alert and continue.
- **[O]** `SendToErp.run` uses `Promise.all` but with already-awaited values — bug-adjacent but harmless (just becomes sequential then the `.then` waits on settled values).
- **[O]** `SetActivationDate.run` does `await saveInVodka; await saveInErp; await checkRows;` and then still wraps them in `Promise.all` — equivalent to the awaited values. The success-alert runs whenever all three are truthy, including when the HTTP call silently fails because the error is swallowed into `return false`.
- **[O]** Base64-or-raw PDF decoding (`GetPdfOrdineArx`, `OrderTools.download`) contains an inline heuristic to detect if the payload is base64 vs raw binary. This logic is duplicated in two JSObjects.
- **[O]** `storeValue('tab', 'Righe')` and `storeValue('tab', 'Informazioni dai tecnici')` are called on save, but no widget reads that store value — vestigial.
- **[O]** Hidden helper widgets (`order_id`, `ref_order_id`, `cdlan_note_bkp`, `cdlan_systemodv`, `cdlan_systemodv_row`, `cdlan_ndoc`, `cdlan_anno`, `profile_lang`) exist to make single fields addressable as `.text` by JSObjects. A rewrite should pass these values as function arguments directly.

#### Presentation rules
- **[P]** Throughout the Info tab, many TEXT widgets carry both a label + a bold key (`<b>Durata rinnovo:</b> {{...}}`) with inline ternaries that map codes to Italian labels (e.g., `cdlan_dur_rin: 1→"Mensile"`, `tacito_rin: 1→"Sì"`, etc.).
- **[P]** `cdlan_dataconferma` default display uses `moment(Order.data[0].cdlan_datadoc).format(...)`.
- **[P]** Filename of activation-form PDF is localized IT/EN based on `profile_lang.text`.
- **[P]** `Data attivazione` in `Lista_righe` is formatted via `moment(...).format("DD/MM/YYYY")` or `"-"`.

### Candidate domain entities (from this page)
- `Order` (vodka `orders`) — full field list matches the Home query plus `cdlan_int_fatturazione_desc`/`cdlan_int_fatturazione_att_desc` (CASE-computed display aliases).
- `OrderRow` (vodka `orders_rows`) — `id`, `cdlan_systemodv_row`, `cdlan_codart`, `cdlan_descart`, `cdlan_prezzo`, `cdlan_prezzo_attivazione`, `cdlan_qta`, `cdlan_prezzo_cessazione`, `cdlan_ragg_fatturazione`, `cdlan_data_attivazione`, `cdlan_serialnumber`, `cdlan_codice_kit`, `index_kit`, `note_tecnici`, `data_annullamento`, `confirm_data_attivazione`.
- `Customer` (Alyante `Tsmi_Anagrafiche_clienti`) — via `NUMERO_AZIENDA`/`RAGIONE_SOCIALE`; only used for the Ragione sociale dropdown, and only the display string is persisted.
- `ArxivarDocument` — referenced via `arx_doc_number` and the deep-link `https://arxivar.cdlan.it/#!/view/<UUID>/…`.

### Open questions
- **[?]** `GW_CancelOrder` receives `order_number` but the URL expects `order_Id`. Either the live binding differs from the exported JSON, or annullamento is silently broken. Verify with prod traffic.
- **[?]** `GW_SendRequestAnnullaOdv` (path `/:OrderId`) is clearly wrong. Is it actually dead code, or is there an alternate annullamento flow we are missing?
- **[?]** `CheckConfirmRows` uses `data_annullamento <> null`, which is never true in MySQL. Confirm whether "annullata" rows should be counted as "confirmed" for the ATTIVO transition; if yes, fix to `IS NOT NULL`.
- **[?]** `butt_perso` is hidden. How do orders become PERSO today? Manually in the DB, or from another app?
- **[?]** `cdlan_stato = 'CREATO'` is hard-coded in `GW_SendToErp` body while vodka sets `cdlan_stato = 'INVIATO'` after the call — confirm that the ERP-side state and the vodka-side state are expected to diverge.
- **[?]** Why does `SendToErp.run` swallow row-level errors (sets `err=1` but keeps looping), and then commit the state transition anyway only when `err == 0`? A single row failure aborts the state transition but leaves the other rows written to ERP — leaving both systems in a partially-committed state.

### Migration notes
- Re-implement the state machine and permission rules on the backend (Go handlers), expressed as:
  - Keycloak role `app_ordini_access` for general access.
  - Sub-role or group that maps to the current `CustomerRelations` group for the edit/ERP-trigger actions.
- Replace the per-row `SendToErp` loop with a single backend endpoint that transactionally sends to ERP, writes state to `vodka`, and uploads to Arxivar. This collapses three failure modes into one.
- Move the base64-or-raw PDF payload normalization to the backend so the frontend only consumes `application/pdf`.
- Do not copy the `Promise.all([])` + already-awaited-values pattern; just sequence the awaits.
- Replace all widget-scoped hidden helpers (`order_id.text`, `cdlan_ndoc.text`, etc.) with explicit function arguments passed through a service layer.
- The editable `Lista_righe` columns are: `Numero seriale` (only). Everything else is read-only. Confirm this is still the desired scope.
- Reference lookups (payment terms, billing frequencies, colocation kinds, service types, document types, order types) must become backend enums/endpoints — never hard-coded in the React code.
