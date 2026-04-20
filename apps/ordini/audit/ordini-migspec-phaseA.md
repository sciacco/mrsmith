# Ordini — Migration Spec · Phase A: Entity-Operation Model

Source: `apps/ordini/audit/{app-inventory,page-audit,findings-summary,datasource-catalog}.md`.
Scope directive: **port 1:1, ignore dead features**. Only `Home` and `Dettaglio ordine` are in scope.

---

## Entities extracted from the audit

### 1. `Order` — vodka.`orders` (primary aggregate)
**Purpose.** The lifecycle record for a sales order (proposta/ordine). Created elsewhere (Customer Portal or ERP), then managed through this app from BOZZA to ATTIVO (with ANNULLATO side branch).

**Fields (inferred types from SQL + widget bindings).**

| Field | Type | Evidence | Notes |
|---|---|---|---|
| `id` | bigint PK | `WHERE id = …` everywhere | Vodka-local surrogate key. |
| `cdlan_systemodv` | string | TEXT widget, ERP payload | ERP-side ODV identifier. |
| `cdlan_tipodoc` | enum `TSC-ORDINE-RIC` \| `TSC-ORDINE` | Select_Orders_Table, widget | "Ordine ricorrente" vs "Ordine spot". |
| `cdlan_ndoc` | int | `concat(cdlan_ndoc,'/',cdlan_anno)` | Proposta number. |
| `cdlan_anno` | int (4-digit year) | composite key display | |
| `cdlan_sost_ord` | string \| null | "Sostituisce ordini (Num/Anno)" | Free text. |
| `cdlan_cliente` | string | `UPDATE … SET cdlan_cliente = erp_an_cli.selectedOptionValue` | **Persisted as the RAGIONE_SOCIALE string** today. Audit recommends adding a `customer_id` (NUMERO_AZIENDA). **Phase A open question.** |
| `cdlan_cliente_id` | ? | present in `Order` SELECT list | Unused in the UI; value/semantics unknown. |
| `cdlan_datadoc` | date | display | |
| `cdlan_dataconferma` | date \| null | editable in BOZZA, precondition for "INVIA in ERP" | |
| `cdlan_stato` | enum `BOZZA` \| `INVIATO` \| `ATTIVO` \| `PERSO` \| `ANNULLATO` | state machine | PERSO/ANNULLATO transitions not driven from this app in current scope. |
| `cdlan_evaso` | 0 \| 1 | set to 1 on send-to-ERP | Used in annullamento precondition (legacy modal). |
| `cdlan_chiuso` | 0 \| 1 | returned by `Order` | Not referenced in UI bindings. |
| `cdlan_tipo_ord` | enum `A` \| `N` \| `R` | display mapping | Sostituzione/Nuovo/Rinnovo. |
| `cdlan_dur_rin` | int ∈ {1,2,3,4,6,12} | display | Months. |
| `cdlan_tacito_rin` | 0 \| 1 | display | |
| `cdlan_tacito_rin_in_pdf` | 0 \| 1 | `Order` SELECT | Not rendered in UI. |
| `cdlan_int_fatturazione` | int | CASE in `Order` SQL | **Enum drift:** `Order` SQL maps `5`→Quadrimestrale; `Form ordine` dropdown uses `4`. `Form ordine` is dead — 1:1 port means `5` wins. |
| `cdlan_int_fatturazione_att` | enum `1` \| `2` | display | 1 = All'ordine, 2 = All'attivazione. |
| `cdlan_cod_termini_pag` | string code | display only | ~30 codes; live in the DB (`loader.erp_metodi_pagamento`), not maintained here. |
| `origin_cod_termini_pag` | string | `Order` SELECT | Not rendered. |
| `cdlan_commerciale` | string | `Order` SELECT | Not rendered. |
| `cdlan_tempi_ril` | string | `Order` SELECT | Not rendered. |
| `cdlan_durata_servizio` | string | `Order` SELECT | Not rendered. |
| `cdlan_note` | text | `cdlan_note_bkp` hidden widget | Not visibly edited. Kept for completeness. |
| `cdlan_rif_ordcli` | string | editable in BOZZA | Customer PO number. |
| `cdlan_rif_tech_nom` | string | Referenti SALVA | |
| `cdlan_rif_tech_tel` | string | Referenti SALVA | |
| `cdlan_rif_tech_email` | string | Referenti SALVA | |
| `cdlan_rif_altro_tech_nom` | string | Referenti SALVA | |
| `cdlan_rif_altro_tech_tel` | string | Referenti SALVA | |
| `cdlan_rif_altro_tech_email` | string | Referenti SALVA | |
| `cdlan_rif_adm_nom` | string | Referenti SALVA | |
| `cdlan_rif_adm_tech_tel` | string | Referenti SALVA | Note the asymmetric column name (`tech_tel` on ADM). |
| `cdlan_rif_adm_tech_email` | string | Referenti SALVA | |
| `written_by` | string | `Order` SELECT | Not rendered. |
| `profile_iva`, `profile_cf`, `profile_address`, `profile_city`, `profile_cap`, `profile_pv`, `profile_sdi` | strings | Azienda tab | Readonly. |
| `profile_lang` | enum `it` \| `en` | Azienda tab (hidden INPUT) | Used for PDF filename localization. |
| `cdlan_valuta` | string | `Order` SELECT | Not rendered. |
| `service_type` | comma-separated string | display | Multi-select: Connettività/Cloud/Security/Voce/Supporto. |
| `data_decorrenza` | date | `Order` SELECT | Not rendered. |
| `is_colo` | string | display | 0 (Altre soluzioni) / `Colocation variabile` / `Iaas payperuse` / `Iaas payperuse indiretto`. |
| `is_arxivar` | 0 \| 1 | `Order` SELECT | Not rendered in UI. |
| `arx_doc_number` | string \| null | "Visualizza ordine firmato" precondition | Arxivar document number. |
| `from_cp` | 0 \| 1 | annullamento precondition in legacy modal | CP-originated orders cannot be cancelled. |

