# Findings summary — Ordini

## Embedded business rules (must be re-expressed on the backend)

1. **Order state machine** — `BOZZA → INVIATO → ATTIVO`, with side branches `PERSO` and `ANNULLATO`. The only legal transitions actually wired in the UI are:
   - BOZZA → INVIATO via `SendToErp.run` → `UpdateOrderState` (also sets `cdlan_evaso=1`).
   - INVIATO → ATTIVO implicit, triggered inside `SetActivationDate.run` when every row has `confirm_data_attivazione=1` OR `data_annullamento` set OR `cdlan_qta=0` (the count from `CheckConfirmRows` equals `RigheOrdine.data.length`).
   - BOZZA → PERSO via `order_perso` SQL (button currently hidden — branch likely unused).
   - INVIATO → ANNULLATO via `GW_CancelOrder` / `GW_SendRequestAnnullaOdv` (server-side effect, vodka state not changed by this app).
2. **Permission rules** — three roles matter:
   - Viewer (any authenticated user): can open and see details.
   - `CustomerRelations` group (Keycloak): only this group can edit PO/data conferma/ragione sociale, trigger "INVIA in ERP", request cancellation, edit per-row serial numbers, edit referents, download kick-off/activation form.
   - Technicians (implicit — not CustomerRelations): edit the `note_tecnici` column via the "Informazioni dai tecnici" tab. Today the `Button6` (Referenti SALVA) is also gated to CustomerRelations, but the `Lista_righe_tecnici` editActions is not gated explicitly in the DSL — technicians can save notes.
3. **"INVIA in ERP" precondition** — all of: `cdlan_dataconferma` set, `erp_an_cli` selected (Ragione sociale), `arxivar.files.length > 0` (signed PDF selected), `cdlan_stato == 'BOZZA'`.
4. **PDF-action preconditions**:
   - Kick-off PDF: state `INVIATO` + CustomerRelations.
   - Activation form PDF: state in {`INVIATO`,`ATTIVO`} + CustomerRelations.
   - Scarica PDF ordine: only when `arx_doc_number IS NULL` (pre-Arxivar generation).
   - Visualizza ordine firmato: only when `arx_doc_number IS NOT NULL`.
5. **Annullamento precondition** (from the legacy Home modal): `cdlan_evaso == 1 AND cdlan_stato == 'INVIATO' AND from_cp == 0`. Orders that originated in the Customer Portal cannot be cancelled from this app.
6. **Form-field dependency**:
   - `cdlan_sost_ord` must be empty / disabled when `cdlan_tipo_ord == 'N'` (Nuovo).
   - `service_type` is locked when `is_colo != 0` (colocation/IaaS orders encode their "service type" via the colocation code).
7. **Per-row activation promotion** — when every row is confirmed, the order auto-transitions to ATTIVO. This is the **only path** to ATTIVO.
8. **Dual-write invariant** — activation date is written to both vodka (`orders_rows`) and Alyante (via GW `/orders/v1/set-order-activation`). Same for state transitions: vodka local state flips to INVIATO while the ERP receives `"cdlan_stato": "CREATO"` in the payload. These two states are intentionally different and must be kept so in the rewrite.
9. **Filename localization** — Activation form PDF: `"Modulo di Attivazione"` (it) / `"Activation Form"` (en) based on `profile_lang`.
10. **Ragione sociale persistence shape** — the order row stores the human-readable company name (string), not the `NUMERO_AZIENDA` ID. The rewrite should persist both (and refer to customers by ID going forward).
11. **Hidden enums embedded in the UI** — these are the de-facto business catalogs:
    - `cdlan_tipodoc`: `TSC-ORDINE-RIC` (Ordine ricorrente) / `TSC-ORDINE` (Ordine spot).
    - `cdlan_tipo_ord`: `A` Sostituzione / `N` Nuovo / `R` Rinnovo.
    - `cdlan_dur_rin`, `cdlan_int_fatturazione`: `1,2,3,4,6,12` (Mensile … Annuale). **Note:** `Form ordine` uses `4` = Quadrimestrale, but `Order` SQL CASE uses `5` = Quadrimestrale — mismatched, verify which is canonical.
    - `cdlan_int_fatturazione_att`: `1` All'ordine / `2` All'attivazione del servizio/Consegna.
    - `profile_lang`: `it` / `en`.
    - `is_colo`: `0` = Altre soluzioni, `Colocation variabile`, `Iaas payperuse` (diretto), `Iaas payperuse indiretto`.
    - `service_type` multi-select: `Connettività, Cloud, Security, Voce, Supporto`.
    - `cdlan_cod_termini_pag`: ~30 codes (BB/SDD/Vista fattura variants). The same list also exists in `db-mistra.loader.erp_metodi_pagamento` (via `get_payment_methods`) but is not consumed anywhere.

