# Ordini — Appsmith ↔ MrSmith parity report

Source of truth: `apps/ordini/Ordini.json.gz` (decompressed to `artifacts/claude/Ordini.json` for this audit, not committed). Supporting docs in `apps/ordini/audit/*` were used only as orientation; classifications below are anchored on the JSON when the two disagreed.

MrSmith target: `backend/internal/ordini/**` + `apps/ordini/src/**`.

Plan / deviation register: `apps/ordini/docs/IMPL-ORDINI.md` §1 (scope), §8 (role/state matrix), §9 (revised business rules), §10–13 (workflows), §21 (deferred).

## Summary

- Total Appsmith items audited: **103** (5 pages, 5 datasources, 55 actions/JS handlers, 38 interactive/data widgets with semantic gates)
- parity_confirmed: **78**
- intentional_deviation: **17**
- gap_blocking: **0**
- gap_minor: **7**
- cannot_verify: **1**
- **Go/no-go recommendation:** **GO** — no blocking gap; the 7 minor gaps are display-only readonly fields whose underlying data is already exposed by `/api/ordini/v1/orders/{id}`, so they can be surfaced as a follow-up without backend changes.

---

## Gaps — blocking

None.

---

## Gaps — minor

### G1. `cdlan_note` (Note legali) not rendered

- **Appsmith location:** `Dettaglio ordine` text widget `cdlan_note` — `<b>Note Legali: </b>{{Order.data[0].cdlan_note == null ? "—" : Order.data[0].cdlan_note}}`.
- **MrSmith location:** field exists at `backend/internal/ordini/types.go:43` and `apps/ordini/src/api/types.ts:35` but is not displayed in any tab (`apps/ordini/src/components/InfoTab.tsx:62-75`, `AziendaTab.tsx`).
- **Drift:** operators reading note legali on the legacy detail page see "—" / no field on MrSmith.
- **Suggested resolution:** add a `Field label="Note legali"` in `InfoTab` (or a dedicated Note tab) using `order.cdlan_note`.

### G2. `cdlan_tacito_rin` (Tacito rinnovo) not rendered

- **Appsmith location:** `Dettaglio ordine` text widget `cdlan_tacito_rin` — `<b>Tacito rinnovo: </b>{{Order.data[0].cdlan_tacito_rin}}`.
- **MrSmith location:** field in `types.go:45` / `types.ts:37`, no UI surface in `InfoTab.tsx:62-75`.
- **Drift:** "Durata rinnovo" is shown, but the separate Tacito rinnovo flag is missing.
- **Suggested resolution:** add a `Field` in InfoTab (formatter probably `formatSiNo` if value is 0/1, else raw). The legacy text rendered the raw value — verify in DB whether `cdlan_tacito_rin` holds `0/1`, `S/N`, or months count before picking the format.

### G3. `cdlan_cod_termini_pag` (Condizioni di pagamento) not rendered

- **Appsmith location:** `Dettaglio ordine` text widgets `cdlan_cod_termini_pag` and `Text4Copy1CopyCopy` (Home modal) — `<b>Condizioni di pagamento:</b> {{Order.data[0].cdlan_cod_termini_pag}}`.
- **MrSmith location:** field in `types.go:42` / `types.ts:34`, no UI surface.
- **Suggested resolution:** add a `Field` in `InfoTab`. Note that `origin_cod_termini_pag` is also exposed by the backend — clarify with operators whether they want one or both.

### G4. `written_by` (Redatto da) not rendered

- **Appsmith location:** `Dettaglio ordine` text widget `written_by` — `<b>Redatto da:</b> {{Order.data[0].written_by}}`.
- **MrSmith location:** field in `types.go:62` / `types.ts:54`, no UI surface.
- **Suggested resolution:** add a `Field` in `InfoTab` (or `AziendaTab`) using `order.written_by`.

### G5. `cdlan_ragg_fatturazione` (Codice raggruppamento fatturazione) not in righe table

- **Appsmith location:** Home `Lista_righe` column visible (`Codice raggruppamento fatturazione`); on `Dettaglio ordine` the column is `isVisible: False` so users cannot toggle it on either. The Home modal showed it.
- **MrSmith location:** column not rendered in `apps/ordini/src/components/RigheTab.tsx:55-67`; field present in `OrderRow.cdlan_ragg_fatturazione` (`types.go:97` / `types.ts:82`).
- **Drift:** since the column was hidden on the Appsmith Dettaglio ordine but visible on the Home modal, the bar of legacy behaviour is low. Operators relying on the Home modal would lose it; operators using only the Detail page already could not see it.
- **Suggested resolution:** optional — add a hideable column to `RigheTab` if the field is operationally useful.

### G6. Existing activation date not shown in activation modal

- **Appsmith location:** `Dettaglio ordine` modal `ModificaRiga`, text widget `TXT_data_atti_tech` — `Data indicata dai tecnici: {{Lista_righe.selectedRow.Data attivazione}}` (the tech-provided date already on the row).
- **MrSmith location:** `apps/ordini/src/components/ActivationModal.tsx:30-46` shows row code + descrizione but does NOT show the current `row.cdlan_data_attivazione` for reference. The same value is visible in the row of `RigheTab` behind the modal, so it is not lost.
- **Drift:** small UX convenience — operators no longer see "this is what the techs wrote" inside the modal; they have to glance at the row.
- **Suggested resolution:** add a read-only line in `ActivationModal` showing `formatDate(row.cdlan_data_attivazione)` above the date input.

### G7. `cdlan_int_fatturazione` catch-all label changed

- **Appsmith location:** `Dettaglio ordine` action `Order` (`artifacts/claude/Ordini.json` → `actionList` entry `Order`) — `CASE cdlan_int_fatturazione WHEN '1' THEN 'Mensile' ... ELSE 'Annuale' END`. Any unmapped value falls through to **'Annuale'**.
- **MrSmith location:** `apps/ordini/src/lib/formatters.ts:71-89` — explicit `1/2/3/(4|5)/6/12` and otherwise returns `formatEmpty(value)` (i.e. `'—'`).
- **Drift:** if rows hold values outside `{1,2,3,4,5,6,12}` (e.g. legacy `0` or NULL), MrSmith will display `'—'` while Appsmith displayed `'Annuale'`. The §9 Q3 rewrite is documented for `4↔5`; the broader catch-all change is a side-effect not explicitly called out.
- **Suggested resolution:** either (a) leave as-is (MrSmith is more honest about unmapped values), or (b) match legacy by defaulting unmapped non-empty values to `'Annuale'`. Recommend confirming with operators which is preferred.