**Operations (in scope after dead-feature drop).**

| Operation | Source | Trigger | Inputs | Effects | Preconditions (state + role) |
|---|---|---|---|---|---|
| `listOrders` | Home table | page load | — | returns rows + display labels | any authenticated |
| `getOrder` | Dettaglio page load | `?id=` | order id | returns full order | any authenticated |
| `updateBozzaHeader` | `SaveDataConfermaRifOrderCli` | Info SALVA | `cdlan_dataconferma`, `cdlan_rif_ordcli`, `cdlan_cliente` | persists | BOZZA + CustomerRelations |
| `updateReferents` | `SaveOrderReferents` | Referenti SALVA | 9 `cdlan_rif_*` fields | persists | BOZZA or INVIATO + CustomerRelations |
| `sendToErp` | `SendToErp.run` orchestration | "INVIA in ERP" | order id, arxivar PDF file (multipart) | for each row → `GW_SendToErp`; then `UpdateOrderState` → INVIATO + cdlan_evaso=1; then `GW_SavePdfToArxivar` (if file present) | BOZZA + CustomerRelations + `cdlan_dataconferma` set + `erp_an_cli` selected + file attached |
| ~~`requestCancellation`~~ | `GW_CancelOrder` | "RICHIEDI ANNULLAMENTO" | — | **DEFERRED post-v1** (see Q1 resolution and `docs/TODO.md` → Ordini App). Button not rendered in v1. | — |
| `getKickoffPdf` | `GetPdf.kickOff` → `GW_Kickoff` | button | order id | download | INVIATO + CustomerRelations |
| `getActivationFormPdf` | `GetPdf.activationForm` → `GW_ActivationForm` | button | order id, `profile_lang` for filename | download | INVIATO or ATTIVO + CustomerRelations |
| `getOrderPdf` | `OrderTools.download` → `DownloadOrderPDFintGW` | button | order id | download (base64-or-raw) | `arx_doc_number IS NULL` |
| `getSignedPdf` | `GetPdfOrdineArx` → `GW_GetPDFArxivarOrder` | button | order id | download (base64-or-raw) | `arx_doc_number IS NOT NULL` |

**Relationships.**
- `Order` → `OrderRow` 1:N via `orders_rows.orders_id = orders.id`.
- `Order.cdlan_cliente` → `Customer.RAGIONE_SOCIALE` (string match; fragile).
- `Order.arx_doc_number` → Arxivar document (external).