## Duplication & orphans

### Duplicated logic
- The "full detail" view exists on **two pages**: the `Dettaglio_ordine` modal on `Home` and the entire `Dettaglio ordine` page. The modal is dead. Only port the page.
- `Dettaglio_ordine_vero` and `Lista_righe_d_ordine` queries exist both on `Home` (keyed off `triggeredRow.id`) and on `Form ordine` (keyed off `URL.queryParams.id`). Same SQL, different input.
- `SendToErp` JSObject and `run` action (`UNUSED_DATASOURCE`) contain the same ERP-push loop body. Appsmith serializes JSObject methods as pseudo-actions; they are not extra logic.
- PDF decode logic (base64-or-raw + blob download) is duplicated between `GetPdfOrdineArx.GetPdfOrdineArx` and `OrderTools.download`.
- Cancellation has **two** paths: `butt_annullato.onClick → GW_CancelOrder` (with param-name bug) and `SendRequestAnnullaOdv JSObject → GW_SendRequestAnnullaOdv` (broken path). Only one is reachable today; both should collapse into a single backend endpoint.

### Orphan/dead code — do not port
- Page: `Draft gp da offerta` (empty), `Ordini semplificati` (unfinished).
- Page section: `Home.Dettaglio_ordine` modal and everything inside it; `Home.Nuovo_Ordine` icon (hidden+disabled).
- Page section: `Dettaglio ordine.butt_perso` (hidden).
- Queries: `Query1`, `Insert_orders1`, `Update_orders1`, `Select_orders1`, `Total_record_orders1`, `Dettaglio_riga_d_ordine`, `Lista_righe_d_ordine_info_tecn`, `get_payment_methods`, `order_perso`.
- JS: `Home.JSObject1`, `Dettaglio_ordine.JSObject1`, `Ordini_semplificati.utils` (except `globals.formVisibile`), `SendRequestAnnullaOdv`.
- REST actions: `GW_SendRequestAnnullaOdv`.

### Unfinished features
- `Form ordine` has no save action. `Button1`/`Button2` carry no `onClick`. The page is prefilled from the URL query param and cannot be submitted from the UI.
- `Ordini semplificati.ButtonGroup1` and `Form1` carry only default Appsmith placeholder config (Favorite/Add/More, Submit/Reset with no handlers).
- The "create new order" UX is not reachable in this app. Confirm whether order creation happens in the Customer Portal / ERP and this app is a lifecycle-only frontend.

## Security & correctness concerns

1. **SQL injection everywhere.** All 15 vodka queries use string-interpolated `{{…}}` values:
   - `Select_orders1` interpolates `Lista_ordini.searchText` directly into `LIKE '%…%'` and `Lista_ordini.sortOrder.column` directly into `ORDER BY`.
   - Every `UPDATE` on Dettaglio ordine reads widget `.text`/`.formattedDate` values and concatenates them.
   - `Insert_orders1`/`Update_orders1` concatenate 50 widget values each.
   - The rewrite must use parameterized queries or a Go ORM layer; this is non-negotiable.