---

## Cannot verify

### CV1. Arxivar upload payload contract

- **Appsmith call:** `GW_SavePdfToArxivar` (`actionList` entry, `Dettaglio ordine` page) — `POST /orders/v1/send-to-arxivar` with JSON body `{ "file": "<base64>", "orderId": <id> }` (the Appsmith file picker emits base64; the body header was JSON, not multipart).
- **MrSmith call:** `backend/internal/ordini/gateway.go:73-109` `gatewayUploadToArxivar` — `POST /orders/v1/send-to-arxivar` as **multipart/form-data** with parts `file` (raw bytes, not base64), `orderId`, `filename`, `multipart=application/pdf`.
- **What's needed to resolve:** confirm the GW endpoint accepts multipart (and the extra `filename` / `multipart` fields are ignored or required). This cannot be checked from the repo alone — the GW spec lives outside this audit's scope. The full GW request can be exercised end-to-end in staging; the rest of the workflow is wired correctly.

---

## Intentional deviations — verified

All citations below were verified against current code; each implementation matches the documented intent.

| # | Appsmith behavior | IMPL-ORDINI anchor | MrSmith verification |
|---|---|---|---|
| D1 | `data_annullamento <> null` in `CheckConfirmRows` | §9 Q2 | `backend/internal/ordini/workflow_activate.go:94-101` uses `data_annullamento IS NOT NULL` (and `canActivateOrderRow` at `workflow_activate.go:128-136` rejects cancelled / qty=0 rows up-front). |
| D2 | `cdlan_int_fatturazione = 5 → 'Quadrimestrale'`, drift vs `4` | §9 Q3 | `apps/ordini/src/lib/formatters.ts:79-81` maps both `'4'` and `'5'` to `'Quadrimestrale'`; no DB migration. |
| D3 | Column alias `Attivazione` / `Prezzo attivazione` | §9 Q4 | DTO `activation_price` in `backend/internal/ordini/types.go:95`; UI label `Prezzo attivazione` in `apps/ordini/src/components/RigheTab.tsx:62`. |
| D4 | Client-side `appsmith.user.groups.includes('CustomerRelations')` | §9 Q5 | Keycloak role `app_customer_relations` declared in `backend/internal/platform/applaunch/catalog.go:91`, enforced backend at `backend/internal/ordini/permissions.go:12-26`, mirrored frontend at `apps/ordini/src/lib/permissions.ts:1-8`. |
| D5 | Arxivar file picker OR-chain (always-true bug) | §9 Q6 | `apps/ordini/src/lib/permissions.ts:34-37` and `backend/internal/ordini/permissions.go:38-41` enforce `state NOT IN {ANNULLATO,PERSO,ATTIVO} AND CR`. |
| D6 | Partial-failure ERP silently noisy | §9 Q8 / C1 | Per-row outcome in `backend/internal/ordini/types.go:154-166`, computed at `workflow_send.go:61-77`, surfaced in UI at `apps/ordini/src/components/SendToErpResultPanel.tsx`. |
| D7 | Header SALVA only writes `cdlan_cliente` (string), never `cdlan_cliente_id` | §9 C2 / §12 | `backend/internal/ordini/store_orders.go:259-274` looks up the customer by `cdlan_cliente_id` (`NUMERO_AZIENDA`) via Alyante (`store_customers.go:21-27`) and writes both `cdlan_cliente_id` and `cdlan_cliente` from the trusted Alyante name. |
| D8 | `cdlan_stato` sent to ERP equals the current local state | §9 ERP state | `backend/internal/ordini/gateway.go:149` hard-codes `"cdlan_stato": "CREATO"` in the GW payload. |
| D9 | `arx_doc_number` write path invisible / unowned | §9 Arxivar doc number | Ordini never writes `arx_doc_number` — `grep` of `backend/internal/ordini` confirms no `UPDATE orders ... arx_doc_number`. |
| D10 | `RICHIEDI ANNULLAMENTO` button + `GW_CancelOrder`/`GW_SendRequestAnnullaOdv` | §1 out-of-scope + §21 | No equivalent button or backend route. The two GW actions are absent from `backend/internal/ordini/gateway.go`. |
| D11 | `ORDINE PERSO` button + `order_perso` UPDATE | §1 out-of-scope + §21 | Appsmith button already has `isVisible: false` in the export; MrSmith has no equivalent button or backend write. |
| D12 | Order creation form (`Form ordine` page) + `Insert_orders1` | §1 out-of-scope | No create route in `apps/ordini/src/routes.tsx` (only `/ordini` and `/ordini/:id`); no `INSERT INTO orders` in `backend/internal/ordini/store_orders.go`. |
| D13 | Generic row update (`Update_orders1`) keyed off `Lista_ordini.updatedRow` | §1 out-of-scope | No full-row PATCH; only the narrow PATCH endpoints listed in §5 are implemented. |
| D14 | `Select_orders1` / `Total_record_orders1` server-side pagination | §21 (server-side pagination deferred) | `backend/internal/ordini/store_orders.go:36-54` returns the full list; pagination is client-side in `apps/ordini/src/pages/OrderListPage.tsx:11,50-52`. |
| D15 | Inline edit of `note_tecnici` gated by `INVIATO + CR` | §8 role/state matrix (`note_tecnici` row: `sì / sì / any`) and §15 ("Disponibile a ogni `app_ordini_access`") | `backend/internal/ordini/store_rows.go:228-266` requires neither CR nor a specific state; `apps/ordini/src/components/TechnicalNotesTab.tsx` has no gating beyond row presence. |
| D16 | Inline edit of `cdlan_serialnumber` gated by `BOZZA + CR` | §8 (`cdlan_serialnumber` row: `sì / sì / BOZZA`) | `backend/internal/ordini/store_rows.go:176-226` requires only `BOZZA` (no CR check); `apps/ordini/src/lib/permissions.ts:26-28` mirrors that. |
| D17 | Arxivar PDF upload was conditional on `arxivar.files.length > 0` (operator could skip) | §10 ("PDF presente e valido" as server-side precondition) | `backend/internal/ordini/workflow_send.go:47-50,104-126` requires a valid `%PDF` multipart part, returning `missing_pdf` / `invalid_pdf` otherwise. Operators can no longer fire `Invia in ERP` without uploading. |

