# Ordini — Migration Spec · Phase C: Logic Placement

For each non-trivial JSObject method / inline expression in the in-scope surface (Home + Dettaglio ordine), classify as **domain**, **orchestration**, or **presentation**, and assign a placement: **backend (Go)**, **frontend (React)**, or **shared**. Phase A/B resolutions are applied in-line (1:1 fixes and deferrals).

Dropped JSObjects (`Home.JSObject1`, `Ordini_semplificati.utils`, `Dettaglio.JSObject1` debug stub, `SendRequestAnnullaOdv`) are not listed here.

---

## 1. Send-to-ERP orchestration

**Today.** `SendToErp.run` (+ `setState`) loops `RigheOrdine.data`, calls `GW_SendToErp` per row with a ~40-field payload, accumulates an `err` flag, then — only if `err == 0` — calls `UpdateOrderState` (vodka → INVIATO + evaso=1) and `GW_SavePdfToArxivar` (multipart upload of the selected Arxivar PDF), finally `navigateTo('Home')`. Partial failure leaves ERP with committed rows and vodka in BOZZA.

**Classification.** Domain (state transition) + orchestration (three-system fan-out) + security-critical (role/state enforcement).

**Placement.** **Backend.** Single endpoint `POST /api/ordini/:id/send-to-erp` (multipart: the Arxivar PDF). Auth: `app_customer_relations` + `stato == 'BOZZA'` + preconditions (dataconferma set, cliente set) enforced server-side, not trusted from the client.