**Business rules attached to the entity (inferred).**
- State machine: `BOZZA → INVIATO` (via sendToErp); `INVIATO → ATTIVO` (auto when every row is confirmed — see OrderRow); `INVIATO → ANNULLATO` (via GW, state flip is server-side).
- Dual-write to ERP on INVIATO and on per-row activation. The ERP uses `cdlan_stato = "CREATO"` regardless of local state; vodka uses `INVIATO`. **Intentional — keep as-is.**
- `cdlan_sost_ord` must be empty when `cdlan_tipo_ord = 'N'` (Form ordine rule; Form ordine is dropped, but this rule applies to legacy data and the read path).

---

### 2. `OrderRow` — vodka.`orders_rows` (child of Order)

**Purpose.** One article line per row. Holds per-row activation state, serial number, technical notes.

**Fields.**

| Field | Type | Evidence | Notes |
|---|---|---|---|
| `id` | bigint PK | `'ID Riga'` alias | |
| `orders_id` | bigint FK → orders.id | WHERE clause | |
| `cdlan_systemodv_row` | int | `'System ODV Riga'` alias, used as selector in UPDATEs | Stable business key used across vodka + ERP. |
| `cdlan_codice_kit` | string | bundle code computation | Empty string if not in a bundle. |
| `index_kit` | int | bundle index | |
| `cdlan_codart` | string | `'Codice articolo'` | |
| `cdlan_descart` | string | `'Descrizione articolo'` | |
| `cdlan_prezzo` | decimal | `'Canone'` | |
| `cdlan_prezzo_attivazione` | decimal | `'Attivazione'` (Dettaglio) / `'Prezzo attivazione'` (Home/Form) | **Alias drift — see §Open questions.** |
| `cdlan_qta` | int | `'Quantità'`; counted as "confirmed" when 0 | Zero-qty rows auto-count toward ATTIVO promotion. |
| `cdlan_prezzo_cessazione` | decimal | `'Prezzo cessazione'` | |
| `cdlan_ragg_fatturazione` | string | `'Codice raggruppamento fatturazione'` | |
| `cdlan_data_attivazione` | date \| null | editable (per-row modal) | |
| `cdlan_serialnumber` | string \| null | editable inline (BOZZA only) | |
| `note_tecnici` | text (needs UTF8 convert) | editable in "Informazioni dai tecnici" tab | |
| `data_annullamento` | date \| null | filter | Row-level cancellation date. Set by backend (not this app). |
| `confirm_data_attivazione` | 0 \| 1 | side-effect of activation write | **Implicit business rule:** set to 1 when the activation date is saved. |

**Operations.**

| Operation | Source | Trigger | Inputs | Effects | Preconditions |
|---|---|---|---|---|---|
| `listRows` | `RigheOrdine` | page load | order id | returns row array | any authenticated |
| `listTechnicalNotes` | `RigheOrdineTecnici` | page load | order id | returns per-row `note_tecnici` + `data_annullamento` | any authenticated |
| `updateSerialNumber` | `upd_row_serNum` + `utili.salvaRiga` | Righe tab row-save | `cdlan_systemodv_row`, `cdlan_serialnumber` | persists | BOZZA (per widget binding) |
| `updateTechnicalNotes` | `upd_row_note_tecnici` + `utili.salvaNoteTecniche` | Info-tecnici tab row-save | `cdlan_systemodv_row`, `note_tecnici` | persists | technicians (no explicit role gate in DSL) |
| `setActivationDate` | `SetActivationDate` orchestration | Modifica modal CONFERMA | `cdlan_systemodv`, `cdlan_systemodv_row`, `cdlan_data_attivazione` | (a) `SaveActivationDate` UPDATE with `confirm_data_attivazione=1`, (b) `GW_SetActivationDate` POST, (c) `CheckConfirmRows` COUNT, (d) if `COUNT == RigheOrdine.data.length` → `SetOrderStateAttivo` flips `orders.cdlan_stato` to ATTIVO | INVIATO + CustomerRelations |

**Relationships.** Belongs to `Order` via `orders_id`.