---

## Per-page inventory

Classification key: **OK** = parity_confirmed; **DEV** = intentional_deviation; **GAPm** = gap_minor; **GAPb** = gap_blocking; **CV** = cannot_verify.

### Page: `Home` (Appsmith)

MrSmith counterpart: `apps/ordini/src/pages/OrderListPage.tsx` + `apps/ordini/src/components/OrdersTable.tsx`.

| Appsmith item | Type | MrSmith location | Class |
|---|---|---|---|
| `Select_Orders_Table` action (main list query) | SQL onPageLoad | `backend/internal/ordini/store_orders.go:36-54` + `apps/ordini/src/api/queries.ts:15-21` | OK |
| `Select_orders1` action (paginated list) | SQL | n/a — replaced by full-list + client-side pagination | DEV (D14) |
| `Total_record_orders1` action (count for pagination) | SQL | n/a | DEV (D14) |
| `Insert_orders1` action (creation) | SQL | n/a | DEV (D12) |
| `Update_orders1` action (generic row update) | SQL | n/a | DEV (D13) |
| `Dettaglio_ordine_vero` action (modal preview) | SQL onPageLoad | superseded by detail page nav | OK (functionality moves to `/ordini/:id`) |
| `Lista_righe_d_ordine` action (modal preview rows) | SQL onPageLoad | superseded by detail page | OK |
| `Lista_righe_d_ordine_info_tecn` action (modal tech rows) | SQL | superseded by detail page | OK |
| `Dettaglio_riga_d_ordine` action | SQL with broken `WHERE id = {{Lista_righe_d_ordine. }}` | n/a (dead code in Appsmith — invalid template literal) | OK (orphan) |
| `Query1` action (`SELECT * FROM orders`) | SQL | n/a | OK (orphan / unused) |
| `JSObject1.myFun1` / `myFun2` | JS stubs | n/a | OK (empty) |
| `Lista_ordini` table tableData | TABLE_WIDGET_V2 | `OrdersTable.tsx:18-72` | OK |
| Column `Codice ordine` (`concat(ndoc,'/',anno)`) | text | `OrdersTable.tsx:24,48` via `formatters.ts:131-136 orderCode()` | OK |
| Column `Numero proposta` (hidden) | text | `OrderSummary.cdlan_ndoc` exposed, surfaced inside `orderCode()` | OK |
| Column `Anno documento` (hidden) | text | `OrderSummary.cdlan_anno` exposed, surfaced inside `orderCode()` | OK |
| Column `System ODV` (hidden) | number | `OrdersTable.tsx:25,50` (visible "ODV") | OK |
| Column `Tipo di documento` | text | `OrdersTable.tsx:29,54` + `formatTipoDoc` (renames raw `TSC-ORDINE` → "Ordine spot") | OK (label rewrite is copy choice) |
| Column `Tipo di proposta` | text (SQL CASE A/N/R) | `OrdersTable.tsx:30,55` + `formatTipoProposta` | OK |
| Column `Tipo di servizi` (`IF(is_colo!=0, is_colo, service_type)`) | text | `OrdersTable.tsx:31,56` + `formatServiceTypes` | OK |
| Column `Sostituisce ordini` | text | `OrdersTable.tsx:35,60` | OK |
| Column `Ragione sociale` | text | `OrdersTable.tsx:26,51` | OK |
| Column `Data proposta` | date | `OrdersTable.tsx:28,53` | OK |
| Column `Data conferma` | date | `OrdersTable.tsx:32,57` | OK |
| Column `Stato` | text | `OrdersTable.tsx:27,52` + `StatusBadge` | OK |
| Column `Lingua` | text | `OrdersTable.tsx:36,61` | OK |
| Column `cdlan_evaso` (hidden) | number | `OrdersTable.tsx:33,58` (visible "Evaso") | OK |
| Column `Dal CP?` (`IF from_cp != 0`) | number→Sì/No | `OrdersTable.tsx:34,59` + `formatSiNo` | OK |
| Column `id` (hidden) | number | exposed in `OrderSummary.id`, used for nav | OK |
| Column custom `Visualizza` → `navigateTo('Dettaglio ordine', {id})` | iconButton | `OrdersTable.tsx:38-41,63-67` "Apri" button + double-click row → `navigate('/ordini/:id')` | OK |
| (extra in MrSmith) Column `Doc.` (`arx_doc_number`) | n/a | `OrdersTable.tsx:37,62` | OK (additive) |
| `Lista_ordini.onRowSelected` (empty `{{}}`) | handler | no-op preserved (Apri button is the explicit nav) | OK |
| Toolbar search `Lista_ordini.searchText` (server-side WHERE LIKE) | implicit | `OrderListPage.tsx:16,30-47` client-side filter on code/cliente/sost/ODV/servizi | DEV (D14) |
| `Lista_ordini.sortOrder` (server-side ORDER BY) | implicit | `OrderListPage.tsx:55-64,137-155` client-side sort (id/code/customer/date/state) | DEV (D14) |
| `Lista_ordini.pageSize` / `pageOffset` (server-side paging) | implicit | `OrderListPage.tsx:11,50-52,119-123` client-side pager (50/page) | DEV (D14) |
| `Nuovo_Ordine` icon button (`isVisible: false`, `isDisabled: true`) | ICON_BUTTON_WIDGET | absent | DEV (D12) |
| Home inline detail modal (Text widgets for Data conferma, Stato, Ragione sociale, Tipo doc, Commerciale, Sostituisce, etc.) | TEXT_WIDGETs | superseded by `/ordini/:id` (detail page renders all the same fields) | OK |
| Home modal `Input1Copy` (Data conferma) `DIS={{stato!='BOZZA'}}` | INPUT | superseded; edit happens on detail page (`InfoTab.tsx`) gated by `canEditBozzaHeader` | OK |
| Home modal `Input1` (Riferimento PO Cliente) `DIS={{stato!='BOZZA'}}` | INPUT | superseded; edit on detail page | OK |
| Home modal `Button2Copy` "ANNULLA" `VIS={{cdlan_evaso==1 && stato=='BOZZA'}}`, no `onClick` | BUTTON | absent (button had no handler — dead in Appsmith) | OK |
| Home modal `Lista_righe` table (rows preview) | TABLE | superseded by `RigheTab` on detail page | OK |
| Home modal `Table2` (technical rows preview) with `tableData={{Lista_righe_d_ordine_info_tecn.run(...)}}` | TABLE | superseded by `TechnicalNotesTab` | OK |