**Behaviour per Q8 (revised).**
- Per-row push, no transaction, no compensation — GW is called one line at a time as in the source. Each row's outcome is recorded.
- Vodka `cdlan_stato → INVIATO` + `cdlan_evaso → 1` and the Arxivar upload happen **only when every row succeeded** (matches the source's `err == 0` guard). On any row failure these side-effects are skipped.
- Response shape on both paths:
  ```json
  {
    "rows": [
      { "rowId": 1234, "cdlan_systemodv_row": 5678, "status": "ok" },
      { "rowId": 1235, "cdlan_systemodv_row": 5679, "status": "error", "error": "..." }
    ],
    "stateTransitioned": false,
    "arxivarUploaded": false
  }
  ```
- HTTP status: `200` on full success, `207 Multi-Status` on partial failure (or `200` with `stateTransitioned: false` if preferred — handler detail).
- UI renders the per-row list. On full success: confirmation + navigate back to Home. On partial failure: stay on Dettaglio, show which rows made it to the ERP and which did not, so the operator has the same mental model as the source (ERP has N committed rows, vodka still BOZZA).

**Alias note (Q4).** Where the source reads `item["Attivazione"]`, the backend reads `row.cdlan_prezzo_attivazione` directly; DTO exposes it as `activation_price`. The alias drift stops at the ORM boundary.

**Logic that does NOT port.**
- `Promise.all([awaited values])` pseudo-pattern — irrelevant in Go; sequential per-row calls with a short-circuit on failure.
- `navigateTo('Home')` inside the orchestration — moves to the frontend (on 200 success, router.push('/ordini')).
- `storeValue('tab', …)` vestigial calls — dropped.

---

## 2. Per-row activation + auto-ATTIVO

**Today.** `SetActivationDate.run` orchestrates `saveInVodka` (UPDATE `orders_rows` with `confirm_data_attivazione = 1` side-effect) → `saveInErp` (`GW_SetActivationDate`) → `checkRows` (COUNT) → if count equals total rows, `SetOrderStateAttivo` (UPDATE `orders.cdlan_stato = 'ATTIVO'`).

**Classification.** Domain (row state + implicit order state transition) + orchestration (vodka + GW).

**Placement.** **Backend.** Endpoint `PATCH /api/ordini/:id/rows/:rowId/activate` with body `{activation_date}`. Auth: `app_customer_relations` + order state must be `INVIATO`.

**Behaviour.**
- In one transaction: UPDATE `orders_rows` (set `cdlan_data_attivazione` and `confirm_data_attivazione = 1`), call `GW_SetActivationDate`, run `CheckConfirmRows` with the Q2 fix (`data_annullamento IS NOT NULL`), and if the count matches `SELECT COUNT(id) FROM orders_rows WHERE orders_id = :id`, flip `orders.cdlan_stato` to `ATTIVO` — all before commit.
- Transactional scope notes: the vodka UPDATE + COUNT + state flip are inside one DB transaction; the GW call is outside. If the GW call fails after the vodka UPDATE, the endpoint returns an error and the transaction rolls back. If the GW call succeeds and the DB commit fails, surface the error — the ERP is now ahead of vodka; operator must retry. Document this boundary in the handler.

**Logic that does NOT port.**
- The `Promise.all([awaited values]).then(…)` pattern.
- The error-swallowing `return false` in each JSObject method.
- The success alert after a "success" that could have silently failed.

---

## 3. PDF downloads

**Today.** Four separate JSObjects/actions:
- `GetPdf.kickOff` → `GW_Kickoff` → `download(response, "kick off_" + ndoc + "_" + anno + ".pdf", "application/pdf")`.
- `GetPdf.activationForm` → `GW_ActivationForm` → filename is `"Modulo di Attivazione"` (it) or `"Activation Form"` (en) based on `profile_lang.text`.
- `OrderTools.download` → `DownloadOrderPDFintGW` → base64-or-raw heuristic → blob download.
- `GetPdfOrdineArx.GetPdfOrdineArx(orderId)` → `GW_GetPDFArxivarOrder` → same base64-or-raw heuristic (duplicated).

**Classification.** Orchestration + presentation (filename localization).

**Placement.** **Backend** proxy endpoints, one per PDF kind. The base64-or-raw decode is done server-side once; clients receive `application/pdf` directly.

| Endpoint | GW target | Filename | Auth |
|---|---|---|---|
| `GET /api/ordini/:id/kickoff.pdf` | `GET /orders/v1/kick-off/:id` | `kick off_<ndoc>_<anno>.pdf` | `app_customer_relations` + `stato == 'INVIATO'` |
| `GET /api/ordini/:id/activation-form.pdf` | `GET /orders/v1/activation-form/:id` | `Modulo di Attivazione_<ndoc>_<anno>.pdf` or `Activation Form_<ndoc>_<anno>.pdf` based on `order.profile_lang` | `app_customer_relations` + `stato ∈ {INVIATO, ATTIVO}` |
| `GET /api/ordini/:id/pdf` | `GET /orders/v1/order/pdf/:id/generate` | `<ndoc>_<anno>.pdf` | `app_ordini_access` + `arx_doc_number IS NULL` |
| `GET /api/ordini/:id/signed-pdf` | `GET /orders/v1/order/pdf/:id?from=vodka` | `<ndoc>_<anno>_firmato.pdf` | `app_ordini_access` + `arx_doc_number IS NOT NULL` |

**Logic that does NOT port.**
- The base64-or-raw client-side heuristic — backend always returns `application/pdf` bytes with `Content-Disposition: attachment; filename="…"`.
- The `profile_lang.text` hidden widget — backend reads `orders.profile_lang` directly.

---

## 4. Row inline edits (serial number, technical notes)

**Today.**
- `utili.salvaRiga(row)` → `upd_row_serNum.run({cdlanSerialNumber, cdlanSystemodv})` → `RigheOrdine.run()`.
- `utili.salvaNoteTecniche(row)` → `upd_row_note_tecnici.run({noteTecnici, idRiga})` → `RigheOrdineTecnici.run()`.

Both read params through fragile widget aliases (e.g. the `idRiga` that maps to the `cdlan_systemodv_row AS 'ID riga'` alias).

**Classification.** Domain (data persistence) + trivial orchestration.

**Placement.** **Backend.**
- `PATCH /api/ordini/:id/rows/:rowId/serial-number` body `{serial_number}`. Auth: `app_ordini_access` + `stato == 'BOZZA'` (matches the current widget gate).
- `PATCH /api/ordini/:id/rows/:rowId/technical-notes` body `{technical_notes}`. Auth: `app_ordini_access` (no state/role gate beyond the app baseline — matches today).

The UTF8 conversion on read (`convert(note_tecnici using UTF8)`) stays in the read query exactly as-is. Writes go through parameterized queries (no raw-string interpolation).

**Note on `cdlan_systemodv_row` vs `id`.** The current UPDATE keys off `cdlan_systemodv_row`. The route param `:rowId` in the rewrite is the vodka `orders_rows.id`; the backend resolves to `cdlan_systemodv_row` (and validates `orders_id = :id`) before UPDATE. Drops the fragile alias-name dependency.

---

## 5. Header edit + Referents edit

**Today.**
- `SaveDataConfermaRifOrderCli.run()` (raw-string SQL) → `Order.run()`.
- `SaveOrderReferents.run()` (raw-string SQL) → order.run() (implicit via refresh).

**Classification.** Domain.

**Placement.** **Backend.**
- `PATCH /api/ordini/:id` body `{customer_po, confirmation_date, customer_id}` (three fields only — the other readonly Info-tab fields are immutable here). Auth: `app_customer_relations` + `stato == 'BOZZA'`. `customer_id` is the dropdown value from `erp_an_cli.selectedOptionValue` — i.e. `NUMERO_AZIENDA` from Alyante. **Per C2 (resolved):** the UPDATE writes **both** `orders.cdlan_cliente = <RAGIONE_SOCIALE>` (string, as today) and `orders.cdlan_cliente_id = <NUMERO_AZIENDA>`. No schema change — `cdlan_cliente_id` already exists in vodka and is already in the read SELECT (datasource-catalog.md:188). This strengthens the cross-DB linkage (Alyante ID ↔ Mistra `customers.customer.id` ↔ Grappa `cli_fatturazione.codice_aggancio_gest`, per `docs/IMPLEMENTATION-KNOWLEDGE.md`) at zero cost.
- `PATCH /api/ordini/:id/referents` body `{technical:{name,phone,email}, technical_alt:{…}, administrative:{…}}`. Auth: `app_customer_relations` + `stato ∈ {BOZZA, INVIATO}`.

All writes are parameterized.

---

## 6. Reference dropdowns

**Today.** `erp_anagrafiche_cli` (Alyante) loads on page mount, binds the `erp_an_cli` SINGLE_SELECT_TREE.

**Classification.** Reference read.

**Placement.** **Backend.** `GET /api/ordini/ref/customers` — runs the exact filter from the audit (`DATA_DISMISSIONE IS NULL AND RAGGRUPPAMENTO_3 <> 'Ecommerce' AND TIPOLOGIA_AZIENDA <> 'DIPENDENTE'`, grouped by both returned columns). Returns `[{id: NUMERO_AZIENDA, name: RAGIONE_SOCIALE}]`. Auth: `app_ordini_access`.

Dropped references (HubSpot potentials, payment methods) are not exposed.

---

## 7. Display mappings (presentation)

All move to the **frontend** as formatters — backend returns raw codes.

| Source code | Raw field | Labels |
|---|---|---|
| `cdlan_tipo_ord` | `A` / `N` / `R` | Sostituzione / Nuovo / Rinnovo |
| `cdlan_tipodoc` | `TSC-ORDINE-RIC` / `TSC-ORDINE` | Ordine ricorrente / Ordine Spot |
| `from_cp` | `0` / `≠0` | No / Sì |
| `cdlan_evaso` | `0` / `1` | No / Sì (B2) |
| `cdlan_int_fatturazione` | `1/2/3/4/6/12` (accept legacy `5` as 4) | Mensile / Bimestrale / Trimestrale / Quadrimestrale / Semestrale / Annuale (per Q3) |
| `cdlan_int_fatturazione_att` | `1` / `2` | All'ordine / All'attivazione della Soluzione/Consegna |
| `cdlan_dur_rin` | same as fatturazione | same labels |
| `cdlan_tacito_rin` | `0` / `1` | No / Sì |
| `is_colo` | `0` / string | "Altre soluzioni" / raw string (Colocation variabile, Iaas payperuse, Iaas payperuse indiretto) |
| `service_type` | comma-joined string | split + render chips |
| `cdlan_data_attivazione` | date-ish | `DD/MM/YYYY` or `-` |
| `profile_lang` | `it` / `en` | "Italiano" / "English" (display), but also drives backend filename localization |

Mapping table lives in a single TS module in `apps/ordini/src/…/formatters.ts`. No cross-app package — avoid premature abstraction.

---

## 8. Role / state gates (defense in depth)

**Today.** All `isVisible`/`isDisabled` gates live in Appsmith widget bindings. The `CustomerRelations` check is purely client-side.

**Placement.** **Dual — frontend presentation + backend enforcement.**
- **Frontend:** the same widget gates re-expressed as React conditionals; rules centralized in a helper `canEditBozzaHeader(order, user)`, `canSendToErp(order, user, attachments)`, `canEditReferents(order, user)`, `canOpenActivationModal(order, user)`, `canEditSerialNumber(order, user)`, `canEditTechnicalNotes(order, user)`, `canShowArxivarFilePicker(order, user)` (Q6 fix). These gates drive visibility/disabled state only — they are advisory.
- **Backend:** every mutating endpoint re-checks the same preconditions (role + state + entity-level constraints like `dataconferma set`) from the server-held order record, never trusting the client payload. 403 on role mismatch; 409 on state mismatch.

**Role token plumbing.** `app_ordini_access` is the baseline gate at the router level. `app_customer_relations` is checked per-handler against the Keycloak claims. Technicians have no dedicated role — any `app_ordini_access` user can write `note_tecnici` + (BOZZA) serial numbers.

---

## 9. Rules being **revised** rather than ported

Collected from Phases A/B so Phase D has a single reference:

| Rule | Source behaviour | New behaviour | Rationale |
|---|---|---|---|
| `CheckConfirmRows` | `data_annullamento <> null` (always false) | `data_annullamento IS NOT NULL` | Q2 — intent-preserving fix. |
| `cdlan_int_fatturazione` decoding | CASE with `'5' → Quadrimestrale` | enum `{1,2,3,4,6,12}` + read-time `'5' → 4` alias | Q3 — align with `{1,2,3,4,6,12}` months semantics, preserve legacy data without migration. |
| `activation_price` alias | `'Attivazione'` vs `'Prezzo attivazione'` drift | single DTO field `activation_price`; UI label "Prezzo attivazione" | Q4 — kill the alias ambiguity. |
| `CustomerRelations` group gate | client-side only | dual enforcement; role renamed `app_customer_relations` | Q5 — security boundary. |
| Arxivar file-picker gate | buggy OR chain | `stato NOT IN {ANNULLATO, PERSO, ATTIVO} AND app_customer_relations` | Q6 — matches intent. |
| Send-to-ERP partial-success | per-row loop, state flip only on total success, silent alert on failure | **same per-row semantics**, plus structured per-row outcome report the UI renders explicitly | Q8 — preserve source behaviour, fix only the UX opacity. |
| RICHIEDI ANNULLAMENTO | broken Appsmith binding | **removed from v1**; re-enabled via TODO | Q1 — avoid porting a plausibly-broken path. |
| ORDINE PERSO | hidden button | **removed**; no PERSO transition from this app | no valid flow in the audit. |
| Display mappings (Tipo doc, Tipo proposta, Dal CP, etc.) | SQL CASE in read queries | frontend formatter | standard separation of concerns. |
| PDF base64-or-raw heuristic | duplicated in two JSObjects | backend normalizes; client receives `application/pdf` | kill fragile heuristic. |
| `profile_lang.text` hidden widget | global accessor | backend derives filename from `orders.profile_lang` | kill hidden-widget-as-global pattern. |
| Raw-string SQL interpolation | 15 vodka queries | all parameterized via Go ORM / `sql.DB` | SQLi. |
| Direct multi-DB connections from browser | 4 datasources in Appsmith | backend-only; frontend never sees DB | security boundary. |

---

## 10. Open questions for Phase C

### C1. — **RESOLVED: per-row push, per-row UI feedback**

No transactional rollback against the ERP. The endpoint calls `GW_SendToErp` one row at a time (as the source does today) and returns a structured per-row outcome report. vodka state flips only on full success (source parity). The UI renders every row's status so the operator sees exactly which lines made it to the ERP. Post-v1 follow-up for partial-failure retry tracked in `docs/TODO.md`.

### C2. `customer_id` persistence in v1 — **RESOLVED: write both `cdlan_cliente` (string) and `cdlan_cliente_id` (NUMERO_AZIENDA) on BOZZA header save**

The BOZZA header save UPDATE sets both columns from the same dropdown value. Zero schema change (column already exists). This is a deliberate, minimal deviation from strict 1:1: Appsmith reads `cdlan_cliente_id` but never writes it; the rewrite both reads and writes it.

The write happens only at edit point #1 (Info tab SALVA) — once the order leaves BOZZA, the Ragione sociale dropdown is not editable, so there is no second backfill opportunity through the UI.

---

## Phase C exit criteria

**Phase C complete.** C1 → per-row push, per-row UI feedback, no transaction. C2 → write both `cdlan_cliente` and `cdlan_cliente_id` on BOZZA header save. Ready for Phase D (Integration & Data Flow).