**Business rules.**
- `confirm_data_attivazione = 1` is written unconditionally when `setActivationDate` writes the date.
- `CheckConfirmRows` counts rows where `confirm_data_attivazione=1 OR data_annullamento <> null OR cdlan_qta=0`. **`<> null` is always false in MySQL — the cancelled-row branch never matches.** 1:1 port preserves the bug unless explicitly corrected (see §Open questions).
- Only the `cdlan_serialnumber` column is inline-editable on the Righe tab today.
- The "Modifica" per-row iconButton is only visible when order state is `INVIATO`.

---

### 3. `Customer` — Alyante.`Tsmi_Anagrafiche_clienti` (reference only)

**Purpose.** Source of the `erp_an_cli` single-select tree on the Dettaglio page for picking a "Ragione sociale" when saving BOZZA header.

**Fields used.**
- `NUMERO_AZIENDA` (identifier, **not persisted** on the order).
- `RAGIONE_SOCIALE` (display string, **persisted** in `orders.cdlan_cliente`).

**Filter rules (verbatim from `erp_anagrafiche_cli`).**
`DATA_DISMISSIONE IS NULL AND RAGGRUPPAMENTO_3 <> 'Ecommerce' AND TIPOLOGIA_AZIENDA <> 'DIPENDENTE'`, grouped by both returned columns.

**Operations.**
| Operation | Source | Trigger | Inputs | Effects |
|---|---|---|---|---|
| `listCustomers` | `erp_anagrafiche_cli` | page load | — | populates the dropdown |

**Relationships.** Loose join to `Order.cdlan_cliente` by string equality.

---

### 4. `ArxivarDocument` — external (GW / Arxivar)

Not a local entity. Referenced only by `orders.arx_doc_number` and the deep-link URL `https://arxivar.cdlan.it/#!/view/<uuid>/…`. No CRUD from this app; uploading the signed PDF is an effect of `sendToErp`.

---

## Dropped entities / candidates (not re-surfacing under 1:1)

- `HubSpotPotential` (Ordini semplificati): out of scope.
- `PaymentMethod` / `loader.erp_metodi_pagamento` (`get_payment_methods`): never consumed, drop.
- `OrderProfile` / `OrderContacts` (conceptually split in Form ordine): under 1:1 these stay as columns on `Order`, not separate entities — the Dettaglio page treats them as inline fields.

---

## Audit gaps & ambiguities — resolutions

### Q1. `GW_CancelOrder` — **RESOLVED: DEFER post-v1**

The "RICHIEDI ANNULLAMENTO" button and its backing endpoint are **out of scope for v1**. The canonical contract in `docs/mistra-dist.yaml:1819-1858` (`POST /orders/v2/order/{order_number}/cancel`, body `{customer_name}`) diverges from the Appsmith binding on both URL param name and body shape — it is unknown whether the feature works in production at all. Rather than port a plausibly-broken flow, v1 ships without the button.

Follow-up task recorded in `docs/TODO.md` → "Ordini App → RICHIEDI ANNULLAMENTO (cancel order) — deferred post-v1". That task also captures the two sub-questions (Q1a: which field maps to `order_number`; Q1b: source of `customer_name`) and the open `from_cp` decision, to be resolved when re-enabling.

### Q2. `CheckConfirmRows` always-false clause — **RESOLVED: fix**
Replace `data_annullamento <> null` with `data_annullamento IS NOT NULL` in the backend helper that counts confirmed rows. Cancelled rows now correctly contribute to the ATTIVO auto-promotion.

### Q3. `cdlan_int_fatturazione` enum — **RESOLVED: use `4` for Quadrimestrale, no DB migrations**

Canonical enum in the rewrite: `{1:Mensile, 2:Bimestrale, 3:Trimestrale, 4:Quadrimestrale, 6:Semestrale, 12:Annuale}` (matches the `{1,2,3,4,6,12}` months pattern).

**Read-path tolerance (no migration allowed).** Legacy prod rows may still contain `'5'` for Quadrimestrale (the Dettaglio `Order` SQL used `5`). The backend enum mapper accepts **both `'4'` and `'5'` as Quadrimestrale on read**. Writes (none in the current 1:1 scope) would always emit `'4'`. Zero-migration compatibility.