### Page: `Ordini semplificati` (Appsmith)

This page is essentially a half-built order-creation form referencing HubSpot deals. MrSmith does not include order creation (§1 out-of-scope), so the whole page falls under D12.

| Appsmith item | Type | MrSmith counterpart | Class |
|---|---|---|---|
| `get_potentials` action (HubSpot deal list from Mistra `loader.hubs_*`) | SQL onPageLoad | n/a | DEV (D12) |
| `get_payment_methods` action (Alyante metodi pagamento) | SQL | n/a | DEV (D12) |
| `utils.globals.formVisibile` JS object | JS stub | n/a | OK (dead) |
| `Table1` widget (HubSpot deals) | TABLE | n/a | DEV (D12) |
| `Button1` / `Button2` (Submit / Reset) | BUTTON | n/a | DEV (D12) |
| `Text1` "Form" | TEXT | n/a | DEV (D12) |

### Page: `Draft gp da offerta` (Appsmith)

Empty page (single canvas, no widgets, no actions). No equivalent needed.

### Page: `Form ordine` (Appsmith)

Order creation form pre-populated from URL `?id`. Whole page falls under D12.

| Appsmith item | Type | MrSmith counterpart | Class |
|---|---|---|---|
| `Dettaglio_ordine_vero` action | SQL onPageLoad | n/a | DEV (D12) |
| `Lista_righe_d_ordine` action | SQL onPageLoad | n/a | DEV (D12) |
| Header inputs: `cdlan_systemodv`, `cdlan_tipodoc`, `cdlan_ndoc`, `cdlan_anno`, `cdlan_datadoc`, `cdlan_cliente`, `cdlan_dataconferma`, `cdlan_rif_ordcli`, `cdlan_tipo_ord`, `cdlan_sost_ord` `DIS={{cdlan_tipo_ord=='N'}}`, `cdlan_dur_rin`, `cdlan_tempi_ril`, `cdlan_durata_servizio`, `cdlan_int_fatturazione`, `cdlan_int_fatturazione_att`, `cdlan_note`, `cdlan_stato`, `cdlan_cod_termini_pag`, `data_decorrenza`, `is_colo`, `service_type` `DIS={{is_colo!=0}}`, `written_by`, `cdlan_potential` | INPUT/SELECT | n/a | DEV (D12) |
| Profile inputs: `profile_iva`, `profile_cf`, `profile_address`, `profile_city`, `profile_cap`, `profile_pv`, `profile_lang` | INPUT | n/a | DEV (D12) |
| Referenti inputs (9 fields) | INPUT | n/a | DEV (D12) |
| `Table1` righe d'ordine + `Button2` "Aggiungi riga", `Button1` "Verifica numeri d'ordine" | TABLE/BUTTON | n/a | DEV (D12) |

### Page: `Dettaglio ordine` (Appsmith)

MrSmith counterpart: `apps/ordini/src/pages/OrderDetailPage.tsx` + tab components.

**Read actions:**

| Appsmith item | Type | MrSmith location | Class |
|---|---|---|---|
| `Order` action — `SELECT id, … FROM orders WHERE id = {{appsmith.URL.queryParams.id}}` (with SQL CASE for `cdlan_int_fatturazione` and `cdlan_int_fatturazione_att`) | SQL onPageLoad | `store_orders.go:80-192` `getOrder` / `getOrderWithoutOrigin` returns full DTO; label CASEs moved to frontend formatters (`formatters.ts:71-100`). | OK |
| `RigheOrdine` action — order rows incl. `id AS 'ID Riga'`, `confirm_data_attivazione` | SQL onPageLoad | `store_rows.go:11-50` `listOrderRows` returns equivalent DTO (`OrderRow`). | OK |
| `RigheOrdineTecnici` action — `CONVERT(note_tecnici USING UTF8)` | SQL onPageLoad | `store_rows.go:85-111` `listTechnicalRows` preserves the `CONVERT … USING UTF8`. | OK |
| `erp_anagrafiche_cli` action — Alyante customer dropdown | SQL onPageLoad (Alyante) | `store_customers.go:12-49` + `/ordini/v1/ref/customers`; same WHERE clause incl. `DATA_DISMISSIONE IS NULL`, `RAGGRUPPAMENTO_3 <> 'Ecommerce'`, `TIPOLOGIA_AZIENDA <> 'DIPENDENTE'`. | OK |
| `CheckConfirmRows` action — `COUNT … WHERE confirm_data_attivazione=1 OR data_annullamento <> null OR cdlan_qta=0` | SQL | `workflow_activate.go:94-101` — same shape but Q2 fix (`IS NOT NULL`). | OK + DEV (D1) |
| `Dettaglio_riga_d_ordine` action | SQL | n/a | OK (orphan — broken template) |
| `JSObject1.myFun1` (console.log) | JS | n/a | OK (debug stub) |

**Write actions:**