2. **Direct multi-DB connections from the browser.** Appsmith bridges these connections, so raw credentials are server-side, but (a) the schema is exposed to any authenticated Appsmith user, (b) there is no API boundary to enforce permissions, and (c) the `CustomerRelations` check is client-side only. The rewrite must hide Alyante, db-mistra and vodka behind Go endpoints with Keycloak-role-based authorization.
3. **GW credentials and auth model.** The REST datasource has no declared auth in the export; production likely carries an `Authorization` header or IP-based ACL. Verify and move all GW calls server-side.
4. **Base64-or-raw PDF heuristic.** The JS detects whether the payload starts with `%PDF` or matches a base64 charset — fragile. Normalize the payload shape in the backend instead.
5. **`GW_CancelOrder` parameter mismatch.** `butt_annullato.onClick` passes `{order_number: this.order_id.text}` but the URL template expects `{{order_Id}}`. The compiled URL becomes `/orders/v2/order//cancel`. Either the live export differs from the JSON we inspected, or the feature is silently broken. **Verify before porting.**
6. **`CheckConfirmRows` has an always-false clause.** `data_annullamento <> null` never matches; should be `IS NOT NULL`. This likely underreports the "confirmed rows" count and prevents the auto-transition to ATTIVO when a row has been cancelled. Confirm intended behavior before copying.
7. **Partial-success state divergence on ERP push.** `SendToErp.run` loops rows, sets `err=1` on first failure, continues, and only runs `UpdateOrderState` if `err == 0`. If the loop fails on row 5 of 10, rows 1–4 are in the ERP but vodka stays in BOZZA and no alert distinguishes "all failed" from "some failed". The rewrite must wrap this in a transactional endpoint or at minimum return a per-row outcome and explicit partial-success UX.
8. **`GW_SendToErp` hard-codes `"cdlan_stato": "CREATO"`.** ERP receives CREATO regardless of local state — intended (the ERP owns its state) but worth flagging so nobody "fixes" it inadvertently.
9. **`Promise.all` on awaited values.** Both `SendToErp.run` and `SetActivationDate.run` do `await saveInVodka; await saveInErp; await checkRows; Promise.all([saveInVodka, saveInErp, checkRows])`. The `.then` receives the already-resolved values; no harm but confusing, and the `.catch` never fires because the individual calls swallow errors into `return false`. Keep the dual-write but simplify the flow.
10. **Hidden helper widgets as globals.** `order_id.text`, `cdlan_ndoc.text`, `cdlan_anno.text`, `profile_lang.text`, etc., are referenced by JSObjects as if they were globals. Fragile on rename; replace with explicit arguments in the rewrite.
11. **Alias drift between pages.** On Dettaglio ordine, `cdlan_prezzo_attivazione` is aliased to `'Attivazione'`; on Home/Form ordine, the same column is aliased to `'Prezzo attivazione'`. `SendToErp.run` reads `item["Attivazione"]` — works only because the instance on Dettaglio ordine uses `'Attivazione'`. Standardize in the rewrite.
12. **State "ATTIVO" vs "INVIATO" in arxivar binding.** `arxivar.isDisabled` contains `(Order.data[0].cdlan_stato != 'ANNULLATO' || Order.data[0].cdlan_stato != 'PERSO' || Order.data[0].cdlan_stato != 'ATTIVO')` — this is always `true` because those comparisons are OR'd and a single state value can't equal all three simultaneously. Combined with `&& appsmith.user.groups.includes('CustomerRelations')==false`, the file picker is likely enabled in more cases than intended. Flag for review.

## Migration blockers

- **Authoritative source for enumerations.** Payment terms live in `loader.erp_metodi_pagamento` *and* hard-coded in `Form ordine`. The rewrite needs a single source of truth; decide with the product owner.
- **`GW_CancelOrder` contract.** The exported binding looks broken; the spec for the backend cancellation endpoint depends on verifying the live behavior.
- **`CheckConfirmRows` SQL bug.** Fixing the `<> null` bug changes the count for rows with `data_annullamento` set. Confirm whether the promotion to ATTIVO should include cancelled rows (the bug currently says no).
- **Customer ID vs display name on `orders.cdlan_cliente`.** The column stores the string. Migrating to an ID reference requires a backfill script over existing data (map `RAGIONE_SOCIALE → NUMERO_AZIENDA`) and a schema change (`customer_id` column).
- **Authorization source of truth.** The current app reads `appsmith.user.groups`. The rewrite uses Keycloak with `app_ordini_access`; mapping the `CustomerRelations` group to a Keycloak role needs to be confirmed.
- **Order creation scope.** Not implemented here, but likely needed. Before migrating, decide whether the rewrite implements it, reuses the Customer Portal, or keeps it entirely in the ERP.
- **Partial-success UX.** The rewrite needs a designed error state for "some rows failed to sync to ERP". The Appsmith version hides this.

## Recommended next steps

1. Run `appsmith-migration-spec` on this audit to produce Phase A–D migration specs. Use the page, datasource, and findings docs as the primary inputs.
2. Freeze the scope decisions above with the product owner before Phase B (SPEC):
   - Is order creation in-scope for the rewrite?
   - Is "ORDINE PERSO" still needed?
   - Is the annullamento flow in or out?
   - Does the rewrite continue to use `from_cp` semantics?
   - Keep the auto-ATTIVO transition as-is or expose it as an explicit action?
3. Pre-define the Go API contracts the rewrite will expose (`GET /ordini`, `GET /ordini/:id`, `PATCH /ordini/:id`, `PATCH /ordini/:id/referents`, `POST /ordini/:id/send-to-erp` (multipart), `PATCH /ordini/:id/rows/:rowId/activate`, `PATCH /ordini/:id/rows/:rowId/serial-number`, `PATCH /ordini/:id/rows/:rowId/technical-notes`, `POST /ordini/:id/cancel-request`, `GET /ordini/:id/{kickoff,activation-form,pdf,signed-pdf}.pdf`, `GET /ordini/ref/{customers,payment-terms,service-types,potentials}`). Validate against `docs/mistra-dist.yaml` for any overlap with Mistra NG Internal API.
4. Confirm whether `GW_CancelOrder` works in production today; if not, decide whether the rewrite fixes or removes it.