Same enum applies to `cdlan_dur_rin`.

### Q4. `cdlan_prezzo_attivazione` alias — **RESOLVED: standardize in the DTO**
Backend DTO field: `activation_price` (JSON). UI label: "Prezzo attivazione". The Dettaglio-only `'Attivazione'` alias does not leave the backend.

### Q5. Roles — **RESOLVED: `app_customer_relations`**

| Keycloak role | Capabilities in Ordini |
|---|---|
| `app_ordini_access` | Read Home list, read Dettaglio, edit `note_tecnici` (Informazioni dai tecnici tab), edit `cdlan_serialnumber` inline on Righe tab (only when order is BOZZA). |
| `app_customer_relations` | Info tab SALVA + input edits; INVIA in ERP; RICHIEDI ANNULLAMENTO; Download Kickoff; Genera Modulo Attivazione; Arxivar file picker; per-row "Modifica" iconButton + activation-date modal; Referenti SALVA. |

The role name is app-agnostic (not `app_ordini_customer_relations`) — intentional, per the expert, because the same role will gate equivalent actions in other mini-apps.

**Technicians** have no dedicated role: anyone authenticated with `app_ordini_access` who is not also `app_customer_relations` operates at technician level. Matches current Appsmith behaviour (Informazioni-tecnici edits are never role-gated).

### Q6. `arxivar.isDisabled` precedence bug — **RESOLVED: fix the rule**

Drop the Appsmith OR-chain. Canonical rule for the Arxivar file picker:

- **Enabled when:** `cdlan_stato NOT IN ('ANNULLATO', 'PERSO', 'ATTIVO')` AND user has `app_customer_relations`.
- **Disabled otherwise.**

(Effectively: only `BOZZA` and `INVIATO` orders, only CustomerRelations users. The original `arx_doc_number IS NOT NULL` clause is not re-imposed — if a later review shows re-uploading is undesirable once Arxivar has the signed PDF, add the check in Phase B.)

### Q7. `from_cp` annullamento rule — **RESOLVED: rolled into the deferred cancel-order task**
Since cancel-order is out of v1 scope, the `from_cp` re-imposition decision moves to the post-v1 follow-up in `docs/TODO.md`.

### Q8. Partial-success on ERP push — **RESOLVED: per-row semantics preserved, structured per-row feedback**

v1 semantics for `POST /api/ordini/:id/send-to-erp` (matches current Appsmith behaviour, upgrades the UX):
- Backend loops rows and calls `GW_SendToErp` for each one (no batch, no transactional rollback — GW is called one line at a time as today).
- Each row's outcome (success / failure + reason) is recorded.
- **State transition rule:** `cdlan_stato → INVIATO` + `cdlan_evaso → 1` + Arxivar PDF upload happen **only if every row succeeded**. On any row failure, vodka state is not flipped and the Arxivar upload is not performed — identical to the source's `err == 0` guard.
- Response (both success and partial-failure paths): a structured per-row outcome report `{rows: [{rowId, cdlan_systemodv_row, status: 'ok'|'error', error?}], stateTransitioned: bool, arxivarUploaded: bool}`. The UI renders the per-row status list so the operator can see exactly which rows made it to the ERP and which did not.
- On full success the UI confirms and navigates back to Home; on partial failure the UI stays on Dettaglio with the per-row list visible so the operator knows the divergence.

**Follow-up TODO** (rewritten in `docs/TODO.md` → Ordini App): post-v1 retry path for partial-failure — let a CustomerRelations user re-send only the rows that failed on a previous attempt, without duplicating rows that already made it into the ERP.

### Q9. `cdlan_cliente_id` column — **RESOLVED: surface it, preserve SQL**

Keep the `Order` SELECT list as-is (it already includes `cdlan_cliente_id` — no query change). Add the field to the Order DTO and render it on the Dettaglio page. Exact UI placement (Info tab header row alongside Ragione sociale, or Azienda tab) decided in Phase B.

---

## Phase A exit criteria

**Phase A complete.** Q1–Q9 all resolved (Q1/Q7 deferred post-v1 via TODO; others resolved inline). Ready to proceed to Phase B (UX Pattern Map).