| Appsmith item | Type | MrSmith location | Class |
|---|---|---|---|
| `SaveDataConfermaRifOrderCli` — UPDATE `cdlan_dataconferma`, `cdlan_rif_ordcli`, `cdlan_cliente` | SQL | `store_orders.go:226-284` `handlePatchOrderHeader` — also writes `cdlan_cliente_id` and re-fetches name from Alyante. | OK + DEV (D7) |
| `SaveOrderReferents` — UPDATE 9 referent fields | SQL | `store_orders.go:286-332` `handlePatchReferents`. | OK |
| `SaveActivationDate` — UPDATE `cdlan_data_attivazione`, `confirm_data_attivazione=1` | SQL | `workflow_activate.go:76-79` (same SQL, transactional). | OK |
| `UpdateOrderState` — UPDATE `cdlan_stato='INVIATO', cdlan_evaso=1` | SQL | `workflow_send.go:79-90` (with `AND cdlan_stato='BOZZA'` guard). | OK |
| `SetOrderStateAttivo` — UPDATE `cdlan_stato='ATTIVO'` | SQL | `workflow_activate.go:103-110` (with `AND cdlan_stato='INVIATO'` guard). | OK |
| `upd_row_serNum` — UPDATE `cdlan_serialnumber` | SQL | `store_rows.go:212-215` `handlePatchSerialNumber`. | OK |
| `upd_row_note_tecnici` — UPDATE `note_tecnici` | SQL | `store_rows.go:252-255` `handlePatchTechnicalNotes`. | OK |
| `order_perso` — UPDATE `cdlan_stato='PERSO'` | SQL | n/a | DEV (D11) |

**Gateway / GW actions:**

| Appsmith item | Method/URL | MrSmith location | Class |
|---|---|---|---|
| `GW_Kickoff` | `GET /orders/v1/kick-off/{id}` | `pdf.go:19-27` `handleKickoffPDF` → `GET /api/ordini/v1/orders/{id}/kickoff.pdf`. | OK |
| `GW_ActivationForm` | `GET /orders/v1/activation-form/{id}` | `pdf.go:29-42` `handleActivationFormPDF` → `/activation-form.pdf`; IT/EN filename driven by `profile_lang`. | OK |
| `GW_SendToErp` | `POST /orders/v1/erp` (per-row JSON) | `gateway.go:52-58,111-185` `gatewaySendToERP` + `buildSendToERPPayload`. Payload preserves header+row keys; `cdlan_stato` is overridden to `"CREATO"`. | OK + DEV (D8) |
| `GW_SetActivationDate` | `POST /orders/v1/set-order-activation` | `gateway.go:60-71` `gatewaySetActivationDate`. | OK |
| `GW_SavePdfToArxivar` | `POST /orders/v1/send-to-arxivar` JSON `{file, orderId}` | `gateway.go:73-109` `gatewayUploadToArxivar` — **multipart** with parts `file`/`orderId`/`filename`/`multipart`. | CV (CV1) |
| `GW_GetPDFArxivarOrder` | `GET /orders/v1/order/pdf/{id}?from=vodka` header `from=vodka` | `pdf.go:58-71` `handleSignedPDF` with `Query: "from=vodka"`. | OK |
| `DownloadOrderPDFintGW` | `GET /orders/v1/order/pdf/{id}/generate` | `pdf.go:44-56` `handleOrderPDF`. | OK |
| `GW_CancelOrder` | `POST /orders/v2/order/{id}/cancel` | n/a | DEV (D10) |
| `GW_SendRequestAnnullaOdv` | `GET /{{OrderId}}` (suspicious/broken URL in Appsmith) | n/a | DEV (D10) |

**JS objects (orchestrators):**

| Appsmith item | What it does | MrSmith location | Class |
|---|---|---|---|
| `SendToErp.run()` | loop rows → call `GW_SendToErp` per row; on `err==0` call `setState` then `GW_SavePdfToArxivar` if file selected | `workflow_send.go:18-102` — loop rows; if all OK, UPDATE state then upload Arxivar; per-row outcome returned. | OK + DEV (D6, D17) |
| `SendToErp.setState(order_id)` | call `UpdateOrderState.run({OrderId})` | inlined into `workflow_send.go:79-90`. | OK |
| `SetActivationDate.run()` | parallel `saveInVodka` / `saveInErp` / `checkRows`; if all OK and `totale==countAllRows`, call `SetOrderStateAttivo` | `workflow_activate.go:13-126` — sequenced (DB tx → GW → counts → optional ATTIVO update → commit), preserving the auto-ATTIVO trigger. | OK |
| `SetActivationDate.saveInVodka(row)` | `SaveActivationDate.run({SystemodvRow, DataAttivazione})` | inlined in `workflow_activate.go:76-79`. | OK |
| `SetActivationDate.saveInErp(order, row)` | `GW_SetActivationDate.run(...)` | `workflow_activate.go:83-87`. | OK |
| `SetActivationDate.checkRows(order)` | `CheckConfirmRows.run({orderID})` | `workflow_activate.go:89-101`. | OK + DEV (D1) |
| `SetActivationDate.SetOrderStateAttivo(order)` | `SetOrderStateAttivo.run({OrderId})` | `workflow_activate.go:102-112`. | OK |
| `GetPdf.kickOff()` | `GW_Kickoff` → `download(response, 'Kick-off …pdf')` | `apps/ordini/src/pages/OrderDetailPage.tsx:187-208` + `downloads.kickoff` (`queries.ts:146`) + filename via `apps/ordini/src/api/pdf.ts`. | OK |
| `GetPdf.activationForm()` | `GW_ActivationForm` → filename IT/EN by `profile_lang` | `OrderDetailPage.tsx:187-208` + `pdf.go:33-41` (server picks filename). | OK |
| `OrderTools.download()` | `DownloadOrderPDFintGW` → base64/raw detection → blob download | `OrderDetailPage.tsx:187-208` + `pdf.go:158-200` (server-side `normalizePDFBody` handles base64/JSON wrapper). | OK |
| `GetPdfOrdineArx.GetPdfOrdineArx(orderId)` | `GW_GetPDFArxivarOrder` → blob | `OrderDetailPage.tsx:187-208` + `pdf.go:58-71` + normalization. | OK |
| `SendRequestAnnullaOdv.run()` | `GW_SendRequestAnnullaOdv` | n/a | DEV (D10) |
| `utili.salvaRiga()` | `upd_row_serNum.run({serial, systemodv})` then `RigheOrdine.run()` | `OrderDetailPage.tsx:132-142` (`saveSerial`) + `usePatchSerialNumber` invalidates rows query. | OK |
| `utili.salvaNoteTecniche()` | `upd_row_note_tecnici.run({note, idRiga})` then `RigheOrdineTecnici.run()` | `OrderDetailPage.tsx:144-154` (`saveNotes`) + `usePatchTechnicalNotes` invalidates technical-rows query. | OK |

**Detail-header buttons & gates:**

| Appsmith widget | Gate expression | MrSmith counterpart | Class |
|---|---|---|---|
| `TornaIndietro` "Torna alla lista ordini" → `navigateTo('Home')` | none | `DetailHeader.tsx:31-34` Back button → `navigate('/ordini')` | OK |
| `Visualizza_odv_arx` "Visualizza ordine firmato" `DIS={{arx_doc_number == null}}` | state-independent | `DetailHeader.tsx:45-47` "Ordine firmato" + `canDownloadSignedPdf` (`permissions.ts:52-54`) | OK |
| `Download_kickoff` `DIS={{stato!='INVIATO' || CR==false}}` | INVIATO + CR | `DetailHeader.tsx:36-38` + `canDownloadKickoffPdf` (`permissions.ts:39-41`) + backend `pdf.go:19-27 AllowedStates=[INVIATO] RequiresCR=true` | OK |
| `Genera_MA` `DIS={{(stato!='ATTIVO' && stato!='INVIATO') || CR==false}}` | INVIATO/ATTIVO + CR | `DetailHeader.tsx:39-41` + `canDownloadActivationFormPdf` + backend `pdf.go:29-42 AllowedStates=[INVIATO,ATTIVO] RequiresCR=true` | OK |
| `Scarica_PDF_button` `DIS={{arx_doc_number != null}}` | when no Arxivar doc | `DetailHeader.tsx:42-44` + `canDownloadOrderPdf` + backend `pdf.go:44-56 Check` | OK |

**Info tab fields (Appsmith Dettaglio modal & header texts):**

| Appsmith text widget | Source field | MrSmith location | Class |
|---|---|---|---|
| `cdlan_tipodoc` "Tipo di documento" | `cdlan_tipodoc` | `InfoTab.tsx:63` (label "Tipo documento") | OK |
| `cdlan_tipo_ord` "Tipo di ordine" | `cdlan_tipo_ord` | `InfoTab.tsx:64` (label "Tipo proposta") | OK |
| `Text4` (System ODV) | `cdlan_systemodv` | `InfoTab.tsx:65` (label "ODV") | OK |
| (Commerciale) Text4CopyCopyCopy | `cdlan_commerciale` | `InfoTab.tsx:66` | OK |
| `cdlan_dur_rin` "Durata rinnovo" | `cdlan_dur_rin` | `InfoTab.tsx:67` + `formatDurRin` | OK |
| `mod_fatt_canoni` "Modalità di fatturazione canoni anticipata" | `cdlan_int_fatturazione` | `InfoTab.tsx:68` + `formatFatturazione` | OK (label rewrite; also DEV D2 / GAPm G7) |
| `mod_attiv` "Modalità di fatturazione attivazione" | `cdlan_int_fatturazione_att` | `InfoTab.tsx:69` + `formatFatturazioneAtt` | OK |
| `data_decorrenza` "Data decorrenza" | `data_decorrenza` | `InfoTab.tsx:70` | OK |
| `cdlan_tempi_ril` "Tempi rilascio" | `cdlan_tempi_ril` | `InfoTab.tsx:71` | OK |
| `cdlan_durata_servizio` "Durata servizio (mesi)" | `cdlan_durata_servizio` | `InfoTab.tsx:72` | OK |
| `cdlan_sost_ord` "Sostituisce ordini" | `cdlan_sost_ord` | `InfoTab.tsx:73` | OK |
| `profile_lang_2` "Lingua" | `profile_lang` | `InfoTab.tsx:74` | OK |
| `service_type` "Tipo di servizi" | `service_type` / `is_colo` | `DetailHeader.tsx:60` + `formatServiceTypes` | OK |
| `cdlan_datadoc` "Data ordine" | `cdlan_datadoc` | `DetailHeader.tsx:58` | OK |
| `cdlan_stato` "Stato" | `cdlan_stato` | `DetailHeader.tsx:55` `StatusBadge` | OK |
| `cdlan_cliente` (Ragione sociale, multiple instances) | `cdlan_cliente` | `DetailHeader.tsx:53` + `InfoTab.tsx:100` (in editable block) + `AziendaTab.tsx:12` | OK |
| `cdlan_cod_termini_pag` "Condizioni di pagamento" | `cdlan_cod_termini_pag` | **not rendered** (data exposed in DTO) | **GAPm (G3)** |
| `cdlan_tacito_rin` "Tacito rinnovo" | `cdlan_tacito_rin` | **not rendered** (data exposed in DTO) | **GAPm (G2)** |
| `cdlan_note` "Note Legali" | `cdlan_note` | **not rendered** (data exposed in DTO) | **GAPm (G1)** |
| `written_by` "Redatto da" | `written_by` | **not rendered** (data exposed in DTO) | **GAPm (G4)** |
| `Text6` "Consulta ordine in arxivar" + `Text8` static link | static `<a href=arxivar.cdlan.it/#!/view/27ad1a56…>` | `InfoTab.tsx:76-81` **dynamic** link `https://arxivar.cdlan.it/#!/view/${arx_doc_number}` shown only when `arx_doc_number` present | OK (improvement) |

**Info tab — editable block (BOZZA save):**

| Appsmith widget | Gate | MrSmith counterpart | Class |
|---|---|---|---|
| `cdlan_rif_ordcli` INPUT `DIS={{stato!='BOZZA' \|\| CR==false}}` | BOZZA + CR | `InfoTab.tsx:90-93` driven by `canEditHeader` (= `canEditBozzaHeader`, `permissions.ts:14-16`) | OK |
| `cdlan_dataconferma` DATE_PICKER same gate | BOZZA + CR | `InfoTab.tsx:94-97` same gate | OK |
| `erp_an_cli` SINGLE_SELECT_TREE same gate | BOZZA + CR | `InfoTab.tsx:98-103` + `CustomerSelect.tsx`; options from `useCustomers` (Alyante) | OK |
| `Button3` "SALVA" `VIS={{stato=='BOZZA' && CR==true}}` → `SaveDataConfermaRifOrderCli.run()` + `Order.run()` | BOZZA + CR | `InfoTab.tsx:104-106` Save button + `OrderDetailPage.tsx:114-121 saveHeader` → `PATCH /ordini/v1/orders/{id}`; backend writes `cdlan_cliente_id` too | OK + DEV (D7) |

**Info tab — Invia in ERP block:**

| Appsmith widget | Gate | MrSmith counterpart | Class |
|---|---|---|---|
| `arxivar` FILE_PICKER `DIS={{(arx_doc_number!=null) \|\| (stato!='ANNULLATO' \|\| stato!='PERSO' \|\| stato!='ATTIVO') && CR==false}}` (buggy OR-chain) | Q6 bug rewrite to `state NOT IN {…} AND CR` | `InfoTab.tsx:113-119` driven by `canUploadPdf` (= `canShowArxivarFilePicker`, `permissions.ts:34-37`) | OK + DEV (D5) |
| `invia` BUTTON "INVIA in ERP" `DIS={{ (dataconferma==null || dataconferma=='') || cliente=='' || files.length==0 || stato!='BOZZA' || CR==false }}` (full expression cut in dump, but matches preconditions) | dataconferma + cliente + file + BOZZA + CR | `InfoTab.tsx:120-122` driven by `readyToSend` + `canEditHeader` + file presence; backend `workflow_send.go:36-58` repeats the checks (`missing_confirmation_date`, `missing_customer`, `missing_pdf`, `wrong_state`, `precondition_missing`) | OK + DEV (D17) |
| (Appsmith Arxivar file optional in JS) | `if (arxivar.files.length > 0)` | MrSmith requires file unconditionally | DEV (D17) |

**Azienda fields:**

| Appsmith | MrSmith | Class |
|---|---|---|
| `profile_iva`, `profile_cf`, `profile_address`, `profile_city`, `profile_cap`, `profile_pv`, `profile_sdi`, `cdlan_cliente_2`, `profile_lang_2` | `AziendaTab.tsx:11-23` Ragione sociale / ID cliente / Partita IVA / Codice fiscale / Indirizzo / Città / CAP / Provincia / SDI / Profilo lingua / Soluzione (`is_colo`) | OK |

**Referenti fields:**

| Appsmith INPUT_WIDGET_V2 | Gate | MrSmith |
|---|---|---|
| `cdlan_rif_tech_nom`, `cdlan_rif_tech_tel`, `cdlan_rif_tech_email`, `cdlan_rif_altro_tech_nom`, `cdlan_rif_altro_tech_tel`, `cdlan_rif_altro_tech_email`, `cdlan_rif_adm_nom`, `cdlan_rif_adm_tech_tel`, `cdlan_rif_adm_tech_email` `DIS={{(stato!='BOZZA' && stato!='INVIATO') \|\| CR==false}}` | BOZZA/INVIATO + CR | `ReferentiTab.tsx:24-39,42-81` 3 groups (Tecnico/Altro tecnico/Amministrativo) gated by `canEditReferents` (`permissions.ts:22-24`); backend `store_orders.go:286-332` enforces same. **OK** |
| `Button6` "Salva" same gate → `SaveOrderReferents.run()` | BOZZA/INVIATO + CR | `ReferentiTab.tsx:35-37` Save button. **OK** |

**Righe table (`Lista_righe`):**

| Appsmith column | Editable gate | MrSmith counterpart | Class |
|---|---|---|---|
| `Codice articolo bundle` (`IF cdlan_codice_kit != '' THEN CONCAT(cdlan_codice_kit,'-',index_kit)`) | readonly | `RigheTab.tsx:71` + `OrderRow.bundle_code` (computed in `store_rows.go:17`) | OK |
| `Codice articolo` | readonly | `RigheTab.tsx:72` | OK |
| `Descrizione articolo` | readonly | `RigheTab.tsx:73` | OK |
| `Quantità` | readonly | `RigheTab.tsx:74` | OK |
| `Canone` | readonly | `RigheTab.tsx:75` + `formatMoney(canone, cdlan_valuta)` | OK |
| `Attivazione` aliased "Setup" | readonly | `RigheTab.tsx:76` label "Prezzo attivazione" + `formatMoney(activation_price, cdlan_valuta)` | OK + DEV (D3) |
| `Numero seriale` `isCellEditable={{stato=='BOZZA' && CR==true}}` | BOZZA + CR (Appsmith) | `RigheTab.tsx:77-92` inline edit; gate `canEditSerialNumber(order)` = `BOZZA` only (no CR); backend `store_rows.go:176-226` likewise | DEV (D16) |
| `Data attivazione` | readonly | `RigheTab.tsx:92` | OK |
| `customColumn1` "Modifica" iconButton `isCellVisible={{stato=='INVIATO' && CR==true}}` → `showModal('ModificaRiga')` | INVIATO + CR | `RigheTab.tsx:98-100` `Modifica` button + `canOpenActivationModal` (also requires `data_annullamento==null && cdlan_qta!==0` per IMPL §11) → opens `ActivationModal` | OK |
| `EditActions1` Save/Discard | implicit | `RigheTab.tsx:78-91` inline check/x icons | OK |
| `ID Riga` (hidden), `Prezzo cessazione` (hidden), `confirm_data_attivazione` (hidden), `System ODV Riga` (hidden), `Codice raggruppamento fatturazione` (hidden) | n/a | `id`, `termination_price`, `confirm_data_attivazione`, `cdlan_systemodv_row`, `cdlan_ragg_fatturazione` all present in DTO (`types.go:84-102`) | OK (data); GAPm G5 if surfaced is wanted |

**Activation modal (`ModificaRiga`):**

| Appsmith widget | Gate | MrSmith counterpart | Class |
|---|---|---|---|
| `Text5` "Riga numero {{Lista_righe.triggeredRow[…]}}" | readonly | `ActivationModal.tsx:30-35` shows codart / descart of selected row | OK |
| `cdlan_data_attivazione` DATE_PICKER `DIS={{stato=='ATTIVO'}}` | not ATTIVO | `ActivationModal.tsx:36-46` date input; modal only opens for INVIATO so the gate is enforced upstream by `canOpenActivationModal` | OK |
| `TXT_data_atti_tech` "Data indicata dai tecnici: …" | readonly preview of existing date | **not shown in modal** (visible on the row itself) | **GAPm (G6)** |
| `cdlan_serialnumber` INPUT inside modal `DIS={{stato!='BOZZA'}}` | BOZZA | not duplicated in modal (only the per-row inline edit exists in `RigheTab`) | OK (function preserved at column level) |
| `BTN_confirm_act_modal` "CONFERMA" `VIS={{stato!='ATTIVO'}}`, `DIS={{cdlan_data_attivazione==null}}` → `SetActivationDate.run()` | not ATTIVO + date required | `ActivationModal.tsx:49-51` Confirm button disabled until `activationDate` set; `OrderDetailPage.tsx:156-165 confirmActivation` → `PATCH /ordini/v1/orders/{id}/rows/{rowId}/activate` | OK |
| `BTN_close_act_modal` / `IconButton1` close handlers | always | `ActivationModal.tsx:21-24,47-48` Cancel + Modal close | OK |

**Tecnici table (`Lista_righe_tecnici`):**

| Appsmith column | Editable gate | MrSmith counterpart | Class |
|---|---|---|---|
| `codice articolo bundle` (`concat(cdlan_codice_kit,'-',index_kit)`) | readonly | `TechnicalNotesTab.tsx:62` + `TechnicalRow.bundle_code` (`store_rows.go:89`) | OK |
| `codice articolo` | readonly | `TechnicalNotesTab.tsx:63` | OK |
| (extra in MrSmith) `Descrizione articolo` | readonly | `TechnicalNotesTab.tsx:64` + `TechnicalRow.cdlan_descart` (`store_rows.go:91`) | OK (additive) |
| `note tecnici` `isCellEditable={{stato=='INVIATO' && CR==true}}` (with `CONVERT note_tecnici USING UTF8`) | INVIATO + CR (Appsmith) | `TechnicalNotesTab.tsx:65-71,74-83` inline edit always available; backend `store_rows.go:228-266` has no CR/state gate | DEV (D15) |
| `data annullamento` | readonly | `TechnicalNotesTab.tsx:72` | OK |
| `ID riga` (hidden) | n/a | `TechnicalRow.id` exposed | OK |
| `EditActions1` Save/Discard | implicit | `TechnicalNotesTab.tsx:74-83` inline Save/Annulla | OK |

**Other Dettaglio ordine widgets:**

| Appsmith widget | Gate | MrSmith counterpart | Class |
|---|---|---|---|
| `butt_annullato` "RICHIEDI ANNULLAMENTO" `DIS={{stato!='INVIATO' \|\| CR==false}}` → `GW_CancelOrder.run({order_number: order_id.text})` | INVIATO + CR | absent | DEV (D10) |
| `butt_perso` "ORDINE PERSO" `VIS=false` → `order_perso.run() … 'stato aggiornato'` | hidden in Appsmith | absent | DEV (D11) |
| `Titolo` "Codice ordine: …/…" | readonly | `DetailHeader.tsx:51-52` `<h1>Codice ordine: {orderCode(ndoc, anno)}</h1>` | OK |
| `Modal1` "Consulta ordine in arxivar" (static iframe-style link) | readonly | replaced by dynamic external link in `InfoTab.tsx:76-81` | OK |

---

## Cross-cutting wiring (verified)

| Item | Reference | Notes |
|---|---|---|
| Access role `app_ordini_access` | `backend/internal/platform/applaunch/catalog.go:90,528-530` + `packages/auth-client/src/roles.ts:32` | Mirrors IMPL §16/§4. |
| Capability role `app_customer_relations` | `catalog.go:91,532-534` + `permissions.go:12-26` + frontend `permissions.ts:4-7` | Enforced backend; advisory frontend. |
| Catalog entry MKT&Sales | `catalog.go:213-225` | `AccessRoles: OrdiniAccessRoles()` only (CR not listed as access role). |
| `OrdiniAppURL` env override | `backend/internal/platform/config/config.go` + `backend/cmd/server/main.go:406-409` | Dev fallback `http://localhost:5192`. |
| Mistra fallback for origin | `store_origin.go:13-15` | If `MISTRA_DSN` absent, `origin` is omitted; matches IMPL §7 origin contract. |
| Vodka-not-configured guard | `handler.go:62-68` returns `503 vodka_database_not_configured`; catalog hides app when `cfg.VodkaDSN == ""` (`main.go:476`) | OK. |

## Datasources mapping

| Appsmith datasource | MrSmith binding | Class |
|---|---|---|
| `vodka` (MySQL) | `Deps.Vodka` (`backend/internal/ordini/handler.go:21`); reads/writes in `store_orders.go`, `store_rows.go`, `workflow_*.go` | OK |
| `Alyante` (MSSQL) | `Deps.Alyante` (`handler.go:22`); `store_customers.go` for `Tsmi_Anagrafiche_clienti` | OK |
| `db-mistra` (Postgres) | `Deps.Mistra` (`handler.go:23`); `store_origin.go` for `orders.legacy_orders` + `quotes.quote` | OK (HubSpot deal tables `loader.hubs_*` & `loader.erp_metodi_pagamento` are referenced only by the OUT-OF-SCOPE `Ordini semplificati` page — DEV D12) |
| `GW internal CDLAN` (REST) | `Deps.Arak` (`*arak.Client`, `handler.go:24`); typed wrappers in `gateway.go` + `pdf.go` | OK |
